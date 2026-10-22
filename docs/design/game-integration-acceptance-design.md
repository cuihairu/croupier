---
title: Game Integration and Compatibility Acceptance
---

# Game Integration and Compatibility Acceptance

## Status and Scope

- Status: the spec is pending review; `docs/sdks/sdk-parity-matrix.md` is the cross-language integration matrix, and `sdks/SDK_FEATURE_MATRIX.md` remains the master table of implementation status.
- This page defines the criterion for "the game business is integrated": not registered metadata, not a passing compile, and not a README claim — the full chain of query and controlled write passes acceptance.

## Applicable Paths

| Path                      | Applicable scenario                                                          | Minimum requirement                                                                    |
| ------------------------- | ---------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| SDK Provider              | Register functions inside the game service                                   | L1 required capabilities + input/output schemas + scope authorization.                 |
| OpenAPI                   | Low-cost integration of an existing REST gateway                             | OpenAPI 3, stable operationIds, schemas consistent with the implementation.            |
| External function plugins | Non-game-process, multi-language, or independently lifecycle-scoped business | Manifests, permissions, process supervision, timeout/cancellation, and crash recovery. |
| Server Invoker            | Ops console, automation, or external system invocation                       | Server HTTP, task status/cancel/events, authorization, and idempotency.                |
| PageSpec                  | Ops page orchestration                                                       | Published snapshots, approval policies, schema digests, and the renderer ABI.          |

Missing capabilities must return `CAPABILITY_UNSUPPORTED`; a page existing, a menu existing, or a descriptor existing does not mean the business is integrated.

## Minimum Capabilities

1. Identity: `gameId/env` non-empty, `agentId/serviceId/sdkVersion` consistent with reality; cross-scope sessions or function references fail.
2. Query: at least one stable read-only function, schema-validatable, errors field-locatable, permission-denied when unauthorized.
3. Controlled write: uses `operationId` + the function contract fingerprint; high-risk functions execute only after approval.
4. Idempotency: the game side persists `(scope, operationId, requestFingerprint)`; replaying the same ID returns the original result or credential.
5. Unknown outcome: timeouts/disconnects can only be resolved by querying the original `operationId`; new random retry keys that could produce side effects are forbidden.
6. Disconnect and cancel: the SDK/plugin supports reconnect, drain of in-flight work, task cancellation, and terminal-state queries.
7. Probing: read-only health/business probes may fail independently; an unavailable probe never substitutes for business verification.
8. Observability: registration, invocation, approval, tasks, probing, and disconnects produce correlatable IDs and error classifications.

## Compatibility Matrix

| Component     | Version/contract threshold                                                          | Action when unmet                                                 |
| ------------- | ----------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| Platform wire | Protocol version and the capability closed-set                                      | Refuse the connection or report the missing capability.           |
| SDK           | `sdkVersion` below the configured floor is refused; below the high-water mark warns | Upgrade the SDK.                                                  |
| Agent         | `agentVersion`, carrying capability, wire version                                   | Disable non-compliant targets and advise an upgrade.              |
| Function      | Function version, schema digest, approval policy                                    | Low versions may be configured as refused; old bindings go stale. |
| Plugins       | Manifest, dependencies, ABI/IPC version, permissions                                | Refuse to install or enable.                                      |
| OpenAPI       | OpenAPI 3, stable `operationId`, schemas                                            | Import fails, keeping the old contract.                           |
| PageSpec      | Renderer ABI, binding snapshots, approval policies                                  | Block publishing or execution.                                    |

The upgrade order is protocol/platform → Server → Agent → SDK/plugins → PageSpec. Mixed windows allow additive fields only; deletions or semantic changes require a two-version transition.

## Integration Steps

1. Choose the path: in-game functions use the SDK; existing REST uses OpenAPI; separate processes/multi-language use plugins.
2. Declare identity and capabilities: scope, service name, language version, the capability closed-set, and the unsupported semantics of missing capabilities.
3. Define contracts: stable function IDs, schemas, risk, permissions, approval, and idempotency requirements.
4. Import the reference implementation: at least covering the query/create/update/delete and `player.kick` of `examples/openapi-provider`.
5. Wire pages: PageSpec binds function contracts, with high-risk operations enabling approval and confirmation copy.
6. Run the acceptance tools: static API/matrix gates first, then start the real chain and run the nine integration chains of `docs/sdks/sdk-parity-matrix.md`.
7. Produce the acceptance report: record versions, contract fingerprints, case results, known unsupported items, and performance hop counts.

## Acceptance Scenarios

1. Reference game query and controlled write: `player.list/player.get` queries pass; `player.create/player.update/player.kick` execute under authorization, approval, and idempotency rules.
2. Same ID registered from multiple ends: different SDK versions can coexist; instances below the floor are refused, instances below the high-water mark warn.
3. Old contracts: after a function schema changes, old PageSpec bindings are judged stale and execution is blocked.
4. Wrong scope: calling a `game-b/prod` target with a `game-a/dev` session returns `SCOPE_MISMATCH` and leaves an audit record.
5. Unsupported capability: requesting an unimplemented plugin or probe returns `CAPABILITY_UNSUPPORTED` — never an empty success.
6. Unknown outcome: after an HTTP response is lost, querying with the same `operationId` converges to the original credential; substitute retries are refused.
7. Disconnect recovery: after a provider reconnects, the catalog is consistent; after drain, new requests are refused and in-flight requests reach terminal states.
8. Task cancellation: after cancel, the function stops producing side effects, events show the cancellation reason, and the result is queryable.
9. Probe failure: when the port is reachable but the business function fails, integration acceptance does not pass.
10. Observability: the latency, hops, and failure classifications of the SDK path and the external OpenAPI path are comparable.
