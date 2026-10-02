"""BUG-038 回归：同步 invoke 的 handler 异常必须回错误响应帧，不吞。

修复前：`_handle_inbound_invoke` 抛出的异常被 transport `_process_inbound`
的 except 分支吞掉（只打日志不回帧），调用方阻塞到自身 deadline
（15s 超时假象）。对齐 Go SDK：invoke 错误回 `InvokeResponse{"error":...}`
payload 帧；transport 层兜底回空 body 帧（「失败也要答 Agent」）。
"""

import json
from unittest import mock

import croupier as croupier_pkg
from croupier import CroupierClient, ClientConfig, FunctionDescriptor
from croupier import protocol
from croupier.transport.tcp import TCPTransport


def make_client(handler) -> CroupierClient:
    client = CroupierClient(ClientConfig())
    descriptor = FunctionDescriptor(
        id="inventory.grant",
        version="1.0.0",
        input_schema={
            "type": "object",
            "properties": {
                "playerId": {"type": "string"},
                "templateId": {"type": "string"},
            },
            "required": ["playerId", "templateId"],
        },
    )
    client.register_function(descriptor, handler)
    return client


def invoke_request(function_id: str, payload: dict) -> bytes:
    req = croupier_pkg.invocation_pb2.InvokeRequest(
        function_id=function_id,
        payload=json.dumps(payload).encode("utf-8"),
    )
    return req.SerializeToString()


def parse_invoke_response(body: bytes):
    resp = croupier_pkg.invocation_pb2.InvokeResponse()
    resp.ParseFromString(body)
    return resp


def test_handler_exception_returns_error_payload():
    """handler 抛异常 → 错误 payload 帧秒回，不再冒泡吞掉。"""
    client = make_client(lambda metadata, payload: (_ for _ in ()).throw(ValueError("playerId and templateId are required")))
    body = client._handle_inbound_invoke(invoke_request("inventory.grant", {"playerId": "p1"}))
    resp = parse_invoke_response(body)
    error = json.loads(resp.payload.decode("utf-8"))
    assert "playerId and templateId are required" in error["error"]


def test_unregistered_function_returns_error_payload():
    """未注册函数 → 错误 payload 帧（修复前 raise 被吞 → 调用方超时）。"""
    client = make_client(lambda metadata, payload: b"ok")
    body = client._handle_inbound_invoke(invoke_request("player.ban", {"id": "p1"}))
    resp = parse_invoke_response(body)
    error = json.loads(resp.payload.decode("utf-8"))
    assert "player.ban" in error["error"]


def test_handler_success_path_unchanged():
    """正常路径不受影响：payload 原样回。"""
    client = make_client(lambda metadata, payload: json.dumps({"ok": True}).encode())
    body = client._handle_inbound_invoke(invoke_request("inventory.grant", {"playerId": "p1", "templateId": "sword_01"}))
    resp = parse_invoke_response(body)
    assert json.loads(resp.payload.decode("utf-8")) == {"ok": True}


def test_transport_fallback_always_answers():
    """transport 兜底：handler 漏网异常（如反序列化失败）也回空 body 帧。"""
    transport = TCPTransport()
    transport.set_handler(
        lambda msg_id, req_id, body: (_ for _ in ()).throw(RuntimeError("boom"))
    )
    sent = []
    with mock.patch.object(transport, "send_response", side_effect=lambda m, r, b: sent.append((m, r, b))):
        msg_id = protocol.MSG_INVOKE_REQUEST
        transport._process_inbound(msg_id, req_id=42, body=b"\xff")  # 非法 protobuf
    assert len(sent) == 1
    resp_msg_id, req_id, resp_body = sent[0]
    assert resp_msg_id == protocol.get_response_msg_id(msg_id)
    assert req_id == 42
    assert resp_body == b""


def test_transport_success_path_sends_handler_body():
    """对照：handler 正常返回时 transport 回 handler body。"""
    transport = TCPTransport()
    transport.set_handler(lambda msg_id, req_id, body: b"handled")
    sent = []
    with mock.patch.object(transport, "send_response", side_effect=lambda m, r, b: sent.append((m, r, b))):
        msg_id = protocol.MSG_INVOKE_REQUEST
        transport._process_inbound(msg_id, req_id=7, body=b"")
    assert sent == [(protocol.get_response_msg_id(msg_id), 7, b"handled")]
