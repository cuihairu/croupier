#!/usr/bin/env python3
"""Reference probe plugin: probe-echo (todo.md:40).

Mirrors the Go probe plugin example in Python, so the probe contract has
a cross-language proof: the real synthetics controller drives this
process through the real go-plugin handshake in the SDK test suite
(hostdriver scenarios).

Semantics: a target named "fail..." always reports ok=False with the
synthetic reason; anything else passes with status 200. An optional
{"sleep_ms": N} payload makes the check wait first (bounded by the RPC
context), which is what the host-side deadline scenario drives.

Run (the host supplies the handshake env; direct execution is refused):
    python3 examples/probe_echo.py
"""

from __future__ import annotations

import json
import sys
import time
from pathlib import Path

if __package__ in (None, ""):
    # Runnable from a checkout without pip install; installed via
    # `pip install -e sdks/plugins/python` in CI.
    sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import croupier_plugin as cp  # noqa: E402

PLUGIN_NAME = "probe-echo"
PLUGIN_VERSION = "1.0.0"


class ProbeEchoPlugin(cp.ProbePlugin):
    def describe(self) -> tuple[str, str]:
        return PLUGIN_NAME, PLUGIN_VERSION

    def check(self, call: cp.CheckInput) -> cp.CheckResult:
        started = time.monotonic()

        sleep_ms = 0
        if call.payload_json:
            try:
                payload = json.loads(call.payload_json)
                if isinstance(payload, dict) and isinstance(payload.get("sleep_ms"), int):
                    sleep_ms = payload["sleep_ms"]
            except ValueError:
                pass  # payload is plugin-specific; unparseable means "no payload"
        deadline = time.monotonic() + max(0, sleep_ms) / 1000.0
        while time.monotonic() < deadline:
            if not call.context.is_active():
                return cp.FailResult(
                    "check cancelled", latency_ms=self._ms(started)
                )
            time.sleep(min(0.02, max(0.0, deadline - time.monotonic())))

        latency = self._ms(started)
        if call.target.startswith("fail"):
            return cp.FailResult(
                "synthetic probe failure",
                status=500,
                latency_ms=latency,
                detail=f"probe-echo checked {call.target}",
            )
        return cp.PassResult(
            status=200,
            latency_ms=latency,
            detail=f"probe-echo checked {call.target}",
        )

    @staticmethod
    def _ms(started: float) -> int:
        return int((time.monotonic() - started) * 1000)


def main() -> None:
    cp.serve_probe_plugin(ProbeEchoPlugin())


if __name__ == "__main__":
    main()
