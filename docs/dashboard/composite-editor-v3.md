---
title: Composite page editor V3
---

# Composite page editor V3

V3 is the componentized production editor for composite operator pages. It turns function contracts into editable components and reuses the published renderer for preview, so the page an operator edits is the page the runtime will execute.

## Editor capabilities

| Area            | Capability                                                                                                                |
| --------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Entry points    | Create a composite page from the proposal inbox or edit an existing composite page.                                       |
| Component panel | Function components grouped by resource, plus buttons, modals, containers, and text.                                      |
| Outline         | Tree navigation with click-to-locate selection.                                                                           |
| Insertion       | Click insertion, drag insertion, drop indicators, and contract-aware container children.                                  |
| Instances       | Multiple instances of the same function; automatic stable key allocation and rebinding.                                   |
| Canvas          | Grid width adjustment, ordering, context menu, parent selection, copy, and delete.                                        |
| Selection       | Shift-click accumulation and bulk deletion.                                                                               |
| History         | Undo and redo for tree changes with a 50-step snapshot window.                                                            |
| Modals          | Enter a modal's inner canvas, edit several forms, and return through breadcrumbs.                                         |
| Properties      | Schema-driven configuration and action tabs for the selected node.                                                        |
| Events          | Button, table row, form success/failure, field, and text interactions.                                                    |
| Action chains   | Ordered execution with abort on a failed execution step, plus refresh, modal close, navigation, and notification actions. |
| Data debugging  | Try a function in the bottom panel and inspect the result before publishing.                                              |

## Typical workflow

1. Create a composite page in the current `gameId + env` scope.
2. Drag a collection query into a table and enable automatic execution if appropriate.
3. Add a detail modal, map row fields to its inputs, and wire row actions.
4. Add operation forms and configure confirmation or refresh behavior.
5. Preview with the published renderer and try the data panel.
6. Save the proposal, resolve validation diagnostics, publish, and mount the page.

## Compilation model

The editor serializes a `CompositeSection` tree into a `PageSpec`. The compiler:

- assigns section keys for page state;
- converts node props into typed page fields;
- validates selector mappings and output shapes;
- preserves refresh, visibility, and action-chain edges;
- keeps dialogs as execution groups even when they are grouped for display.

Refresh edges (`refreshOn`) compile straight through to the published spec. At runtime the cascade is driven by a per-section result revision, so a same-state new upstream result (two consecutive successful runs with different data) cascades as well; a section whose `refreshOn` graph cycles back to itself is refused the cascade.

Published behavior is deterministic: hidden inline sections still execute according to their configuration, while rendering is filtered by `visibleWhen`.

## Preview pipeline (B1)

Preview consumes the same generated spec as save and publish. Entering preview compiles the tree, posts the compiled sections to `POST /api/v1/versioning/pages/composite/preview`, and renders the returned generated spec through the production `PageRenderer`/`CompositeRenderer` — only the execution adapter is replaced. The mock executor synthesizes sample data from each binding's contract `outputSchema`, returns it as a synchronous result, and so exercises the real rendering, refresh, action-chain, and pagination paths. The endpoint runs the same generation chain as creation (contract lookup, binding assignment, static-section interleaving, template snapshot, publish-level selector validation) and writes no proposal, draft, or audit row, so it needs no `pages:publish` permission; its output is field-for-field identical to what saving and publishing would store.

The response's `quality` and `diagnostics` render inline in the preview pane, so generation diagnostics are visible before saving. Fallback: a tree that cannot be losslessly compiled (text nodes, non-card containers) or a failed preview request falls back to the legacy structural preview, with the blockers listed, so preview is never blank.

**Data source switch (B1, 2026-10)**. The preview header carries a mock/real switch. Mock (default) keeps the mock executor described above. Real preview executes through the server-validated draft entry `POST /api/v1/versioning/pages/composite/preview/execute`: the pane posts the same generation input the preview endpoint received (page key, compiled sections, template snapshot) plus the `bindingId` and execution `context`, the server rebuilds the draft spec through the identical generation chain, re-runs publish-level selector validation, and then applies the same gates as a published page — permission check, approval gate, contract snapshot; the browser never submits `functionId`/`route`/`scope`. Switching the data source remounts the rendered page so results, pagination, and selections do not leak across modes.

**One shared runtime**. The production `CompositeRenderer` and the editor preview pane no longer keep separate state machines: both consume the `useCompositeRuntime` hook (`web/src/components/PageRenderer/compositeRuntime.tsx`) — results ledger, revision-driven cascades, action chains, abort-on-failure, danger confirmation, form submit exits, and server pagination live there once. The three execution modes differ only in the injected `execute` adapter: mock synthesis, draft execute, and the published binding execute. This is what makes "same configuration behaves identically in mock preview, real preview, and the published page" hold by construction.

