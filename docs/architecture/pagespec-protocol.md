---
title: PageSpec protocol specification
icon: schema
order: 7
category:
  - System architecture
tag:
  - JSON Schema
  - ProComponents
  - PageSpec
---

# PageSpec protocol specification

> **Status**: Current -- this document is the sole documented source of the PageSpec / form presentation / selector wire contracts. The authoritative implementations are `internal/dashboard/spec` (Go DTOs) and `web/src/types/dashboard.ts` (frontend shared types), corresponding item by item; when this document and the implementation disagree, the code wins and this document gets fixed.

## Two different schemas

| Protocol    | Layer              | Purpose                                                                       | From registration? |
| ----------- | ------------------ | ----------------------------------------------------------------------------- | ------------------ |
| JSON Schema | FunctionContract   | Inputs, outputs, validation, and field candidates                             | Yes                |
| PageSpec    | Page orchestration | Page types, lists, details, actions, tasks, reports, navigation, and bindings | No                 |

JSON Schema is not treated as a page layout tree. PageSpec does not reuse JSON Schema's component extension fields either.

## FormPresentationSpec

Form presentation is described by the following stable structures (mirroring `spec/form_presentation.go`):

```ts
interface FormPresentationSpec {
  jsonSchema: JSONSchema; // 表单校验的事实源
  layout?: "vertical" | "horizontal" | "inline" | "grid";
  groups?: FormGroupSpec[]; // 字段分组（key/title/fields/collapsible）
  fields?: FormFieldSpec[]; // 逐字段展示覆盖
  submitButton?: FormButtonSpec;
  cancelButton?: FormButtonSpec;
}

interface FormFieldSpec {
  key: string; // JSON Schema 字段路径，如 "name"、"address.city"
  label?: LocalizedText; // 覆盖 schema title
  description?: LocalizedText; // 帮助文本
  placeholder?: LocalizedText;
  role?: "identity" | "status" | "reason" | "expiry" | "amount" | "quantity"; // x-field-role（D5）
  unit?: LocalizedText; // x-field-unit 单位后缀（D5）
  widget?: FormWidget; // 受控 antd 组件名枚举
  width?: number; // grid 栅格 1-12
  order?: number; // 排序，小者在前
  visible?: boolean;
  visibleWhen?: ConditionSpec; // 只读 form/page state
  disabled?: boolean;
  required?: boolean; // 覆盖 schema required
  defaultValue?: JSONValue;
  enumOptions?: EnumOption[]; // select/radio 选项
  remoteOptions?: RemoteOptionsSpec; // 远程选项源（x-options-source 派生）；
  // 与 enumOptions 互斥，优先 remoteOptions
  widgetProps?: Record<string, JSONValue>;
  validationRules?: ValidationRule[];
}

interface RemoteOptionsSpec {
  functionId: string; // 数据源函数 ID（同 scope 已注册的 collection_query）；
  // 调用走既有 RBAC，失败静默降级为普通输入
  labelPath?: string; // 选项标签取值路径（JSON Pointer，支持 * 通配数组段）
  valuePath?: string; // 选项取值路径；缺省复用 labelPath
  searchParam?: string; // 搜索关键词参数名；声明后下拉搜索以该参数重新调用
}
```

Renderer-effective semantics (`SchemaFormRenderer`, closed out 2026-09): `required` and `validationRules[].type="required"` merge into the runtime schema's `required` (`required: false` can retract a schema-level requirement; fields hidden by visibleWhen stay exempt); `min`/`max` map by field type (string→`minLength/maxLength`, number/integer→`minimum/maximum`, array→`minItems/maxItems`), and `pattern` writes straight into the JSON Schema `pattern`; `custom` cannot be expressed in JSON Schema, so it warns explicitly at render time and is ignored (never silently dropped). Required string fields (effective from both the schema `required` and the fields override) get `minLength: 1` auto-injected during runtime-schema derivation when no explicit `minLength` is declared — AJV's `required` only checks key existence and accepts the empty string, so without the injection "required" fails to block a cleared submit (BUG-038 related observation: the empty string only errors once it reaches the game side); an explicit `minLength` (including `validationRules` mapping products) is not overridden, and visibleWhen-hidden exempt fields are not injected; the error message for `minLength limit=1` specializes to "{title} 不能为空". `defaultValue` participates in seeding initial form values: it fills only fields with no provided value, an explicit `initialValues` wins, and `null`/empty string count as provided and are not overridden.

`widget` is a governed enum of antd/ProComponents component names (PascalCase): `Input`, `TextArea`, `InputNumber`, `Password`, `Select`, `MultiSelect`, `Radio`, `Checkbox`, `Switch`, `DatePicker`, `TimePicker`, `DateRange`, `Upload`, `ImageUpload`, `FileUpload`, `RichText`, `Code`, `Cascader`, `TreeSelect`, `Color`, `Slider`, `Rate`, `JSON`, `KeyValue`, `Array`, `Object`. Extending widgets requires modifying the spec package and syncing the frontend types; the frontend may not add them privately.

Visibility conditions are restricted expressions: they cannot read row/detail data or call functions:

```ts
interface ConditionSpec {
  kind: "equals" | "notEquals" | "exists" | "all" | "any";
  path?: string; // 当前表单或 page state 的指针
  key?: string; // 区块级条件专用：来源区块 key（非空时从页面状态
  // results[key] 按 path 取值，如 /values/mode、/data/total；
  // 空 = 表单内相对路径，字段级 visibleWhen 原语义）
  value?: JSONValue; // equals/notEquals 的比较值
  conditions?: ConditionSpec[]; // all/any 的子条件
}
```

