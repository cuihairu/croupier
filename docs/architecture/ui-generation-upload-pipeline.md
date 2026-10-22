---
title: "Upload to page: orthogonalizing contract and binding"
icon: rocket
order: 10
category:
  - System architecture
tag:
  - OpenAPI
  - Page Studio
  - Design
---

# Upload to page: orthogonalizing contract and binding

> **Status**: Current -- D1–D7 have all landed (todo.md T1–T10, T12, T13; T11 wrapping up). This document is the authoritative design reference for the orthogonalized model.
> Prerequisite reading: ProComponents page generation and runtime, the [Dashboard resource/page model](./dashboard-page-model.md), and [OpenAPI / SDK Descriptor v2](./openapi-sdk-descriptor-v2.md).

## 1. Problem statement: why "the document uploads but no page is generated"

The existing pipeline couples **"what the interface looks like" (the contract)** with **"whether the interface can execute" (the runtime binding)** as prerequisites of each other:

```text
现状（耦合）：
  OpenAPI 上传 → operations
    → Binding 要求 functionId 已运行时注册，否则 400    ← 断点①
    → FunctionContract → 页面提案
  组件模板只能手动 POST /component-templates/regenerate  ← 断点②
  组合页强制 ≥2 函数区块，单组件页面被禁止                ← 断点③
  全部保存都必须走 提案→接受→发布 双重门禁               ← 断点④
```

Consequence: uploading an OpenAPI document produces not a single component or page; the user must hand-relay across four pages — OpenAPISources / ComponentTemplates / PageStudio inbox / CompositeEditor — and stalls at any step they do not know to perform.

The root cause is not missing implementation but a misplaced model:

- **Binding is an attribute of a contract, not a precondition of it.** The UI shape of an interface (form fields, table columns, actions) depends only on the schema, not on "whether an agent happens to be online".
- **Review gates should be risk-tiered, not one-size-fits-all for every save.** Proposal → review → publish is a governance mechanism designed for "pages generated automatically from contracts", applied indiscriminately to hand-dragged pages.
- **The ≥2-block constraint** comes from a naming purism of "composite page = Composite", while "one table is one page" is precisely the most common shape of an operations back office.

## 2. Core decisions (no longer subject to change)

- **D1 Everything is material**: whether a contract comes from SDK registration, an OpenAPI upload, or hand-writing, landing it in the database generates component material (component template + function panel entry). UI generation never depends on execution readiness.
- **D2 Binding state orthogonalized**: FunctionContract gains an execution-state dimension, `executionState: bound | unbound`. An unbound component can be dragged onto the canvas and its page published; when a runtime action executes, the server rejects it and returns a structured diagnostic (`executor_unbound`).
- **D3 Automatic binding**: when functions registered by a runtime agent/SDK match an unbound contract on `(game_id, env, function_id)`, it flips to bound automatically, with no manual step.
- **D4 Upload-is-a-pipeline**: an OpenAPI upload is one complete generation action; within a single request it completes parse → contracts → component templates → page proposals, and the response carries a generation digest plus direct editor entry points.
- **D5 Tiered publishing**: whether the proposal-review path applies is decided by an env-level policy; dev defaults to save = publish (snapshots and version history still recorded), prod keeps proposal → review → publish.
- **D6 Zero protocol change**: the PageSpec wire contract, renderers, and the permission/audit system are unchanged. This design changes only contract admission conditions and flow orchestration.
- **D7 Copy i18n: default name required, translations optional**: LocalizedText fields such as menu/page titles require only the **default name** (the system default language `systemDefaultLocale`) to be non-empty; every other language (en-US included) is an optional translation. Publish validation no longer forces the zh-CN+en-US dual write (abolishing `hasDefaultLocale`'s dual-key test and `ensureBilingual`'s forced fill-in); the render-side fallback chain is unchanged (current UI language → system default language → any non-empty value). The single editing entry point is the `LocalizedTextEditor` component (already the only implementation in the repo): language options come from the global supported list, users enter translations as needed, and a missing non-default language neither warns nor blocks.
  - **Landing status**: T12/T13 have landed — the backend's three `hasDefaultLocale` checks are unified as "any non-empty locale passes" (still rejected if entirely blank), the generator is renamed `ensureDefaultLocale` and its forced en-US fill-in abolished, the frontend `REQUIRED_LOCALES` is downgraded to `['zh-CN']`; the editor (LocalizedTextEditor) has dropped its dropdown ⚠ markers and the `contractHint` dual-required wording, a missing default language surfaces only as a non-blocking remediation hint inside the 🌐 popover (`defaultLocaleMissing`), and missing non-default languages prompt nothing. The `normalizeLocalizedText` normalization layer has also been relaxed (2026-09-16): inputs are normalized to BCP47 keys and passed through as-is, with no forced dual-key output — bare strings normalize to a single default-language key, other BCP47 keys are preserved as-is.

