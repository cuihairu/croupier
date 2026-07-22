---
title: Dashboard Resource/Page 模型
icon: layout-dashboard
order: 5
category:
  - 系统架构
tag:
  - Dashboard
  - Formily
  - 函数注册
  - 动态菜单
---

# Dashboard Resource/Page 模型

> **状态**：Current — Dashboard 动态页面、函数注册默认界面、运行控制台菜单的权威设计。

本文档定义 Croupier Dashboard 的目标模型。后续实现、重构和评审以本文档为准。

## 设计目标

Croupier 的 Dashboard 应接近 API Platform Admin 的思路：

```text
API/函数契约内省
  -> 资源与操作归一化
  -> 自动生成默认页面
  -> 用户按需覆盖页面
  -> 发布到运行控制台
```

但 Croupier 不能直接照搬 CRUD Admin。游戏运营后台存在大量命令、任务、报表和高风险操作，例如 `player.ban`、`mail.send`、`reward.batchGrant`、`cache.refresh`。这些函数不能被强行塞进 Entity Page。

目标是：

- 函数注册后能自动生成一个可运行的默认界面。
- 默认界面不满意时，用户编辑 Page Schema，而不是改前端代码。
- 动态菜单来自已发布 Page，不来自静态路由和静态国际化文件。
- 函数输入表单和页面编排都统一使用 Formily JSON Schema。
- 缺失关键语义时显式报错或进入待编排状态，不静默猜测成稳定页面。

## 核心原则

1. **函数不是页面**：Function 只是最小可执行能力；Page 才是运营人员完成任务的界面。
2. **资源先于菜单**：菜单按已发布 Resource/Page 组织，不按原始函数列表堆叠。
3. **Formily 是唯一 UI 协议**：函数表单和页面编排都必须是 Formily JSON Schema。
4. **动态文案跟随配置**：动态分类、资源、页面标题的多语言文案来自注册或 Page metadata，不进入 `web/src/locales/*/menu.ts`。
5. **不把字典当主模型**：字典适合枚举、状态、下拉选项，不适合作为动态导航分类的事实源。
6. **不在前端运行时猜语义**：函数归类、操作类型、页面候选和默认 Page 生成由 Server 归一化完成。
7. **发布产物必须自洽**：运行控制台只消费已发布 PageSpec；缺少标题、分类、多语言、Formily Schema 或绑定函数时发布失败。

## 模型总览

```text
SDK / OpenAPI / DB Template
  -> Raw Function Descriptor
  -> FunctionSpec
  -> ResourceSpec + OperationSpec
  -> Generated PageSpec
  -> Workspace Draft
  -> Published PageSpec
  -> ConsoleMenuSpec
```

| 模型 | 职责 | 不负责 |
| --- | --- | --- |
| `FunctionSpec` | 单个函数能力、输入/输出契约、默认函数表单 | 页面布局、菜单位置 |
| `ResourceSpec` | 稳定业务对象或能力域，如 `player`、`mail`、`inventory` | 具体调用执行 |
| `OperationSpec` | 某个资源上的操作语义和放置位置 | 全局菜单结构 |
| `PageSpec` | 页面级 Formily 组件树，组合函数、表格、详情、动作和分页 | 底层协议和函数注册 |
| `WorkspaceDraft` | 用户正在编辑的 PageSpec 草稿集合 | 运行控制台展示 |
| `PublishedPageSpec` | 已校验、已发布、可运行的页面产物 | 继续猜测和补全 |
| `ConsoleMenuSpec` | 从已发布页面生成的左侧运行控制台菜单 | 保存业务配置 |

## 概念边界

### Function 与 Function UI

Function 是平台最小可执行能力，例如 `player.list`、`player.ban`、`mail.send`、`reward.batchGrant`。

Function UI 只解决一个问题：这个函数需要哪些输入字段。它必须是 Formily JSON Schema，用于单函数调用、弹窗表单或 PageSpec 中的局部表单。

Function UI 不负责：

- 页面路由。
- 左侧菜单。
- 分页状态。
- 表格列。
- 详情布局。
- 多函数组合。

### Resource

Resource 是 Dashboard 页面组织用的稳定业务资源或能力域，不等同于数据库表，也不等同于传统后端 Entity。

示例：

