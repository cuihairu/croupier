"""
Tests for CroupierClient's TCP session lifecycle and inbound dispatch,
using an in-process fake agent (see fake_agent.py).
"""

import importlib
import json
import sys
import threading
import time
from unittest import mock

import pytest
from fake_agent import FakeAgent

import croupier
from croupier import protocol


def _make_client(agent: FakeAgent, **overrides) -> croupier.CroupierClient:
    defaults = {
        "agent_addr": agent.addr(),
        "service_id": "python-test-svc",
        "service_version": "1.0.0",
        "heartbeat_interval": 30,
        "timeout_seconds": 3,
        "reconnect_interval": 0.2,
    }
    defaults.update(overrides)
    client = croupier.CroupierClient(croupier.ClientConfig(**defaults))
    client.register_function(
        croupier.FunctionDescriptor(id="echo", version="1.0.0"),
        lambda ctx, payload: "echo:" + payload.decode("utf-8"),
    )
    return client


@pytest.fixture
def agent():
    fake = FakeAgent(session_id="sess-abc-123")
    yield fake
    fake.stop()


class TestConnectLifecycle:
    """connect() handshake, idempotency, heartbeats and disconnect()."""

    def test_connect_handshake_heartbeat_and_disconnect(self, agent):
        agent.session_id = "sess-abc-123"
        client = _make_client(agent, heartbeat_interval=1)
        old_session = agent.session_id

        client.connect()

        assert client._connected is True
        assert client._session_id == old_session

        connects = agent.wait_connects(1)
        assert connects[0].service_id == "python-test-svc"
        assert [f.id for f in connects[0].functions] == ["echo"]
        assert connects[0].sdk_language == "python"

        # Second connect() is a no-op.
        client.connect()
        assert len(agent.wait_connects(1)) == 1

        # Heartbeat loop fires within ~1s and carries the session id.
        heartbeats = agent.wait_heartbeats(1, timeout=4)
        assert heartbeats[0].session_id == old_session
        assert heartbeats[0].service_id == "python-test-svc"

        client.disconnect()

        assert client._connected is False
        assert client._session_id == ""
        assert client._heartbeat_thread is None
        assert client._transport is None

    def test_connect_rejects_empty_session_id(self):
        empty_agent = FakeAgent(session_id="")
        try:
            client = _make_client(empty_agent)

            with pytest.raises(RuntimeError, match="empty session_id"):
                client.connect()

            assert client._transport is None
            assert client._connected is False
        finally:
            empty_agent.stop()

    def test_reconnect_closes_previous_transport(self, agent):
        client = _make_client(agent)
        client.connect()
        first_transport = client._transport
        assert first_transport is not None

        client._connect_and_register()  # direct re-dial

        assert client._transport is not first_transport
        assert first_transport.is_connected() is False
        assert len(agent.wait_connects(2)) == 2
        assert client._session_id == "sess-abc-123"

        client.disconnect()

    def test_send_heartbeat_raises_when_not_registered(self):
        client = croupier.CroupierClient()

        with pytest.raises(RuntimeError, match="not registered"):
            client._send_heartbeat()

    def test_heartbeat_failure_triggers_reconnect(self, agent):
        client = _make_client(agent, heartbeat_interval=1)
        client.connect()
        assert len(agent.wait_connects(1)) == 1

        # Simulate agent restart: drop connections, keep the listener open.
        agent.close_connections()

        # The heartbeat call fails, the client recovers and re-registers.
        assert len(agent.wait_connects(2, timeout=10)) == 2
        assert client._session_id == "sess-abc-123"

        client.disconnect()

    def test_recover_connection_stops_after_disconnect(self):
        from fake_agent import free_port

        client = croupier.CroupierClient(
            croupier.ClientConfig(
                agent_addr=f"127.0.0.1:{free_port()}",  # nothing listening
                reconnect_interval=0.1,
            )
        )
        client.register_function(
            croupier.FunctionDescriptor(id="echo", version="1.0.0"),
            lambda ctx, payload: "ok",
        )

        with mock.patch.object(
            client, "_connect_and_register", side_effect=OSError("dial failed")
        ):
            thread = threading.Thread(target=client._recover_connection, daemon=True)
            thread.start()
            time.sleep(0.3)
            client._heartbeat_stop.set()
            thread.join(timeout=3)

        assert not thread.is_alive()


