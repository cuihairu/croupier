---
title: High-Risk Operation Protocol Review
---

# High-Risk Operation Protocol Review

Date: 2026-11-22. Status: **proposal awaiting owner decision**. This page is the
GAME-009 方案决策 deliverable. It reviews
[high-risk-operation-contract-design](high-risk-operation-contract-design.md)
(which remains `pending design review`) and resolves its §10 "Decisions Pending
Review" into a concrete proposal with alternatives for the owner to approve.
**No wire, model, or task/approval behavior is changed by this page** — that is
the GAME-010 implementation item, gated on this proposal's approval.

## 1. Risk Surface (what is broken today)

The platform has no business-level execution identity for high-risk writes.
Grounding is current code: the invoke chain
(`internal/platform/functioninvoke`) and task model (`internal/tasks`) carry
`IdempotencyKey`/`TaskID`, and page execution has server-side idempotency
(`page_execute_idempotencies`, migration 0048). There is no
`Credential`/`Receipt`/`operationId` persistence anywhere in `internal/`
(verified: no `BusinessOperation`/`OperationCredential` model exists). The
concrete failure modes:

| #   | Failure mode                                 | Today's behavior                                                                                                                        | Consequence                                                                                                            |
| --- | -------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| R1  | **Lost response double-grant**               | A function call times out; the platform has no durable game-side dedup contract, so a retry re-invokes the game function.               | The game grants twice (gold, items, mail, currency).                                                                   |
| R2  | **Unknown misjudged as failure**             | `ExecutionLog.Status` is `ok/error`; a timeout is logged as `error`.                                                                    | A business success after a lost response is reported as failure; operators substitute rewards or retry, amplifying R1. |
| R3  | **Platform idempotency ≠ game exactly-once** | `taskId`/page-key idempotency dedups platform replays only; it does not constrain the game's own side effects across restart/reconnect. | Exactly-once is claimed where only transport-level dedup exists.                                                       |
| R4  | **Cancellation claims rollback**             | Task cancel sets a canceled state without game confirmation of side effects.                                                            | "Canceled" reads as "rolled back" even when assets were already granted.                                               |
| R5  | **Same operationId parameter drift**         | No snapshot fingerprint is enforced at the game side; a retry can carry modified parameters under the same identity.                    | A re-approval is bypassed; the approved intent is silently altered.                                                    |
| R6  | **Credential absence → success fabricated**  | No credential/query contract; an unknown outcome is resolved by re-execution rather than by querying the game for the original result.  | Duplicate grants (R1) and unrecoverable "unknown" states with no query path.                                           |

The design must _not_ rebuild a duplicate of `taskId`/page idempotency: those
remain the platform transport/page-replay layer. The business identity
(`operationId`) and game-side durable dedup are a **separate, game-cooperative
layer**. The two reference each other but never substitute.

## 2. Protocol Design (proposal for the 5 open decisions)

### D1 — operationId encoding

**Proposal:** `operationId = <gameId>:<env>:<domain>:<intentUUID>`, where
`intentUUID` is a UUIDv7 (time-ordered, index-friendly). Max length 128 ASCII
lowercase; character set `[a-z0-9:-]`. The platform generates it at business
intent creation; it is immutable and globally unique per `(gameId, env,
domain, intentUUID)`. A short display code (last 8 of the UUID, checksummed)
is shown to players — never the internal auto-increment ID.

**Why:** encodes scope+domain into the identity so cross-game/scope collisions
are structurally impossible and a DB unique index on the composite is natural.
UUIDv7 keeps inserts append-friendly vs UUIDv4 random.

### D2 — credential storage form

**Proposal:** a dedicated `business_operation_credentials` table
(`(game_id, env, operation_id, request_fingerprint, outcome, game_receipt,
result_json, query_hint, created_at, updated_at)`), unique on
`(game_id, env, operation_id)`, one row per operationId. `game_receipt` masked
on display with a controlled full-read path for operators/auditors.

**Why:** decouples long-lived business credentials from short-retention
execution logs; the credential is queryable by `operationId` for convergence
(R2/R6) and survives log cleanup. A `TaskRun`/`ExecutionLog` extension couples
credential lifetime to transport lifecycle; a pure structured `ResultPayload`
in the log is not independently queryable at scale.

### D3 — SDK result envelope

**Proposal:** each game SDK ships a `GameOperationResult` envelope
(`operationId`, `requestFingerprint`, `outcome`, `gameReceipt`, `result`,
`retryableAfter?`, `queryHint?`) as the canonical return for high-risk
functions. External function plugins proxying an HTTP interface must declare a
`durableIdempotency` capability; without it the function is refused entry into
the high-risk execution catalog.

**Why:** a single envelope type makes outcome parseable by the platform
without per-game ad-hoc mapping; capability declaration is the only way to
admit external plugins safely (they cannot be inspected for idempotency).

### D4 — unknown outcome policy

**Proposal:** default auto-query every 60s (max 10 attempts) against the
contract's read-result function, then alert at `unknown`; auto-retry only for
`retryable_failed`/`unknown` reusing the same operationId and snapshot, bounded
by backoff (1s→60s), max 8 attempts, concurrency cap, game-health gate; no
substitute rewards beyond the auto-retry cap — only alerts and operator
queries. Unknown is **never** rewritten to success/failure.

**Why:** periodic query converges R6 without new execution; the cap prevents an
infinite query loop. Substituting a reward on unknown would fabricate success.

### D5 — cancellation protocol

