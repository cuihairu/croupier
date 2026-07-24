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
| 函数注册 / OpenAPI 导入 | ID、版本、输入/输出 schema、描述、风险和可选业务 hints | FunctionSpec | 服务开发者或 OpenAPI 所有者 | 否 |
| Server 归一化与生成 | FunctionSpec、JSON Schema、可选 hints | 自动派生 Function Form + 保守 PageCandidate + diagnostics | Server | 否，可随契约重新生成 |
| Page Studio | PageCandidate、binding、数据映射、组件 ABI | PageDraft -> PublishedPageSpec | 页面管理员 / 运营 | 是，发布后冻结 |

具体规则：

- **函数表单**在 Server 读取 `inputSchema` 时自动派生为 Formily；没有 schema 时仅显示 JSON `payload` 输入和诊断。它不是 SDK 传来的 UI。
- **页面候选**在 Resource/Operation 归一化之后生成；Server 只做可验证的保守推导，无法确定列表字段、分页、动作输入或图表映射时标为 `needs_review`，不猜测。
- **最终页面 UI**只在 Page Studio 保存并在发布时确定。菜单、页面组件、表格列、数据 mapping、binding 和确认策略都属于 PageSpec，绝不回写到函数注册或 OpenAPI 文档。
- 函数表单可由管理员在注册后保存 override；override 仍只管输入体验，不影响 PageSpec 的布局和导航。

因此服务开发者只需把业务请求/响应契约写清楚。`operationKind`、`placement`、分类和显示名都只能提高候选质量，不能成为“必须懂前端才能注册函数”的门槛。

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

### 用户上传 OpenAPI 契约源（目标，P0-11 未实现）

目标是让用户在 Dashboard 上传 JSON 或 YAML 的 OpenAPI 3.x 文档。上传创建 `OpenAPISource`，保留来源、hash、校验 diagnostics、scope 和操作清单，而不是直接伪造可执行 Function。

```text
OpenAPI file / pasted document
  -> OpenAPISource (validated contract catalog)
  -> ExecutionBinding (optional)
  -> FunctionSpec
  -> Function Form / PageCandidate
```

`ExecutionBinding` 必须是以下二者之一：

- `provider`：将 operationId 显式绑定到当前 scope 内已注册的 SDK Provider Handler。
- `httpConnector`：由平台��理的受控 HTTP 调用器，使用独立配置的 base URL、SecretRef、允许的 host、超时、重试和 scope 策略。

未绑定执行器的 Source 只能作为契约目录和页面候选输入，不能发布包含可执行 binding 的页面；绑定后才形成可调用 FunctionSpec。HTTP Connector 的凭据和目标地址不允许写进 OpenAPI 文件、PageSpec 或前端请求。

目标 API：

```text
POST   /api/v1/openapi/sources                 # multipart file 或 raw JSON/YAML
GET    /api/v1/openapi/sources/:sourceId
POST   /api/v1/openapi/sources/:sourceId/bindings
DELETE /api/v1/openapi/sources/:sourceId/bindings/:bindingId
GET    /api/v1/openapi/sources/:sourceId/diagnostics
```

历史 `POST /api/v1/openapi/import` 只接受 JSON object，并把 operation 存到 registry；它没有 Handler/Connector binding、scope、source version 或执行安全策略，不能作为目标上传 API。

## 为什么需要 v2

旧模型只有：

```text
category / entity / operation / risk
```

这不足以稳定生成游戏运营后台页面，原因是：

- `operation` 被混用成 CRUD 类型和业务动作名。
- 没有字段表达函数应放在查询区、表格数据源、行操作、详情操作还是独立页面。
- `category` 只有 key，没有动态多语言显示名。
- 没有区分 Entity Page、Operation Page、Task Page、Report Page。
- 缺少默认页面生成所需的显式语义，只能退回前端猜测。

v2 的核心变化：

