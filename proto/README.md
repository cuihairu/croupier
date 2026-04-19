# Proto Definitions

## 目录结构

```text
proto/
├── buf.gen.yaml
├── buf.yaml
├── README.md
├── croupier/
│   ├── agent/v1/
│   │   ├── task.proto
│   │   └── register.proto
│   ├── component/v1/
│   │   ├── dashboard_ui.proto
│   │   ├── function_options.proto
│   │   └── ui_options.proto
│   ├── external/v1/
│   │   └── platform.proto
│   ├── ops/v1/
│   │   └── ops.proto
│   └── sdk/v1/
│       ├── invocation.proto
│       └── provider.proto
└── examples/
    ├── games/
    └── integrations/
```

## 设计定位

本目录定义的是 Croupier 的平台协议与组件元数据，而不是某种固定 transport 的专属协议实现。

当前推荐理解方式：

- `shared session runtime`
  - `tcp`
  - 可选 `tls`
  - framing
  - request/response mux
  - reconnect / heartbeat / drain
- `subprotocol`
  - `sdk-agent subprotocol`
  - `agent-server subprotocol`

Proto 文件主要描述的是这些子协议之上的消息格式。

## 各目录含义

### `croupier/component/v1`

组件元数据，不直接代表传输链路：

| 文件 | 用途 |
| --- | --- |
| `function_options.proto` | 方法级元数据扩展 |
| `ui_options.proto` | 字段级 UI 元数据扩展 |
| `dashboard_ui.proto` | UI / i18n / 菜单 / 权限定义 |

### `croupier/sdk/v1`

定义 `SDK <-> Agent` 边界的消息：

| 文件 | 用途 |
| --- | --- |
| `provider.proto` | provider session 建连、心跳、drain、函数描述符 |
| `invocation.proto` | 调用、作业、取消、结果查询 |

当前语义：

- SDK 通过 `ProviderConnectRequest` 建立 provider session
- SDK 不再注册 `rpc_addr`
- SDK 不再开启本地监听端口

### `croupier/agent/v1`

定义 `Agent <-> Server` 边界的消息：

| 文件 | 用途 |
| --- | --- |
| `register.proto` | agent session 建连、心跳、能力注册 |
| `task.proto` | 任务状态与事件类型 |

当前语义：

- `RegisterRequest` 在语义上等价于 agent session connect/register
- `rpc_addr` 字段仍有历史兼容痕迹，但不应再作为目标架构的长期依赖

### `croupier/ops/v1`

定义运维与观测相关消息：

| 文件 | 用途 |
| --- | --- |
| `ops.proto` | 指标、系统信息、进程与运维控制 |

### `croupier/external/v1`

定义 Server 与第三方平台集成时使用的统一接口：

| 文件 | 用途 |
| --- | --- |
| `platform.proto` | 外部平台适配接口 |

### `examples`

示例 proto，展示如何配合组件元数据描述业务对象和函数。

## 传输与序列化边界

需要明确区分三件事：

1. 平台协议消息
   - 由 protobuf `message` 定义
   - 用于 session 建连、能力协商、调用路由、作业控制
2. 用户业务 payload
   - 默认使用 UTF-8 JSON
   - 一般承载在 protobuf 消息里的 `bytes payload`
3. 底层传输
   - 当前推荐为独立 `TCP session`
   - 按需启用 `TLS`

因此：

- protobuf 是平台协议格式
- JSON 是默认业务 payload 格式
- 不应再把 proto 文档描述为“旧传输 协议文档”或“gRPC API 文档”

## 关于 `service` 定义

仓库中的 `service` 定义主要用于：

- 表达消息分组和语义归属
- 辅助生成多语言类型与文档
- 帮助识别请求/响应族

它们不应自动被理解为“当前推荐通过 gRPC 暴露这些接口”。

## 相关文档

- [架构总览](../docs/architecture/README.md)
- [SDK-Agent 传输重构设计](../docs/architecture/sdk-agent-transport-redesign.md)
- [Agent-Server TCP Session 重构设计](../docs/architecture/agent-server-session-transport-redesign.md)
- [SDK Wire Protocol](../docs/architecture/sdk-wire-protocol.md)
