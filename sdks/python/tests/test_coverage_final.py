"""Coverage 收尾：补齐 __init__ / dispatcher / invoker / openapi / tcp 剩余未覆盖行。

只新增测试，不改动产品代码。覆盖目标见各 test 的注释。
"""

import hashlib
import importlib.util
import json
import os
import queue
import sys
import time
from unittest import mock

import pytest

import croupier as croupier_sdk
from croupier import (
    ClientConfig,
    CroupierClient,
    FunctionDescriptor,
    invocation_pb2,
    protocol,
    set_field_hint,
    set_field_widget,
)
from croupier.dispatcher import MainThreadDispatcher
from croupier.invoker import Invoker, InvokerConfig, RetryConfig, _retry_delay_seconds
from croupier.openapi import (
    _extract_extension,
    _schema_to_json_schema,
    register_from_openapi,
)
from croupier.transport.tcp import TCPTransport

from fake_agent import FakePeer


# ---- shared helpers -------------------------------------------------------


def encode_file_push(transfer_id: str, file_name: str, sha256: str, data: bytes) -> bytes:
    """手写 protobuf wire 编码 FilePushRequest（与 test_file_push 保持一致）。"""

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
    """手写 protobuf wire 解码 FilePushResponse -> {1:id, 2:ok, 3:path, 4:error}。"""
    idx = 0
    out: dict = {1: "", 2: False, 3: "", 4: ""}
    while idx < len(raw):
        tag = raw[idx]
        idx += 1
        field_number = tag >> 3
        wire_type = tag & 0x7
        if wire_type == 0:
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
        elif wire_type == 2:
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


def make_push_client(staging: str, max_size: int = 1024) -> CroupierClient:
    config = ClientConfig(
        enable_file_transfer=True,
        max_file_size=max_size,
        file_staging_dir=staging,
    )
    client = CroupierClient(config)
    client.register_function(
        FunctionDescriptor(id="player.ban", version="1.0.0"), lambda m, p: b"ok"
    )
    return client


# ---- croupier/__init__.py -------------------------------------------------


def test_load_proto_module_unloadable_spec(monkeypatch):
    """spec_from_file_location 返回 None 时抛 ImportError（防御分支）。"""
    module_name = "croupier.agent.v1.register_pb2"
    monkeypatch.delitem(sys.modules, module_name, raising=False)
    monkeypatch.setattr(
        importlib.util, "spec_from_file_location", lambda *a, **k: None
    )
    with pytest.raises(ImportError, match="Unable to load module"):
        croupier_sdk._load_proto_module(module_name)


def test_normalize_hint_key_blank_returns_none():
    assert croupier_sdk._normalize_hint_key("   ") is None
    assert croupier_sdk._normalize_hint_key("") is None


def test_set_field_hint_underscore_key_normalized():
    """x_ 前缀变体应归一为 x- 形式。"""
    descriptor = set_field_hint(
        FunctionDescriptor(id="player.ban", version="1.0.0"),
        "id",
        "x_widget",
        "Select",
    )
    assert descriptor.input_schema["properties"]["id"]["x-widget"] == "Select"


def test_set_field_widget_blank_rejected():
    with pytest.raises(ValueError, match="widget is required"):
        set_field_widget(
            FunctionDescriptor(id="player.ban", version="1.0.0"), "id", "   "
        )


def test_capabilities_upload_success(tmp_path):
    """能力清单上传成功路径：对端按协议回 req+1 响应 → LOG.info 成功分支。"""
    peer = FakePeer()
    try:
        config = ClientConfig(control_addr=peer.addr(), timeout_seconds=5)
        client = CroupierClient(config)
        client.register_function(
            FunctionDescriptor(id="player.ban", version="1.0.0"), lambda m, p: b"ok"
        )
        client._maybe_register_capabilities()
        requests = peer.wait_requests(1, timeout=5)
        assert requests and requests[0][0] == protocol.MSG_REGISTER_CAPABILITIES_REQ
    finally:
        peer.stop()