| Resource | 含义 | 典型函数 |
| --- | --- | --- |
| `player` | 玩家运营资源 | `player.list`、`player.get`、`player.ban` |
| `mail` | 邮件运营资源 | `mail.template.list`、`mail.send` |
| `inventory` | 背包资源 | `inventory.query`、`item.grant` |
| `analytics` | 分析能力域 | `analytics.retention` |

Resource 的作用是把相关 Operation 组织成页面候选能力。它不是运行控制台菜单本身。

### Operation

Operation 是某个 Function 在 Resource 或页面中的业务动作和页面语义。

它必须拆成三个字段理解：

| 字段 | 含义 | 示例 |
| --- | --- | --- |
| `operation` | 业务动作 key | `list`、`get`、`ban`、`grant`、`send` |
| `operationKind` | 页面生成语义 | `list`、`get`、`action`、`task`、`report` |
| `placement` | 页面放置位置 | `tableData`、`rowAction`、`toolbarAction`、`standalone` |

不能把 `operation` 当成页面类型。页面类型只由 `operationKind` 和 PageSpec 决定。

### Page

Page 是运营人员实际使用的业务页面。它可以组合一个或多个 Function。

示例：

```text
player.manage
  query       -> player.list
  table       -> player.list.response.items
  detail      -> player.get
  rowAction   -> player.ban / player.update
  batchAction -> reward.batchGrant
```

Page 负责页面级能力：

- 查询条件。
- 分页状态。
- 表格数据映射。
- 详情面板。
- 弹窗表单。
- 行操作、批量操作、工具栏操作。
- 任务状态和报表图表。

因此 Page 不是单个 Function，也不是所有 Function 的平铺列表。

### Page 工作台

Page 工作台是 PageSpec 草稿管理器。它负责生成建议、编辑、预览、校验、发布和回滚。

Page 工作台不负责：

- 从函数名猜语义。
- 维护动态菜单字典。
- 把未发布页面展示到运行控制台。
- 生成第二套非 Formily UI 协议。

### 运行控制台

运行控制台是执行层，只展示已发布 PageSpec。

它不读取函数目录来生成左侧菜单，不读取草稿，不在前端推断分类。左侧菜单唯一来源是：

```text
PublishedPageSpec[] -> ConsoleMenuSpec
```

## 判断规则

读代码或设计新功能时，按下面顺序判断，不跳步：

1. 先判断是不是一个可执行能力：是则进入 `FunctionSpec`。
2. 再判断它是否围绕稳定资源展开：是则关联 `ResourceSpec`，否则只保留为独立 Operation 候选。
3. 再判断页面语义：用 `operationKind` 决定是列表、详情、动作、任务还是报表。
4. 再判断页面位置：用 `placement` 决定放查询区、表格数据、行操作、工具栏、批量操作或独立页。
5. 再生成 PageSpec 建议：建议不是发布结果，必须进入 Page 工作台确认。
6. 最后发布：只有 PublishedPageSpec 才能进入运行控制台菜单。

典型判断：

| 函数 | Resource | Operation | Page 类型 | Placement | 说明 |
| --- | --- | --- | --- | --- | --- |
| `player.list` | `player` | `list` / `list` | Entity Page | `tableData` | 玩家列表页的数据源 |
| `player.get` | `player` | `get` / `get` | Entity Page | `detailData` | 玩家详情的数据源 |
| `player.ban` | `player` | `ban` / `action` | Entity Page | `rowAction` | 需要选中具体玩家，适合作为行操作 |
| `mail.send` | `mail` 或无 Resource | `send` / `action` | Operation Page 或 Entity Page | `standalone` / `toolbarAction` | 如果是全局发邮件，独立页；如果在邮件模板页内触发，可作为工具栏动作 |
| `reward.batchGrant` | `reward` | `batchGrant` / `task` | Task Page | `standalone` / `batchAction` | 长耗时或批量操作需要任务页状态 |
| `analytics.retention` | `analytics` | `retention` / `report` | Report Page | `standalone` | 查询结果更适合图表和报表 |
| `cache.refresh` | 无 Resource 或 `system` | `refresh` / `action` | Operation Page | `standalone` | 运维命令不属于 Entity Page |

关键约束：

