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
  key: string; // 字段标识：裸顶层属性名（"name"）或 JSON Pointer（"/address/city"），见「字段标识」
  label?: LocalizedText; // 覆盖 schema title
  description?: LocalizedText; // 帮助文本
  placeholder?: LocalizedText;
  role?: "identity" | "status" | "reason" | "expiry" | "amount" | "quantity"; // x-field-role（D5）
  unit?: LocalizedText; // x-field-unit 单位后缀（D5）
  perTarget?: boolean; // x-field-per-target 数量/金额按每个选中目标逐项计（D2 跨目标总量）
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

Renderer-effective semantics (`SchemaFormRenderer`, closed out 2026-09): `required` and `validationRules[].type="required"` merge into the runtime schema's `required` (`required: false` can retract a schema-level requirement; fields hidden by visibleWhen stay exempt); `min`/`max` map by field type (string→`minLength/maxLength`, number/integer→`minimum/maximum`, array→`minItems/maxItems`), and `pattern` writes straight into the JSON Schema `pattern`; `custom` cannot be expressed in JSON Schema, so it warns explicitly at render time and is ignored (never silently dropped). Required string fields (effective from both the schema `required` and the fields override) get `minLength: 1` auto-injected during runtime-schema derivation when no explicit `minLength` is declared — AJV's `required` only checks key existence and accepts the empty string, so without the injection "required" fails to block a cleared submit (BUG-038 related observation: the empty string only errors once it reaches the game side); an explicit `minLength` (including `validationRules` mapping products) is not overridden, and visibleWhen-hidden exempt fields are not injected; the error message for `minLength limit=1` specializes to "{title} 不能为空". `defaultValue` participates in seeding initial form values: it fills only fields with no provided value, an explicit `initialValues` wins, and `null`/empty string count as provided and are not overridden. Submission values go through one projection rule (`projectSubmitValues`, closed out 2026-11): every submit path — the form's native submit, the function debug page, resource create/edit, row-action forms, and imperative handle callers — extracts the payload with the same `projectSubmitValues(values, spec)` (exposed on the renderer handle as `getSubmitValues()`); hidden keys are computed from the current values (`visible: false` or a failing `visibleWhen`) and stripped from the payload, while the values themselves stay in form state so toggling visibility back restores the user's edits. Raw `getValues()` still returns all fields including hidden ones and is for state mirrors/echo only — never build a submit payload from it.

### Field identifiers (SUI-019)

`FormFieldSpec.key` (and every `groups.fields` entry) is a field identifier with two accepted forms:

- **Legacy bare top-level key** — a property name at the schema root (`"name"`). This is the only form the Go generator (`buildFormFields`) and the historical frontend `derivePresentationSpec` produce; behaviour is unchanged. A dotted name such as `address.city` is *not* a path — it is looked up as a literal property name and normally does not resolve.
- **JSON Pointer (RFC 6901)** — `/address/city` for object subfields, `/tags/-/label` for array-item fields (`-` addresses `items`). `~1` escapes `/` and `~0` escapes `~`, so keys containing dots or slashes locate unambiguously.

Renderer behaviour for pointer keys (`SchemaFormRenderer`, closed out 2026-11): presentation hints (label, description, enumNames, disabled, placeholder, widget, required, validationRules, defaultValue, visibility) apply to the schema node the pointer resolves to — `label`/`description`/`enumNames` write into the resolved `properties` node, `required` merges into that node's parent schema, and the `uiSchema` branch is created along the pointer path (array segments use rjsf's `-` convention). Hidden pointer fields get `ui:widget: "hidden"` at their nested branch and their `required` retracted from the host schema; submit projection deletes them by path (`deleteAtPath`), and value seeding writes nested `defaultValue` by path (`setAtPath`). Boundaries: pointer keys are excluded from `ui:order` (rjsf accepts root property names only); value delete/set on array-item paths is a no-op because arrays submit and project as a whole; the frontend `derivePresentationSpec` now emits pointer keys for nested fields that carry hints (no hints → the nested structure renders as one whole field, unchanged); the Go-side nested derivation aligns with this in SUI-025. A pointer that resolves to no schema node is reported with `console.warn` (`presentation field path unknown: <key>：<reason>`) instead of silently degrading; promoting that to a save/publish-time blocker lands in SUI-021.

`widget` is a governed enum of antd/ProComponents component names (PascalCase): `Input`, `TextArea`, `InputNumber`, `Password`, `Select`, `MultiSelect`, `Radio`, `Checkbox`, `Switch`, `DatePicker`, `TimePicker`, `DateRange`, `Upload`, `ImageUpload`, `FileUpload`, `RichText`, `Code`, `Cascader`, `TreeSelect`, `Color`, `Slider`, `Rate`, `JSON`, `KeyValue`, `Array`, `Object`. Extending widgets requires modifying the spec package and syncing the frontend types; the frontend may not add them privately.

### Widget capability table (SUI-020)

The renderer's `SchemaFormRenderer.widgetToRjsf` maps every `FormFieldSpec.widget` value to an RJSF widget/field. The shared table `web/src/utils/widgetCapabilities.ts` (`WIDGET_CAPABILITIES` + `widgetCapability(widget)`) records the ground truth for all 26 enum values: accepted JSON Schema types, the submitted value shape, and the fallback behavior when types do not match or the widget is unimplemented. It is typed data only — no rendering changes. Honest boundaries, verified by `web/src/utils/__tests__/widgetCapabilities.test.ts`:

