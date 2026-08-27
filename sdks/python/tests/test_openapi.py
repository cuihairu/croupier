"""Tests for Descriptor v2 fields in the OpenAPI import helper."""

from __future__ import annotations

from typing import List, Tuple

import pytest

from croupier import FunctionDescriptor, ImportOptions, register_from_openapi

SPEC = {
    "openapi": "3.0.3",
    "info": {"title": "GM API", "version": "1.0.0"},
    "paths": {
        "/players/{id}/ban": {
            "put": {
                "operationId": "player_ban",
                "summary": "Ban player",
                "tags": ["gm"],
                "x-resource": "player",
                "x-operation": "ban",
                "x-capability": "action",
                "x-execution": "sync",
                "x-permission": "player.ban",
                "x-risk": "high",
                "x-approval": {"required": True, "policyKey": "gm.player.ban"},
                "requestBody": {
                    "content": {
                        "application/json": {
                            "schema": {
                                "type": "object",
                                "required": ["playerId"],
                                "properties": {"playerId": {"type": "string"}},
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
        "/leaderboard": {
            "get": {
                "x-capability": "collection_query",
                "x-risk": "low",
                "responses": {
                    "200": {
                        "content": {
                            "application/json": {"schema": {"type": "array"}}
                        }
                    }
                },
            }
        },
        "/mail/batch-send": {
            "post": {
                "operationId": "mail_batch_send",
                "x-capability": "task",
                "x-execution": "task",
                "x-risk": "medium",
                "x-approval": {"required": False},
                "requestBody": {
                    "content": {
                        "application/json": {
                            "schema": {
                                "type": "object",
                                "properties": {"title": {"type": "string"}},
                            }
                        }
                    }
                },
            }
        },
    },
}


class RecordingClient:
    """Minimal CroupierClient stand-in that records registrations."""

    def __init__(self) -> None:
        self.registered: List[Tuple[FunctionDescriptor, object]] = []

    def register_function(self, descriptor: FunctionDescriptor, handler) -> None:
        self.registered.append((descriptor, handler))


def _handler(function_id: str):
    def handler(context: str, payload: bytes) -> str:
        return "{}"

    return handler


def _register(client: RecordingClient, options=None):
    handlers = {fid: _handler(fid) for fid in ("player_ban", "leaderboard", "mail_batch_send")}
    return register_from_openapi(client, SPEC, options, handlers=handlers)


def _by_id(client: RecordingClient, function_id: str) -> FunctionDescriptor:
    for descriptor, _ in client.registered:
        if descriptor.id == function_id:
            return descriptor
    raise AssertionError(f"function not registered: {function_id}")


def test_v2_extensions_map_to_descriptor_fields():
    client = RecordingClient()
    _register(client)

    descriptor = _by_id(client, "player_ban")
    assert descriptor.capability == "action"
    assert descriptor.execution == "sync"
    assert descriptor.approval_required is True
    assert descriptor.approval_policy_key == "gm.player.ban"
    assert descriptor.risk == "high"
    assert descriptor.input_schema == {
        "type": "object",
        "required": ["playerId"],
        "properties": {"playerId": {"type": "string"}},
    }
    assert descriptor.output_schema == {"type": "object", "properties": {"ok": {"type": "boolean"}}}


def test_deprecated_risk_aliases_normalize_to_canonical():
    client = RecordingClient()
    _register(client)

    assert _by_id(client, "leaderboard").risk == "safe"
    assert _by_id(client, "mail_batch_send").risk == "warning"


def test_task_execution_and_optional_approval():
    client = RecordingClient()
    _register(client)

    descriptor = _by_id(client, "mail_batch_send")
    assert descriptor.capability == "task"
    assert descriptor.execution == "task"
    assert descriptor.approval_required is False
    assert descriptor.approval_policy_key is None


def test_path_fallback_derives_id_and_summary():
    client = RecordingClient()
    _register(client)

    descriptor = _by_id(client, "leaderboard")
    assert descriptor.id == "leaderboard"
    assert descriptor.summary == "Leaderboard"


def test_invalid_capability_and_execution_are_dropped():
    spec = {
        "paths": {
            "/things": {
                "get": {
                    "operationId": "things_get",
                    "x-capability": "dashboard",
                    "x-execution": "async",
                }
            }
        }
    }
    client = RecordingClient()
    register_from_openapi(client, spec, handlers={"things_get": _handler("t")})

    descriptor = client.registered[0][0]
    assert descriptor.capability is None
    assert descriptor.execution is None


def test_import_options_accept_default_timeout_ms():
    options = ImportOptions(resource_prefix="game", default_timeout_ms=5000, continue_on_error=True)
    client = RecordingClient()
    registered = _register(client, options)

    assert options.default_timeout_ms == 5000
    assert _by_id(client, "player_ban").resource == "game.player"
    assert registered == ["player_ban", "leaderboard", "mail_batch_send"]


def test_continue_on_error_skips_missing_handlers():
    client = RecordingClient()
    options = ImportOptions(continue_on_error=True)
    registered = register_from_openapi(
        client,
        SPEC,
        options,
        handlers={"player_ban": _handler("x")},
    )

    assert registered == ["player_ban"]
    assert [descriptor.id for descriptor, _ in client.registered] == ["player_ban"]


def test_missing_handler_without_continue_on_error_raises():
    client = RecordingClient()
    with pytest.raises(ValueError, match="no handler provided for function: player_ban"):
        register_from_openapi(client, SPEC, handlers={"leaderboard": _handler("l")})