The Server generates a default FormPresentationSpec from the input JSON Schema; administrators may change presentation info only in Page Studio, never the FunctionContract payload. The frontend renderer validates payloads with `@rjsf/antd + @rjsf/validator-ajv8` fixed. The rjsf `uiSchema` exists only as an in-memory derivation inside the adapter and must never become an SDK/OpenAPI registration field or a persisted page protocol.

Historical form presentation data offers no conversion or import path; the only option is export/backup before the old path is deleted, followed by manual reconstruction in Page Studio against the current FormPresentationSpec. Old data must never enter the new page publish flow.

### Descriptor presentation hints (x-ui-\*)

Game/SDK sides may declare `x-ui-*` extension fields on field schemas inside `input_schema`
(widget/label/grouping/links/remote option sources etc.); the frontend derives them into this
protocol's `FormPresentationSpec` via `derivePresentationSpec` (the debug path) and the server's
`buildFormFields` (the publish path, including `x-options-source` → `remoteOptions` from U4 on),
with identical behavior on both ends. For the field list, derivation rules, and governance
boundaries see the presentation hints contract. Hints never enter page orchestration
(PageSpec); the wire contract is untouched.

## PageSpec view nodes

PageSpec uses business nodes, not component names. The node set is fixed:

```text
ListViewSpec | DetailViewSpec | ActionSpec | ConfirmActionSpec
| TaskViewSpec | ResultViewSpec | FormPresentationSpec
| DatasetSpec | ChartSpec | CompositePageSpec
```

The orchestration of each page type is in the [Dashboard resource/page model](./dashboard-page-model.md). Each node has versioned, strongly typed fields and a server-side validator. The page renderer picks ProComponents by node type; concrete React component names or arbitrary component props must never appear in a PageSpec.

### CompositePageSpec (the composite-page wire contract)

The body of a `type: composite` page (authoritative implementation `internal/dashboard/spec/types.go`, frontend `web/src/types/dashboard.ts`). The creation entry point is `POST /api/v1/versioning/pages/composite`, with a request of `CompositeSectionRequest[]` (fields passed through with the same names as the spec; see [usage guide §3](../dashboard/composite-editor-v3.md)).

**Editor preview endpoint (B1)**: `POST /api/v1/versioning/pages/composite/preview` takes the exact same request body as the creation endpoint and returns `{ pageKey, pageSpec }`, where `pageSpec` is the full generated spec (`type`/`title`/`composite`/`bindings`/`componentTemplates` plus the generation metadata `quality` and `diagnostics`). It runs the same generation chain as creation — section contract lookup, binding assignment, static-section interleaving, template-snapshot passthrough, and publish-level selector validation — so the returned spec is field-for-field identical to what saving and publishing would store (verified by `TestBuildCompositePageSpec_SameAsPersistedProposal` and `TestV9Handler_PreviewCompositePage_SameSpecAsSaved`). It is read-only: no proposal, draft, or audit row is written, and it therefore carries no `pages:publish` requirement; contract visibility is still constrained by the scope and permission middleware on the request path. A missing contract for any section returns a `4xx` in the unified `{ error, message, details }` shape rather than a partial spec.

Input-validation failures never return a partial spec: a missing `pageKey`, a missing or duplicate section `key`, a static section without `form.jsonSchema`, a section referencing an unregistered function, or a canvas with no functional section returns `400 + { error: "validation_failed", message, details }`, where every `details` key is a request-body field path — `pageKey`, `sections[i].key`, `sections[i].functionId`, `sections[i].form.jsonSchema`, or bare `sections` when no functional section is left to generate. Generator-level diagnostics are mapped the same way (their `field`, falling back to their `code`). A database failure looking up a contract is not a canvas problem and stays a `500`. The preview pane and the save/publish dialogs render these entries inline through the shared `ErrorDetailList` so the operator can point at the offending section.

**Draft preview execute endpoint (B1, 2026-10)**: `POST /api/v1/versioning/pages/composite/preview/execute` lets the editor preview pane execute a section against the draft instead of against mock data. The request body is the preview endpoint's body plus `bindingId` and `context`:

```json
{
  "pageKey": "composite--preview",
  "bindingId": "player.list",
  "sections": [
    { "key": "player.list", "functionId": "player.list", "view": "table" }
  ],
  "componentTemplates": [{ "key": "tpl--card", "digest": "d1" }],
  "context": {
    "form": {},
    "row": {},
    "selection": [],
    "detail": {},
    "pageState": {}
  }
}
```

The server rebuilds the draft `PageSpec` from `sections` through the identical generation chain (so the executed spec is the one the preview pane rendered), re-runs the publish-level selector validation, and then runs the binding through the same execution gates as a published page — permission check, approval gate, and the bound `execution.functionId` snapshot (the browser never submits `functionId`, `route`, or `scope`; a missing/extra `bindingId` is a `4xx`). The response is `{ result }`, the same `PageExecutionResult` shape as the published binding execute endpoint, so the renderer handles both identically. It is still read-only — no proposal, draft, approval record, or audit row is persisted — but a real function call is dispatched, so operation functions and approvals do have real side effects in draft mode.