```text
operation      = 业务操作 key，例如 ban / grant / send
operationKind  = 可选页面候选语义提示，例如 list / get / action / task / report
placement      = 可选页面候选放置提示，例如 tableData / rowAction / standalone
display labels = 动态多语言文案，随注册或 PageSpec 发布
```

## 三层边界

| 层 | 输入/输出 | 职责 |
| --- | --- | --- |
| Raw Descriptor | OpenAPI Operation / SDK descriptor | 承载函数契约和扩展字段 |
| FunctionSpec / ResourceSpec / OperationSpec | Server 归一化产物 | 清洗、校验、补全、诊断 |
| PageSpec | Server 生成或用户编辑 | 表达完整业务页面，必须是 Formily JSON Schema |

OpenAPI 和 SDK descriptor 可以存在兼容字段，但 PageSpec 生成只能依赖归一化后的强类型模型。描述符表达的是函数**当前契约**；Page 发布时必须把实际依赖的函数契约摘要冻结到 PublishedPageSpec，运行时不能直接把“最新 descriptor”当作已发布页面的事实。

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

### 治理字段

| 平台字段 | OpenAPI 扩展 | SDK descriptor 字段 | 说明 |
| --- | --- | --- | --- |
| `category` | `x-category` | `category` | 分类 key，不是显示名 |
| `risk` | `x-risk` | `risk` | `safe` / `warning` / `high` / `danger` |
| `enabled` | `x-enabled` | `enabled` | 是否启用 |
| `permission` | `x-permission` | `permission` | 可选权限标识 |

### Resource/Page 语义字段

| 平台字段 | OpenAPI 扩展 | SDK descriptor 字段 | 必要性 | 说明 |
| --- | --- | --- | --- | --- |
| `categoryDisplay` | `x-category-display` | `category_display` | 推荐 | 分类多语言标题 |
| `entity` | `x-entity` | `entity` | 推荐 | 资源 key，例如 `player` |
| `entityDisplay` | `x-entity-display` | `entity_display` | 推荐 | 资源多语言标题 |
| `operation` | `x-operation` | `operation` | 推荐 | 业务操作 key，例如 `ban` |
| `operationDisplay` | `x-operation-display` | `operation_display` | 推荐 | 操作多语言标题 |
| `operationKind` | `x-operation-kind` | `operation_kind` | 可选提示 | 默认页面候选语义 |
| `placement` | `x-placement` | `placement` | 可选提示 | 默认页面候选放置位置 |
| `pageHint` | `x-page-hint` | `page_hint` | 可选 | 目标页面建议 |

函数注册的最小要求只是可执行能力契约，不包含任何 Formily 或 Page UI。`operationKind` 和 `placement` 存在时可提高默认候选质量；缺失时函数仍正常注册，Server 只能生成保守候选或 `needs_review` diagnostics。无论是否提供，函数注册都不会自动发布正式 Page。

## 页面生成还需要的契约信息

v2 不只为分类和菜单服务。若要把生成结果标为 `ready`，生成器还必须能验证输入、输出和执行约束；不能只根据 `operationKind + placement` 拼一个外观类似的页面。

| 目标能力 | 描述符/归一化模型必须提供 | 缺失时的结果 |
| --- | --- | --- |
| 查询表单 | `inputSchema` 与可生成的 `inputFormilySchema` | 可做单函数调用；Page 候选为 `needs_review` |
| 分页表格 | 分页输入字段、`items` 路径、`total` 路径、列定义或可验证的输出对象 schema | 禁止生成 ready `DataTable` |
| 行/详情/批量操作 | 显式 `inputMapping` 所需字段可从 selected row/detail/page state 取得 | 禁止把整行对象盲传给函数 |
| 任务页面 | `executionMode=task` 或等价的异步任务契约，以及 task status/events/result 来源 | 只能生成 blocked/needs_review 候选 |
| 报表图表 | 显式 chart type、series/category/value 路径或结构化报表契约 | 只能生成结果面板，不能假装是可用图表 |

