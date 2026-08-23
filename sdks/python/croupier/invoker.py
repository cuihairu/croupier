"""Independent Python L3 Invoker for the Croupier Server HTTP API.

The provider SDK still owns its TCP session to the Agent for function
registration and execution. This module is deliberately separate: callers use
the Server HTTP API so authorization, scope checks, audit and task persistence
cannot be bypassed.
"""

from __future__ import annotations

import asyncio
import json
import logging
import random
import ssl
from dataclasses import dataclass, field
from typing import Any, AsyncIterator, Dict, List, Optional
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urlsplit, urlunsplit
from urllib.request import Request, urlopen

LOG = logging.getLogger(__name__)

DEFAULT_SERVER_API_URL = "http://127.0.0.1:18780/api/v1"
DEFAULT_TASK_POLL_INTERVAL_SECONDS = 0.5
_MAX_RESPONSE_BYTES = 64 * 1024


@dataclass
class ReconnectConfig:
    """Deprecated compatibility configuration for the former TCP invoker.

    HTTP is request based and has no persistent session to reconnect. The
    value is kept only so existing configuration objects remain constructible.
    """

    enabled: bool = True
    max_attempts: int = 0
    initial_delay_ms: int = 1000
    max_delay_ms: int = 30000
    backoff_multiplier: float = 2.0
    jitter_factor: float = 0.2


def _default_reconnect_config() -> ReconnectConfig:
    return ReconnectConfig()


@dataclass
class RetryConfig:
    """Configuration for retrying retryable Server HTTP requests."""

    enabled: bool = True
    max_attempts: int = 3
    initial_delay_ms: int = 100
    max_delay_ms: int = 5000
    backoff_multiplier: float = 2.0
    jitter_factor: float = 0.1


def _default_retry_config() -> RetryConfig:
    return RetryConfig()


def _calculate_reconnect_delay(attempt: int, config: ReconnectConfig) -> float:
    """Retained for callers that used the legacy configuration helper."""
    delay = config.initial_delay_ms * (config.backoff_multiplier**attempt)
    delay = min(delay, config.max_delay_ms)
    jitter = delay * config.jitter_factor * (random.random() * 2 - 1)
    return max(delay + jitter, 0) / 1000.0


@dataclass
class InvokerConfig:
    """Configuration for the independent Server HTTP invoker."""

    address: str = DEFAULT_SERVER_API_URL
    auth_token: Optional[str] = None
    game_id: Optional[str] = None
    env: Optional[str] = None
    task_poll_interval: float = DEFAULT_TASK_POLL_INTERVAL_SECONDS
    timeout: int = 30000  # milliseconds
    insecure: bool = False
    ca_file: str = ""
    cert_file: str = ""
    key_file: str = ""
    reconnect: ReconnectConfig = field(default_factory=_default_reconnect_config)
    retry: RetryConfig = field(default_factory=_default_retry_config)


@dataclass
class InvokeOptions:
    """Per-request options for function invocation and task creation."""

    idempotency_key: Optional[str] = None
    timeout: Optional[int] = None  # milliseconds
    headers: Optional[Dict[str, str]] = None
    retry: Optional[RetryConfig] = None


@dataclass
class TaskEventInfo:
    """An event returned from the Server task-events endpoint."""

    type: str
    task_id: str
    payload: Optional[str] = None
    message: Optional[str] = None
    progress: Optional[int] = None
    error: Optional[str] = None
    done: bool = False


@dataclass
class TaskStatus:
    """Server-persisted state returned by ``GET /api/v1/tasks/:id``."""

    task_id: str
    status: str
    function_id: Optional[str] = None
    progress: Optional[int] = None
    message: Optional[str] = None
    result: Optional[str] = None
    error: Optional[str] = None
    game_id: Optional[str] = None
    env: Optional[str] = None
    agent_id: Optional[str] = None
    actor: Optional[str] = None
    trace_id: Optional[str] = None
    started_at: Optional[str] = None
    finished_at: Optional[str] = None
    created_at: Optional[str] = None
    updated_at: Optional[str] = None