class TestInboundDispatch:
    """Agent-pushed invokes / tasks routed through _handle_inbound."""

    def test_inbound_invoke_roundtrip(self, agent):
        client = _make_client(agent)
        client.connect()

        seen: dict[str, object] = {}

        def capturing_handler(ctx, payload):
            seen["ctx"] = ctx
            seen["payload"] = payload
            return "echo:" + payload.decode("utf-8")

        client._handlers["echo"] = capturing_handler

        req = croupier.invocation_pb2.InvokeRequest(
            function_id="echo", payload=b"hello", metadata={"k": "v"}
        )
        agent.push(
            protocol.MSG_INVOKE_REQUEST, 201, req.SerializeToString()
        )
        response = agent.wait_response(201, timeout=3)

        assert response is not None
        resp_msg_id, resp_body = response
        assert resp_msg_id == protocol.MSG_INVOKE_RESPONSE
        parsed = croupier.invocation_pb2.InvokeResponse()
        parsed.ParseFromString(resp_body)
        assert parsed.payload == b"echo:hello"
        assert json.loads(str(seen["ctx"])) == {"k": "v"}
        # active-call tracker returned to zero after the call finished
        assert client._active_calls._counter == 0

        client.disconnect()

    def test_inbound_start_task_and_cancel_roundtrip(self, agent):
        release = threading.Event()
        client = _make_client(agent)
        client._handlers["echo"] = lambda ctx, payload: (
            release.wait(5) and "done"
        )
        client.connect()

        start_req = croupier.invocation_pb2.InvokeRequest(
            function_id="echo", payload=b"job"
        )
        agent.push(
            protocol.MSG_START_TASK_REQUEST, 202, start_req.SerializeToString()
        )
        resp_msg_id, resp_body = agent.wait_response(202, timeout=3)

        assert resp_msg_id == protocol.MSG_START_TASK_RESPONSE
        start_resp = croupier.invocation_pb2.StartTaskResponse()
        start_resp.ParseFromString(resp_body)
        task_id = start_resp.task_id
        assert task_id.startswith("echo-")

        state = client._tasks.get(task_id)
        assert state is not None
        assert not state.done.is_set()

        cancel_req = croupier.invocation_pb2.CancelTaskRequest(task_id=task_id)
        agent.push(
            protocol.MSG_CANCEL_TASK_REQUEST, 203, cancel_req.SerializeToString()
        )
        cancel_resp = agent.wait_response(203, timeout=3)

        assert cancel_resp is not None
        # Body is an empty InvokeResponse (no CancelTaskResponse proto exists),
        # carried under the CancelTaskResponse msg id.
        assert cancel_resp[0] == protocol.MSG_CANCEL_TASK_RESPONSE
        assert cancel_resp[1] == b""
        assert state.cancelled.is_set()

        release.set()
        client.disconnect()

    def test_handle_inbound_unknown_msg_id_returns_empty(self):
        client = croupier.CroupierClient()

        assert client._handle_inbound(0xFFFFFF, 1, b"") == b""

    def test_stream_task_response_states(self):
        client = croupier.CroupierClient()

        def response_for(task_id: str) -> croupier.invocation_pb2.TaskEvent:
            req = croupier.invocation_pb2.TaskStreamRequest(task_id=task_id)
            event = croupier.invocation_pb2.TaskEvent()
            event.ParseFromString(client._handle_inbound_stream_task(req.SerializeToString()))
            return event

        # Unknown task -> error
        event = response_for("nope")
        assert event.type == "error"
        assert "task not found" in event.message

        # Running task -> progress
        running = croupier._TaskState()
        with client._task_lock:
            client._tasks["running"] = running
        event = response_for("running")
        assert event.type == "progress"
        assert event.message == "task is running"

        # Cancelled task -> error
        cancelled = croupier._TaskState()
        cancelled.cancelled.set()
        with client._task_lock:
            client._tasks["cancelled"] = cancelled
        event = response_for("cancelled")
        assert event.type == "error"
        assert event.message == "task was cancelled"

        # Done task with events -> last event is echoed back
        done = croupier._TaskState()
        done.push(
            croupier.invocation_pb2.TaskEvent(
                type="completed", message="finished", progress=100, payload=b"res"
            ),
            finished=True,
        )
        with client._task_lock:
            client._tasks["done"] = done
        event = response_for("done")
        assert event.type == "completed"
        assert event.progress == 100
        assert event.payload == b"res"
        assert "done" not in client._tasks  # drained and removed

        # Done task whose queue only holds the closing sentinel -> error
        empty = croupier._TaskState()
        empty.queue.put(None)
        empty.done.set()
        with client._task_lock:
            client._tasks["empty"] = empty
        event = response_for("empty")
        assert event.type == "error"
        assert event.message == "task completed with no events"