- 函数注册成功只代表函数目录可见，不代表运行控制台可见。
- 缺少 `operationKind` 或 `placement` 时，只能进入待编排建议，不能自动发布 Page。
- 缺少动态 labels 时，可以预览草稿，但不能发布到运行控制台。
- `mail.send`、`cache.refresh` 这类函数不能因为有 `mail` 或 `cache` 前缀就强行进入 Entity Page。

## 术语定义

### FunctionSpec

`FunctionSpec` 是函数注册归一化后的内部模型。

必填字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 全局唯一函数 ID，例如 `player.ban` |
| `version` | 函数版本 |
| `inputFormilySchema` | 函数输入表单，必须是 Formily JSON Schema |

推荐字段：

| 字段 | 说明 |
| --- | --- |
| `displayName` | 函数显示名，多语言文本 |
| `summary` | 一句话简介，多语言文本 |
| `description` | 详细说明，支持 Markdown |
| `category` | 导航分类 key |
| `categoryDisplay` | 分类多语言显示名 |
| `entity` | 资源 key，例如 `player` |
| `entityDisplay` | 资源多语言显示名 |
| `operation` | 业务操作 key，例如 `ban` |
| `operationKind` | 操作类型，例如 `list`、`get`、`action` |
| `placement` | 推荐页面放置位置 |
| `risk` | 风险等级 |
| `tags` | 搜索和治理标签 |
| `outputSchema` | 响应 JSON Schema |

### ResourceSpec

`ResourceSpec` 表示 Dashboard 可组织页面的稳定对象或能力域。

示例：

```json
{
  "key": "player",
  "labels": {
    "zh-CN": "玩家",
    "en-US": "Players"
  },
  "category": {
    "key": "gameplay",
    "labels": {
      "zh-CN": "玩法运营",
      "en-US": "Gameplay Ops"
    },
    "order": 20
  }
}
```

`ResourceSpec` 可以来自显式 `entity`，也可以由函数 ID 的对象段确定。进入发布态前必须完成归一化，不允许运行控制台临时推断。

### OperationSpec

`OperationSpec` 描述函数在页面里的语义。

```json
{
  "functionId": "player.ban",
  "resourceKey": "player",
  "operation": "ban",
  "kind": "action",
  "placement": "rowAction",
  "labels": {
    "zh-CN": "封禁",
    "en-US": "Ban"
  },
  "risk": "danger"
}
```

`kind` 必须来自明确 metadata。允许的值：

| `kind` | 含义 | 典型页面 |
| --- | --- | --- |
| `list` | 列表查询 | Entity Page |
| `get` | 单对象读取 | Entity Page |
| `create` | 新建对象 | Entity Page |
| `update` | 更新对象 | Entity Page |
| `delete` | 删除对象 | Entity Page |
| `action` | 同步命令 | Entity Page / Operation Page |
| `task` | 异步任务 | Task Page |
| `report` | 报表查询 | Report Page |

`placement` 定义操作在页面中的推荐位置：

| `placement` | 含义 |
| --- | --- |
| `query` | 查询区 |
| `tableData` | 表格数据源 |
| `rowAction` | 表格行操作 |
| `detailAction` | 详情页操作 |
| `toolbarAction` | 页面工具栏操作 |
| `standalone` | 独立操作页 |

缺少 `kind` 或 `placement` 的函数可以出现在函数目录，但不能自动发布为正式 Page。

### PageSpec

`PageSpec` 是 Dashboard 的页面编排产物。它不是 OpenAPI operation，也不是单函数表单。

```json
{
  "key": "player.manage",
  "type": "entity",
  "resourceKey": "player",
  "title": {
    "zh-CN": "玩家管理",
    "en-US": "Player Management"
  },
  "category": {
    "key": "gameplay",
    "labels": {
      "zh-CN": "玩法运营",
      "en-US": "Gameplay Ops"
    },
    "order": 20
  },
  "schema": {
    "type": "void",
    "x-component": "ConsolePage",
    "x-component-props": {
      "pageKey": "player.manage"
    },
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
}
```

`schema` 必须是 Formily JSON Schema。页面级能力通过平台注册的 Formily 组件表达，例如：

| 组件 | 职责 |
| --- | --- |
| `ConsolePage` | 页面根节点 |
| `QueryForm` | 查询表单 |
| `DataTable` | 表格和分页 |
| `DetailPanel` | 详情展示 |
| `ActionButton` | 单个操作 |
| `ActionGroup` | 行操作、批量操作、工具栏操作 |
| `ResultPanel` | 函数调用结果 |
| `TaskTimeline` | 异步任务进度 |
| `ChartPanel` | 图表 |

