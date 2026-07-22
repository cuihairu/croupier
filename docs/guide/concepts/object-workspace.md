---
title: Page 工作台
icon: dashboard
order: 4
category:
  - 核心概念
tag:
  - UI
  - 页面装配
  - 工作台
---

# Page 工作台

Page 工作台是 Croupier 将函数能力编排成运营页面的地方。它面向产品、运营和开发人员，负责 PageSpec 草稿编辑、预览、校验、发布和回滚。

Page 工作台不是“对象管理页编辑器”。Entity Page 只是 Page 类型之一，独立操作、任务和报表同样通过 PageSpec 管理。

## 在产品链路中的位置

| 层次 | 定位 | 数据来源 | 典型操作 |
| --- | --- | --- | --- |
| 函数目录 | 能力供给层 | FunctionSpec / 函数注册目录 | 查看函数是否注册成功、Schema 是否正确、是否有可调用实例、单函数 invoke |
| Page 工作台 | 页面装配层 | ResourceSpec / OperationSpec / PageSpec 草稿 | 生成默认页面、编辑 Formily Page Schema、预览、发布、回滚 |
| 运行控制台 | 执行层 | PublishedPageSpec / ConsoleMenuSpec | 面向运营人员执行业务操作 |

推荐工作流：

```text
函数注册
  -> Server 归一化 FunctionSpec / ResourceSpec / OperationSpec
  -> 生成 PageSpec 建议
  -> Page 工作台编辑和确认
  -> 发布 PublishedPageSpec
  -> 运行控制台动态菜单展示
```

## Page 工作台负责什么

- 管理 PageSpec 草稿。
- 基于 Server 生成的建议创建默认页面。
- 编辑页面级 Formily Schema。
- 绑定多个函数到一个业务页面。
- 配置查询区、分页、表格、详情、弹窗、批量操作、任务状态和图表。
- 校验 PageSpec 是否可发布。
- 发布和回滚页面版本。
- 管理页面级权限、排序、分类和多语言标题。

Page 工作台不负责：

- 从函数 ID 后缀猜测操作语义。
- 从 JSON Schema 在前端运行时生成第二套 UI。
- 把所有函数强行塞入 Entity Page。
- 维护运行控制台动态分类字典。
- 修改前端静态国际化文件。

## Page 类型

### Entity Page

适用于围绕稳定业务对象生命周期展开的页面。

示例：

```text
player.manage
  query -> player.list
  table -> player.list.response.items
  rowAction -> player.ban
  detail -> player.get
```

只有具备明确 `resourceKey/entity`，并且操作能自然放到查询、列表、详情或行操作中的函数，才进入 Entity Page。

### Operation Page

适用于独立同步操作，例如：

```text
broadcast.send
cache.refresh
maintenance.rollback
```

这些函数有 Formily 输入表单，但不属于 Entity Page。

### Task Page

适用于异步任务和批处理，例如：

```text
reward.batchGrant
report.generate
index.rebuild
```

### Report Page

适用于分析查询和图表，例如：

```text
analytics.retention
analytics.payments
```

## 自动生成与人工覆盖

Server 可以根据 `ResourceSpec + OperationSpec` 生成默认 PageSpec。

默认页面只是初稿：

- 元信息完整时，可以生成可预览草稿。
- 缺少 `operationKind`、`placement`、`outputSchema` 或动态 labels 时，只能生成待编排建议。
- 用户确认后，可以编辑 PageSpec 覆盖默认布局。
- 发布前必须通过校验。

这类似 API Platform Admin 的 guesser 模式：先根据 API 描述生成默认管理界面，不满意时覆盖局部。但 Croupier 的页面模型必须覆盖非 CRUD 操作、任务、报表和高风险动作。

## Formily Page Schema

Page 工作台编辑的页面也是 Formily JSON Schema。

示例结构：

```json
{
  "type": "void",
  "x-component": "ConsolePage",
  "properties": {
    "query": {
      "type": "void",
      "x-component": "QueryForm",
      "x-component-props": {
        "functionId": "player.list",
        "formSchemaRef": "function://player.list/ui"
      }
    },
    "table": {
      "type": "void",
      "x-component": "DataTable",
      "x-component-props": {
        "dataSource": "$.query.response.items",
        "pagination": {
          "pageField": "page",
          "pageSizeField": "pageSize",
          "totalField": "$.query.response.total"
        }
      }
    }
  }
}
```

活跃设计不定义 `layout` 枚举、实体 CRUD 配置或动态分类字典。Page 工作台保存和发布的唯一页面协议是 PageSpec。

## 与运行控制台的关系

运行控制台只展示已发布 PageSpec。

动态菜单生成链路：

```text
PublishedPageSpec[] -> ConsoleMenuSpec -> 左侧菜单
```

分类、页面标题、多语言显示名、排序和图标都来自 PublishedPageSpec metadata。

动态分类不写入 `web/src/locales/*/menu.ts`。静态 locale 只用于固定系统菜单，例如“运行控制台”“系统配置”“权限管理”。

## 相关文档

- [核心概念总览](./overview.md)
- [函数管理](./function-management.md)
- [函数注册与默认界面](./function-registration-ui.md)
- [Dashboard Resource/Page 模型](../../architecture/dashboard-page-model.md)
- [运行控制台动态菜单](../../design/console-dynamic-menu.md)
