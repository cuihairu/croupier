"""Synthetics probe plugin SDK (todo.md:40).

A probe plugin is a resident subprocess the synthetics controller
launches and asks to check one configured target. The host keeps only
the check; the plugin holds no Croupier credentials.

Slice 1 only implements describe/check: status page rendering, plugin
routing, probe groups and alert policies are server-side and out of
scope for a probe plugin.
"""

from __future__ import annotations

import abc
import time
from dataclasses import dataclass

import grpc

from . import probeplugin_pb2, probeplugin_pb2_grpc
from . import serve as _serve

# Handshake constants mirrored from the host (agents/synthetics/
# pluginhost/contract.go); a mismatch fails the subprocess before any
# RPC happens.
COOKIE_KEY = "CROUPIER_SYNTHETICS_PLUGIN"
COOKIE_VALUE = "croupier-synthetics-probe-plugin"
PROTOCOL_VERSION = 1


def _now_ms() -> int:
    return int(time.time() * 1000)


@dataclass(frozen=True, slots=True)
class CheckInput:
    """One host->plugin check.

    target is the configured target name (informational, no URL
    passed), timeout_ms is the remaining host-side deadline and
    payload_json is an optional plugin-specific payload. The plugin
    keeps the target parameters it needs as its own config, exactly
    like a function adapter keeps its game interface address.
    """

    target: str
    timeout_ms: int
    payload_json: str
    context: grpc.ServicerContext


@dataclass(slots=True)
class CheckResult:
    """One probe outcome. status is plugin-semantic (e.g. an HTTP status
    for an HTTP-like probe); error is the readable reason when ok=False
    and detail carries the plugin-specific diagnostic."""

    ok: bool
    status: int = 0
    latency_ms: int = 0
    error: str = ""
    detail: str = ""
    at_unix_ms: int | None = None

    def to_proto(self) -> probeplugin_pb2.CheckResult:
        return probeplugin_pb2.CheckResult(
            ok=self.ok,
            status=self.status,
            latency_ms=self.latency_ms,
            error=self.error,
            detail=self.detail,
            at_unix_ms=self.at_unix_ms if self.at_unix_ms is not None else _now_ms(),
        )


@dataclass(init=False, slots=True)
class PassResult(CheckResult):
    """Convenience success constructor: ok=True with a status code and
    the measured latency."""

    def __init__(
        self,
        status: int = 0,
        latency_ms: int = 0,
        detail: str = "",
        at_unix_ms: int | None = None,
    ) -> None:
        # Explicit parent init: dataclass(slots=True) rebuilds the class,
        # so a zero-arg super() inside __init__ resolves against the stale
        # __class__ cell and raises TypeError on Python 3.12/3.13.
        CheckResult.__init__(
            self,
            ok=True,
            status=status,
            latency_ms=latency_ms,
            detail=detail,
            at_unix_ms=at_unix_ms,
        )


@dataclass(init=False, slots=True)
class FailResult(CheckResult):
    """Convenience failure constructor: ok=False with the readable
    reason; status is the observed plugin-semantic status when there is
    one."""

    def __init__(
        self,
        error: str,
        status: int = 0,
        latency_ms: int = 0,
        detail: str = "",
        at_unix_ms: int | None = None,
    ) -> None:
        # Same explicit parent init as PassResult (see the note there).
        CheckResult.__init__(
            self,
            ok=False,
            status=status,
            latency_ms=latency_ms,
            error=error,
            detail=detail,
            at_unix_ms=at_unix_ms,
        )


class ProbePlugin(abc.ABC):
    """Contract a synthetics probe plugin implements.

    The host issues describe once after the handshake to stamp plugin
    metadata onto every probe result, and check per probe run.
    """

    @abc.abstractmethod
    def describe(self) -> tuple[str, str]:
        """Return (plugin_name, version). The name must match the
        controller's configured plugin name."""

    @abc.abstractmethod
    def check(self, call: CheckInput) -> CheckResult:
        """Check one target. call.context.is_active() is False once the
        host's per-call deadline fires or the run is cancelled — poll it
        across a long wait instead of sleeping past the deadline."""


class _ProbePluginServicer(probeplugin_pb2_grpc.ProbePluginServicer):
    def __init__(self, impl: ProbePlugin) -> None:
        self._impl = impl

    def Describe(self, request, context):  # noqa: N802 (grpc naming)
        name, version = self._impl.describe()
        return probeplugin_pb2.DescribeResponse(name=name, version=version)

    def Check(self, request, context):  # noqa: N802 (grpc naming)
        call = CheckInput(
            target=request.target,
            timeout_ms=request.timeout_ms,
            payload_json=request.payload_json,
            context=context,
        )
        return self._impl.check(call).to_proto()


def serve(plugin: ProbePlugin) -> None:
    """Run one probe plugin until the controller shuts it down."""
    _serve.serve_plugin(
        COOKIE_KEY,
        COOKIE_VALUE,
        lambda server: probeplugin_pb2_grpc.add_ProbePluginServicer_to_server(
            _ProbePluginServicer(plugin), server
        ),
        PROTOCOL_VERSION,
    )
