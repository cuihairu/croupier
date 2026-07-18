---
title: 函数注册与默认界面
order: 4
category:
  - 核心概念
tag:
  - 函数注册
  - 默认界面
  - JSON Schema
---

# 函数注册与默认界面

Croupier 的函数入口采用 metadata-driven UI：业务服务只注册函数和元信息，平台先生成一个可用的默认界面；当默认界面不满足运营场景时，再保存 UI 覆盖配置。

## 目标

- 函数注册后立即能在 Dashboard 中被发现、理解和调用。
- SDK 只负责声明能力，不绑定具体前端实现。
- 默认界面可被覆盖，但覆盖层不能破坏函数注册的真实 schema 来源。
- 函数升级后，未覆盖的页面自动继承最新注册信息。

## 注册元信息

函数注册的最小字段只有 `id` 和 `version`。为了生成更好的目录、权限、表单和工作台，应尽量补齐推荐字段。

| 字段 | 级别 | 用途 |
| --- | --- | --- |
| `id` | 必填 | 全局函数 ID，建议使用 `<domain>.<entity>.<operation>`，如 `game.player.get` |
| `version` | 必填 | 函数版本，用于实例选择和变更排查 |
| `summary` | 推荐 | 函数的一句话简介，用于目录标题、页面副标题和搜索 |
| `description` | 推荐 | Markdown 详细说明，用于函数信息抽屉和帮助文案 |
| `tags` | 推荐 | 目录聚合和搜索过滤 |
| `category` | 推荐 | 顶层分类，如 `game`、`player`、`order`、`system` |
| `entity` | 推荐 | 业务实体，如 `player`、`order`、`mail` |
| `operation` | 推荐 | 操作类型，如 `list`、`read`、`create`、`update`、`delete`、`custom` |
| `risk` | 推荐 | 治理等级，如 `safe`、`warning`、`danger` |
| `input_schema` | 推荐 | JSON Schema 请求体，用于自动生成参数表单和校验 |
| `output_schema` | 可选 | JSON Schema 响应体，用于结果视图和实体工作台 |
| `operation_id` | 可选 | OpenAPI operationId，默认可使用 `id` |
| `deprecated` | 可选 | 标记函数是否已废弃 |

缺失字段可以由平台推断，但推断结果只适合兜底，不应作为长期元数据质量标准。

## SDK 注册契约

多语言 SDK 注册函数时应把同一组元信息写入 provider connect 和 manifest。Dashboard 不直接理解某一种 SDK 的内部结构，只消费平台统一后的 descriptor。

