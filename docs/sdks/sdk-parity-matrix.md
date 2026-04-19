# SDK 对齐矩阵

本文档记录的是各语言 SDK 必须对齐的统一基线，而不是继续维护历史 `local server`、`LocalControl`、`gRPC callback` 的静态遗留能力表。

## 评估口径

统一使用以下状态：

- `Required`: 所有 SDK 必须支持
- `Optional`: 可选增强项，但语义必须一致
- `Forbidden`: 新设计中禁止继续实现或继续宣传

## 统一基线

| Capability | 基线 |
| --- | --- |
| SDK 主动连接 Agent 本地 gateway | `Required` |
| 单条长连接复用注册、心跳、调用、作业控制 | `Required` |
| `ProviderConnectRequest` 作为首帧 | `Required` |
| 默认 transport 为独立 `tcp session` | `Required` |
| `tls` 作为 `tcp` 的可选安全配置 | `Required` |
| `InvokeRequest/Response/JobEvent` 默认 JSON payload | `Required` |
| `input_schema/output_schema` 描述 JSON Schema | `Required` |
| 自动重连 + 指数退避 + 上限后廉价周期重试 | `Required` |
| drain / overload / backpressure 语义 | `Required` |
| JSON payload 自动编解码 | `Required` |
| 多地址回退 | `Optional` |
| IPC / 本地专用优化 transport | `Optional` |
| mTLS | `Optional` |
| SDK 本地监听端口 | `Forbidden` |
| `rpc_addr` / 回拨式注册 | `Forbidden` |
| SDK 侧 `NNGServer` | `Forbidden` |
| 以 `gRPC` 作为 SDK 默认主链路 | `Forbidden` |

## 协议边界

所有 SDK 必须遵守以下分层：

### 协议层

- protobuf message
- 8 字节 header
- request / response 对应规则
- `ProviderConnectRequest` / `ProviderHeartbeatRequest`
- `InvokeRequest` / `StartTaskRequest` / `CancelTaskRequest`

### 业务 payload 层

- 默认固定为 UTF-8 JSON
- SDK 负责把语言原生对象编码为 JSON
- Agent 不默认解析业务 JSON 内部结构

## 统一配置面

所有 SDK 建议至少暴露以下统一配置语义：

| 字段 | 说明 |
| --- | --- |
| `address` | Agent 本地 gateway 地址 |
| `connectTimeoutMs` | 建连超时 |
| `requestTimeoutMs` | 单请求超时 |
| `tls.enabled` | 是否启用 TLS |
| `tls.caFile` | CA 文件 |
| `tls.certFile` | 客户端证书 |
| `tls.keyFile` | 客户端私钥 |
| `tls.serverName` | TLS SNI / 校验名 |
| `tls.insecureSkipVerify` | 仅开发环境跳过校验 |
| `heartbeat.intervalMs` | 心跳间隔 |
| `reconnect.enabled` | 自动重连 |
| `reconnect.initialDelayMs` | 初始退避 |
| `reconnect.maxDelayMs` | 最大退避 |
| `reconnect.multiplier` | 退避倍率 |
| `reconnect.jitter` | 抖动比例 |
| `reconnect.steadyStateDelayMs` | 到达上限后的廉价重试周期 |

## 命名要求

为了减少跨语言认知成本，建议统一术语：

- `shared session runtime`
- `sdk-agent subprotocol`
- `provider session`
- `payload`
- `drain`
- `backpressure`

不建议继续把新的实现文档写成：

- `LocalControl`
- `RegisterLocal`
- `local_listen`
- `rpc_addr`
- `session client/server`

## 为什么主仓库不再维护旧式“现状能力表”

过去那种静态矩阵有两个问题：

1. 很快过期
2. 会把历史实现细节误写成目标能力

主仓库现在更适合维护：

- 统一协议与配置基线
- 统一术语
- 统一禁止项

而每个 SDK 当前实现状态，应由各 SDK 仓库自己的 README、测试矩阵和 CI 结果负责给出。

## SDK 仓库

- Go: `croupier-sdk-go`
- C++: `croupier-sdk-cpp`
- Java: `croupier-sdk-java`
- JS/TS: `croupier-sdk-js`
- Python: `croupier-sdk-python`
- C#: `croupier-sdk-csharp`

## 相关文档

- [SDK 规范](../sdk/specification.md)
- [SDK-Agent 传输重构设计](../architecture/sdk-agent-transport-redesign.md)
- [SDK Wire Protocol](../architecture/sdk-wire-protocol.md)
