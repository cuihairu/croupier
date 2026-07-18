# Dashboard UI 架构边界

## 状态

Current（2026-07-18）

## 决策概要

Croupier Dashboard 不把 OpenAPI、函数表单、对象管理页和页面编排混为一个模型。

当前统一采用以下边界：

1. **OpenAPI 是函数能力契约**：描述函数如何调用、输入输出结构、错误和文档信息。
2. **Function 是可执行能力**：函数可以是查询、命令、任务、审批动作或对象操作。
3. **Entity 是业务对象模型**：只用于确实围绕某个对象生命周期展开的管理界面。
4. **Function UI 是单函数输入表单**：唯一格式是 Formily Schema。
5. **Page 是业务页面编排**：组合查询区、分页表格、详情、弹窗表单、批量操作和结果视图。

运行时只消费一种 UI Schema：**Formily Schema**。非 Formily Schema 必须报错，不能转换、猜测或静默降级。

---

## 一、OpenAPI 的职责

OpenAPI 在 Croupier 中是成熟的函数契约来源，适合承载：

- `operationId` / 函数 ID
- `summary` / `description` / `tags`
- request schema
- response schema
- error response
- 安全和治理扩展

OpenAPI 不直接等于 Dashboard Page。它可以生成默认函数表单初稿，也可以帮助推断页面候选能力，但不能直接决定页面布局。

正确的数据流是：

```text
OpenAPI / SDK descriptor
    -> FunctionDescriptor
    -> input_schema / output_schema
    -> Formily Schema 初稿
    -> Function UI / Page 使用
```

错误的数据流是：

```text
OpenAPI
    -> Dashboard Page
    -> 页面运行时临时猜组件
```

---

## 二、Function 的职责

Function 表示一个可执行能力，不等同于 CRUD。

函数可以属于以下类型：

| 类型 | 示例 | 推荐 UI |
|------|------|---------|
| 对象查询 | `player.list`、`order.get` | Entity Page / Page 查询区 |
| 对象变更 | `player.update`、`item.grant` | Entity Page 行操作或弹窗表单 |
| 全局命令 | `broadcast.send`、`cache.refresh` | Operation Page / Tool Page |
| 异步任务 | `report.generate`、`reward.batchGrant` | Task Page / Wizard Page |
| 分析查询 | `analytics.retention` | Report Page / Chart Page |
| 审批动作 | `approval.approve`、`approval.reject` | Approval Page |

函数注册必须尽量提供：

- `id`
- `version`
- `summary`
- `description`
- `category`
- `entity`
- `operation`
- `risk`
- `input_schema`
- `output_schema`

缺少 `input_schema` 时，后端可以生成最小 Formily Schema 初稿，但该初稿仍必须是 Formily Schema，不能在前端运行时再生成第二套格式。

---

## 三、Entity 的职责

Entity 表示稳定业务对象，例如：

- `player`
- `order`
- `item`
- `mail`
- `activity`

只有满足以下条件的函数才应该进入对象管理页：

- 有明确 `entity`
- 操作围绕该对象生命周期展开
- 存在稳定对象标识或列表查询
- 返回结构可映射到表格、详情或对象状态
- 操作可以自然挂载到查询区、行操作、详情页或批量操作

不应进入对象管理页的函数：

- 全局命令：`cache.refresh`
- 批处理任务：`reward.batchGrant`
- 分析报表：`analytics.retention`
- 平台运维：`maintenance.rollback`
- 无主对象的工具动作：`broadcast.send`

这些函数仍然有函数表单，但应该由 Operation Page、Task Page、Report Page 或 Tool Page 承载。

---

## 四、Function UI 的职责

Function UI 只描述**单个函数的输入表单**。

唯一合法格式是 Formily Schema：

```json
{
  "type": "object",
  "properties": {
    "player_id": {
      "type": "string",
      "title": "玩家 ID",
      "x-component": "Input",
      "x-decorator": "FormItem",
      "x-component-props": {
        "placeholder": "请输入玩家 ID"
      }
    }
  },
  "required": ["player_id"]
}
```

Function UI 的加载优先级：

