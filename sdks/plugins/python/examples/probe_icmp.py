#!/usr/bin/env python3
"""Reference ICMP probe plugin for hosts with CAP_NET_RAW.

The destination is passed through ``payloadJson`` as
``{"host": "127.0.0.1", "count": 1}``. A missing capability is a normal
plugin failure result, never a process crash.
"""

from __future__ import annotations

import json
import os
import socket
import struct
import sys
import time
from pathlib import Path

if __package__ in (None, ""):
    sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import croupier_plugin as cp  # noqa: E402

PLUGIN_NAME = "probe-icmp"
PLUGIN_VERSION = "1.0.0"
ICMP_ECHO_REQUEST = 8
ICMP_ECHO_REPLY = 0


class ProbeICMPPlugin(cp.ProbePlugin):
    def describe(self) -> tuple[str, str]:
        return PLUGIN_NAME, PLUGIN_VERSION

    def check(self, call: cp.CheckInput) -> cp.CheckResult:
        started = time.monotonic()
        try:
            payload = json.loads(call.payload_json)
            host = payload["host"]
            count = int(payload.get("count", 1))
            forced = payload.get("force_capability_error") is True
        except (KeyError, TypeError, ValueError):
            return cp.FailResult(
                "configuration: payload must contain host",
                latency_ms=self._ms(started),
            )
        if not isinstance(host, str) or not host or count < 1:
            return cp.FailResult(
                "configuration: host must be a non-empty string and count >= 1",
                latency_ms=self._ms(started),
            )
        if forced:
            return cp.FailResult(
                "capability: CAP_NET_RAW is required for ICMP: forced by test",
                latency_ms=self._ms(started),
            )

        deadline = time.monotonic() + max(0.001, call.timeout_ms / 1000.0)
        for _ in range(count):
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                return cp.FailResult("timeout: no ICMP reply", latency_ms=self._ms(started))
            error = self._ping(host, remaining)
            if error is None:
                return cp.PassResult(
                    status=200,
                    latency_ms=self._ms(started),
                    detail=f"probe-icmp received reply from {host}",
                )
            if error.startswith("capability:") or not error.startswith("timeout:"):
                return cp.FailResult(error, latency_ms=self._ms(started))
        return cp.FailResult("timeout: no ICMP reply", latency_ms=self._ms(started))

    def _ping(self, host: str, timeout: float) -> str | None:
        try:
            address = socket.getaddrinfo(host, None, 0, socket.SOCK_RAW)[0][4][0]
            family = socket.AF_INET if ":" not in address else socket.AF_INET6
            sock = socket.socket(family, socket.SOCK_RAW, socket.IPPROTO_ICMP)
        except PermissionError as exc:
            return f"capability: CAP_NET_RAW is required for ICMP: {exc}"
        except OSError as exc:
            return f"dns: {exc}"

        identifier = os.getpid() & 0xFFFF
        sequence = int(time.monotonic() * 1000) & 0xFFFF
        header = struct.pack("!BBHHH", ICMP_ECHO_REQUEST, 0, 0, identifier, sequence)
        payload = os.urandom(8)
        checksum = self._checksum(header + payload)
        packet = struct.pack("!BBHHH", ICMP_ECHO_REQUEST, 0, checksum, identifier, sequence) + payload
        with sock:
            sock.settimeout(timeout)
            try:
                sock.sendto(packet, (address, 0))
                while True:
                    data, _ = sock.recvfrom(1500)
                    if family == socket.AF_INET and len(data) > 20:
                        data = data[20:]
                    if len(data) < 8:
                        continue
                    received_type, _, _, received_id, received_seq = struct.unpack("!BBHHH", data[:8])
                    if received_type == ICMP_ECHO_REPLY and received_id == identifier and received_seq == sequence:
                        return None
            except socket.timeout:
                return "timeout: no ICMP reply"
            except OSError as exc:
                return f"network: {exc}"
        return "timeout: no ICMP reply"

    @staticmethod
    def _checksum(data: bytes) -> int:
        if len(data) % 2:
            data += b"\0"
        total = sum(struct.unpack("!%dH" % (len(data) // 2), data))
        total = (total >> 16) + (total & 0xFFFF)
        return ~total & 0xFFFF

    @staticmethod
    def _ms(started: float) -> int:
        return int((time.monotonic() - started) * 1000)


def main() -> None:
    cp.serve_probe_plugin(ProbeICMPPlugin())


if __name__ == "__main__":
    main()
