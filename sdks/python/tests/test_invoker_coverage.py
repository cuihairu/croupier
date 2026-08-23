"""Coverage boost tests for croupier.invoker edge paths."""

from __future__ import annotations

import asyncio
import json
import ssl
import urllib.error
from io import BytesIO
from typing import Any, Dict, List

import pytest

from croupier.invoker import (
    Invoker,
    InvokerConfig,
    InvokeOptions,
    RetryConfig,
    ServerHTTPError,
    SyncInvoker,
    TaskEventInfo,
    _calculate_reconnect_delay,
    _endpoint_url,
    _is_retryable_error,
    _json_string,
    _normalize_server_api_url,
    _optional_int,
    _optional_string,
    _parse_json_payload,
    _reject_header_injection,
    _retry_delay_seconds,
    _server_error_message,
    _task_event_from_response,
    _task_poll_interval,
    _timeout_seconds,
)
from tests.test_invoker import MockServer, header


# ---------------------------------------------------------------------------
# module helpers
# ---------------------------------------------------------------------------


def test_calculate_reconnect_delay_bounds():
    config = RetryConfig(initial_delay_ms=100, max_delay_ms=1000, backoff_multiplier=2.0, jitter_factor=0.5)
    for attempt in range(6):
        delay = _calculate_reconnect_delay(attempt, config)  # type: ignore[arg-type]
        assert 0 <= delay <= 1.5


def test_calculate_reconnect_delay_zero_jitter():
    config = RetryConfig(initial_delay_ms=100, max_delay_ms=1000, backoff_multiplier=2.0, jitter_factor=0.0)
    assert _calculate_reconnect_delay(0, config) == 0.1  # type: ignore[arg-type]
    assert _calculate_reconnect_delay(2, config) == 0.4  # type: ignore[arg-type]


def test_parse_json_payload_blank_returns_empty_object():
    assert _parse_json_payload("") == {}
    assert _parse_json_payload("   ") == {}


def test_parse_json_payload_invalid_json_raises():
    with pytest.raises(ValueError):
        _parse_json_payload("{not json")


def test_optional_string_and_int():
    assert _optional_string("x") == "x"
    assert _optional_string("") is None
    assert _optional_string(5) is None
    assert _optional_int(3) == 3
    assert _optional_int(True) is None
    assert _optional_int("3") is None


def test_timeout_seconds_minimum_is_one_millisecond():
    assert _timeout_seconds(0) == 0.001
    assert _timeout_seconds(1500) == 1.5


def test_task_poll_interval_fallback_when_non_positive():
    assert _task_poll_interval(0) == 0.5
    assert _task_poll_interval(-1) == 0.5
    assert _task_poll_interval(2) == 2


def test_retry_delay_seconds_branches():
    assert _retry_delay_seconds(0, None) == 0
    cfg = RetryConfig(initial_delay_ms=100, max_delay_ms=250, backoff_multiplier=2.0, jitter_factor=0.0)
    assert _retry_delay_seconds(5, cfg) == 0.25  # capped by max_delay_ms
    negative = RetryConfig(initial_delay_ms=100, backoff_multiplier=-1, jitter_factor=0.0, max_delay_ms=0)
    assert _retry_delay_seconds(1, negative) == 0.2  # fallback multiplier 2.0, no cap


def test_is_retryable_error_matrix():
    assert _is_retryable_error(ServerHTTPError(429, "rate")) is True
    assert _is_retryable_error(ServerHTTPError(503, "boom")) is True
    assert _is_retryable_error(ServerHTTPError(404, "nope")) is False
    assert _is_retryable_error(RuntimeError("transport")) is True
    assert _is_retryable_error(ValueError("other")) is False


def test_server_error_message_extracts_message_or_error():
    assert _server_error_message('{"message":"m"}') == "m"
    assert _server_error_message('{"error":"e"}') == "e"
    assert _server_error_message("<html>") == "<html>"
    assert _server_error_message("") == "empty response body"
    assert _server_error_message('{"message":"  "}') == '{"message":"  "}'


def test_reject_header_injection_blocks_crlf():
    with pytest.raises(ValueError):
        _reject_header_injection({"X-A": "va\r\nlue"})
    with pytest.raises(ValueError):
        _reject_header_injection({"X-\rA": "v"})
    _reject_header_injection({"X-A": "fine"})


def test_normalize_server_api_url_rejects_bad_scheme():
    with pytest.raises(ValueError):
        _normalize_server_api_url("ftp://host")
    with pytest.raises(ValueError):
        _normalize_server_api_url("http://")


def test_endpoint_url_quotes_segments():
    url = _endpoint_url("http://h/api/v1", ("a b/c", "d"), {"k": "1 2"})
    assert url == "http://h/api/v1/a%20b%2Fc/d?k=1+2"


def test_json_string_ensures_ascii_false():
    assert _json_string({"n": "中"}) == '{"n":"中"}'


def test_task_event_done_maps_to_completed():
    event = _task_event_from_response("t1", {"type": "done"})
    assert event.type == "completed"
    assert event.done is True
    assert _task_event_from_response("t1", {}).type == "unknown"