class ServerHTTPError(RuntimeError):
    """A non-success response returned by the Croupier Server."""

    def __init__(self, status_code: int, message: str):
        self.status_code = status_code
        self.message = message
        super().__init__(f"server returned HTTP {status_code}: {message}")


class Invoker:
    """L3 caller that only uses the Croupier Server HTTP contract.

    ``connect`` is retained for a consistent SDK lifecycle but intentionally
    does not open a Provider-like TCP session. Every business request goes to
    the Server API under ``/api/v1``.
    """

    def __init__(self, config: Optional[InvokerConfig] = None):
        self.config = config or InvokerConfig()
        self._base_url = _normalize_server_api_url(self.config.address)
        self._schemas: Dict[str, Dict[str, Any]] = {}
        self._connected = False
        self._lock: Optional[asyncio.Lock] = None

    async def connect(self) -> None:
        """Mark this request-based Server invoker ready for use."""
        if self._lock is None:
            self._lock = asyncio.Lock()
        async with self._lock:
            self._connected = True

    async def invoke(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        """Call ``POST /api/v1/functions/:id/invoke`` and return raw result JSON."""
        _validate_identifier("function ID", function_id)
        await self._validate_configured_payload(function_id, payload)
        params = _parse_json_payload(payload)
        response = await self._request_json(
            "POST",
            ("functions", function_id, "invoke"),
            {"params": params},
            options,
        )
        if not isinstance(response, dict) or "result" not in response:
            raise ValueError("server invoke response does not contain result")
        return _json_string(response["result"])

    async def start_task(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        """Create a Server task and return the Server-issued task ID."""
        _validate_identifier("function ID", function_id)
        await self._validate_configured_payload(function_id, payload)
        response = await self._request_json(
            "POST",
            ("tasks",),
            {"functionId": function_id, "params": _parse_json_payload(payload)},
            options,
        )
        task_id = response.get("taskId") if isinstance(response, dict) else None
        if not isinstance(task_id, str) or not task_id.strip():
            raise ValueError("server start task response does not contain taskId")
        return task_id

    async def get_task_status(self, task_id: str) -> TaskStatus:
        """Get the current Server-persisted state for a task."""
        _validate_identifier("task ID", task_id)
        response = await self._request_json("GET", ("tasks", task_id), None, None)
        if not isinstance(response, dict):
            raise ValueError("server task status response must be an object")
        result = response.get("result")
        return TaskStatus(
            task_id=_optional_string(response.get("id")) or task_id,
            function_id=_optional_string(response.get("functionId")),
            status=_optional_string(response.get("status")) or "unknown",
            progress=_optional_int(response.get("progress")),
            message=_optional_string(response.get("message")),
            result=None if result is None else _json_string(result),
            error=_optional_string(response.get("error")),
            game_id=_optional_string(response.get("gameId")),
            env=_optional_string(response.get("env")),
            agent_id=_optional_string(response.get("agentId")),
            actor=_optional_string(response.get("actor")),
            trace_id=_optional_string(response.get("traceId")),
            started_at=_optional_string(response.get("startedAt")),
            finished_at=_optional_string(response.get("finishedAt")),
            created_at=_optional_string(response.get("createdAt")),
            updated_at=_optional_string(response.get("updatedAt")),
        )

    async def stream_task(self, task_id: str) -> AsyncIterator[TaskEventInfo]:
        """Poll Server task events until the task is terminal or cancelled."""
        _validate_identifier("task ID", task_id)
        after_seq = 0
        while True:
            response = await self._request_json(
                "GET",
                ("tasks", task_id, "events"),
                None,
                None,
                {"after_seq": str(after_seq)},
            )
            if not isinstance(response, dict):
                raise ValueError("server task events response must be an object")
            items = response.get("items", [])
            if not isinstance(items, list):
                raise ValueError("server task events response items must be an array")

            emitted = False
            for item in items:
                if not isinstance(item, dict):
                    raise ValueError("server task event must be an object")
                seq = _optional_int(item.get("seq"))
                if seq is not None:
                    after_seq = max(after_seq, seq)
                event = _task_event_from_response(task_id, item)
                emitted = True
                yield event

            if response.get("done") is True:
                return
            if not emitted:
                await asyncio.sleep(_task_poll_interval(self.config.task_poll_interval))

    async def cancel_task(self, task_id: str) -> None:
        """Ask the Server to cancel an existing task."""
        _validate_identifier("task ID", task_id)
        await self._request_json("POST", ("tasks", task_id, "cancel"), {}, None)

    async def set_schema(self, function_id: str, schema: Dict[str, Any]) -> None:
        """Set optional local JSON Schema-like required-field validation."""
        _validate_identifier("function ID", function_id)
        self._schemas[function_id] = schema

    async def close(self) -> None:
        """Drop local validation state and mark the invoker closed."""
        if self._lock is None:
            self._lock = asyncio.Lock()
        async with self._lock:
            self._schemas.clear()
            self._connected = False

    async def _validate_configured_payload(self, function_id: str, payload: str) -> None:
        schema = self._schemas.get(function_id)
        if schema is not None:
            self._validate_payload(payload, schema)

    def _validate_payload(self, payload: str, schema: Dict[str, Any]) -> None:
        if not schema:
            return
        value = _parse_json_payload(payload)
        errors = _draft7_validation_errors(value, schema)
        if errors:
            raise ValueError(f"payload validation failed: {'; '.join(errors)}")

    async def _request_json(
        self,
        method: str,
        segments: tuple[str, ...],
        body: Optional[Dict[str, Any]],
        options: Optional[InvokeOptions],
        query: Optional[Dict[str, str]] = None,
    ) -> Any:
        request_options = options or InvokeOptions()
        retry = request_options.retry or self.config.retry
        attempts = retry.max_attempts if retry and retry.enabled else 1
        attempts = max(attempts, 1)
        url = _endpoint_url(self._base_url, segments, query)
        encoded_body = None if body is None else json.dumps(body, separators=(",", ":")).encode("utf-8")

        last_error: Optional[Exception] = None
        for attempt in range(attempts):
            try:
                response_text = await asyncio.to_thread(
                    self._send_request,
                    method,
                    url,
                    encoded_body,
                    self._headers(request_options),
                    _timeout_seconds(request_options.timeout or self.config.timeout),
                )
                try:
                    return json.loads(response_text) if response_text else {}
                except json.JSONDecodeError as exc:
                    raise ValueError(f"server returned invalid JSON: {exc}") from exc
            except Exception as exc:  # Retry only selected transport/server failures below.
                last_error = exc
                if attempt == attempts - 1 or not _is_retryable_error(exc):
                    raise
                await asyncio.sleep(_retry_delay_seconds(attempt, retry))

        raise last_error or RuntimeError("request failed")

    def _headers(self, options: InvokeOptions) -> Dict[str, str]:
        headers = dict(options.headers or {})
        _reject_header_injection(headers)
        normalized = {name.lower(): name for name in headers}
        if options.idempotency_key and "idempotency-key" not in normalized:
            headers["Idempotency-Key"] = options.idempotency_key
        if self.config.game_id and "x-game-id" not in normalized:
            headers["X-Game-ID"] = self.config.game_id
        if self.config.env and "x-env" not in normalized:
            headers["X-Env"] = self.config.env
        if self.config.auth_token and "authorization" not in normalized:
            token = self.config.auth_token.strip()
            if token and not token.lower().startswith("bearer "):
                token = f"Bearer {token}"
            if token:
                headers["Authorization"] = token
        return headers

    def _send_request(
        self,
        method: str,
        url: str,
        body: Optional[bytes],
        headers: Dict[str, str],
        timeout: float,
    ) -> str:
        request_headers = dict(headers)
        if body is not None:
            request_headers.setdefault("Content-Type", "application/json")
        request = Request(url, data=body, headers=request_headers, method=method)
        try:
            with urlopen(request, timeout=timeout, context=self._ssl_context()) as response:
                return response.read(_MAX_RESPONSE_BYTES).decode("utf-8")
        except HTTPError as exc:
            body_text = exc.read(_MAX_RESPONSE_BYTES).decode("utf-8", errors="replace")
            raise ServerHTTPError(exc.code, _server_error_message(body_text)) from exc
        except URLError as exc:
            raise RuntimeError(f"send HTTP request: {exc.reason}") from exc

    def _ssl_context(self) -> Optional[ssl.SSLContext]:
        if urlsplit(self._base_url).scheme != "https":
            return None
        if self.config.insecure:
            return ssl._create_unverified_context()  # nosec B323: explicit SDK configuration
        context = ssl.create_default_context(cafile=self.config.ca_file or None)
        if self.config.cert_file or self.config.key_file:
            context.load_cert_chain(self.config.cert_file, self.config.key_file or None)
        return context


def default_invoker_config() -> InvokerConfig:
    """Create default Server HTTP invoker configuration."""
    return InvokerConfig()


def create_invoker(config: Optional[InvokerConfig] = None) -> Invoker:
    """Create the asynchronous Server HTTP invoker."""
    return Invoker(config)


class SyncInvoker:
    """Blocking wrapper around the asynchronous Server HTTP invoker."""

    def __init__(self, config: Optional[InvokerConfig] = None):
        self._async_invoker = Invoker(config)
        self._loop: Optional[asyncio.AbstractEventLoop] = None

    def _get_loop(self) -> asyncio.AbstractEventLoop:
        try:
            loop = asyncio.get_event_loop()
            if loop.is_running():
                raise RuntimeError("running event loop detected; use the asynchronous Invoker")
            return loop
        except RuntimeError:
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
            return loop

    def connect(self) -> None:
        self._get_loop().run_until_complete(self._async_invoker.connect())

    def invoke(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        return self._get_loop().run_until_complete(
            self._async_invoker.invoke(function_id, payload, options)
        )

    def start_task(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        return self._get_loop().run_until_complete(
            self._async_invoker.start_task(function_id, payload, options)
        )

    def get_task_status(self, task_id: str) -> TaskStatus:
        return self._get_loop().run_until_complete(self._async_invoker.get_task_status(task_id))

    def stream_task(self, task_id: str):
        loop = self._get_loop()
        async_generator = self._async_invoker.stream_task(task_id)

        class SyncIterator:
            def __iter__(self) -> "SyncIterator":
                return self

            def __next__(self) -> TaskEventInfo:
                try:
                    return loop.run_until_complete(async_generator.__anext__())
                except StopAsyncIteration as exc:
                    raise StopIteration from exc

        return SyncIterator()

    def cancel_task(self, task_id: str) -> None:
        self._get_loop().run_until_complete(self._async_invoker.cancel_task(task_id))

    def set_schema(self, function_id: str, schema: Dict[str, Any]) -> None:
        self._async_invoker._schemas[function_id] = schema

    def close(self) -> None:
        self._get_loop().run_until_complete(self._async_invoker.close())


def create_sync_invoker(config: Optional[InvokerConfig] = None) -> SyncInvoker:
    """Create the blocking Server HTTP invoker."""
    return SyncInvoker(config)


def _draft7_validation_errors(value: Any, schema: Dict[str, Any]) -> List[str]:
    """Validate ``value`` against ``schema`` using JSON Schema Draft 7."""
    try:
        from jsonschema import Draft7Validator
    except ImportError:  # pragma: no cover - jsonschema is a declared dependency
        return _legacy_required_validation_errors(value, schema)
    validator = Draft7Validator(schema)
    messages = []
    for error in sorted(validator.iter_errors(value), key=lambda e: list(e.absolute_path)):
        location = "$".join(str(part) for part in error.absolute_path) or "root"
        messages.append(f"{location}: {error.message}")
    return messages


def _legacy_required_validation_errors(value: Any, schema: Dict[str, Any]) -> List[str]:
    if not isinstance(value, dict):
        return ["root: expected a JSON object"]
    required = schema.get("required", [])
    if isinstance(required, list):
        return [
            f"root: missing required field '{field_name}'"
            for field_name in required
            if field_name not in value
        ]
    return []


def _normalize_server_api_url(address: str) -> str:
    candidate = (address or "").strip() or DEFAULT_SERVER_API_URL
    if "://" not in candidate:
        candidate = f"http://{candidate}"
    parsed = urlsplit(candidate)
    if parsed.scheme not in ("http", "https") or not parsed.netloc:
        raise ValueError("InvokerConfig.address must be an HTTP(S) Server address")
    path = parsed.path.rstrip("/")
    if not path.endswith("/api/v1"):
        path = f"{path}/api/v1" if path else "/api/v1"
    return urlunsplit((parsed.scheme, parsed.netloc, path, "", ""))


def _endpoint_url(
    base_url: str, segments: tuple[str, ...], query: Optional[Dict[str, str]]
) -> str:
    parsed = urlsplit(base_url)
    path = "/".join([parsed.path.rstrip("/"), *(quote(segment, safe="") for segment in segments)])
    return urlunsplit((parsed.scheme, parsed.netloc, path, urlencode(query or {}), ""))


def _parse_json_payload(payload: str) -> Any:
    if not payload or not payload.strip():
        return {}
    try:
        return json.loads(payload)
    except json.JSONDecodeError as exc:
        raise ValueError(f"payload must be valid JSON: {exc}") from exc


def _validate_identifier(name: str, value: str) -> None:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} cannot be empty")


def _task_event_from_response(task_id: str, item: Dict[str, Any]) -> TaskEventInfo:
    event_type = _optional_string(item.get("type")) or "unknown"
    if event_type == "done":
        event_type = "completed"
    message = _optional_string(item.get("message"))
    payload = item.get("payload")
    return TaskEventInfo(
        type=event_type,
        task_id=task_id,
        payload=None if payload is None else _json_string(payload),
        message=message,
        progress=_optional_int(item.get("progress")),
        error=message if event_type in ("error", "failed", "cancelled", "timed_out") else None,
        done=event_type in ("completed", "error", "failed", "cancelled", "timed_out"),
    )


def _optional_string(value: Any) -> Optional[str]:
    return value if isinstance(value, str) and value else None


def _optional_int(value: Any) -> Optional[int]:
    return value if isinstance(value, int) and not isinstance(value, bool) else None


def _json_string(value: Any) -> str:
    return json.dumps(value, separators=(",", ":"), ensure_ascii=False)


def _task_poll_interval(interval: float) -> float:
    return interval if interval > 0 else DEFAULT_TASK_POLL_INTERVAL_SECONDS


def _timeout_seconds(timeout_ms: int) -> float:
    return max(timeout_ms, 1) / 1000.0


def _retry_delay_seconds(attempt: int, retry: Optional[RetryConfig]) -> float:
    if retry is None:
        return 0
    multiplier = retry.backoff_multiplier if retry.backoff_multiplier > 0 else 2.0
    delay = retry.initial_delay_ms * (multiplier**attempt)
    if retry.max_delay_ms > 0:
        delay = min(delay, retry.max_delay_ms)
    if retry.jitter_factor > 0:
        delay += delay * retry.jitter_factor * (random.random() * 2 - 1)
    return max(delay, 0) / 1000.0


def _is_retryable_error(error: Exception) -> bool:
    if isinstance(error, ServerHTTPError):
        return error.status_code == 429 or error.status_code >= 500
    return isinstance(error, RuntimeError)


def _server_error_message(body: str) -> str:
    try:
        payload = json.loads(body)
    except json.JSONDecodeError:
        return body.strip() or "empty response body"
    if isinstance(payload, dict):
        for key in ("message", "error"):
            value = payload.get(key)
            if isinstance(value, str) and value.strip():
                return value
    return body.strip() or "empty response body"


def _reject_header_injection(headers: Dict[str, str]) -> None:
    for name, value in headers.items():
        if "\r" in name or "\n" in name or "\r" in value or "\n" in value:
            raise ValueError("HTTP header values cannot contain CR or LF")


__all__ = [
    "DEFAULT_SERVER_API_URL",
    "ReconnectConfig",
    "RetryConfig",
    "InvokerConfig",
    "InvokeOptions",
    "TaskEventInfo",
    "TaskStatus",
    "ServerHTTPError",
    "Invoker",
    "SyncInvoker",
    "default_invoker_config",
    "create_invoker",
    "create_sync_invoker",
]
