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

## 简介

Croupier Python SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 Python 客户端实现。SDK 作为 **Provider 端被调用方**，通过 **单条 TCP session**（`sdk-agent subprotocol`）与 Agent 通信——不监听本地端口。

## 正式文档

- 功能矩阵（跨语言一致性的单一事实来源）：[`sdks/SDK_FEATURE_MATRIX.md`](../SDK_FEATURE_MATRIX.md)
- 线协议约定：[`docs/architecture/sdk-wire-protocol.md`](../../docs/architecture/sdk-wire-protocol.md)
- 统一文档站入口：`/docs/sdks/python/`
- 仓库内路径：`docs/sdks/python`

## 主项目

| 项目         | 描述               | 链接                                                      |
| ------------ | ------------------ | --------------------------------------------------------- |
| **Croupier** | 游戏后端平台主项目 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言  | 目录                                                                       | CI                                                                                                                                                                    | Docs                          |
| ----- | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| Go    | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml)         | [README](../go/README.md)     |
| C++   | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp)       | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml)       | [README](../cpp/README.md)    |
| Java  | [sdks/java/](https://github.com/cuihairu/croupier/tree/main/sdks/java)     | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml)     | [README](../java/README.md)   |
| JS/TS | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml)         | [README](../js/README.md)     |
| C#    | [sdks/csharp/](https://github.com/cuihairu/croupier/tree/main/sdks/csharp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](../csharp/README.md) |

## 核心特性

按 [功能矩阵](../SDK_FEATURE_MATRIX.md) 分层：

**L1 Core Provider（必备）**

- 单条 TCP 连接，多路复用请求/响应（`sdk-agent subprotocol`）
- 自动心跳与断线重连（指数退避 + jitter）
- 函数注册 + handler 签名 `(context: str, payload: bytes) -> str | bytes`
- Provider drain 处理

**L2 Provider 扩展（可选）**

- 内置 TLS（`cert_file` / `key_file` / `ca_file` / `server_name`）
- 文件传输（`enable_file_transfer=true`）
- JSON Schema 校验

**L3 Invoker（独立调用方）**

- `croupier.invoker.Invoker`（`croupier/invoker.py`）仅调用 Server HTTP API，独立于 Provider TCP session
- 覆盖 `POST /api/v1/functions/:id/invoke`、任务创建、查询、事件轮询与取消；鉴权、scope、审计和任务持久化由 Server 执行

### L3 Invoker 快速示例

```python
from croupier import InvokerConfig, InvokeOptions, create_invoker

invoker = create_invoker(InvokerConfig(
    address="https://server.example/api/v1",
    auth_token="server-access-token",
    game_id="game-a",
    env="production",
))

result = await invoker.invoke("player.ban", '{"playerId":"p-1"}', InvokeOptions(
    idempotency_key="ban-p-1-20260817",
))
task_id = await invoker.start_task("report.generate", '{"range":"daily"}')
status = await invoker.get_task_status(task_id)
```

`connect()` 只标记 HTTP Invoker 就绪，不会创建 Provider TCP session。完整示例见
[`examples/invoker_example.py`](examples/invoker_example.py)。

## 支持平台

| 平台        | 架构                       | 状态    |
| ----------- | -------------------------- | ------- |
| **Windows** | x64                        | ✅ 支持 |
| **Linux**   | x64, ARM64                 | ✅ 支持 |
| **macOS**   | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 系统要求

- **Python** >= 3.12
- **protobuf**

## 安装

### pip

```bash
python -m pip install -e .
```

### uv（推荐）

```bash
pip install uv
uv sync --dev --all-extras
```

## 快速开始

```python
import json
from croupier import CroupierClient, ClientConfig, FunctionDescriptor

config = ClientConfig(
    agent_addr="127.0.0.1:19091",
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

### 完整游戏后台 Demo

`examples/game_demo.py` 包含19个业务动作函数（player/order/leaderboard/inventory/mail），与 Go SDK demo 功能对齐：

```bash
cd sdks/python
uv run python examples/game_demo.py
```

环境变量：`CROUPIER_AGENT_ADDR`（默认 `127.0.0.1:19091`）、`CROUPIER_GAME_ID`、`CROUPIER_ENV`。

## 架构设计

SDK 是 `sdk-agent subprotocol` 上的 Provider session 客户端，单条 TCP 连接上完成：

1. **握手**：SDK 发送 `ProviderConnectRequest`（函数描述符 + 能力声明），接收 `ProviderConnectResponse(session_id)`
2. **心跳**：周期性发送 `ProviderHeartbeatRequest` 保持会话活跃
3. **调用**：Agent 在同一连接上推送 `InvokeRequest`，SDK 调用 handler 并回 `InvokeResponse`
4. **作业**：Agent 发送 `StartTaskRequest`，SDK 异步处理并回流 `TaskEvent`
5. **Drain**：收到 `ProviderDrainRequest` 时停止接收新请求、完成在途、回 `ProviderDrainResponse`

不监听本地端口，不依赖回调模型，不依赖外部 TCP 库。详见 [`docs/architecture/sdk-wire-protocol.md`](../../docs/architecture/sdk-wire-protocol.md)。

## 开发指南

```bash
uv sync --dev --all-extras
uv run pytest
uv run python examples/main.py
```

## 许可证

[Apache License 2.0](LICENSE)