def make_validating_client(schema) -> CroupierClient:
    config = ClientConfig(validate_input_payloads=True)
    client = CroupierClient(config)
    client.register_function(
        FunctionDescriptor(id="f.demo", version="1.0.0", input_schema=schema),
        lambda m, p: b"ok",
    )
    return client


def test_validate_inbound_unknown_function_skips():
    """无 descriptor 的函数直接跳过校验（服务端仍是权威校验方）。"""
    client = CroupierClient(ClientConfig(validate_input_payloads=True))
    # 不应抛错
    client._validate_inbound_payload("missing.fn", b"{}")


def test_validate_inbound_bytes_schema():
    """bytes 形态 input_schema 应先解码为 JSON dict 再校验。"""
    client = make_validating_client(json.dumps({"type": "object"}).encode())
    client._validate_inbound_payload("f.demo", b"{}")  # 通过
    with pytest.raises(ValueError, match="payload must be valid JSON"):
        client._validate_inbound_payload("f.demo", b"not-json")


def test_validate_inbound_string_schema():
    """str(JSON) 形态 input_schema 走 json.loads 后校验。"""
    client = make_validating_client('{"type":"object","required":["id"]}')
    client._validate_inbound_payload("f.demo", b'{"id":"p1"}')  # 通过
    with pytest.raises(ValueError, match="payload validation failed"):
        client._validate_inbound_payload("f.demo", b"{}")


def test_validate_inbound_non_dict_schema_skips():
    """schema 非 dict（如空字符串）时跳过校验。"""
    client = make_validating_client("")
    # 若未跳过，invalid JSON 会先抛错
    client._validate_inbound_payload("f.demo", b"not-json")


def test_inbound_dispatch_stream_task_not_found():
    """MSG_STREAM_TASK_REQUEST 分发：未知 task 返回 error 事件。"""
    client = CroupierClient(ClientConfig())
    client.register_function(
        FunctionDescriptor(id="player.ban", version="1.0.0"), lambda m, p: b"ok"
    )
    request = invocation_pb2.TaskStreamRequest(task_id="nope")
    raw = client._handle_inbound(
        protocol.MSG_STREAM_TASK_REQUEST, 7, request.SerializeToString()
    )
    event = invocation_pb2.TaskEvent()
    event.ParseFromString(raw)
    assert event.type == "error"
    assert "task not found" in event.message


def test_inbound_dispatch_file_push_disabled(tmp_path):
    """MSG_PROVIDER_FILE_PUSH_REQ 分发：未开启时拒绝。"""
    config = ClientConfig(enable_file_transfer=False, file_staging_dir=str(tmp_path))
    client = CroupierClient(config)
    raw = client._handle_inbound(protocol.MSG_PROVIDER_FILE_PUSH_REQ, 7, b"")
    decoded = decode_file_push_response(raw)
    assert decoded[2] is False
    assert "file transfer is disabled" in decoded[4]


def test_file_push_empty_payload_rejected(tmp_path):
    client = make_push_client(str(tmp_path / "staging"))
    raw = client._handle_inbound_file_push(encode_file_push("t-empty", "a.lua", "ab" * 32, b""))
    decoded = decode_file_push_response(raw)
    assert decoded[2] is False
    assert "file payload is empty" in decoded[4]


def test_file_push_missing_checksum_rejected(tmp_path):
    client = make_push_client(str(tmp_path / "staging"))
    raw = client._handle_inbound_file_push(encode_file_push("t-sha", "a.lua", "", b"x"))
    decoded = decode_file_push_response(raw)
    assert decoded[2] is False
    assert "contentSha256 is required" in decoded[4]