**Proposal:** cancellation is a minimum integration capability for high-risk
functions (query/confirm a game cancel request); functions may additionally
declare `supportsUndo`. The platform sends a game cancel request and requires a
game confirmation before reporting `canceled`; `cancel_failed` keeps the
original result and routes only through undo/compensation (new approval + new
operationId).

**Why:** without a confirmation, `canceled` would fabricate R4. Making it
mandatory (not optional) keeps the state machine honest for every high-risk
function; `supportsUndo` stays optional because native undo is game-specific.

## 3. Alternative Comparison

| Decision       | Recommended                                                 | Alternatives considered                                                    | Why rejected                                                                                                                                                                    |
| -------------- | ----------------------------------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1 operationId | Composite `gameId:env:domain:UUIDv7`                        | Raw UUIDv4 only; server auto-increment; short code only                    | UUIDv4 loses time-ordering; auto-increment leaks volume and isn't globally unique; short code alone isn't a unique key.                                                         |
| D2 credentials | Dedicated `business_operation_credentials` table            | Extend `TaskRun`/`ExecutionLog` columns; structured `ResultPayload` in log | Log retention is shorter than credentials; coupling credential lifetime to transport lifecycle breaks convergence after log cleanup; log payload isn't independently queryable. |
| D3 envelope    | SDK `GameOperationResult` envelope + capability declaration | Structured error-code mapping only; plugin self-declared as safe           | Error mapping cannot express `unknown`/`conflict`/`queryHint`; a plugin declaring safety without a capability check can't be audited.                                           |
| D4 unknown     | Auto-query (60s×10) → alert; retry cap 8; never substitute  | Retry forever; mark failed after N; query only on operator demand          | Retry forever loops; marking failed fabricates failure; query-only delays convergence.                                                                                          |
| D5 cancel      | Mandatory confirm; `supportsUndo` optional                  | Optional cancel protocol; implicit rollback on cancel                      | Optional cancel lets functions skip confirmation and reproduce R4; implicit rollback is exactly what the design forbids.                                                        |

## 4. Acceptance Mapping (design → contract scenarios)

The contract's §9 acceptance scenarios are satisfied as follows: scenario 1
(lost response never double-grants) by D2+D3 durable dedup + D4 query;
scenario 2 (same ID, changed params, refused) by D1 fingerprint + D3 `conflict`;
scenario 3 (post-approval scope change intercepted) by the snapshot/re-check in
contract §3.2 (unchanged here); scenario 4 (unknown never misjudged) by D4;
scenario 5 (cancellation fabricates no rollback) by D5; scenario 6 (trace chain
complete) by D1 identity + D2 credential linkage; scenario 7 (expired never
resurrected) by contract §4.4 retention + D2.

## 5. Scope of the GAME-010 Implementation

On approval, GAME-010 implements the **platform side** of the proposal:
`operationId` generation, `business_operation_credentials` persistence
(model + numbered migration), the `GameOperationResult` envelope in the Go SDK,
the read-result query path, the auto-query/retry policy (D4), and the
cancellation confirmation boundary (D5). Explicitly **out of scope until a
real game environment is available**: the game-side durable dedup store, the
end-to-end acceptance against a live game (blocked on GAME-002/GAME-008 and
a real integration environment, per todo.md). A Go in-memory/state-file demo is
not a substitute for game-side durable dedup.

## 6. Decisions Needed From the Owner

**Status (2026-11-27): D1–D5 approved by the owner (2026-11-27); GAME-010
platform side implemented.** Platform-side delivery slices:

- `2ae38cc86` — D1/D2: operationId composite identifier + `business_operation_credentials`
  unique store (model + migration 0069)
- `cfca4c5df` — D3: `x-durable-idempotency` capability on the OpenAPI provider
  import path + invoke-chain registration/gating (high-risk functions without a
  declaration fail closed; broadcast rejected)
- `9f7fdaacc` — D4: auto-converge loop (query 60s×10, retry same
  operationId+snapshot 1s→60s capped ×8, exhaustion alerts, unknown never
  rewritten) + migration 0070 (request payload snapshot)
- `403014a24` — D5: game-confirmed cancellation (`canceled`/`cancel_failed`
  only; `cancel_failed` preserves the original result; terminal credentials
  refuse cancellation) + ops endpoints
  `GET/POST /ops/business-operations/:operationId(/cancel)`
- SDK half: proto `durable_query`/`durable_cancel`/`durable_undo` on both
  function descriptors, Go SDK `SetDurableIdempotency` builder + OpenAPI import
  extraction + `GameOperationResult` envelope helpers
  (`sdks/go/pkg/croupier/durable.go`); wire contract documented in
  `docs/architecture/sdk-wire-protocol.md` ("Durable operation envelope").

Known boundaries (honest scope): the game-side durable dedup store and the
end-to-end acceptance against a live game remain gated on GAME-002/GAME-008 and
a real integration environment; only the OpenAPI provider import path and the
SDK descriptor path declare the capability today (the function-contract catalog
does not surface it yet); other-language SDKs do not ship envelope helpers yet.
A Go in-memory/state-file demo is not a substitute for game-side durable dedup.

1. ~~Approve or amend D1–D5 as proposed.~~ **Approved 2026-11-27.**
2. Confirm the credential retention period (proposed: 180 days, configurable) —
   still open; credentials currently retained indefinitely (no TTL pruning).
3. Confirm that GAME-010 proceeds on the platform side even if a real game
   environment is not yet available (the game-side half is separately gated) —
   answered by the 2026-11-27 approval.