class TestDrain:
    """ProviderDrainRequest handling and post-drain reconnect."""

    def test_drain_full_cycle_replies_and_reconnects(self, agent):
        client = _make_client(agent)
        client.connect()
        assert len(agent.wait_connects(1)) == 1

        agent.push(protocol.MSG_PROVIDER_DRAIN_REQUEST, 300, b"")

        # Immediate empty ProviderDrainResponse...
        response = agent.wait_response(300, timeout=5)
        assert response is not None
        assert response[0] == protocol.MSG_PROVIDER_DRAIN_RESPONSE
        assert response[1] == b""

        # ...followed by DrainComplete and an automatic re-register.
        assert agent.wait_drain_completes(1, timeout=5) == 1
        assert len(agent.wait_connects(2, timeout=5)) == 2
        assert client.is_draining() is False
        assert client._session_id == "sess-abc-123"

        client.disconnect()

    def test_drain_waits_for_in_flight_call(self, agent):
        client = _make_client(agent)
        client.connect()

        with client._active_call_tracker():
            assert client._active_calls._counter == 1

            client._handle_drain_request(b"")
            time.sleep(0.3)
            # Drain must still be waiting for the in-flight call.
            assert client.is_draining() is True
            assert agent.wait_drain_completes(1, timeout=0.1) == 0

        # Released: drain completes and the client re-registers.
        assert agent.wait_drain_completes(1, timeout=5) == 1
        assert len(agent.wait_connects(2, timeout=5)) == 2

        deadline = time.monotonic() + 5
        while client.is_draining() and time.monotonic() < deadline:
            time.sleep(0.02)
        assert client.is_draining() is False

        client.disconnect()

    def test_drain_request_while_already_draining_returns_empty(self):
        client = croupier.CroupierClient()
        client._draining.set()

        assert client._handle_drain_request(b"") == b""
        assert client.is_draining() is True

    def test_drain_send_and_reconnect_failures_are_swallowed(self):
        client = croupier.CroupierClient(
            croupier.ClientConfig(auto_reconnect=True)
        )

        with mock.patch.object(
            client, "_send_drain_complete", side_effect=OSError("send failed")
        ), mock.patch.object(
            client, "_recover_connection", side_effect=RuntimeError("recover failed")
        ):
            client._drain_and_reconnect()  # must not raise

        assert client.is_draining() is False

    def test_send_drain_complete_without_transport_is_noop(self):
        client = croupier.CroupierClient()

        client._send_drain_complete()  # no transport/session -> returns silently


class TestTaskExecutionEdges:
    """Branches of the background task runner in start_task()."""

    def test_push_after_done_is_ignored(self):
        state = croupier._TaskState()
        state.push(
            croupier.invocation_pb2.TaskEvent(type="completed"), finished=True
        )

        state.push(croupier.invocation_pb2.TaskEvent(type="error"))

        assert state.queue.qsize() == 2  # completed event + closing sentinel only

    def test_task_result_discarded_when_cancelled_after_handler_returns(self):
        client = croupier.CroupierClient()
        handler_started = threading.Event()

        def slow_handler(ctx, payload):
            handler_started.set()
            time.sleep(0.15)
            return "late"

        client.register_function(
            croupier.FunctionDescriptor(id="slow", version="1.0.0"), slow_handler
        )

        task_id = client.start_task("slow", b"x")
        assert handler_started.wait(timeout=2)
        client.cancel_task(task_id)

        events = list(client.stream_task(task_id))
        types = [e.type for e in events]

        assert types == ["started", "cancelled"]
        assert "completed" not in types

    def test_task_error_suppressed_when_cancelled(self):
        client = croupier.CroupierClient()
        release = threading.Event()

        def failing_handler(ctx, payload):
            release.wait(timeout=5)
            raise ValueError("boom")

        client.register_function(
            croupier.FunctionDescriptor(id="failing", version="1.0.0"), failing_handler
        )

        task_id = client.start_task("failing", b"x")
        client.cancel_task(task_id)

        with mock.patch.object(croupier, "LOG") as fake_log:
            release.set()
            time.sleep(0.2)  # let the runner hit its except branch
            assert fake_log.exception.call_count == 0

        events = list(client.stream_task(task_id))
        assert [e.type for e in events] == ["started", "cancelled"]

    def test_task_with_bytes_result(self):
        client = croupier.CroupierClient()
        client.register_function(
            croupier.FunctionDescriptor(id="bin", version="1.0.0"),
            lambda ctx, payload: bytearray(b"\x00\x01"),
        )

        task_id = client.start_task("bin", b"")

        events = list(client.stream_task(task_id))
        assert events[-1].type == "completed"
        assert events[-1].payload == b"\x00\x01"