这些字段不要求全部由函数提供者填写。通用 JSON Schema 负责输入/输出结构；Server 先按确定性规则生成保守候选，无法无歧义得出的页面映射由 Page Studio 中的页面管理员填写。关键原则是：**函数注册不包含 UI；发布产物中的每个数据路径都必须可验证，不能依赖函数名、第一批响应或前端猜测。**

如果服务团队愿意提供机器可读的页面提示，可在 OpenAPI 加入受版本控制的 `x-page-contract`。它完全可选，SDK 也可以不提供等价字段；缺失时不影响函数注册，只降低自动候选质量。该扩展不得包含 Formily、菜单、路由或任意前端组件配置：

```yaml
x-page-contract:
  version: v1
  executionMode: sync # sync | task
  pagination:
    pageField: page
    pageSizeField: pageSize
    itemsPath: $.response.items
    totalPath: $.response.total
  table:
    columns:
      - key: id
        title: { zh-CN: 玩家 ID, en-US: Player ID }
        valuePath: id
```

`x-page-contract` 只是生成候选的输入，不是 PageSpec 的替代品。生成后，组件 props、bindingId 和输入/输出映射仍必须写入 PageSpec 并由发布校验冻结。

## operation 与 operationKind

`operation` 是业务动作名：

```text
list / get / ban / grant / send / rollback / refresh
```

`operationKind` 是平台页面生成语义：

| `operationKind` | 含义 | 示例 |
| --- | --- | --- |
| `list` | 列表查询 | `player.list` |
| `get` | 单对象读取 | `player.get` |
| `create` | 新建对象 | `mail.template.create` |
| `update` | 更新对象 | `player.update` |
| `delete` | 删除对象 | `item.delete` |
| `action` | 同步命令 | `player.ban`、`mail.send` |
| `task` | 异步任务 | `reward.batchGrant` |
| `report` | 报表查询 | `analytics.retention` |

禁止继续把 `x-operation` 解释成页面类型。如果函数是 `player.ban`：

```yaml
x-operation: ban
x-operation-kind: action
```

`x-operation` 必须是稳定业务动作名，不能写成泛化分类或页面类型。

## placement

`placement` 描述默认候选中建议放在页面哪里：

| `placement` | 含义 | 典型函数 |
| --- | --- | --- |
| `query` | 查询表单 | `player.search` |
| `tableData` | 表格数据源 | `player.list` |
| `detailData` | 详情数据源 | `player.get` |
| `rowAction` | 表格行操作 | `player.ban` |
| `detailAction` | 详情操作 | `player.resetPassword` |
| `toolbarAction` | 页面工具栏 | `mail.send` |
| `batchAction` | 批量操作 | `reward.batchGrant` |
| `standalone` | 独立页面 | `cache.refresh` |

`placement` 不能由前端根据函数名推断。它是可选的 descriptor 提示；缺失时由 Server 输出保守候选和 diagnostics。发布正式 Page 前，实际位置和 mapping 必须明确写入 PageSpec。

## 多语言字段

动态显示名使用 `LocalizedText`：

```json
{
  "zh-CN": "封禁",
  "en-US": "Ban"
}
```

SDK 为了书写方便，可以临时接受短 key：

```json
{
  "zh": "封禁",
  "en": "Ban"
}
```

Server 归一化时必须转换成完整 locale key，例如 `zh-CN`、`en-US`。

动态文案不得写入前端静态 locale 文件：

- `categoryDisplay` 用于运行控制台分类。
- `entityDisplay` 用于资源标题。
- `operationDisplay` 用于按钮、动作标题和默认页面块。
- `summary` / `description` 用于帮助文案和搜索。

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
      x-category: support
      x-category-display:
        zh-CN: 客服
        en-US: Support
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
  "category": "support",
  "category_display": {
    "zh-CN": "客服",
    "en-US": "Support"
  },
  "entity": "player",
  "entity_display": {
    "zh-CN": "玩家",
    "en-US": "Player"
  },
  "operation": "ban",
  "operation_display": {
    "zh-CN": "封禁",
    "en-US": "Ban"
  },
  "operation_kind": "action",
  "placement": "rowAction",
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

