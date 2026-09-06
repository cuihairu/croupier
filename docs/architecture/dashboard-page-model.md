---
title: Dashboard Resource/Page 模型
icon: layout-dashboard
order: 5
category:
  - 系统架构
tag:
  - Dashboard
  - ProComponents
  - 函数注册
  - 动态菜单
---

# Dashboard Resource/Page 模型

> **状态**：In progress -- 本文是 Dashboard 页面模型的权威定义。实现、文档和 SDK 以本文的正向模型为准；旧模型按 [旧模型删除清单](./legacy-deletion-inventory.md) 清理，并由 `scripts/dashboard_vnext_guard.sh` 防回流；真实浏览器回归仍以根目录 `todo.md` 的未完成项目为准。

## 决策

Croupier 保留 **React + Umi + Ant Design Pro + ProComponents**，不集成 React Admin，也不把 React Admin 的 CRUD `DataProvider` 作为平台协议。

ProComponents 已经提供后台所需的成熟组件能力；Croupier 缺少的是把函数能力稳定地变成页面的领域模型，而不是另一套组件库。

```text
SDK / OpenAPI
  -> FunctionContract
  -> ResourceCapability
  -> CapabilitySemantics
  -> PageProposal
  -> PageDraft
  -> PublishedPageSpec
  -> ConsoleMenuSpec
  -> Controlled Page Execution
```

本模型同时满足两个事实：

- 游戏后台有大量玩家、订单、配置、邮件模板、活动等 **CRUD Resource**；CRUD 是默认页面生成的主路径，而不是被排斥的旧概念。
- 游戏后台也有封禁、补偿、批量发奖、任务、审批、报表、运维命令等 **非 CRUD 能力**；它们必须是一等页面类型，不能被强行套进 CRUD。

## 产品目标

运营人员的完整路径必须是：

```text
注册函数或导入 OpenAPI
  -> Server 解析并持久化能力、语义和诊断
  -> 生成可追溯的默认 PageProposal
  -> 用户直接接受并发布，或在 Page Studio 调整
  -> Console 按已发布 Page 的分类生成左侧菜单
  -> 页面通过已发布 binding 受控执行
```

要求：

1. 服务开发者只提交函数能力契约，不提交页面、菜单、表格列、按钮位置或前端组件。
2. 只要能力足以安全执行，平台必须生成可直接发布的默认 Operation Page；用户不能从空白 JSON 开始。
3. 具备标准 CRUD 语义的 Resource，平台必须生成可用的列表、详情、新建、编辑、删除和资源动作页面。
4. 发布后的页面、菜单、权限、审计和 trace 必须可复现；函数变更不得静默改变已发布 UI。
5. 正常用户编辑的是“资源、列表、列、表单、动作、分类”等业务概念；JSON 仅允许用于导入导出和受控诊断，不能作为主工作流。

## 为什么 JSON Schema 不足以直接生成完整页面

输入 JSON Schema 可以稳定生成表单字段，输出 JSON Schema 可以稳定生成候选列和详情字段；它们只描述数据形状，不描述业务用法。

例如，`player.ban` 的 `playerId` 可能来自手工输入、列表选中行或详情上下文；一个数组输出可能是表格、下拉选项或图表序列。平台不能把这些选择伪装成 JSON Schema 推论。

因此自动生成分为两层：

| 层       | 输入                                              | 产物                    | 是否可由 SDK/OpenAPI 提供              |
| -------- | ------------------------------------------------- | ----------------------- | -------------------------------------- |
| 数据契约 | JSON Schema、HTTP method/path、函数版本和治理字段 | FunctionContract        | 可以                                   |
| 能力语义 | CRUD 意图、对象标识、任务/报表执行特征            | CapabilitySemantics     | 可从 REST 推导；SDK 可显式提供有限语义 |
| 页面编排 | 分类、标题、列、筛选、动作位置、映射和表单展示    | PageProposal / PageSpec | 不可以；只由 Server/Page Studio 产生   |

`CapabilitySemantics` 不是 UI。它不含菜单、路由、标题、表格列或按钮位置；它只回答“该函数在资源生命周期中做什么”。

## 三层模型

