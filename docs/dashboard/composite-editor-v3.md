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
- Execution requires a bound runtime contract and a published page binding.
- Table pagination is a published-spec setting (`table.pagination`, D3) with no editor or template entry yet: when `enabled` the renderer requests `page`/`pageSize` with a renderer-derived `offset=(page-1)*pageSize`, uses the response `total` verbatim or, when absent, estimates the bound as "a full page means a next page may exist" and closes off on a partial page, resets to page 1 when the upstream filter input changes, and keeps the current page on a post-operation refresh; without the setting the table keeps client-side pagination. Cursor pagination and a per-table `totalKey` are not supported, and tables rendered inside a dialog do not paginate.