当前 `LocalFunctionDescriptor` 已有一等字段：

```text
id / version / tags / summary / description / operation_id / deprecated
input_schema / output_schema / category / risk / entity / operation
```

v2 新字段短期可以通过扩展字段承载，避免立即破坏多语言 SDK：

```json
{
  "extensions": {
    "x-category-display": "{\"zh-CN\":\"客服\",\"en-US\":\"Support\"}",
    "x-entity-display": "{\"zh-CN\":\"玩家\",\"en-US\":\"Player\"}",
    "x-operation-display": "{\"zh-CN\":\"封禁\",\"en-US\":\"Ban\"}",
    "x-operation-kind": "action",
    "x-placement": "rowAction"
  }
}
```

长期应升级 proto，把 v2 字段提升为一等字段，并保留 `extensions` 作为第三方扩展出口。

目标字段：

```protobuf
map<string, string> category_display = 14;
map<string, string> entity_display = 15;
map<string, string> operation_display = 16;
string operation_kind = 17;
string placement = 18;
string page_hint = 19;
map<string, string> extensions = 20;
```

## 归一化规则

Server 解析 descriptor 时按以下顺序处理：

1. 校验 `id` 和 `version`。
2. 读取 OpenAPI 标准字段或 SDK descriptor 基础字段。
3. 读取 v2 扩展字段。
4. 归一化 locale key。
5. 生成单函数 Formily Schema。
6. 生成 FunctionSpec。
7. 按显式 `entity`、可选语义提示和确定性契约分析生成 ResourceSpec / OperationSpec 候选。
8. 对缺失字段生成诊断。
9. 只有诊断通过的 OperationSpec 才能进入 PageSpec 默认生成。

允许 Server 给出建议，但建议必须带诊断来源：

| 诊断 | 处理 |
| --- | --- |
| 缺少 `entity` | 仍进入函数目录；可生成独立操作候选或待编排建议，不猜对象管理页 |
| 缺少 `operationKind` | 生成保守候选或待编排建议 |
| 缺少 `placement` | 生成保守候选或待编排建议 |
| 缺少动态 labels | 可以预览，不允许发布到运行控制台 |

## 与 PageSpec 的关系

descriptor v2 只提供页面生成所需语义。它不直接等于 PageSpec。

正确链路：

```text
descriptor v2
  -> OperationSpec
  -> Generated PageSpec
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
- 禁止没有明确 PageSpec、映射和发布确认就自动发布页面。
- 禁止动态分类写入静态 locale 文件。
- 禁止前端根据函数 ID 后缀推断正式 Page。
- 禁止 OpenAPI 直接作为 Dashboard Page Schema。
- 禁止在 OpenAPI / SDK descriptor 注册中接受 `x-ui` / `ui`、Formily、菜单或页面组件配置。

## 迁移策略

### 阶段 1：文档和 DTO 收敛

- 更新 OpenAPI 注册文档。
- 更新 SDK conventions。
- 后端 descriptor 输出透出 v2 字段。
- 前端类型补齐 v2 字段。

### 阶段 2：Server 归一化服务

- 新增 FunctionSpec / ResourceSpec / OperationSpec 强类型模型。
- 旧 `category/entity/operation` 继续读取，但 `operation` 改为业务操作 key。
- 旧 `operation` 泛化分类不再用于 Page 生成，只作为待编排诊断。

### 阶段 3：PageSpec 生成

- Server 根据 OperationSpec 生成 PageSpec 建议。
- 缺字段只生成待编排建议。
- Page Studio 负责确认和编辑。

### 阶段 4：SDK 升级

- 多语言 SDK 增加 v2 descriptor 字段。
- 旧 SDK 可继续注册函数；Server 仍可生成保守候选，但绝不自动生成正式 Page。
- SDK demo 必须展示函数契约；页面提示字段可作为可选增强示例，不能成为接入门槛。
