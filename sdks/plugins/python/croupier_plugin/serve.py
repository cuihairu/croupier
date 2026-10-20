"""go-plugin local gRPC server plumbing shared by the Croupier plugin
SDKs (todo.md:38 / todo.md:40).

This module speaks the subprocess half of HashiCorp go-plugin's local
gRPC mode (pinned at the version in go.mod): the magic-cookie gate, the
PLUGIN_PROTOCOL_VERSIONS negotiation, the one-line stdout handshake, the
gRPC control-plane services the host touches at startup (GRPCStdio,
GRPCController, grpc.health.v1) and a bounded graceful shutdown.

A plugin process must never write anything but the handshake line to
stdout — the host parses stdout as a protocol channel. Plugin logs go to
stderr, which the host captures.
"""

from __future__ import annotations

import os
import signal
import socket
import sys
import tempfile
import threading
import time
from concurrent import futures
from typing import Callable, Iterable

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from . import plugin_controller_pb2_grpc, plugin_stdio_pb2_grpc

# go-plugin's CoreProtocolVersion: the version of the plugin system
# itself, carried as the first field of the handshake line.
CORE_PROTOCOL_VERSION = 1

# How long Shutdown()/SIGTERM gets to drain in-flight RPCs before the
# process exits; bounded, same discipline as the host's bounded drain.
STOP_GRACE_SECONDS = 2.0


class PluginHandshakeError(Exception):
    """The host environment does not speak a protocol this plugin serves."""


def _require_cookie(cookie_key: str, cookie_value: str) -> None:
    """Fail closed unless the host presented the exact magic cookie.

    This is what keeps an accidental direct execution from doing plugin
    work: without the cookie env the process is just a script.
    """
    if not cookie_key or not cookie_value:
        raise PluginHandshakeError(
            "no magic cookie key or value was configured; "
            "report this as an SDK bug"
        )
    if os.environ.get(cookie_key, "") != cookie_value:
        sys.stderr.write(
            "This binary is a plugin. These are not meant to be executed directly.\n"
            "Please execute the program that consumes these plugins, which will\n"
            "load any plugins automatically\n"
        )
        raise SystemExit(1)


def negotiate_protocol_version(supported: int) -> int:
    """Pick the protocol version shared with the host.

    go-plugin hosts always send PLUGIN_PROTOCOL_VERSIONS as a
    comma-separated list of the versions they can speak; the plugin must
    answer with one of them or refuse to start.
    """
    raw = os.environ.get("PLUGIN_PROTOCOL_VERSIONS", "")
    if not raw:
        # A host that did not send the list is a legacy host; the pinned
        # go-plugin clients this SDK targets always send it.
        return supported
    offered: list[int] = []
    for part in raw.split(","):
        part = part.strip()
        if not part:
            continue
        try:
            offered.append(int(part))
        except ValueError:
            raise PluginHandshakeError(
                f"host sent an invalid plugin protocol version {part!r}"
            ) from None
    if supported not in offered:
        offered_str = ",".join(str(v) for v in offered) or "(empty)"
        raise PluginHandshakeError(
            f"host speaks plugin protocol {offered_str} but this plugin "
            f"serves {supported}"
        )
    return supported


def _bind(server: grpc.Server) -> tuple[str, str]:
    """Bind the gRPC server on a local endpoint and return (network, addr)
    for the handshake line. Prefers a unix domain socket like the Go
    plugin runtime; falls back to loopback TCP on platforms without one.
    """
    if hasattr(socket, "AF_UNIX"):
        path = os.path.join(tempfile.mkdtemp(prefix="plugin"), "grpc.sock")
        try:
            os.unlink(path)
        except OSError:
            pass
        if server.add_insecure_port(f"unix://{path}") == 0:
            raise PluginHandshakeError(f"cannot bind the plugin unix socket {path}")
        return "unix", path
    port = server.add_insecure_port("tcp://127.0.0.1:0")
    if port == 0:
        raise PluginHandshakeError("cannot bind the plugin TCP socket")
    return "tcp", f"127.0.0.1:{port}"


def _handshake_line(protocol_version: int, network: str, addr: str) -> str:
    """Build the stdout protocol line, field by field:

    core|protocol|network|addr|protocol|server-cert[|multiplex]

    The server certificate field is empty — the Croupier hosts run the
    plugin IPC without mTLS; the field stays in the format because the
    host parses by position.
    """
    line = (
        f"{CORE_PROTOCOL_VERSION}|{protocol_version}|{network}|{addr}|grpc|"
    )
    if os.environ.get("PLUGIN_MULTIPLEX_GRPC"):
        line += "|true"
    return line


class _StdioServicer(plugin_stdio_pb2_grpc.GRPCStdioServicer):
    """GRPCStdio: the host opens this stream right after the handshake and
    forwards everything the plugin sends onto its own synced
    stdout/stderr. The Croupier plugin SDK emits nothing — plugin logs
    already go to plain stderr, which the host captures — but the stream
    must stay open, so the generator parks while the RPC is active.
    """

    def StreamStdio(self, request, context):  # noqa: N802 (grpc naming)
        while context.is_active():
            time.sleep(0.2)


class _ControllerServicer(plugin_controller_pb2_grpc.GRPCControllerServicer):
    """GRPCController: the host's graceful Kill. Shutdown must not block
    the RPC it arrives on (the host waits for the reply before tearing
    down), so the actual stop runs on a separate thread.
    """

    def __init__(self, stop: threading.Event) -> None:
        self._stop = stop

    def Shutdown(self, request, context):  # noqa: N802 (grpc naming)
        threading.Thread(target=self._stop.set, daemon=True).start()
        from . import plugin_controller_pb2

        return plugin_controller_pb2.Empty()


def serve_plugin(
    cookie_key: str,
    cookie_value: str,
    register: Callable[[grpc.Server], None],
    protocol_version: int = 1,
) -> None:
    """Serve one plugin process over the go-plugin local gRPC protocol.

    Blocks until the host shuts the plugin down (GRPCController.Shutdown
    or SIGTERM), then drains in-flight RPCs within STOP_GRACE_SECONDS and
    returns. Plugin authors do not call this directly; the contract
    modules (funcplugin / probeplugin) wrap it with their handshake
    constants and servicer registration.
    """
    _require_cookie(cookie_key, cookie_value)
    version = negotiate_protocol_version(protocol_version)

    stop = threading.Event()
    # Worker capacity must exceed the host engine's default concurrency
    # (core/pluginhost DefaultMaxConcurrent = 8): GRPCStdio parks one
    # worker for the whole process lifetime, so 8 workers would starve the
    # ninth concurrent call.
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=16))
    register(server)
    plugin_stdio_pb2_grpc.add_GRPCStdioServicer_to_server(_StdioServicer(), server)
    plugin_controller_pb2_grpc.add_GRPCControllerServicer_to_server(
        _ControllerServicer(stop), server
    )
    health_servicer = health.HealthServicer()
    health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)

    network, addr = _bind(server)
    server.start()
    sys.stdout.write(_handshake_line(version, network, addr) + "\n")
    sys.stdout.flush()

    def _on_signal(signum, frame):  # noqa: ANN001 (signal handler signature)
        stop.set()

    for sig in (signal.SIGTERM, signal.SIGINT):
        signal.signal(sig, _on_signal)
    stop.wait()
    server.stop(STOP_GRACE_SECONDS).wait(STOP_GRACE_SECONDS + 1)
