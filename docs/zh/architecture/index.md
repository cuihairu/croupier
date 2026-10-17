---
title: 架构总览
---

# 架构总览

Croupier 是由三个协作组件构成的游戏运营控制面：

| 组件       | 职责                                                                       |
| ---------- | -------------------------------------------------------------------------- |
| **Server** | 持有注册表、策略、审批、审计、请求路由与运营控制台 API。                   |
| **Agent**  | 维护到 Server 的上行会话，并接入游戏进程与 SDK 的本地 provider 会话。      |
| **SDK**    | 嵌入游戏进程或本地服务，向 Agent 暴露函数能力，并在需要时消费 SDK 侧接口。 |

当前传输基线是统一 session 模型：

- `Agent ↔ Server` 使用默认启用 TLS 的 TCP session。
- `SDK ↔ Agent` 使用本地 TCP provider session。
- 请求、响应、心跳、drain、重连和背压共享同一套 session runtime。

## 业务边界

标准业务边界是 `gameId + env`。`dev`、`test`、`staging`、`prod` 是逻辑环境，不直接代表某个数据库、主机或集群。部署位置通过 target、agent、node 单独表达。

## 核心链路

```text
函数注册 -> FunctionContract -> 资源语义 -> PageProposal -> 已发布页面 -> 运营执行
```

1. SDK 向 Agent 注册函数、provider 与进程能力。
2. Agent 通过自己的 Server session 同步摘要。
3. Server 把契约归一化为资源能力和业务语义。
4. Page Studio 预览、校验、发布、合并和回滚运营页面。
5. 运行控制台只执行已发布的页面绑定。

## 分层视角

- **协议层**使用 protobuf 承载平台控制字段。
- **payload 层**使用 UTF-8 JSON 承载业务数据。
- **契约层**使用 JSON Schema 和 OpenAPI Schema 描述输入输出。
- **呈现层**由强类型 PageSpec 驱动，不依赖临时组件树。

## 阅读路径

- 先看[分层](/zh/architecture/layers)了解组件与边界。
- 看[术语与分层](/zh/architecture/terms-and-layering)统一词汇。
- 通过[数据流](/zh/architecture/data-flow)理解注册、分发和页面执行。
- 通过[会话生命周期](/zh/architecture/session-lifecycle)和 [SDK Wire 协议](/zh/architecture/sdk-wire-protocol)理解传输行为。
