"""F：文件下发接收——Python wire 编解码与安全链测试。"""

import hashlib
import json
import os

import pytest

from croupier import CroupierClient, ClientConfig, FunctionDescriptor
from croupier.protocol import MSG_PROVIDER_FILE_PUSH_REQ


def encode_file_push(transfer_id: str, file_name: str, sha256: str, data: bytes) -> bytes:
    """手写 protobuf wire 编码 FilePushRequest（与 Go/JS 一致）。"""

    def varint(value: int) -> bytes:
        out = b""
        while value >= 0x80:
            out += bytes([(value & 0x7F) | 0x80])
            value >>= 7
        return out + bytes([value])

    def field(field_number: int, value: bytes) -> bytes:
        return bytes([(field_number << 3) | 2]) + varint(len(value)) + value

    out = b""
    if transfer_id:
        out += field(1, transfer_id.encode())
    if file_name:
        out += field(2, file_name.encode())
    if sha256:
        out += field(3, sha256.encode())
    if data:
        out += field(4, data)
    return out


def decode_file_push_response(raw: bytes) -> dict:
    """手写 protobuf wire 解码 FilePushResponse。"""
    idx = 0
    out: dict = {1: "", 2: False, 3: "", 4: ""}
    while idx < len(raw):
        tag = raw[idx]
        idx += 1
        field_number = tag >> 3
        wire_type = tag & 0x7
        if wire_type == 0:  # varint (bool)
            value = 0
            shift = 0
            while True:
                byte = raw[idx]
                idx += 1
                value |= (byte & 0x7F) << shift
                if not byte & 0x80:
                    break
                shift += 7
            out[field_number] = bool(value)
        elif wire_type == 2:  # length-delimited
            length = 0
            shift = 0
            while True:
                byte = raw[idx]
                idx += 1
                length |= (byte & 0x7F) << shift
                if not byte & 0x80:
                    break
                shift += 7
            out[field_number] = raw[idx : idx + length].decode("utf-8")
            idx += length
    return out


def make_client(validate: bool, staging: str) -> CroupierClient:
    config = ClientConfig(
        enable_file_transfer=validate,
        max_file_size=1024,
        file_staging_dir=staging,
    )
    client = CroupierClient(config)
    client.register_function(
        FunctionDescriptor(id="player.ban", version="1.0.0"), lambda m, p: b"ok"
    )
    return client


def push(client: CroupierClient, body: bytes) -> dict:
    raw = client._handle_inbound_file_push(body)
    decoded = decode_file_push_response(raw)
    return {2: decoded.get(2, False), 4: decoded.get(4, ''), 3: decoded.get(3, '')}


def test_valid_payload_staged_and_ok(tmp_path):
    staging = str(tmp_path / "staging")
    client = make_client(True, staging)
    data = b"print('hotfix')"
    response = push(
        client, encode_file_push("t-1", "hotfix.lua", hashlib.sha256(data).hexdigest(), data)
    )
    assert response[2] is True
    with open(response[3], "rb") as handle:
        assert handle.read() == data


def test_disabled_flag_rejects(tmp_path):
    client = make_client(False, str(tmp_path))
    data = b"x"
    response = push(
        client, encode_file_push("t-2", "a.lua", hashlib.sha256(data).hexdigest(), data)
    )
    assert response[2] is False
    assert "file transfer is disabled" in response[4]


def test_checksum_mismatch_rejects(tmp_path):
    client = make_client(True, str(tmp_path))
    data = b"x"
    response = push(
        client, encode_file_push("t-3", "a.lua", "de" * 32, data)
    )
    assert response[2] is False
    assert "checksum mismatch" in response[4]


@pytest.mark.parametrize("evil", ["../evil.lua", "sub/dir/evil.lua", "/etc/evil.lua"])
def test_path_traversal_rejects(tmp_path, evil):
    client = make_client(True, str(tmp_path))
    data = b"x"
    response = push(
        client, encode_file_push("t-4", evil, hashlib.sha256(data).hexdigest(), data)
    )
    assert response[2] is False
    assert "bare basename" in response[4]
    assert not os.path.exists(os.path.join(tmp_path, "evil.lua"))


def test_oversize_rejects(tmp_path):
    client = make_client(True, str(tmp_path))
    data = b"x" * 2048
    response = push(
        client, encode_file_push("t-5", "big.lua", hashlib.sha256(data).hexdigest(), data)
    )
    assert response[2] is False
    assert "exceeds max" in response[4]


def test_empty_transfer_id_rejects(tmp_path):
    client = make_client(True, str(tmp_path))
    data = b"x"
    response = push(
        client, encode_file_push("", "a.lua", hashlib.sha256(data).hexdigest(), data)
    )
    assert response[2] is False
    assert "transferId is required" in response[4]
