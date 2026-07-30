---
title: OpenAPI / SDK Descriptor v2
icon: file-code
order: 6
category:
  - 系统架构
tag:
  - OpenAPI
  - SDK
  - 函数注册
  - PageSpec
---

# OpenAPI / SDK Descriptor v2

> **状态**：Target -- 本文是函数注册与页面自动生成的权威契约。SDK/OpenAPI 只负责可执行能力，不负责页面设计。

## 目的

OpenAPI 和 SDK descriptor 负责描述 **可执行能力**，而不是描述页面。平台据此识别 CRUD 和非 CRUD 语义，再生成可追溯的 PageProposal。

```text
OpenAPI operation / SDK descriptor
  -> FunctionContract
  -> CapabilitySemantics
  -> PageProposal
  -> PageSpec
```

详细页面模型见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)。

## 注册字段

所有 SDK 必须收敛到同一 FunctionContract：

| 字段 | OpenAPI 来源 | SDK 字段 | 说明 |
| --- | --- | --- | --- |
| `id` | `operationId` | `id` | 稳定函数 ID |
| `version` | 文档/服务版本 | `version` | 函数契约版本 |
| `summary` / `description` | 标准字段 | 同名字段 | 目录和诊断说明，非菜单事实 |
| `inputSchema` | request body schema | `inputSchema` | 请求 JSON Schema |
| `outputSchema` | response schema | `outputSchema` | 响应 JSON Schema |
| `resourceKey` | REST path 推导或 `x-resource` | `resource` | 稳定业务资源 key |
| `operationKey` | REST operation 或 `x-operation` | `operation` | 业务动作 key |
| `capability` | REST 形态推导或 `x-capability` | `capability` | 受控业务语义，非 UI |
| `execution` | 标准响应/`x-execution` | `execution` | `sync`、`task`、`approval` |
| `risk` / `permission` | `x-risk` / `x-permission` | 同名字段 | 治理约束 |

`capability` 只允许：

```text
collection_query | item_query | create | update | delete | action | task | report
```

它回答“函数在资源生命周期中做什么”，不回答“按钮显示在哪里”或“页面长什么样”。

## OpenAPI REST 自动识别

当 OpenAPI 同时满足稳定 path 和 schema 条件时，Server 必须产生高置信度 CRUD 语义：

| OpenAPI 操作 | 默认 capability | 额外验证 |
| --- | --- | --- |
| `GET /players` | `collection_query` | 可识别 collection schema；分页为可选能力 |
| `POST /players` | `create` | request body 对应资源输入 |
| `GET /players/{playerId}` | `item_query` | path parameter 是资源 identity 候选 |
| `PUT/PATCH /players/{playerId}` | `update` | request body 与 identity 可关联 |
| `DELETE /players/{playerId}` | `delete` | identity 可关联 |
| 其他 REST 操作 | `action` / `task` / `report` | 需显式 execution 或可验证响应特征 |

推导出的语义必须记录来源和置信度。REST path 不满足规范、identity 不可验证或 schema 矛盾时，平台降级为 OperationPage Proposal 并给出诊断，不得猜测成 Resource CRUD。

## SDK 函数语义

SDK 没有 HTTP method/path，不能只凭函数名生成完整 CRUD 页面。SDK 可以提供 `resourceKey + capability`，这是能力注册的一部分，不是 UI 配置：

```ts
registerFunction({
  id: 'player.update',
  resource: 'player',
  operation: 'update',
  capability: 'update',
  inputSchema: PlayerUpdateSchema,
  outputSchema: PlayerSchema,
  risk: 'warning',
});
```

SDK 若只提供函数 ID 和 JSON Schema，仍可正常注册并获得默认 OperationPage；若要参加 Resource CRUD 组合，必须提供受控 capability 或由平台管理员在 Resource Catalog 补充语义。这个要求不会让 SDK 设计菜单、列、Form、页面或 mapping。

## 注册输入边界

SDK 和 OpenAPI Source 导入边界只接受 FunctionContract 字段。页面展示和编排信息必须留在 PageProposal/PageSpec 或 Page Studio 中，包括：

- 表格列、分页绑定、详情布局、按钮位置、图表配置和任务事件绑定。
- 菜单、路由、分类显示名、页面标题、按钮文案和多语言页面 labels。
- 任意浏览器运行时 target、scope、secret、connector URL 或 HTTP header。
- 任意组件树、组件 props、页面 mapping 或布局 DSL。

导入器遇到上述页面字段必须返回结构化 diagnostics，不得静默丢弃或转换。

## OpenAPI Source 与执行绑定

上传 OpenAPI 只产生 Source、FunctionContract 候选和 diagnostics；它不直接注册可调用函数。

```text
OpenAPI Source
  -> parse / validate / normalize
  -> provider binding 或受控 http connector
  -> FunctionContract
  -> CapabilitySemantics / PageProposal
```

当前 provider binding 和未来受控 http connector 都必须按 `game_id + env` 隔离，并经过权限、审计与 OTel。OpenAPI 文档不允许包含 Secret、任意内网 URL 或页面配置。

## 生成边界

Server 以 FunctionContract + CapabilitySemantics 生成 Proposal：

- 标准 CRUD 语义生成 ResourcePage Proposal。
- 可执行但缺少 Resource 语义的同步函数生成 `basic` OperationPage Proposal。
- `execution=task` 生成 TaskPage Proposal；真实 task 状态契约不完整时为 `needs_review`。
- `capability=report` 生成 ReportPage Proposal；数据集/指标语义不完整时为 `needs_review`。

生成器不可读取或写入 SDK/OpenAPI 中的页面 UI。PageProposal 的列、映射、导航、表单展示和动作位置都由 Server 按生成规则产生，随后由 Page Studio 管理。

## 函数变更

每次 FunctionContract 或 CapabilitySemantics 变化必须：

1. 保存新版本和摘要。
2. 重新计算受影响的 ResourceCapability 与 PageProposal。
3. 与 PageDraft/PublishedPageSpec 的 source digest 做 diff。
4. 标记 stale，阻断不安全执行。
5. 允许用户在 Page Studio 查看、合并、修复并重新发布。

不得以最新函数 descriptor 静默替换已发布页面。

## SDK 交付要求

- 所有 SDK 对 `FunctionContract` 的字段语义一致；不支持的字段必须明确报错或在能力矩阵标记为未支持。
- Go 的 `RegisterFromOpenAPI` 只是本地 handler 注册 helper；其他 SDK 不能假称有同等能力。
- Demo 必须至少覆盖一个 CRUD Resource、一个 Operation、一个 Task 和一个 Report contract，并通过 Server 的 Proposal 生成 E2E。
- SDK 绝不携带页面 UI 或 PageSpec。