## 3. Target pipeline

```text
契约来源（SDK / OpenAPI 上传 / 手写）──┐
                                      ▼
                            FunctionContract（统一物料）
                            executionState: bound | unbound
                                      │
              ┌───────────────────────┼───────────────────────┐
              ▼                       ▼                       ▼
        组件模板（自动重建）     页面提案（自动生成）      编辑器拖拽物料
              └───────────────────────┴───────────────────────┘
                                      ▼
                          PageSpec → 发布（分级）
```

### 3.1 The upload pipeline (D4)

```text
POST /api/v1/openapi/sources（上传）
  → 解析 spec → operations + diagnostics（现状保留）
  → 每个 operation：
      有运行时函数 → 现状 provider 绑定路径（不变）
      无运行时函数 → 建 unbound FunctionContract（不再 400）
  → 自动 RegenerateFromContracts（组件模板，不再需要手动按钮触发）
  → 自动产出页面提案（现有 generator 复用）
  → 响应摘要：
      { operations: 24, contractsCreated: 20, templatesUpdated: 24,
        proposalsCreated: 6, diagnostics: [...] }
  → 前端展示摘要 + [打开编辑器] / [查看提案] CTA
```

### 3.2 Closing the loop inside the editor

- Unbound material in the component panel/template library carries an "unbound" marker; dragging is **not** disabled.
- Selecting an unbound component on the canvas → the property panel shows "Executor: unbound [go bind]" → a drawer lists the runtime functions registered under the current scope to complete the binding (reusing the existing Binding model, `kind=provider`).
- Opening the drawer traces provenance: the frontend replicates the server's deterministic mapping (`DeriveFunctionID` + `unboundFunctionID` normalization, see `web/src/pages/PageStudio/CompositeEditor/unboundTrace.ts`), reverse-maps the component's unbound functionId back to (source, operationId), and pre-fills; on zero hits it degrades to manual selection.
- Function candidates = bound descriptors ∪ functions unique to runtime providers, consistent with `CreateBinding`'s `registeredFunctionMetaInScope` validation source; saving rebuilds the bound contract and refreshes the editor's contract view.
- **Same-name binding** (the chosen function id == the unbound functionId): follows T6's in-place flip semantics — the unbound contract becomes bound in place, the "unbound" marker disappears on refresh, and the component needs no change.
- **Different-name binding**: the bound contract is created under the runtime function's name while the component still references the unbound material → after the drawer saves, a confirmation dialog guides switching the component's function reference (`patchProps({functionId})` rebinds the scaffold; columns/fields/links are rebuilt against the new function).
- The BindingModal on the OpenAPISources page sinks into the editor drawer; the source management page degrades to an upload entry + a diagnostics/binding-status overview.
- The ≥2-block limit on composite pages is dropped: single-block pages are legal, and the save-compile, publish, and render chains all let them through.

### 3.3 The runtime execution boundary

- Executing a binding whose function is unbound → the server returns `409 { "error": "executor_unbound", "message": "该函数尚未绑定执行器" }`, which the frontend renders as an "executor unbound" empty state with a go-bind entry point; silent failure or fabricated data is **forbidden**.
- When an agent/SDK registers a function (the `RebuildContractFromFunctionMeta` path), if an unbound contract with the same functionId exists in the same scope → it flips to bound automatically and triggers a freshness recompute for templates/proposals.

### 3.4 Tiered publishing (D5)

| Scenario                        | Flow                                                                     |
| ------------------------------- | ------------------------------------------------------------------------ |
| dev env (default) / hand-dragged pages | Save = publish directly (published_page_specs snapshots + page_versions recorded as usual) |
| prod env, or pages containing high-risk functions | Proposal → review → publish (existing chain unchanged)             |

Policy configuration: `pages.publishReview: auto | required` (env-level; default dev=auto / prod=required). The approved-proposal chain and quality gates (error-level diagnostics block publishing) are reused entirely.

