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

> **状态**：Current — 函数注册、OpenAPI 扩展字段、SDK descriptor 和 PageSpec 生成之间的权威契约。

本文档定义 Croupier 函数注册描述符 v2。它不推翻 OpenAPI，而是明确 OpenAPI、SDK 注册和平台内部模型的边界。

## 设计结论

OpenAPI 是函数能力契约，不是 Dashboard Page。

SDK descriptor 是函数注册输入，不是运行控制台菜单，也不是 UI 配置。

平台内部必须先归一化为强类型模型，再生成默认页面：

```text
OpenAPI Operation / SDK descriptor / DB template
  -> RawFunctionDescriptor
  -> FunctionSpec
  -> ResourceSpec + OperationSpec
  -> PageSpec 候选
  -> Page Studio 草稿
  -> PublishedPageSpec
  -> ConsoleMenuSpec
```

前端不得直接根据 OpenAPI operation 或函数 ID 后缀生成正式页面。

## UI 确定时机与职责

函数注册、SDK OpenAPI 解析和用户上传 OpenAPI 都只负责提供**可执行能力契约**，不要求也不接受界面定义。UI 分三步确定，责任必须固定：

| 阶段 | 输入 | 产物 | 责任人 | 是否最终 UI |
| --- | --- | --- | --- | --- |
| 函数注册 / OpenAPI Source 解析 | ID、版本、输入/输出 schema、描述、风险和可选业务归属 key | FunctionSpec | 服务开发者或 OpenAPI 所有者 | 否 |
| Server 归一化与生成 | FunctionSpec、JSON Schema、业务归属 key | 自动派生 Function Form + 保守 PageCandidate + diagnostics | Server | 否，可随契约重新生成 |
| Page Studio | PageCandidate、binding、数据映射、组件 ABI | PageDraft -> PublishedPageSpec | 页面管理员 / 运营 | 是，发布后冻结 |

具体规则：

- **函数表单**在 Server 读取 `inputSchema` 时自动派生为 Formily；没有 schema 时仅显示 JSON `payload` 输入和诊断。它不是 SDK 传来的 UI。
- **页面候选**在 Resource/Operation 归一化之后生成；Server 只做可验证的保守推导，无法确定列表字段、分页、动作输入或图表映射时标为 `needs_review`，不猜测。
- **最终页面 UI**只在 Page Studio 保存并在发布时确定。菜单、页面组件、表格列、数据 mapping、binding 和确认策略都属于 PageSpec，绝不回写到函数注册或 OpenAPI 文档。
- 函数表单可由管理员在注册后保存 override；override 仍只管输入体验，不影响 PageSpec 的布局和导航。

因此服务开发者只需把业务请求/响应契约写清楚。页面分类、标题、多语言显示名、按钮位置、表格列、图表和 mapping 都不属于函数注册；这些只能在 Server 候选和 Page Studio 中确定。

## OpenAPI 两种接入模式

OpenAPI 文档本身不能执行请求。平台必须区分“解析契约”和“绑定执行器”，避免上传一份文档后错误地把它当成可调用函数。

### SDK 本地解析并注册

SDK 可提供 `RegisterFromOpenAPI(spec, handlers)` 便利方法：本地解析 OpenAPI，再把每个 `operationId` 映射到同进程的明确 Handler，最后走普通 Provider 注册链路。

```text
OpenAPI bytes + operationId -> local Handler map
  -> SDK descriptor + local executable handler
  -> Provider registration
  -> FunctionSpec
```

约束：

- 每个导入 operation 必须有稳定 `operationId` 和对应 Handler；缺失时整体失败，或显式 `continueOnError` 后在结果中报告。
- SDK 解析只是开发便利，不能成为平台唯一 parser；所有官方 SDK 的导入结果必须和 Server 归一化契约一致。
- 当前实现只有 Go SDK 提供 `RegisterFromOpenAPI`；JS、Python、Java、C#、C++ 尚未提供等价 API，不能宣称跨语言 SDK 已支持。
- SDK helper 不上传 UI、不生成 PageSpec、不自动发布页面。

### 用户上传 OpenAPI 契约源（已落地基础闭环）

用户可以在 Dashboard 上传 JSON 或 YAML 的 OpenAPI 3.x 文档。上传创建 `OpenAPISource`，保留来源、hash、校验 diagnostics、scope 和操作清单，而不是直接伪造可执行 Function。

