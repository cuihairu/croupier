"""Mock HTTP contract tests for the independent Python L3 Invoker."""

from __future__ import annotations

import asyncio
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any, Callable, Dict, List, Tuple

import pytest

from croupier.invoker import (
    DEFAULT_SERVER_API_URL,
    Invoker,
    InvokerConfig,
    InvokeOptions,
    RetryConfig,
    ServerHTTPError,
    SyncInvoker,
    TaskEventInfo,
    TaskStatus,
    create_invoker,
    create_sync_invoker,
    default_invoker_config,
)


RecordedRequest = Tuple[str, str, Dict[str, str], Any]
Responder = Callable[[str, str, Dict[str, str], Any], Tuple[int, Any]]


def header(headers: Dict[str, str], name: str) -> str:
    return next(value for key, value in headers.items() if key.lower() == name.lower())


class MockServer:
    def __init__(self, responder: Responder):
        self.requests: List[RecordedRequest] = []
        self._responder = responder
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self) -> None:  # noqa: N802
                self._respond()

            def do_POST(self) -> None:  # noqa: N802
                self._respond()

            def _respond(self) -> None:
                length = int(self.headers.get("Content-Length", "0"))
                body = self.rfile.read(length) if length else b""
                payload = json.loads(body) if body else None
                headers = {name: value for name, value in self.headers.items()}
                outer.requests.append((self.command, self.path, headers, payload))
                status, response = outer._responder(self.command, self.path, headers, payload)
                raw = json.dumps(response, separators=(",", ":")).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def log_message(self, _format: str, *_args: object) -> None:
                return

        self._server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)

    @property
    def address(self) -> str:
        return f"http://127.0.0.1:{self._server.server_port}"

    def __enter__(self) -> "MockServer":
        self._thread.start()
        return self

    def __exit__(self, _exc_type: object, _exc: object, _traceback: object) -> None:
        self._server.shutdown()
        self._server.server_close()
        self._thread.join()


def run(coroutine: Any) -> Any:
    return asyncio.run(coroutine)


def test_invoker_defaults_to_server_http() -> None:
    config = InvokerConfig()
    invoker = Invoker()

    assert config.address == DEFAULT_SERVER_API_URL
    assert invoker._base_url == DEFAULT_SERVER_API_URL
    assert invoker._connected is False
    assert invoker._schemas == {}


def test_invoker_normalizes_server_root_and_host_port() -> None:
    assert Invoker(InvokerConfig(address="https://server.example"))._base_url == "https://server.example/api/v1"
    assert Invoker(InvokerConfig(address="server.example:18780"))._base_url == "http://server.example:18780/api/v1"
    assert Invoker(InvokerConfig(address="https://server.example/custom"))._base_url == "https://server.example/custom/api/v1"


def test_invoker_rejects_non_http_address() -> None:
    with pytest.raises(ValueError, match="HTTP"):
        Invoker(InvokerConfig(address="tcp://127.0.0.1:19090"))


def test_invoke_uses_server_contract_headers_and_result() -> None:
    def responder(method: str, path: str, headers: Dict[str, str], body: Any) -> Tuple[int, Any]:
        assert method == "POST"
        assert path == "/api/v1/functions/player.ban/invoke"
        assert header(headers, "Authorization") == "Bearer server-token"
        assert header(headers, "X-Game-ID") == "game-a"
        assert header(headers, "X-Env") == "staging"
        assert header(headers, "Idempotency-Key") == "invoke-1"
        assert body == {"params": {"playerId": "p-1"}}
        return 200, {"result": {"status": "banned"}}

    with MockServer(responder) as server:
        invoker = Invoker(InvokerConfig(
            address=server.address,
            auth_token="server-token",
            game_id="game-a",
            env="staging",
        ))
        result = run(invoker.invoke("player.ban", '{"playerId":"p-1"}', InvokeOptions(idempotency_key="invoke-1")))

    assert result == '{"status":"banned"}'


def test_request_headers_can_override_configured_authorization_and_scope() -> None:
    def responder(_method: str, _path: str, headers: Dict[str, str], _body: Any) -> Tuple[int, Any]:
        assert header(headers, "Authorization") == "Bearer explicit-token"
        assert header(headers, "X-Game-ID") == "request-game"
        assert header(headers, "X-Env") == "production"
        return 200, {"result": "ok"}

    with MockServer(responder) as server:
        invoker = Invoker(InvokerConfig(address=server.address, auth_token="configured-token", game_id="default-game", env="dev"))
        result = run(invoker.invoke("health.check", "{}", InvokeOptions(headers={
            "Authorization": "Bearer explicit-token",
            "X-Game-ID": "request-game",
            "X-Env": "production",
        })))

    assert result == '"ok"'


