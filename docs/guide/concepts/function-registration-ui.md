---
title: 函数注册与默认界面
order: 4
category:
  - 核心概念
tag:
  - 函数注册
  - 默认界面
  - Formily
---

# 函数注册与默认界面

Croupier 采用“函数契约驱动、平台生成 UI”的模型：业务服务只注册函数能力，Server 根据函数契约生成单函数 Formily 表单，并进一步生成 PageSpec 候选。函数提供者不需要也不应该在注册时设计界面。

函数注册字段以 [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md) 为准。本文只说明这些字段如何影响默认 UI。

运行时只有一种 UI 协议：**Formily JSON Schema**。单函数输入表单和页面级 PageSpec 都必须使用 Formily。非 Formily Schema 必须报错，不能转换、猜测或静默降级。

## 目标

- 函数注册后能在 Dashboard 中被发现、理解和调用。
- SDK 只声明能力契约，不绑定具体前端实现，也不提交 Formily/Page Schema。
- 默认函数表单由 Server 生成，生成结果必须是 Formily Schema。
- 默认业务页面由 Server 基于 FunctionSpec / ResourceSpec / OperationSpec 生成 PageSpec 建议。
- 页面管理员可以在注册后编辑函数表单 override 或 PageSpec；两者都不是 SDK 注册输入。
- 函数表单和 Dashboard Page 分离，避免把所有函数强行塞进 Entity Page。

## 注册元信息

函数注册的最小字段只有 `id` 和 `version`。为了生成稳定的目录、权限、表单和页面候选能力，应补齐推荐字段。

| 字段 | 级别 | 用途 |
| --- | --- | --- |
| `id` | 必填 | 全局函数 ID，建议使用 `<domain>.<entity>.<operation>`，如 `game.player.get` |
| `version` | 必填 | 函数版本，用于实例选择和变更排查 |
| `summary` | 推荐 | 函数一句话简介，用于目录标题、页面副标题和搜索 |
| `description` | 推荐 | Markdown 详细说明，用于函数信息抽屉和帮助文案 |
| `tags` | 推荐 | 目录聚合和搜索过滤 |
| `category` | 推荐 | 顶层分类，如 `game`、`player`、`order`、`system` |
| `category_display` | 推荐 | 分类多语言显示名，用于动态菜单 |
| `entity` | 推荐 | 业务实体，如 `player`、`order`、`mail` |
| `entity_display` | 推荐 | 资源多语言显示名 |
| `operation` | 推荐 | 业务操作 key，如 `list`、`get`、`create`、`update`、`delete`、`ban`、`grant`、`send` |
| `operation_kind` | 可选 | 默认页面候选语义提示，如 `list`、`get`、`action`、`task`、`report` |
| `placement` | 可选 | 默认候选放置提示，如 `query`、`tableData`、`rowAction`、`toolbarAction`、`standalone` |
| `risk` | 推荐 | 治理等级，如 `safe`、`warning`、`danger` |
| `input_schema` | 推荐 | 请求体 JSON Schema，用于生成 Formily 表单初稿 |
| `output_schema` | 推荐 | 响应 JSON Schema，用于结果视图、表格和页面编排 |
| `operation_id` | 可选 | OpenAPI operationId，默认可使用 `id` |
| `deprecated` | 可选 | 标记函数是否已废弃 |

最小注册只需要可执行函数的 ID、版本和输入/输出契约。缺失语义字段不影响注册或函数表单派生，只会让页面候选更保守并附带 diagnostics；函数注册永远不会自动发布页面。

## SDK 注册契约

多语言 SDK 注册函数时应把同一组元信息写入 provider connect 和 manifest。Dashboard 不直接理解某一种 SDK 的内部结构，只消费平台统一后的 descriptor。

推荐注册内容：

