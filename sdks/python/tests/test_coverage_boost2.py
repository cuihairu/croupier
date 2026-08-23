"""Second coverage boost: config validation, descriptor/manifest behaviour,
protocol helpers, OpenAPI edge cases and Draft-07 schema corners."""

from __future__ import annotations

import json

import pytest

from croupier import (
    ClientConfig,
    CroupierClient,
    FunctionDescriptor,
    ImportOptions,
    register_from_openapi,
)
from croupier import protocol
from croupier.invoker import (
    Invoker,
    InvokerConfig,
    RetryConfig,
    SyncInvoker,
    _draft7_validation_errors,
    _json_string,
    _normalize_server_api_url,
    _task_event_from_response,
    _timeout_seconds,
)
from croupier.openapi import _derive_operation_id, _to_title_case


def noop_handler(context: str, payload: bytes) -> str:
    return "{}"


def make_descriptor(**overrides) -> FunctionDescriptor:
    fields = {
        "id": "player.ban",
        "version": "1.0.0",
        "resource": "player",
        "operation": "ban",
        "summary": "Ban player",
        "tags": ["gm"],
        "risk": "high",
    }
    fields.update(overrides)
    return FunctionDescriptor(**fields)


class RecordingClient:
    def __init__(self) -> None:
        self.registered = []

    def register_function(self, descriptor, handler) -> None:
        self.registered.append((descriptor, handler))


# ---------------------------------------------------------------------------
# ClientConfig & descriptor validation
# ---------------------------------------------------------------------------


def test_client_config_defaults():
    config = ClientConfig()
    assert config.agent_addr == "127.0.0.1:19091"
    assert config.insecure is True
    assert config.env == "development"
    assert config.provider_lang == "python"
    assert config.provider_sdk == "croupier-python-sdk"
    assert config.auto_reconnect is True
    assert config.retry is None
    assert config.service_id.startswith("python-sdk-")


def test_client_config_service_ids_are_unique():
    assert ClientConfig().service_id != ClientConfig().service_id


def test_register_function_requires_id_and_version():
    client = CroupierClient(ClientConfig())

    with pytest.raises(ValueError, match="id and version"):
        client.register_function(FunctionDescriptor(id="", version="1.0.0"), noop_handler)
    with pytest.raises(ValueError, match="id and version"):
        client.register_function(FunctionDescriptor(id="fn", version=""), noop_handler)

    client.register_function(make_descriptor(), noop_handler)
    assert client.is_draining() is False


def test_register_function_overwrites_duplicate_id():
    client = CroupierClient(ClientConfig())
    client.register_function(make_descriptor(), noop_handler)
    client.register_function(make_descriptor(summary="Updated"), noop_handler)

    manifest = json.loads(client.build_manifest())
    assert len(manifest["functions"]) == 1
    assert manifest["functions"][0]["summary"] == "Updated"


def test_build_manifest_includes_optional_fields():
    client = CroupierClient(ClientConfig())
    client.register_function(
        make_descriptor(
            deprecated=True,
            input_schema={"type": "object"},
            output_schema='{"type":"object"}',
            permission="player.ban",
            capability="action",
            execution="sync",
        ),
        noop_handler,
    )
    manifest = json.loads(client.build_manifest())

    entry = manifest["functions"][0]
    assert entry["id"] == "player.ban"
    assert entry["deprecated"] is True
    assert entry["tags"] == ["gm"]
    assert entry["resource"] == "player"
    assert entry["input_schema"] == {"type": "object"}
    assert entry["output_schema"] == '{"type":"object"}'
    assert manifest["provider"]["id"] == client._config.service_id
    assert manifest["provider"]["lang"] == "python"


def test_build_manifest_omits_unset_optional_fields():
    client = CroupierClient(ClientConfig())
    client.register_function(FunctionDescriptor(id="bare.fn", version="0.1.0"), noop_handler)
    entry = json.loads(client.build_manifest())["functions"][0]
    assert "tags" not in entry
    assert "summary" not in entry
    assert "deprecated" not in entry
    assert "input_schema" not in entry


def test_descriptor_defaults():
    descriptor = FunctionDescriptor(id="x")
    assert descriptor.version == "1.0.0"
    assert descriptor.tags == []
    assert descriptor.deprecated is False
    assert descriptor.enabled is True
    assert descriptor.approval_required is False


# ---------------------------------------------------------------------------
# Protocol helpers
# ---------------------------------------------------------------------------


def test_protocol_round_trip_all_fields():
    body = b"\x01\x02\x03"
    message = protocol.new_message(protocol.MSG_INVOKE_REQUEST, 0x00ABCDEF, body)
    version, msg_id, req_id, parsed = protocol.parse_message(message)

    assert version == 1
    assert msg_id == protocol.MSG_INVOKE_REQUEST
    assert req_id == 0x00ABCDEF
    assert parsed == body