# ---------------------------------------------------------------------------
# Invoker._send_request error mapping (monkeypatched urlopen)
# ---------------------------------------------------------------------------


class _FakeResponse(BytesIO):
    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()
        return False


def test_send_request_http_error_wrapped(monkeypatch):
    invoker = Invoker()

    def fake_urlopen(*args, **kwargs):
        raise urllib.error.HTTPError(
            "url", 500, "Server Error", hdrs=None, fp=BytesIO(b'{"message":"boom"}')
        )

    monkeypatch.setattr("croupier.invoker.urlopen", fake_urlopen)
    with pytest.raises(ServerHTTPError) as excinfo:
        invoker._send_request("GET", "http://h/api/v1/tasks/1", None, {}, 1.0)
    assert excinfo.value.status_code == 500
    assert "boom" in str(excinfo.value)


def test_send_request_url_error_wrapped(monkeypatch):
    invoker = Invoker()

    def fake_urlopen(*args, **kwargs):
        raise urllib.error.URLError("conn refused")

    monkeypatch.setattr("croupier.invoker.urlopen", fake_urlopen)
    with pytest.raises(RuntimeError, match="conn refused"):
        invoker._send_request("GET", "http://h/x", None, {}, 1.0)


def test_ssl_context_https_insecure(monkeypatch):
    invoker = Invoker(InvokerConfig(address="https://h", insecure=True))
    ctx = invoker._ssl_context()
    assert isinstance(ctx, ssl.SSLContext)
    assert ctx.verify_mode == ssl.CERT_NONE


def test_ssl_context_https_default(monkeypatch):
    invoker = Invoker(InvokerConfig(address="https://h"))
    ctx = invoker._ssl_context()
    assert ctx is not None
    assert ctx.verify_mode == ssl.CERT_REQUIRED


def test_ssl_context_http_is_none():
    invoker = Invoker(InvokerConfig(address="http://h"))
    assert invoker._ssl_context() is None


# ---------------------------------------------------------------------------
# Async invoker edge cases against a mock server
# ---------------------------------------------------------------------------


def test_invoke_response_without_result_raises():
    with MockServer(lambda method, path, headers, payload: (200, {"other": 1})) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def run():
            with pytest.raises(ValueError, match="result"):
                await invoker.invoke("f", "{}")

        asyncio.run(run())


def test_invoke_invalid_json_response_raises():
    def responder(method, path, headers, payload):
        raw = {"__raw__": "not-json"}

    # Use monkeypatched urlopen returning plain text instead of MockServer JSON.
    invoker = Invoker()

    def fake_urlopen(*args, **kwargs):
        return _FakeResponse(b"plain text")

    import croupier.invoker as invoker_module

    original = invoker_module.urlopen
    invoker_module.urlopen = fake_urlopen
    try:

        async def run():
            with pytest.raises(ValueError, match="invalid JSON"):
                await invoker.invoke("f", "{}")

        asyncio.run(run())
    finally:
        invoker_module.urlopen = original


def test_get_task_status_non_object_response_raises():
    with MockServer(lambda method, path, headers, payload: (200, [1, 2])) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def run():
            with pytest.raises(ValueError, match="object"):
                await invoker.get_task_status("t1")

        asyncio.run(run())


def test_stream_task_rejects_non_object_response():
    with MockServer(lambda method, path, headers, payload: (200, [1])) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def run():
            with pytest.raises(ValueError, match="object"):
                async for _ in invoker.stream_task("t1"):
                    pass

        asyncio.run(run())


def test_stream_task_rejects_non_array_items():
    with MockServer(lambda method, path, headers, payload: (200, {"items": {"a": 1}})) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def run():
            with pytest.raises(ValueError, match="array"):
                async for _ in invoker.stream_task("t1"):
                    pass

        asyncio.run(run())


def test_stream_task_rejects_non_object_event():
    with MockServer(lambda method, path, headers, payload: (200, {"items": [42]})) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def run():
            with pytest.raises(ValueError, match="event"):
                async for _ in invoker.stream_task("t1"):
                    pass

        asyncio.run(run())


def test_stream_task_polls_until_done():
    state = {"calls": 0}

    def responder(method, path, headers, payload):
        state["calls"] += 1
        if state["calls"] == 1:
            return 200, {"items": []}  # no events yet -> sleep then poll again
        return 200, {"items": [{"seq": 1, "type": "progress", "progress": 50}], "done": True}

    with MockServer(responder) as server:
        invoker = Invoker(InvokerConfig(address=server.address, task_poll_interval=0.01))

        async def run():
            events = [event async for event in invoker.stream_task("t1")]

        asyncio.run(run())
        assert state["calls"] == 2


def test_validate_payload_empty_schema_is_noop():
    invoker = Invoker()
    invoker._validate_payload("{}", {})


def test_validate_payload_non_object_payload_raises():
    invoker = Invoker()
    with pytest.raises(ValueError, match="payload validation failed"):
        invoker._validate_payload("[1,2]", {"type": "object", "required": ["a"]})


