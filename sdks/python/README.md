<p align="center">
  <h1 align="center">Croupier Python SDK</h1>
  <p align="center">
    <strong>Python SDK for Croupier Game Function Registration and Invocation</strong>
  </p>
</p>

<p align="center">
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://www.python.org/">
    <img src="https://img.shields.io/badge/Python-3.12+-3776AB.svg" alt="Python Version">
  </a>
</p>

<p align="center">
  <a href="#">
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg" alt="Platform">
  </a>
  <a href="https://github.com/cuihairu/croupier">
    <img src="https://img.shields.io/badge/Main%20Project-Croupier-green.svg" alt="Main Project">
  </a>
</p>

---

## Introduction

Croupier Python SDK is the official Python client for the [Croupier](https://github.com/cuihairu/croupier) game backend platform. It connects to the Agent via a **single bidirectional TCP session** — the SDK is a session client (no local port listening).

Key features:
- Single TCP connection with multiplexed request/response
- Automatic heartbeat and reconnection
- Synchronous function invocation and asynchronous job execution
- Built-in TLS support
- Zero external TCP dependency

## Requirements

- **Python** >= 3.12
- **protobuf**

## Installation

### pip

```bash
python -m pip install -e .
```

### uv (recommended)

```bash
pip install uv
uv sync --dev --all-extras
```

## Quick Start

```python
import json
from croupier import CroupierClient, ClientConfig, FunctionDescriptor

config = ClientConfig(
    agent_addr="127.0.0.1:19090",
    service_id="python-demo",
    service_version="1.0.0",
)

client = CroupierClient(config)

def player_ban(_context: str, payload: bytes) -> str:
    req = json.loads(payload.decode("utf-8"))
    return json.dumps({"status": "ok", "player_id": req["player_id"]})

client.register_function(FunctionDescriptor(id="player.ban", version="1.0.0"), player_ban)
client.connect()
print("Connected — handling invocations from agent")
```

## Architecture

The SDK connects to the Agent's local TCP gateway. Over a single connection:

1. **Handshake**: SDK sends `ProviderConnectRequest` (function descriptors), receives `ProviderConnectResponse` (session ID)
2. **Heartbeat**: SDK sends periodic `HeartbeatRequest` to keep the session alive
3. **Invocation**: Agent pushes `InvokeRequest` to SDK on the same connection; SDK responds inline
4. **Jobs**: Agent sends `StartJobRequest`; SDK processes asynchronously and streams events back

No local port listening. No callback model. No TCP dependency.

## Development

```bash
uv sync --dev --all-extras
uv run pytest
uv run python examples/main.py
```

## Other Language SDKs

| Language | Repository |
| --- | --- |
| Go | [croupier-sdk-go](https://github.com/cuihairu/croupier-sdk-go) |
| C++ | [croupier-sdk-cpp](https://github.com/cuihairu/croupier-sdk-cpp) |
| Java | [croupier-sdk-java](https://github.com/cuihairu/croupier-sdk-java) |
| JS/TS | [croupier-sdk-js](https://github.com/cuihairu/croupier-sdk-js) |
| C# | [croupier-sdk-csharp](https://github.com/cuihairu/croupier-sdk-csharp) |

## License

[Apache License 2.0](LICENSE)
