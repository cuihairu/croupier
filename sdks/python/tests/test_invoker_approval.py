"""AUD-069: pending-approval responses must be preserved, not mis-decoded."""

from __future__ import annotations

import asyncio
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any, Dict, Tuple

from croupier.invoker import Invoker, InvokerConfig, PendingApprovalError


class ApprovalServer:
    def __init__(self, body: Dict[str, Any]):
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self) -> None:  # noqa: N802
                raw = outer.body_bytes

                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def log_message(self, _format: str, *_args: object) -> None:
                return

        import json

        self.body_bytes = json.dumps(body).encode("utf-8")
        self._server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)

    @property
    def address(self) -> str:
        return f"http://127.0.0.1:{self._server.server_port}"

    def __enter__(self) -> "ApprovalServer":
        self._thread.start()
        return self

    def __exit__(self, _exc_type: object, _exc: object, _traceback: object) -> None:
        self._server.shutdown()
        self._server.server_close()
        self._thread.join()


def test_invoke_pending_approval_is_preserved() -> None:
    body = {
        "taskId": "",
        "approvalId": "apr-1",
        "approvalRequired": True,
        "approvalWorkflow": "two_person",
        "traceId": "tr-1",
    }
    with ApprovalServer(body) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def call() -> None:
            try:
                await invoker.invoke("player.ban", "{}")
            except PendingApprovalError as error:
                assert error.approval_id == "apr-1"
                assert error.approval_workflow == "two_person"
                assert "apr-1" in str(error)
            else:
                raise AssertionError("expected PendingApprovalError")

        asyncio.run(call())


def test_start_task_pending_approval_is_preserved() -> None:
    body = {
        "taskId": "",
        "status": "approval_required",
        "approvalId": "apr-2",
        "approvalWorkflow": "single_admin",
    }
    with ApprovalServer(body) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def call() -> None:
            try:
                await invoker.start_task("player.ban", "{}")
            except PendingApprovalError as error:
                assert error.approval_id == "apr-2"
                assert error.status == "approval_required"
                assert error.approval_workflow == "single_admin"
            else:
                raise AssertionError("expected PendingApprovalError")

        asyncio.run(call())


def test_missing_approval_id_stays_missing_outcome() -> None:
    body = {"taskId": "", "status": "approval_required", "approvalWorkflow": "single_admin"}
    with ApprovalServer(body) as server:
        invoker = Invoker(InvokerConfig(address=server.address))

        async def call() -> None:
            try:
                await invoker.start_task("player.ban", "{}")
            except PendingApprovalError:
                raise AssertionError("must not synthesize approval without an ID")
            except ValueError as error:
                assert "taskId" in str(error)
            else:
                raise AssertionError("expected missing taskId error")

        asyncio.run(call())
