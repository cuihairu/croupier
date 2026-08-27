"""Tests for OpenAPI 3 import and Draft7 JSON Schema payload validation."""

from __future__ import annotations

import json
from typing import List, Optional, Tuple

import pytest

from croupier import (
    ClientConfig,
    CroupierClient,
    FunctionDescriptor,
    ImportOptions,
    RegisterFromOpenAPI,
    register_from_openapi,
)
from croupier.invoker import Invoker, InvokerConfig, _draft7_validation_errors

SPEC = {
    "openapi": "3.0.3",
    "info": {"title": "GM API", "version": "1.0.0"},
    "paths": {
        "/players/{id}/ban": {
            "put": {
                "operationId": "player_ban",
                "summary": "Ban player",
                "description": "Bans a player account",
                "tags": ["gm", "risk"],
                "x-resource": "player",
                "x-operation": "ban",
                "x-permission": "player.ban",
                "x-risk": "high",
                "requestBody": {
                    "content": {
                        "application/json": {
                            "schema": {
                                "type": "object",
                                "required": ["playerId", "reason"],
                                "properties": {
                                    "playerId": {"type": "string", "description": "Player ID"},
                                    "reason": {"type": "string"},
                                },
                            }
                        }
                    }
                },
                "responses": {
                    "200": {
                        "content": {
                            "application/json": {
                                "schema": {"type": "object", "properties": {"ok": {"type": "boolean"}}}
                            }
                        }
                    }
                },
            }
        },
        "/players/search": {
            "get": {
                "tags": ["query"],
                "responses": {"200": {"content": {"application/json": {"schema": {"type": "array"}}}}},
            }
        },
    },
}


class RecordingClient:
    """Minimal CroupierClient stand-in that records registrations."""

    def __init__(self) -> None:
        self.registered: List[Tuple[FunctionDescriptor, object]] = []
        self.fail_ids: set[str] = set()

    def register_function(self, descriptor: FunctionDescriptor, handler) -> None:
        if descriptor.id in self.fail_ids:
            raise RuntimeError("rejected")
        self.registered.append((descriptor, handler))


def handler_for(function_id: str):
    def handler(context: str, payload: bytes) -> str:
        return "{}"

    return handler


# ---------------------------------------------------------------------------
# RegisterFromOpenAPI
# ---------------------------------------------------------------------------


def test_register_from_openapi_registers_all_operations():
    client = RecordingClient()
    registered = register_from_openapi(client, SPEC, handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})

    assert registered == ["player_ban", "players.search"]
    assert len(client.registered) == 2


def test_register_from_openapi_maps_metadata_fields():
    client = RecordingClient()
    register_from_openapi(client, SPEC, handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})

    descriptor, _ = client.registered[0]
    assert descriptor.id == "player_ban"
    assert descriptor.summary == "Ban player"
    assert descriptor.description == "Bans a player account"
    assert descriptor.tags == ["gm", "risk"]
    assert descriptor.resource == "player"
    assert descriptor.operation == "ban"
    assert descriptor.permission == "player.ban"
    assert descriptor.risk == "high"


def test_register_from_openapi_converts_schemas():
    client = RecordingClient()
    register_from_openapi(client, SPEC, handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})

    descriptor, _ = client.registered[0]
    assert descriptor.input_schema == {
        "type": "object",
        "required": ["playerId", "reason"],
        "properties": {
            "playerId": {"type": "string", "description": "Player ID"},
            "reason": {"type": "string"},
        },
    }
    assert descriptor.output_schema == {"type": "object", "properties": {"ok": {"type": "boolean"}}}


def test_register_from_openapi_derives_id_from_path_when_missing():
    client = RecordingClient()
    register_from_openapi(client, SPEC, handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})

    descriptor, _ = client.registered[1]
    assert descriptor.id == "players.search"
    # title-case only transforms underscores (Go parity): "players.search" stays
    assert descriptor.summary == "Players.search"


def test_register_from_openapi_risk_defaults_to_medium():
    client = RecordingClient()
    register_from_openapi(client, SPEC, handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})
    assert client.registered[1][0].risk == "warning"
    assert client.registered[0][0].risk == "high"


def test_register_from_openapi_applies_prefix_options():
    client = RecordingClient()
    options = ImportOptions(resource_prefix="game", tag_prefix="svc-")
    register_from_openapi(client, SPEC, options, handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})

    descriptor, _ = client.registered[0]
    assert descriptor.resource == "game.player"
    assert descriptor.tags == ["svc-gm", "svc-risk"]


def test_register_from_openapi_missing_handler_raises():
    client = RecordingClient()
    with pytest.raises(ValueError, match="no handler provided for function: player_ban"):
        register_from_openapi(client, SPEC, handlers={})