```text
OpenAPI file / pasted document
  -> OpenAPISource (validated contract catalog)
  -> ExecutionBinding (optional)
  -> FunctionSpec
  -> Function Form / PageCandidate
```

`ExecutionBinding` 必须是以下二者之一：

- `provider`：将 operationId 显式绑定到当前 scope 内已注册的 SDK Provider Handler。
- `httpConnector`：由平台管理的受控 HTTP 调用器，使用独立配置的 base URL、SecretRef、允许的 host、超时、重试和 scope 策略。

未绑定执行器的 Source 只能作为契约目录和页面候选输入，不能发布包含可执行 binding 的页面；绑定后才形成可调用 FunctionSpec。HTTP Connector 的凭据和目标地址不允许写进 OpenAPI 文件、PageSpec 或前端请求。

当前实现只把 `provider` binding 的 operation 合并进 Resource/PageCandidate 生成视图：`operationId -> functionId` 必须显式绑定，且该 `functionId` 必须来自当前 scope 已注册 Provider。未绑定 Source 不会伪造成可执行函数，也不会绕过 PageSpec 发布校验。

当前 API：

```text
GET    /api/v1/openapi/sources
POST   /api/v1/openapi/sources                 # multipart file、raw JSON/YAML 或 { name, spec }
GET    /api/v1/openapi/sources/:sourceId
PUT    /api/v1/openapi/sources/:sourceId       # raw JSON/YAML 或 { name, spec }，生成新 revision
POST   /api/v1/openapi/sources/:sourceId/bindings
DELETE /api/v1/openapi/sources/:sourceId/bindings/:bindingId
GET    /api/v1/openapi/sources/:sourceId/diagnostics
```

固定系统入口是 `/system/functions/openapi-sources`。该页面只管理 Source、operation diagnostics 和 Provider binding，不是运行控制台动态菜单，也不决定 Page UI；运行页面仍必须在 Page Studio 中保存、校验、发布。

权限规则：

- 读取 Source、operation、diagnostics、binding：`openapi_sources:read`，或具备 `openapi_sources:write/resources:read/resources:diagnose/functions:read/functions:manage/pages:read/pages:edit`。
- 上传或更新 Source、创建或删除 Provider binding：`openapi_sources:write`，或具备 `resources:diagnose/functions:manage/pages:edit`。

审计和追踪规则：

- 上传 Source 写 `openapi_source.create` 审计事件，记录 `game_id/env/source_id/revision/format/openapi_version/operation_count/diagnostic_count/content_hash`。
- 更新 Source 写 `openapi_source.update` 审计事件，记录 `game_id/env/source_id/previous_revision/revision/format/openapi_version/operation_count/diagnostic_count/content_hash`。
- 创建 Provider binding 写 `openapi_source.binding_create` 审计事件，记录 `source_id/revision/binding_id/operation_id/function_id/provider_id`。
- 删除 Provider binding 写 `openapi_source.binding_delete` 审计事件，记录 `source_id/revision/binding_id`。
- Source 管理 span 使用 `openapi.source.create`、`openapi.source.update`、`openapi.source.binding.create`、`openapi.source.binding.delete`，只记录 scope、source、operation、binding、function、provider 等非敏感字段，不记录 OpenAPI 原文、示例 payload 或 Secret。

当前实现已切断历史 `POST /api/v1/openapi/import` 路由。Source 上传不会写入 runtime registry；`provider` binding 可以显式绑定当前 scope 内已注册函数。`httpConnector` 仍未启用，必须等 allowlist、SecretRef、超时/重试和审计策略完整后才能开放。

## 为什么需要 v2

旧模型只有：

```text
category / entity / operation / risk
```

这不足以稳定生成游戏运营后台页面，原因是：

- `operation` 被混用成 CRUD 类型和业务动作名。
- 资源被误解成数据库表或通用 CRUD 页面，导致所有函数被强行塞进同一种页面形态。
- 函数注册和页面发布边界不清，容易把菜单、显示名、按钮位置和布局塞回 SDK。
- 缺少统一的 Server 归一化模型，前端只能退回函数名猜测。

v2 的核心变化：

```text
resource  = 业务资源或能力域 key，例如 player / mail / analytics
operation = 业务动作 key，例如 list / ban / grant / send
risk      = 治理风险级别
schema    = 输入/输出 JSON Schema
```

