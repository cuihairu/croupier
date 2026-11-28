#!/usr/bin/env python3
"""Reference Minecraft Java Server List Ping (SLP) probe plugin.

The first game-protocol reference probe (todo PROBE-001): SLP is the
public read-only status query (handshake state=1 -> status request ->
status response JSON) per wiki.vg, implemented with the standard library
only. It never touches the login/encryption chain, so it verifies any
Java server with zero accounts — and a pass here is server-status
evidence only, never player-login evidence (binding clause from
mc-online-login-survey-2026-11-28).

Host payload: {"host": "127.0.0.1", "port": 25565} — port optional,
defaulting to Minecraft Java's 25565.

The internal read budget is 80% of the host deadline so a silent
non-SLP target surfaces as a plugin business failure before the
host-side degradation deadline races it.
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

PLUGIN_NAME = "probe-mc-slp"
PLUGIN_VERSION = "1.0.0"

DEFAULT_PORT = 25565
MAX_STATUS_BYTES = 2 << 20
# Handshake declares 1.8.9; servers do not validate the version on the
# status path (version negotiation happens in the login flow).
DECLARED_PROTOCOL = 47


class SLPError(Exception):
    """A game-protocol failure that maps onto a plugin business failure."""


class SLPCancelled(Exception):
    """The host cancelled the check while a socket read was pending."""


class ProbeMCSLPPlugin(cp.ProbePlugin):
    def describe(self) -> tuple[str, str]:
        return PLUGIN_NAME, PLUGIN_VERSION

    def check(self, call: cp.CheckInput) -> cp.CheckResult:
        started = time.monotonic()

        def ms() -> int:
            return int((time.monotonic() - started) * 1000)

        try:
            payload = json.loads(call.payload_json) if call.payload_json else {}
        except ValueError:
            payload = {}
        host = payload.get("host") if isinstance(payload, dict) else None
        port = payload.get("port") if isinstance(payload, dict) else None
        if not isinstance(host, str) or not host:
            return cp.FailResult(
                "configuration: payload must contain host (port optional, default 25565)",
                latency_ms=ms(),
            )
        if port is None:
            port = DEFAULT_PORT
        if isinstance(port, bool) or not isinstance(port, int) or not 1 <= port <= 65535:
            return cp.FailResult(
                "configuration: port must be an integer in [1, 65535]",
                latency_ms=ms(),
            )

        # 80% read budget: answer before the host deadline (see module doc).
        deadline = time.monotonic() + max(0.05, call.timeout_ms * 0.8 / 1000.0)
        try:
            with socket.create_connection(
                (host, port), timeout=max(0.001, deadline - time.monotonic())
            ) as sock:
                sock.sendall(_handshake(host, port) + _status_request())
                status_json = _read_status(sock, deadline, call)
        except SLPCancelled:
            return cp.FailResult("check cancelled", latency_ms=ms())
        except (OSError, SLPError) as exc:
            return cp.FailResult(
                f"minecraft slp: {exc}",
                latency_ms=ms(),
                detail=f"probe-mc-slp checked {host}:{port}",
            )

        try:
            status = json.loads(status_json)
        except ValueError as exc:
            return cp.FailResult(
                f"minecraft slp: invalid status json: {exc}", latency_ms=ms()
            )
        version = status.get("version")
        if not isinstance(version, dict):
            return cp.FailResult(
                "minecraft slp: response missing version", latency_ms=ms()
            )
        name = version.get("name", "")
        protocol = version.get("protocol", 0)
        detail = f"probe-mc-slp checked {host}:{port} version={name} protocol={protocol}"
        players = status.get("players")
        if isinstance(players, dict):
            detail += f" players={players.get('online', 0)}/{players.get('max', 0)}"
        return cp.PassResult(status=200, latency_ms=ms(), detail=detail)


def _varint(value: int) -> bytes:
    out = bytearray()
    v = value & 0xFFFFFFFF
    while True:
        if v & ~0x7F == 0:
            out.append(v)
            return bytes(out)
        out.append((v & 0x7F) | 0x80)
        v >>= 7


def _frame(packet_id: int, payload: bytes) -> bytes:
    data = bytes([packet_id]) + payload
    return _varint(len(data)) + data


def _handshake(host: str, port: int) -> bytes:
    host_bytes = host.encode("utf-8")
    payload = (
        _varint(DECLARED_PROTOCOL)
        + _varint(len(host_bytes))
        + host_bytes
        + port.to_bytes(2, "big")
        + _varint(1)  # nextState: 1 = status
    )
    return _frame(0x00, payload)


def _status_request() -> bytes:
    return _frame(0x00, b"")


def _read_exact(sock: socket.socket, count: int, deadline: float, call: cp.CheckInput) -> bytes:
    chunks = bytearray()
    while len(chunks) < count:
        if not call.context.is_active():
            raise SLPCancelled()
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise SLPError("timed out waiting for status response")
        sock.settimeout(remaining)
        chunk = sock.recv(min(4096, count - len(chunks)))
        if not chunk:
            raise SLPError("connection closed before status response completed")
        chunks.extend(chunk)
    return bytes(chunks)


def _read_varint(sock: socket.socket, deadline: float, call: cp.CheckInput) -> int:
    result = 0
    for i in range(5):
        byte = _read_exact(sock, 1, deadline, call)[0]
        result |= (byte & 0x7F) << (7 * i)
        if byte & 0x80 == 0:
            return result
    raise SLPError("varint too long")


def _read_status(sock: socket.socket, deadline: float, call: cp.CheckInput) -> bytes:
    length = _read_varint(sock, deadline, call)
    if length <= 0 or length > MAX_STATUS_BYTES:
        raise SLPError(f"invalid frame length {length}")
    payload = _read_exact(sock, length, deadline, call)
    cursor = 0

    def take_varint() -> int:
        nonlocal cursor
        result = 0
        for i in range(5):
            if cursor >= len(payload):
                raise SLPError("varint too long")
            byte = payload[cursor]
            cursor += 1
            result |= (byte & 0x7F) << (7 * i)
            if byte & 0x80 == 0:
                return result
        raise SLPError("varint too long")

    packet_id = take_varint()
    if packet_id != 0x00:
        raise SLPError(f"unexpected packet id 0x{packet_id:x}")
    str_len = take_varint()
    if str_len > len(payload) - cursor:
        raise SLPError(f"string length {str_len} exceeds payload")
    return payload[cursor : cursor + str_len]


def main() -> None:
    cp.serve_probe_plugin(ProbeMCSLPPlugin())


if __name__ == "__main__":
    main()