```typescript
client.registerFunction(
  {
    id: 'player.ban',
    version: '1.0.0',
    summary: '封禁玩家',
    description: '封禁指定玩家账号，可设置封禁原因和时长。',
    tags: ['player', 'moderation'],
    category: 'player',
    category_display: { zh: '玩家', en: 'Players' },
    entity: 'player',
    entity_display: { zh: '玩家', en: 'Player' },
    operation: 'ban',
    risk: 'danger',
    input_schema: {
      type: 'object',
      properties: {
        player_id: { type: 'string', title: '玩家 ID' },
        reason: { type: 'string', title: '封禁原因' },
        duration_seconds: { type: 'integer', title: '封禁时长（秒）', minimum: 60 }
      },
      required: ['player_id', 'reason']
    },
    output_schema: {
      type: 'object',
      properties: {
        success: { type: 'boolean', title: '是否成功' },
        ban_id: { type: 'string', title: '封禁记录 ID' }
      }
    }
  },
  async (_ctx, payload) => JSON.stringify({ success: true, ban_id: 'ban_001' })
);
```

字段流转规则：

- `summary` 用于目录标题、函数信息和默认 UI 标题。
- `description` 用于帮助说明、OpenAPI description 和函数信息抽屉。
- `input_schema` 是 Server 派生 Function Form 的唯一输入 UI 事实来源。
- `output_schema` 用于结果视图、表格列和 Page 候选分析。
- `category/entity/operation/risk/tags` 用于资源归一化、权限治理、页面候选能力和搜索；`operation_kind/placement` 只是可选候选提示。
- `category_display/entity_display/displayName/summary` 用于动态菜单、资源标题、页面标题和帮助文案。

## OpenAPI 与 Formily 的关系

OpenAPI 是函数契约，不是 Dashboard Page。

允许的数据流：

```text
OpenAPI / SDK descriptor
    -> input_schema / output_schema
    -> Server 生成 Formily Schema
    -> Dashboard SchemaRenderer 渲染
```

不允许的数据流：

```text
OpenAPI / JSON Schema
    -> Dashboard 运行时临时推断组件
    -> 非 Formily Schema（直接报错）
```

OpenAPI operation 和 SDK descriptor 不接受 `x-ui` / `ui`。单函数 Formily 表单由 Server 从 `input_schema` 派生；管理员需要优化时，在注册后通过函数表单配置保存 override。

OpenAPI 扩展字段建议：

| OpenAPI 扩展 | 平台字段 |
| --- | --- |
| `x-category` | `category` |
| `x-category-display` | `category_display` |
| `x-entity` | `entity` |
| `x-entity-display` | `entity_display` |
| `x-operation` | `operation` |
| `x-operation-kind` | 可选 `operation_kind` 候选提示 |
| `x-placement` | 可选 `placement` 候选提示 |
| `x-risk` | `risk` |

`x-operation` 是业务操作 key，例如 `ban`、`grant`、`send`。页面候选语义可使用 `x-operation-kind`，但没有它仍可注册；不能继续把 `x-operation` 当成页面类型。

## 默认 UI 解析顺序

Server 按以下优先级解析 Function Form：

1. Page Studio/函数目录中由管理员保存的函数表单 override。
2. 根据 `input_schema` 或 OpenAPI request schema 派生 Formily Schema。
3. 缺失输入 schema 时生成单个 JSON `payload` 字段和 `input_schema_missing` diagnostic。

每一层输出都必须是 Formily Schema。前端执行页不再自己生成、转换或降级。

## 默认生成规则

当存在结构化 `input_schema` 时，Server 按 JSON Schema 生成 Formily Schema：

| JSON Schema | Formily component |
| --- | --- |
| `string` | `Input` |
| `string` + `enum` | `Select` |
| `string` + `format: date` | `DatePicker` |
| `string` + `format: time` | `TimePicker` |
| `number` / `integer` | `NumberPicker` |
| `boolean` | `Switch` |
| `array` + `items.enum` | `Select` + `mode: multiple` |
| `array` + `items.object` | `ArrayTable` / `ArrayItems` |
| `object` | `Card` + nested `properties` |

