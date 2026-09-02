"""F：控制面 manifest 上传——_maybe_register_capabilities 行为验证。"""

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
    """读一帧请求并回确认帧（4 字节长度 + 8 字节头 + body）。"""
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
        # 8 字节头：version(1) + msg_id(3) + request_id(4)
        assert frame[0] == 1
        msg_id = int.from_bytes(frame[1:4], "big")
        assert msg_id == MSG_REGISTER_CAPABILITIES_REQ
        body = frame[8:]
        # field 1 provider（tag 0x0A + len + bytes）在前，跳过
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
        # field 2 manifest_json_gz（tag 0x12 + varint 长度 + gz bytes）
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
        # 回空响应帧
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
    # 不应抛错
    client._maybe_register_capabilities()
