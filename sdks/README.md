# Croupier SDKs

本目录包含 Croupier 的官方多语言 SDK，作为 monorepo 的一部分维护。

## 设计基线

所有 SDK 共享：

- 主仓库根目录的 **Protobuf 协议**：[`proto/croupier/sdk/v1/`](../proto/croupier/sdk/v1/)
- **线协议约定**：[`docs/architecture/sdk-wire-protocol.md`](../docs/architecture/sdk-wire-protocol.md)
- **功能点对照单一事实来源**：[`SDK_FEATURE_MATRIX.md`](./SDK_FEATURE_MATRIX.md)

`sdk-agent subprotocol` 模型：单条 TCP 长连接、4 字节 FrameLength + 8 字节 Header + protobuf body、首帧 `ProviderConnectRequest`、心跳 + 自动重连 + drain。SDK **不监听本地端口**。

## 能力分层（速览）

| 层级                 | 含义                                                   |
| -------------------- | ------------------------------------------------------ |
| **L1 Core Provider** | 被调用方接入平台的最小集合，所有 SDK 必备              |
| **L2 Provider 扩展** | JSON Schema / Drain / 控制面 manifest / TLS / 文件传输 |
| **L3 Invoker**       | 调用方能力，独立模块                                   |
| **L4 语言/引擎扩展** | VirtualObject / DI / Unity / Lua / Spring Boot starter |

详见 [`SDK_FEATURE_MATRIX.md`](./SDK_FEATURE_MATRIX.md)。

## 各 SDK 状态

| SDK                 | 传输        | L1 Provider | L2 扩展                                   | L3 Invoker | L4 语言扩展                                       |
| ------------------- | ----------- | ----------- | ----------------------------------------- | ---------- | ------------------------------------------------- |
| [Go](./go/)         | TCP session | ✅          | TLS / Reconnect / Manifest / Retry / File | ✅         | —                                                 |
| [Python](./python/) | TCP session | ✅          | TLS / Reconnect / File / Drain            | ✅         | —                                                 |
| [Java](./java/)     | TCP session | ✅          | TLS / Reconnect / Manifest / Retry / File | ✅         | Spring Boot starter                               |
| [JS/TS](./js/)      | TCP session | ✅          | TLS / Reconnect / Manifest / File         | ✅         | —                                                 |
| [C++](./cpp/)       | TCP session | ✅          | TLS / Reconnect / Manifest / File         | ✅         | VirtualObject / Component / Lua / Plugin / Skynet |
| [C#](./csharp/)     | TCP session | ✅          | TLS / Reconnect / File                    | ✅         | DI / Unity                                        |

## CI 状态

| SDK                 | CI                                                                                                                                                                    | Docs                         |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| [Go](./go/)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml)         | [README](./go/README.md)     |
| [JS/TS](./js/)      | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml)         | [README](./js/README.md)     |
| [Python](./python/) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](./python/README.md) |
| [Java](./java/)     | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml)     | [README](./java/README.md)   |
| [C#](./csharp/)     | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](./csharp/README.md) |
| [C++](./cpp/)       | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml)       | [README](./cpp/README.md)    |

## 规划文档

- [功能矩阵（单一事实来源）](./SDK_FEATURE_MATRIX.md)
- [SDK Wire Protocol](../docs/architecture/sdk-wire-protocol.md)
- [SDK-Agent 传输重构设计](../docs/architecture/sdk-agent-transport-redesign.md)