```text
1. functions.metadata.ui
2. configs/ui/functions.override/{function-id}.yaml|json
3. configs/ui/functions/{function-id}.yaml|json
4. OpenAPI operation["x-ui"]
5. input_schema / OpenAPI request schema 生成的 Formily Schema
6. function id / entity / operation 生成的最小 Formily Schema
```

所有来源的输出都必须是 Formily Schema。任一来源输出非 Formily Schema 时，应在保存或渲染阶段报错。

Function UI 不负责：

- 分页状态
- 表格列
- 页面路由
- 多函数组合
- 对象详情布局
- 审批流编排

这些属于 Page。

---

## 五、Page 的职责

Page 是用户完成业务任务的页面编排模型。Page 可以组合多个函数和多个 UI 区块。

典型对象管理 Page：

```text
player.manage
    查询区 -> player.list
    分页表格 -> player.list.response.items
    行详情 -> player.get
    行操作 -> player.ban / player.update
    批量操作 -> reward.batchGrant
```

典型报表 Page：

```text
analytics.retention
    筛选区 -> analytics.retention.query
    图表区 -> analytics.retention.response.series
    导出动作 -> analytics.retention.export
```

典型任务 Page：

```text
reward.batchGrant
    参数表单 -> reward.batchGrant
    提交后 -> task.detail / task.events
    结果区 -> task.result
```

Page 可以支持分页查询。分页属于 Page 的列表组件状态，通常绑定到某个 list/search/query 函数的参数：

```json
{
  "queryAction": "player.list",
  "pagination": {
    "pageField": "page",
    "pageSizeField": "pageSize",
    "totalField": "total",
    "itemsField": "items"
  }
}
```

Page 不应把所有函数都强行套成 CRUD。CRUD 只是对象管理 Page 的一种模板，不是函数系统的总模型。

---

## 六、Formily Schema 规范

### 顶层结构

```json
{
  "type": "object",
  "properties": {
    "fieldName": { "type": "string", "x-component": "Input" }
  },
  "required": ["fieldName"]
}
```

### 常用组件映射

| JSON Schema type | 条件 | Formily component |
|------------------|------|-------------------|
| `string` | 默认 | `Input` |
| `string` | `format: date` | `DatePicker` |
| `string` | `format: date-time` | `DatePicker` + `showTime` |
| `string` | `format: time` | `TimePicker` |
| `string` | `format: textarea` | `Input.TextArea` |
| `string` | `enum` | `Select` |
| `number` / `integer` | 默认 | `NumberPicker` |
| `boolean` | 默认 | `Switch` |
| `array` | `items.enum` | `Select` + `mode: multiple` |
| `array` | `items.object` | `ArrayTable` / `ArrayItems` |
| `object` | 嵌套对象 | `Card` + `properties` |

### 布局

布局使用 Formily 组件表达：

```json
{
  "type": "void",
  "x-component": "FormGrid",
  "x-component-props": {
    "minColumns": 2,
    "maxColumns": 3
  },
  "properties": {
    "player_id": {
      "type": "string",
      "title": "玩家 ID",
      "x-component": "Input",
      "x-decorator": "FormItem"
    }
  }
}
```

---

## 七、实现职责

| 模块 | 职责 | 输入 | 输出 |
|------|------|------|------|
| SDK / Provider | 注册函数能力 | 代码声明 / OpenAPI | FunctionDescriptor |
| Server descriptor | 归一化函数元信息 | FunctionDescriptor | API 契约 |
| Server UI resolver | 解析或生成函数表单 | 函数记录 | Formily Schema |
| `/api/v1/functions/:id/ui` | 读写函数表单 | Formily Schema | Formily Schema |
| `SchemaRenderer` | 渲染函数表单 | Formily Schema | React Form |
| Function Invoke Page | 调用单个函数 | Formily values | invoke/task |
| Entity Page | 对象管理 | Entity + Actions | 查询/表格/详情/动作 |
| Page Schema | 页面编排 | 多个函数和布局 | Dashboard Page |

验收规则：

- 执行页只使用 `SchemaRenderer`。
- 保存接口只接受 Formily Schema。
- 编辑器只编辑 Formily Schema。
- OpenAPI 只参与契约归一化和初稿生成。
- Entity Page 只承载明确属于同一 Entity 的动作。
- Page 负责分页、表格、详情、弹窗和多函数组合。
