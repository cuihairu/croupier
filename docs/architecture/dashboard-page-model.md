# Dashboard Page 模型

## 状态

Current（2026-07-18）

## 核心定义

Dashboard Page 是用户完成一个业务任务的界面编排，不是 OpenAPI operation，也不是单个函数表单。

Page 可以组合：

- 一个或多个函数。
- 查询区。
- 分页状态。
- 表格。
- 详情区。
- 弹窗表单。
- 行操作。
- 批量操作。
- 异步任务状态。
- 图表和结果视图。

## 与其他模型的边界

| 模型 | 职责 | 不负责 |
|------|------|--------|
| OpenAPI | 描述函数调用契约 | 页面布局 |
| Function | 表示一个可执行能力 | 多函数页面编排 |
| Entity | 表示稳定业务对象 | 全局命令、报表、任务 |
| Function UI | 单函数输入表单，格式为 Formily Schema | 分页、表格、详情、路由 |
| Page | 组合多个函数和 UI 区块 | 定义底层调用协议 |

## Page 类型

### Entity Page

适用于围绕单个业务对象生命周期展开的管理页面。

准入条件：

- 函数集合有明确相同的 `entity`。
- 存在 list/search/query 函数作为数据源，或存在可直接定位对象的 read/detail 函数。
- 输出结构可映射为表格或详情。
- 变更动作能自然挂载到行操作、详情操作或批量操作。

示例：

```text
player.manage
    queryForm -> player.list.input
    table -> player.list.output.items
    pagination -> player.list.input.page/pageSize + output.total
    rowDetail -> player.get
    rowActions -> player.update / player.ban
    batchActions -> reward.batchGrant
```

### Operation Page

适用于不围绕对象生命周期展开的动作。

示例：

```text
broadcast.send
cache.refresh
maintenance.rollback
```

这些函数可以有 Formily 输入表单，但不应该强行进入对象管理页。

### Task Page

适用于异步、批处理或长耗时任务。

示例：

```text
reward.batchGrant
report.generate
index.rebuild
```

典型结构：

```text
参数表单 -> startTask
任务状态 -> task.detail
事件流 -> task.events
结果区 -> task.result
```

### Report Page

适用于分析查询和图表。

示例：

```text
analytics.retention
analytics.payments
```

典型结构：

```text
筛选表单 -> query function
图表 -> response.series
表格 -> response.items
导出 -> export function
```

## 分页查询

分页是 Page 的职责，不是 Function UI 的职责。

Function UI 只渲染单次函数输入表单。Page 负责维护分页状态，并把分页字段映射到查询函数参数。

推荐 Page Schema 表达：

```json
{
  "type": "entity",
  "entity": "player",
  "queryAction": "player.list",
  "pagination": {
    "pageField": "page",
    "pageSizeField": "pageSize",
    "totalField": "total",
    "itemsField": "items"
  }
}
```

如果函数的分页字段不同，应在 Page Schema 中显式声明映射，不由前端运行时猜测。

## 对象管理页准入规则

不能假设所有函数都是 CRUD。

可以进入对象管理页的函数：

- `player.list`
- `player.get`
- `player.update`
- `player.ban`
- `order.list`
- `order.refund`

不应该进入对象管理页的函数：

- `broadcast.send`
- `cache.refresh`
- `analytics.retention`
- `report.generate`
- `maintenance.rollback`
- `approval.approve`

这些函数仍可调用，但应被编排到 Operation Page、Task Page、Report Page 或 Approval Page。

## 自动生成策略

自动生成只能生成初稿，不能引入第二套运行时协议。

允许：

```text
FunctionDescriptor -> Formily Function UI 初稿
Entity + actions -> Entity Page 初稿
Function group + intent -> Operation/Task/Report Page 初稿
```

不允许：

```text
页面运行时根据 JSON Schema 临时猜组件
把所有函数默认塞入对象管理页
把 Function UI 当 Page Schema 使用
```

## 验收规则

- Function Invoke Page 只渲染 Formily Schema。
- Entity Page 必须声明 `entity` 和绑定函数集合。
- Page 分页字段必须显式映射。
- Operation/Task/Report Page 与 Entity Page 分开建模。
- OpenAPI 只作为契约输入和生成依据，不直接作为 Page Schema。