def test_task_lifecycle_uses_server_endpoints() -> None:
    event_calls = 0

    def responder(method: str, path: str, _headers: Dict[str, str], body: Any) -> Tuple[int, Any]:
        nonlocal event_calls
        if method == "POST" and path == "/api/v1/tasks":
            assert body == {"functionId": "report.generate", "params": {"range": "daily"}}
            return 200, {"taskId": "server-task-42", "status": "dispatching"}
        if method == "GET" and path == "/api/v1/tasks/server-task-42":
            return 200, {
                "id": "server-task-42", "functionId": "report.generate", "status": "running",
                "progress": 50, "message": "halfway", "result": {"partial": True},
            }
        if method == "GET" and path == "/api/v1/tasks/server-task-42/events?after_seq=0":
            event_calls += 1
            return 200, {"items": [{"seq": 1, "type": "progress", "progress": 50, "message": "halfway", "payload": {"count": 1}}], "done": False}
        if method == "GET" and path == "/api/v1/tasks/server-task-42/events?after_seq=1":
            event_calls += 1
            return 200, {"items": [{"seq": 2, "type": "completed", "payload": {"ok": True}}], "done": True}
        if method == "POST" and path == "/api/v1/tasks/server-task-42/cancel":
            assert body == {}
            return 200, {"message": "accepted"}
        return 404, {"message": f"unexpected {method} {path}"}

    async def exercise(invoker: Invoker) -> Tuple[str, TaskStatus, List[TaskEventInfo]]:
        task_id = await invoker.start_task("report.generate", '{"range":"daily"}')
        status = await invoker.get_task_status(task_id)
        events = [event async for event in invoker.stream_task(task_id)]
        await invoker.cancel_task(task_id)
        return task_id, status, events

    with MockServer(responder) as server:
        task_id, status, events = run(exercise(Invoker(InvokerConfig(address=server.address, task_poll_interval=0.001))))

    assert task_id == "server-task-42"
    assert status == TaskStatus(task_id="server-task-42", function_id="report.generate", status="running", progress=50, message="halfway", result='{"partial":true}')
    assert [(event.type, event.payload, event.done) for event in events] == [
        ("progress", '{"count":1}', False),
        ("completed", '{"ok":true}', True),
    ]
    assert event_calls == 2


def test_start_task_rejects_missing_server_task_id() -> None:
    with MockServer(lambda *_args: (200, {"status": "dispatching"})) as server:
        with pytest.raises(ValueError, match="taskId"):
            run(Invoker(InvokerConfig(address=server.address)).start_task("report.generate", "{}"))


def test_invalid_payload_schema_and_identifiers_never_send_requests() -> None:
    with MockServer(lambda *_args: pytest.fail("request must not be sent")) as server:
        invoker = Invoker(InvokerConfig(address=server.address))
        with pytest.raises(ValueError, match="function ID"):
            run(invoker.invoke(" ", "{}"))
        with pytest.raises(ValueError, match="valid JSON"):
            run(invoker.start_task("report.generate", "not-json"))
        run(invoker.set_schema("report.generate", {"required": ["range"]}))
        with pytest.raises(ValueError, match="payload validation"):
            run(invoker.invoke("report.generate", "{}"))
        with pytest.raises(ValueError, match="task ID"):
            run(invoker.get_task_status(""))


def test_server_error_and_removed_legacy_path_are_reported() -> None:
    def responder(_method: str, path: str, _headers: Dict[str, str], _body: Any) -> Tuple[int, Any]:
        assert "/api/function/" not in path
        return 403, {"message": "scope denied"}

    with MockServer(responder) as server:
        invoker = Invoker(InvokerConfig(address=server.address, retry=RetryConfig(enabled=False)))
        with pytest.raises(ServerHTTPError, match="scope denied"):
            run(invoker.invoke("player.ban", "{}"))


def test_retry_repeats_retryable_server_failures() -> None:
    attempts = 0

    def responder(_method: str, _path: str, _headers: Dict[str, str], _body: Any) -> Tuple[int, Any]:
        nonlocal attempts
        attempts += 1
        if attempts < 3:
            return 503, {"message": "temporarily unavailable"}
        return 200, {"result": {"ok": True}}

    with MockServer(responder) as server:
        result = run(Invoker(InvokerConfig(address=server.address, retry=RetryConfig(
            max_attempts=3, initial_delay_ms=0, max_delay_ms=0, jitter_factor=0,
        ))).invoke("health.check", "{}"))

    assert result == '{"ok":true}'
    assert attempts == 3


def test_connect_close_and_factories() -> None:
    invoker = create_invoker()
    assert isinstance(invoker, Invoker)
    assert default_invoker_config().address == DEFAULT_SERVER_API_URL
    run(invoker.connect())
    assert invoker._connected is True
    run(invoker.close())
    assert invoker._connected is False

    sync_invoker = create_sync_invoker(InvokerConfig(address="server.example:18780"))
    assert isinstance(sync_invoker, SyncInvoker)
    assert sync_invoker._async_invoker._base_url == "http://server.example:18780/api/v1"
    sync_invoker.connect()
    sync_invoker.close()


def test_sync_invoker_queries_task_status() -> None:
    with MockServer(lambda method, path, _headers, _body: (
        (200, {"id": "task-1", "status": "succeeded", "result": {"ok": True}})
        if method == "GET" and path == "/api/v1/tasks/task-1"
        else (404, {"message": "unexpected"})
    )) as server:
        invoker = SyncInvoker(InvokerConfig(address=server.address))
        status = invoker.get_task_status("task-1")
        invoker.close()

    assert status.task_id == "task-1"
    assert status.status == "succeeded"
    assert status.result == '{"ok":true}'