不定义第二套 `layout` 运行时协议。PageSpec 的 `schema` 是唯一页面 UI 协议。

## 前端技术栈落地

Croupier 保留当前 Ant Design / Ant Design Pro / Umi / React / Formily 技术栈，不引入 API Platform Admin 或 React Admin 作为运行时框架。

API Platform Admin 提供的是设计模式参考：

```text
API 描述内省 -> 资源识别 -> 默认页面生成 -> 用户局部覆盖
```

不是框架替换目标。Croupier 需要自建 `ResourceSpec / OperationSpec / PageSpec` 归一化和生成器，因为游戏后台存在大量非 CRUD 操作、任务、报表和高风险命令。

当前技术栈职责如下：

| 技术 | 职责 | 不负责 |
| --- | --- | --- |
| Umi | 固定参数路由、运行时 layout、权限接入、请求封装 | 生成业务 PageSpec |
| Ant Design ProLayout | 全局布局和左侧菜单容器 | 决定动态菜单分类和多语言 |
| Ant Design ProComponents | 表格、详情、卡片、统计、搜索等后台组件实现 | 归一化函数语义 |
| Ant Design | Modal、Drawer、Button、Tag、Alert 等基础交互组件 | 页面协议 |
| Formily | 函数表单和 PageSpec 的统一 schema 渲染引擎 | 业务函数执行路由 |
| React | 组件运行时和状态组织 | 后端元数据归一化 |

推荐前端运行结构：

```text
Umi route: /console/:categoryKey/:pageKey
  -> RuntimeConsolePage
  -> fetch PublishedPageSpec
  -> FormilyPageRenderer
  -> Page component registry
  -> Antd / ProComponents 实现具体组件
```

页面级组件注册示例：

| Formily `x-component` | 推荐实现 |
| --- | --- |
| `ConsolePage` | React 容器组件 |
| `QueryForm` | Formily form + Antd inputs |
| `DataTable` | ProTable |
| `DetailPanel` | ProDescriptions 或 Antd Descriptions |
| `ActionButton` | Antd Button + Modal/Drawer/Popconfirm |
| `ActionGroup` | Antd Space / Dropdown |
| `TaskTimeline` | Antd Timeline / Steps |
| `ChartPanel` | 项目内图表组件 |

动态菜单落地方式：

```text
GET /api/v1/console/menu
  -> ConsoleMenuSpec
  -> ProLayout menu.request / menuDataRender
  -> locale: false
  -> name 使用当前语言 labels
```

禁止前端做以下事情：

- 在 `web/src/locales/*/menu.ts` 里维护动态分类。
- 在 React 页面里根据函数 ID 后缀猜页面类型。
- 在 ProLayout 菜单生成阶段补业务语义。
- 在运行时把 JSON Schema 临时转换成非 Formily UI。

允许前端做以下事情：

- 渲染 Server 已发布的 ConsoleMenuSpec。
- 渲染 Server 已发布的 Formily PageSpec。
- 在 Page 工作台中编辑 PageSpec。
- 在编辑期展示 Server 返回的 PageSpec 建议和诊断。

## Page 类型

### Entity Page

适用于围绕稳定业务对象生命周期展开的页面。

准入条件：

- 所有核心操作有明确相同 `resourceKey`。
- 至少存在 `list` 或 `get` 操作。
- 列表或详情响应结构可映射到表格/详情组件。
- 变更操作能自然放到行操作、详情操作、批量操作或工具栏。

示例：

```text
player.manage
  query -> player.list
  table -> player.list.response.items
  detail -> player.get
  rowAction -> player.ban / player.update
```

### Operation Page

适用于不围绕对象生命周期展开的同步操作。

示例：

```text
broadcast.send
cache.refresh
maintenance.rollback
```

这些函数可以有 Formily 输入表单，但不应强行进入 Entity Page。

### Task Page

适用于异步、批量或长耗时任务。

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

分页映射必须显式写入 PageSpec：

```json
{
  "x-component": "DataTable",
  "x-component-props": {
    "queryFunctionId": "player.list",
    "pagination": {
      "pageField": "page",
      "pageSizeField": "pageSize",
      "totalField": "$.response.total",
      "itemsField": "$.response.items"
    }
  }
}
```

