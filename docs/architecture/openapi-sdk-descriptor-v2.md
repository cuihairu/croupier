---
title: OpenAPI / SDK Descriptor v2
icon: file-code
order: 6
category:
  - System architecture
tag:
  - OpenAPI
  - SDK
  - Function registration
  - PageSpec
---

# OpenAPI / SDK Descriptor v2

> **Status**: Current -- this document is the authoritative contract for function registration and automatic page generation, guarded by the per-SDK parity tests and the registration boundary guard (`internal/function/registrationguard/`). SDK/OpenAPI are responsible for executable capabilities only, never for page design.
>
> **The six-language demo contracts share one origin**: the six-language SDK demos (go/python/java/js/csharp/cpp) register identically named functions against the same `(game_id, env)`, and in the registry the latest registration wins — a drift in any language's contract slots (schema/copy/approval) overwrites the correct values when another demo restarts, observed live three times in 2026-09 as `function_contract_versions` flapping to updated and published-page bindingFreshness stuck permanently stale. A contract change in any demo must be synchronized across all six; CI guards this with `scripts/check-demo-contract-baseline.sh` (a copy baseline plus `scripts/check_demo_contract_parity.py`, which compares all ten columns slot by slot, baseline = the Go demo); functions unique to one language are excluded from the comparison.

## Purpose

OpenAPI and the SDK descriptor describe **executable capabilities**, not pages. From them the platform identifies CRUD and non-CRUD semantics, then generates a traceable PageProposal.

```text
OpenAPI operation / SDK descriptor
  -> FunctionContract
  -> CapabilitySemantics
  -> PageProposal
  -> PageSpec
```

For the detailed page model see the [Dashboard resource/page model](./dashboard-page-model.md).

## Registration fields

All SDKs must converge on the same FunctionContract:

| Field                     | OpenAPI source                     | SDK field      | Notes                                                        |
| ------------------------- | ----------------------------------- | -------------- | ------------------------------------------------------------- |
| `id`                      | `operationId`                       | `id`           | Stable function ID                                            |
| `version`                 | Document/service version            | `version`      | Function contract version                                     |
| `summary` / `description` | Standard fields                     | Same-named     | Catalog and diagnostic descriptions, not menu facts           |
| `inputSchema`             | request body schema                 | `inputSchema`  | Request JSON Schema                                           |
| `outputSchema`            | response schema                     | `outputSchema` | Response JSON Schema                                          |
| `resourceKey`             | Derived from the REST path or `x-resource` | `resource` | Stable business resource key                               |
| `operationKey`            | REST operation or `x-operation`     | `operation`    | Business action key                                           |
| `capability`              | Derived from REST shape or `x-capability` | `capability` | Governed business semantics, not UI                        |
| `execution`               | Standard responses/`x-execution`    | `execution`    | `sync` or `task`                                              |
| `approval`                | `x-approval`                        | `approval`     | Whether approval is required plus an optional policyKey; orthogonal to execution |
| `risk` / `permission`     | `x-risk` / `x-permission`           | Same-named     | Governance constraints; risk is `safe/warning/high/danger`    |
| `enabled` / `deprecated`  | Standard fields                     | Same-named     | Catalog and execution switches; a disabled contract is persisted but not executable and not publishable |
| `tags`                    | Standard fields                     | `tags`         | Catalog and search purposes, not menu facts                   |

### Naming conventions

The SDK / JSON field names a FunctionContract exposes externally are uniformly lowerCamelCase (e.g. `inputSchema`, `outputSchema`, `approvalRequired`). Proto field names keep snake_case per protobuf convention, and their generated `json_name` is lowerCamelCase; documents, examples, and SDK external surfaces must always refer to contract fields in lowerCamelCase, and passing proto field names straight through as JSON/SDK keys is forbidden. Language-native identifiers follow their own conventions (e.g. Python kwargs are snake_case), but when an SDK serializes to JSON/wire it must use this table's lowerCamelCase contract keys.

