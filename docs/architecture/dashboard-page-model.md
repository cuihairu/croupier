---
title: The Dashboard resource/page model
icon: table-cells-large
order: 5
category:
  - System architecture
tag:
  - Dashboard
  - ProComponents
  - Function registration
  - Dynamic menu
---

# The Dashboard resource/page model

> **Status**: In progress -- this document is the authoritative definition of the Dashboard page model. Implementation, documentation, and SDKs follow this document's forward model; the old model is cleaned up per the old-model deletion list and guarded against re-entry by `scripts/dashboard_vnext_guard.sh`; for real-browser regressions, the unfinished items in the root `todo.md` remain the source of truth.

## Decisions

Croupier keeps **React + Umi + Ant Design Pro + ProComponents**, does not integrate React Admin, and does not adopt React Admin's CRUD `DataProvider` as a platform protocol.

ProComponents already provides the mature component capabilities a back office needs; what Croupier lacks is a domain model that turns function capabilities into pages reliably — not another component library.

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

This model satisfies two facts at once:

- Game back offices carry large numbers of players, orders, configs, mail templates, events, and similar **CRUD resources**; CRUD is the main path of default page generation, not a shunned legacy concept.
- Game back offices also carry bans, compensation, bulk granting, tasks, approvals, reports, ops commands, and other **non-CRUD capabilities**; they must be first-class page types, never forced into CRUD.

## Product goals

The operator's complete path must be:

```text
Register a function or import OpenAPI
  -> The Server parses and persists capabilities, semantics, and diagnostics
  -> A traceable default PageProposal is generated
  -> The user accepts and publishes directly, or adjusts it in Page Studio
  -> The Console builds the left menu from the menu tree (menu_items) + the mounted published pages
  -> Pages execute only through published bindings
```

Requirements:

1. Service developers submit only function capability contracts — never pages, menus, table columns, button positions, or frontend components.
2. As long as the capability is sufficient to execute safely, the platform must generate a directly publishable default Operation Page; the user never starts from blank JSON.
3. For a Resource with standard CRUD semantics, the platform must generate usable list, detail, create, edit, delete, and resource-action pages.
4. Published pages, menus, permissions, audit, and traces must be reproducible; a function change must never silently alter a published UI.
5. Normal users edit business concepts — "resources, lists, columns, forms, actions, categories"; JSON is reserved for import/export and controlled diagnostics, never the main workflow.

## Why JSON Schema alone cannot generate complete pages

An input JSON Schema can generate form fields reliably, and an output JSON Schema can generate candidate columns and detail fields reliably; they describe only data shapes, not business usage.

For example, `player.ban`'s `playerId` may come from manual input, a selected list row, or the detail context; an array output may be a table, a dropdown's options, or a chart series. The platform must not disguise these choices as JSON Schema inferences.

Automatic generation therefore splits into two layers:

| Layer                | Inputs                                                                              | Outputs                 | Providable by SDK/OpenAPI?                                        |
| -------------------- | ----------------------------------------------------------------------------------- | ----------------------- | ----------------------------------------------------------------- |
| Data contract        | JSON Schemas, HTTP method/path, function version, and governance fields             | FunctionContract        | Yes                                                               |
| Capability semantics | CRUD intent, object identity, task/report execution traits                          | CapabilitySemantics     | Derivable from REST; SDKs may supply limited semantics explicitly |
| Page orchestration   | Categories, titles, columns, filters, action placement, mappings, form presentation | PageProposal / PageSpec | No; produced only by the Server/Page Studio                       |

`CapabilitySemantics` is not UI. It contains no menus, routes, titles, table columns, or button positions; it answers only "what does this function do in the resource lifecycle".

## The three-layer model

### FunctionContract

FunctionContract is each function's executable capability and governance contract:

```ts
interface FunctionContract {
  id: string;
  version: string;
  enabled: boolean;
  summary?: LocalizedText;
  description?: LocalizedText;
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;
  previousInputSchema?: JSONSchema; // 上一次注册的 inputSchema（prev 列，只存一版）
  previousOutputSchema?: JSONSchema; // 上一次注册的 outputSchema
  risk: RiskLevel;
  permission?: string;
  execution: "sync" | "task";
  executionState: "bound" | "unbound"; // 执行状态（D2）：契约存在与运行时绑定正交
  approval: ApprovalPolicy;
  resourceKey?: string;
  operationKey?: string;
  capability?: CapabilityKind;
}
```

`executionState` (the database `execution_state` column, default `bound`) expresses whether the contract already has an executable
runtime function: **an OpenAPI upload generates `unbound` contract material directly** (upload-to-page; no prior registration required);
when an agent/SDK registers a function with the same functionId in the same scope at runtime, it flips to `bound` automatically (T6); a
differently named function can be bound manually from the editor's binding drawer (T9). An `unbound` contract enters the template and
proposal pipelines normally and can be saved and published; only execution is blocked: binding execute returns `409 executor_unbound` (T8).
The field does not participate in contract digest/stale determination (locked by tests); existing rows are all `bound` after migration,
behaviorally identical to the old model.

`previousInputSchema`/`previousOutputSchema` (the database `prev_input_schema`/`prev_output_schema` columns) keep the previous version of the schema from before this registration, **one version only** (full history in FunctionContractVersion below):

