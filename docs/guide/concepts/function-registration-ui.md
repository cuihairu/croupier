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

Croupier 采用函数注册驱动的默认界面模型：业务服务注册函数能力，Server 根据函数契约生成 Formily 表单初稿，Dashboard 直接渲染 Formily Schema。

运行时只有一种函数 UI 协议：**Formily Schema**。非 Formily Schema 必须报错，不能转换、猜测或静默降级。

## 目标

- 函数注册后能在 Dashboard 中被发现、理解和调用。
- SDK 只声明能力契约，不绑定具体前端实现。
- 默认函数表单由 Server 生成，生成结果必须是 Formily Schema。
- 用户可以编辑 Formily Schema，但保存内容仍必须是完整 Formily Schema。
- 函数表单和 Dashboard Page 分离，避免把所有函数强行塞进对象管理页。

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
| `entity` | 推荐 | 业务实体，如 `player`、`order`、`mail` |
| `operation` | 推荐 | 操作类型，如 `list`、`read`、`create`、`update`、`delete`、`ban`、`grant`、`custom` |
| `risk` | 推荐 | 治理等级，如 `safe`、`warning`、`danger` |
| `input_schema` | 推荐 | 请求体 JSON Schema，用于生成 Formily 表单初稿 |
| `output_schema` | 推荐 | 响应 JSON Schema，用于结果视图、表格和页面编排 |
| `operation_id` | 可选 | OpenAPI operationId，默认可使用 `id` |
| `deprecated` | 可选 | 标记函数是否已废弃 |

缺失字段可以用于生成最小 Formily 表单，但缺失元信息会降低目录、权限和页面编排质量。

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
    entity: 'player',
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
- `input_schema` 是生成 Formily 表单初稿的事实来源。
- `output_schema` 用于结果视图、表格列和 Page 编排。
- `category/entity/operation/risk/tags` 用于菜单归类、权限治理、页面候选能力和搜索。

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

OpenAPI operation 的 `x-ui` 如果存在，也必须直接是 Formily Schema。

## 默认 UI 解析顺序

Server 按以下优先级解析函数 UI：

1. 数据库中的 `functions.metadata.ui`。
2. `configs/ui/functions.override/<function-id>.yaml|json`。
3. `configs/ui/functions/<function-id>.yaml|json`。
4. OpenAPI operation 的 `x-ui`。
5. 根据 `input_schema` 或 OpenAPI request schema 生成 Formily Schema。
6. 根据 `id/entity/operation` 生成最小 Formily Schema。

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

当没有可用 schema 时，Server 生成最小 Formily Schema：

- `create/add/new`：`data` 对象字段。
- `read/get/query/search/detail`：对象 ID 或查询字段。
- `update/edit/patch`：对象 ID 和 patch 字段。
- `delete/remove`：对象 ID 字段。
- 其他操作：`payload` 对象字段。

这只是 Formily 初稿，不是第二套 UI 协议。

## Function UI 与 Page 的边界

Function UI 只负责单个函数的输入表单。

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

因此，不是所有函数都应该进入对象管理页。只有明确属于某个 `entity` 生命周期的函数，才进入 Entity Page。全局命令、报表、任务、运维动作应进入 Operation Page、Report Page、Task Page 或 Tool Page。

## 实施边界

- SDK demo 必须注册 `summary`、`description`、`input_schema` 和 `output_schema`。
- 后端 descriptor 必须透出 `displayName`、`summary`、`inputSchema`、`outputSchema` 和治理字段。
- `/api/v1/functions/:id/ui` 只读写 Formily Schema。
- 前端调用页只使用 `SchemaRenderer` 渲染 Formily Schema。
- UI 编辑器只编辑 Formily Schema。
- Page 生成和 Entity Page 生成必须基于明确的 `entity + actions`，不能把全部函数套进 CRUD 模板。
