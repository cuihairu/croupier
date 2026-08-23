"""Third coverage boost: exercise remaining uncovered branches in openapi,
dispatcher, invoker and client helpers (error paths, edge inputs)."""

from __future__ import annotations

import json
import threading
import time
from typing import Any, Dict, List

import pytest

from croupier import CroupierClient, ClientConfig, FunctionDescriptor
from croupier.dispatcher import MainThreadDispatcher
from croupier.invoker import (
    Invoker,
    InvokerConfig,
    _legacy_required_validation_errors,
    _retry_delay_seconds,
    RetryConfig,
)
from croupier.openapi import (
    _derive_summary,
    _extract_extension,
    _iter_operations,
    _json_content_schema,
    _schema_to_json_schema,
    _to_title_case,
)
from croupier import openapi as openapi_module


def noop_handler(context: str, payload: bytes) -> str:
    return "{}"


class RecordingClient:
    def __init__(self) -> None:
        self.registered: List[tuple] = []

    def register_function(self, descriptor, handler) -> None:
        self.registered.append((descriptor, handler))


# ---------------------------------------------------------------------------
# openapi helper edge branches
# ---------------------------------------------------------------------------


def test_openapi_summary_unnamed_for_missing_everything():
    assert _derive_summary({}, "unknown.function") == "Unnamed Function"
    assert _derive_summary({"summary": ""}, "unknown.function") == "Unnamed Function"


def test_openapi_schema_conversion_rejects_non_dicts():
    assert _schema_to_json_schema(None) is None
    assert _schema_to_json_schema("string") is None
    assert _schema_to_json_schema([]) is None
    assert _schema_to_json_schema({}) is None


def test_openapi_schema_conversion_ignores_malformed_properties():
    schema = {"type": "object", "properties": "not-a-dict", "required": "not-a-list"}
    assert _schema_to_json_schema(schema) == {"type": "object"}


def test_openapi_schema_conversion_skips_non_dict_props():
    schema = {"properties": {"good": {"type": "string"}, "bad": "scalar"}}
    result = _schema_to_json_schema(schema)
    assert result == {"properties": {"good": {"type": "string"}}}


def test_openapi_json_content_schema_tolerates_malformed_holders():
    assert _json_content_schema(None) is None
    assert _json_content_schema("string") is None
    assert _json_content_schema({"content": "nope"}) is None
    assert _json_content_schema({"content": {"application/json": "nope"}}) is None
    assert _json_content_schema({"content": {"application/json": {"schema": {}}}}) is None


def test_openapi_extract_extension_non_string_types():
    assert _extract_extension({"x-n": 42}, "x-n") == "42"
    assert _extract_extension({"x-l": [1, 2]}, "x-l") == "[1, 2]"
    assert _extract_extension({"x-d": {"a": 1}}, "x-d") == '{"a": 1}'
    assert _extract_extension({}, "x-missing") == ""


def test_openapi_iter_operations_yields_method_and_path():
    spec = {"paths": {"/a": {"get": {"operationId": "g"}, "post": {"operationId": "p"}}, "/b": "invalid"}}
    pairs = [(path, op.get("operationId")) for path, method, op in _iter_operations(spec)]
    assert pairs == [("/a", "g"), ("/a", "p")]


def test_openapi_iter_operations_ignores_non_dict_paths():
    assert list(_iter_operations({"paths": "nope"})) == []
    assert list(_iter_operations({"paths": {"/x": None}})) == []


def test_openapi_register_rejects_wrong_spec_type():
    with pytest.raises(ValueError, match="spec must be"):
        openapi_module.register_from_openapi(RecordingClient(), 12345, handlers={})


def test_openapi_to_title_case_single_words():
    assert _to_title_case("word") == "Word"
    assert _to_title_case("WORD") == "Word"
    assert _to_title_case("a") == "A"


# ---------------------------------------------------------------------------
# dispatcher edge branches
# ---------------------------------------------------------------------------


def test_dispatcher_immediate_execution_swallows_callback_errors():
    dispatcher = MainThreadDispatcher()
    dispatcher.initialize()
    try:
        ran = []
        dispatcher.enqueue(lambda: ran.append(1))  # executed immediately on main thread
        assert ran == [1]

        # A raising callback must be swallowed (logged), not propagated.
        def boom() -> None:
            raise RuntimeError("callback exploded")

        dispatcher.enqueue(boom)
    finally:
        MainThreadDispatcher.reset_instance()


