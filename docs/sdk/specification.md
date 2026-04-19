# Croupier SDK 行为规范

> 协议与架构入口：
>
> - `docs/architecture/sdk-wire-protocol.md`
> - `docs/architecture/sdk-agent-transport-redesign.md`
> - `docs/architecture/agent-server-session-transport-redesign.md`
> - `docs/sdks/sdk-parity-matrix.md`
> - `docs/sdk/missing-features.md`

本文档定义所有 Croupier SDK 的统一行为规范。
旧的“SDK 启动本地 gRPC/local server、暴露 `local_addr/rpc_addr` 给 Agent 回调”的模型已经废弃。

## 核心原则

1. SDK 默认作为 `Agent` 的 session client，而不是本地服务端。
2. SDK 不需要监听端口。
3. 业务 payload 默认使用 UTF-8 JSON。
4. 平台信封、控制字段和路由字段使用 protobuf。
5. SDK 与 `Agent` 共用 `shared session runtime`，运行在 `sdk-agent subprotocol` 上。

## subprotocol 说明

这里的 `subprotocol` 指“共享同一套 session runtime，但握手消息与业务语义不同的子协议”，不是个性化配置。

在 SDK 文脉下：

- `shared session runtime`
  - `tcp/tls`
  - framing
  - mux
  - heartbeat
  - reconnect
  - drain
  - backpressure
- `sdk-agent subprotocol`
  - 首条消息为 `ProviderConnectRequest`
  - 使用 provider session 语义

## Client 接口

所有 SDK 应提供统一风格的 client：

| 方法 | 行为 |
| --- | --- |
| `registerFunction` | 注册本地函数描述与 handler |
| `registerFunctions` | 批量注册函数 |
| `connect` | 连接 Agent 并建立 provider session |
| `serve` | 阻塞等待，直到连接关闭或显式停止 |
| `close` | 关闭 session 与本地资源 |

说明：

- `serve` 只表示“阻塞等待 session 生命周期结束”，不再表示“启动本地监听服务”
- `connect` 后不应再允许动态增删函数，除非该语言 SDK 明确支持热更新并有一致语义

## Invoker 接口

如果 SDK 同时支持主动调用远端函数，应提供：

| 方法 | 行为 |
| --- | --- |
| `invoke` | 同步调用 |
| `startJob` | 发起异步任务 |
| `streamJob` | 订阅任务事件 |
| `cancelJob` | 取消任务 |
| `close` | 关闭资源 |

## LocalFunctionDescriptor

所有 SDK 应继续使用 `LocalFunctionDescriptor` 表达函数能力。

标准字段：

- `id`
- `version`
- `tags`
- `summary`
- `description`
- `operation_id`
- `deprecated`
- `input_schema`
- `output_schema`
- `category`
- `risk`
- `entity`
- `operation`

约束：

- `input_schema` / `output_schema` 描述 JSON payload 的 JSON Schema
- schema 是可选增强项，不是接入前置条件
- 不允许把 Agent 需要理解的控制字段塞进 JSON payload

## 配置规范

建议所有 SDK 统一支持：

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `transport.kind` | 固定优先为 `tcp` | `tcp` |
| `transport.address` | Agent 地址 | `127.0.0.1:19090` |
| `transport.connect_timeout_ms` | 连接超时 | `5000` |
| `transport.request_timeout_ms` | 请求超时 | `30000` |
| `transport.tls.enabled` | 是否启用 TLS | `false` |
| `reconnect.enabled` | 是否自动重连 | `true` |
| `reconnect.initial_delay_ms` | 初始退避 | `1000` |
| `reconnect.max_delay_ms` | 最大退避 | `30000` |
| `reconnect.backoff_multiplier` | 退避倍率 | `2.0` |
| `reconnect.jitter_factor` | 抖动系数 | `0.2` |
| `backpressure.max_concurrency` | 本地并发上限 | 实现自定 |
| `backpressure.max_queue_size` | 本地排队上限 | 实现自定 |

## 明确废弃的字段

以下字段不应再作为新设计的一部分：

- `local_listen`
- `local_addr`
- `rpc_addr`
- `getLocalAddress`
- “启动本地 gRPC server”
- “启动本地 local server”

## 生命周期

推荐流程：

1. 创建 client
2. 注册全部函数
3. `connect()`
4. SDK 发送 `ProviderConnectRequest`
5. 进入 `serve()` 或事件循环
6. `heartbeat`
7. 连接断开时自动重连
8. 重连后重新发送 `ProviderConnectRequest`

## 错误与重试

SDK 应至少区分：

- 参数错误
- 协议错误
- 认证/鉴权错误
- 连接错误
- 超时错误
- 远端不可用

重试规则：

- 仅对连接类、超时类、瞬时不可用类错误自动重试
- 协议不兼容、认证失败、配置错误不应无限重试

## 日志与安全

- 默认不得打印密码、token、密钥
- 默认不得打印完整敏感 payload
- SDK-Agent 默认不强制 TLS，但应支持配置启用
- 在内网同机/同网段场景，默认明文 `tcp` 是允许的

## 最终要求

所有 SDK 的默认接入体验应满足一句话：

> 用户只需要注册函数、连接 Agent、传入 JSON 对象，不需要启动本地服务，也不需要先定义 `.proto`。