- **Concrete implementations** (`implemented: true`): `Input`/`TextArea`/`InputNumber`/`Password`/`Select`/`MultiSelect`/`Radio`/`Checkbox`/`Switch`/`DatePicker`/`TimePicker`/`Color`/`Slider` use the mapped RJSF widget; `Switch` maps to the RJSF `checkbox` and still submits a boolean; `Code` and `JSON` map to `textarea` and submit a plain string (no parsing, no highlighting); `Cascader`/`TreeSelect`/`Rate` are registered custom widgets; `Upload`/`ImageUpload`/`FileUpload` share the custom `upload` widget; `KeyValue` joins via the custom `keyValue` field; `Array`/`Object` are renderer-managed composite fields that ignore `ui:widget`.
- **Declared-only** (`implemented: false`): `DateRange` and `RichText` have no implementation and no custom widget registered; `widgetToRjsf` returns `undefined` for them, so the UI falls back to the type default (plain input or textarea per schema type) and the declared intent is silently ignored.
- The Page Studio widget selector's 12 options are a subset of the declared capabilities and now carry a native `title` hint (accepted types + value shape + fallback) per option; option values are unchanged.

Extending widgets still requires modifying the spec package and syncing the frontend types; the capability table must be updated in the same change, and the table asserts full enum coverage so a new enum value cannot land without a documented capability.

### Form presentation validation (SUI-021)

Save and publish validate every `FormPresentationSpec` carried by the PageSpec — `resource.createForm`, `resource.updateForm`, `resource.actions[].form`, `operation.form`, `task.form`, `report.queryForm`, and every `composite.sections[].form`. Rules:

- **Field keys**: non-empty, unique, and resolvable in the form's `jsonSchema` (legacy bare keys and JSON Pointer keys follow the SUI-019 contract). Diagnostics locate `…form.fields[i].key`.
- **Groups**: group keys are non-empty and unique; every group field must be declared in `form.fields` and resolve in the schema; the same field cannot be assigned to two groups (`…groups[i].fields[j]`, `…conflict`).
- **Widgets**: the value must be one of the 26 governed enum values, and its accepted schema types (from the SUI-020 capability table) must match the resolved field schema type; declared-only `DateRange`/`RichText` are not rejected merely for lacking an implementation. Missing/unrecognized schema types skip the compatibility check.
- **Width**: `0` (unset) or `1–12` on the 12-column grid.

Enforcement is one rule source: `spec.ValidateFormPresentations` is appended by `ValidatePublishablePageShape` (publish, accept-and-publish, rollback preflight, proposal validation) and by `ValidatePageSpecInScope` on the save/preview path; the client reuses `web/src/utils/fieldPath.ts` and `web/src/utils/widgetCapabilities.ts` in `web/src/utils/formValidation.ts` before `savePageDraft` and `CompositeEditor.save()` so invalid configuration never reaches the server. Errors use the existing `{field, message}` details contract. Regression tests include a fully valid fixture that still publishes.

### Date and time value serialization (SUI-022)

Date-type form fields submit calendar values, not UTC-folded instants. The antd theme's built-in `DateWidget` serialized `date-time` with `toISOString()` — the picker's local offset was discarded in favor of a UTC `Z` suffix — and passed `null` through on clear, which survives rjsf v6 `formData` (only `undefined` removes a key) and then fails AJV `type: string`. `@rjsf/core`'s `TimeWidget` (the antd theme ships none) builds the value by string concatenation `${value}:00`, which mangles the antd TimePicker's Dayjs values. The renderer therefore registers three overrides under the rjsf widgetMap registry keys `DateWidget` / `DateTimeWidget` / `TimeWidget` in `web/src/components/SchemaFormRenderer/widgets.tsx`; one registration covers both the `format`-default path (`format: date` / `date-time` / `time` resolve to the same-named registry key) and the explicit `x-widget` path (`DatePicker` → `date`, `TimePicker` → `time` via `widgetToRjsf`).

Serialization contract (`web/src/utils/datetime.ts`):

