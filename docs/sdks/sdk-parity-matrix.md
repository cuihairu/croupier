---
title: SDK Capability Matrix
---

# SDK Capability Matrix

This page defines the cross-language unified baseline. For implementation status and language mapping, see the root-level `sdks/SDK_FEATURE_MATRIX.md`; for protocol details, see [SDK Wire Protocol](../zh/architecture/sdk-wire-protocol.md).

## Layers

| Layer | Positioning                    | Unified requirement                                                                              |
| ----- | ------------------------------ | ------------------------------------------------------------------------------------------------ |
| L1    | Minimal provider callee set    | Required for all official SDKs: registration, invocation, heartbeat, reconnection, TLS, drain.   |
| L2    | Provider governance extensions | Must align when implemented: schemas, manifests, file transfer, OpenAPI import.                  |
| L3    | Standalone Invoker caller      | Server HTTP only; reusing the Provider TCP session is forbidden.                                 |
| L4    | Language/engine extensions     | No cross-language alignment, but platform versions and language runtime limits must be declared. |

## Required and Optional

| Capability                | Layer | Status   | Onboarding requirement                                                                 |
| ------------------------- | ----- | -------- | -------------------------------------------------------------------------------------- |
| Registration & session    | L1    | Required | Non-empty `gameId/env/agentId/serviceId/sdkVersion`.                                   |
| Function catalog          | L1    | Required | Stable function IDs, input/output schemas, risk and permissions.                       |
| Heartbeat/reconnect/drain | L1    | Required | In-flight calls settle first; no new requests accepted after drain.                    |
| OpenAPI Provider          | L2    | Optional | Stable `operationId`; schema matches the implementation.                               |
| Tasks                     | L3    | Optional | Start, status, events, cancellation, and idempotent result queries.                    |
| External function plugins | L2    | Optional | Declared contract, process lifecycle, timeout/cancellation, fault isolation.           |
| File delivery             | L2    | Optional | Verify SHA256, size, and write-to-disk boundaries.                                     |
| Probing                   | L2    | Optional | Clear protocol semantics and network placement; unimplemented must return unsupported. |

## Versioning and Compatibility

| Object   | Required contract                                                         | Compatibility rule                                                                                   |
| -------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| SDK      | Language, `sdkVersion`, `gameId/env`, closed set of wire capabilities     | Registration rejected below the configured floor; accepted with a warning below the high-water mark. |
| Agent    | `agentVersion`, hosted capabilities, wire version, `gameId/env`           | Missing capability reports unsupported; never silently degrades.                                     |
| Function | `functionId`, version, input/output schemas, permissions, approval policy | A rejection threshold may be set for low versions; old bindings judged stale by digest.              |
| OpenAPI  | OpenAPI 3, stable `operationId`, schemas, risk annotations                | Validated at import; a schema or ID change is a new contract.                                        |
| Plugins  | Manifest, dependencies, ABI/IPC versions, capabilities, permissions       | Install/enable rejected if the version is undeclared or dependencies unmet.                          |
| PageSpec | Page, bindings, approval policy, renderer ABI, schema digest              | Published snapshots are frozen; stale bindings block execution.                                      |

Upgrade order: protocol & platform → Server → Agent → SDK/plugins → PageSpec publication. During mixed-version windows, old fields must remain readable; deleting fields or changing semantics is a breaking change — publish a compatible transitional contract first.

## Closed Error Set

| Status                   | Trigger                                                       | Consumer action                                                        |
| ------------------------ | ------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `MISSING_SCOPE`          | Missing `gameId/env`                                          | Fix the identity and reconnect; no local defaults.                     |
| `SCOPE_MISMATCH`         | Session scope differs from the function/page target           | Stop execution and check the registration source.                      |
| `SCHEMA_INVALID`         | Payload violates the input/output contract                    | Do not invoke the business implementation.                             |
| `VERSION_TOO_LOW`        | SDK/function below the rejection threshold                    | Upgrade, then register again.                                          |
| `CONTRACT_STALE`         | Page snapshot inconsistent with the current function contract | Republish the page or update the function.                             |
| `CAPABILITY_UNSUPPORTED` | Capability requested but not implemented                      | Report the error truthfully; never treat metadata as success.          |
| `RESULT_UNKNOWN`         | Timeout, disconnection, or lost business response             | Only query the original operationId; substitute retries are forbidden. |

## Onboarding Acceptance Chain

1. Registration: non-empty scope, agent/service identity, SDK version, and function contracts are stored successfully; an erroneous scope is rejected.
2. Schema: a valid minimal payload passes; missing fields, wrong types, and out-of-scope fields fail with errors pinpointed to the field.
3. Authorization: calls without permission are rejected; approval-gated functions execute only after approval, and contract changes after approval are blocked.
4. Controlled writes: replaying the same `operationId` returns the original credential or an idempotent result; same ID with different parameters is a conflict.
5. Unknown results: after simulating a lost response, only querying the original ID is allowed; no substitute side effects may be issued.
6. Reconnection recovery: after rebuilding the session the function catalog is consistent, and reconnect storms back off.
7. Task cancellation/status: a running task stops in-flight work once cancelled; terminal states and events are queryable.
8. Probing failures: unavailable probing produces a locatable error, never a disguised business success.
9. Observability: the SDK/OpenAPI/plugin paths record latency, hop counts, and failure classifications.

## Acceptance Tooling and References

- Reference game service: `examples/openapi-provider` offers `/players` queries, CRUD, and `player.kick`, with `operationId`s matching the OpenAPI 3 contract; the default `default/dev` is overridable.
- Cross-language tooling: extend `scripts/check-sdk-matrix.sh` into an integration acceptance runner that runs L1/L3 contract tests, the closed error set, and mixed-version cases per language; the Go/Python reference plugins are filled in as implementations land.
- The existing matrix check in this repo only proves the API surface exists; the acceptance chain on this page additionally requires invocation behavior, authorization, idempotency, and unknown-result handling to pass.
