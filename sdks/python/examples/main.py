"""
Simple end-to-end demo for the synchronous Croupier Python SDK.
"""

from __future__ import annotations

import json
import signal
import sys
import time
from typing import Callable

from croupier import ClientConfig, CroupierClient, FunctionDescriptor


def player_ban(_context: str, payload: bytes) -> str:
    request = json.loads(payload.decode("utf-8") or "{}")
    return json.dumps(
        {
            "status": "success",
            "player_id": request.get("player_id"),
            "reason": request.get("reason", "unspecified"),
            "banned_at": time.time(),
        }
    )


def server_status(_context: str, _payload: bytes) -> str:
    return json.dumps(
        {
            "status": "running",
            "uptime_seconds": time.time(),
            "timestamp": time.time(),
        }
    )


def main() -> None:
    config = ClientConfig(
        agent_addr="127.0.0.1:19090",
        service_id="python-example",
        service_version="1.0.0",
    )
    client = CroupierClient(config)

    client.register_function(
        FunctionDescriptor(
            id="player.ban",
            version="1.0.0",
            category="moderation",
            risk="high",
            entity="player",
            operation="update",
        ),
        player_ban,
    )
    client.register_function(
        FunctionDescriptor(id="server.status", version="1.0.0"), server_status
    )

    client.connect()
    print("Connected to agent at 127.0.0.1:19090")
    print("Press Ctrl+C to stop.\n")

    def _shutdown(_sig, _frame):
        print("\nStopping...")
        client.disconnect()
        sys.exit(0)

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        _shutdown(None, None)


if __name__ == "__main__":
    main()
