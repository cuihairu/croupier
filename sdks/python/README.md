<p align="center">
  <h1 align="center">Croupier Python SDK</h1>
  <p align="center">
    <strong>Python SDK for Croupier Game Function Registration and Invocation</strong>
  </p>
</p>

<p align="center">
  <a href="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml">
    <img src="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg" alt="CI">
  </a>
  <a href="https://codecov.io/gh/cuihairu/croupier">
    <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=python-sdk" alt="Coverage">
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://www.python.org/">
    <img src="https://img.shields.io/badge/Python-3.12+-3776AB.svg" alt="Python Version">
  </a>
</p>

<p align="center">
  <a href="#支持平台">
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg" alt="Platform">
  </a>
  <a href="https://github.com/cuihairu/croupier">
    <img src="https://img.shields.io/badge/Main%20Project-Croupier-green.svg" alt="Main Project">
  </a>
</p>

---

## Introduction

Croupier Python SDK is the official Python client for the [Croupier](https://github.com/cuihairu/croupier) game backend platform. It connects to the Agent via a **single bidirectional TCP session** — the SDK is a session client (no local port listening).

## Documentation

- Unified docs site entry: `/docs/sdks/python/`
- In-repo path: `docs/sdks/python`

## 主项目

| 项目 | 描述 | 链接 |
|------|------|------|
| **Croupier** | 游戏后端平台主项目 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言 | 目录 | CI | Docs |
| --- | --- | --- | --- |
| Go | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml) | [README](sdks/go/README.md) |
| C++ | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml) | [README](sdks/cpp/README.md) |
| Java | [sdks/java/](https://github.com/cuihairu/croupier/tree/main/sdks/java) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml) | [README](sdks/java/README.md) |
| JS/TS | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml) | [README](sdks/js/README.md) |
| C# | [sdks/csharp/](https://github.com/cuihairu/croupier/tree/main/sdks/csharp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](sdks/csharp/README.md) |

## 核心特性

- Single TCP connection with multiplexed request/response
- Automatic heartbeat and reconnection
- Synchronous function invocation and asynchronous job execution
- Built-in TLS support
- Zero external TCP dependency

## 支持平台

| 平台 | 架构 | 状态 |
|------|------|------|
| **Windows** | x64 | ✅ 支持 |
| **Linux** | x64, ARM64 | ✅ 支持 |
| **macOS** | x64, ARM64 (Apple Silicon) | ✅ 支持 |

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