### FunctionContract

FunctionContract 是每个函数的可执行能力和治理契约：

```ts
interface FunctionContract {
  id: string;
  version: string;
  enabled: boolean;
  summary?: LocalizedText;
  description?: LocalizedText;
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;
  risk: RiskLevel;
  permission?: string;
  execution: "sync" | "task";
  approval: ApprovalPolicy;
  resourceKey?: string;
  operationKey?: string;
  capability?: CapabilityKind;
}

type LocaleCode = string;
type LocalizedText = Readonly<Record<LocaleCode, string>>;
type JsonPointer = "" | `/${string}`;
type JSONPrimitive = string | number | boolean | null;
type JSONValue = JSONPrimitive | JSONValue[] | { [key: string]: JSONValue };
type JSONSchema = boolean | { [key: string]: JSONValue };
type RiskLevel = "safe" | "warning" | "high" | "danger";

interface Scope {
  gameId: string;
  env: string;
}

interface FunctionRef {
  functionId: string;
  contractVersion: string;
  inputSchemaDigest: string;
  outputSchemaDigest: string;
}

interface SourceDigest {
  kind: "function_contract" | "capability_semantics";
  id: string;
  digest: string;
}

interface Diagnostic {
  code: string;
  severity: "info" | "warning" | "error";
  message: LocalizedText;
  path?: JsonPointer;
}

interface ApprovalPolicy {
  required: boolean;
  policyKey?: string;
}

type CapabilityKind =
  | "collection_query"
  | "item_query"
  | "create"
  | "update"
  | "delete"
  | "action"
  | "task"
  | "report";
```

`execution` 与 `approval` 正交：`execution: 'task'` 且 `approval.required: true` 表示审批通过后才启动异步任务；同步操作也可以要求审批。`approval.policyKey` 只引用平台已配置的治理策略，缺失时由 Server 按风险策略解析默认值；它不是页面 UI，不能由浏览器覆盖。

`resourceKey`、`operationKey` 和 `capability` 都是业务能力语义，不是页面语义。`capability` 只允许上述受控枚举；页面设计只能由 PageProposal/PageSpec 表达。

函数缺少 `resourceKey` 或 `capability` 不得阻断注册；它仍可生成独立 Operation Page。任意 SDK 函数没有 REST method/path，若想可靠加入 Resource CRUD 页面，必须提供这一个受控能力语义，不能要求 Server 从函数名猜测。

### ResourceCapability 与 CapabilitySemantics

ResourceCapability 将同一资源的函数关联起来；CapabilitySemantics 是其可验证、版本化的语义结果。

```ts
interface ResourceCapability {
  resourceKey: string;
  functions: FunctionContract[];
}

interface CapabilitySemantics {
  resourceKey: string;
  identity?: IdentitySemantic;
  collection?: CollectionSemantic;
  item?: ItemSemantic;
  lifecycle: Partial<Record<"create" | "update" | "delete", FunctionRef>>;
  actions: ActionSemantic[];
  tasks: TaskSemantic[];
  reports: ReportSemantic[];
  sourceDigest: string;
  provenance: SemanticProvenance[];
  diagnostics: Diagnostic[];
}

interface IdentitySemantic {
  itemPath: JsonPointer; // canonical item schema 中的唯一标识字段
  valueType: JsonScalarType;
}

type JsonScalarType = "string" | "number" | "integer" | "boolean";

interface CollectionSemantic {
  query: FunctionRef;
  itemsPath: JsonPointer; // collection query 输出中项目数组的位置
  itemSchemaDigest: string;
  pagination?: OffsetPaginationSemantic | CursorPaginationSemantic;
}

interface ItemSemantic {
  query?: FunctionRef;
  itemPath: JsonPointer;
  itemSchemaDigest: string;
}

interface OffsetPaginationSemantic {
  kind: "offset";
  request: { offset: JsonPointer; limit: JsonPointer };
  response: { total?: JsonPointer; hasMore?: JsonPointer };
}

interface CursorPaginationSemantic {
  kind: "cursor";
  request: { cursor: JsonPointer; limit?: JsonPointer };
  response: {
    nextCursor: JsonPointer;
    previousCursor?: JsonPointer;
    hasMore?: JsonPointer;
  };
}

interface ActionSemantic {
  function: FunctionRef;
  subject: "resource_item" | "resource_selection" | "none";
  identityInput?: JsonPointer;
}

interface TaskSemantic {
  start: FunctionRef;
  taskId: { resultPath: JsonPointer; valueType: JsonScalarType };
  status: {
    function: FunctionRef;
    taskIdInput: JsonPointer;
    statePath: JsonPointer;
  };
  events?: {
    function: FunctionRef;
    taskIdInput: JsonPointer;
    eventsPath: JsonPointer;
  };
  result?: {
    function: FunctionRef;
    taskIdInput: JsonPointer;
    resultPath: JsonPointer;
  };
  cancel?: { function: FunctionRef; taskIdInput: JsonPointer };
  retry?: { function: FunctionRef; taskIdInput: JsonPointer };
}

interface ReportSemantic {
  query: FunctionRef;
  datasetPath: JsonPointer;
  dimensions: JsonPointer[];
  metrics: JsonPointer[];
}

interface SemanticProvenance {
  field: string;
  source: "openapi_rest" | "sdk_explicit" | "platform_review";
  sourceDigest: string;
  confidence: "high" | "low";
  status: "effective" | "overridden" | "conflict";
}
```