### 3.5 Menu and copy internationalization (D7)

The current "zh-CN + en-US both required" is over-engineering: the default name is already determinable by the generator (humanize / the system default language), forcing users to re-enter English is duplicate labor, and the editor's ⚠ on every LocalizedText field is noise.

Target model:

```text
编辑：默认名称（必填，系统默认语言） + 翻译（可选，全局支持语言列表任选）
校验：仅默认名称非空；任何翻译缺失不警告、不阻断发布
渲染：当前界面语言 → 系统默认语言 → 任一非空值（现状回退链，不变）
组件：LocalizedTextEditor 为唯一编辑组件，REQUIRED_LOCALES 双必填标记移除
```

- The generator continues to guarantee only the system default language (consistent with ui-generation.md's "the generator guarantees only the system default language"); `ensureBilingual`'s forced en-US fill-in is abolished in favor of writing the system default language only.
- The `normalizeLocalizedText` normalization layer no longer forces a `{ zh-CN, en-US }` dual-key output; inputs are normalized to BCP47 keys and passed through as-is (short-key read fallback retained).

## 4. Data model changes

### 4.1 FunctionContract gains an execution state

```text
function_contracts 新增列：execution_state VARCHAR(16) NOT NULL DEFAULT 'bound'
  - bound   ：有运行时执行器（现状全部契约迁移后为此值）
  - unbound ：仅有契约，无执行器（OpenAPI 上传未匹配运行时函数时产生）
```

- The migration file is independent; existing rows default to `bound`, behaviorally identical to today.
- `execution_state` affects only the execution path and UI markers; it does **not** participate in schema digest / stale determination.

### 4.2 Adjusted contract admission

The `functionId is not registered in current game/env runtime` check in `internal/api/openapi/service.go` `CreateBinding` remains only on the "explicit binding" path; unbound contracts produced by the upload pipeline bypass Binding and go directly through a `RebuildContractFromFunctionMeta`-equivalent entry point (source=openapi, executionState=unbound).

## 5. Non-goals

- No second form/page runtime is introduced.
- No "simulated execution" for unbound functions (mock data is available only inside the editor's data dry-run panel; forbidden on published pages).
- httpConnector direct execution remains disabled as today (requires an allowlist + SecretRef policy; designed separately).

## 6. Acceptance criteria

- Uploading an OpenAPI document with N operations (no agent online at all): N items appear in the component library, matching page proposals appear in the proposal inbox, and the response digest numbers are correct.
- A single-block composite page can be saved, published, rendered, and executed (bound function).
- After an unbound component's page is published: rendering is normal, executing an action returns the structured `executor_unbound` error and shows the empty state.
- After an agent registers a same-named function, the unbound contract flips to bound automatically and the page becomes executable, with no manual step.
- Component templates rebuild automatically after contract changes (no manual regenerate), with correct template stale markers.
- dev env: save = publish; prod env still goes through proposal review.
- A menu/page title with only the default name entered (no en-US) can be saved and published; translations are optional, and render fallback is correct.
- Real-browser E2E (`web/e2e/` real-dashboard) covers the chains above.

## 7. Known boundaries

- An unbound contract's functionId comes from a deterministic mapping of the OpenAPI operationId; when an agent-side registered functionId is named differently, automatic binding is impossible and a manual bind in the editor drawer is required.
- After a different-name binding succeeds, the original unbound contract row is cleaned up immediately (`removeSupersededUnboundContract`, executed symmetrically inside the `CreateBinding` transaction and on upload replay) — otherwise the resource-semantics slot ends up with two same-origin candidates, and an unresolved conflict downgrades the resource proposal to needs_review.
- The drawer shows no proposal/template freshness hints — the proposal queue has its own entry point; the drawer solves exactly one thing, "binding".
- Tiered publishing's env determination depends on scope propagation being correct; when `X-Env` is missing, the strictest policy (required) applies.
- The upload pipeline is a synchronous transaction, so very large documents risk timeouts (full-scope template rebuild + per-resource proposal generation, bounded by the HTTP WriteTimeout). Since M6 there is a large-document guard: when the operations count exceeds `openapi.pipelineOperationGuard` (default 500, negative disables), the create/update response digest appends a warn-level `large_document_pipeline` diagnostic suggesting splitting the source or uploading in batches; the guard only warns, never blocks — asynchronization is a separate discussion.
