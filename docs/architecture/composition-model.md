---
title: Composition model and expressiveness boundaries
icon: blocks
order: 9
category:
  - System architecture
tag:
  - Composite pages
  - PageSpec
  - Design overview
---

# Composition model and expressiveness boundaries

> **Status**: Current -- this document is the **design overview** for composite pages / page generation: it defines the mental model, the composition primitives, the expressiveness boundaries, and the extension methodology. Rule details live in pagespec-protocol.md (wire), dashboard-page-model.md (model), and presentation-hints.md (hints); this document does not repeat them. For a newcomer's introduction (why the design exists and how the full flow works), see How the UI is generated: core idea and end-to-end path.

## Positioning: why this is a key product design

Composing a composite page is "declarative React, made visual": the user drags blocks, wires up data, and configures actions, and **gets a fully governed admin page without writing code**. What separates it from low-code products on the market is not the number of building blocks but the design trade-offs in the composition primitives:

- **Governance-first whitelisted primitives**: every composition capability is a spec field + a renderer capability + a validator --
  not a free canvas with arbitrary code. Permissions, approvals, and audit hang off function bindings, so no page composition can bypass governance (the browser only submits `page_state`; functionId/target/scope are resolved server-side).
- **Declarative data-level composition**: a page is a JSON tree, not code -- it can be validated, snapshotted, diffed, rolled back, and rendered across languages (rjsf consumes the schema; PageRenderer consumes the spec).
- **Three generation sources**: automatic generation from contracts (Type A), configuration import (Type B, e.g. constant Excel/JSON), and user authoring (multi-select on the canvas saved as a component). A composite page = a drag-and-drop combination of the three.

## Mental model: declarative React, made visual

The essence of React is `UI = f(data)`: declare what the interface looks like, and the framework handles updates. This system turns `f` from code into data -- a constrained, declarative subset of React.

### Division of labor between the two layers

| Layer           | Responsibility                                   | Carrier     | Consumer                              |
| --------------- | ------------------------------------------------ | ----------- | ------------------------------------- |
| **Widget layer**   | Fields / types / enums / validation / widget types | JSON Schema | `@rjsf/antd` (SchemaFormRenderer)     |
| **Orchestration layer** | Layout / data flow / actions / lifecycle    | PageSpec    | PageRenderer (CompositeRenderer etc.) |

JSON Schema only governs "widgets and data shapes" (rjsf is already the mature visual equivalent of "schema → React form"); layout, linking, actions, and lifecycle belong to PageSpec. Together the two layers make up the complete visual React.

### React composition primitives [PageSpec mapping]

| React primitive                    | PageSpec counterpart                                                                                                                                                                                                                                                      | Status                                                              |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| Nested `children`                  | The children tree of container/modal                                                                                                                                                                                                                                      | ✅                                                                  |
| **Props in** (parent → child data) | SelectorAST input assignment (form/row/detail/page_state/literal + JsonPointer) + implicit merge of same-named `refreshOn` fields + explicit parameter mapping (`ParamMappingEditor`, shipped 2026-09)                                                                    | ✅ Implicit same-name merge + explicit mapping (literal constants / upstream block.field) |
| **Callbacks out** (child → parent behavior) | events → action chains (runBinding/refreshNode/openModal/closeModal/navigate/showMessage)                                                                                                                                                                       | ✅                                                                  |
| Context shared state               | page_state (output assigns stateKey)                                                                                                                                                                                                                                      | ✅                                                                  |
| Derived values (computed props)    | transform whitelist: `pick` only                                                                                                                                                                                                                                          | ❌ Missing rename/default/format                                    |
| Conditional rendering              | In-form `visibleWhen` + block-level `visibleWhen` (leaf `key`+`path` reads page state, U10)                                                                                                                                                                               | ✅ (dialog blocks not included)                                     |
| List `map` (execute per row)       | Built-in rendering inside fnTable; no cross-block batching                                                                                                                                                                                                                | ❌ Missing (selection semantics not closed)                         |
| Lifecycle                          | autoRun / refreshOn cascades / events                                                                                                                                                                                                                                     | ✅ (no failure policy yet)                                          |
| Component parameterization (props defaults) | Template scaffold + property panel                                                                                                                                                                                                                               | ⚠️ Missing template-level batch configuration                       |

### The four composition axes and their gaps

