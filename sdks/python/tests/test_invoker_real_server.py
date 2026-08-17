"""Real Server fixture lifecycle checks for the Python L3 Invoker.

The test is opt-in because it owns no Server process. The repository-level
fixture runner supplies the URL, JWT and scope, and this test exercises only
the public Python Invoker against that real process.
"""

from __future__ import annotations

import os
import time

import pytest

from croupier.invoker import (
    InvokerConfig,
    ServerHTTPError,
    SyncInvoker,
    create_sync_invoker,
)


def _real_invoker() -> SyncInvoker:
    return create_sync_invoker(
        InvokerConfig(
            address=os.environ["CROUPIER_SERVER_URL"],
            auth_token=os.environ["CROUPIER_SERVER_TOKEN"],
            game_id=os.environ.get("CROUPIER_GAME_ID", "e2e-game"),
            env=os.environ.get("CROUPIER_ENV", "e2e"),
            task_poll_interval=0.01,
            timeout=10_000,
        )
    )


def _wait_for_status(invoker: SyncInvoker, task_id: str, expected: str):
    deadline = time.monotonic() + 20
    status = None
    while time.monotonic() < deadline:
        status = invoker.get_task_status(task_id)
        if status.status == expected:
            return status
        time.sleep(0.05)
    raise AssertionError(f"task {task_id} status={status.status if status else None!r}, want {expected!r}")


@pytest.mark.integration
def test_real_server_http_invoker_lifecycle() -> None:
    required = ("CROUPIER_SERVER_URL", "CROUPIER_SERVER_TOKEN")
    if any(not os.environ.get(name) for name in required):
        pytest.skip("real Server fixture variables are not configured")

    unauthenticated = create_sync_invoker(
        InvokerConfig(
            address=os.environ["CROUPIER_SERVER_URL"],
            game_id=os.environ.get("CROUPIER_GAME_ID", "e2e-game"),
            env=os.environ.get("CROUPIER_ENV", "e2e"),
            retry=None,
        )
    )
    try:
        with pytest.raises(ServerHTTPError) as error:
            unauthenticated.invoke("mail.send", '{"player_id":"p-001","title":"denied"}')
        assert error.value.status_code in (401, 403)
    finally:
        unauthenticated.close()

    invoker = _real_invoker()
    try:
        result = invoker.invoke(
            "mail.send",
            '{"player_id":"p-001","title":"Python","content":"body"}',
        )
        assert '"mail_id":"mail-0001"' in result

        completed_id = invoker.start_task(
            "mail.send",
            '{"player_id":"p-001","title":"Python task","content":"body"}',
        )
        completed_events = list(invoker.stream_task(completed_id))
        assert {event.type for event in completed_events} >= {"started", "completed"}
        completed = _wait_for_status(invoker, completed_id, "succeeded")
        assert completed.game_id == invoker._async_invoker.config.game_id
        assert completed.env == invoker._async_invoker.config.env

        cancelled_id = invoker.start_task("mail.wait", '{"wait_ms":30000}')
        _wait_for_status(invoker, cancelled_id, "running")
        invoker.cancel_task(cancelled_id)
        cancelled_events = list(invoker.stream_task(cancelled_id))
        assert any(event.type == "cancelled" for event in cancelled_events)
        assert _wait_for_status(invoker, cancelled_id, "cancelled").task_id == cancelled_id
    finally:
        invoker.close()
