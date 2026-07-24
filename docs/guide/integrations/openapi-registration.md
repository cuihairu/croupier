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

> **状态**：Current — OpenAPI 是函数契约输入；Descriptor v2 是 OpenAPI / SDK 注册与 PageSpec 生成之间的统一契约。

## 概述

Croupier 的函数注册模型基于 Provider Session 设计：

- SDK 或本地业务进程主动连接 Agent 本地 gateway。
- 在同一条 `sdk-agent subprotocol` session 上完成 `ProviderConnectRequest`。
- 通过连接内 provider session 上报函数描述与心跳。
- 注册、心跳、drain、调用都复用同一条 session。

OpenAPI 在这里提供函数契约和元数据来源，不定义单独的注册 RPC，也不直接等于 Dashboard Page。

完整字段契约见 [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)。

## 当前设计结论

- 不再要求 SDK 暴露 `rpc_addr`。
- 不再要求 SDK 开本地监听端口。
- 不再以 `LocalControlService` 作为实现依据。
- OpenAPI 标准字段映射到 RawFunctionDescriptor。
- OpenAPI `x-*` 扩展字段映射到 Descriptor v2。
- Server 先归一化 FunctionSpec / ResourceSpec / OperationSpec，再生成 PageSpec 建议。
- 前端运行控制台不得直接从 OpenAPI operation 生成菜单或页面。

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

- 首帧必须是 `ProviderConnectRequest`。
- `functions[]` 直接挂在连接内 provider session 上。
- Agent 后续通过既有 session 向 SDK 下发调用请求。

## OpenAPI 字段映射

基础字段：

| Croupier 字段 | OpenAPI 字段 | 说明 |
| --- | --- | --- |
| `id` | `operationId` 或导入器生成 | 函数唯一标识，例如 `player.ban` |
| `version` | `x-version` | 函数版本 |
| `tags` | `tags` | 分组标签 |
| `summary` | `summary` | 简短摘要 |
| `description` | `description` | 详细描述 |
| `deprecated` | `deprecated` | 是否废弃 |
| `input_schema` | `requestBody.content.application/json.schema` | 输入 JSON Schema |
| `output_schema` | `responses.*.content.application/json.schema` | 输出 JSON Schema |

治理和页面生成字段：

| Croupier 字段 | OpenAPI 扩展 | 说明 |
| --- | --- | --- |
| `category` | `x-category` | 分类 key |
| `category_display` | `x-category-display` | 分类多语言显示名 |
| `risk` | `x-risk` | 风险级别 |
| `entity` | `x-entity` | 资源 key |
| `entity_display` | `x-entity-display` | 资源多语言显示名 |
| `operation` | `x-operation` | 业务操作 key，例如 `ban`、`send` |
| `operation_display` | `x-operation-display` | 操作多语言显示名 |
| `operation_kind` | `x-operation-kind` | 页面生成语义，例如 `list`、`action`、`task` |
| `placement` | `x-placement` | 页面放置位置，例如 `tableData`、`rowAction` |
| `page_hint` | `x-page-hint` | 可选页面生成建议 |

`x-operation` 不再表示页面类型。CRUD、命令、任务、报表等页面语义由 `x-operation-kind` 表达。

## OpenAPI 示例

```yaml
paths:
  /players/{player_id}/ban:
    post:
      operationId: player.ban
      tags:
        - player
        - moderation
      summary: 封禁玩家
      description: 封禁指定玩家账号，可设置原因和时长。
      x-version: 1.0.0
      x-category: support
      x-category-display:
        zh-CN: 客服
        en-US: Support
      x-risk: danger
      x-entity: player
      x-entity-display:
        zh-CN: 玩家
        en-US: Player
      x-operation: ban
      x-operation-display:
        zh-CN: 封禁
        en-US: Ban
      x-operation-kind: action
      x-placement: rowAction
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - player_id
                - reason
              properties:
                player_id:
                  type: string
                  title: 玩家 ID
                reason:
                  type: string
                  title: 封禁原因
                duration_seconds:
                  type: integer
                  title: 封禁时长
      responses:
        "200":
          description: 封禁结果
          content:
            application/json:
              schema:
                type: object
                properties:
                  success:
                    type: boolean
                  ban_id:
                    type: string
```

该 operation 会被归一化为：

```text
FunctionSpec(player.ban)
  -> ResourceSpec(player)
  -> OperationSpec(kind=action, placement=rowAction)
  -> Entity PageSpec 建议
```