如果函数分页字段不同，必须在 PageSpec 中显式声明映射，不由前端运行时猜测。

## 动态分类与菜单

分类 key 的确定规则只有一套：

1. 显式 `category.key` 优先。
2. 没有显式分类时，使用 `resourceKey` 的第一个 `.` 前段。
3. 没有 `resourceKey` 时，使用 `pageKey` 的第一个 `.` 前段。
4. 没有 `.` 时，整个 `resourceKey` 或 `pageKey` 就是分类。

示例：

| 输入 | 最终分类 |
| --- | --- |
| `category.key = support`, `resourceKey = player` | `support` |
| `resourceKey = player.ban` | `player` |
| `resourceKey = mail.send` | `mail` |
| `resourceKey = mail` | `mail` |
| `pageKey = analytics.retention` | `analytics` |

分类确定时机：

- FunctionSpec 归一化时可以从 descriptor 计算候选分类。
- ResourceSpec 生成时可以继承或覆盖候选分类。
- PageSpec 生成或保存时必须写入最终分类。
- 发布时校验分类 key 和 labels。
- 运行控制台只读取 PublishedPageSpec，不再重新推断分类。

动态菜单生成规则：

```text
PublishedPageSpec[]
  -> 按 category.key 分组
  -> 按 category.order / page.order 排序
  -> 生成 ConsoleMenuSpec
```

菜单路径：

```text
/console/home
/console/:categoryKey
/console/:categoryKey/:pageKey
```

动态菜单项必须设置 `locale: false`，显示名直接来自 PublishedPageSpec 的动态多语言 labels。禁止为动态分类生成 `menu.ControlConsole.category.*` 静态 key。

## 多语言模型

动态文案使用 `LocalizedText`：

```json
{
  "zh-CN": "玩家管理",
  "en-US": "Player Management"
}
```

适用范围：

- 分类标题。
- 资源标题。
- 页面标题。
- 操作标题。
- 函数显示名。
- 函数简介和说明。
- 字段标题和帮助文案。

发布校验要求：

- 必须提供系统默认语言。
- 必须覆盖 Dashboard 启用的语言集合。
- 缺失语言时发布失败。

运行时不从静态 locale 文件补动态分类和页面标题。静态 locale 只用于固定系统菜单，例如“系统配置”“运行控制台”“权限管理”。

## 默认生成策略

Server 可以生成默认 PageSpec，但生成必须基于明确元数据。

允许：

```text
FunctionSpec -> Function UI Formily Schema
ResourceSpec + OperationSpec -> Entity PageSpec 初稿
OperationSpec(kind=action, placement=standalone) -> Operation PageSpec 初稿
OperationSpec(kind=task) -> Task PageSpec 初稿
OperationSpec(kind=report) -> Report PageSpec 初稿
```

不允许：

```text
前端按函数 ID 后缀临时猜页面类型
把所有函数默认塞入 Entity Page
把 Function UI 当 Page Schema
缺少 kind/placement 仍发布正式 Page
为动态分类改静态 i18n 文件
```

对于缺失 `operationKind`、`placement`、`entity`、`outputSchema` 等关键字段的函数，Server 可以生成“待编排建议”，但建议不是发布产物。用户确认并补齐后才生成 PageSpec。

## Workspace 职责

Workspace 是 PageSpec 的草稿和发布管理容器，不是函数语义归一化器。

Workspace 负责：

- 保存用户编辑后的 PageSpec。
- 预览 PageSpec。
- 校验 PageSpec。
- 发布 PublishedPageSpec。
- 回滚历史版本。
- 管理页面权限和发布记录。

Workspace 不负责：

- 从函数 ID 猜 operation kind。
- 从 JSON Schema 临时猜 Formily 组件。
- 维护动态分类字典。
- 渲染未发布页面到运行控制台。

## 数据存储建议

目标存储应体现模型边界：

| 存储 | 内容 |
| --- | --- |
| `function_specs` | 归一化函数能力和治理字段 |
| `function_ui_schemas` | 单函数 Formily 表单 |
| `resource_specs` | 资源、分类、资源多语言、排序 |
| `page_specs` | 草稿 PageSpec |
| `published_page_specs` | 已发布 PageSpec 快照 |
| `config_versions` | PageSpec / Workspace 发布历史 |