**Constant form sections (static, 2026-09)**: `CompositeSection` gains a `static: true` shape —
no bindingId is bound and no function executes; `form.jsonSchema` is defined at editor design
time and passed through `CompositeSectionRequest.form` (the server validates schema existence),
and values are written only into page state for refreshOn/action-chain consumption. Publish
validation: a static section must not carry a bindingId, its view must be form, and it must
include form.jsonSchema. Section order: static sections are **persisted interleaved at their
request-input positions** (a 2026-09 revision — previously the server appended all static
sections to the end, so pages with a "constant filter at the top of the page" drifted in order
after publishing); after accept-and-publish, the publish snapshot's section order matches the
request.

```json
{
  "sections": [
    {
      "key": "player.list",
      "bindingId": "player.list",
      "view": "table",
      "span": 24,
      "autoRun": true,
      "refreshOn": [],
      "table": {
        "columns": [
          { "key": "uid", "title": { "zh-CN": "用户" }, "dataType": "string" }
        ],
        "rowActions": [
          {
            "label": { "zh-CN": "发邮件" },
            "targetSection": "modal-ab12cd",
            "params": { "player_id": "uid" },
            "chain": [{ "kind": "refreshNode", "target": "player.list" }]
          }
        ]
      },
      "toolbar": {
        "actions": [
          {
            "label": { "zh-CN": "批量补偿" },
            "targetSection": "modal-ab12cd",
            "danger": true
          }
        ]
      }
    },
    {
      "key": "mail.send",
      "group": "modal-ab12cd",
      "bindingId": "mail.send",
      "view": "form",
      "display": "dialog",
      "onSuccessRefresh": ["player.list"]
    }
  ]
}
```