def test_dispatcher_enqueue_with_data_ignores_none_callback():
    dispatcher = MainThreadDispatcher()
    MainThreadDispatcher.reset_instance()  # not initialized: enqueue paths queue instead
    assert dispatcher.enqueue_with_data(lambda data: None, 5) is None
    assert dispatcher.enqueue_with_data(None, 5) is None  # None branch
    dispatcher.clear()


def test_dispatcher_is_main_thread_before_initialize():
    dispatcher = MainThreadDispatcher()
    MainThreadDispatcher.reset_instance()
    assert dispatcher.is_main_thread() is False


def test_dispatcher_clear_and_pending_count():
    dispatcher = MainThreadDispatcher()
    MainThreadDispatcher.reset_instance()
    dispatcher.enqueue(lambda: None)
    dispatcher.enqueue(lambda: None)
    assert dispatcher.get_pending_count() == 2
    dispatcher.clear()
    assert dispatcher.get_pending_count() == 0


def test_dispatcher_process_queue_empty_returns_zero():
    dispatcher = MainThreadDispatcher()
    MainThreadDispatcher.reset_instance()
    assert dispatcher.process_queue() == 0


# ---------------------------------------------------------------------------
# invoker legacy fallback and retry helpers
# ---------------------------------------------------------------------------


def test_legacy_required_validation_errors_branches():
    assert _legacy_required_validation_errors("scalar", {}) == ["root: expected a JSON object"]
    assert _legacy_required_validation_errors({"a": 1}, {"required": ["a"]}) == []
    assert _legacy_required_validation_errors({}, {"required": ["a"]}) == [
        "root: missing required field 'a'"
    ]
    assert _legacy_required_validation_errors({}, {"required": "not-a-list"}) == []


def test_retry_delay_seconds_zero_jitter_is_deterministic():
    retry = RetryConfig(initial_delay_ms=100, backoff_multiplier=2.0, jitter_factor=0.0, max_delay_ms=0)
    assert _retry_delay_seconds(0, retry) == 0.1
    assert _retry_delay_seconds(1, retry) == 0.2


def test_retry_delay_seconds_negative_delay_clamped_to_zero():
    retry = RetryConfig(initial_delay_ms=0, jitter_factor=0.0, max_delay_ms=0)
    assert _retry_delay_seconds(5, retry) == 0.0


def test_invoker_start_task_empty_function_id_raises():
    invoker = Invoker(InvokerConfig(address="http://127.0.0.1:1"))

    import asyncio

    async def run():
        with pytest.raises(ValueError, match="function ID"):
            await invoker.start_task("", "{}")

    asyncio.run(run())


def test_invoker_cancel_task_empty_id_raises():
    invoker = Invoker(InvokerConfig(address="http://127.0.0.1:1"))

    import asyncio

    async def run():
        with pytest.raises(ValueError):
            await invoker.cancel_task("  ")

    asyncio.run(run())


# ---------------------------------------------------------------------------
# client: unsupported inbound messages and descriptor access
# ---------------------------------------------------------------------------


def test_client_handle_inbound_unknown_msgid_returns_empty():
    client = CroupierClient(ClientConfig())
    client.register_function(FunctionDescriptor(id="fn", version="1.0.0"), noop_handler)

    from croupier import protocol

    result = client._handle_inbound(0x99AABB, 1, b"")
    assert result == b""


def test_client_get_function_descriptor_unknown_returns_none():
    client = CroupierClient(ClientConfig())
    client.register_function(
        FunctionDescriptor(id="known.fn", version="1.0.0", input_schema={"type": "object"}),
        noop_handler,
    )
    assert client.get_function_descriptor("ghost.fn") is None

    descriptor = client.get_function_descriptor("known.fn")
    assert descriptor is not None
    assert descriptor.input_schema == '{"type": "object"}'


def test_client_string_schema_passthrough():
    client = CroupierClient(ClientConfig())
    client.register_function(
        FunctionDescriptor(id="str.fn", version="1.0.0", input_schema='{"type":"array"}'),
        noop_handler,
    )
    descriptor = client.get_function_descriptor("str.fn")
    assert descriptor.input_schema == '{"type":"array"}'


def test_client_normalize_agent_addr_strips_scheme():
    from croupier import CroupierClient as CC

    assert CC._normalize_agent_addr("tcp://10.0.0.1:19091") == "10.0.0.1:19091"
    assert CC._normalize_agent_addr("10.0.0.1:19091") == "10.0.0.1:19091"
