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
  -> 默认页面候选
  -> Page Studio 编辑草稿
  -> 受控发布快照
  -> 运行控制台菜单和受控执行
```

但 Croupier 不能直接照搬 CRUD Admin。游戏运营后台存在大量命令、任务、报表和高风险操作，例如 `player.ban`、`mail.send`、`reward.batchGrant`、`cache.refresh`。这些函数不能被强行塞进 Entity Page。

目标是：

- 函数注册后能自动生成可预览的函数表单和页面候选；注册者不需要编写任何界面。
- 默认界面不满意时，用户编辑 Page Schema，而不是改前端代码。
- 动态菜单来自已发布 Page，不来自静态路由和静态国际化文件。
- 函数输入表单和页面编排都统一使用 Formily JSON Schema。
- 缺失关键语义时显式报错或进入待编排状态，不静默猜测成稳定页面。
- 页面配置遵循全局 `game_id + env` 作用域；页面内不重复选择游戏和环境。
- 运行时只能通过已发布页面的绑定执行函数，不能让浏览器绕过页面版本、权限、审批和审计直接拼函数调用。

## 核心原则

1. **函数不是页面**：Function 只是最小可执行能力；Page 才是运营人员完成任务的界面。
2. **资源先于菜单**：菜单按已发布 Resource/Page 组织，不按原始函数列表堆叠。
3. **Formily 是唯一 UI 协议**：函数表单和页面编排都必须是 Formily JSON Schema。
4. **动态文案跟随 Page**：动态分类、资源、页面标题和按钮文案来自 PageSpec / PublishedPageSpec 的强类型 labels，不进入函数注册或 `web/src/locales/*/menu.ts`。
5. **不把字典当主模型**：字典适合枚举、状态、下拉选项，不适合作为动态导航分类的事实源。
6. **不在前端运行时猜语义**：函数归类、操作类型、页面候选和默认 Page 生成由 Server 归一化完成。
7. **发布产物必须自洽**：运行控制台只消费已发布 PageSpec；缺少标题、分类、多语言、Formily Schema 或绑定函数时发布失败。

## 模型总览

```text
SDK / OpenAPI / DB Template
  -> Raw Function Descriptor
  -> FunctionSpec
  -> ResourceSpec + OperationSpec
  -> GeneratedPageCandidate
  -> PageDraft
  -> Published PageSpec
  -> ConsoleMenuSpec
```

| 模型 | 职责 | 不负责 |
| --- | --- | --- |
| `FunctionSpec` | 单个函数能力、输入/输出契约、默认函数表单 | 页面布局、菜单位置 |
| `ResourceSpec` | 稳定业务对象或能力域，如 `player`、`mail`、`inventory` | 具体调用执行 |
| `OperationSpec` | 某个资源上的业务动作、治理信息和页面候选诊断 | 全局菜单结构、最终页面位置 |
| `PageSpec` | 页面级 Formily 组件树和显式数据映射，组合函数、表格、详情、动作和分页 | 底层协议和函数注册 |
| `PageDraft` | 一个作用域内可编辑的 PageSpec 草稿及其修订 | 运行控制台展示 |
| `PublishedPageSpec` | 已校验、冻结函数契约、可运行的页面快照 | 继续猜测和补全 |
| `ConsoleMenuSpec` | 从已发布页面生成的左侧运行控制台菜单 | 保存业务配置 |

`Workspace` 不是 Dashboard 领域模型。历史的 WorkspaceConfig 已废弃；用户面对的编辑入口叫 **Page Studio（页面工作台）**，运行入口叫 **运行控制台**。两者分别操作草稿和已发布快照，不能共享旧 `WorkspaceConfig`、`objectKey` 或 `layout` 概念。

## 概念边界

### Function 与 Function Form

Function 是平台最小可执行能力，例如 `player.list`、`player.ban`、`mail.send`、`reward.batchGrant`。

函数注册只提交能力契约，尤其是 `inputSchema`、`outputSchema`、版本、说明和可选的业务归属；它不提交 Formily、页面组件、表格列、路由、菜单或布局。

Server 在归一化阶段根据 `inputSchema` 自动派生 **Function Form**：它是可重复生成的 Formily 输入表单，用于函数目录的单函数调用，以及 Page Studio 中 binding 的默认表单。这个派生表单不是 SDK 注册者设计的页面 UI。没有 `inputSchema` 时，只生成一个 JSON `payload` 输入框并显示诊断，不根据函数名猜业务字段。

管理员如果确实需要调整单函数输入体验，可在函数目录的“表单配置”中于注册之后创建受控 override；它只影响该函数输入表单，不决定页面布局、菜单、表格、详情或操作位置。

Function Form 不负责：

- 页面路由。
- 左侧菜单。
- 分页状态。
- 表格列。
- 详情布局。
- 多函数组合。

OpenAPI / SDK 注册中不接受 `x-ui` / `ui` 作为 Function Form 或 Page UI 的来源。这样函数提供者不需要了解 Dashboard 的 Formily 组件；所有人工 UI 决策都在平台注册后的函数表单配置或 Page Studio 中完成。

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

Operation 是某个 Function 在 Resource 中的业务动作，不是页面类型。

注册侧只允许表达业务动作：

| 字段 | 含义 | 示例 |
| --- | --- | --- |
| `operation` | 业务动作 key | `list`、`get`、`ban`、`grant`、`send` |

页面候选类型和位置由 Server 内部生成器或 Page Studio 决定：

| 概念 | 含义 | 所属模型 |
| --- | --- | --- |
| `PageCandidate.kind` | 候选页面形态，例如 `entity`、`operation`、`task`、`report` | Server 生成诊断 |
| `PageFunctionBinding.usage` | 页面实际消费函数的方式，例如 `query`、`detail`、`action`、`task`、`report` | PageSpec |
| 组件位置 | 表格数据源、行操作、工具栏、独立页、图表等 | PageSpec Formily schema |

SDK/OpenAPI 注册不得提供 `operationKind`、`placement`、`pageHint`、菜单或动态显示字段。最终页面类型、组件和位置只由 Page Studio 保存的 PageSpec 决定。

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

### Page Studio（页面工作台）

Page Studio 是 PageSpec 草稿管理器。它负责生成建议、编辑、预览、校验、发布和回滚，并且只在全局 `game_id + env` scope 内工作；scope 不是编辑器字段。

Page Studio 不负责：

- 从函数名猜语义。
- 维护动态菜单字典。
- 把未发布页面展示到运行控制台。
- 生成第二套非 Formily UI 协议。

### 运行控制台

运行控制台是执行层，只展示当前 scope 的已发布 PageSpec，并且只能通过已发布 binding 执行函数。

它不读取函数目录来生成左侧菜单，不读取草稿，不在前端推断分类。左侧菜单唯一来源是：

```text
PublishedPageSpec[] -> ConsoleMenuSpec
```

## 判断规则

读代码或设计新功能时，按下面顺序判断，不跳步：

1. 先判断是不是一个可执行能力：是则进入 `FunctionSpec`。
2. 再判断它是否围绕稳定资源展开：是则关联 `ResourceSpec`，否则只保留为独立 Operation 候选。
3. 再分析页面候选：只能使用 FunctionSpec、JSON Schema、执行模式和可验证 PageContract 做确定性结构分析；无法确认时生成保守候选和 diagnostics，不能根据函数名猜业务字段。
4. 再生成 PageSpec 候选：候选不是发布结果，必须进入 Page Studio 确认、修改或直接接受。
6. 校验页面组件 props、数据映射、函数契约和权限后发布冻结快照。
7. 只有当前 scope 的 PublishedPageSpec 才能进入运行控制台菜单和执行链路。

典型判断：

| 函数 | Resource | Operation | 可能页面形态 | 最终位置示例 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `player.list` | `player` | `list` | Entity Page | DataTable 数据源 | 玩家列表页的数据源 |
| `player.get` | `player` | `get` | Entity Page | DetailPanel 数据源 | 玩家详情的数据源 |
| `player.ban` | `player` | `ban` | Entity Page 或 Operation Page | 行操作或独立操作 | 只有 PageSpec 映射到选中玩家后才能作为行操作 |
| `mail.send` | `mail` 或无 Resource | `send` | Operation Page 或 Entity Page | 独立页或工具栏动作 | 如果是全局发邮件，独立页；如果在邮件模板页内触发，可作为工具栏动作 |
| `reward.batchGrant` | `reward` | `batchGrant` | Task Page | 独立任务页或批量操作 | 长耗时或批量操作需要任务页状态 |
| `analytics.retention` | `analytics` | `retention` | Report Page | 图表或报表页 | 查询结果更适合图表和报表 |
| `cache.refresh` | 无 Resource 或 `system` | `refresh` | Operation Page | 独立操作页 | 运维命令不属于 Entity Page |

关键约束：

- 函数注册成功只代表函数目录可见，不代表运行控制台可见。
- 缺少可验证的 PageContract、分页、列、mapping 或默认语言 labels 时，可以生成候选或预览草稿，但不能发布到运行控制台。
- `mail.send`、`cache.refresh` 这类函数不能因为有 `mail` 或 `cache` 前缀就强行进入 Entity Page。

## 术语定义

### FunctionSpec

`FunctionSpec` 是函数注册归一化后的内部模型。

必填字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 全局唯一函数 ID，例如 `player.ban` |
| `version` | 函数版本 |
| `inputSchema` | 函数输入 JSON Schema；注册的唯一输入 UI 事实来源 |

推荐字段：

| 字段 | 说明 |
| --- | --- |
| `summary` | 一句话简介，多语言文本 |
| `description` | 详细说明，支持 Markdown |
| `operation` | 业务操作 key，例如 `ban` |
| `resource` | 业务资源或能力域 key，例如 `player` |
| `risk` | 风险等级 |
| `tags` | 搜索和治理标签 |
| `outputSchema` | 响应 JSON Schema |

`summary`、`description` 只能用于函数目录、搜索和候选说明，不能成为运行控制台菜单、页面标题或按钮文案的事实来源。SDK/OpenAPI 注册不得包含 `displayName`、分类 labels、资源 labels、操作 labels、`operationKind`、`placement` 或 `pageHint`。

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

`ResourceSpec` 可以来自显式 `resource`，也可以由函数 ID 的对象段作为候选确定。它可以带 labels 供 Page Studio 初始化草稿，但运行控制台最终只读取 PublishedPageSpec 的分类和标题。

### OperationSpec

`OperationSpec` 描述归一化后的业务动作和候选诊断。它不是 SDK descriptor 的直接镜像，也不保存最终页面位置。

```json
{
  "functionId": "player.ban",
  "resourceKey": "player",
  "operation": "ban",
  "risk": "danger",
  "candidate": {
    "kind": "operation",
    "quality": "needs_review",
    "diagnostics": [
      {
        "code": "page_mapping_required",
        "message": "需要在 Page Studio 中确认 playerId 来源和按钮位置"
      }
    ]
  }
}
```

`candidate.kind` 是归一化后的候选页面形态，只能由 Server 的确定性契约分析或 Page Studio 编辑结果给出；缺失时只能标记 `needs_review`，不能让前端猜测。允许的值：

| `candidate.kind` | 含义 | 典型页面 |
| --- | --- | --- |
| `entity` | 围绕稳定资源组合列表、详情和动作 | 玩家管理、邮件模板管理 |
| `operation` | 独立同步命令或一次性操作 | 发送邮件、刷新缓存 |
| `task` | 异步、批量或长耗时任务 | 批量发奖 |
| `report` | 查询和图表/报表展示 | 留存分析 |

候选不等于发布。函数可以出现在函数目录；Server 可以给出保守候选或 `needs_review` diagnostics。无论候选质量如何，函数注册都不会自动发布正式 Page。

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
  "bindings": [
    {
      "id": "player.list.query",
      "functionId": "player.list",
      "usage": "query",
      "inputMapping": { "page": "$.pagination.page", "pageSize": "$.pagination.pageSize" },
      "outputMapping": { "stateKey": "players", "itemsPath": "$.response.items", "totalPath": "$.response.total" },
      "execution": { "mode": "sync" }
    }
  ],
  "schema": {
    "schemaVersion": "v1",
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
          "bindingId": "player.list.query",
          "formSchemaRef": "binding://player.list.query/input",
          "resultStateKey": "players"
        }
      },
      "table": {
        "type": "void",
        "x-component": "DataTable",
        "x-component-props": {
          "bindingId": "player.list.query",
          "stateKey": "players",
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

`PageFunctionBinding.usage` 是页面实际如何消费函数（`query`、`detail`、`action`、`task`、`report` 等）。页面位置由 Formily schema 中的组件和 `bindingId` 引用决定，不能复用旧 `role` 或注册期 `placement` 字段。发布校验验证映射、组件 props、权限和契约快照，而不是把候选信息当成不可变运行规则。

### 作用域、标识和并发

Dashboard 页面不是全局配置。Croupier 的标准业务作用域始终是全局上下文中的 `game_id + env`；Page Studio、Console API 和函数执行都必须使用同一个作用域。

页面的稳定身份由三元组构成：

```text
PageIdentity = scope(game_id, env) + pageKey
```

因此：

- 数据库唯一约束必须是 `(game_id, env, page_key)`，`page_key` 不能全局唯一。
- URL 保持 `/console/:categoryKey/:pageKey`，不携带 scope；scope 只由全局选择器和服务端 session/context 解析，禁止页面内再次选择。
- 请求 scope 只由全局选择器通过统一请求头/上下文传递，服务端必须按当前登录用户的 game/env 权限解析并授权；不能信任页面 JSON、URL 参数或调用 payload 自带的 scope 覆盖值。
- `resourceKey`、`functionId` 是跨 scope 的逻辑 key；注册表解析、权限与调度仍必须在当前 scope 内查找实际能力。
- 草稿保存必须携带乐观并发版本（例如 `draftRevision` / `ETag`）。版本不匹配返回 `409 conflict` 和当前摘要，禁止最后写入者静默覆盖他人编辑。

`PageSpec` 描述的是页面内容；作用域、草稿修订、创建人和发布时间属于包裹它的 `PageDraft` / `PublishedPageSpec` 记录，不能散落进 `metadata`。

### 发布快照与函数契约冻结

发布不是把草稿标记为 `published`，而是生成可复现的运行产物。每次发布都要保存完整 `PageSpec`，并冻结它依赖的函数契约摘要：

```text
PublishedPageSpec
  = PageSpec
  + scope
  + page revision / publish version
  + binding snapshots(functionId, descriptorVersion, inputSchemaDigest,
                      outputSchemaDigest, risk, permission, executionMode)
  + rendererSchemaVersion
```

原因是函数注册可以独立升级。若 `player.list` 的输入、输出、风险或权限改变，旧页面不能在运行时悄悄按新含义执行。

- 发布时：每个 binding 都必须解析到当前 scope 内已启用的 FunctionSpec，并写入其契约摘要。
- 运行时：执行前比较当前 FunctionSpec 和发布快照。兼容变更可按明确规则继续执行；不兼容变更必须使页面显示 `binding_stale`，拒绝调用并要求重新校验/发布。
- 取消发布只使该 scope 的 active snapshot 失效；历史快照、审计记录和可回滚版本必须保留。
- 草稿回滚、发布回滚和运行快照回滚是不同动作。回滚草稿不会自动替换运行态，仍需要一次显式发布。

不得以“每次运行都从最新函数目录读取”为由跳过冻结；那会让发布、审计和审批失去可追溯性。

### Page Schema、组件 Props 与数据契约

“Formily 是唯一 UI 协议”指 Page 的序列化结构只有一份 `schema`，并不意味着任意 `x-component-props` 都可以是无约束 JSON。页面组件是平台 ABI，必须有版本化、强类型、服务端可校验的 props 契约。

`PageSpec.schema` 至少包含：

- `schemaVersion`：页面组件协议版本；未知版本直接拒绝预览、发布和渲染。
- 仅允许注册表中的 `x-component`：`ConsolePage`、`QueryForm`、`DataTable`、`DetailPanel`、`ActionButton`、`ActionGroup`、`ResultPanel`、`TaskTimeline`、`ChartPanel`。
- `x-component-props` 必须匹配对应组件的 JSON Schema；未知组件、未知关键 props、缺少必填 props 都是发布错误。
- 所有函数引用必须使用 `bindingId`，而不是在任意组件里写裸 `functionId`。binding 定义函数、角色、输入映射、输出映射和执行策略；同一个函数可以安全地被多个组件以不同方式使用。

页面数据流只能通过显式的页面状态和映射发生：

```text
QueryForm values --inputMapping--> PageExecution binding
  -> response/task handle --outputMapping--> page state
  -> DataTable / DetailPanel / ChartPanel
```

禁止组件读取“上一条任意函数结果”或从表格整行对象直接当作操作 payload。行操作、批量操作、详情操作都必须声明输入映射，例如 `playerId <- row.id`、`ids <- selection[*].id`。这样才能在发布时验证字段路径，并在函数契约变化时准确失效。

最小 props 合同如下：

| 组件 | 发布时必须明确的字段 |
| --- | --- |
| `QueryForm` | `bindingId`、可选 `formSchemaRef`、`inputMapping`、`resultStateKey` |
| `DataTable` | `bindingId`、`itemsPath`、`totalPath`、分页 `pageField/pageSizeField`、列定义或 `columnsPath` |
| `DetailPanel` | `bindingId` 或 `stateKey`、详情 `dataPath`、选择项输入映射 |
| `ActionButton` | `bindingId`、`inputMapping`、确认策略；高风险动作不能由页面降低函数风险 |
| `ActionGroup` | 已声明的 action binding 列表；批量操作必须声明 selection mapping |
| `TaskTimeline` | `taskBindingId`、任务状态/事件/结果来源 |
| `ChartPanel` | `stateKey`、`dataPath`、显式 chart type 和字段映射 |

表格列自动从运行时首批数据推断只能作为 Page Studio 预览辅助，不能进入 PublishedPageSpec。发布产物必须有稳定列定义或明确 `columnsPath`，否则 `needs_review` 或 `blocked`。

### 受控页面执行

运行控制台不能复用无页面上下文的 `POST /functions/:id/invoke` 作为最终执行入口。该接口仍可保留给函数目录和 API 调试，但 Page 运行必须经过独立的受控入口：

```text
POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute
  -> load active PublishedPageSpec in current scope
  -> verify page/category route and binding snapshot
  -> validate payload against frozen input schema
  -> enforce function permission + page permission + risk/approval policy
  -> inject immutable scope, actor, pageKey, publishVersion, bindingId, trace context
  -> dispatch / start task
  -> audit execution and return typed execution envelope
```

执行响应要统一为 `PageExecutionResult`，至少包含 `kind`（`sync` / `task` / `approval`）、`requestId`、`traceId`、`data` 或 `taskId/approvalId`、结构化 diagnostics。Task 进度和结果必须按 `taskId` 读取现有任务链路，不允许用前端内存中的“最后结果”模拟。

安全与治理边界：

- 函数权限始终是下限；Page 只能增加更严格的页面可见性/执行权限，不能绕过或放宽函数权限。
- 函数风险、审批和审计策略由 FunctionSpec/Policy 决定；Page 可增加确认文案，但不能把 `danger` 降为 `safe`。
- 审计与 OTel span 必须记录 `game_id`、`env`、`page_key`、`publish_version`、`binding_id`、`function_id`、操作者和调度目标，形成从菜单点击到 Agent 调度的连续关联。
- 发布后页面不可通过浏览器传入任意 `functionId`、`route`、`targetServiceID` 或 scope 来提升执行范围；这些参数必须由绑定和服务端策略决定。

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
- 在 Page Studio 中编辑 PageSpec。
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

分页是 Page 的职责，不是 Function Form 的职责。

Function Form 只渲染单次函数输入表单。Page 负责维护分页状态，并把分页字段映射到查询函数参数。

分页映射必须显式写入 PageSpec：

```json
{
  "x-component": "DataTable",
  "x-component-props": {
    "bindingId": "player.list.table",
    "pagination": {
      "pageField": "page",
      "pageSizeField": "pageSize",
      "totalField": "$.response.total",
      "itemsField": "$.response.items"
    }
  }
}
```

如果函数分页字段不同，必须在 PageSpec 中显式声明映射，不由前端运行时猜测。默认生成器只有在输出契约明确包含 items、total 和对应分页输入字段时才能产生 ready 的 `DataTable`；否则只能标记 `needs_review` 或 `blocked`。

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

- FunctionSpec 归一化阶段最多产生 `resourceKey` 或 PageCandidate 分组建议，不产生运行菜单分类。
- ResourceSpec 生成阶段最多为 Page Studio 提供候选，不是菜单事实源。
- PageSpec 保存或发布时必须写入最终 `category.key` 与 `category.labels`。
- 发布时校验分类 key、默认语言 labels 和页面 title。
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
- 字段标题和帮助文案。

发布校验要求：

- 必须提供系统默认语言。
- 必须覆盖 Dashboard 启用的语言集合。
- 缺失语言时发布失败。

上述动态页面文案只属于 PageSpec / PublishedPageSpec。函数注册中的 `summary`、`description` 和 JSON Schema 字段说明可以用于函数目录和候选说明，但不能作为运行控制台分类、页面标题或按钮文案的事实来源。运行时不从静态 locale 文件补动态分类和页面标题。静态 locale 只用于固定系统菜单，例如“系统配置”“运行控制台”“权限管理”。

## 默认生成策略

Server 可以生成默认 PageSpec，但生成必须基于明确元数据。

允许：

```text
FunctionSpec.inputSchema -> Server 派生 Function Form
ResourceSpec + OperationSpec + 可验证 PageContract -> Entity PageSpec 初稿
OperationSpec + sync execution contract -> Operation PageSpec 初稿
OperationSpec + task execution contract -> Task PageSpec 初稿
OperationSpec + report/chart contract -> Report PageSpec 初稿
```

不允许：

```text
前端按函数 ID 后缀临时猜页面类型
把所有函数默认塞入 Entity Page
把 Function Form 当 Page Schema
要求函数注册者提供 Formily、Page schema、表格列或菜单布局
缺少可验证 PageContract、mapping、labels 或 binding 时把猜测结果自动发布为正式 Page
为动态分类改静态 i18n 文件
```

对于缺失 `resource`、`operation`、`outputSchema`、分页字段、列定义、输入/输出 mapping、任务追踪或图表契约的函数，Server 可以生成保守“待编排建议”，但建议不是发布产物。用户在 Page Studio 确认和补齐后才生成可发布 PageSpec。

## Page Studio 职责

Page Studio 是 PageSpec 的草稿和发布管理容器，不是函数语义归一化器。

Page Studio 负责：

- 保存用户编辑后的 PageSpec。
- 预览 PageSpec。
- 校验 PageSpec。
- 发布 PublishedPageSpec。
- 回滚历史版本。
- 管理页面可见性/执行权限和发布记录。
- 使用乐观并发控制草稿编辑冲突。
- 显示 binding 契约变更和发布失效 diagnostics。

Page Studio 不负责：

- 从函数 ID 猜 operation kind。
- 从 JSON Schema 临时猜 Formily 组件。
- 维护动态分类字典。
- 渲染未发布页面到运行控制台。
- 让页面内自行选择或覆盖全局 `game_id/env`。

## 数据存储建议

目标存储应体现模型边界：

| 存储 | 内容 |
| --- | --- |
| `function_specs` | 当前 scope 内归一化函数能力和治理字段 |
| `function_form_overrides` | 管理员创建的单函数 Formily 表单 override；无 override 时从 inputSchema 派生 |
| `resource_specs` | 资源、分类、资源多语言、排序 |
| `page_specs` | 带 scope 和草稿修订的 PageSpec |
| `published_page_specs` | 带 scope、binding contract snapshot 和 renderer schema version 的不可变 PageSpec 快照 |
| `page_versions` | 草稿与发布前完整 PageSpec 历史 |
| `page_execution_audits` 或既有审计表 | 页面 binding 执行与 trace/task/approval 关联 |

活跃设计不定义 `layout` 枚举、实体 CRUD 配置或动态分类字典。已有数据需要通过一次性迁移转换为 PageSpec，迁移失败的记录不能发布。

## API 边界

建议服务端提供以下能力：

| API | 职责 |
| --- | --- |
| `GET /api/v1/functions/descriptors` | 查看函数原始能力目录 |
| `GET /api/v1/functions/:id/form` | 获取 Server 派生或管理员 override 的单函数 Formily 表单 |
| `PUT /api/v1/functions/:id/form` | 管理员保存单函数表单 override；不属于 SDK/OpenAPI 注册 |
| `GET /api/v1/resources` | 获取归一化 ResourceSpec |
| `GET /api/v1/pages/generated` | 获取默认 PageSpec 候选和生成 diagnostics |
| `GET /api/v1/pages` | 获取当前 scope 的 PageSpec 草稿 |
| `PUT /api/v1/pages/:pageKey` | 以 `If-Match` / `draftRevision` 保存 PageSpec 草稿 |
| `POST /api/v1/pages/:pageKey/validate` | 校验 schema、组件 props、映射、binding、权限与契约摘要 |
| `POST /api/v1/pages/:pageKey/publish` | 生成当前 scope 的不可变 PageSpec 发布快照 |
| `GET /api/v1/console/pages` | 获取已发布 PageSpec |
| `GET /api/v1/console/menu` | 获取 ConsoleMenuSpec |
| `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute` | 受控执行已发布页面 binding |

Dashboard 不应直接从函数目录组装运行控制台菜单。

## 验收规则

- 新注册 `mail.send` 后，不改前端代码、不改静态 locale 文件，也能生成默认操作页或待编排建议。
- `player.ban` 没有显式分类时归入 `player`；显式 `support` 时归入 `support`。
- 动态分类、资源、页面标题来自 PageSpec 的强类型 labels，不依赖不受约束的 metadata。
- 所有运行时 UI schema 都是 Formily JSON Schema，且组件 props 满足平台的版本化 ABI。
- 非 Formily Schema 在保存或渲染阶段直接报错。
- 缺少可验证 PageContract、分页、列、mapping 或默认语言 labels 的函数仍可注册；只能生成保守候选或待编排 diagnostics，绝不自动发布。
- 运行控制台左侧菜单只展示已发布 PageSpec。
- 分页字段必须显式映射。
- 函数目录、Page Studio、运行控制台之间没有重复菜单生成逻辑。
- 同一个 `pageKey` 可安全存在于不同 `game_id/env`，但不能跨 scope 读取、发布或执行。
- 函数输入、输出、风险或权限发生不兼容变更后，关联发布页必须被标为 stale，不能静默调用。
- 页面执行的审计记录和 trace 能关联到 page version、binding 和函数调度。

## 迁移计划

### 阶段 1：模型冻结

- 本文档作为权威设计入口。
- 删除动态分类静态 locale 设计。
- 删除旧 layout 配置作为新协议的文档入口。
- 冻结 PageIdentity、binding snapshot、组件 props ABI 和受控执行入口；未完成前不得宣称 P0 可发布。

### 阶段 2：后端归一化

- 新增强类型 `FunctionSpec`、`ResourceSpec`、`OperationSpec`、`PageSpec`。
- `DescriptorsLogic` 输出保持目录用途，但 Page 生成改用归一化服务。
- HTTP 边界上可以用 `json.RawMessage` 承载 JSON，但归一化服务输出必须替换为强类型 DTO。

### 阶段 3：默认 Page 生成

- Server 根据函数契约归一化结果生成 Function Form 和 Formily PageSpec 候选。
- 缺失关键语义时只生成待编排建议，不发布。
- 生成结果带来源、版本和质量诊断。

### 阶段 4：前端渲染收敛

- Page Studio 编辑 PageSpec，并使用乐观并发保护草稿。
- Runtime renderer 只渲染 Formily PageSpec。
- 移除前端基于函数 ID 后缀的页面类型猜测。
- 移除自定义 layout 枚举写入路径。

### 阶段 5：运行控制台菜单收敛

- 菜单改为读取 PublishedPageSpec 或 ConsoleMenuSpec。
- 动态菜单项全部 `locale: false`。
- 动态标题由当前语言读取 PageSpec labels。
- 缺失语言或分类 labels 时发布失败。

### 阶段 6：数据迁移和清理

- 将既有页面配置转换为带 scope 的 PageSpec。
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
