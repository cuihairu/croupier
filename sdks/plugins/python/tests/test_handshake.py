"""Handshake tests against real plugin subprocesses.

These run the reference plugins as the host would launch them — env
vars, stdout protocol line, signal shutdown — so the go-plugin local
gRPC protocol is proven against the real grpcio runtime, not a mock.
"""

from __future__ import annotations

import os
import signal
import subprocess
import sys
from pathlib import Path

from croupier_plugin.funcplugin import COOKIE_KEY, COOKIE_VALUE

ROOT = Path(__file__).resolve().parents[1]
FUNC_ECHO = ROOT / "examples" / "func_echo.py"
PROBE_ECHO = ROOT / "examples" / "probe_echo.py"

FUNC_ENV = {
    COOKIE_KEY: COOKIE_VALUE,
    "PLUGIN_PROTOCOL_VERSIONS": "1",
}


def _env(extra: dict[str, str]) -> dict[str, str]:
    env = os.environ.copy()
    # A stale cookie from the ambient environment must not leak into a
    # case that expects its absence.
    env.pop(COOKIE_KEY, None)
    env.pop("PLUGIN_PROTOCOL_VERSIONS", None)
    env.pop("PLUGIN_MULTIPLEX_GRPC", None)
    env.update(extra)
    return env


def _run_and_read_handshake(
    script: Path, extra: dict[str, str]
) -> tuple[subprocess.Popen, list[str]]:
    proc = subprocess.Popen(
        [sys.executable, str(script)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=_env(extra),
        text=True,
    )
    # Startup failures (refusals) still write a full buffered line or
    # close the pipe; readline blocks at most until the process exits.
    assert proc.stdout is not None
    line = proc.stdout.readline().rstrip("\n")
    return proc, line.split("|")


def test_direct_execution_is_refused() -> None:
    proc, fields = _run_and_read_handshake(FUNC_ECHO, {})
    assert proc.wait(timeout=10) == 1
    assert "not meant to be executed directly" in (proc.stderr.read() or "")


def test_wrong_cookie_value_is_refused() -> None:
    proc, _ = _run_and_read_handshake(FUNC_ECHO, {COOKIE_KEY: "nope"})
    assert proc.wait(timeout=10) == 1


def test_unsupported_protocol_version_is_refused() -> None:
    proc, _ = _run_and_read_handshake(
        FUNC_ECHO, {COOKIE_KEY: COOKIE_VALUE, "PLUGIN_PROTOCOL_VERSIONS": "2,3"}
    )
    assert proc.wait(timeout=10) != 0
    assert "speaks plugin protocol" in (proc.stderr.read() or "")


def test_invalid_protocol_version_list_is_refused() -> None:
    proc, _ = _run_and_read_handshake(
        FUNC_ECHO, {COOKIE_KEY: COOKIE_VALUE, "PLUGIN_PROTOCOL_VERSIONS": "1,banana"}
    )
    assert proc.wait(timeout=10) != 0
    assert "invalid plugin protocol version" in (proc.stderr.read() or "")


def test_handshake_line_and_sigterm_shutdown() -> None:
    proc, fields = _run_and_read_handshake(FUNC_ECHO, FUNC_ENV)
    try:
        # core|protocol|network|addr|protocol|server-cert
        assert len(fields) == 6, fields
        assert fields[0] == "1"  # go-plugin core protocol version
        assert fields[1] == "1"  # negotiated plugin protocol version
        assert fields[2] == "unix"
        assert fields[3].endswith(".sock")
        assert fields[4] == "grpc"
        assert fields[5] == ""  # no server cert: the IPC runs without mTLS
        assert Path(fields[3]).parent.name.startswith("plugin")
    finally:
        proc.send_signal(signal.SIGTERM)
    # SIGTERM takes the bounded graceful-stop path and exits cleanly.
    assert proc.wait(timeout=10) == 0


def test_multiplex_flag_adds_seventh_field() -> None:
    proc, fields = _run_and_read_handshake(
        FUNC_ECHO, {**FUNC_ENV, "PLUGIN_MULTIPLEX_GRPC": "1"}
    )
    try:
        assert len(fields) == 7, fields
        assert fields[6] == "true"
    finally:
        proc.send_signal(signal.SIGTERM)
    assert proc.wait(timeout=10) == 0


def test_probe_example_speaks_the_probe_handshake() -> None:
    from croupier_plugin.probeplugin import COOKIE_KEY as PROBE_KEY
    from croupier_plugin.probeplugin import COOKIE_VALUE as PROBE_VALUE

    proc, fields = _run_and_read_handshake(
        PROBE_ECHO, {PROBE_KEY: PROBE_VALUE, "PLUGIN_PROTOCOL_VERSIONS": "1"}
    )
    try:
        assert len(fields) == 6
        assert fields[1] == "1"
    finally:
        proc.send_signal(signal.SIGTERM)
    assert proc.wait(timeout=10) == 0