`IdentitySemantic.itemPath` 必须在 collection item 或 item query 的输出 schema 中唯一存在；item/update/delete/action 的 identity input 由 typed selector 显式映射并校验，不得要求 collection query 的 input 包含 identity。`CollectionSemantic.pagination` 必须同时声明请求参数和响应元数据的 JSON Pointer；offset 分页必须至少提供 `total` 或 `hasMore`，cursor 分页必须提供 `nextCursor`；缺失时只生成不带分页控件的列表，不得猜测 offset/cursor 协议。

`ActionSemantic.subject` 是资源操作所需的业务上下文，不是按钮位置：`resource_item` 映射为行操作，`resource_selection` 映射为批量操作，`none` 映射为资源工具栏操作；无法安全判定 subject 或 identity input 时只生成独立 OperationPage。`TaskSemantic` 与 `ReportSemantic` 的所有 pointer 都必须可由对应 FunctionContract schema 验证，否则只能生成 `needs_review`。

可接受的来源优先级：

1. OpenAPI 标准 REST 形态，如 `GET /players`、`POST /players`、`GET/PATCH/DELETE /players/{id}`。
2. SDK descriptor 的受控 `resourceKey + capability` 语义。
3. 平台管理员在 Resource Catalog 中保存的语义补充；它独立于函数注册，需版本化、审计和权限控制。

不允许从 `player.list`、`player.ban` 等名称猜测对象 ID、分页字段、动作位置或页面类型。允许将确定性 REST 形态与 JSON Schema 字段产生为“高置信度建议”，但低置信度建议不得自动发布。

来源冲突按字段裁决：`platform_review` 是人工最终裁决，优先级最高；`sdk_explicit` 高于 `openapi_rest` 推导。每个有效值和被覆盖值都必须记录在 `SemanticProvenance`，不能以单一 `source` 掩盖多个来源。未解决冲突必须保留 diagnostic，受影响 Proposal 降级为 `needs_review` 并禁止发布；管理员以版本化 `platform_review` 明确选择后才消除冲突。Resource Catalog 的覆盖必须保存版本、记录审计，并在生效时触发受影响 ResourceCapability 与 PageProposal 的重新计算。

`action` 与 `update` 的判定：幂等修改资源自身字段的生命周期操作归为 `update`；触发资源相关副作用或流程（封禁、补偿、重置、发放）归为 `action`。例如 `player.ban` 是 `player` 资源的 `action`，生成行操作而不是编辑表单。

### PageProposal、PageDraft 与 PublishedPageSpec

PageProposal 是可重新生成的默认页面建议，不是草稿，也不是运行页面：

