# Dashboard UI 架构边界

## 状态

Current（2026-07-18）

## 决策概要

Croupier Dashboard 不把 OpenAPI、函数表单、Entity Page 和页面编排混为一个模型。

当前统一采用以下边界：

1. **OpenAPI 是函数能力契约**：描述函数如何调用、输入输出结构、错误和文档信息。
2. **Function 是可执行能力**：函数可以是查询、命令、任务、审批动作或对象操作。
3. **Resource 是页面组织资源**：只用于确实围绕某个资源或能力域展开的页面候选。
4. **Function Form 是单函数输入表单**：唯一格式是 Formily Schema。
5. **Page 是业务页面编排**：组合查询区、分页表格、详情、弹窗表单、批量操作和结果视图。

运行时只消费一种 UI Schema：**Formily JSON Schema**。非 Formily Schema 必须报错，不能转换、猜测或静默降级。

这条规则同时适用于：

- 单函数输入表单。
- Dashboard Page 页面编排。

---

## 一、OpenAPI 的职责

OpenAPI 在 Croupier 中是成熟的函数契约来源，适合承载：

- `operationId` / 函数 ID
- `summary` / `description` / `tags`
- request schema
- response schema
- error response
- 安全和治理扩展

OpenAPI 不直接等于 Dashboard Page。它可以生成默认函数表单初稿，也可以帮助推断页面候选能力，但不能直接决定页面布局、菜单、动态显示名或按钮位置。

正确的数据流是：

```text
OpenAPI / SDK descriptor
    -> FunctionDescriptor
    -> input_schema / output_schema
    -> Formily Schema 初稿
    -> Function Form / PageCandidate
    -> Page Studio 确认 PageSpec
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
- `resource`
- `operation`
- `risk`
- `input_schema`
- `output_schema`

函数注册不得提供动态显示名、菜单分类、页面类型、页面放置、Formily 或 Page schema。缺少 `input_schema` 时，后端可以生成最小 Formily Schema 初稿，但该初稿仍必须是 Formily Schema，不能在前端运行时再生成第二套格式。

---

## 三、Resource 与 Entity Page 的职责

Resource 表示 Dashboard 页面组织用的稳定业务资源或能力域，例如：

- `player`
- `order`
- `item`
- `mail`
- `activity`

Entity Page 是 Page 的一种类型，只适合围绕同一 Resource 生命周期展开的页面。

只有满足以下条件的函数才应该进入 Entity Page：

- 有明确 `resource`
- 操作围绕该对象生命周期展开
- 存在稳定对象标识或列表查询
- 返回结构可映射到表格、详情或对象状态
- 操作可以自然挂载到查询区、行操作、详情页或批量操作

不应进入 Entity Page 的函数：

- 全局命令：`cache.refresh`
- 批处理任务：`reward.batchGrant`
- 分析报表：`analytics.retention`
- 平台运维：`maintenance.rollback`
- 无主对象的工具动作：`broadcast.send`

这些函数仍然有函数表单，但应该由 Operation Page、Task Page、Report Page 或 Tool Page 承载。

---

## 四、Function Form 的职责

Function Form 只描述**单个函数的输入表单**。

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

Function Form 的加载优先级：

```text
1. 管理员在函数目录保存的 Formily override
2. input_schema / OpenAPI request schema 生成的 Formily Schema
3. 无可用 schema 时生成仅包含 `payload` 对象字段的最小 Formily Schema
```

所有来源的输出都必须是 Formily Schema。SDK/OpenAPI 注册中的 `ui/x-ui/Formily/layout/components` 必须在注册或导入边界拒绝，不能作为 Function Form 来源。

Function Form 不负责：

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

典型 Entity Page：

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

PageSpec 必须使用 Formily JSON Schema 表达。页面级组件通过 `x-component` 表达，例如 `ConsolePage`、`QueryForm`、`DataTable`、`DetailPanel`、`ActionButton`、`TaskTimeline`、`ChartPanel`。

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

Page 不应把所有函数都强行套成 CRUD。CRUD 只是 Entity Page 的一种模板，不是函数系统的总模型。

完整 Page 模型见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)。

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
| Server form resolver | 解析或生成函数表单 | 函数记录 | Formily Schema |
| `/api/v1/functions/:id/form` | 读写函数表单 | Formily Schema | Formily Schema |
| `SchemaRenderer` | 渲染函数表单 | Formily Schema | React Form |
| Function Invoke Page | 调用单个函数 | Formily values | invoke/task |
| Server Page generator | 生成默认页面建议 | ResourceSpec + OperationSpec | Formily PageSpec |
| Page Studio | 编辑页面草稿 | Formily PageSpec | PageSpec Draft |
| Runtime Page renderer | 运行控制台渲染页面 | PublishedPageSpec | Dashboard Page |

## 八、前端技术栈边界

Croupier Dashboard 保留 Ant Design / Ant Design Pro / Umi / React / Formily 技术栈。

API Platform Admin 的价值是“内省、默认生成、局部覆盖”的模式，不是框架替换目标。Croupier 的资源归一化和 PageSpec 生成必须由 Server 自己实现。

| 技术 | 在 UI 生成中的职责 |
| --- | --- |
| Umi | 固定参数路由和运行时 layout |
| ProLayout | 承载已发布 ConsoleMenuSpec 生成的左侧菜单 |
| ProTable | 实现 Formily PageSpec 中的 `DataTable` |
| ProDescriptions / Antd Descriptions | 实现 `DetailPanel` |
| Antd Modal / Drawer / Popconfirm | 实现操作确认和弹窗表单 |
| Formily | 渲染单函数表单和页面级 PageSpec |

前端不负责从函数目录推断页面结构。运行控制台只消费：

```text
PublishedPageSpec + ConsoleMenuSpec
```

动态菜单项必须直接使用 PageSpec/ConsoleMenuSpec 中的多语言 labels，并设置 `locale: false`。

验收规则：

- 执行页只使用 `SchemaRenderer`。
- 保存接口只接受 Formily Schema。
- 编辑器只编辑 Formily Schema。
- OpenAPI 只参与契约归一化和初稿生成。
- Entity Page 只承载明确属于同一 Entity 的动作。
- Page 负责分页、表格、详情、弹窗和多函数组合。
- Page Schema 也是 Formily JSON Schema。
- 动态菜单只来自 PublishedPageSpec，不来自静态 i18n 文件。
- 不引入 React Admin / API Platform Admin 替换现有前端框架。