- `date` → local calendar date `YYYY-MM-DD` (no time, no offset);
- `date-time` → RFC3339 with an explicit local offset, `YYYY-MM-DDTHH:mm:ssZ` (dayjs's `Z` token emits `±HH:MM`, e.g. `2026-01-31T08:30:45+08:00`); unlike the previous `toISOString()` form the operator's wall clock and offset are preserved, the instant itself is unchanged and losslessly convertible (UTC consumers parse the same moment), precision to seconds;
- `time` → `HH:mm:ss`;
- clearing a value submits `undefined` and the key is removed from the payload — the SchemaFormRenderer normalize layer filters `undefined` entries before folding `formData` (previously a stale `null` leaked back into payloads).

Display seeds (`initialValues` / `defaultValue` / previously submitted values) are normalized in the same module before reaching a picker: naive strings without a timezone designator (DB-style `2026-01-01T08:00:00`) parse as local wall clock and are never silently shifted, while `Z`/offset seeds keep the instant (dayjs default semantics) — in a non-UTC browser timezone the displayed calendar day may be an adjacent day; that is documented behavior (keeping the instant, not the original calendar day), asserted by TZ-guarded tests. `normalizeDateTimeSeed` is the single hook point if seed interpretation ever changes. Time seeds accept `HH:mm`/`HH:mm:ss` only; out-of-range values render empty rather than rolling over.

Boundaries: the serialization contract follows the widget, not a separate option — `x-widget: DatePicker` on a `format: date-time` field submits the date-only form (and vice versa for `TimePicker`); UTC consumers get an identical instant after parsing; cross-day display of `Z`/offset seeds in non-UTC timezones is intentional; `DateRange` remains declared-only (SUI-020). Verification: real rjsf engine e2e (`datetime-widgets.test.tsx`) asserts submitted strings and clear behavior through the actual RJSF submit path under both UTC and non-UTC process timezones.

### Enum label association (SUI-023)

Enum dropdown labels attach to values, never to positions. The GUX-017 rendering chain (schema `x-enum-options` / `FormFieldSpec.enumOptions` → `ui:options.enumNames`, because rjsf v5's `optionsList` consumes only `ui:options.enumNames`) is unchanged, but the label table is now built by matching `String(value)` keys against the schema's `enum` array instead of relying on declaration order:

- **Scalar values are equal**: hint/option `value` accepts strings, numbers, and booleans (previously non-string hint values were silently dropped, so numeric enums fell back to raw values); `false` associates normally — it is a value, not an empty state.
- **Reordered options**: `x-enum-options` or `enumOptions` entries may be listed in any order; labels follow the schema `enum` order because the association is keyed by value.
- **Missing labels/translations**: an enum value without a matching entry renders the raw value string (never a fabricated label); a hint whose value is not in `enum` is ignored and cannot desynchronize the table; the emitted `enumNames` array always has the same length and order as `enum` (rjsf matches by index, so length alignment is the correctness precondition). A label declared with only `zh-CN` renders that label (the GUX-017 locale precedence, `zh-CN` first, is unchanged; locale-following for all form copy is SUI-024).
- **Submitted values are untouched**: labels live only in `ui:options.enumNames`/schema `enumNames`; `schema.enum` is never rewritten. A real-rjsf submit-path e2e (`enum-labels.test.tsx`) selects an option by its visible label and asserts the payload carries the raw enum value (numeric `2`, not the label text).

Without a schema `enum` the `fields` path falls back to declaration order (legacy contract, unchanged behavior).

### Form copy follows the current locale (SUI-024)

All form presentation copy — field titles, descriptions, placeholders, group titles, enum dropdown labels, and the submit-button text — renders in the platform's current locale. The previously hardcoded `zh-CN` locale argument in the renderer's presentation paths is replaced by a single locale source (`web/src/utils/uiLocale.ts` `currentUiLocale()`): umi runtime returns the current platform locale, and any non-umi context falls back to `zh-CN`. The unified fallback chain is unchanged (`localizedText`: current locale → zh/en cross-fallback → legacy short keys → any non-empty → the explicit fallback argument, e.g. the field key for a missing label, the raw enum value for a missing label entry).

Locale switching without page reload (umi `setLocale(lang, false)`) broadcasts a window `languagechange` event; the SchemaFormRenderer subscribes and recomputes the derived schema/uiSchema so visible copy switches live. Filled data is **not** reset by a locale switch: the renderer's form state is the live value mirror, a re-derivation only changes presentation metadata, and the initialValues reseed logic is keyed on the initialValues JSON (untouched by locale). The default reload-style switch reloads the whole page and does not involve this mechanism.

Boundaries: copy declared with only `zh-CN` renders the zh text in an en UI (cross-fallback, no invented translations); static schema-embedded copy (`title`/`description` written directly into `jsonSchema` by the generator) is served as-is — locale-following applies to the presentation layer (`FormPresentationSpec` labels, placeholders, group titles, enum hints) that sits on top of it; templates that embed their own copy follow the platform locale through the same chain elsewhere.

### Default form derivation parity (SUI-025)

When a function contract ships no form presentation config, the form is not lost — it is derived, and the derivation is one contract on both sides. The Go generator (`internal/dashboard/generator` `deriveFormPresentation`) and the web debug page (`web/src/utils/schemaHints.ts` `derivePresentationSpec`) share one expected-result fixture (`testdata/form_presentation_parity.json`); both sides' test suites read the same file and assert identical output per case, covering top-level field order, widgets, groups, and layout hints.

Derivation rules (identical on both sides):

- **Field order**: top-level properties emit in JSON declaration order, then stable-sort by declared `x-order` (any finite number; undeclared sorts last, insertion order breaks ties). Nested object/array-item properties emit only when they carry hints, located by JSON Pointer (`/address/city`, `/tags/-/name`, `~0`/`~1` escaping, depth-capped). A schema carrying any hint emits all top-level fields plus the hinted nested children; a schema with no hints emits none.
- **Widgets**: a valid `x-widget` overrides; otherwise the type-default table fills in without changing the hint state — `integer`/`number` → `InputNumber` (an integer with `enum` still derives `InputNumber`; declare `x-widget: Select` to get the dropdown), `boolean` → `Switch`, `format: date`/`date-time` → `DatePicker`, `format: time` → `TimePicker`, `maxLength > 120` → `TextArea`, array with `items.enum` → `MultiSelect`, `enum` → `Select`.
- **Groups**: declared via `x-ui-groups` (localized title, `collapsible`/`collapsed`), fields joined by `x-group`; an auto-group appends when `x-group` names an undeclared group (title is the humanized key); groups that end up with no fields are dropped.
- **Layout**: `x-ui-layout` validates to `vertical` (default) / `horizontal` / `inline` / `grid`; invalid values fall back to `vertical`.
- **No configuration = the standard vertical form**: a schema with no hints at all emits no `fields` and no `groups` — the wire carries only `layout: vertical`. The renderer fills the gap (`SchemaFormRenderer` → `deriveDefaultPresentationSpec`): a fields-less spec merges the derived standard fields (including type-default widgets) and backfills `layout`/`groups` only when the spec does not declare them. Debug preview and published rendering stay identical because both hit the same fallback. The renderer resolves a derived `DatePicker` to the `date-time` registry widget when the schema format is `date-time`, preserving the SUI-022 serialization.

The derivation no longer emits display fallbacks the renderer already owns: humanized labels (`x-label` > `schema.title` > humanized key covers display) and `请输入X` placeholders are gone from the derived output — a placeholder now requires an explicit `x-placeholder`.

Boundaries: digit-adjacent keys humanize differently on the two sides (Go `Auto 1` vs web `Auto1`) — visible only in auto-group titles; declared `x-order: 0` sorts first on both sides but Go's wire omits `order: 0` (`omitempty` int); non-integer `x-order` values live only in web editor state (Go stores integers); legacy published specs that carry explicit `fields` are untouched — the fallback applies only to specs without fields. Verification: the shared fixture drives `TestFormPresentationParityFixture` (Go) and `form-presentation-parity.test.ts` (web); the renderer fallback is exercised by a real-rjsf e2e (`form-default-fallback.test.tsx`).

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

### JSON editing for non-object-root requests (SUI-026)

Structured form controls can only express an object root. When a function contract declares a non-object root type (`array`, a bare scalar, or `null` — the `degraded` tier of the SUI-011 request-compatibility grading), `SchemaFormRenderer` switches to a JSON editing form (`JsonRootEditor`) instead of failing to render: the seed comes from the SUI-017/018 defaults layer (root array `[]`/items skeleton, scalar type placeholder) or explicit initial values, the text is edited as the whole request payload, and submit goes through the same handle contract (`validate`/`getSubmitValues`/`submit`) as the object form — the same `@rjsf/validator-ajv8` validator runs the payload against the contract schema, and the parsed value is sent verbatim (arrays/scalars never get an object shell, preserving the SUI-017 roundtrip semantics). The UI states the concrete reason (the declared root type) and the actual available entry (this editor). A plain-object initial value is ignored for a non-object root (callers fall back to `|| {}` seeds that can never be valid there) and the schema-derived seed takes over.

The same applies to composite sections: a section whose `form.jsonSchema` declares a non-object root counts as having a form (`sectionHasForm`) and renders the JSON editor instead of the bare execute button; the submit path sends the array/scalar verbatim to the execute chain, and the page-state merge records it as `{ values: <payload> }` only (an array cannot object-spread into flat keys).

Boundaries: the JSON editor covers declared non-object roots only — a `degraded` object root (conditional keywords, tuples, unions) keeps the structured form, and variant rendering belongs to SUI-042/043; `externalErrors` field-level injection does not apply in JSON mode (the whole payload fails as one); the manual tier (missing request mapping) still produces `needs_review` proposals — its concrete reasons surface as `openapi_schema_compatibility*` diagnostics keyed by `functionId` in the OpenAPI Sources operations table (`requestCompatibility` column, tier tag + up to three reason lines) and Functions detail diagnostics.

### Per-instance field IDs (SUI-027)

The renderer generates every field control's id, `label htmlFor`, and error/description `aria` linkage from the rjsf `idPrefix` (`{prefix}_{path}`, default `root`). Two same-schema forms mounted on one page (a composite page with two sections of one contract, or the dual preview) would otherwise produce identical ids and cross-associate labels, errors, and descriptions between instances. `SchemaFormRenderer` allocates the prefix per live instance: the first instance keeps the default `root` (existing `#root_*` page selectors and tests stay valid), concurrent instances claim `root-2`/`root-3`…, and an unmounted instance releases its prefix so a serial re-render reclaims `root` instead of drifting. The prefix is claimed once per instance lifetime (`useRef`) and is stable across re-renders of the same instance; only a remount may change it.

### Form width and responsive layout (SUI-028)

`FormFieldSpec.width` is expressed in a 12-column protocol grid (valid range 1–12); the renderer converts an explicitly declared width to an Ant Design 24-column `Col span` by multiplying by two (`width: 6` → `ant-col-12`). The conversion applies only to this protocol field: `formContext.colSpan` (set to 12 by `layout: grid`) is already an antd column count and stays unchanged, as does the type-derived default (full row for a lone field, `object`/`array`/textarea, or fewer than two fields; half row otherwise). Historical values of `0` (unset) or out-of-range (`>12`, negative) are ignored and fall back to that same precedence — no migration or silent rescaling of old configurations; such configs render with the standard layout until an admin edits them in Page Studio.

Every template column also carries responsive spans: `xs=24` (all fields take a full row below 768px), `md=max(12, ceil(span/2))` (at least half a row from 768px), `lg=span` (full declared width from 992px). Full-row spans (24) stay 24 at every breakpoint to prevent overflow. Groups and long-text fields remain readable and operable in both viewports; field input, validation, and submit bindings are unchanged. Browser coverage runs against the mock operation page at desktop and 375px widths.

### Editing form groups in Page Studio (SUI-029)

Page Studio's `FormPresentationEditor` exposes the full `FormGroupSpec` list for every form-bearing page (operation, task, report, resource and composite forms): groups can be created, renamed (key and localized title), reordered by drag handle, and deleted; each group's field membership is a multiple-select over the declared `form.fields` (in selection order), with fields already claimed by another group disabled to prevent `form_group_field_conflict`; collapsible/collapsed switches set the two optional behavior flags. The edited list is written back as `FormPresentationSpec.groups` in the same draft and is read back verbatim (including order) when the draft is reopened — no separate editor-only state. Save/publish still run the SUI-021 validation (unique keys, known fields, single owner per field), so invalid edits surface as located diagnostics rather than silently landing in a published page. Boundary: the editor edits presentation only (schema, binding, and payload are untouched), and group membership selection is constrained to fields declared in `form.fields` — nested fields need a declared pointer-key field entry first.

### Display-vs-call parameter boundary (SUI-045)

The form an operator sees and the request that is finally sent are two different objects, joined by three orthogonal responsibilities:

| Layer                  | Content                                                                             | Responsibility                                                                                                     |
| ---------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Base form (jsonSchema) | `FormPresentationSpec.jsonSchema` (derived from the function contract input schema) | Source of truth for the call: field set, types, `required`, constraints                                            |
| Display overrides      | per-field `fields[]` overrides + `layout`/`groups`/buttons                          | Operator input surface: how fields are presented and which are visible/enabled                                     |
| Binding                | `PageBinding.selectors.input.assignments` (Selector AST)                            | Call value mapping: maps form/row/selection/detail/page_state/literal sources onto payload targets (`$.` pointers) |

Display overrides never modify the jsonSchema, and bindings never modify the presentation. `SchemaFormRenderer` consumes the first two to produce the input surface; on the server `buildBindingPayloadFromSelectors` (`internal/api/console/console_execute.go:523`) consumes the binding and the execution context to produce the request payload — the payload is built **only** from selector assignments; a field without an assignment never reaches the payload.

**Two-way split of `FormFieldSpec` fields:**

- **Display-only (no effect on the call):** `label`/`placeholder`/`description`/`role`/`unit`/`perTarget`/`widget`/`width`/`order`/`enumOptions`/`remoteOptions`/`widgetProps`, plus page-level `layout`/`groups`/`submitButton`/`cancelButton`. `layout` illustrates the boundary: `deriveRuntimeSchema` maps it only into `formContext` layout config (`web/src/components/SchemaFormRenderer/index.tsx:480-500`) and never touches the schema, validation, or payload — **under the same schema and the same binding, standard and compact layouts produce identical requests** (the design premise of SUI-031/036).
- **Call-affecting (changes values or validation):**
  - `defaultValue` — seeds form initial values: fills only `undefined` fields, binding injection/`initialValues` wins, `null`/empty string count as provided and are not overwritten (`applySpecDefaults`, `index.tsx:412-424`);
  - `visible`/`visibleWhen`/`disabled` — visibility (see below);
  - `required`/`validationRules` — merged into the runtime schema: `required: true` adds the key, `required: false` revokes a schema-level required, `min`/`max` map to ajv keywords per type, `pattern` is written directly, `custom` is explicitly warned and ignored (`mergeFieldValidity`, `index.tsx:352-408`).

**Three exits for hidden fields** (the SUI-015 boundary):

1. **Value is retained:** a hidden field (unsatisfied `visibleWhen`, or `visible: false`, via the unified `isFieldHidden` predicate) stays in the schema as `ui:widget: "hidden"`, its value stays in form state; the submit payload goes through the single projection `projectSubmitValues` (SUI-014's unified submit-value rule, `index.tsx:445-450`) — hidden keys are dropped per current values, everything else is kept verbatim; native submit and the imperative `getSubmitValues()` (`index.tsx:661-665`) share that one rule, while `getValues()` returns every value including hidden ones and is for state mirroring only (`index.tsx:680-688`).
2. **Binding-reachable:** a hidden field's value can enter the payload through a binding `{ kind: "form", path }` — the server reads it by path from execution context `context.form` (`resolveSourceValue`, `console_execute.go:593-594`); in the browser `projectBindingContext` narrows the context to the fields actually referenced by the published selector (`web/src/components/PageRenderer/runtime.ts:78-120`), so a hidden field referenced by a selector still ships with the narrowed result. **A hidden required field whose value comes from a binding is executable.**
3. **Server-side backstop:** the hidden-required exemption applies only to the **client** runtime schema (removed from `required`, `index.tsx:538-540`); the server validates the final payload _after_ bindings are applied — `validateBindingExecutePayload` returns a structured 422 for payloads that do not match the input schema (`console_execute.go:512-544`). **A hidden required item with no source at all (no assignment, no literal/default-transform fallback) therefore gets a field-level structured error on the server** and is never silently dropped and sent as a wrong request. Since SUI-016 the verdict engine and the unknown-field policy are shared with the direct function invoke path: a missing schema passes through, a schema that cannot compile (or a non-JSON payload) is rejected as a schema/payload defect instead of being treated as absent, and unknown fields follow standard JSON Schema semantics (accepted unless the schema declares `additionalProperties: false`) — the same fixtures lock both entries.

**Final validation order** (already the implemented order; unchanged after SUI-014's unified submit-value interface):

- Client: binding injection (dialog params / row injection / cascade, passed as `initialValues`) → `applySpecDefaults` seeds → `deriveRuntimeSchema` (visibleWhen exempted from required → `mergeFieldValidity` override merge → required strings get `minLength: 1`) → AJV submit validation → the single projection `projectSubmitValues` drops hidden keys → `onFinish`. Injection **overwrites** rather than fills gaps: when the injected JSON changes the whole form resets to the seeded values (`index.tsx:633-639`).
- Server (`executeBindingFromSnapshot`, `console_execute.go:186-202`): executor bound → binding fresh → governance → **first** `buildBindingPayloadFromSelectors` applies all bindings and default transforms → **then** `validateBindingExecutePayload` validates the final request → idempotency claim → approval gate → dispatch. That is "validate the final request only after bindings and explicit defaults are applied."

**Legacy compatibility:** the four override classes — `required` (including `required: false` revocation), `defaultValue`, `visible`/`visibleWhen`, and `validationRules` — keep their current semantics (locked by `web/src/components/SchemaFormRenderer/__tests__/field-validity.test.tsx`); this section adds no wire fields and is purely a semantic clarification. The split also constrains the display-variant protocol (SUI-031): a variant may override display-only fields only; call-affecting fields are part of the call contract and no variant may change them.

**Examples** (one function `player.ban`, schema with `playerId *`, `env *`, `reason *`, `notifyDays`):

- **Standard vs compact layout:** the two variants differ only in `layout` (`vertical` vs a compact grid) and `width`; schema, call-affecting field overrides, and binding are identical → the same input produces the **same** request `{ "playerId": ..., "env": ..., "reason": ..., "notifyDays": ... }` under both layouts.
- **Manual entry vs selected-row injection:** the same target `$/playerId` via two sources — manual entry `{ kind: "form", path: "/playerId" }` (a form field the operator types into); row injection `{ kind: "row", path: "/id" }` (a row-action dialog seeds the selected row's `id` as the form's initial values; the published runtime `mapRowParams` → `openDialog`, `web/src/components/PageRenderer/compositeRuntime.tsx:1063/865`; the editor preview aligns via `openDialogPrefill`, `PreviewRuntime.tsx:104`). Both go through the same value path: the injected value is seeded as `initialValues`, and a later user edit wins.
- **Missing required source:** `env` is schema-required, the display override hides it with `visible: false`, and no assignment provides it. The client exempts and strips it → the payload lacks `env` → the server returns 422:

```json
{
  "error": "validation_failed",
  "message": "binding payload does not match input schema",
  "details": {
    "bindingId": "ban-1",
    "functionId": "player.ban",
    "schema": "inputSchema",
    "error": "payload does not match schema: jsonschema validation failed with 'file:///...schema.json#' - at '': missing property 'env'"
  }
}
```

### Named display variants (SUI-031)

One form can carry several named presentations — the canonical pair is a standard vertical form and a compact grid — without forking the call contract. This section defines the protocol: storage (SUI-032 — variants ride inside the form node through the existing draft/snapshot `SpecJSON` blob with zero table changes), publish validation (SUI-033 — variant shape rules and current-contract freshness, proven by spec-level and service-level tests including the rollback preflight), and Page Studio management (SUI-034 — the form tab of operation/task/report/resource pages manages named styles: add/rename/delete, set default with rename/delete sync, per-field width/widget overrides, per-style preview; button/group/label overrides and variant-only fields remain API-authored — see the usage doc) have landed, while default resolution (SUI-035) and published-form switching (SUI-036) are still pending — until those land, the rendered form ignores `defaultVariant` and offers no style switch; what follows is the contract they implement.

**Stable form identity.** Every form-bearing position in a PageSpec is addressed by a canonical path built from stable segment keys, never array indexes: `resource.createForm`, `resource.updateForm`, `resource.actions.<actionKey>.form` (ActionSpec `key`), `operation.form`, `task.form`, `report.queryForm`, and `composite.sections.<sectionKey>.form` (CompositeSection `key`). Reordering sections or actions does not change the identity. Array-index paths (e.g. `composite.sections[2].form.fields[0].width`) remain the diagnostics convention for validation errors, not identity. Variants live inside the form node they overlay, so a variant inherits its form's identity and can never dangle across drafts, snapshots, or rollbacks.

**Wire shape (additive; `page-spec:1` unchanged).** `FormPresentationSpec` gains two optional fields:

```ts
interface FormPresentationSpec {
  // …既有字段（jsonSchema/layout/groups/fields/submitButton/cancelButton）…
  variants?: FormVariantSpec[]; // 命名展示变体（SUI-031）；数组顺序 = 管理列表顺序
  defaultVariant?: string;      // 生效默认变体 key；未声明 = 基础呈现（原表单）生效
}

interface FormVariantSpec {
  key: string;            // 变体 key：本表单内唯一
  title?: LocalizedText;  // 操作者可见名称（风格列表 / 切换 UI）
  layout?: "vertical" | "horizontal" | "inline" | "grid";
  groups?: FormGroupSpec[];
  fields?: FormVariantFieldSpec[]; // 逐字段纯展示覆盖（键同 SUI-019）
  submitButton?: FormButtonSpec;
  cancelButton?: FormButtonSpec;
}

interface FormVariantFieldSpec {
  key: string;
  label?: LocalizedText;
  description?: LocalizedText;
  placeholder?: LocalizedText;
  role?: "identity" | "status" | "reason" | "expiry" | "amount" | "quantity";
  unit?: LocalizedText;
  perTarget?: boolean;
  widget?: FormWidget;
  width?: number; // 1-12，同 SUI-028
  order?: number;
  enumOptions?: EnumOption[];
  remoteOptions?: RemoteOptionsSpec;
  widgetProps?: Record<string, JSONValue>;
}
```

The variant field type structurally omits `required`/`defaultValue`/`visible`/`visibleWhen`/`disabled`/`validationRules` — call-affecting overrides cannot be expressed in a variant (see below). A renderer predating these fields ignores them and renders the base presentation (additive optional fields; the runtime never downgrades, guesses, or converts — ABI rules unchanged).

**What a variant may override.** Exactly the SUI-045 display-only set: per field `label`/`description`/`placeholder`/`role`/`unit`/`perTarget`/`widget`/`width`/`order`/`enumOptions`/`remoteOptions`/`widgetProps`; page-level `layout`/`groups`/`submitButton`/`cancelButton`. The call contract is not overridable by any variant: `jsonSchema`, `required` (both directions), `validationRules`, `defaultValue`, `visible`/`visibleWhen`, `disabled`, and the form's `PageBinding` assignments. Publish validation rejects call-affecting keys inside a variant with a located error (`form_variant_illegal_override`), so a variant can never change what is submitted, how it is validated, or how the call is governed.

**Overlay semantics.** The base presentation is the existing `FormPresentationSpec`, unchanged and always effective. The effective presentation of variant V is the base overlaid by V:

- `jsonSchema` and every call-affecting override always come from the base.
- `layout`/`groups`/`submitButton`/`cancelButton`: present on V replaces the base value wholesale; absent on V inherits the base.
- `fields`: per-key merge. A V entry overlays the display-only properties of the base entry with the same key (V wins per declared property; properties V does not declare keep the base value). Keys declared only on V are legal when they resolve in `jsonSchema` (SUI-019 rules); the rest of their presentation comes from schema defaults. Base-only entries pass through untouched.
- Removal is not expressible: a variant cannot retract a base display override (no null-to-delete semantics in v1). To "undo" a base override for one variant, correct the base — this keeps the merge rule predictable.

A variant that declares nothing but `key` (plus optionally `title`) is legal and renders identically to the base — the recommended way to name the "standard" style alongside a compact one. There are no reserved variant keys; "standard"/"compact" in acceptance criteria are ordinary user-chosen keys.

**Default variant.** `defaultVariant` names the variant that renders when the operator has not chosen one. When set it must resolve to a declared variant key, else publish fails with a located error (`form_variant_default_unknown`, SUI-033); when unset — including when `variants` is empty or absent — the base presentation is the effective default, and `defaultVariant` set on a node without variants is itself a located error. The base presentation is therefore always available and unnamed: it is the zero-config floor.

**Shared everything that matters.** All variants of one form share one `jsonSchema`, one set of bindings, and one call governance (permission, risk, approval, danger confirmation, idempotency — all attach at binding/function level, none at variant level). Under the same schema, binding, and input, every variant of a form produces the identical request payload; switching variants is a display operation by construction (the SUI-045 premise, now enforced by the allowed-override set). This is also why switching preserves the operator's input and error state: form state is keyed by schema paths, which variants never touch.

**Resolution semantics (SUI-035 implements).** The effective variant of a rendered form is, in order: the operator's explicit in-session choice (runtime UI state, SUI-036) → the published `defaultVariant` → the base presentation → `deriveDefaultPresentationSpec`'s standard vertical form (a base form carrying no `fields`, per SUI-025). A key that fails to resolve at runtime (stale cache) falls down the same chain; publish validation makes that unreachable in practice.

**Publish snapshot and rollback.** The variants and `defaultVariant` live inside the form node and therefore inside the same publish snapshot — one digest, existing tables, no side stores (SUI-032). Rollback restores the previous snapshot atomically: its variants and default come back exactly as published, never merged across versions. Rollback preflight already re-runs `ValidatePublishablePageShape` (SUI-021); SUI-033 extends that validation to variants and to the **current** function contract, so rolling back a snapshot whose variants reference a since-removed or renamed contract field fails preflight with located errors naming the affected variant — a stale snapshot is surfaced, not silently published.

**Old pages and new pages.** Pages published before this protocol carry no `variants` and keep rendering their original presentation verbatim — no pseudo-variant is invented for them. A newly generated page with no variant config emits no variants either; its base form without `fields` is the derived standard vertical form (SUI-025), which is this protocol's zero-config default. The operator's selected variant is runtime display state; the protocol persists only the published default — per-user/per-session persistence of the choice is out of scope.

**Validation rules (implemented in SUI-033).** `ValidateFormPresentations` runs the variant checks on every validation path (save/preview, publish, accept-and-publish, proposal, versioning-rollback preflight) under the existing `{field, message}` details contract: variant keys non-empty (`form_variant_key_missing`) and unique within the form (`form_variant_key_conflict`); `defaultVariant` resolution as above (`form_variant_default_unknown`); variant field keys non-empty (`form_variant_field_key_missing`), non-duplicated (`form_variant_field_duplicate`), and resolvable in the form's `jsonSchema` (`form_variant_field_unknown`); call-affecting keys on a variant field are located even though `encoding/json` would silently drop them — `FormVariantFieldSpec.UnmarshalJSON` audits every key outside the display-only allow-list into `UnknownKeys`, and validation reports `form_variant_illegal_override` naming the offending keys; `widget` validity/type compatibility and `width` range under the same SUI-020/SUI-021/SUI-028 rules as base fields, plus variant group rules mirroring the base (`form_group_key_missing`/`form_group_key_duplicate`/`form_group_field_unknown`/`form_group_field_duplicate`/`form_group_field_conflict`, where a variant group field may be declared in the base fields or the variant's own fields). The web client mirrors the same rules in `web/src/utils/formValidation.ts` (`unknownVariantOverrideKeys` plays the role of the Go `UnmarshalJSON` audit) so drafts are checked before save.

**Contract freshness (SUI-033).** Shape validation above resolves variant keys against the form's own `jsonSchema` — which is only as current as the draft. Two publish-path checks close the gap against the **current** function contract (`ValidateVariantContractFreshness`, run wherever `ValidatePageSpecInScope` runs with `publish=true`, including the versioning-rollback preflight): after a contract field is deleted or renamed, the draft's frozen schema still resolves the stale variant key, so publish re-resolves every variant field and group-field key against the current input schema of the function the carrier form executes and fails with `form_variant_contract_stale`, naming the affected variant key and function. Carrier→function follows the structural references of the page model: resource action forms via `actions[i].bindingId`, composite form sections via `sections[i].bindingId` (static sections skipped), and `operation.form`/`task.form`/`report.queryForm` via the page's single `action`/`task`/`report` usage binding; ambiguous or missing mappings are skipped rather than guessed. Known boundary: `resource.createForm`/`updateForm` have no structural function reference in the spec, so their variants are checked against the form schema (shape + regeneration) but not the current contract — their submission wiring is generator-internal. Scoping needs nothing new: variants live inside the page snapshot, already scoped by (game, env) + pageKey — the same pageKey in different games/envs has independent variants.

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

- `table.columns[].unit` / `columns[].enum` / `columns[].render` / **explicit field semantics** (D5) — the output side of the `x-*` presentation hints. The game SDK declares semantics on the contract output schema's item properties (mechanically parsed, never guessed from field names): `x-field-role` (`identity|status|reason|expiry|amount|quantity`) and `x-field-unit` (a bare string or a `{zh-CN,en-US}` object), alongside the existing `x-enum-options`. The generator collapses these into wire fields: `role=identity` → the column gets `fixed:"left"` + `render:"copy"` (a pinned, copyable identity column); `x-enum-options` → the column gets `enum` + `render:"tag"` (a status enum tag column); `x-field-unit` → the column gets `unit`. The input side carries them into `FormFieldSpec.role`/`unit`/`perTarget` (parsed in `form_hints.go` and the frontend `derivePresentationSpec`, invalid roles silently ignored; `x-field-per-target` declares the field per-target only when exactly `true`, any other value is not declared). Rendering: the composite renderer and the resource renderer append the unit to the column header (`标题 (单位)`) and to `number` cell values; `render:"copy"` renders a copyable cell; `dataType:"datetime"` cells render with an explicit browser-local timezone label (`UTC±HH:MM` — the platform does not declare game-server timezones, so the label makes the offset explicit instead of implying UTC); `enum`+`tag` renders the option label as a colored antd Tag. The confirm dialog (D2 danger confirmation) picks the semantics up from the target form's `FormFieldSpec.role`/`unit`: identity entries are sorted first and bolded, expiry entries render in danger red, amount/quantity values gain their unit suffix, so the operator reads who/what/how-much/how-long before confirming. Boundaries: units are display-only (no amount scaling or conversion — a declared unit is appended verbatim, except that a field declared `x-field-per-target` additionally shows a batch total in the D2 danger confirm when multiple targets are selected); roles other than `identity` do not change column layout (status/reason/expiry drive confirm emphasis and future defaults only); the timezone label is the browser's local offset (no game-level tz configuration); composite column semantics ride on the same generator derivation (a composite table section bound to a contract with hints gets the same columns), and a dialog-rendered composite table keeps the semantic cell rendering but no header sort UI; enum option labels and units follow the current UI locale (the deployed zh-CN/en-US set; other locales fall back to zh-CN) instead of a fixed zh-CN; an enum value outside the declared option set renders an explicit unknown marker that keeps the raw value readable (a default-colored tag or warning text — not color-only signaling), and an unparseable datetime renders an explicit invalid marker instead of an `Invalid Date (UTC±xx)` concatenation (the shared formatter likewise passes unparseable input through raw) (GUX-005/006/007)

- `danger` + `chain` (rowActions / toolbar.actions): **the dangerous-action confirmation contract** (D2) — a `danger` action never executes before an explicit confirmation. With a `targetSection` the confirm body lists the modal's prefilled parameters and the chain steps, and only then does the modal open and the chain run; without a `targetSection` (a pure action-chain button) the confirm body lists the chain steps. The confirm body renders parameters by the target form's field titles, maps enum values to their option labels, and shows parameter names humanized rather than raw field keys, so the reviewed content matches what will actually be submitted. A non-danger action with a `targetSection` opens the modal and runs its chain immediately (a detail modal uses its chain to preload its own fields/table data); a non-danger action without a target runs its chain immediately. With D5 the confirm body additionally carries the target form's field semantics (`FormFieldSpec.role`/`unit`): identity entries sort first and render bold, expiry entries render in danger red, and amount/quantity values carry their declared unit suffix. Boundaries: the emphasis covers identity/reason/expiry/amount/quantity declared via `x-field-role` (fields without a role render as before), `openModal` chain steps carry no danger flag (the wire has no such field) so a chain step that opens a modal does not confirm, the confirmation happens before the modal opens and edits inside the modal that change the submitted payload re-confirm against the final values before submit (unchanged edits do not re-prompt), and a field declared `x-field-per-target` additionally shows a batch total (`value × selected-target count`, unit/role reused) in the danger confirm when the source table selection has more than one target — only declared per-target fields aggregate, non-declared fields never multiply (no semantic guessing)
- **Execution result feedback** (D4, no new spec field): the renderer classifies each execution result by `PageExecutionResult.kind` instead of treating any non-error return as success. A `sync` (or kind-less) result is business success — it fires the form's `success` event binding, re-runs the `onSuccessRefresh` targets, and closes the dialog. An `approval` result keeps the dialog open with a "submitted for approval" notice showing the `approvalId` and a deep link to the approval center (`/approvals?approvalId=...`), and fires neither the success event nor the refresh (the business side has not changed until it is approved). A `task` result keeps the dialog open with a "task submitted" notice showing the `taskId`. A thrown error keeps the dialog and its entered values open and renders the failure inline: the stable `error` code from the response body, the server `message`, and the field-level entries of `details` mapped to the form's field titles (raw field keys are not shown); an error returned in the result body rather than thrown (a top-level `error` field) is treated as a failure too and fires no success event. Permission-denied failures are normalized server-side to `403 permission_denied` with machine-readable `details` (`required` permissions, `allowedRoles`, `functionId`, or task scope); the renderer surfaces those values as a compact reason line instead of only the generic status copy. Field-level `details` entries whose field exists in the section's form schema are injected under the matching control through the shared form renderer's `externalErrors` prop (RJSF `extraErrors`), so only form-external fields remain in the failure summary. Boundaries: an inline (non-dialog) form section enters a loading skeleton while submitting and loses its entered values on a failed submit (dialog forms keep them), the operation page's task result panel carries an explicit "go to task center" link (`/ops/jobs`) alongside the "query execution status" entry, and the approval link is a fixed `/approvals` route
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

For the `query` usage (a resource page's list binding), the input assignments' form-source paths are validated against a derived **list query schema** = `listView.filters` keys ∪ `{current, pageSize}` (the latter two only when `pagination.enabled`); form paths outside this set fail structural validation at save/publish (GUX-017: this forces the query bar to be wired — a list binding without form assignments would otherwise render a query bar that never filters). The resource query bar itself renders exactly the declared `listView.filters` plus the pagination fields, so the validated form-source set and the visible query form stay in lockstep.

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
- Form presentation changes must never silently alter a published page; a publish snapshot must include the FormPresentationSpec and its named display variants (`variants`/`defaultVariant`, SUI-031) — rollback restores them atomically and re-validates them against the current contract.
