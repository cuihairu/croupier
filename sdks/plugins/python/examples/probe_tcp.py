#!/usr/bin/env python3
"""Reference TCP probe plugin for non-HTTP destinations.

The host passes this plugin's destination through ``payloadJson``:

    {"host": "127.0.0.1", "port": 5432}

TCP reachability is intentionally only a connection assertion. It must
never be reported as a game-protocol login success; game-protocol
plugins own that stronger semantic.
"""

from __future__ import annotations

import json
import socket
import sys
import time
from pathlib import Path

if __package__ in (None, ""):
    sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import croupier_plugin as cp  # noqa: E402

PLUGIN_NAME = "probe-tcp"
PLUGIN_VERSION = "1.0.0"


class ProbeTCPPlugin(cp.ProbePlugin):
    def describe(self) -> tuple[str, str]:
        return PLUGIN_NAME, PLUGIN_VERSION

    def check(self, call: cp.CheckInput) -> cp.CheckResult:
        started = time.monotonic()
        try:
            payload = json.loads(call.payload_json)
            sleep_ms = payload.get("sleep_ms", 0)
        except (AttributeError, TypeError, ValueError):
            sleep_ms = 0
        if isinstance(sleep_ms, int) and sleep_ms > 0:
            deadline = time.monotonic() + sleep_ms / 1000.0
            while time.monotonic() < deadline:
                if not call.context.is_active():
                    return cp.FailResult("check cancelled", latency_ms=self._ms(started))
                time.sleep(min(0.02, max(0.0, deadline - time.monotonic())))
        try:
            payload = json.loads(call.payload_json)
            host = payload["host"]
            port = payload["port"]
        except (KeyError, TypeError, ValueError):
            return cp.FailResult(
                "configuration: payload must contain host and port",
                latency_ms=self._ms(started),
            )
        if not isinstance(host, str) or not isinstance(port, int) or not 1 <= port <= 65535:
            return cp.FailResult(
                "configuration: host must be a string and port an integer in [1, 65535]",
                latency_ms=self._ms(started),
            )

        timeout = max(0.001, call.timeout_ms / 1000.0)
        try:
            with socket.create_connection((host, port), timeout=timeout) as sock:
                sock.settimeout(0)
                if sock.getpeername() is None:
                    raise OSError("no peer address")
        except OSError as exc:
            return cp.FailResult(
                f"tcp: {exc.strerror or exc}",
                latency_ms=self._ms(started),
                detail=f"probe-tcp checked {host}:{port}",
            )
        return cp.PassResult(
            status=200,
            latency_ms=self._ms(started),
            detail=f"probe-tcp connected to {host}:{port}",
        )

    @staticmethod
    def _ms(started: float) -> int:
        return int((time.monotonic() - started) * 1000)


def main() -> None:
    cp.serve_probe_plugin(ProbeTCPPlugin())


if __name__ == "__main__":
    main()
