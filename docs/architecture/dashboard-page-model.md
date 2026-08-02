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

> **状态**：Target -- 本文定义下一版 Dashboard 的权威模型。实现、文档和 SDK 必须以本文的正向模型为准；不符合本文边界的旧实现按根目录 `todo.md` 物理清理。

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

| 层 | 输入 | 产物 | 是否可由 SDK/OpenAPI 提供 |
| --- | --- | --- | --- |
| 数据契约 | JSON Schema、HTTP method/path、函数版本和治理字段 | FunctionContract | 可以 |
| 能力语义 | CRUD 意图、对象标识、任务/报表执行特征 | CapabilitySemantics | 可从 REST 推导；SDK 可显式提供有限语义 |
| 页面编排 | 分类、标题、列、筛选、动作位置、映射和表单展示 | PageProposal / PageSpec | 不可以；只由 Server/Page Studio 产生 |

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
  execution: 'sync' | 'task' | 'approval';
  resourceKey?: string;
  operationKey?: string;
  capability?: CapabilityKind;
}

type CapabilityKind =
  | 'collection_query'
  | 'item_query'
  | 'create'
  | 'update'
  | 'delete'
  | 'action'
  | 'task'
  | 'report';
```

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
  lifecycle: Partial<Record<'create' | 'update' | 'delete', FunctionRef>>;
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

type JsonScalarType = 'string' | 'number' | 'integer' | 'boolean';

interface CollectionSemantic {
  query: FunctionRef;
  itemsPath: JsonPointer; // collection query 输出中项目数组的位置
  itemSchemaDigest: string;
  pagination?: OffsetPaginationSemantic | CursorPaginationSemantic;
}

interface OffsetPaginationSemantic {
  kind: 'offset';
  request: { offset: JsonPointer; limit: JsonPointer };
  response: { total?: JsonPointer; hasMore?: JsonPointer };
}

interface CursorPaginationSemantic {
  kind: 'cursor';
  request: { cursor: JsonPointer; limit?: JsonPointer };
  response: { nextCursor: JsonPointer; previousCursor?: JsonPointer; hasMore?: JsonPointer };
}

interface ActionSemantic {
  function: FunctionRef;
  subject: 'resource_item' | 'resource_selection' | 'none';
  identityInput?: JsonPointer;
}

interface TaskSemantic {
  start: FunctionRef;
  taskId: { resultPath: JsonPointer; valueType: JsonScalarType };
  status: { function: FunctionRef; taskIdInput: JsonPointer; statePath: JsonPointer };
  events?: { function: FunctionRef; taskIdInput: JsonPointer; eventsPath: JsonPointer };
  result?: { function: FunctionRef; taskIdInput: JsonPointer; resultPath: JsonPointer };
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
  source: 'openapi_rest' | 'sdk_explicit' | 'platform_review';
  sourceDigest: string;
  confidence: 'high' | 'low';
  status: 'effective' | 'overridden' | 'conflict';
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
  quality: 'ready' | 'basic' | 'needs_review';
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
  repairHint: string;
}
```

`proposalKey` 是生成器幂等身份：一个 ResourceCapability 只能有 `resource:<resourceKey>`，每个独立 Operation/Task/Report 函数分别有 `<kind>:<functionId>`。`pageKey` 固定为 `resource--<resourceKey>` 或 `<kind>--<functionId>`，其中 source key 必须符合 `[a-z0-9][a-z0-9._-]*`；它是可读的路由与发布身份，不得从 summary、labels 或本次生成结果随机生成。分类建议的默认规则以 [ProComponents 页面生成与运行时](./ui-generation.md) 为唯一出处；不得从带 kind 前缀的 pageKey 推断分类。Resource action 只有在 `subject` 可验证时才并入唯一 ResourcePage；否则保留为函数自己的 OperationPage，避免重复菜单或覆盖资源页。

- `ready`：完整且可验证的 Resource CRUD、Task 或 Report 页面，可直接接受并发布。
- `basic`：安全的 Operation Page，含输入表单、确认、受控执行和结果区，可直接接受并发布。
- `needs_review`：语义或映射不完整，必须在 Page Studio 决策后发布。
- 函数不可执行、权限/风险不可校验、schema 无效或 binding 不安全时，生成 BlockedProposalIssue，禁止物化和发布；`blocked` 不是 Proposal quality。

PageDraft 是用户接受 Proposal 后形成的可编辑页面；PublishedPageSpec 是包含完整 PageSpec、binding snapshot、表单展示快照和 renderer version 的不可变运行产物。Proposal 重新生成绝不覆盖 Draft 或 PublishedPageSpec。

## PageSpec：业务级页面协议

PageSpec 是平台唯一的页面编排协议。它不持久化 `ProTable`、`ProForm` 等具体组件名，而是强类型的业务级 DSL：

```ts
type PageSpec = ResourcePageSpec | OperationPageSpec | TaskPageSpec | ReportPageSpec;

interface PageBase {
  pageKey: string;
  scope: Scope;
  navigation: NavigationSpec;
  bindings: PageBinding[];
  permissions: PagePermissionSpec;
}

interface ResourcePageSpec extends PageBase {
  kind: 'resource';
  resourceKey: string;
  list?: ListViewSpec;
  detail?: DetailViewSpec;
  create?: FormActionSpec;
  update?: FormActionSpec;
  delete?: ConfirmActionSpec;
  rowActions: ResourceActionSpec[];
  toolbarActions: ResourceActionSpec[];
}

interface OperationPageSpec extends PageBase {
  kind: 'operation';
  action: FormActionSpec | ConfirmActionSpec;
  result: ResultViewSpec;
}

interface TaskPageSpec extends PageBase {
  kind: 'task';
  start: FormActionSpec;
  task: TaskViewSpec;
  result?: ResultViewSpec;
}

interface ReportPageSpec extends PageBase {
  kind: 'report';
  query: QueryViewSpec;
  visualizations: ReportViewSpec[];
}
```