- Write timing: when the registration path finds an existing row, it copies that row's schema into the prev columns; a re-registration whose schema is unchanged is skipped by the upsert's semantic-equality check, so prev is never refreshed pointlessly.
- Consumers: one-click selector sync (sync-selectors) uses the prev→new field diff for precise rename inference; freshness's selector-level stale diagnostics also consume it to produce rename candidate hints.
- Exactness determination: when prev matches the published page snapshot's schema digest on both sides (freshness's dual-algorithm digestMatch), rename candidates are marked `confidence=high`; a missing digest (old snapshots) or multi-hop drift (contract changed again after publishing) always degrades to heuristic `low`.
- Existing rows without prev (contracts registered before the feature shipped): the first sync runs heuristically; contracts revived after soft delete carry no prev.

### FunctionContractVersion (B2: the contract change history)

FunctionContract has exactly one row per function; the `function_contract_versions` table (goose 0029) records content changes as an append-only snapshot stream keyed by `(game_id, env, function_id)`:

```ts
interface FunctionContractVersion {
  seq: number; // 函数内单调递增（max+1 计算，无唯一索引——并发允许并列，按 seq,id 排序）
  version?: string;
  source?: string; // sdk|openapi|catalog
  sourceDigest?: string;
  changeType: "created" | "updated" | "removed";
  breaking: boolean; // input/output schema 破坏性变更（schemadiff）
  actor?: string; // system|用户名（注册路径恒 system）
  snapshot: FunctionSpec; // 变更后投影；removed 为删除前最后一版
  diff?: Array<{
    field: string;
    from?: string;
    to?: string;
    change?: string;
    findings?: SchemaFinding[];
  }>;
  createdAt: string;
}
```

- **The write criterion is exactly the same as `UpsertContract`'s "skip the write when content is unchanged"** (the same `contractSemanticallyEqual`): heartbeat-style re-registrations produce no history; `RemoveFunctionContract` writes a `removed` event.
- **Retention policy (decided with the user)**: a per-function cap of 50 entries (`FunctionContractVersionRetention`), evicting oldest beyond that.
- **Failure semantics**: history is derived audit data (same tier as proposal/template rebuilds); a write failure degrades to a warning and never blocks registration; the table's existence is guaranteed at startup by `MinimumRequiredVersion=42`.
- **Consumers**: the function detail page's "change history" tab (list / snapshot Drawer / two-version compare); REST in [the function API §22-24](https://cuihairu.github.io/croupier/zh/api/function).
- The history table never enters the contract digest nor stale determination; the contract row remains the sole authoritative source of truth.

```ts
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

`execution` and `approval` are orthogonal: `execution: 'task'` with `approval.required: true` means the async task starts only after approval; synchronous operations may require approval too. `approval.policyKey` references only a governance policy configured on the platform; when missing, the Server resolves a default per the risk policy. It is not page UI and cannot be overridden by the browser.

`resourceKey`, `operationKey`, and `capability` are all business capability semantics, not page semantics. `capability` allows only the governed enum above; page design is expressed only by PageProposal/PageSpec.

A function lacking `resourceKey` or `capability` must not be blocked from registration; it still generates a standalone Operation Page. An arbitrary SDK function has no REST method/path, so to join Resource CRUD pages reliably it must supply this one governed capability semantic — the Server must not be asked to guess from the function name.

### ResourceCapability and CapabilitySemantics

ResourceCapability associates the functions of one resource; CapabilitySemantics is its verifiable, versioned semantic result.

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

`IdentitySemantic.itemPath` must exist uniquely in the collection item's or the item query's output schema; the identity inputs of item/update/delete/action are explicitly mapped and validated by typed selectors — the collection query's input is never required to contain the identity. `CollectionSemantic.pagination` must declare both the request parameters' and the response metadata's JSON Pointers; offset pagination must supply at least `total` or `hasMore`, and cursor pagination must supply `nextCursor`; when missing, only a list without pagination controls is generated — the offset/cursor protocol is never guessed.

`ActionSemantic.subject` is the business context a resource operation needs, not a button position: `resource_item` maps to a row action, `resource_selection` to a batch action, `none` to a resource toolbar action; when subject or identity input cannot be determined safely, only a standalone OperationPage is generated. Every pointer in `TaskSemantic` and `ReportSemantic` must be validatable against the corresponding FunctionContract schema, otherwise the result can only be `needs_review`.

The acceptable source priority:

1. Standard OpenAPI REST shapes, such as `GET /players`, `POST /players`, `GET/PATCH/DELETE /players/{id}`.
2. The SDK descriptor's governed `resourceKey + capability` semantics.
3. Semantic supplements saved by a platform administrator in the Resource Catalog; independent of function registration, requiring versioning, audit, and permission control.

Guessing object IDs, pagination fields, action placement, or page types from names like `player.list` or `player.ban` is not allowed. Deterministic REST shapes plus JSON Schema fields may produce "high-confidence suggestions", but low-confidence suggestions must never auto-publish.

Source conflicts are adjudicated per field: `platform_review` is the human final ruling with the highest priority; `sdk_explicit` outranks `openapi_rest` derivation. Every effective value and every overridden value must be recorded in `SemanticProvenance` — a single `source` field must not mask multiple origins. Unresolved conflicts keep their diagnostics, the affected Proposal degrades to `needs_review` and is barred from publishing; the conflict clears only after an administrator makes an explicit versioned `platform_review` choice. A Resource Catalog override must save a version, write audit, and trigger recomputation of the affected ResourceCapability and PageProposals when it takes effect.

Judging `action` versus `update`: a lifecycle operation that idempotently modifies the resource's own fields is `update`; one that triggers resource-related side effects or workflows (ban, compensate, reset, grant) is `action`. For example `player.ban` is an `action` on the `player` resource, generating a row action rather than an edit form.

### PageProposal, PageDraft, and PublishedPageSpec

A PageProposal is a regenerable default page suggestion — not a draft and not a runtime page:

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

// 不可物化时问题不在 Proposal：只保存诊断与修复指引，不携带 spec。
interface BlockedProposalIssue {
  id: string;
  scope: Scope;
  sourceDigests: SourceDigest[];
  diagnostics: Diagnostic[];
  repairHint: LocalizedText;
  status: "open" | "resolved" | "dismissed";
}
```

