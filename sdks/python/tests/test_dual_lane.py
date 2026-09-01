"""双车道派发（控制优先级）：业务洪峰下心跳依然可达。

回归背景：业务/控制共队列时，业务 handler 洪峰打满队列 → 心跳被
fail-fast 拒绝 → Agent 判定会话死亡 → 过载升级为连接雪崩。
双车道后控制消息（心跳/注册/drain）走独立车道，永不拒绝。
"""

import threading
import time

import pytest

from croupier import protocol
from croupier.transport.tcp import TCPTransport

from fake_agent import FakeAgent


def test_is_control_request_classification():
    for msg_id in (
        protocol.MSG_PROVIDER_HEARTBEAT_REQUEST,
        protocol.MSG_PROVIDER_CONNECT_REQUEST,
        protocol.MSG_PROVIDER_DRAIN_REQUEST,
        protocol.MSG_HEARTBEAT_REQUEST,
        protocol.MSG_REGISTER_REQUEST,
        protocol.MSG_REGISTER_CAPABILITIES_REQ,
    ):
        assert protocol.is_control_request(msg_id), hex(msg_id)
    for msg_id in (
        protocol.MSG_INVOKE_REQUEST,
        protocol.MSG_START_TASK_REQUEST,
        protocol.MSG_CANCEL_TASK_REQUEST,
        protocol.MSG_STREAM_TASK_REQUEST,
        protocol.MSG_INVOKE_RESPONSE,
    ):
        assert not protocol.is_control_request(msg_id), hex(msg_id)


def test_control_lane_survives_business_saturation():
    """业务队列被慢 handler 打满后，心跳仍必须在时限内得到响应。"""
    agent = FakeAgent()
    try:
        t = TCPTransport(agent.addr(), inbound_workers=1)
        slow_entered = threading.Event()

        def handler(msg_id: int, req_id: int, body: bytes) -> bytes:
            if msg_id == protocol.MSG_PROVIDER_HEARTBEAT_REQUEST:
                return b"pong"
            # 业务消息：占住唯一业务 worker（未完成前队列随即打满）
            slow_entered.set()
            time.sleep(3.0)
            return b""

        t.set_handler(handler)
        t.connect()
        try:
            # 打满业务车道：worker 1 个 + 队列上限 4 → 连发 6 个业务请求
            for req_id in range(101, 107):
                agent.push(protocol.MSG_INVOKE_REQUEST, req_id, b"")
            assert slow_entered.wait(2.0), "business handler should have started"

            # 洪峰中的心跳：双车道下应立即被控制车道处理
            start = time.monotonic()
            agent.push(protocol.MSG_PROVIDER_HEARTBEAT_REQUEST, 900, b"")
            resp = agent.wait_response(900, timeout=2.0)
            elapsed = time.monotonic() - start
            assert resp is not None, "heartbeat response must arrive during business saturation"
            assert resp[0] == protocol.MSG_PROVIDER_HEARTBEAT_RESPONSE
            assert resp[1] == b"pong"
            assert elapsed < 2.0, f"control lane too slow under load: {elapsed:.2f}s"

            # 对照：业务队列满 → 新业务请求被 fail-fast 回空响应
            agent.push(protocol.MSG_INVOKE_REQUEST, 901, b"")
            resp2 = agent.wait_response(901, timeout=2.0)
            assert resp2 is not None, "saturated business lane must fast-fail with a frame"
            assert resp2[1] == b""
        finally:
            t.close()
    finally:
        agent.stop()