def test_protocol_request_response_parity():
    assert protocol.is_request(protocol.MSG_INVOKE_REQUEST) is True
    assert protocol.is_response(protocol.MSG_INVOKE_RESPONSE) is True
    assert protocol.is_request(protocol.MSG_INVOKE_RESPONSE) is False
    assert protocol.is_response(protocol.MSG_INVOKE_REQUEST) is False
    # Even IDs are responses, odd IDs are requests.
    assert protocol.is_request(0x000003) is True
    assert protocol.is_response(0x000004) is True


def test_protocol_msg_id_encoding_boundaries():
    assert protocol.get_msg_id(protocol.put_msg_id(0)) == 0
    assert protocol.get_msg_id(protocol.put_msg_id(0xFFFFFF)) == 0xFFFFFF
    assert protocol.get_msg_id(protocol.put_msg_id(0x050105)) == protocol.MSG_PROVIDER_DRAIN_REQUEST


def test_protocol_msg_id_string_known_and_unknown():
    assert protocol.msg_id_string(protocol.MSG_PROVIDER_CONNECT_REQUEST) == "ProviderConnectRequest"
    assert "Unknown" in protocol.msg_id_string(0x99AABB)


def test_drain_constants_match_wire_protocol():
    assert protocol.MSG_PROVIDER_DRAIN_REQUEST == 0x050105
    assert protocol.MSG_PROVIDER_DRAIN_RESPONSE == 0x050106


# ---------------------------------------------------------------------------
# Invoker helpers
# ---------------------------------------------------------------------------


def test_normalize_server_api_url_variants():
    assert _normalize_server_api_url("host:1234") == "http://host:1234/api/v1"
    assert _normalize_server_api_url("https://h/api/v1/") == "https://h/api/v1"
    assert _normalize_server_api_url("  ") == "http://127.0.0.1:18780/api/v1"


def test_retry_config_defaults_match_go():
    retry = RetryConfig()
    assert retry.enabled is True
    assert retry.max_attempts == 3
    assert retry.initial_delay_ms == 100
    assert retry.max_delay_ms == 5000
    assert retry.backoff_multiplier == 2.0
    assert retry.jitter_factor == 0.1


def test_timeout_seconds_clamps_to_one_millisecond():
    assert _timeout_seconds(-5) == 0.001
    assert _timeout_seconds(2500) == 2.5


def test_json_string_preserves_unicode():
    assert _json_string({"msg": "已封禁"}) == '{"msg":"已封禁"}'


def test_task_event_error_mapping():
    for event_type in ("error", "failed", "cancelled", "timed_out"):
        event = _task_event_from_response("t1", {"type": event_type, "message": "boom"})
        assert event.error == "boom"
        assert event.done is True

    progress = _task_event_from_response("t1", {"type": "progress", "progress": 40})
    assert progress.error is None
    assert progress.done is False
    assert progress.progress == 40


# ---------------------------------------------------------------------------
# Draft-07 corner cases
# ---------------------------------------------------------------------------


def test_draft7_type_arrays():
    schema = {"type": ["string", "null"]}
    assert _draft7_validation_errors("x", schema) == []
    assert _draft7_validation_errors(None, schema) == []
    assert _draft7_validation_errors(5, schema) != []


def test_draft7_boolean_true_schema_accepts_anything():
    assert _draft7_validation_errors(42, True) == []


def test_draft7_local_ref_resolution():
    schema = {
        "definitions": {"positive": {"type": "number", "minimum": 1}},
        "$ref": "#/definitions/positive",
    }
    assert _draft7_validation_errors(2, schema) == []
    assert _draft7_validation_errors(0, schema) != []


def test_draft7_nested_array_of_objects():
    schema = {
        "type": "object",
        "properties": {
            "items": {
                "type": "array",
                "items": {"type": "object", "required": ["id"], "properties": {"id": {"type": "string"}}},
            }
        },
    }
    valid = {"items": [{"id": "a"}, {"id": "b"}]}
    missing_id = {"items": [{"id": "a"}, {}]}
    wrong_type = {"items": [{"id": 7}]}

    assert _draft7_validation_errors(valid, schema) == []
    errors = _draft7_validation_errors(missing_id, schema)
    assert any("id" in message for message in errors)
    assert _draft7_validation_errors(wrong_type, schema) != []