`proposalKey` is the generator's idempotent identity: one ResourceCapability has exactly `resource:<resourceKey>`, and each standalone Operation/Task/Report function has `<kind>:<functionId>`. `pageKey` is fixed to `resource--<resourceKey>` or `<kind>--<functionId>`, where the source key must match `[a-z0-9][a-z0-9._-]*`; it is the human-readable routing and publishing identity and must never be generated randomly from summaries, labels, or the generation run. The default rules for category suggestions have their sole source in [ProComponents page generation and runtime](https://cuihairu.github.io/croupier/zh/architecture/ui-generation); categories are never inferred from a kind-prefixed pageKey. A resource action merges into the single ResourcePage only when its `subject` is verifiable; otherwise it stays as the function's own OperationPage, avoiding duplicate menus or clobbering the resource page.

- `ready`: every declared capability of the page has verifiable bindings, selectors, governance, and renderer support, and it can be accepted and published directly. A ResourcePage's write capabilities are optional, so a read-only ResourcePage can also be `ready`.
- `basic`: a safe synchronous OperationPage with an input form, confirmation, governed execution, and a result area; it may require approval and shows the real result after approval; it can be accepted and published directly.
- `needs_review`: semantics or mappings are incomplete; it must be published only after decisions in Page Studio.
- When a function is not executable, permission/risk is unverifiable, a schema is invalid, or a binding is unsafe, a BlockedProposalIssue is generated and materialization/publishing is barred; `blocked` is not a Proposal quality.

A PageDraft is the editable page formed after the user accepts a Proposal; a PublishedPageSpec is the immutable runtime artifact containing the complete PageSpec, the binding snapshot, the form presentation snapshot, and the renderer version. Proposal regeneration never overwrites a Draft or a PublishedPageSpec.

### ComponentTemplate (the component template layer, V4)

Component templates are the **reuse middle layer** among the three layers: several functions (together with their PageNode subtree orchestration) are packaged as a component that can be instantiated wholesale, and components compose into pages.

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
  digest: string; // 内容指纹 sha256(canonical Tree JSON)，U11 更新提醒
}
```

Generation and maintenance:

- **Contract → automatic template generation**: after an agent registers functions, the generator aggregates by ResourceCapability
  and produces builtin templates (e.g. `player.crud` = the CRUD subtree of the list/create/update/delete four functions);
  the entry point is `POST /api/v1/component-templates/regenerate` (manual trigger; agent registration does not auto-regenerate)
- **User saves as a component**: in the composite page editor, select 1..N nodes → the toolbar's "save as component" →
  the PageNode subtree is serialized and stored as a `builtin=false` template
- **Instantiation**: clicking a template in the component library panel → the tree is copied + ids reassigned + function references remapped onto the canvas;
  the availability check runs locally in the frontend (`requiredFunctions` compared against the scope's function set; missing functions gray out)
- **Digest and update reminders (U11)**: `digest` = sha256(canonical Tree JSON), covered on all three write paths
  (Create / Update handler + the `UpsertBuiltin` reached by regenerate). Instantiation is
  **copy semantics** — the page saves a copy of the content as of that moment; the editor snapshots the used templates' `{key, digest}`
  into `PageSpec.componentTemplates` (a page-level optional field, passed through as JSON across proposal→draft→published,
  not participating in publish validation); when the page is opened again it compares against the template library's current digests and, on mismatch, prompts
  "a used template has a newer version" (a prompt only, never an auto-sync)
- **UpsertBuiltin content gating (M3)**: `UpsertBuiltin` compares a builtin row on all content fields
  (Name/Description/Category/Icon/RequiredFunctions/Tree; JSON columns compared after normalization, falling back to byte comparison on parse failure — better a spurious write than a spurious skip), and skips the write entirely when content is identical
  (not touching `updated_at`). The gating's value: the full template rebuild triggered by per-function registration no longer
  rewrites unchanged templates over and over. Builtin rows only — a custom row occupying the key (`builtin=false`) keeps the overwrite path,
  and the Builtin marker never flips; manual modifications retain force-restore semantics (different content always writes)

REST: `/api/v1/component-templates` (List/Get/Create/Update/Delete/Regenerate);
the wire contract is in [the API docs](https://cuihairu.github.io/croupier/zh/api/component-templates). The usage-layer docs are in
[Composite page editor V4](https://cuihairu.github.io/croupier/zh/dashboard/composite-editor-v4-design).

## PageSpec: the business-level page protocol

PageSpec is the platform's sole page orchestration protocol. It does not persist concrete component names like `ProTable` or `ProForm`; it is a strongly typed business-level DSL:

```text
PageSpec = (pageKey, type, resourceKey?, category, title, icon, order,
            navigation?, resource? | operation? | task? | report?, bindings[])