`PageBinding` 只引用发布期允许执行的 FunctionContract。输入输出映射必须使用受控的 typed selector AST，禁止保存无约束 JSON mapping、裸整行透传或运行时猜路径；AST 定义与校验规则见 [UI Schema 与 PageSpec 规范](./ui-schema-spec.md)。

`NavigationSpec` 承载 `category.key`、`category.labels`、`title`、排序和图标。它只在 PageProposal/PageSpec 中确定，注册侧不能提供菜单事实。

## CRUD 是主路径，非 CRUD 是一等扩展

### ResourcePage

当 ResourceCapability 有可验证的 `collection_query` 与对象 identity 时，生成 ResourcePage；生命周期写操作是可选能力。只读资源也必须生成查询/列表/详情页面，不能因为缺少 create/update/delete 被错误降级为 OperationPage。语义到页面节点与组件的生成模板见 [ProComponents 页面生成与运行时](./ui-generation.md)。

JSON Schema 为列表列、详情项和表单字段生成候选。`CapabilitySemantics` 解决分页、identity、集合与对象响应；PageProposal 决定列、默认排序、动作位置和展示文本。管理员可覆盖 Proposal，但不能绕过类型和发布校验。

### OperationPage

无法可靠归入资源生命周期的同步函数生成 OperationPage，例如 `mail.send`、`cache.refresh`、`broadcast.send`。它的默认形态是输入表单、风险确认、受控执行和结构化结果，不应被塞入 ResourcePage。

### TaskPage 与 ReportPage

异步函数生成 TaskPage；报告语义生成 ReportPage。TaskPage 必须接入真实 task 状态、事件、取消/重试和结果，不得只显示 taskId。ReportPage 必须使用已验证的数据集、指标和图表字段，不得只显示 JSON。

## 前端运行时：ProComponents 页面渲染器

页面运行时固定使用 Ant Design Pro/ProComponents；PageSpec 节点与运行时组件的对应关系见 [ProComponents 页面生成与运行时](./ui-generation.md)。

Renderer 只接受 PublishedPageSpec，并只通过 `POST /console/pages/:pageKey/bindings/:bindingId/execute` 执行。浏览器不得传 functionId、route、target、gameId 或 env 来选择执行目标。

PageSpec 必须与组件库解耦。未来更换表单或图表库时只替换 renderer adapter，不迁移 FunctionContract、PageSpec、菜单、发布快照或审计。

## 表单策略

JSON Schema 是函数输入/输出的持久化标准；表单展示由 `FormPresentationSpec` 表达，协议定义见 [UI Schema 与 PageSpec 规范](./ui-schema-spec.md)，渲染链路与唯一 runtime 约束见 [ProComponents 页面生成与运行时](./ui-generation.md)。

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
- 每个 binding 的函数版本、输入/输出 schema digest、风险、权限、执行模式和语义 digest。
- Renderer ABI version 与 generator version。

函数或 CapabilitySemantics 变化后，Server 生成新的 Proposal 并计算 diff。已发布页标记 stale 且拒绝执行；Page Studio 必须提供“查看差异、自动合并安全字段、解决冲突、重新发布”。绝不静默更新 Draft 或 PublishedPageSpec。

自动合并的安全集只包含展示类字段：列顺序与显隐、字段 label/help、order、group、widget hint、导航标题、分类 labels、图标和排序。`visibleWhen` 只有经校验证明不影响 required 输入、binding payload 和 selector 引用时才允许自动合并，否则归入冲突集。执行类字段——bindings、functionId、input/output assignment、confirmation、permissions、risk——出现任何差异都必须人工确认，不得自动合并。

## 模型边界

- SDK/OpenAPI 只能提交 FunctionContract 与受控 capability 语义。
- Resource Catalog 只能补充 CapabilitySemantics，不能编辑页面 UI。
- PageProposal/PageSpec 是页面生成、编辑、发布和动态菜单的唯一来源。
- Renderer 只能消费 PublishedPageSpec，不能从最新函数目录或运行结果临时补字段。
- 动态菜单文本来自 PublishedPageSpec 的 NavigationSpec，不能依赖静态 locale 或字典事实源。
- 浏览器只能通过 published binding execute API 执行，不能选择 function、target、route 或 scope。
- 历史页面配置无自动迁移路径。旧 WorkspaceConfig、objectKey、layout 等模型的数据只能导出、备份和人工重建；不提供自动转换桥。

## 完成定义

下一版只有满足以下条件才可宣称可发布：

1. 一个 OpenAPI REST Resource 可自动生成并直接发布完整 CRUD 页面。
2. 一个 SDK 显式能力 Resource 可自动生成并直接发布 CRUD 页面。
3. 未提供 CRUD 语义的函数可自动生成并直接发布安全 Operation Page。
4. Task 和 Report 页面使用真实任务/图表数据，不包含“最小实现”或 JSON 占位。
5. Page Studio 的常用路径不要求编辑原始 PageSpec JSON 或自定义 mapping。
6. 发布、动态菜单、scope、权限、审批、审计、OTel 和函数变更 stale 在真实浏览器 E2E 中闭环。

详细迁移顺序、删除清单、验收场景和交接责任见根目录 `todo.md`。