def test_register_from_openapi_missing_handler_continue_on_error():
    client = RecordingClient()
    options = ImportOptions(continue_on_error=True)
    registered = register_from_openapi(client, SPEC, options, handlers={"players.search": handler_for("y")})
    assert registered == ["players.search"]


def test_register_from_openapi_registration_failure_continue_on_error():
    client = RecordingClient()
    client.fail_ids.add("player_ban")
    options = ImportOptions(continue_on_error=True)
    registered = register_from_openapi(client, SPEC, options, handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})
    assert registered == ["players.search"]


def test_register_from_openapi_invalid_json_raises():
    with pytest.raises(ValueError, match="load OpenAPI spec failed"):
        register_from_openapi(RecordingClient(), "{not json", handlers={})


def test_register_from_openapi_missing_paths_raises():
    with pytest.raises(ValueError, match="paths"):
        register_from_openapi(RecordingClient(), {"openapi": "3.0.3"}, handlers={})


def test_register_from_openapi_requires_resolver():
    with pytest.raises(ValueError, match="handler_resolver or handlers"):
        register_from_openapi(RecordingClient(), SPEC)


def test_register_from_openapi_accepts_json_string():
    client = RecordingClient()
    registered = RegisterFromOpenAPI(client, json.dumps(SPEC), handlers={"player_ban": handler_for("x"), "players.search": handler_for("y")})
    assert registered == ["player_ban", "players.search"]


def test_register_from_openapi_empty_paths():
    client = RecordingClient()
    assert register_from_openapi(client, {"paths": {}}, handlers={}) == []


def test_register_from_openapi_risk_level_mapping():
    from croupier.openapi import _parse_risk_level

    assert _parse_risk_level("low") == "safe"
    assert _parse_risk_level("safe") == "safe"
    assert _parse_risk_level("medium") == "warning"
    assert _parse_risk_level("moderate") == "warning"
    assert _parse_risk_level("warning") == "warning"
    assert _parse_risk_level("HIGH") == "high"
    assert _parse_risk_level("critical") == "danger"
    assert _parse_risk_level("bogus") == "warning"


# ---------------------------------------------------------------------------
# Draft7 validation helpers
# ---------------------------------------------------------------------------


def test_draft7_validation_errors_empty_for_valid_payload():
    schema = {"type": "object", "required": ["a"], "properties": {"a": {"type": "integer"}}}
    assert _draft7_validation_errors({"a": 1}, schema) == []


def test_draft7_validation_errors_reports_type_and_required():
    schema = {"type": "object", "required": ["a"], "properties": {"a": {"type": "integer"}}}
    errors = _draft7_validation_errors({"a": "x"}, schema)
    assert len(errors) == 1
    assert "a" in errors[0]

    errors = _draft7_validation_errors({}, schema)
    assert any("a" in message for message in errors)


def test_draft7_validation_errors_enums_and_bounds():
    enum_schema = {"enum": ["a", "b"]}
    assert _draft7_validation_errors("c", enum_schema) != []

    bounds_schema = {"type": "number", "minimum": 1, "maximum": 10}
    assert _draft7_validation_errors(0.5, bounds_schema) != []
    assert _draft7_validation_errors(5, bounds_schema) == []


def test_draft7_validation_errors_nested_locations():
    schema = {
        "type": "object",
        "properties": {"items": {"type": "array", "items": {"type": "string"}}},
    }
    errors = _draft7_validation_errors({"items": [1, "ok"]}, schema)
    assert len(errors) == 1
    assert "items" in errors[0]


def test_invoker_validate_payload_uses_draft7():
    invoker = Invoker(InvokerConfig(address="http://127.0.0.1:1"))
    schema = {
        "type": "object",
        "required": ["playerId"],
        "properties": {"playerId": {"type": "string", "minLength": 3}},
    }

    invoker._validate_payload('{"playerId":"abc"}', schema)  # ok

    with pytest.raises(ValueError, match="payload validation failed"):
        invoker._validate_payload('{"playerId":"ab"}', schema)

    with pytest.raises(ValueError, match="payload validation failed"):
        invoker._validate_payload('{"playerId":42}', schema)


def test_invoker_validate_payload_empty_schema_passes():
    invoker = Invoker(InvokerConfig(address="http://127.0.0.1:1"))
    invoker._validate_payload("anything", {})


def test_client_config_accepts_retry_config():
    from croupier.invoker import RetryConfig

    config = ClientConfig()
    assert config.retry is None

    config.retry = RetryConfig(max_attempts=5)
    assert config.retry.max_attempts == 5