Boundary: real preview performs real function calls against the draft (operation functions and approval submissions have real side effects) but publishes nothing; the legacy structural fallback renderer (`PreviewRuntime`) keeps its own simpler interaction loop and is not yet on the shared hook; an `autoRun` failure in mock mode is recorded as a per-section error marker (surfaced via cascade policies) and does not raise the section-level failure alert, which is only set by explicit form submits. A tree whose save would be refused reports that refusal through the request error rather than as an inline diagnostic list.

## Component development

- Register a component with its accepted props and child contract.
- Separate schema-driven configuration from action configuration.
- Implement round-trip serialization to preserve existing keys and references.
- Test insertion, deletion, undo, redraw, publication, and decompilation.
- Avoid component-specific logic in the runtime renderer; prefer typed PageSpec fields.

## Later hardening

The page also records subsequent production hardening: selector diagnostics shared by save and publish, contract drift repair, template update notices, registration-derived rebuild warnings, workspace filtering, pagination pushdown, and expression binding. Consult the implementation tests listed in the guide before changing compiler or round-trip behavior.

## Known boundaries

- Container rendering intentionally simplifies very deep descendant layout.
- Text components are editor aids and do not publish runtime content.
- Preview data is an editor-only debugging aid; published pages have no mock fallback.
- Preview (B1) renders the server-generated spec through the production renderer on the shared `useCompositeRuntime` hook; the data source switches between a mock executor and a server-validated draft execute entry. Real preview exercises operation functions and approval flows against the draft (real side effects, nothing published), so it must be used on test games. A tree that cannot be losslessly compiled (text nodes, non-card containers) or a failed preview request falls back to the legacy structural preview and lists the blockers — the preview is then structural rather than publish-identical, and that fallback renderer keeps its own interaction loop rather than the shared hook.
- Execution requires a bound runtime contract and a published page binding.
- Table pagination is configurable from the table component's "server-side pagination" setting (D3): enabling it compiles to `table.pagination` on the published section (`enabled` plus optional `defaultSize` and `pageSizes`, clamped to [1, 200] at compile time and deduplicated ascending). The generator does not infer pagination for composite sections (no resource-semantics context), so it stays off unless explicitly enabled, and a setting that is unset or disabled keeps the pre-D3 client-side pagination. When `enabled` the renderer requests `page`/`pageSize` with a renderer-derived `offset=(page-1)*pageSize`, uses the response `total` verbatim or, when absent, estimates the bound as "a full page means a next page may exist" and closes off on a partial page, resets to page 1 when the upstream filter input changes, and keeps the current page on a post-operation refresh. The bound function must actually consume `page`/`pageSize`: the platform passes extra input keys through, so a function that ignores them returns the full set on every page (identical page contents, an inflated page count). Cursor pagination and a per-table `totalKey` are not supported, server-side sorting/filter pushdown is not implemented (header sorting stays client-side), and tables rendered inside a dialog do not paginate.
- Dangerous actions confirm before executing (D2): the confirm body lists the target modal's prefilled parameters by field title (enum values as option labels, no raw field keys) and the chain steps; with a `targetSection` the modal and its chain proceed only after the user confirms, and a pure-chain danger button confirms before running. Boundaries: batch target review (multi-select, cross-page selection, total vs current-page counts, a target list entry, clearing the selection, stale selections after a filter change) is not implemented — selection stays single-row radio and there is no `table.selection` wire field; the confirm carries no field-semantic emphasis (identity / reason / expiry / amounts — that depends on D5's explicit field semantics); `openModal` chain steps carry no danger flag, and the confirmation happens before the modal opens so in-modal edits are not re-confirmed.
- Execution feedback splits submission outcomes (D4): a synchronous result is business success (fires the success event, re-runs `onSuccessRefresh` targets, and closes the dialog); an `approval` result shows "submitted for approval" with the approval id and a link to the approvals center, and a `task` result shows "task submitted" with the task id — neither closes the dialog, fires the success event, or refreshes targets. A failure keeps the dialog and its entered values open and lists the stable error code, the server message, and field-level errors by field title. Boundaries: field errors are text next to the form rather than injected into the RJSF widgets (the shared form renderer has no external-error prop), console page execution has no server-side idempotency key, there is no tasks route so a pending task shows only its id, and the approval link is a fixed `/approvals` route.