活跃设计不定义 `layout` 枚举、实体 CRUD 配置或动态分类字典。已有数据需要通过一次性迁移转换为 PageSpec，迁移失败的记录不能发布。

## API 边界

建议服务端提供以下能力：

| API | 职责 |
| --- | --- |
| `GET /api/v1/functions/descriptors` | 查看函数原始能力目录 |
| `GET /api/v1/functions/:id/ui` | 获取单函数 Formily 表单 |
| `PUT /api/v1/functions/:id/ui` | 保存单函数 Formily 表单 |
| `GET /api/v1/resources` | 获取归一化 ResourceSpec |
| `GET /api/v1/pages/generated` | 获取默认 PageSpec 建议 |
| `GET /api/v1/workspaces/pages` | 获取 PageSpec 草稿 |
| `PUT /api/v1/workspaces/pages/:pageKey` | 保存 PageSpec 草稿 |
| `POST /api/v1/workspaces/pages/:pageKey/publish` | 发布 PageSpec |
| `GET /api/v1/console/pages` | 获取已发布 PageSpec |
| `GET /api/v1/console/menu` | 获取 ConsoleMenuSpec |

Dashboard 不应直接从函数目录组装运行控制台菜单。

## 验收规则

- 新注册 `mail.send` 后，不改前端代码、不改静态 locale 文件，也能生成默认操作页或待编排建议。
- `player.ban` 没有显式分类时归入 `player`；显式 `support` 时归入 `support`。
- 动态分类、资源、页面标题来自 PageSpec metadata。
- 所有运行时 UI schema 都是 Formily JSON Schema。
- 非 Formily Schema 在保存或渲染阶段直接报错。
- 缺少 `operationKind` 或 `placement` 的函数不会自动进入 Entity Page。
- 运行控制台左侧菜单只展示已发布 PageSpec。
- 分页字段必须显式映射。
- 函数目录、Workspace 编辑器、运行控制台之间没有重复菜单生成逻辑。

## 迁移计划

### 阶段 1：模型冻结

- 本文档作为权威设计入口。
- 删除动态分类静态 locale 设计。
- 删除旧 layout 配置作为新协议的文档入口。

### 阶段 2：后端归一化

- 新增强类型 `FunctionSpec`、`ResourceSpec`、`OperationSpec`、`PageSpec`。
- `DescriptorsLogic` 输出保持目录用途，但 Page 生成改用归一化服务。
- HTTP 边界上可以用 `json.RawMessage` 承载 JSON，但归一化服务输出必须替换为强类型 DTO。

### 阶段 3：默认 Page 生成

- Server 根据 `ResourceSpec + OperationSpec` 生成 Formily PageSpec。
- 缺失关键语义时只生成待编排建议，不发布。
- 生成结果带来源、版本和质量诊断。

### 阶段 4：前端渲染收敛

- WorkspaceEditor 编辑 PageSpec。
- Runtime renderer 只渲染 Formily PageSpec。
- 移除前端基于函数 ID 后缀的页面类型猜测。
- 移除自定义 layout 枚举写入路径。

### 阶段 5：运行控制台菜单收敛

- 菜单改为读取 PublishedPageSpec 或 ConsoleMenuSpec。
- 动态菜单项全部 `locale: false`。
- 动态标题由当前语言读取 PageSpec labels。
- 缺失语言或分类 metadata 时发布失败。

### 阶段 6：数据迁移和清理

- 将既有页面配置转换为 PageSpec。
- 转换失败的记录标记为迁移错误，禁止发布。
- 删除动态分类静态 locale。
- 删除前端多套 schema 兜底逻辑。

## 与 API Platform Admin 的差异

API Platform Admin 主要围绕 Resource 自动生成 CRUD 管理页面。Croupier 借鉴它的“内省、默认生成、局部覆盖”模式，但模型必须覆盖游戏运营后台的非 CRUD 场景：

- 高风险命令。
- 批量任务。
- 跨资源操作。
- 报表页面。
- 审批流。
- 游戏环境和目标实例选择。

因此 Croupier 的最小稳定单位不是 CRUD Resource，而是：

```text
ResourceSpec + OperationSpec + PageSpec
```

这也是避免 Dashboard 混乱的核心边界。