```ts
interface PageProposal {
  id: string;
  scope: Scope;
  proposalKey: string;
  pageKey: string;
  spec: PageSpec;
  quality: "ready" | "basic" | "needs_review";
  generatorVersion: string;
  sourceDigests: SourceDigest[];
  diagnostics: Diagnostic[];
  createdAt: string;
}

// 不可物化的问题不是 Proposal：只保存诊断与修复指引，不携带 spec。
interface BlockedProposalIssue {
  id: string;
  scope: Scope;
  sourceDigests: SourceDigest[];
  diagnostics: Diagnostic[];
  repairHint: LocalizedText;
  status: "open" | "resolved" | "dismissed";
}
```

`proposalKey` 是生成器幂等身份：一个 ResourceCapability 只能有 `resource:<resourceKey>`，每个独立 Operation/Task/Report 函数分别有 `<kind>:<functionId>`。`pageKey` 固定为 `resource--<resourceKey>` 或 `<kind>--<functionId>`，其中 source key 必须符合 `[a-z0-9][a-z0-9._-]*`；它是可读的路由与发布身份，不得从 summary、labels 或本次生成结果随机生成。分类建议的默认规则以 [ProComponents 页面生成与运行时](./ui-generation.md) 为唯一出处；不得从带 kind 前缀的 pageKey 推断分类。Resource action 只有在 `subject` 可验证时才并入唯一 ResourcePage；否则保留为函数自己的 OperationPage，避免重复菜单或覆盖资源页。

- `ready`：页面全部已声明能力都有可验证的 binding、selector、治理与 renderer 支持，可直接接受并发布。ResourcePage 的写能力是可选的，因此只读 ResourcePage 也可以是 `ready`。
- `basic`：安全的同步 OperationPage，含输入表单、确认、受控执行和结果区；可要求审批，并在审批通过后展示真实结果；可直接接受并发布。
- `needs_review`：语义或映射不完整，必须在 Page Studio 决策后发布。
- 函数不可执行、权限/风险不可校验、schema 无效或 binding 不安全时，生成 BlockedProposalIssue，禁止物化和发布；`blocked` 不是 Proposal quality。

PageDraft 是用户接受 Proposal 后形成的可编辑页面；PublishedPageSpec 是包含完整 PageSpec、binding snapshot、表单展示快照和 renderer version 的不可变运行产物。Proposal 重新生成绝不覆盖 Draft 或 PublishedPageSpec。

### ComponentTemplate（组件模板层，V4）

组件模板是三层之间的**复用中间层**：多个函数（连同其 PageNode 子树编排）
封装为可整体实例化的组件，组件组合为页面。

```ts
interface ComponentTemplate {
  key: string; // 唯一标识（snake/kebab 域名风格）
  name: LocalizedText; // BCP47 本地化名
  description?: LocalizedText;
  category?: string; // 组件库分组（如「资源管理」）
  icon?: string;
  requiredFunctions: string[]; // 实例化前置：scope 内必须存在的函数 id
  tree: PageNode[]; // 组合页编辑器 PageNode 子树（含引用关系）
  builtin: boolean; // 内置模板（由 regenerate 维护）vs 用户保存
  createdBy?: string;
}
```

生成与维护：

- **契约 → 模板自动生成**：agent 注册函数后，生成器按 ResourceCapability
  聚合产出 builtin 模板（如 `player.crud` = list/create/update/delete 四函数
  CRUD 子树），入口 `POST /api/v1/component-templates/regenerate`（手动触发；
  agent 注册不会自动 regenerate）
- **用户保存为组件**：组合页编辑器选中 1..N 节点 → 顶栏「保存为组件」→
  序列化 PageNode 子树存为 `builtin=false` 模板
- **实例化**：组件库面板点击模板 → tree 复制 + id 重分配 + 函数引用重映射进画布；
  可用性检查在前端本地完成（`requiredFunctions` 与 scope 函数集比对，缺失置灰）

REST：`/api/v1/component-templates`（List/Get/Create/Update/Delete/Regenerate），
wire 契约见 [API 文档](../api/component-templates.md)。使用层文档见
[组合页编辑器 V4](../dashboard/composite-editor-v4-design.md)。

## PageSpec：业务级页面协议

PageSpec 是平台唯一的页面编排协议。它不持久化 `ProTable`、`ProForm` 等具体组件名，而是强类型的业务级 DSL：