class TestManifestOptionalFields:
    """build_manifest() with every optional descriptor field populated."""

    def test_build_manifest_includes_all_optional_fields(self):
        client = croupier.CroupierClient(
            croupier.ClientConfig(service_id="svc-x", service_version="9.9.9")
        )
        client.register_function(
            croupier.FunctionDescriptor(
                id="full.fn",
                version="2.0.0",
                tags=["ops", "beta"],
                summary="Short summary",
                description="Long description",
                operation_id="fullFn",
                deprecated=True,
                input_schema={"type": "object"},
                output_schema={"type": "string"},
                resource="player",
                operation="ban",
                capability=" moderation",
                execution="async",
                approval_required=True,
                approval_policy_key="two-person",
                risk="high",
                permission="player.ban",
            ),
            lambda ctx, payload: "ok",
        )

        manifest = json.loads(client.build_manifest().decode("utf-8"))
        fn = manifest["functions"][0]

        assert fn["tags"] == ["ops", "beta"]
        assert fn["summary"] == "Short summary"
        assert fn["description"] == "Long description"
        assert fn["operationId"] == "fullFn"
        assert fn["deprecated"] is True
        assert fn["inputSchema"] == {"type": "object"}
        assert fn["outputSchema"] == {"type": "string"}
        assert fn["resource"] == "player"
        assert fn["operation"] == "ban"
        assert fn["capability"] == " moderation"
        assert fn["execution"] == "async"
        assert fn["approvalRequired"] is True
        assert fn["approvalPolicyKey"] == "two-person"
        assert fn["risk"] == "high"
        assert fn["permission"] == "player.ban"


class TestModuleInternals:
    """Small module-level helpers and import fallbacks."""

    def test_normalize_agent_addr_strips_scheme(self):
        normalize = croupier.CroupierClient._normalize_agent_addr

        assert normalize("tcp://127.0.0.1:19091") == "127.0.0.1:19091"
        assert normalize("tls://agent.example.com:19091") == "agent.example.com:19091"
        assert normalize("127.0.0.1:19091") == "127.0.0.1:19091"

    def test_version_falls_back_to_unknown_on_metadata_error(self):
        import importlib.metadata

        try:
            with mock.patch.object(
                importlib.metadata, "version", side_effect=Exception("metadata boom")
            ):
                importlib.reload(croupier)
                assert croupier.__version__ == "unknown"
        finally:
            importlib.reload(croupier)
        assert croupier.__version__ != "unknown"

    def test_invoker_import_error_is_tolerated(self):
        saved = sys.modules.get("croupier.invoker")
        sys.modules["croupier.invoker"] = None  # blocks the submodule import
        try:
            # Reload must not raise even when the invoker module is missing.
            importlib.reload(croupier)
            # OBSERVED BEHAVIOR (recorded, not fixed): the ImportError
            # fallback leaves the previously imported Invoker classes in the
            # module namespace instead of removing them.
            assert hasattr(croupier, "Invoker")
        finally:
            if saved is not None:
                sys.modules["croupier.invoker"] = saved
            else:
                sys.modules.pop("croupier.invoker", None)
            importlib.reload(croupier)
        assert hasattr(croupier, "Invoker")

    def test_integrations_package_importable(self):
        import croupier.integrations

        assert croupier.integrations.__name__ == "croupier.integrations"