def test_draft7_pattern_and_format_keywords():
    schema = {"type": "object", "properties": {"email": {"type": "string", "pattern": "^.+@.+$"}}}
    assert _draft7_validation_errors({"email": "a@b"}, schema) == []
    assert _draft7_validation_errors({"email": "nope"}, schema) != []
    # Unknown formats are ignored in Draft-07 by default.
    permissive = {"type": "string", "format": "mime-type"}
    assert _draft7_validation_errors("anything", permissive) == []


def test_invoker_validate_payload_multiple_errors_joined():
    invoker = Invoker(InvokerConfig(address="http://127.0.0.1:1"))
    schema = {
        "type": "object",
        "required": ["a", "b"],
        "properties": {"a": {"type": "integer"}, "b": {"type": "string"}},
    }
    with pytest.raises(ValueError) as excinfo:
        invoker._validate_payload('{"a":"x","b":2}', schema)
    message = str(excinfo.value)
    assert "; " in message or "payload validation failed" in message


# ---------------------------------------------------------------------------
# OpenAPI edge cases
# ---------------------------------------------------------------------------


def test_openapi_ignores_non_string_tags():
    spec = {
        "paths": {
            "/a": {
                "post": {
                    "operationId": "a_post",
                    "tags": ["valid", 42, None],
                    "responses": {},
                }
            }
        }
    }
    client = RecordingClient()
    register_from_openapi(client, spec, handlers={"a_post": noop_handler})
    descriptor = client.registered[0][0]
    assert descriptor.tags == ["valid"]


def test_openapi_skips_non_object_path_items():
    spec = {"paths": {"/bad": "not-an-object", "/ok": {"get": {"operationId": "ok_get", "responses": {}}}}}
    client = RecordingClient()
    registered = register_from_openapi(client, spec, handlers={"ok_get": noop_handler})
    assert registered == ["ok_get"]


def test_openapi_resolver_exception_propagates():
    spec = {"paths": {"/a": {"get": {"operationId": "a_get", "responses": {}}}}}
    client = RecordingClient()

    def broken_resolver(function_id):
        raise RuntimeError("resolver exploded")

    with pytest.raises(RuntimeError, match="resolver exploded"):
        register_from_openapi(client, spec, handler_resolver=broken_resolver)


def test_openapi_bytes_spec_accepted():
    spec = json.dumps({"paths": {"/a": {"get": {"operationId": "a_get", "responses": {}}}}}).encode()
    client = RecordingClient()
    registered = register_from_openapi(client, spec, handlers={"a_get": noop_handler})
    assert registered == ["a_get"]


def test_openapi_prefix_options_empty_prefixes_are_noops():
    spec = {"paths": {"/a": {"get": {"operationId": "a_get", "x-resource": "thing", "responses": {}}}}}
    client = RecordingClient()
    options = ImportOptions(resource_prefix="", tag_prefix="")
    register_from_openapi(client, spec, options, handlers={"a_get": noop_handler})
    descriptor = client.registered[0][0]
    assert descriptor.resource == "thing"


def test_openapi_helper_title_case():
    assert _to_title_case("player_ban") == "Player Ban"
    assert _to_title_case("a_b_c") == "A B C"
    assert _to_title_case("") == ""
    assert _to_title_case("_leading") == " Leading"
    assert _to_title_case("trailing_") == "Trailing "


def test_openapi_derive_operation_id_fallbacks():
    assert _derive_operation_id({}, "/a/b/c") == "a.b.c"
    assert _derive_operation_id({}, "/single") == "single"
    assert _derive_operation_id({}, "///") == "unknown.function"
    assert _derive_operation_id({"operationId": "custom"}, "/ignored") == "custom"


def test_openapi_empty_responses_yields_no_schemas():
    spec = {"paths": {"/a": {"post": {"operationId": "a_post", "responses": {}}}}}
    client = RecordingClient()
    register_from_openapi(client, spec, handlers={"a_post": noop_handler})
    descriptor = client.registered[0][0]
    assert descriptor.input_schema is None
    assert descriptor.output_schema is None


# ---------------------------------------------------------------------------
# SyncInvoker smoke against a mock server
# ---------------------------------------------------------------------------


def test_sync_invoker_schema_and_close_roundtrip():
    from test_invoker import MockServer

    with MockServer(lambda method, path, headers, payload: (200, {"result": {}})) as server:
        invoker = SyncInvoker(InvokerConfig(address=server.address))
        invoker.connect()
        invoker.set_schema("fn", {"type": "object", "required": ["a"]})

        with pytest.raises(ValueError):
            invoker.invoke("fn", "{}")
        result = json.loads(invoker.invoke("fn", '{"a":1}'))
        assert result == {}

        invoker.close()
        # Schemas are cleared on close.
        invoker.set_schema("fn", {"type": "object", "required": ["a"]})
        invoker.close()
