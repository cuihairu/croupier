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

SDK descriptor 是函数注册输入，不是运行控制台菜单。

平台内部必须先归一化为强类型模型，再生成默认页面：

```text
OpenAPI Operation / SDK descriptor / DB template
  -> RawFunctionDescriptor
  -> FunctionSpec
  -> ResourceSpec + OperationSpec
  -> PageSpec 建议
  -> Workspace 草稿
  -> PublishedPageSpec
  -> ConsoleMenuSpec
```

前端不得直接根据 OpenAPI operation 或函数 ID 后缀生成正式页面。

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
operationKind  = 页面生成语义，例如 list / get / action / task / report
placement      = 页面放置位置，例如 tableData / rowAction / standalone
display labels = 动态多语言文案，随注册或 PageSpec 发布
```

## 三层边界

| 层 | 输入/输出 | 职责 |
| --- | --- | --- |
| Raw Descriptor | OpenAPI Operation / SDK descriptor | 承载函数契约和扩展字段 |
| FunctionSpec / ResourceSpec / OperationSpec | Server 归一化产物 | 清洗、校验、补全、诊断 |
| PageSpec | Server 生成或用户编辑 | 表达完整业务页面，必须是 Formily JSON Schema |

OpenAPI 和 SDK descriptor 可以存在兼容字段，但 PageSpec 生成只能依赖归一化后的强类型模型。

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
| `operationKind` | `x-operation-kind` | `operation_kind` | Page 生成必需 | 页面生成语义 |
| `placement` | `x-placement` | `placement` | Page 生成必需 | 页面放置位置 |
| `pageHint` | `x-page-hint` | `page_hint` | 可选 | 目标页面建议 |
| `ui` | `x-ui` | `ui` | 可选 | 单函数 Formily Schema |

`operationKind` 和 `placement` 是默认 PageSpec 生成的硬边界。缺少任一字段时，函数可以进入函数目录和单函数调用页，但不能自动发布为正式 Page。

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

`placement` 描述操作推荐放在页面哪里：

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

`placement` 不能由前端根据函数名推断。Server 可以在导入期给出建议，但发布正式 Page 前必须明确写入 PageSpec 或 OperationSpec。

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
7. 按 `entity` 和 `operationKind` 生成 ResourceSpec / OperationSpec。
8. 对缺失字段生成诊断。
9. 只有诊断通过的 OperationSpec 才能进入 PageSpec 默认生成。

允许 Server 给出建议，但建议必须带诊断来源：

| 诊断 | 处理 |
| --- | --- |
| 缺少 `entity` | 仅进入函数目录，不生成 Entity Page |
| 缺少 `operationKind` | 仅生成待编排建议 |
| 缺少 `placement` | 仅生成待编排建议 |
| 缺少动态 labels | 可以预览，不允许发布到运行控制台 |
| 非 Formily `x-ui` | 直接报错 |

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
- 禁止没有 `operationKind` 和 `placement` 就自动发布页面。
- 禁止动态分类写入静态 locale 文件。
- 禁止前端根据函数 ID 后缀推断正式 Page。
- 禁止 OpenAPI 直接作为 Dashboard Page Schema。
- 禁止非 Formily `x-ui` 被保存或渲染。

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
- Page 工作台负责确认和编辑。

### 阶段 4：SDK 升级

- 多语言 SDK 增加 v2 descriptor 字段。
- 旧 SDK 可继续注册函数，但不能自动生成正式 Page。
- SDK demo 必须展示 v2 字段。
