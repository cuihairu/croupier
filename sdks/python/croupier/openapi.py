"""OpenAPI 3.0 import helpers for the Croupier provider SDK.

Mirrors the Go SDK's ``function.RegisterFromOpenAPI``: parses an OpenAPI 3
spec, converts every operation into a ``FunctionDescriptor`` and registers it
on a ``CroupierClient`` with a caller-supplied handler.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Optional, Union

from . import FunctionDescriptor, FunctionHandler

__all__ = ["ImportOptions", "RegisterFromOpenAPI", "register_from_openapi"]

_OPERATION_METHODS = (
    "get",
    "put",
    "post",
    "delete",
    "options",
    "head",
    "patch",
    "trace",
)


@dataclass
class ImportOptions:
    """Controls OpenAPI import behaviour (mirrors Go ImportOptions)."""

    resource_prefix: str = ""
    tag_prefix: str = ""
    continue_on_error: bool = False


def _derive_operation_id(operation: Dict[str, Any], path: str) -> str:
    operation_id = operation.get("operationId")
    if isinstance(operation_id, str) and operation_id:
        return operation_id
    if path:
        segments = [seg for seg in path.strip("/").split("/") if seg != ""]
        if segments:
            return ".".join(segments)
    return "unknown.function"


def _derive_summary(operation: Dict[str, Any], operation_id: str) -> str:
    summary = operation.get("summary")
    if isinstance(summary, str) and summary:
        return summary
    if operation_id and operation_id != "unknown.function":
        return _to_title_case(operation_id)
    return "Unnamed Function"


def _to_title_case(value: str) -> str:
    return " ".join(
        word[:1].upper() + word[1:].lower() if word else word
        for word in value.split("_")
    )


def _schema_to_json_schema(schema: Optional[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    """Shallow OpenAPI-schema -> JSON-Schema conversion (Go parity)."""
    if not isinstance(schema, dict) or not schema:
        return None
    result: Dict[str, Any] = {}
    schema_type = schema.get("type")
    if isinstance(schema_type, str) and schema_type:
        result["type"] = schema_type
    description = schema.get("description")
    if isinstance(description, str) and description:
        result["description"] = description
    properties = schema.get("properties")
    if isinstance(properties, dict) and properties:
        props: Dict[str, Any] = {}
        for name, prop in properties.items():
            if isinstance(prop, dict):
                entry: Dict[str, Any] = {"type": prop.get("type") or "object"}
                if isinstance(prop.get("description"), str) and prop["description"]:
                    entry["description"] = prop["description"]
                props[name] = entry
        result["properties"] = props
    required = schema.get("required")
    if isinstance(required, list) and required:
        result["required"] = required
    return result or None


def _json_content_schema(holder: Optional[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    """Extract the application/json schema from a request-body/response map."""
    if not isinstance(holder, dict):
        return None
    content = holder.get("content")
    if not isinstance(content, dict):
        return None
    media = content.get("application/json")
    if not isinstance(media, dict):
        return None
    return _schema_to_json_schema(media.get("schema"))


def _extract_extension(operation: Dict[str, Any], key: str) -> str:
    value = operation.get(key)
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    if isinstance(value, bool):
        return "true" if value else "false"
    return json.dumps(value, ensure_ascii=False, default=str)


def _parse_risk_level(level: str) -> str:
    normalized = level.lower()
    if normalized in ("low", "safe"):
        return "low"
    if normalized in ("high",):
        return "high"
    if normalized in ("danger", "critical"):
        return "danger"
    return "medium"


def _operation_to_descriptor(
    path: str,
    method: str,
    operation: Dict[str, Any],
    options: Optional[ImportOptions],
) -> FunctionDescriptor:
    function_id = _derive_operation_id(operation, path)
    tags = [tag for tag in operation.get("tags", []) if isinstance(tag, str)]

    descriptor = FunctionDescriptor(
        id=function_id,
        summary=_derive_summary(operation, function_id),
        description=operation.get("description") if isinstance(operation.get("description"), str) else None,
        tags=tags,
        resource=_extract_extension(operation, "x-resource") or None,
        operation=_extract_extension(operation, "x-operation") or None,
        permission=_extract_extension(operation, "x-permission") or None,
    )

    descriptor.input_schema = _json_content_schema(operation.get("requestBody"))
    responses = operation.get("responses")
    if isinstance(responses, dict):
        descriptor.output_schema = _json_content_schema(responses.get("200"))

    risk = _extract_extension(operation, "x-risk")
    if risk:
        descriptor.risk = _parse_risk_level(risk)
    else:
        descriptor.risk = "medium"

    if options is not None:
        if options.resource_prefix and descriptor.resource:
            descriptor.resource = f"{options.resource_prefix}.{descriptor.resource}"
        if options.tag_prefix:
            descriptor.tags = [options.tag_prefix + tag for tag in descriptor.tags]

    return descriptor


def _iter_operations(spec: Dict[str, Any]):
    paths = spec.get("paths")
    if not isinstance(paths, dict):
        return
    for path, path_item in paths.items():
        if not isinstance(path_item, dict):
            continue
        for method in _OPERATION_METHODS:
            operation = path_item.get(method)
            if isinstance(operation, dict):
                yield path, method, operation


def register_from_openapi(
    client,
    spec: Union[str, bytes, Dict[str, Any]],
    options: Optional[ImportOptions] = None,
    handler_resolver: Optional[Callable[[str], Optional[FunctionHandler]]] = None,
    handlers: Optional[Dict[str, FunctionHandler]] = None,
) -> List[str]:
    """Import an OpenAPI 3 spec and register every operation on ``client``.

    Handlers come from either ``handler_resolver`` (callable receiving the
    derived function ID) or the ``handlers`` mapping. Returns the list of
    registered function IDs. Raises ``ValueError`` on invalid specs or when a
    handler is missing (unless ``options.continue_on_error`` is set).
    """
    if isinstance(spec, (str, bytes)):
        try:
            document = json.loads(spec)
        except json.JSONDecodeError as exc:
            raise ValueError(f"load OpenAPI spec failed: {exc}") from exc
    elif isinstance(spec, dict):
        document = spec
    else:
        raise ValueError("spec must be a JSON string, bytes or dict")

    if not isinstance(document, dict) or "paths" not in document:
        raise ValueError("OpenAPI spec must be an object containing 'paths'")

    if handler_resolver is None and handlers is not None:
        handler_resolver = lambda function_id: handlers.get(function_id)  # noqa: E731
    if handler_resolver is None:
        raise ValueError("handler_resolver or handlers must be provided")

    registered: List[str] = []
    for path, method, operation in _iter_operations(document):
        descriptor = _operation_to_descriptor(path, method, operation, options)
        handler = handler_resolver(descriptor.id)
        if handler is None:
            if options is not None and options.continue_on_error:
                continue
            raise ValueError(f"no handler provided for function: {descriptor.id}")
        try:
            client.register_function(descriptor, handler)
        except Exception as exc:  # noqa: BLE001 - mirror Go continue-on-error
            if options is not None and options.continue_on_error:
                continue
            raise ValueError(f"register function {descriptor.id} failed: {exc}") from exc
        registered.append(descriptor.id)
    return registered


# Go-compatible PascalCase alias.
RegisterFromOpenAPI = register_from_openapi