Field semantics (detailed rules in the [Dashboard page model's CompositePage section](./dashboard-page-model.md)):

- `key`: the section's unique identifier; **multiple instances of the same function** number as `fid`/`fid-2`…, editors may declare a fixed key (`sectionKey` pinned, round-trip stable), and the creation endpoint errors explicitly on duplicate keys
- `display`: `inline` (default) | `dialog` (a modal, occupies no grid) | `tab` (aggregated by the renderer into Tabs) | `card` (aggregated by the renderer into a Card)
- `group`: the dialog/tab/card grouping — `dialog` sections aggregate by group into one modal (form + field cards + tables mixed); action targets (`targetSection`) point at the group; `tab` sections aggregate by group into one Tabs; `card` sections aggregate by group into one Card
- `tab`: the tab label (LocalizedText; effective when `display=tab`) — within a group, sections aggregate by label onto the matching Tabs page, stacked full-width within the page; when absent the renderer falls back to "Tab N"
- `cardTitle`: the card title (LocalizedText; effective when `display=card`) — the group title when same-group sections render into one Card (full-width); falls back to the group name when absent. On read-back it is restored into the card container's `props.title`
- `rowActions[].params`: the row-field → form-parameter mapping (`"player_id": "uid"` = the row's uid fills the modal form's player_id)
- `chain`: the action chain, executed in order after the main action: `runBinding|refreshNode`
- `onSuccessRefresh`: section keys re-run automatically after a form submits successfully — and only on business success: an `approval` or `task` result re-runs no target (D4, see the execution result feedback bullet)
- `events`: general event bindings (`rowClick`/`rowSelected`/`success`/`error`/`click` → action + chain); action kinds: `runBinding`/`refreshNode`/`openModal`/`closeModal`/`navigate`/`showMessage`; step `params` support source references (`"sectionkey.field"`, `"row.field"`, literals)
- `refreshOn`: page_state linking — changes to an upstream section key re-run this section automatically, with the upstream output's top-level same-named fields merged into this section's input
- `visibleWhen`: **section-level conditional display** (U10) — a `ConditionSpec` leaf requires `key` (the source section), reads its value from page state `results[key]` by `path` (`/values/field`, `/data/field`, `/selectedRow/field`) and evaluates; an inline/tab section that evaluates false does not render but still executes (autoRun/refreshOn unaffected); nested `all`/`any` supported (depth ≤ 4); publish validation requires key ∈ page sections, path to be a JSON Pointer, and equals/notEquals to carry a value; dialog sections do not participate (modals are triggered explicitly by actions)
- `cascadePolicy`: **the refreshOn cascade failure policy** (U9, `"pause" | "clear" | "keep"`, empty = default `pause`) — this section's behavior when any upstream dependency's latest result is a failure. None of the three re-runs: `pause` keeps the data + a message.warning notice (linking paused); `keep` silently retains the last result; `clear` empties the section's data. Renderer implementation (B2): when a section's execution fails it writes an `{error}` failure marker into `results[key]`, and the cascade signal is computed as "section key : result revision : failure marker (o/e)" — the revision increments once per new result (a success **and** a failure are both new results), so a same-state new upstream result (two consecutive successful runs with different data) also re-runs the downstream section; a failed→succeeded flip likewise re-triggers it (a paused cascade resumes automatically once the upstream recovers). Only the latest in-flight response of a section is committed (a superseded response is discarded), a section whose `refreshOn` graph cycles back to itself is refused the cascade (no duplicate or infinite runs), and a `pause` notice is raised only when the upstream failure first appears rather than on every repeated failure. Publish validation requires a legal enum (`composite_section_cascade_policy_invalid`). Static sections execute no binding (they cannot fail themselves), so the field has no real effect there but passes through

- `table.pagination`: **server-side pagination for a table section** (D3) — the resource page's `PaginationSpec` shape (`{enabled, defaultSize?, pageSizes?}`). When `enabled`, the renderer appends `page` (1-based), `pageSize`, and `offset=(page-1)*pageSize` to the section's request form — the offset is derived by the renderer and sent alongside the page number (the page number is never passed as an offset and vice versa), and the platform input schema passes extra keys through by default. The response is consumed as `items` plus an optional `total`: a present `total` is used verbatim, and when it is absent the renderer estimates the bound as "a full page means a next page may exist" and closes off on a partial page, so a missing total never reads as "only the current page left". Flipping pages or the page size re-runs the section with only the pagination parameters changed; an upstream filter/query change (the section's non-pagination inputs) resets to page 1; a post-operation refresh (`onSuccessRefresh` or an event `refreshNode`) keeps the current page and the filter. Unset or `enabled: false` keeps the pre-D3 client-side pagination over the returned rows (no pagination request parameters). The field is set from the composite editor's table component (a server-side pagination toggle with page-size and size-options fields) and travels through the composite section request as a section-level `pagination`, which the generator writes into `table.pagination`; the generator never infers pagination for a composite section (no resource-semantics context), so it is opt-in per table. The bound function must consume `page`/`pageSize`: the platform passes extra input keys through, so a function that ignores them returns the full set on every page (identical page contents, an inflated page count). Boundaries: cursor pagination and a per-table `totalKey` are not supported (the wire `PaginationSpec` has no cursor field), tables rendered inside a dialog do not paginate and do not offer header sorting, and server-side filter pushdown is not implemented (composite filtering goes through the query-form refreshOn cascade — see the column-sorting bullet for the sort pushdown that is implemented)

- `table.selection`: **table row selection for a composite section** (D2) — `{multiple?, identityKey?}`. Unset or `multiple: false` keeps the pre-D2 single-row radio selection (row selection is the row's `selectedRow`; the `rowSelected` event fires with that single row). `multiple: true` switches the section to checkbox multi-select plus a selection bar above the table — a running "selected N item(s)" count, a "M visible on this page" count shown only when some selected rows are not on the current page (so an operator cannot verify targets from the visible page alone), a "view target list" entry opening a modal that lists every selected row (identity column first, then up to two more display columns), a "clear selection" action, and a stale-selection warning when the section's filter inputs change after a selection (pagination is not a filter change; a changed filter with a non-empty selection warns because a preserved cross-page key cannot distinguish "still on another page" from "excluded by the new filter" — the operator is told to review, then clear or re-select). The `rowKey` and the row-identity used by the target list, the counts, and the runtime `selectedRow`/`selectedRows` state all use `selection.identityKey` → `table.identityKey` → the row's `id`/`key` → the row itself. The selection writes the chosen rows into the section's runtime state (`selectedRow` = first row, `selectedRows` = all rows) — the same state `rowSelected`/<code v-pre>{{...selectedRow.x}}</code> expressions and server-side input assignments read — and `rowSelected` still fires with the first selected row on every change. The field is set from the composite editor's table component (a "row selection" toggle: enable multi-select + a row-identity column pick from the output schema) and travels through the composite section request as a section-level `selection`, which the generator writes into `table.selection`; the generator never infers row selection for a composite section (no resource-semantics context), so it is opt-in per table. Boundaries: a dialog-rendered table does not offer the selection bar, the target-list modal truncates to three display columns (identity first), the count "visible on this page" is computed over the section's `items` (the current result page under server-side pagination; the full result set under client-side pagination, where it therefore matches the selected count and the visible-count hint is not shown), there is no per-table select-all semantics beyond the column header, and `getCheckboxProps`-style per-row disable/selectable is not exposed

- `table.columns[].sortable`: **column sorting for a table section** (D3) — the generator marks a column sortable iff its output-schema `dataType` is `number` or `datetime` (a mechanical dataType-derived annotation, not resource-semantics inference; unlike pagination/selection it is not an editor opt-in, because sorting never presumes the bound function accepts sort parameters when server-side pagination is off). A sortable column renders the antd header sort UI; a column whose `dataType` is `string`/`boolean`/`object`/`array` stays unsortable. When `table.pagination.enabled` is false (or unset) the sort is local: the renderer sorts the returned rows with a compare function (number by numeric value, other dataTypes by string code point, missing values last) and sends no sort request parameters. When server-side pagination is on the sorter is UI-only (`sorter: true`) and the controlled order state drives the request: clicking a header re-runs the section with `sortBy` (the column key) and `sortOrder` (`asc`/`desc`) appended to the request form, resetting to page 1; clicking the same header again toggles to the opposite order (two-valued, no cancel state). Sorting is not part of the section's filter fingerprint: it is held in the renderer's per-section table state (not in `sectionInputs`), so a sort re-run does not raise the D2 stale-selection warning, while an upstream filter change (which resets to page 1) keeps the current sort. Boundaries: only two-valued order (no three-state cancel, no multi-column sort), sorting does not persist across a page reload, the request parameters are `sortBy`/`sortOrder` (no `orderBy`/`direction` alias), the bound function must actually consume them (the platform passes extra input keys through, so a function that ignores them returns an unsorted page while the UI shows a sort arrow), and a dialog-rendered table renders its columns verbatim (no header sort UI)

- `table.columns[].unit` / `columns[].enum` / `columns[].render` / **explicit field semantics** (D5) — the output side of the `x-*` presentation hints. The game SDK declares semantics on the contract output schema's item properties (mechanically parsed, never guessed from field names): `x-field-role` (`identity|status|reason|expiry|amount|quantity`) and `x-field-unit` (a bare string or a `{zh-CN,en-US}` object), alongside the existing `x-enum-options`. The generator collapses these into wire fields: `role=identity` → the column gets `fixed:"left"` + `render:"copy"` (a pinned, copyable identity column); `x-enum-options` → the column gets `enum` + `render:"tag"` (a status enum tag column); `x-field-unit` → the column gets `unit`. The input side carries them into `FormFieldSpec.role`/`unit` (parsed in `form_hints.go` and the frontend `derivePresentationSpec`, invalid roles silently ignored). Rendering: the composite renderer and the resource renderer append the unit to the column header (`标题 (单位)`) and to `number` cell values; `render:"copy"` renders a copyable cell; `dataType:"datetime"` cells render with an explicit browser-local timezone label (`UTC±HH:MM` — the platform does not declare game-server timezones, so the label makes the offset explicit instead of implying UTC); `enum`+`tag` renders the option label as a colored antd Tag. The confirm dialog (D2 danger confirmation) picks the semantics up from the target form's `FormFieldSpec.role`/`unit`: identity entries are sorted first and bolded, expiry entries render in danger red, amount/quantity values gain their unit suffix, so the operator reads who/what/how-much/how-long before confirming. Boundaries: units are display-only (no amount scaling or conversion — a declared unit is appended verbatim, and a per-unit × batch-target total needs D2 batch context); roles other than `identity` do not change column layout (status/reason/expiry drive confirm emphasis and future defaults only); the timezone label is the browser's local offset (no game-level tz configuration); composite column semantics ride on the same generator derivation (a composite table section bound to a contract with hints gets the same columns), and a dialog-rendered composite table keeps the semantic cell rendering but no header sort UI

- `danger` + `chain` (rowActions / toolbar.actions): **the dangerous-action confirmation contract** (D2) — a `danger` action never executes before an explicit confirmation. With a `targetSection` the confirm body lists the modal's prefilled parameters and the chain steps, and only then does the modal open and the chain run; without a `targetSection` (a pure action-chain button) the confirm body lists the chain steps. The confirm body renders parameters by the target form's field titles, maps enum values to their option labels, and shows parameter names humanized rather than raw field keys, so the reviewed content matches what will actually be submitted. A non-danger action with a `targetSection` opens the modal and runs its chain immediately (a detail modal uses its chain to preload its own fields/table data); a non-danger action without a target runs its chain immediately. With D5 the confirm body additionally carries the target form's field semantics (`FormFieldSpec.role`/`unit`): identity entries sort first and render bold, expiry entries render in danger red, and amount/quantity values carry their declared unit suffix. Boundaries: the emphasis covers identity/reason/expiry/amount/quantity declared via `x-field-role` (fields without a role render as before), `openModal` chain steps carry no danger flag (the wire has no such field) so a chain step that opens a modal does not confirm, and the confirmation happens before the modal opens so edits made inside the modal are not re-confirmed
- **Execution result feedback** (D4, no new spec field): the renderer classifies each execution result by `PageExecutionResult.kind` instead of treating any non-error return as success. A `sync` (or kind-less) result is business success — it fires the form's `success` event binding, re-runs the `onSuccessRefresh` targets, and closes the dialog. An `approval` result keeps the dialog open with a "submitted for approval" notice showing the `approvalId` and a deep link to the approval center (`/approvals?approvalId=...`), and fires neither the success event nor the refresh (the business side has not changed until it is approved). A `task` result keeps the dialog open with a "task submitted" notice showing the `taskId`. A thrown error keeps the dialog and its entered values open and renders the failure inline: the stable `error` code from the response body, the server `message`, and the field-level entries of `details` mapped to the form's field titles (raw field keys are not shown); an error returned in the result body rather than thrown (a top-level `error` field) is treated as a failure too and fires no success event. Permission-denied failures are normalized server-side to `403 permission_denied` with machine-readable `details` (`required` permissions, `allowedRoles`, `functionId`, or task scope); the renderer surfaces those values as a compact reason line instead of only the generic status copy. Field-level `details` entries whose field exists in the section's form schema are injected under the matching control through the shared form renderer's `externalErrors` prop (RJSF `extraErrors`), so only form-external fields remain in the failure summary. Boundaries: an inline (non-dialog) form section enters a loading skeleton while submitting and loses its entered values on a failed submit (dialog forms keep them), there is no tasks route so a pending task shows the `taskId` without a navigation target, and the approval link is a fixed `/approvals` route
- **Execution idempotency** (D4, `Idempotency-Key` header; no new spec field): every page binding execution — published and draft-preview alike — is marked with a client-generated idempotency key, and the server guarantees a key executes at most once within its claim window. The claim is taken after all execution gates pass (executor bound, contract freshness, governance, payload validation) and before the approval/invoke fork, so a gate failure never consumes a key. Outcomes: a completed key replays the stored result verbatim (`replayed: true` in the response, no re-dispatch, no duplicate audit or execution-log record — one action is one record); a key still running inside the claim window is rejected with `409 idempotency_key_in_use` (result unknown — retry the same request later); the same key with different request content (page version, binding, function, or payload) is rejected with `409 idempotency_key_reused`; a `running` claim older than 15 minutes counts as abandoned and may be reclaimed by an identical retry. No header means no dedupe — the legacy behavior (every submit executes) is unchanged. The client keeps one key per execution intent (page + binding + context) until the intent resolves: a `sync` result releases it (the next identical submit is a new intent), while `approval`/`task` results keep it so an accidental resubmit replays the same approval/task instead of creating a duplicate, over a retention window (10 minutes, a per-tab in-memory cache) deliberately shorter than the server claim window; a replayed sync success is announced as "returned the server-recorded result" rather than a fresh success. Records are purged lazily (24h retention, at most one DELETE per 10 minutes per process). Boundaries: a failed dispatch leaves the claim `running` for the full 15-minute window (the side effect is genuinely unknown — retries inside the window get `idempotency_key_in_use`, not a silent re-run), the client key lives in memory only (a page reload generates a fresh key, and a cache-evicted retry degrades to a new intent), a stale claim is reclaimed by a real re-execution after the window (not a replay), async (`task` mode) submissions are claimed at submit time (the task system's own idempotency is separate), and the `replayed` flag travels in the response envelope rather than inside `PageExecutionResult`

**The page-level optional field `componentTemplates`** (U11, a template usage snapshot; located at
the PageSpec root, not inside a section): `componentTemplates?: Array<{ key: string; digest: string }>` —
carried in the `POST /api/v1/versioning/pages/composite` request body, normalized by the server
(trim / dedupe same-key keeping the first / drop blank keys), persisted with
`PageSpec.componentTemplates`, and passed through as JSON across proposal→draft→published.
Semantics: records the content fingerprints of the component templates used when the page was
created/edited (the digest algorithm and what the template API exposes are in the component
template API docs); when the editor opens the page it compares against the template library's
current digests and prompts "a used template has a newer version". It **does not participate in
publish validation and does not participate in rendering** — instantiation is copy semantics; the
page content is always the copy as of save time (the reminder never auto-syncs).

**The creation endpoint response's publish-tiering fields (T10)**: besides `proposalKey`/`pageKey`/
`pageType`/`quality`, the response adds `published: boolean` and an optional `publishError?: string` —

- `pages.publishReview=required` (the default policy): `published: false`, no `publishError`
  field, behavior identical to history (saving only creates a proposal; the ProposalInbox accepts
  and publishes manually);
- `pages.publishReview=auto`: publishing happens directly after a successful save, `published: true`;
  when publishing is rejected by a quality gate (error-level diagnostics / publish validation) or
  fails, `published: false` and `publishError` carries the reason — **the save itself has already
  succeeded and the proposal is retained**; the frontend degrades to the manual chain per the prompt and retries;
- Publishing goes through the real chain (snapshots / version history as usual) and requires `pages:publish`;
  policy exempts review only, not the publish permission. See the "publish tiering" subsection of the
  [Dashboard resource/page model](./dashboard-page-model.md).

## Data references and mapping (the Selector AST)

Input/output mappings must be validatable ASTs (mirroring `spec/selector_ast.go`):

```ts
type TransformSpec =
  | { type: "pick" }
  | { type: "rename"; params: Record<string, string> }
  | { type: "default"; params: { value: JSONValue } };

type ValueSource =
  | { kind: "form"; path: JsonPointer; transform?: TransformSpec }
  | { kind: "row"; path: JsonPointer; transform?: TransformSpec }
  | { kind: "selection"; path: JsonPointer; transform?: TransformSpec }
  | { kind: "detail"; path: JsonPointer; transform?: TransformSpec }
  | {
      kind: "page_state";
      key: string;
      path?: JsonPointer;
      transform?: TransformSpec;
    }
  | { kind: "literal"; value: JSONValue };

interface InputAssignment {
  target: JsonPointer;
  source: ValueSource;
}

interface OutputAssignment {
  stateKey: string;
  source: JsonPointer;
  shape: "scalar" | "object" | "collection" | "task" | "dataset";
}
```

`transform` is a whitelist-controlled set of transforms; adding one requires extending the spec package and syncing validators. The current three:

- **`pick`** (selection only): extracts the field at path from each selected row, producing an identity array.
- **`rename`** (row / selection / page_state, and path must be empty — the transform applies to the whole object):
  `params` is a "source field name → target field name" mapping (values must be strings). The output object
  **contains only fields hit by the mapping** (a governed whitelist, same rationale as pick); for selection
  sources it applies element-wise. Use it to rename when upstream field names do not line up with function
  parameter names; fields the mapping does not cover are dropped. Non-object sources (and non-array
  selections) error with 422 at execution time.
- **`default`** (any kind): `params: { value: <JSON literal> }`. When the source value is missing (the upstream
  section never produced the field) or `null`, it falls back to `value`; when a value exists it is not overridden.
  Used for the "default value" of parameter mappings.

The composite page editor's parameter mapping compiles into these transforms: cross-section field renames →
`rename` mappings; a "default value" input → `default` (the editor input is a string; pure numbers/booleans/null
keep their JSON types).

Validators must confirm:

- the target exists in the bound function's input JSON Schema.
- sources reference only defined forms, rows, selections, details, or page state.
- source/target types are assignable; passing a raw whole-row object to a function is not allowed.
- A ListView's collection, pagination, and identity references are verifiable against the output JSON Schema or CapabilitySemantics.
- A Report's dataset, metric, and dimension references are verifiable.

Saving unstructured `map[string]any`, arbitrary JSONPath strings, or component-level ad-hoc mapping overrides is forbidden.

## PageBinding

```ts
interface PageFunctionBinding {
  id: string;
  functionId: string;
  usage:
    | "query"
    | "detail"
    | "action"
    | "task"
    | "task_status"
    | "task_events"
    | "task_result"
    | "task_cancel"
    | "task_retry"
    | "report";
  selectors?: {
    input: SelectorAST; // InputAssignment 集
    output: OutputAssignment[];
  };
  execution: {
    mode: "sync" | "task";
    requireConfirm?: boolean;
  };
}
```

The task lifecycle capabilities split into standalone usages (`task_status`/`task_events`/`task_result`/`task_cancel`/`task_retry`), mapped one-to-one by TaskSemantic's start/status/events/result/cancel functions. There are no `create`/`update`/`delete` usages — write operations unify as `action`, referenced by a ResourcePage's CreateForm/UpdateForm/DeleteAction nodes. Confirmation is expressed by `execution.requireConfirm`; there is no separate ConfirmationSpec.

Schema nodes and page actions may reference only a `bindingId`. At runtime the server resolves functionId, permission, risk, scope, and dispatch target from the active PublishedPageSpec; the browser has no say in these.

### Binding execute error codes (the execution-blocking contract)

`POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute` applies two structured blocks before execution, both returning `409 + { error, message, details }` (the error codes are stable codes the frontend branches on):

| error              | Trigger                                                                                                                           | details                               |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------- |
| `executor_unbound` | The bound contract has `executionState=unbound` (uploaded material not yet bound to a runtime executor; checked before freshness) | `bindingId`, `functionId`             |
| `binding_stale`    | The published snapshot's contract digest has drifted from the current contract (schema/version change)                            | `bindingId`, `functionId`, `statuses` |

The frontend branches on the stable `error` code: `executor_unbound` renders an "executor unbound" empty state and guides the user to complete binding in OpenAPI Sources (after same-named function registration, T6 flips it to bound automatically — no page edit or republish needed); `binding_stale` goes through the diff / republish chain. Published pages (the Console runtime) allow no mock-data fallback of any kind — a failed execution is an explicit failure.

### One-click selector sync (the sync-selectors wire contract)

A precise repair of a **draft** after schema drift, touching only the affected assignments:

```ts
// POST /api/v1/pages/:pageKey/sync-selectors   （权限 pages:edit）
interface PageSyncSelectorsRequest {
  draftRevision: number; // 必填；乐观锁（409 冲突与 SaveDraft 同语义）
  dryRun: boolean; // true 只出报告不落库
  bindingIds?: string[]; // 省略 = 全部 binding
}

interface PageSyncSelectorsResponse {
  pageKey: string;
  dryRun: boolean;
  applied: boolean;
  draftRevision: number; // dryRun=原值；apply=+1
  syncedBindings: BindingSelectorSyncReport[];
  remainingDiagnostics?: Diagnostic[]; // 同步后发布级校验（不因错误拒绝保存）
}

interface BindingSelectorSyncReport {
  bindingId: string;
  functionId: string;
  changed: boolean;
  executionModeFixed?: boolean; // task/sync 模式按最新契约修正
  input?: SelectorSyncInputEntry[];
  output?: SelectorSyncOutputEntry[];
  manual?: Diagnostic[]; // 不可自动修复项（函数缺失/governance 漂移等）
}

interface SelectorSyncInputEntry {
  target: JsonPointer;
  action:
    | "kept" // 未受影响，原样保留（含 Transform/literal 定制）
    | "renamed" // 消失 target 重映射（newTarget）
    | "removed" // 摘除（无唯一候选且非必需）
    | "added" // required 差集补齐：identity 字段语义命中接 row 源，否则 form 同名映射
    | "type_changed" // 类型漂移，保留待人工核对
    | "manual_required"; // 无法安全自动处理
  newTarget?: JsonPointer;
  sourceKind?: ValueSource["kind"]; // added 时：row（identity 语义命中）| form（同名回落）
  confidence?: "high" | "low"; // high=prev schema 精确命中或 identity 语义命中；low=启发式
  reason: string;
}

interface SelectorSyncOutputEntry {
  stateKey: string;
  source: JsonPointer;
  action: SelectorSyncAction;
  newSource?: JsonPointer;
  newShape?: OutputAssignment["shape"];
  required: boolean; // 必需输出（items/detail/dataset/taskStatus/…矩阵）
  confidence?: "high" | "low";
  reason: string;
}
```

Semantic keys: dry-run and apply share the same planner (`spec/selector_sync.go`), so preview and persistence cannot drift; apply does not auto-publish, and when `remainingDiagnostics` contains an error, publishing stays blocked until the `manual_required` items are handled; required outputs are never removed into missing on any path (a failed derivation keeps the original assignment + `manual_required`). For the prev schema's origin and exactness determination (`previousInputSchema`/`previousOutputSchema`, the dual-algorithm digest matching degradation strategy), see the [Dashboard resource/page model](./dashboard-page-model.md).

### Bulk selector sync (the bulk-sync-selectors wire contract, M4)

The draft-side batch close-out of the contract-change queue — running the same planner/apply as single-page sync-selectors, page by page:

```ts
// POST /api/v1/pages/bulk-sync-selectors   （权限 pages:edit；body 可为空）
interface PageBulkSyncSelectorsRequest {
  pageKeys?: string[]; // 省略/空白 = 审批收件箱 ContractChanges 全量（去重）
}

interface PageBulkSyncSelectorsResult {
  total: number;
  synced?: string[]; // 成功页（selector 拉齐 + DraftRevision+1）
  skipped?: {
    pageKey: string;
    reason: string; // 恒 "manual_required"
    manual?: Diagnostic[]; // 不可自动修复漂移的诊断透传（governance/version 等）
  }[];
  failed?: { pageKey: string; error: string }[]; // 单页失败不中断其余页面
}
```

Semantic keys: **strictly draft-only writes** (`published_page_specs` untouched; going live still needs `bulk-republish`); revisions are read by the server from the current draft version (concurrency conflicts 409 inside the transaction, counted per page into `failed` while the rest continue); a dry-run gate first — pages with Manual diagnostics (governance/version and other drifts a selector sync cannot fix) are `skipped` wholesale with their diagnostics passed through, no half-done sync; no auto-publish.

## Draft list filtering and resource aggregation (wire contract)

The page workbench's list filtering, keyword search, and pagination are all executed server-side (OPEN-ISSUES #13/#30): the frontend must not pull the full set and derive on its own; a list response is always one filtered pagination window.

```ts
// GET /api/v1/pages?resourceKey=&status=&keyword=&page=&pageSize=
//   （权限 pages:read；query 条件可组合）
// - resourceKey：按「页面涉及资源」过滤（resourceKey 列 ∪ binding 函数契约资源，
//   服务端读取时计算；旧单列页面同样命中其 binding 关联资源）
// - status：draft / published / archived（SQL 条件下推）
// - keyword：对 pageKey、标题各语言、涉及资源做不区分大小写的包含匹配
//   （scope 内候选集内存过滤，scope 本身已限定数据面）
// - page/pageSize：服务端分页窗口；缺省 page=1、pageSize=20，pageSize 上限 200，
//   非法值（<1 或超上限）钳制，不报错
// 响应：{
//   items: PageSpecDraftSummary[];  // 当前窗口；条目含 resources 投影（见下）
//   total: number;   // 过滤后的总条数（分页依据）
//   page: number;    // 实际生效页码（钳制后）
//   pageSize: number; // 实际生效页大小（钳制后）
// }

// GET /api/v1/pages/resources   （权限 pages:read）
// scope 内页面涉及资源的服务端聚合——过滤下拉的选项来源，
// 前端禁止从当前列表页自行推导（列表只是过滤后子集）。
interface PageResourceOption {
  resourceKey: string;
  pageCount: number; // 该资源出现在多少个页面（按 #30 关联口径，多资源页参与计数）
}
// 响应：{ items: PageResourceOption[] }，按 resourceKey 升序
```

`PageSpecDraftSummary.resources` (#30): an item-level "page → involved resources" projection, aggregated and deduplicated at server read time from the `resourceKey` column ∪ the `resourceKey`s of the function contracts behind the top-level bindings (ascending). Multi-resource pages expand here; when an old payload lacks the field the frontend falls back to the single `resourceKey` column. The association is **computed at read time, never persisted** — no new column/table, so after a function-contract recompute the next list reflects the latest association.

Boundary (stated honestly): the `resources` association depends on the binding function contracts' `resourceKey` in the database; when a contract is missing or its index not rebuilt, that binding contributes no resources (degrading to the `resourceKey` column basis). The `/pages/resources` aggregation and the list-side association share the same basis (from #30, multi-resource pages count toward `pageCount`). The resource catalog (the function/contract-side resource inventory) is a separate aggregation; the two share no cache. The menu management page's "mounted pages" view always pulls `pageSize=200` in one page to fetch the scope's full mount mapping (mount state is a low-frequency, small data surface) — not a pagination bypass.

## Navigation and multiple languages

Category, title, icon, and order are PageSpec top-level fields (`category{key,order}`, `title`, `icon`, `order`). `category` keeps only `key` and `order`, as workbench-side grouping metadata; the runtime console's navigation is driven by the menu system (the `menu_items` tree + the `page_specs.menu_id` mount mapping; see [the runtime console dynamic menu](./console-dynamic-menu.md)), consumes no `category`, and category display names in each language come from the menu system (`menu_items.labels`). `NavigationSpec` carries only back-navigation behavior:

```ts
interface NavigationSpec {
  title?: LocalizedText;
  breadcrumb?: LocalizedText[];
  showBack?: boolean;
  backPath?: string;
}
```

PageProposals provide defaults from the resource/page key; PageDrafts hold the final values; PublishedPageSpecs are the sole source of the Console's dynamic menu. Static locale data and dictionaries must never become the source of truth for dynamic pages. `ConditionSpec` reads only the current form or page state (section-level reads page state `results[key]` via `key` — the form's current values / function outputs / table selection are all page state projections); accessing rows, details, external functions, or arbitrary JSONPath through visibility conditions is forbidden, keeping save, publish, and runtime semantics consistent.

## The component template API (related wire contract)

The REST contract of component templates (the V4 reuse layer) does not carry the PageSpec body itself, but the template's `tree`
field holds a composite-page PageNode subtree (compiled into a PageSpec by the composite page editor and then published).
Endpoints and DTO definitions are in the component template API docs.

## ABI and versions

`rendererSchemaVersion` (currently `page-spec:1`), `generatorVersion` (currently `page-generator:1`), and each snapshot digest must be stored separately. Publish validation must reject unknown versions, unknown nodes, unknown fields, and illegal mappings. The page runtime must never attempt to downgrade, guess, or convert into another layout/schema.

## Security boundaries

- A PageSpec never stores routes, target services, scope overrides, secrets, or arbitrary HTTP parameters.
- Unknown JSON may exist only at JSON Schema's standard extension boundary; core DTOs use explicit types and `JSONValue`, never `any`.
- Form presentation changes must never silently alter a published page; a publish snapshot must include the FormPresentationSpec.