`capability` allows only:

```text
collection_query | item_query | create | update | delete | action | task | report
```

It answers "what does the function do in the resource's lifecycle", not "where is the button shown" or "what does the page look like".

`execution: task` can combine with `approval.required: true`, meaning a task is created after approval passes; `approval` is not a third enum value of `execution`. SDKs/OpenAPI may declare governance requirements only — the page's waiting state, approval status, and result display are still generated by the Server.

`id`, `resourceKey`, and `operationKey` must be stable lowercase keys matching `[a-z0-9][a-z0-9._-]*`. SDK registrations or OpenAPI imports that violate the format must return a structured error; the platform must not repair identity via runtime encoding, truncation, or case normalization.

## Automatic OpenAPI REST recognition

When an OpenAPI document satisfies both the stable-path and schema conditions, the Server must produce high-confidence CRUD semantics:

| OpenAPI operation               | Default capability           | Additional validation                                  |
| -------------------------------- | ----------------------------- | -------------------------------------------------------- |
| `GET /players`                   | `collection_query`            | A collection schema is recognizable; pagination is an optional capability |
| `POST /players`                  | `create`                      | The request body maps to the resource input              |
| `GET /players/{playerId}`        | `item_query`                  | The path parameter is an identity candidate              |
| `PUT/PATCH /players/{playerId}`  | `update`                      | The request body can be associated with the identity     |
| `DELETE /players/{playerId}`     | `delete`                      | The identity can be associated                           |
| Other REST operations            | `action` / `task` / `report`  | Requires an explicit execution or verifiable response traits |

Derived semantics must record their source and confidence. When a REST path violates the conventions, the identity is unverifiable, or the schema is contradictory, the platform degrades to an OperationPage Proposal with a diagnostic — it must not guess Resource CRUD.

High-confidence decidability conditions (all must hold to mark `ready`):

- The path matches the two-segment `{resource}` or `{resource}/{id}` shapes, with a stable noun in the resource segment.
- The identity must be uniquely determined in the output JSON Schema of the collection item or item query; inputs needed by item/update/delete/action are explicitly mapped and validated by later typed selectors. A REST path parameter is only a candidate and must not serve as the sole identity evidence.
- The `collection_query` output schema is recognizable as an array, or a pagination wrapper with an explicit items field.
- Lifecycle functions aggregated under the same `resourceKey` are version-compatible with traceable digests.

Failing any condition means degrading with a recorded diagnostic — never guessing by "whatever looks closest". The conditions for an SDK to join Resource CRUD composition are: `resourceKey + capability` supplied explicitly, and the resource identity uniquely determinable in the item output schema or versioned-supplemented by the Resource Catalog; item/update/delete/action inputs are explicitly validated by typed selectors. Otherwise only an OperationPage Proposal is generated.

## SDK function semantics

SDKs have no HTTP method/path and cannot have complete CRUD pages generated from a function name alone. An SDK may supply `resourceKey + capability`, which is part of capability registration, not UI configuration:

```ts
registerFunction({
  id: "player.update",
  resource: "player",
  operation: "update",
  capability: "update",
  inputSchema: PlayerUpdateSchema,
  outputSchema: PlayerSchema,
  risk: "warning",
});
```

An SDK that provides only a function ID and JSON Schemas still registers normally and receives a default OperationPage; to join Resource CRUD composition it must supply a governed capability, or a platform administrator must supplement the semantics in the Resource Catalog. This requirement never asks the SDK to design menus, columns, forms, pages, or mappings.

## Registration input boundaries

The SDK and OpenAPI Source import boundaries accept only FunctionContract fields. Page presentation and orchestration information must stay in PageProposal/PageSpec or Page Studio, including:

- Table columns, pagination bindings, detail layouts, button positions, chart configuration, and task event bindings.
- Menus, routes, category display names, page titles, button copy, and multilingual page labels.
- Any browser runtime target, scope, secret, connector URL, or HTTP header.
- Any component tree, component props, page mapping, or layout DSL.