## 三层边界

| 层 | 输入/输出 | 职责 |
| --- | --- | --- |
| Raw Descriptor | OpenAPI Operation / SDK descriptor | 承载函数能力契约 |
| FunctionSpec / ResourceSpec / OperationSpec | Server 归一化产物 | 清洗、校验、补全、诊断 |
| PageSpec | Server 生成或用户编辑 | 表达完整业务页面，必须是 Formily JSON Schema |

PageSpec 生成只能依赖归一化后的强类型模型。描述符表达的是函数**当前契约**；Page 发布时必须把实际依赖的函数契约摘要冻结到 PublishedPageSpec，运行时不能直接把“最新 descriptor”当作已发布页面的事实。

## 字段契约

### 基础字段

| 平台字段 | OpenAPI 来源 | SDK descriptor 字段 | 说明 |
| --- | --- | --- | --- |
| `id` | `operationId` 或导入器生成 | `id` | 全局函数 ID，建议稳定可读 |
| `version` | `x-version` | `version` | 函数版本 |
| `tags` | `tags` | `tags` | 搜索和分组辅助 |
| `summary` | `summary` | `summary` | 一句话简介 |
| `description` | `description` | `description` | 详细说明 |
| `deprecated` | `deprecated` | `deprecated` | 是否废弃 |
| `inputSchema` | request body schema | `input_schema` | JSON Schema，描述业务 payload |
| `outputSchema` | response body schema | `output_schema` | JSON Schema，描述业务响应 |

### 业务与治理字段

| 平台字段 | OpenAPI 扩展 | SDK descriptor 字段 | 说明 |
| --- | --- | --- | --- |
| `resource` | `x-resource` | `resource` | 业务资源或能力域 key，不是菜单分类，不是显示名 |
| `operation` | `x-operation` | `operation` | 业务动作 key，例如 `ban` / `send` / `batchGrant` |
| `risk` | `x-risk` | `risk` | `safe` / `warning` / `high` / `danger` |
| `enabled` | `x-enabled` | `enabled` | 是否启用 |
| `permission` | `x-permission` | `permission` | 可选权限标识 |

函数注册的最小要求只是可执行能力契约，不包含任何 Formily、Page UI、菜单、动态显示名或页面放置位置。缺少 `resource` 或 `operation` 时函数仍正常注册，Server 只能生成更保守的候选或 diagnostics。无论契约多完整，函数注册都不会自动发布正式 Page。

## 页面生成还需要的契约信息

默认页面候选不是函数注册字段的直接投影。若要把生成结果标为 `ready`，生成器必须能验证输入、输出和执行约束；不能只根据函数名或业务 key 拼一个外观类似的页面。

| 目标能力 | 描述符/归一化模型必须提供 | 缺失时的结果 |
| --- | --- | --- |
| 查询表单 | `inputSchema` 与可生成的 `inputFormilySchema` | 可做单函数调用；Page 候选为 `needs_review` |
| 分页表格 | 分页输入字段、`items` 路径、`total` 路径、列定义或可验证的输出对象 schema | 禁止生成 ready `DataTable` |
| 行/详情/批量操作 | 显式 `inputMapping` 所需字段可从 selected row/detail/page state 取得 | 禁止把整行对象盲传给函数 |
| 任务页面 | `executionMode=task` 或等价的异步任务契约，以及 task status/events/result 来源 | 只能生成 blocked/needs_review 候选 |
| 报表图表 | 显式 chart type、series/category/value 路径或结构化报表契约 | 只能生成结果面板，不能假装是可用图表 |

通用 JSON Schema 负责输入/输出结构；Server 先按确定性规则生成保守候选，无法无歧义得出的页面映射由 Page Studio 中的页面管理员填写。关键原则是：**函数注册不包含 UI；发布产物中的每个数据路径都必须可验证，不能依赖函数名、第一批响应或前端猜测。**

## operation 与页面语义

`operation` 只表示业务动作 key：

```text
list / get / ban / grant / send / rollback / refresh
```

它不是页面类型，也不是按钮位置。SDK/OpenAPI 注册不得提供 `operationKind`、`placement` 或 `pageHint`。这些字段一旦出现在注册输入中，说明函数提供者正在承担 Dashboard 页面设计责任，必须在注册边界拒绝，或在 Source diagnostics 中提示迁移到 Page Studio。

