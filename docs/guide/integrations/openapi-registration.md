---
title: OpenAPI 函数注册
icon: code
order: 5
category:
  - 集成指南
tag:
  - OpenAPI
  - 函数注册
  - Agent
---

# OpenAPI 函数注册

## 概述

Croupier 当前的函数注册模型不是历史 `LocalControlService.RegisterLocal` 回拨模型，而是：

- SDK 或本地业务进程主动连接 Agent 本地 gateway
- 在同一条 `sdk-agent subprotocol` session 上完成 `ProviderConnectRequest`
- 通过连接内的 provider session 上报函数描述与心跳

OpenAPI 在这里的作用是提供函数元数据来源，而不是单独定义一套 `gRPC` 注册 API。

## 当前设计结论

- 不再要求 SDK 暴露 `rpc_addr`
- 不再要求 SDK 开本地监听端口
- 不再以 `LocalControlService` 作为新的实现依据
- OpenAPI 字段映射进 `LocalFunctionDescriptor`
- 注册、心跳、drain、调用都复用同一条 session

## 会话流程

```text
+----------------------+        TCP Session (+TLS optional)        +----------------------+
| SDK / Game Process   | ---------------------------------------> | Agent Local Gateway   |
|                      |                                           |                      |
| 1. ProviderConnect   |                                           | 建立 provider session |
| 2. Functions[]       |                                           | 记录 descriptors      |
| 3. Heartbeat         |                                           | 同步到 Server         |
| 4. Invoke Response   |                                           | 下发 Invoke / Task    |
+----------------------+                                           +----------------------+
```

说明：

- 首帧必须是 `ProviderConnectRequest`
- `functions[]` 直接挂在连接内 provider session 上
- Agent 后续通过既有 session 向 SDK 下发调用请求

## OpenAPI 字段映射

`proto/croupier/sdk/v1/provider.proto` 中的 `LocalFunctionDescriptor` 继续承载 OpenAPI 常见字段：

| Croupier 字段 | OpenAPI 字段 | 说明 |
| --- | --- | --- |
| `id` | 自定义 | 函数唯一标识，例如 `game.player.ban` |
| `version` | 自定义 | 函数版本 |
| `tags` | `tags` | 分组标签 |
| `summary` | `summary` | 简短摘要 |
| `description` | `description` | 详细描述 |
| `operation_id` | `operationId` | 操作 ID |
| `deprecated` | `deprecated` | 是否废弃 |
| `input_schema` | `requestBody.content.application/json.schema` | 输入 JSON Schema |
| `output_schema` | `responses.*.content.application/json.schema` | 输出 JSON Schema |
| `category` | `x-category` | 平台分类 |
| `risk` | `x-risk` | 风险级别 |
| `entity` | `x-entity` | 关联实体 |
| `operation` | `x-operation` | CRUD / 自定义操作 |

## 描述符示例

```json
{
  "service_id": "game-server",
  "version": "1.0.0",
  "sdk_language": "cpp",
  "sdk_version": "0.1.0",
  "protocol_version": "1.0",
  "functions": [
    {
      "id": "game.player.ban",
      "version": "1.0.0",
      "tags": ["player", "moderation"],
      "summary": "封禁玩家",
      "description": "封禁指定玩家账号，禁止登录",
      "operation_id": "banPlayer",
      "deprecated": false,
      "category": "player",
      "risk": "high",
      "entity": "Player",
      "operation": "custom",
      "input_schema": "{ \"type\": \"object\", \"properties\": { \"player_id\": { \"type\": \"string\" } } }",
      "output_schema": "{ \"type\": \"object\", \"properties\": { \"success\": { \"type\": \"boolean\" } } }"
    }
  ]
}
```

## 业务 payload 边界

需要明确区分两层：

- 平台协议层
  - `ProviderConnectRequest`
  - `InvokeRequest`
  - `InvokeResponse`
  - `JobEvent`
- 业务 payload 层
  - 默认固定为 UTF-8 JSON

边界规则：

- Agent 需要理解、路由、治理的字段，必须放在 protobuf 协议层
- 只被业务函数消费的字段，放在 JSON payload 中
- `input_schema` / `output_schema` 默认描述 JSON payload，而不是要求用户先定义 `.proto`

## 推荐接入方式

如果你已经有 OpenAPI 文档，推荐做法是：

1. 抽取 `Operation Object` 中与函数治理相关的字段
2. 生成 `LocalFunctionDescriptor`
3. 把请求/响应 schema 转成 JSON Schema 字符串填入 `input_schema` / `output_schema`
4. 通过 SDK 的 provider session 建连逻辑一次性上报

## 明确废弃的旧模型

以下概念不应再出现在新的接入文档和 SDK 设计中：

- `LocalControlService` 作为主语义入口
- `RegisterLocalRequest` / `RegisterLocalResponse` 作为主语义入口
- `rpc_addr`
- SDK 本地监听端口
- `Agent -> SDK` 回拨模型

## 相关文档

- [SDK-Agent 传输重构设计](../../architecture/sdk-agent-transport-redesign.md)
- [SDK Wire Protocol](../../architecture/sdk-wire-protocol.md)
- [SDK 规范](../../sdk/specification.md)