```text
PageSpec = (pageKey, type, resourceKey?, category, title, icon, order,
            navigation?, resource? | operation? | task? | report?, bindings[])
```

四种页面类型的视图编排：

| 页面类型    | 视图节点（实际 DTO）                                                                                                                                                                                                                                         |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `resource`  | `ListViewSpec`（columns/filters/pagination/rowActions/batchActions/toolbarActions）、`DetailViewSpec`（fields/actions）、`CreateForm`/`UpdateForm`（FormPresentationSpec）、`DeleteAction`（ConfirmActionSpec）                                              |
| `operation` | `Form`（FormPresentationSpec）+ `Confirm`（ConfirmActionSpec）+ `ResultViewSpec`                                                                                                                                                                             |
| `task`      | `Form`（FormPresentationSpec）+ `TaskViewSpec`（status/events/result/cancel 的 bindingId 引用）+ `ResultViewSpec`                                                                                                                                            |
| `report`    | `QueryForm`（FormPresentationSpec）+ `DatasetSpec` + `ChartSpec[]` + 表格 `ListViewSpec`                                                                                                                                                                     |
| `composite` | `CompositePageSpec`：`sections[]`（每区块绑定一个函数；`display` inline/dialog、`group` 弹窗分组、`rowActions`/`toolbar` 按钮动作含 `chain` 动作链、`onSuccessRefresh`、`refreshOn` page_state 联动），另有 `static` 常量表单（不绑定函数，值进 page_state） |

字段级的 wire 契约（含 FormPresentationSpec、Selector AST、Binding usage 枚举与 ABI 版本）以 [PageSpec 协议规范](./pagespec-protocol.md) 为唯一出处；其权威实现是 `internal/dashboard/spec`（Go DTO）与 `web/src/types/dashboard.ts`（前端共享类型），两侧逐项对应。

`PageBinding` 只引用发布期允许执行的 FunctionContract。输入输出映射必须使用受控的 typed selector AST，禁止保存无约束 JSON mapping、裸整行透传或运行时猜路径。

分类、标题、图标与排序是 PageSpec 的顶层强类型字段；`NavigationSpec` 仅承载面包屑与返回行为（breadcrumb、showBack、backPath）。它们只在 PageProposal/PageSpec 中确定，注册侧不能提供菜单事实。页面没有独立的 permissions 字段：权限由 binding 级治理（合同 permission/risk/approval 快照）与 action 级 permission 字段承载。

## CRUD 是主路径，非 CRUD 是一等扩展

### ResourcePage

当 ResourceCapability 有可验证的 `collection_query` 与对象 identity 时，生成 ResourcePage；生命周期写操作是可选能力。只读资源也必须生成查询/列表/详情页面，不能因为缺少 create/update/delete 被错误降级为 OperationPage。语义到页面节点与组件的生成模板见 [ProComponents 页面生成与运行时](./ui-generation.md)。

JSON Schema 为列表列、详情项和表单字段生成候选。`CapabilitySemantics` 解决分页、identity、集合与对象响应；PageProposal 决定列、默认排序、动作位置和展示文本。管理员可覆盖 Proposal，但不能绕过类型和发布校验。

### OperationPage

无法可靠归入资源生命周期的同步函数生成 OperationPage，例如 `mail.send`、`cache.refresh`、`broadcast.send`。它的默认形态是输入表单、风险确认、受控执行和结构化结果，不应被塞入 ResourcePage。

### TaskPage 与 ReportPage

异步函数生成 TaskPage；报告语义生成 ReportPage。TaskPage 必须接入真实 task 状态、事件、取消/重试和结果，不得只显示 taskId。

TaskPage 的生命周期能力来自 `TaskSemantic`，生成器把 `start/status/events/result/cancel` 转成 PageSpec binding，并在 `TaskViewSpec` 中保存 bindingId 引用、固定 `taskId` page state key 与 `status.statePath`。运行时仍统一走 `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute`，浏览器只提交 `page_state.taskId` 作为 selector source，不传 functionId、target、gameId 或 env。缺少 `status` binding 或 `statusStatePath` 的 TaskPage 不可发布；`events/result/cancel` 只有存在对应真实函数语义时才显示入口；retry 在真实 runtime 闭环前禁止发布。

