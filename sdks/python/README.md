# Croupier Python SDK

Synchronous Python client for hosting Croupier functions and registering them with a nearby agent.

## Features

- 🛰️ Starts a local gRPC `FunctionService` server and exposes your handlers to the platform.
- 🔄 Registers with the agent (`LocalControlService.RegisterLocal`) and maintains heartbeats.
- 🧱 Thread-safe job state, including streaming events and cancellation notifications.
- 📤 Uploads provider manifests via `ControlService.RegisterCapabilities` when `control_addr` is configured.
- 🧪 Includes an end-to-end example under `examples/main.py`.

## Requirements

- Python ≥ 3.8
- `grpcio`, `protobuf`

Install dependencies from the SDK directory:

```bash
cd sdks/python
python -m venv .venv
source .venv/bin/activate
python -m pip install -U pip
python -m pip install -e .
```

## Quick Start

```python
import json
from croupier import CroupierClient, ClientConfig, FunctionDescriptor

config = ClientConfig(
    agent_addr="127.0.0.1:19090",
    control_addr="127.0.0.1:18080",
    service_id="python-demo",
    service_version="1.0.0",
)

client = CroupierClient(config)

def player_ban(_context: str, payload: bytes) -> str:
    req = json.loads(payload.decode("utf-8"))
    return json.dumps({
        "status": "ok",
        "player_id": req["player_id"],
    })

client.register_function(FunctionDescriptor(id="player.ban", version="1.0.0"), player_ban)
client.connect()
print("✅ python-demo registered and serving gRPC traffic")
```

Press `Ctrl+C` to exit and the client will gracefully disconnect.

## Example

```
cd sdks/python
python examples/main.py
```

The example registers two handlers (`player.ban` and `server.status`) and keeps the process alive until interrupted.

## Project Layout

```
sdks/python/
├── croupier/          # SDK implementation
├── examples/          # Working sample
├── generated/         # Protobuf/gRPC bindings
├── setup.py
└── README.md
```

## Roadmap

- Async/asyncio client
- File transfer helpers for server hot reload

Contributions welcome! Feel free to open issues or PRs. 💡