```

The view orchestration of the four page types:

| Page type   | View nodes (the actual DTOs)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `resource`  | `ListViewSpec`（columns/filters/pagination/rowActions/batchActions/toolbarActions）、`DetailViewSpec`（fields/actions）、`CreateForm`/`UpdateForm`（FormPresentationSpec）、`DeleteAction`（ConfirmActionSpec）                                                                                                                                                                                                                                                                                                                                 |
| `operation` | `Form`（FormPresentationSpec）+ `Confirm`（ConfirmActionSpec）+ `ResultViewSpec`                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `task`      | `Form`（FormPresentationSpec）+ `TaskViewSpec`（status/events/result/cancel 的 bindingId 引用）+ `ResultViewSpec`                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `report`    | `QueryForm`（FormPresentationSpec）+ `DatasetSpec` + `ChartSpec[]` + 表格 `ListViewSpec`                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `composite` | `CompositePageSpec`：`sections[]`（每区块绑定一个函数；`display` inline/dialog/tab/card、`group` 弹窗/页签/卡片分组、`tab` 页签标签、`cardTitle` 卡片标题、`rowActions`/`toolbar` 按钮动作含 `chain` 动作链、`onSuccessRefresh`、`refreshOn` page_state 联动（`cascadePolicy` 级联失败策略），另有 `static` 常量表单（不绑定函数，值进 page_state；sections 顺序=请求输入顺序，static 与函数区块按输入位置交错，不重排到末尾）；页面级可选 `componentTemplates[]`（U11 模板使用快照 `{key, digest}`，仅编辑器更新提醒用，不参与发布校验与渲染） |

Field-level wire contracts (including FormPresentationSpec, the Selector AST, the binding usage enum, and ABI versions) have their sole source in the [PageSpec protocol specification](./pagespec-protocol.md); the authoritative implementations are `internal/dashboard/spec` (Go DTOs) and `web/src/types/dashboard.ts` (frontend shared types), corresponding item by item.

A `PageBinding` may reference only FunctionContracts allowed to execute at publish time. Input/output mappings must use the governed typed selector AST; saving unconstrained JSON mappings, raw whole-row passthroughs, or runtime path guessing is forbidden.

Category, title, icon, and order are PageSpec's top-level strongly typed fields; `NavigationSpec` carries only breadcrumb and back behavior (breadcrumb, showBack, backPath). They are settled only in PageProposal/PageSpec; the registration side can never supply menu facts. A page has no standalone permissions field: permissions are carried by binding-level governance (the contract's permission/risk/approval snapshot) and the action-level permission field. **From T-M8 on, category names are decoupled from page specs**: `PageCategorySpec` keeps only `key` (the grouping locator key) and `order`; multilingual category names come uniformly from the menu system (`menu_items.labels`); existing data is migrated by the script (`scripts/migrate-categories-to-menus.sql`), and page-driven menus fall back to an empty category title that the frontend overrides from `menu_items.labels`.

**The localized-name contract (relaxed in T12; from T-M8, category.labels partially moves to menus)**: the `title` LocalizedText requires only **any one locale to have a non-empty value** (the default name is required, translations optional) — zh-CN is the first preferred display language (first in the render fallback chain), en-US and all other languages are optional, and legacy pages carrying only en-US are not wrongly rejected; all-blank values are rejected by save/publish validation and publish-time diagnostics (`hasDefaultLocale`). The generator's every path normalizes through `ensureDefaultLocale`: blanks are stripped, and when zh-CN is missing any existing value fills in — no forced en-US write anymore. The editor's (LocalizedTextEditor) required baseline has been downgraded in step to the default language only: a missing zh-CN shows a non-blocking remediation hint in the 🌐 popover, while missing other languages neither warns nor blocks; the dropdown marker returns to a single ✓ (recorded languages), and the required ⚠ marker and `contractHint`'s forced dual-write wording were removed with T13; the category title editing entry was removed from the page editor with T-M8 (maintained on the menu management page); since 2026-09 the `category.key` input was removed as well — the sole entry point for navigation attribution is mounting a menu (menu_items + the page's `menuId`), and `category` rides along only as a protocol field on existing specs, with no editing offered. The other LocalizedText fields (confirm copy, result hints, field labels, etc.) remain optional, with the render side falling back zh-CN → en-US → any non-empty value.

## CRUD is the main path; non-CRUD is a first-class extension

### ResourcePage

When a ResourceCapability has a verifiable `collection_query` and object identity, a ResourcePage is generated; lifecycle write operations are an optional capability. Read-only resources must still get query/list/detail pages — never wrongly downgraded to OperationPages for lacking create/update/delete. For the generation templates from semantics to page nodes and components, see [ProComponents page generation and runtime](https://cuihairu.github.io/croupier/zh/architecture/ui-generation).

JSON Schema generates candidates for list columns, detail items, and form fields. `CapabilitySemantics` resolves pagination, identity, and collection versus object responses; the PageProposal decides columns, default sorting, action placement, and display text. Administrators may override a Proposal but cannot bypass type and publish validation.

### OperationPage

Synchronous functions that cannot be reliably placed into a resource lifecycle generate OperationPages, such as `mail.send`, `cache.refresh`, `broadcast.send`. Their default shape is an input form, a risk confirmation, governed execution, and a structured result; they must not be crammed into a ResourcePage.

### TaskPage and ReportPage

Async functions generate TaskPages; report semantics generate ReportPages. A TaskPage must integrate real task status, events, cancel/retry, and results — never a bare taskId.

A TaskPage's lifecycle capabilities come from `TaskSemantic`: the generator turns `start/status/events/result/cancel` into PageSpec bindings, and the `TaskViewSpec` stores the bindingId references, a fixed `taskId` page state key, and `status.statePath`. At runtime everything still goes through `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute`; the browser submits only `page_state.taskId` as a selector source and never passes functionId, target, gameId, or env. A TaskPage lacking the `status` binding or `statusStatePath` cannot be published; the `events/result/cancel` entries appear only when the corresponding real function semantics exist; retry may not be published before a real runtime closed loop exists.

A ReportPage must use verified datasets, metrics, and chart fields — never a bare JSON dump.

### CompositePage (the free-form composite page)

A composite page orchestrates several function blocks into one workbench page (e.g. "player management" = the player table + a row-action modal + submit-and-refresh). Sections are a **flat list**; the editor (composite page editor V3) works on a component tree internally and compiles it into flat sections on save — the publish chain (proposals/validation/versions/menus) has zero special cases.

The `CompositeSection` field model (authoritative implementation `internal/dashboard/spec/types.go`, frontend `web/src/types/dashboard.ts`):

| Field              | Type                    | Semantics                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ------------------ | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `key`              | string                  | The section's unique identifier. **Multiple instances of the same function** number as `fid`, `fid-2`, `fid-3` (one data source may be dragged in as several separately configured components); the editor supports declaring a fixed `sectionKey` (pinned on read-back, immune to add/remove/reorder drift; see composite page editor V4's "section keys"); the creation endpoint errors explicitly on duplicate keys                                                                                                                                                                                                                                                                                                                                              |
| `bindingId`        | string                  | References a binding in `PageSpec.bindings`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `view`             | string                  | `table` / `fields` / `form`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `span`             | int                     | Grid width 1-24 (0 = full row)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `autoRun`          | bool                    | Execute automatically on page entry (query-type sections)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `display`          | string                  | `inline` (default, inside the grid) / `dialog` (a modal, occupies no grid) / `tab` (aggregated by the renderer into Tabs) / `card` (aggregated by the renderer into a Card)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `group`            | string                  | The dialog/tab/card grouping: sections with `display=dialog` and the same `group` render into **one modal** (form + field cards + tables mixed); button/row-action targets point at the `group`; sections with `display=tab` and the same `group` render into **one Tabs** (the editor's tab container pins the group name as its `sectionKey`); sections with `display=card` and the same `group` render into **one Card** (the editor's card container pins the group name as its `sectionKey`)                                                                                                                                                                                                                                                                   |
| `tab`              | LocalizedText           | The tab label (effective when `display=tab`): within a group, sections aggregate by label onto the matching Tabs page, stacked full-width within the page; when absent the renderer falls back to "Tab N". On read-back it is restored into the tab container's `props.title`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `cardTitle`        | LocalizedText           | The card title (effective when `display=card`): the group title when same-group sections render into one Card (full-width), with the group's sections stacked vertically; falls back to the group name when absent. On read-back it is restored into the card container's `props.title`                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `refreshOn`        | []string                | The list of depended-on stateKeys (= upstream section keys) — any new upstream result re-runs this section automatically, driven by a per-section result revision (B2) so a same-state new upstream result also cascades (page_state linking: the upstream output's top-level same-named fields merge into the downstream input)                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `onSuccessRefresh` | []string                | Section keys re-run automatically after an operation succeeds (mail sent → refresh the player table)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `events`           | []CompositeEventBinding | **General event bindings** (published by every component's event triggers): `rowClick`/`rowSelected` (table), `success`/`error` (form), `click` (fields) → action steps (6 kinds) + chains                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `visibleWhen`      | *ConditionSpec          | **Section-level conditional display** (U10): evaluated against page state; an inline/tab section evaluating false does not render (execution unchanged — autoRun/refreshOn keep running). Leaf conditions require `key` (the source section key, addressing `/values/field`, `/data/field`, `/selectedRow/field`); nested `all`/`any` supported (depth ≤ 4). Dialog sections do not participate (modals are triggered explicitly by actions). Publish validation: a legal kind, path a JSON Pointer, key ∈ page sections, equals/notEquals carrying a value                                                                                                                                                                                                         |
| `cascadePolicy`    | string                  | **The refreshOn cascade failure policy** (U9): this section's behavior when any upstream dependency's latest result is a failure — `pause` (default: no re-run, data kept, a message.warning notice) / `keep` (no re-run, the last result silently retained) / `clear` (no re-run, this section's data emptied). None of the three re-runs. The cascade is driven by a per-section result revision (B2) — a same-state new upstream result (two consecutive successes with different data) re-runs the downstream section too, and a failure→success flip re-triggers it (a paused cascade resumes automatically once the upstream recovers). Publish validation requires a legal enum; the violation diagnostic code is `composite_section_cascade_policy_invalid` |
| `table`            | CompositeTableSpec      | `view=table`: columns/**pagination** (the resource page's `PaginationSpec`; `enabled` → server-side pagination — `page`/`pageSize` sent with a renderer-derived `offset=(page-1)*pageSize`, a response `total` used verbatim or, when absent, estimated as "a full page means a next page may exist" and closing off on a partial page; an upstream filter change resets to page 1 while a post-operation refresh keeps the page and filter; unset → client-side pagination) / rowSchema/identityKey/**rowActions**                                                                                                                                                                                                                                                 |
| `toolbar`          | CompositeToolbarSpec    | The table's top button group (actions)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |

**Row actions and button actions** (rowActions / toolbar.actions):

| Field           | Semantics                                                                                                                                                                                                                                                                                                                                                                                   |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `label`         | The button copy (LocalizedText)                                                                                                                                                                                                                                                                                                                                                             |
| `targetSection` | The modal to open: a section key or a **group name** (empty = a pure action-chain button)                                                                                                                                                                                                                                                                                                   |
| `params`        | The parameter mapping: row actions map row field names → form parameter names (`player_id: uid`); toolbar buttons carry static initial values                                                                                                                                                                                                                                               |
| `danger`        | Danger styling + a second confirmation whose body lists what will happen next — the dialog's prefilled parameters (field titles, enum values as option labels, no raw field keys) and the chain steps; the dialog opens and the chain runs only after the user confirms (D2)                                                                                                                |
| `chain`         | **The action chain**: steps executed in order after the main action `[{kind: runBinding\|refreshNode, target: sectionKey}]`. With a `targetSection` it runs when the dialog opens (a detail dialog preloads its own fields/table data); without a target a `danger` chain waits for the confirmation and a non-danger chain runs immediately; a failed execution step aborts the chain (B2) |

The editor (`web/src/pages/PageStudio/CompositeEditor`) and the publish renderer (`PageRenderer`'s `CompositeRenderer`) share this model; the compiler (editor → sections) and the decompiler (sections → editor tree, for read-back re-editing) guarantee the configuration round-trips without loss.

**Variable names and runtime state (V5)**: a section's `key` doubles as the **component variable name** inside the editor (`props.sectionKey` declared and pinned; drag-ins/template instantiations get auto-named and deduped by semantic rules — a `player.list` table → `playerListTable`). Binding expressions (<code v-pre>{{variable.path}}</code>) exist only in the editor layer and compile on save into the existing wire fields (the `inputAssignments` page_state paths / event `params` / row-action `row.field`), with **zero change to the server or the PageSpec protocol**. At runtime each section's state is `{ data, selectedRow, selectedRows, values }`:

- `data`: the function output (<code v-pre>{{var.data.total}}</code>); after a successful execution it merges into `page_state[var]` in merge mode.
- `selectedRow`/`selectedRows`: the table's selected rows (<code v-pre>{{var.selectedRow.uid}}</code>); selection changes do **not** trigger `refreshOn` re-runs.
- `values`: the form's current values (debounced; <code v-pre>{{filterForm.values.keyword}}</code>); the page_state snapshot of constant/function forms is **dual-shape** (flat values for legacy `/field` paths + the `values` wrapper).
- Expression→wire mapping: <code v-pre>{{var.path}}</code> → `inputAssignments: {kind: page_state, key: var, path: /branch/field}` (a JSON Pointer); <code v-pre>{{row.x}}</code> → row-action/event parameter `row.x`; literals pass through. The round-trip is invertible (read-back restores the expression text).
- **Parameter-mapping transforms (U8)**: a structured mapping (source section + field) can carry a `transform` — when the source field name ≠ the parameter name it compiles to `rename` (renames per the mapping, drops uncovered fields; the output is the governed whitelist); a filled **default value** compiles to `default` (when the upstream field is missing/null it falls back to that literal; an existing value is not overridden). Both round-trip (read-back restores the field choice and the default-value input); for the semantics see the transform whitelist in the [PageSpec protocol specification](./pagespec-protocol.md).

## The frontend runtime: the ProComponents page renderer

The page runtime is fixed on Ant Design Pro/ProComponents; the correspondence between PageSpec nodes and runtime components is in [ProComponents page generation and runtime](https://cuihairu.github.io/croupier/zh/architecture/ui-generation).

The Renderer accepts only a PublishedPageSpec and executes only through `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute`. The browser must never pass functionId, route, target, gameId, or env to pick an execution target. Before execution the server applies two structured blocks (wire contract in the [PageSpec protocol specification](./pagespec-protocol.md)): a contract with `executionState=unbound` (uploaded material not yet bound to a runtime executor) returns `409 executor_unbound`, and the frontend renders an "executor unbound" empty state guiding toward binding; a contract drift returns `409 binding_stale`. Published pages allow no mock-data fallback — a failed execution must be presented explicitly.

PageSpec must stay decoupled from the component library. Swapping the form or chart library in the future replaces only the renderer adapter — FunctionContracts, PageSpecs, menus, publish snapshots, and audit never migrate.

## Form strategy

JSON Schema is the persistence standard for function inputs/outputs; form presentation is expressed by `FormPresentationSpec`, with the protocol definition in the [PageSpec protocol specification](./pagespec-protocol.md) and the render chain plus the sole runtime constraint in [ProComponents page generation and runtime](https://cuihairu.github.io/croupier/zh/architecture/ui-generation).

`FormPresentationSpec` governs presentation only and never changes a FunctionContract payload; saving and publishing must both pass the server's structural validation, and a failed validation must error and require the administrator to fix it. The form runtime is fixed at `@rjsf/antd + @rjsf/validator-ajv8`; keeping a second form runtime in parallel inside the project is forbidden.

**The no-schema fallback (a single payload field)**: when a function has no `inputSchema`, the page generator no longer produces an empty form (a rendered blank waste page) but delivers on the normalizer diagnostic's (`input_schema_missing`) promise — it generates a single-`payload` field form (a JSON textarea, `widget: JSON`, bilingual label/description). Known boundary: payload is a wrapper key, so the submitted request body is `{"payload": "<the user's JSON text>"}` and the execution chain does not unwrap it; to get a real form, supply the schema at registration (the openapi provider already derives it automatically; see [Agent Providers](https://cuihairu.github.io/croupier/zh/guide/integrations/agent-providers)).

## Scope, menus, publishing, and evolution

The page identity is fixed as:

```text
PageIdentity = game_id + env + pageKey
```

Page Studio, the Console, Proposals, and execution all take `game_id + env` from the global scope; pages never re-select or override the scope internally.

The dynamic menu's sole source (menu_items-driven; details in [the runtime console dynamic menu](./console-dynamic-menu.md)):

```text
menu_items（菜单树） + page_specs.menu_id（挂载） + active PublishedPageSpec[]（页面内容）
  -> ConsoleMenuSpec -> ProLayout