ReportPage 必须使用已验证的数据集、指标和图表字段，不得只显示 JSON。

### CompositePage（自由组合页）

组合页把多个函数区块编排成一个工作台页面（如「玩家管理」= 玩家表格 + 行操作弹窗 + 提交刷新）。区块是**平铺列表**，编辑器（组合页编辑器 V3）内部是组件树，保存时编译为平铺 sections——发布链（提案/校验/版本/菜单）零特判。

`CompositeSection` 字段模型（权威实现 `internal/dashboard/spec/types.go`，前端 `web/src/types/dashboard.ts`）：

| 字段               | 类型                    | 语义                                                                                                                                                 |
| ------------------ | ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `key`              | string                  | 区块唯一标识。**同函数多实例**时依次 `fid`、`fid-2`、`fid-3`（一个数据源可拖多个组件分别配置）；编辑器支持声明固定 `sectionKey`（回读固化，不随增删/排序漂移，见组合页编辑器 V4「区块 key」）；创建端点重复 key 显式报错                            |
| `bindingId`        | string                  | 引用 `PageSpec.bindings` 的绑定                                                                                                                      |
| `view`             | string                  | `table` / `fields` / `form`                                                                                                                          |
| `span`             | int                     | 栅格宽度 1-24（0=整行）                                                                                                                              |
| `autoRun`          | bool                    | 进入页面自动执行（查询类区块）                                                                                                                       |
| `display`          | string                  | `inline`（默认，栅格内）/ `dialog`（弹窗，不占栅格）                                                                                                 |
| `group`            | string                  | 弹窗分组：`display=dialog` 且 `group` 相同的区块渲染进**同一弹窗**（表单+字段卡+表格混排）；按钮/行操作的动作目标指向 `group`                        |
| `refreshOn`        | []string                | 依赖的 stateKey（=上游区块 key）列表——任一变化自动重跑（page_state 联动：上游输出顶层字段同名合并进下游输入）                                        |
| `onSuccessRefresh` | []string                | 操作成功后自动重跑的区块 key（发邮件成功→刷新玩家表格）                                                                                              |
| `events`           | []CompositeEventBinding | **通用事件绑定**（全组件事件发布触发点）：`rowClick`/`rowSelected`（table）、`success`/`error`（form）、`click`（fields）→ 动作步骤（6 种 kind）+ 链 |
| `table`            | CompositeTableSpec      | `view=table`：columns/pagination/rowSchema/identityKey/**rowActions**                                                                                |
| `toolbar`          | CompositeToolbarSpec    | 表格顶部按钮组（actions）                                                                                                                            |

**行操作与按钮动作**（rowActions / toolbar.actions）：

| 字段            | 语义                                                                        |
| --------------- | --------------------------------------------------------------------------- |
| `label`         | 按钮文案（LocalizedText）                                                   |
| `targetSection` | 打开的弹窗目标：区块 key 或 **group 名**（空=纯动作链按钮）                 |
| `params`        | 参数映射：行操作=行字段名→表单参数名（`player_id: uid`）；顶部按钮=静态初值 |
| `danger`        | 危险样式 + 二次确认                                                         |
| `chain`         | **动作链**：主动作后按序执行的步骤 `[{kind: runBinding                      | refreshNode, target: 区块key}]` |

编辑器（`web/src/pages/PageStudio/CompositeEditor`）与发布渲染器（`PageRenderer` 的 `CompositeRenderer`）共用此模型；编译器（编辑器 → sections）与反编译器（sections → 编辑树，用于回读再编辑）保证配置 round-trip 不丢失。

## 前端运行时：ProComponents 页面渲染器

页面运行时固定使用 Ant Design Pro/ProComponents；PageSpec 节点与运行时组件的对应关系见 [ProComponents 页面生成与运行时](./ui-generation.md)。

Renderer 只接受 PublishedPageSpec，并只通过 `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute` 执行。浏览器不得传 functionId、route、target、gameId 或 env 来选择执行目标。

PageSpec 必须与组件库解耦。未来更换表单或图表库时只替换 renderer adapter，不迁移 FunctionContract、PageSpec、菜单、发布快照或审计。

## 表单策略

JSON Schema 是函数输入/输出的持久化标准；表单展示由 `FormPresentationSpec` 表达，协议定义见 [PageSpec 协议规范](./pagespec-protocol.md)，渲染链路与唯一 runtime 约束见 [ProComponents 页面生成与运行时](./ui-generation.md)。

`FormPresentationSpec` 只负责表单展示，不改变 FunctionContract payload；保存和发布都必须经过服务端结构校验，校验失败必须报错并要求管理员修复。表单 runtime 固定为 `@rjsf/antd + @rjsf/validator-ajv8`，项目内禁止并行保留第二套表单运行时。

## Scope、菜单、发布与演进

页面身份固定为：

```text
PageIdentity = game_id + env + pageKey
```

Page Studio、Console、Proposal 和执行都从全局 scope 获取 `game_id + env`；页面内部不得再次选择或覆盖 scope。

动态菜单唯一来源不变：

```text
active PublishedPageSpec[] -> ConsoleMenuSpec -> ProLayout
```

分类与页面多语言文本来自 PublishedPageSpec 的 NavigationSpec，动态菜单项设置 `locale: false`，不使用静态 locale 或字典作为事实源。

发布时必须冻结：

- PageSpec 与 FormPresentationSpec 完整快照。
- 每个 binding 的函数版本、输入/输出 schema digest、风险、权限、执行模式、审批策略和语义 digest。
- Renderer ABI version 与 generator version。

函数或 CapabilitySemantics 变化后，Server 生成新的 Proposal 并计算 diff。已发布页标记 stale 且拒绝执行；Page Studio 必须提供“查看差异、自动合并安全字段、解决冲突、重新发布”。绝不静默更新 Draft 或 PublishedPageSpec。

自动合并的安全集只包含展示类字段：列顺序与显隐、字段 label/help、order、group、widget hint、导航标题、分类 labels、图标和排序。`visibleWhen` 只有经校验证明不影响 required 输入、binding payload 和 selector 引用时才允许自动合并，否则归入冲突集。执行类字段——bindings、functionId、input/output assignment、confirmation、permissions、risk、approval——出现任何差异都必须人工确认，不得自动合并。

## 模型边界

- SDK/OpenAPI 只能提交 FunctionContract 与受控 capability 语义。
- Resource Catalog 只能补充 CapabilitySemantics，不能编辑页面 UI。
- PageProposal/PageSpec 是页面生成、编辑、发布和动态菜单的唯一来源。
- Renderer 只能消费 PublishedPageSpec，不能从最新函数目录或运行结果临时补字段。
- 动态菜单文本来自 PublishedPageSpec 的 NavigationSpec，不能依赖静态 locale 或字典事实源。
- 浏览器只能通过 published binding execute API 执行，不能选择 function、target、route 或 scope。
- 历史页面配置无自动迁移路径。旧页面配置模型的数据只能导出、备份和人工重建；不提供自动转换桥。

## 完成定义

满足以下条件后才可验收发布（真实浏览器 E2E 回归入口见 `web/e2e/` 与 [真实 Dashboard E2E](../development/real-dashboard-e2e.md)）：

1. 一个 OpenAPI REST Resource 可自动生成并直接发布 ResourcePage；声明写 capability 时提供完整 CRUD，未声明写 capability 时提供只读查询/详情页面。
2. 一个 SDK 显式能力 Resource 可自动生成并直接发布同等 ResourcePage。
3. 未提供 CRUD 语义的函数可自动生成并直接发布安全 Operation Page。
4. Task 和 Report 页面使用真实任务/图表数据，不包含“最小实现”或 JSON 占位。
5. Page Studio 的常用路径不要求编辑原始 PageSpec JSON 或自定义 mapping。
6. 发布、动态菜单、scope、权限、审批、审计、OTel 和函数变更 stale 在真实浏览器 E2E 中闭环。

旧模型的删除记录与防回流证据见 [旧模型删除清单](./legacy-deletion-inventory.md)。