页面候选语义由 Server 内部模型表达：

| 内部语义 | 含义 | 来源 |
| --- | --- | --- |
| `PageCandidate.kind` | `entity`、`operation`、`task`、`report` 等页面候选类型 | Server 根据 FunctionSpec、JSON Schema、执行模式和明确 PageContract 做确定性分析；无法确认时标为 `needs_review` |
| `PageFunctionBinding.usage` | `query`、`detail`、`action`、`task`、`report` 等页面内用途 | PageSpec 生成或 Page Studio 编辑保存 |
| 组件位置 | 行操作、工具栏、独立页、图表、表格等页面位置 | PageSpec 的 Formily schema 和 binding mapping |

因此 `player.ban` 的注册只应表达业务动作：

```yaml
operationId: player.ban
x-resource: player
x-operation: ban
x-risk: danger
```

如果 Server 能从契约中确认它需要选中玩家并执行高风险同步命令，可以生成一个待确认的行操作候选；如果不能确认，只给出 diagnostics。无论哪种情况，都不能自动发布正式页面。

## 页面放置与 binding

页面放置属于 PageSpec，不属于函数注册。

在正式页面中，按钮、表格、详情、分页和图表都通过 PageSpec 表达：

```text
PageSpec.bindings[].usage
PageSpec.bindings[].inputMapping
PageSpec.bindings[].outputMapping
PageSpec.schema.properties[*].x-component
PageSpec.schema.properties[*].x-component-props.bindingId
```

Server 生成器只能产出 `GeneratedPageCandidate`，并标注它为什么 ready、needs_review 或 blocked。Page Studio 保存草稿时才确定最终页面类型、分类、标题、组件位置、绑定、映射和确认策略；发布时冻结为 `PublishedPageSpec`。

## 多语言字段

SDK/OpenAPI 注册不提供动态菜单、分类、资源或页面标题的多语言显示名。

允许进入函数能力契约的文本只有：

- `summary`：函数目录、搜索和候选说明的一句话描述。
- `description`：函数目录、帮助文案和候选说明的详细描述。
- JSON Schema 的 `title` / `description`：输入输出字段说明，可用于 Server 派生 Function Form。

动态页面文案只来自 PageSpec：

| 文案 | 来源 | 确定时机 |
| --- | --- | --- |
| 运行控制台分类标题 | `PageSpec.category.labels` | PageSpec 保存或发布 |
| 页面标题 | `PageSpec.title` | PageSpec 保存或发布 |
| 页面组件标题、按钮文案、表格列名 | `PageSpec.schema` 中受控组件 props 或列定义 | Page Studio 编辑 |

前端静态 locale 只用于固定系统菜单。动态分类和页面标题不得写入 `web/src/locales/*/menu.ts`，也不得由 SDK/OpenAPI descriptor 提供。

## OpenAPI 扩展示例

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
      x-resource: player
      x-operation: ban
      x-risk: danger
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

## SDK descriptor 示例

```json
{
  "id": "player.ban",
  "version": "1.0.0",
  "tags": ["player", "moderation"],
  "summary": "封禁玩家",
  "description": "封禁指定玩家账号，可设置原因和时长。",
  "resource": "player",
  "operation": "ban",
  "risk": "danger",
  "input_schema": {
    "type": "object",
    "required": ["player_id", "reason"],
    "properties": {
      "player_id": { "type": "string", "title": "玩家 ID" },
      "reason": { "type": "string", "title": "封禁原因" },
      "duration_seconds": { "type": "integer", "title": "封禁时长" }
    }
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "success": { "type": "boolean" },
      "ban_id": { "type": "string" }
    }
  }
}
```

## proto 兼容策略

`LocalFunctionDescriptor` 的核心字段应只覆盖函数能力契约：

```text
id / version / tags / summary / description / operation_id / deprecated
input_schema / output_schema / resource / operation / risk / enabled / permission
```

`extensions` 只能作为第三方非核心扩展出口，不能承载平台 UI、菜单、页面放置或动态显示语义。以下字段不得进入 proto、SDK builder 或 OpenAPI 注册白名单：

```text
category_display / entity_display / operation_display
operation_kind / placement / page_hint
x-category-display / x-entity-display / x-operation-display
x-operation-kind / x-placement / x-page-hint
ui / x-ui / formily / menu / route / table columns / page schema
```