## 与 PageSpec 的关系

OpenAPI operation 只能提供函数契约并触发 PageSpec 候选生成，不是 PageSpec 本身，也不是 Function Form 或页面 UI 配置。

正确链路：

```text
OpenAPI Operation
  -> RawFunctionDescriptor
  -> FunctionSpec / ResourceSpec / OperationSpec
  -> Generated PageSpec
  -> Page Studio 编辑
  -> PublishedPageSpec
  -> 运行控制台菜单
```

错误链路：

```text
OpenAPI Operation
  -> 前端运行时猜菜单
  -> 前端运行时猜页面
```

## 业务 payload 边界

需要明确区分两层：

- 平台协议层
  - `ProviderConnectRequest`
  - `InvokeRequest`
  - `InvokeResponse`
  - `TaskEvent`
- 业务 payload 层
  - 默认固定为 UTF-8 JSON

边界规则：

- Agent 需要理解、路由、治理的字段，必须放在协议层或 descriptor metadata。
- 只被业务函数消费的字段，放在 JSON payload 中。
- `input_schema` / `output_schema` 默认描述 JSON payload，而不是要求用户先定义 `.proto`。

## 推荐接入方式

OpenAPI 有两种入口，先选定执行模型：

| 入口 | 适用场景 | 如何变为可执行函数 | UI 责任 |
| --- | --- | --- | --- |
| SDK 本地解析 | OpenAPI 和业务 Handler 在同一服务进程 | SDK 将 `operationId` 显式映射到本地 Handler，再走普通 Provider 注册 | Server 自动派生 Function Form，Page Studio 确定页面 |
| Dashboard 上传（P0-11 目标） | 已有第三方/存量 OpenAPI 文档 | 先保存为契约 Source，再绑定 Provider 或受控 HTTP Connector | 同上；上传文件不含 UI |

目标上传模型中，OpenAPI 不等于开放任意外部 HTTP 调用。未绑定执行器的文档只用于函数目录和页面候选；需要执行时必须显式绑定当前 scope 的 Provider，或由管理员配置受控 HTTP Connector。Connector 的地址和 SecretRef 是平台配置，不在 OpenAPI、PageSpec 或浏览器中传递。当前 `POST /api/v1/openapi/import` 只支持 JSON body 并写入 registry，不是这个目标上传能力。

如果已有 OpenAPI 文档：

1. 为每个 Operation 补齐 `operationId`、`summary`、`description`。
2. 把 request/response schema 写完整。
3. 使用 `x-category`、`x-entity`、`x-operation`、`x-risk` 表达治理和资源归属。
4. 可选地使用 `x-operation-kind` 和 `x-placement` 提高 PageSpec 候选质量；不写不影响函数导入。
5. 可选地使用 `x-category-display`、`x-entity-display`、`x-operation-display` 提供动态多语言文案。
6. 导入 OpenAPI 后，由 Server 生成 FunctionSpec / ResourceSpec / OperationSpec，并从 request schema 派生 Function Form。
7. 在 Page Studio 确认、补齐或编辑 PageSpec 候选后发布。

## 发布限制

以下情况只允许进入函数目录或待编排建议，不能自动发布为正式页面：

- 缺少 `operation_kind`。
- 缺少 `placement`。
- 缺少动态 labels。
- `output_schema` 无法支持声明的表格、详情或报表组件。

## 明确废弃的旧模型

以下概念不应再出现在新的接入文档和 SDK 设计中：

- `LocalControlService` 作为主语义入口。
- `RegisterLocalRequest` / `RegisterLocalResponse` 作为主语义入口。
- `rpc_addr`。
- SDK 本地监听端口。
- `Agent -> SDK` 回拨模型。
- 用 `x-operation` 表示 CRUD 或非 CRUD 类型。
- 前端根据 OpenAPI 或函数 ID 后缀生成正式 Page。
- 在 OpenAPI 中保存 `x-ui`、Formily、菜单、路由或页面组件配置。

## 相关文档

- [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)
- [Dashboard Resource/Page 模型](../../architecture/dashboard-page-model.md)
- [函数注册与默认界面](../concepts/function-registration-ui.md)
- [SDK-Agent 传输重构设计](../../architecture/sdk-agent-transport-redesign.md)
- [SDK Wire Protocol](../../architecture/sdk-wire-protocol.md)
- [SDK 文档](../../sdks/)