当没有可用 `input_schema` 时，Server 只能生成最小 Formily Schema：

- 使用单个 `payload` 对象字段承载业务入参。
- 不根据函数名推断对象 ID、查询字段、CRUD 字段或组件类型。
- 用户需要自定义字段时，必须补充 `input_schema` 或在 UI 编辑器里保存完整 Formily Schema。

这只是单函数 Formily 初稿，不是 PageSpec，也不是第二套 UI 协议。非 Formily UI 输入必须直接报错。

## 默认字段与自定义字段

函数注册只有 `id` 和 `version` 时，平台不知道业务字段。此时默认函数表单只能是：

```json
{
  "type": "object",
  "properties": {
    "payload": {
      "type": "object",
      "title": "Payload",
      "x-component": "Input.TextArea",
      "x-decorator": "FormItem"
    }
  }
}
```

如果用户希望界面展示 `player_id`、`reason`、`duration_seconds` 这类字段，必须通过以下任一方式明确声明：

- 在函数 descriptor 的 `input_schema.properties` 中声明字段。
- 在注册后的函数表单配置中保存完整 Formily Schema override。
- 在 OpenAPI operation 的 `requestBody` 中声明。

规则：

- Server 可以从 `input_schema` 生成 Formily 字段。
- Server 不能根据 `player.ban` 这种函数名猜出 `player_id` 字段。
- UI Schema 中新增字段必须仍然是 Formily Schema。
- UI Schema 与 `input_schema` 明显冲突时必须报错或要求同步更新契约，不能静默丢字段。
- PageSpec 可以复用函数表单，但不能改变函数 payload 契约。

## Function Form 与 Page 的边界

Function Form 只负责单个函数的输入表单。

Page 负责完整业务页面编排，包括：

- 查询区。
- 分页状态。
- 表格列。
- 详情区。
- 弹窗表单。
- 行操作。
- 批量操作。
- 异步任务结果。
- 图表和报表。

例如 `player.manage` 页面可以组合多个函数：

```text
查询区 -> player.list
分页表格 -> player.list.response.items
行详情 -> player.get
行操作 -> player.ban / player.update
批量操作 -> reward.batchGrant
```

因此，不是所有函数都应该进入 Entity Page。只有明确属于某个 `entity` 生命周期的函数，才进入 Entity Page。全局命令、报表、任务、运维动作应进入 Operation Page、Report Page、Task Page 或 Tool Page。

Page Schema 也必须是 Formily JSON Schema。页面级组件通过 `x-component` 表达，例如 `ConsolePage`、`QueryForm`、`DataTable`、`DetailPanel`、`ActionButton`、`TaskTimeline`、`ChartPanel`。

## 实施边界

- SDK demo 必须注册 `summary`、`description`、`input_schema` 和 `output_schema`。
- SDK demo 只要求注册函数契约。`entity`、动态 labels、`operation_kind` 和 `placement` 是提高候选质量的可选增强，不是 UI 接入门槛。
- 后端 descriptor 必须透出 `displayName`、`summary`、`inputSchema`、`outputSchema` 和治理字段。
- 后端归一化层必须输出 FunctionSpec / ResourceSpec / OperationSpec / PageSpec 建议。
- 函数表单 API 只读写由 Server 派生或管理员 override 的 Formily Schema，不接受 SDK/OpenAPI 直接提交 UI。
- 前端调用页只使用 `SchemaRenderer` 渲染 Formily Schema。
- UI 编辑器只编辑 Formily Schema。
- Page 生成和 Entity Page 生成必须基于明确的 `entity + actions`，不能把全部函数套进 CRUD 模板。
- 运行控制台菜单只来自已发布 PageSpec，不来自静态 locale 文件或函数目录。

## 相关文档

- [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)
- [Dashboard Resource/Page 模型](../../architecture/dashboard-page-model.md)
