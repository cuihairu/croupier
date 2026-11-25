"""F: control-plane manifest upload — behavior of _maybe_register_capabilities."""

import gzip
import json
import socket
import struct
import threading
from unittest import mock  # noqa: F401

import pytest

from croupier import ClientConfig, CroupierClient, FunctionDescriptor
from croupier.protocol import MSG_REGISTER_CAPABILITIES_REQ


def server_socket():
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.bind(("127.0.0.1", 0))
    server.listen(1)
    return server


def _serve_once(server_socket_, captured: dict) -> None:
    """Read one request frame and reply with a confirmation frame
    (4-byte length + 8-byte header + body)."""
    conn, _ = server_socket_.accept()
    try:
        header = conn.recv(4)
        (length,) = struct.unpack(">I", header)
        frame = b""
        while len(frame) < length:
            chunk = conn.recv(length - len(frame))
            if not chunk:
                break
            frame += chunk
        # 8-byte header: version(1) + msg_id(3) + request_id(4)
        assert frame[0] == 1
        msg_id = int.from_bytes(frame[1:4], "big")
        assert msg_id == MSG_REGISTER_CAPABILITIES_REQ
        body = frame[8:]
        # field 1 provider (tag 0x0A + length + bytes) comes first; skip it
        assert body[0] == 0x0A
        idx = 1
        provider_len = 0
        shift = 0
        while True:
            byte = body[idx]
            provider_len |= (byte & 0x7F) << shift
            idx += 1
            if not byte & 0x80:
                break
            shift += 7
        idx += provider_len
        # field 2 manifest_json_gz (tag 0x12 + varint length + gz bytes)
        assert body[idx] == 0x12
        idx += 1
        manifest_len = 0
        shift = 0
        while True:
            byte = body[idx]
            manifest_len |= (byte & 0x7F) << shift
            idx += 1
            if not byte & 0x80:
                break
            shift += 7
        manifest = json.loads(gzip.decompress(body[idx : idx + manifest_len]))
        captured.update(manifest)
        # reply with an empty response frame
        resp_frame = bytes([1]) + msg_id.to_bytes(3, "big") + (0).to_bytes(4, "big")
        conn.sendall(struct.pack(">I", len(resp_frame)) + resp_frame)
    finally:
        conn.close()


def test_manifest_uploaded_to_control_plane():
    captured: dict = {}
    server = server_socket()
    threading.Thread(target=_serve_once, args=(server, captured), daemon=True).start()

    host, port = server.getsockname()
    config = ClientConfig(control_addr=f"{host}:{port}")
    client = CroupierClient(config)
    descriptor = FunctionDescriptor(
        id="player.ban",
        version="1.0.0",
        input_schema={"type": "object", "properties": {"id": {"type": "string"}}},
    )
    client.register_function(descriptor, lambda metadata, payload: b"ok")
    client._maybe_register_capabilities()

    assert "provider" in captured
    assert captured["provider"]["id"] == config.service_id
    assert any(fn["id"] == "player.ban" for fn in captured.get("functions", []))
    server.close()


def test_no_control_addr_is_noop():
    client = CroupierClient(ClientConfig())
    client.register_function(FunctionDescriptor(id="f"), lambda m, p: b"ok")
    # must not raise
    client._maybe_register_capabilities()


def test_inbound_workers_serial_mode_exposed_on_transport():
    """Single-threaded game-server compatibility: inbound_workers=1 must pass
    through to TCPTransport (a serialized lane)."""
    from unittest import mock

    from croupier import ClientConfig, CroupierClient

    config = ClientConfig()
    config.inbound_workers = 1
    client = CroupierClient(config)
    from croupier import FunctionDescriptor
    client.register_function(FunctionDescriptor(id="demo.fn", version="1.0.0"),
                             lambda ctx, payload: "{}")

    captured = {}

    real_tcp = __import__("croupier.transport.tcp", fromlist=["TCPTransport"]).TCPTransport

    def spy(**kwargs):
        captured["inbound_workers"] = kwargs.get("inbound_workers")
        return mock.MagicMock(spec=real_tcp)

    with mock.patch("croupier.TCPTransport", side_effect=spy):
        try:
            client.connect()
        except Exception:
            pass  # connection failure does not affect the pass-through assertion

    assert captured.get("inbound_workers") == 1