推荐的注册内容：

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
    risk: 'high',
    input_schema: {
      type: 'object',
      properties: {
        player_id: { type: 'string', title: '玩家 ID' },
        reason: { type: 'string', title: '封禁原因' }
      },
      required: ['player_id', 'reason']
    },
    output_schema: {
      type: 'object',
      properties: {
        success: { type: 'boolean', title: '是否成功' }
      }
    }
  },
  async (_ctx, payload) => JSON.stringify({ success: true })
);
```

字段流转规则：

- `summary` 用于目录标题、函数信息和默认 UI 标题。
- `description` 用于帮助说明、OpenAPI description 和函数信息抽屉。
- `input_schema` 是参数表单的事实来源；自定义 UI 只覆盖布局和组件表现。
- `output_schema` 后续用于结果视图和对象工作台，不参与调用参数校验。
- `category/entity/operation/risk/tags` 用于菜单归类、权限治理、默认字段推断和搜索。

## 默认 UI 解析顺序

Dashboard 渲染函数页面时按以下顺序选择 UI：

1. 数据库中的函数 `metadata.ui`，这是用户保存的自定义覆盖。
2. `configs/ui/functions.override/<function-id>.yaml|json`，用于部署级覆盖。
3. `configs/ui/functions/<function-id>.yaml|json`，用于随仓库发布的默认覆盖。
4. OpenAPI operation 的 `x-ui`。
5. `input_schema` 或 OpenAPI request schema 自动生成表单。
6. 根据 `id/entity/operation` 推断一个弱默认表单，并允许切换到 raw JSON 调用。

原则：默认 UI 永远可替换；覆盖层只表达差异，不复制整份注册元信息。

## 默认生成规则

当存在结构化 `input_schema` 时，平台按 JSON Schema 生成表单：

- `string` -> 输入框；有 `enum` 时使用选择器。
- `number` / `integer` -> 数字输入。
- `boolean` -> 开关。
- `array` -> 数组或多选输入。
- `object` -> JSON 编辑区域。
- `required` -> 必填校验。
- `title` / `description` -> 字段标签和说明。

当没有可用 schema 时，平台按函数 ID 和操作类型推断：

- `create/add/new`：生成 `{ data: object }`。
- `read/get/query/search/detail`：生成 `{ <entity>_id: string }`。
- `update/edit/patch`：生成 `{ <entity>_id: string, patch: object }`。
- `delete/remove`：生成 `{ <entity>_id: string }`。
- 其他操作：生成 `{ payload: object }`。

弱默认表单会展示提示，提醒维护者补齐 `summary`、`description` 和 `input_schema`，但不会阻断调用。

## 开源参考

这个模式不是 Croupier 特有设计，而是复用了常见的 schema-driven UI 思路：

- [`react-jsonschema-form`](https://rjsf-team.github.io/react-jsonschema-form/docs/) 根据 JSON Schema 生成 React 表单，适合做函数参数表单的基础渲染模型。
- [`JSON Forms`](https://jsonforms.io/docs/) 明确区分 data schema 和 UI schema，符合“注册 schema 是事实来源，UI 覆盖只表达展示差异”的原则。
- [`Swagger UI`](https://swagger.io/open-source/swagger-ui/) 基于 OpenAPI operation 生成接口文档和 `Try it out` 调用入口，说明 OpenAPI 元信息可以直接驱动可交互界面。
- [`Backstage Scaffolder`](https://backstage.io/docs/features/software-templates/writing-templates/) 用模板参数 schema 生成执行表单，适合参考“声明能力后生成默认操作页”的产品体验。

因此 Croupier 的方向应是：先让函数注册元信息足够完整，平台自动生成默认调用页和对象工作台；只有运营体验不满意时，才保存 UI 覆盖配置。

## 实施边界

当前阶段只做“声明式函数注册 -> 默认调用界面”的闭环，不引入复杂页面编排引擎：

- SDK demo 必须注册 `summary`、`description`、`input_schema` 和 `output_schema`。
- 后端 descriptor 必须透出 `displayName`、`summary`、`inputSchema`、`outputSchema` 和治理字段。
- 前端调用页必须允许弱默认表单和 raw JSON 调用，不能因为缺 schema 阻断执行。
- 自定义 UI 仅作为覆盖层保存，不能反向改写函数注册 schema。

## 示例

```json
{
  "id": "game.player.ban",
  "version": "1.0.0",
  "summary": "封禁玩家",
  "description": "封禁指定玩家账号，可设置封禁时长和原因。",
  "tags": ["game", "player", "moderation"],
  "category": "game",
  "entity": "player",
  "operation": "custom",
  "risk": "danger",
  "input_schema": {
    "type": "object",
    "properties": {
      "player_id": {
        "type": "string",
        "title": "玩家 ID",
        "description": "要封禁的玩家 ID"
      },
      "duration_seconds": {
        "type": "integer",
        "title": "封禁时长（秒）",
        "minimum": 60
      },
      "reason": {
        "type": "string",
        "title": "封禁原因"
      }
    },
    "required": ["player_id", "reason"]
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "ban_id": { "type": "string", "title": "封禁记录 ID" },
      "success": { "type": "boolean", "title": "是否成功" }
    }
  }
}
```

## 渐进改造路线

1. 已注册函数即使缺少 schema，也先使用弱默认表单或 raw JSON 调用。
2. SDK demo 和示例逐步补齐 `summary`、`description`、`input_schema`。
3. Dashboard 增加元信息质量提示，例如“缺简介”“缺入参 schema”“高风险但未声明 risk”。
4. 按 `entity + operation` 自动合成实体工作台，例如玩家列表、玩家详情、订单管理。
5. 自定义页面只保存 layout/widget 覆盖，默认继承函数注册 schema。