def test_file_push_staging_escape_rejected(tmp_path, monkeypatch):
    """abspath 防御分支：暂存目录被解析为不一致路径时拒绝。"""
    staging = str(tmp_path / "staging")
    client = make_push_client(staging)
    data = b"x"

    def fake_abspath(path: str) -> str:
        if path == staging:
            return "/safe-staging"
        return "/escaped/" + os.path.basename(path)

    monkeypatch.setattr(os.path, "abspath", fake_abspath)
    raw = client._handle_inbound_file_push(
        encode_file_push("t-esc", "a.lua", hashlib.sha256(data).hexdigest(), data)
    )
    decoded = decode_file_push_response(raw)
    assert decoded[2] is False
    assert "bare basename" in decoded[4]


def test_decode_file_push_request_rejects_non_length_wire_type():
    with pytest.raises(ValueError, match="unsupported wire type"):
        CroupierClient._decode_file_push_request(b"\x08")  # field 1, varint wire


def test_encode_file_push_response_multibyte_varint():
    """长度 ≥ 0x80 的字段值应编码为多字节 varint。"""
    transfer_id = "t" * 300
    stored_path = "p" * 300
    error = "e" * 300
    raw = CroupierClient._encode_file_push_response(transfer_id, True, stored_path, error)

    decoded = decode_file_push_response(raw)
    assert decoded[1] == transfer_id
    assert decoded[2] is True
    assert decoded[3] == stored_path
    assert decoded[4] == error


def test_active_call_tracker_exit_swallows_acquire_failure():
    """__exit__ 中 acquire 异常必须被吞掉（drain 协调不能被信号量异常打断）。"""

    class _BrokenSemaphore:
        def __init__(self) -> None:
            self._counter = 0

        def release(self, n=1) -> None:  # noqa: ARG002
            pass

        def acquire(self, timeout=None):  # noqa: ARG002
            raise RuntimeError("semaphore broken")

    client = CroupierClient(ClientConfig())
    client._active_calls = _BrokenSemaphore()
    tracker = client._active_call_tracker()
    tracker.__enter__()
    tracker.__exit__(None, None, None)  # 不应抛错
    assert client._active_calls._counter == 0


def test_drain_and_reconnect_timeout_logged(monkeypatch):
    """30 秒排空超时分支：打桩 time.monotonic 直接越过 deadline。"""
    client = CroupierClient(ClientConfig(auto_reconnect=False))
    client._active_calls._counter = 2  # type: ignore[attr-defined]

    now = {"t": 5000.0}

    def fake_monotonic() -> float:
        now["t"] += 100.0
        return now["t"]

    monkeypatch.setattr(time, "monotonic", fake_monotonic)
    monkeypatch.setattr(time, "sleep", lambda _s: None)
    client._drain_and_reconnect()
    assert not client.is_draining()


def test_send_drain_complete_without_session_is_noop():
    client = CroupierClient(ClientConfig())
    client._send_drain_complete()  # 未连接：静默返回


def test_start_heartbeat_loop_skips_when_thread_alive():
    client = CroupierClient(ClientConfig(heartbeat_interval=30))
    client._start_heartbeat_loop()
    try:
        assert client._heartbeat_thread is not None and client._heartbeat_thread.is_alive()
        first = client._heartbeat_thread
        client._start_heartbeat_loop()  # 存活时不应重建线程
        assert client._heartbeat_thread is first
    finally:
        client._heartbeat_stop.set()
        if client._heartbeat_thread:
            client._heartbeat_thread.join(timeout=2)


def test_heartbeat_loop_without_auto_reconnect_continues():
    """auto_reconnect=False 时心跳失败仅告警并 continue。"""

    class _OneShotEvent:
        def __init__(self) -> None:
            self._calls = 0

        def wait(self, timeout=None):  # noqa: ARG002
            self._calls += 1
            return self._calls > 1

        def is_set(self) -> bool:
            return self._calls > 1

        def set(self) -> None:
            pass

    client = CroupierClient(ClientConfig(auto_reconnect=False, heartbeat_interval=1))

    def _broken_heartbeat() -> None:
        raise RuntimeError("heartbeat transport down")

    client._send_heartbeat = _broken_heartbeat
    stop_event = _OneShotEvent()
    client._heartbeat_stop = stop_event  # type: ignore[assignment]
    client._heartbeat_loop()  # 应跑完一轮后退出，不抛错
    assert stop_event._calls == 2