An importer encountering any of these page fields must return structured diagnostics — never silently drop or transform them.

## The supported JSON Schema subset

The generator guarantees directly publishable forms and lists only for this subset: object/array/scalar types, `required`, `enum`, format hints (`date`, `date-time`), and local `$defs`/`$ref`. More complex combinations such as `oneOf`/`anyOf`/`discriminator` and remote `$ref` do not affect function registration, but form and column generation must degrade to `needs_review` with a recorded diagnostic — never silently swallow them and generate an unusable default page.

## OpenAPI Source and execution binding

An OpenAPI upload completes the full "upload to page" generation chain in a single request ([the contract/binding orthogonalization design](./ui-generation-upload-pipeline.md) D1/D2/D4): after parse validation, every operation lands directly in the database as FunctionContract material (those with no matching runtime function get `executionState=unbound`); component-template rebuild and PageProposal generation trigger in the same transaction; the response carries a `{ operations, contractsCreated, templatesUpdated, proposalsCreated, diagnostics }` digest. A contract's existence no longer presupposes runtime registration.

```text
OpenAPI Source
  -> parse / validate / normalize
  -> FunctionContract（unbound 物料；已有 bound 契约不降级、重复上传幂等）
  -> 组件模板重建 + CapabilitySemantics / PageProposal
  -> 运行时注册同名函数自动翻转 bound / 编辑器抽屉人工绑定
```

Execution binding is an **orthogonal attribute** of the contract: `unbound` contracts enter the template and proposal pipelines as usual and can be saved and published; only execution is blocked — binding execute returns a structured `409 executor_unbound`, and the frontend renders an "executor unbound" empty state that guides toward binding. When an agent/SDK registers a function with the same functionId in the same scope, the corresponding unbound contract flips to `bound` automatically; a differently named runtime function can be bound manually from the binding drawer in the editor property panel or the OpenAPI Sources page (once a different-name binding succeeds, the superseded unbound row is cleaned up immediately).

Bindings and the future governed http connector must both be isolated by `game_id + env`, and pass through permissions, audit, and OTel. OpenAPI documents must not contain secrets, arbitrary intranet URLs, or page configuration.

## Generation boundaries

The Server generates Proposals from FunctionContract + CapabilitySemantics:

- Standard CRUD semantics generate a ResourcePage Proposal.
- An executable synchronous function lacking Resource semantics generates a `basic` OperationPage Proposal.
- `execution=task` generates a TaskPage Proposal, `needs_review` when the real task-state contract is incomplete. With `approval.required=true` the approval waiting state shows first, then the synchronous result or task status after approval.
- `capability=report` generates a ReportPage Proposal, `needs_review` when the dataset/metric semantics are incomplete.

The generator never reads or writes page UI from the SDK/OpenAPI. A PageProposal's columns, mappings, navigation, form presentation, and action placements are all produced by the Server per the generation rules, then managed by Page Studio.

## Function changes

Every FunctionContract or CapabilitySemantics change must:

1. Save the new version and digest.
2. Recompute the affected ResourceCapability and PageProposals.
3. Diff against the source digests of PageDrafts/PublishedPageSpecs.
4. Mark stale and block unsafe execution.
5. Let users inspect, merge, repair, and republish in Page Studio.

A published page must never be silently replaced with the latest function descriptor.

## SDK delivery requirements

- All SDKs agree on the field semantics of `FunctionContract`; unsupported fields must either error explicitly or be marked unsupported in the capability matrix.
- Go's `RegisterFromOpenAPI` is only a local handler-registration helper; other SDKs must not claim equivalent capability.
- Demos must cover at least one CRUD Resource, one Operation, one Task, and one Report contract, and pass the Server's Proposal generation E2E.
- SDKs never carry page UI or PageSpecs.