Composition = four axes: **space + data + behavior + condition**:

1. **Space** (where, how big): grid spans / containers / modal grouping -- ✅ essentially complete;
   known boundary: interaction is fully supported within two levels of container nesting; great-grandchild-level preview is simplified.
2. **Data** (where values come from, where they go): refreshOn cascades + same-name merge + SelectorAST --
   explicit parameter mapping has landed (`ParamMappingEditor`: downstream parameter ← source block.field / literal);
   **P0 gap**: block-instance namespaces (variable-name prefixes, generalized -- the constant form already went first).
3. **Behavior** (what gets triggered): events + action chains -- ✅ usable;
   P1 gaps: conditional actions, failure policy (when an upstream fails: downstream clears / keeps / warns -- undefined).
4. **Condition** (when to show / execute): in-form visibleWhen ✅; block-level conditional display ✅ (U10:
   `CompositeSection.visibleWhen`, leaf `key`+`path` reads page state; false skips rendering but execution proceeds;
   dialog blocks not included; nested all/any depth ≤ 4).

## Expressiveness boundary: deliberate trade-offs

React's expressiveness is **arbitrary code**; PageSpec's expressiveness is **whitelisted primitives**. Every restriction is the price of governance:

| Not allowed                                                | Why                                                                                   |
| ---------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| Code / arbitrary expressions / arbitrary JSONPath in a page spec | Saves are validatable, publishes are snapshotable, nothing escapes at runtime    |
| The browser passing functionId/target/scope                | Governance (permissions/approvals/audit) binds execution only on the server side       |
| The frontend adding widgets/transforms/actions privately   | Primitives are controlled: extensions must go through the full spec-package + validator + documentation chain |
| Execution blocks with no bound function                    | Constant forms (static) only produce page state; execution must hang off a governed function |

## Extension methodology: the five-step chain for adding a primitive

Any new composition capability (new widget / new transform / new block type / new action) must go through the complete chain; a missing step means unfinished:

1. **Spec field** (internal/dashboard/spec + JSON Schema validator)
2. **Frontend type** (web/src/types/dashboard.ts)
3. **Editor** (component registration / property panel / compile-and-read-back round trip)
4. **Renderer** (consumed by PageRenderer/SchemaFormRenderer)
5. **Docs + tests** (three-layer doc sync + unit/snapshot tests)

## Gaps and roadmap

| Priority | Gap                                                                                                                                                  | Notes                                                                                                                                                                                                                     |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ~~P0~~   | ~~Block-instance namespaces (variable-name prefixes / renames)~~ ✅ 2026-09 (todo U5)                                                                | `sectionKey` declared and pinned + read-back does not drift; see Composite page editor V4                                                                                                                                |
| ~~P0~~   | ~~Template-level parameterization (component props defaults)~~ ✅ 2026-09 (todo U6)                                                                  | `params` whitelist (title/span/autoRun) + quick configuration on drop; see V4 §3.6                                                                                                                                        |
| P1       | ~~Failure policy + transform extensions (rename/default/format)~~ ✅ 2026-09 (todo U8/U9)                                                            | Data-flow robustness: transform rename/default; refreshOn cascade cascadePolicy (pause/clear/keep)                                                                                                                        |
| P1       | ~~Cross-template link-break warnings (silent cleanup of dangling references on instantiation)~~ ✅ 2026-09 (todo U7)                                 | instantiateTemplateDetailed detects four kinds of dangling references (action targets / link dependencies / parameter-mapping sources / row-action modals), shows a Modal after drop + quick reconnect on canvas blocks (reconnectTemplateRefs); unreconnected items fall back to compile warnings on save |
| ~~P2~~   | ~~Block-level conditional display~~ ✅ 2026-09 (todo U10); batch (map) composition                                                                    | `visibleWhen` leaf key+path reads page state; batching depends on closing the selection loop                                                                                                                              |
| P3       | New blocks: task-monitor composition (taskStatus node), report charts (chart node)                                                                    | Each = a new node type + renderer; a component-model extension                                                                                                                                                            |

## Relationship to other documents

- Wire contract details: [pagespec-protocol.md](./pagespec-protocol.md)
- Domain model (contract / semantics / proposal / publish): [dashboard-page-model.md](./dashboard-page-model.md)
- Editor usage: [Composite page editor V3](../dashboard/composite-editor-v3.md)