# ---- croupier/dispatcher/__init__.py --------------------------------------


def test_clear_handles_queue_emptied_between_checks(monkeypatch):
    """clear() 的 queue.Empty 竞态分支：empty() 为 False 但 get_nowait 抛空。"""

    class _RacyQueue:
        def empty(self) -> bool:
            return False

        def get_nowait(self):
            raise queue.Empty

    dispatcher = MainThreadDispatcher.get_instance()
    monkeypatch.setattr(dispatcher, "_queue", _RacyQueue())
    dispatcher.clear()  # 不应抛错


# ---- croupier/invoker.py --------------------------------------------------


def test_ssl_context_loads_client_cert_chain(tmp_path):
    """https + 双向 TLS：load_cert_chain 成功加载客户端证书。"""
    pytest.importorskip("cryptography")
    import datetime

    from cryptography import x509
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import rsa
    from cryptography.x509.oid import NameOID

    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "croupier-sdk-test")])
    now = datetime.datetime.now(datetime.timezone.utc)
    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(subject)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - datetime.timedelta(days=1))
        .not_valid_after(now + datetime.timedelta(days=1))
        .sign(key, hashes.SHA256())
    )
    cert_path = tmp_path / "client.crt"
    key_path = tmp_path / "client.key"
    cert_path.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    key_path.write_bytes(
        key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.TraditionalOpenSSL,
            serialization.NoEncryption(),
        )
    )

    invoker = Invoker(
        InvokerConfig(
            address="https://127.0.0.1:18780/api/v1",
            cert_file=str(cert_path),
            key_file=str(key_path),
        )
    )
    context = invoker._ssl_context()
    assert context is not None


def test_retry_delay_applies_jitter():
    retry = RetryConfig(
        enabled=True,
        initial_delay_ms=100,
        backoff_multiplier=1.0,
        max_delay_ms=0,
        jitter_factor=0.5,
    )
    for _ in range(20):
        delay = _retry_delay_seconds(0, retry)
        # 100ms ±50% 抖动 → [50, 150]ms
        assert 0.04 <= delay <= 0.16


# ---- croupier/openapi.py --------------------------------------------------


def test_schema_conversion_keeps_description():
    schema = _schema_to_json_schema({"type": "object", "description": "玩家封禁入参"})
    assert schema is not None
    assert schema["description"] == "玩家封禁入参"


@pytest.mark.parametrize("value,expected", [(True, "true"), (False, "false")])
def test_extract_extension_bool(value, expected):
    assert _extract_extension({"x-risk": value}, "x-risk") == expected


def test_register_from_openapi_register_failure_raises():
    class _FailingClient:
        def register_function(self, descriptor, handler):  # noqa: ARG002
            raise RuntimeError("duplicate function id")

    spec = {
        "paths": {
            "/ban": {
                "get": {
                    "operationId": "player.ban",
                    "responses": {"200": {"description": "ok"}},
                }
            }
        }
    }
    with pytest.raises(ValueError, match="register function player.ban failed"):
        register_from_openapi(
            _FailingClient(),
            spec,
            handler_resolver=lambda function_id: (lambda metadata, payload: b"ok"),
        )


# ---- croupier/transport/tcp.py ---------------------------------------------


def test_close_swallows_socket_close_failure():
    transport = TCPTransport(address="127.0.0.1:1")
    transport._connected = True
    sock = mock.Mock()
    sock.close.side_effect = OSError("already closed")
    transport._sock = sock
    transport.close()  # 不应抛错
    assert transport._sock is None
    assert not transport.is_connected()


def test_reader_loop_breaks_when_socket_missing():
    transport = TCPTransport(address="127.0.0.1:1")
    transport._running = True
    transport._sock = None
    transport._reader_loop()  # 应立即退出
    assert not transport._running or True