```

The menu's multilingual text comes from `menu_items.labels`; the page's multilingual text comes from the PublishedPageSpec's NavigationSpec; dynamic menu items set `locale: false`; static locale data or dictionaries are never the source of truth. Published pages not mounted to a menu stay out of console navigation.

Publishing must freeze:

- The complete PageSpec and FormPresentationSpec snapshots.
- Each binding's function version, input/output schema digests, risk, permission, execution mode, approval policy, and semantic digest.
- The renderer ABI version and the generator version.

After a function or CapabilitySemantics change, the Server generates a new Proposal and computes the diff. Published pages are marked stale and refuse to execute; Page Studio must provide "view the diff, auto-merge safe fields, resolve conflicts, republish".

The automated close-out of contract drift (2026-09, `internal/api/page/stale_heal.go`):

- **The system heal loop** (every 5 minutes, actor `system:contract-heal`) automatically runs selector sync on pages that have a publish snapshot and a non-empty freshness report; auto-publishing covers only the **unambiguous subset** — the input side lets through only kept/removed, and the output side adds shape_updated; `renamed/added/type_changed` and any manual items land as draft only (rename is a semantic judgment; the "unique candidate" heuristic has been observed to mis-judge delete-old+add-new as a rename, so the machine never publishes it). With no changes nothing is written (the idempotence guard lets the loop run frequently).
- **User one-click sync** (the editor/change panel, `pages:edit` + `pages:publish`) **automatically continues into publishing** when the report has no manual items (the response's `autoPublished`), completing "sync → publish → snapshot refresh → console execution restored" in one step; without publish permission it degrades to a prompt handing off to a member who has it.
- Outside the declared paths above, a Draft or PublishedPageSpec is never silently updated.

The auto-merge safe set contains only presentation fields: column order and visibility, field label/help, order, group, widget hints, navigation titles, category key/order, icons, and sorting. `visibleWhen` may auto-merge only when validation proves it does not affect required inputs, binding payloads, or selector references; otherwise it joins the conflict set. Execution fields — bindings, functionId, input/output assignments, confirmation, permissions, risk, approval — must be human-confirmed on any difference; they are never auto-merged.

### Publish tiering (pages.publishReview, T10)

Whether saving goes through proposal review is controlled by an env-level policy: `pages.publishReview: auto | required`
(`configs/server.yaml`, including a `publishReviewByEnv` per-env override; precedence
`publishReviewByEnv[env]` > global > built-in default — `dev=auto`, and every other env plus a missing
`X-Env` degrades strictly to `required`). The policy is L2 configuration (see
configuration layering), judged synchronously in the save chain, not a runtime setting.

The composite save path (`versioning.Service.CreateCompositePage`) under the `auto` policy
publishes directly after a successful save (`page.Service.AutoPublishComposite`, injected as a callback through the routes assembly,
keeping the versioning ↛ api/page one-way dependency):

- **A new page**: goes through the proposal accept-and-publish chain (AcceptAndPublishProposal);
- **An existing page**: goes through the "regenerate proposal draft + publish" combination (identical to BulkRepublish's single-page path) —
  optimistic locking, the published_page_specs snapshot, and page_versions history are all recorded as usual.
  `/pages/:pageKey/republish` and `/pages/:pageKey/rollback-publish` also use the canonical publish boundary and uniformly require `pages:publish`.

Two invariant gates: **the quality gate is not lowered by review exemption** (proposal error-level diagnostics and publish validation reject as usual);
**a publish failure does not roll back the save** (the response's `publishError` carries the reason, the proposal is retained, and the frontend
degrades to the manual chain). Publishing is unified in `page.Service.publishCore`: permissions, the expected draft revision, page/selector
validation, contract snapshots, and audit rules are implemented once in the core. Entries model only their explicit differences with
`pagePublishEntry` (caller, internal caller, batch scope, and auto policy); automated paths cannot bypass draft isolation. The auto path still
requires `pages:publish` — policy-exempt review does not exempt the publish permission (the saver needs both `pages:edit` and `pages:publish`;
the audit event `auto_publish_composite` records the variant and the actor).

### One-click selector sync (sync-selectors)

Beyond presentation-field auto-merging, schema drift also invalidates selectors (`input_schema_stale`/`output_schema_stale`/ vanished targets), blocking publishing through validation and making the runtime console refuse execution. A whole-page regenerate rebuilds with default selectors and washes away row/selection/page_state/literal customizations; manually re-selecting binding by binding is slow and leak-prone. sync-selectors is the third path: **repair only the affected assignments, preserving every unaffected customization** (including Source/Kind/Path/Value/Transform). The planner (`spec/selector_sync.go`) is a pure function; dry-run and apply share the same implementation. Its object is the draft (publish validation judges the draft too); published snapshots are immutable.

The strategy ladder (inputs, per assignment, order preserved):

1. Target exists, type unchanged, source assignable → kept.
2. Target exists but the type drifted → keep the assignment and report `type_changed` (never removed — removing a required target would fail publish validation).
3. Target vanished → the prev-schema diff's rename candidates hit uniquely (`confidence=high|low`; see the prev-column semantics); without prev or with no hit, the heuristic applies (same parent × unoccupied × assignable source, used only on a unique hit, `low`); zero or multiple candidates → removed.
4. Required-set backfill (S2 semantic hit): the required field = the resource identity field (`CapabilitySemantics.IdentityField`, injected into the planner by the service layer with one lookup by the page's resourceKey) and the row source passes all three gates (`isSourceAllowed`: HasDetailView/IsRowAction; the RowSchema contains the path; the type is assignable) → automatically attach the row source (`added`+`row`+`high`, constructed the same way as the generator's `applyIdentityRowSelector`); not injected / no hit / a gate fails → fall back: non-composite pages get a same-named form mapping (gate: the page's form schema must contain the path, otherwise `manual_required`); composite pages are always `manual_required` (composite inputs should only come from page_state/literal).

The strategy ladder (outputs):

1. Source exists and the shape matches → kept.
2. Source exists but the shape is wrong: a required stateKey is re-derived; a non-required one has its shape corrected to the new type (`shape_updated`); task/dataset semantics are never auto-guessed.
3. Source vanished: a required stateKey (the six-class matrix of resource query→items, detail→detail, report→dataset, task→taskStatus/taskEvents/taskResult, kept in sync with publish validation) is re-derived through the three-step ladder of rename candidates / generator defaults / the root object (`source=""`); if all fail, the original assignment is kept + `manual_required` — **a required output is never removed into missing**; a non-required one with no candidate → removed.
4. A required output missing entirely → one is added through the same three-step ladder (`added`); if it cannot be derived → `manual_required`.

The write path and permissions: `POST /api/v1/pages/:pageKey/sync-selectors` (wire contract in the [PageSpec protocol specification](./pagespec-protocol.md)) uses SaveDraft's exact optimistic lock (the revision is re-read inside the transaction) + PageVersion + audit (action=sync_selectors); the permission is `pages:edit`. **Syncing writes the draft only and never auto-publishes** — publishing carries the `pages:publish` permission and the approval/audit semantics; the `published_page_specs` immutable snapshots are not on the sync write path, and the post-sync publish-level validation results sit in `remainingDiagnostics` for the user to review and publish manually. Execution-mode alignment rides along as a companion fix: when a task function is bound as sync (or the reverse), it is corrected per the freshness rules without manufacturing illegal combinations; governance/version drifts cannot be repaired by selector sync and, after a freshness re-run, pass through into the report's manual section.

The bulk close-out (M4, `POST /api/v1/pages/bulk-sync-selectors`): the draft-side wholesale handling of the contract-change queue — running the same planner/apply as single-page sync-selectors page by page, semantics unchanged: **strictly draft-only writes** (published snapshots untouched; going live still needs `bulk-republish`); revisions are read by the server from the current draft, concurrency conflicts 409 inside the transaction and counted per page into `failed` while the rest continue; the dry-run gate first — pages carrying Manual diagnostics (governance/version and other drifts a selector sync cannot fix) are `skipped` wholesale with their diagnostics passed through, no half-done sync.

### The page→resource association and list-condition pushdown (#30)

"Which resources does a page involve" is a derived view, not a stored fact: the `resourceKey` column ∪ the `resourceKey`s of the function contracts behind the top-level bindings, **computed at read time, never persisted** (no new column/table, avoiding a numbered migration contract surface). Multi-resource pages (one page binding functions of several resources) expand naturally under this basis. After a function-contract rebuild, the next list/aggregation read reflects the latest association; bindings whose contract is missing contribute no resources (degrading to the column basis). The association serves three consumers: the draft list items' `resources` projection, the `/pages/resources` filter-option aggregation (`pageCount` deduped by page), and the `resourceKey` filter's hit determination (legacy single-column pages also hit their bindings' associated resources).

The draft list's filtering and pagination run server-side: `status` pushes down as a SQL condition; `resourceKey`/`keyword` (substring matches over pageKey, titles, and involved resources) filter over the scope's candidate set; `page/pageSize` are clamped server-side (default 20, cap 200); the response carries `total/page/pageSize`. The frontend never pulls the full set to compute on its own; the wire contract is in the [PageSpec protocol specification](./pagespec-protocol.md).

## Model boundaries

- SDKs/OpenAPI may submit only FunctionContracts and governed capability semantics.
- The Resource Catalog may supplement only CapabilitySemantics; it can never edit page UI.
- PageProposal/PageSpec are the sole source of page generation, editing, publishing, and the dynamic menu.
- The Renderer consumes only PublishedPageSpecs; it may never fill in fields ad hoc from the latest function catalog or runtime results.
- Dynamic menu text comes from the PublishedPageSpec's NavigationSpec, never from static locale data or dictionaries as sources of truth.
- The browser executes only through the published binding execute API; it can never pick the function, target, route, or scope.
- Historical page configurations have no automatic migration path. Old page configuration data can only be exported, backed up, and manually rebuilt; no automatic conversion bridge is provided.

## Definition of done

Acceptance requires all of the following (the real-browser E2E regression entry is `web/e2e/` and the real Dashboard E2E):

1. An OpenAPI REST Resource can generate and directly publish a ResourcePage automatically; with a declared write capability it provides full CRUD, and without one it provides read-only query/detail pages.
2. An SDK explicit-capability Resource can generate and directly publish an equivalent ResourcePage automatically.
3. A function without CRUD semantics can generate and directly publish a safe Operation Page automatically.
4. Task and Report pages use real task/chart data, containing no "minimal implementation" or JSON placeholders.
5. Page Studio's common paths never require editing raw PageSpec JSON or custom mappings.
6. Publishing, the dynamic menu, scope, permissions, approvals, audit, OTel, and function-change staleness close the loop in a real-browser E2E.

The old model's deletion records and anti-regression evidence are in the old-model deletion list.
