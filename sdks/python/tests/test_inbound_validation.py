"""F15 验收测试：provider 侧入站 payload 校验。"""

import json
from unittest import mock

import pytest

from croupier import CroupierClient, ClientConfig, FunctionDescriptor


def make_client(validate: bool, handler=lambda metadata, payload: b"ok") -> CroupierClient:
    config = ClientConfig(validate_input_payloads=validate)
    client = CroupierClient(config)
    descriptor = FunctionDescriptor(
        id="player.ban",
        version="1.0.0",
        input_schema={
            "type": "object",
            "properties": {"id": {"type": "string"}},
            "required": ["id"],
        },
    )
    client.register_function(descriptor, handler)
    return client


def test_valid_payload_passes():
    client = make_client(True)
    assert client.invoke("player.ban", json.dumps({"id": "p1"}).encode()) == b"ok"


def test_missing_required_rejected():
    handler = mock.Mock(return_value=b"ok")
    client = make_client(True, handler)
    with pytest.raises(ValueError, match="payload validation failed"):
        client.invoke("player.ban", b"{}")
    handler.assert_not_called()


def test_type_mismatch_rejected():
    client = make_client(True)
    with pytest.raises(ValueError, match="payload validation failed"):
        client.invoke("player.ban", json.dumps({"id": 123}).encode())


def test_invalid_payload_json_rejected():
    client = make_client(True)
    with pytest.raises(ValueError, match="payload must be valid JSON"):
        client.invoke("player.ban", b"not-json")


def test_disabled_flag_skips_validation():
    handler = mock.Mock(return_value=b"ok")
    client = make_client(False, handler)
    assert client.invoke("player.ban", b"{}") == b"ok"
    handler.assert_called_once()


def test_start_task_validation():
    client = make_client(True)
    with pytest.raises(ValueError, match="payload validation failed"):
        client.start_task("player.ban", b"{}")