def test_validate_payload_missing_required_field_raises():
    invoker = Invoker()
    with pytest.raises(ValueError, match="'b' is a required property"):
        invoker._validate_payload('{"a":1}', {"required": ["b"]})


def test_validate_payload_extra_schema_constraints_enforced():
    invoker = Invoker()
    invoker._validate_payload('{"a":1}', {"type": "object"})
    with pytest.raises(ValueError, match="payload validation failed"):
        invoker._validate_payload('{"a":1}', {"type": "object", "additionalProperties": False, "properties": {"b": {}}})


# ---------------------------------------------------------------------------
# SyncInvoker paths
# ---------------------------------------------------------------------------


def test_sync_invoker_full_lifecycle():
    def responder(method, path, headers, payload):
        clean = path.split("?")[0]
        if clean.endswith("/invoke"):
            return 200, {"result": {"ok": True}}
        if clean == "/api/v1/tasks":
            return 200, {"taskId": "task-9"}
        if clean.endswith("/events"):
            return 200, {"items": [{"seq": 1, "type": "done"}], "done": True}
        if clean.endswith("/cancel"):
            return 200, {}
        if clean == "/api/v1/tasks/task-9":
            return 200, {"id": "task-9", "status": "completed", "progress": 100}
        return 200, {}

    with MockServer(responder) as server:
        sync_invoker = SyncInvoker(InvokerConfig(address=server.address))
        sync_invoker.connect()
        assert json.loads(sync_invoker.invoke("fn", '{"a":1}')) == {"ok": True}
        assert sync_invoker.start_task("fn", "{}") == "task-9"

        sync_invoker.set_schema("fn", {"required": ["a"]})
        with pytest.raises(ValueError):
            sync_invoker.invoke("fn", "{}")

        events: List[TaskEventInfo] = list(sync_invoker.stream_task("task-9"))
        assert events and events[0].type == "completed"

        status = sync_invoker.get_task_status("task-9")
        assert status.status == "completed"

        sync_invoker.cancel_task("task-9")
        sync_invoker.close()


def test_sync_invoker_rejects_running_loop():
    sync_invoker = SyncInvoker()
    with pytest.raises(RuntimeError):
        asyncio.run(_invoke_from_running_loop(sync_invoker))


async def _invoke_from_running_loop(sync_invoker: SyncInvoker):
    sync_invoker.invoke("f", "{}")


# ---------------------------------------------------------------------------
# Header handling
# ---------------------------------------------------------------------------


def test_headers_do_not_override_user_supplied():
    invoker = Invoker(
        InvokerConfig(auth_token="tok", game_id="g1", env="e1")
    )
    headers = invoker._headers(
        InvokeOptions(
            idempotency_key="ik",
            headers={
                "Idempotency-Key": "custom",
                "X-Game-ID": "custom",
                "X-Env": "custom",
                "Authorization": "Bearer custom",
            },
        )
    )
    assert headers["Idempotency-Key"] == "custom"
    assert header(headers, "X-Game-ID") == "custom"
    assert header(headers, "X-Env") == "custom"
    assert headers["Authorization"] == "Bearer custom"


def test_headers_bearer_prefix_added_once():
    invoker = Invoker(InvokerConfig(auth_token="tok"))
    headers = invoker._headers(InvokeOptions())
    assert headers["Authorization"] == "Bearer tok"

    invoker2 = Invoker(InvokerConfig(auth_token="Bearer tok2"))
    assert invoker2._headers(InvokeOptions())["Authorization"] == "Bearer tok2"


def test_headers_blank_token_omitted():
    invoker = Invoker(InvokerConfig(auth_token="   "))
    assert "Authorization" not in invoker._headers(InvokeOptions())


def test_headers_reject_crlf_values():
    invoker = Invoker()
    with pytest.raises(ValueError):
        invoker._headers(InvokeOptions(headers={"X-Bad": "a\r\nb"}))


def test_retry_uses_request_level_retry_config():
    attempts: List[str] = []

    def responder(method, path, headers, payload):
        attempts.append(path)
        if len(attempts) < 3:
            return 500, {"message": "flaky"}
        return 200, {"result": {"ok": True}}

    with MockServer(responder) as server:
        invoker = Invoker(
            InvokerConfig(address=server.address, retry=RetryConfig(enabled=False))
        )

        async def run():
            result = await invoker.invoke(
                "fn",
                "{}",
                InvokeOptions(retry=RetryConfig(max_attempts=3, initial_delay_ms=1, jitter_factor=0.0)),
            )
            assert json.loads(result) == {"ok": True}

        asyncio.run(run())
        assert len(attempts) == 3


def test_non_retryable_error_not_retried():
    attempts: List[str] = []

    def responder(method, path, headers, payload):
        attempts.append(path)
        return 404, {"message": "missing"}

    with MockServer(responder) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def run():
            with pytest.raises(ServerHTTPError):
                await invoker.invoke("fn", "{}")

        asyncio.run(run())
        assert len(attempts) == 1