如果现有代码已经把这些字段加进 SDK 或 proto，需要删除，而不是保留兼容。旧数据如需迁移，只能迁移到 PageSpec 草稿或 Page Studio 候选，不得继续作为注册字段读取。

## 归一化规则

Server 解析 descriptor 时按以下顺序处理：

1. 校验 `id` 和 `version`。
2. 读取 OpenAPI 标准字段或 SDK descriptor 基础字段。
3. 读取 `resource`、`operation`、`risk`、`enabled`、`permission` 等业务与治理字段。
4. 拒绝 `ui/x-ui/Formily/menu/route/table columns` 以及 `operationKind/placement/pageHint/display` 等越界字段。
5. 生成单函数 Formily Schema。
6. 生成 FunctionSpec。
7. 按显式 `resource`、OpenAPI tag、函数 ID 前缀和确定性契约分析生成 ResourceSpec / OperationSpec 候选。
8. 对缺失字段、契约不可验证或 UI 字段越界生成 diagnostics。
9. 只有 PageCandidate 的契约、mapping、labels 和 binding 都通过校验，才能进入 PageSpec 草稿或发布流程。

允许 Server 给出建议，但建议必须带诊断来源：

| 诊断 | 处理 |
| --- | --- |
| 缺少 `resource` | 仍进入函数目录；可生成独立操作候选或待编排建议，不猜对象管理页 |
| 缺少 `operation` | 仍进入函数目录；PageCandidate 质量降低，需要 Page Studio 确认 |
| 缺少可验证分页、列或 mapping 契约 | 禁止生成 ready DataTable，只能 `needs_review` 或 `blocked` |
| 注册输入包含 UI、显示、多语言菜单或页面放置字段 | 拒绝注册或导入该 operation，并返回越界诊断 |
| PageSpec 缺少默认语言 labels | 草稿可保存，发布失败 |

## 与 PageSpec 的关系

descriptor v2 只提供函数能力契约。它不是 PageSpec，也不包含最终页面语义。

正确链路：

```text
descriptor v2
  -> FunctionSpec
  -> ResourceSpec + OperationSpec
  -> GeneratedPageCandidate
  -> Page Studio 确认分类、标题、组件、binding 和 mapping
  -> 用户编辑 PageSpec
  -> 发布 PublishedPageSpec
```

错误链路：

```text
OpenAPI Operation
  -> 前端运行时临时生成菜单和页面
```

## 禁止项

- 禁止把 `x-operation` 继续定义成页面类型枚举。
- 禁止在注册字段、SDK builder、OpenAPI extension 中加入动态显示名、菜单、`operationKind`、`placement` 或 `pageHint`。
- 禁止没有明确 PageSpec、映射、labels 和发布确认就自动发布页面。
- 禁止动态分类写入静态 locale 文件。
- 禁止前端根据函数 ID 后缀推断正式 Page。
- 禁止 OpenAPI 直接作为 Dashboard Page Schema。
- 禁止在 OpenAPI / SDK descriptor 注册中接受 `x-ui` / `ui`、Formily、菜单或页面组件配置。

## 迁移策略

### 阶段 1：文档和 DTO 收敛

- 更新 OpenAPI 注册文档。
- 更新 SDK conventions。
- 后端 descriptor 输出收敛为能力契约字段。
- 前端类型删除注册侧 UI、显示、页面放置字段。

### 阶段 2：Server 归一化服务

- 新增 FunctionSpec / ResourceSpec / OperationSpec 强类型模型。
- `operation` 固定为业务操作 key。
- 旧 `category/entity` 只能作为迁移输入转换为 `resource` 或 PageSpec 草稿，不得作为运行菜单事实源。

### 阶段 3：PageSpec 生成

- Server 根据 OperationSpec 生成 PageSpec 建议。
- 缺少可验证契约、mapping 或 labels 时只生成待编排建议。
- Page Studio 负责确认和编辑。

### 阶段 4：SDK 升级

- 多语言 SDK 只统一能力契约字段和 OpenAPI helper 能力，不增加 UI、显示、菜单、页面放置字段。
- SDK demo 必须展示函数契约、输入/输出 schema、风险和业务归属 key；不得嵌入 Formily/Page UI。
- 如果 SDK/OpenAPI 输入包含旧 UI 或页面语义字段，必须失败并给出迁移到 Page Studio 的诊断。
