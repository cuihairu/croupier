---
title: Batch Operations and Compensation Design
---

# Batch Operations and Compensation Design

Date: 2026-10-17. Status: **pending design review**. This page completes §4 of the [game console business design backlog](game-operations-design-plan.md), building on the [high-risk business operation execution contract design](high-risk-operation-contract-design.md). Until the review passes, this page modifies no `TaskRun/TaskEvent` structures, adds no batch tables, and implements no mail, compensation, or ban pages.

## 1. Goal and Scope

Batch operations apply one approved business intent to a fixed object set, keeping a recoverable, reconcilable, auditable execution unit per object. First-phase applicability:

- Mail and compensation: grant currency, items, mail attachments, or compensation packages to many players.
- Batch ban/unban: dispose of multiple accounts or characters.
- Batch order compensation: enabled only when the payment/game functions support per-order idempotency and credit verification.

The platform never becomes the game's ledger system, offers no batch database writes bypassing game functions, and never substitutes a batch task's success state for per-object game-side results.

## 2. Batch Identity and Object Snapshots

### 2.1 Identity Model

```text
BatchOperation:
  batchId             # platform batch ID, immutable
  parentOperationId   # optional: incident/upstream cause
  gameId / env
  actor / approver
  businessDomain
  operationType
  functionRef         # functionId + contract version/fingerprint
  selectionSnapshot
  executionPolicy
  budgetSnapshot
  approvalRef
  createdAt / frozenAt / expiresAt

BatchItem:
  batchId
  itemIndex           # stable ordinal within the frozen list
  operationId         # per-object unique business operation ID
  targetSnapshot
  parametersSnapshot
  state
  attempts / lastAttemptAt
  gameReceipt / resultFingerprint
  failure / queryHint
```

`batchId` owns the batch intent, approval, and checkpoints; `operationId` owns each object's game-side idempotency and credentials. Neither changes across retries, recovery, or task restarts.

### 2.2 Object Freezing

Selection results must be frozen into object snapshots before approval submission, including at least:

- Stable identity: account ID, character ID, server/region, channel/platform, external mapping ID.
- Display info: nickname, level, VIP, status — for approver recognition only, never execution identity.
- Target versions: game-side version numbers or snapshot fingerprints such as balance/state/mail eligibility/order status.
- Execution parameters: per-object amount, item, duration, reason, copy, and their fingerprints.
- Exclusion reasons: blacklist, risk control, duplicates, insufficient balance, unmet state, etc.

Saving only filter conditions at approval time is forbidden. If the business allows dynamic lists, the freeze point must be explicit and the approval summary must show the query time, data source, and expiry; recomputing, adding/removing targets, or reordering after approval generates a new batch and requires re-approval.

### 2.3 Duplicate Objects

1. The dedup key is the canonicalized business target + execution parameters — never nickname, row number, or frontend selection sets.
2. Duplicate targets within one batch are merged or rejected at freeze; merges must keep parameters identical and record each object's occurrence count.
3. Cross-batch duplicate execution is not deduplicated by the platform unless the function contract offers query-by-business-key; otherwise risk-control/approval policies may only flag the risk.
4. One account with multiple characters generates one item per character; merging is allowed only when the game function explicitly declares account-level semantics.

## 3. Dry Run

The dry run is backend-generated, immutable audit material — never a frontend count.

| Phase             | Required actions                                                                                            | Side effects          |
| ----------------- | ----------------------------------------------------------------------------------------------------------- | --------------------- |
| Data selection    | Query game/authoritative sources by backend conditions, producing candidate and exclusion lists             | None                  |
| Object validation | Call game read-only verification functions to check eligibility, state, balance, deliverability, blacklists | None                  |
| Aggregation       | Compute target count, per-target amount, total amount, currency, failure/exclusion counts                   | None                  |
| Snapshot          | Save objects, parameters, target versions, sources, query times, and validity                               | Platform records only |
| Freeze            | Fix the snapshot and budget digest at approval submission                                                   | Platform records only |

Dry-run results have a validity period. Once expired, a re-run and re-approval are required before execution; when the function declares support, game-side version numbers may serve incremental verification — but the list is still never silently modified.

## 4. Budget, Rate, and Concurrency

### 4.1 Budget Snapshot

The budget is written into the batch and approval context at freeze, including at least:

- Object count cap and actual count.
- Per-object quantity/amount cap and actuals.
- Total quantity/amount cap and actuals, with currency and decimal precision.
- Counts by type: mail/items/bans etc.
- Per-batch and rolling-window limits, and approval exception IDs.

The executor performs a local atomic deduction before dispatching each item and reconciles actuals when the game's terminal state returns; a game response exceeding budget must stop dispatch and alert. Any parameter change requires a new approval — the executor is never allowed to "truncate the list to the remaining budget" on its own.

### 4.2 Rate and Concurrency

The execution policy must be explicit and can be constrained by game capability:

- Global concurrency, per-game concurrency, per-agent/plugin concurrency.
- Requests per second/minute, in-batch burst windows.
- Game health probe thresholds and circuit-breaker windows.
- Per-object timeout, total batch deadline.
- Pause cooldown and recovery batch size.

The executor must never hold execution leases of multiple same-named batches at once; on recovery it reads per-object states and game query results first, then continues from the first non-terminal object — completed items are never replayed from the start.

### 4.3 Failure Thresholds

The failure policy distinguishes at least these error types:

| Error category                         | Default action                                          |
| -------------------------------------- | ------------------------------------------------------- |
| Parameter/permission/contract conflict | Pause the whole batch immediately; human handling       |
| Target eligibility change              | Mark that object blocked; other objects may continue    |
| Rate limiting/temporary unreachability | Back off and retry; pause past the threshold            |
| Game-explicit final failure            | Record the failure and continue; pause at the threshold |
| Unknown outcome                        | Stop new dispatch; converge by query first              |
| Budget/safety guardrail exceeded       | Pause immediately and alert                             |

Thresholds must state numerator and denominator, e.g. "10 consecutive failures or a 20% total failure rate". Default to pause rather than skip, so risk is never hidden inside success statistics.

## 5. Execution and Recovery States

### 5.1 Batch States

```text
draft → previewing → pending_approval → approved → queued
      → running → paused | stopping
      → completed | partially_completed | failed | canceled | expired
```

A batch terminal state means only that platform scheduling ended — not that every object succeeded. The terminal summary must list the counts and totals of succeeded, finally-failed, unknown, blocked, and canceled objects.

### 5.2 Object States

```text
pending → ready → dispatching → running
       → succeeded | failed | retryable_failed | unknown | blocked | skipped
       \→ cancel_requested → canceled | cancel_failed
```

Every object state transition persists time, requestId, traceId, error code, and the game response. Recovery rules:

1. `succeeded/failed/conflict/canceled` are never replayed.
2. `ready/pending` may be dispatched.
3. `dispatching/running/unknown` first query by operationId or controlled-retry under the same ID — never assumed unexecuted.
4. `blocked/skipped` never change automatically; human handling records the reason.
5. After a platform restart, restore the execution lease and per-object states before requesting rate tokens.

### 5.3 Pause and Resume

- Pausing stops dispatching new items without interrupting dispatched requests; dispatched requests converge under the original state machine.
- Before resuming, re-check approval validity, the function contract, remaining budget, game health, and the list snapshot hash.
- Resume continues the original batch only; changed lists, parameters, or budget create a new batch with re-approval.
- An expired batch dispatches nothing further; remaining objects keep their states with a hint toward a new approval.

## 6. Per-Object Credentials and Final Reconciliation

1. Every item reuses the high-risk execution contract: the same `operationId` + request fingerprint is durably deduplicated by the game, and a lost response never re-executes.
2. Game receipts, request fingerprints, result states, and business change digests must be correlatable across batch, task, execution log, audit, and game queries.
3. Final reconciliation counts separately: platform dispatches, game successes, explicit failures, unknowns, actual granted/disposed totals, and game credential counts.
4. Reconciliation compares more than counts: amounts, currencies, item IDs, targets, batch numbers, and result fingerprints must all agree.
5. `unknown` is never re-judged as failed when the query window closes; it stays unknown and enters the manual/extended-query flow.
6. The reconciliation report is generated after the batch's terminal state; failed objects support export, cloning into a new batch draft, and referencing the original operationId — approvals are never inherited automatically.

## 7. Pages and Permissions

- The batch pages provide: condition editing, dry runs, snapshot diffs, approval summaries, execution progress, per-object filtering, failure handling, and reconciliation reports.
- The approval page must show the snapshot digest, total caps, data source/query time, validity, and function contract version — never just a task name or a frontend count.
- The execution page is read-only by default; retry, pause, resume, cancel, export, and compensation are each permission-controlled.
- Sensitive targets and parameters are masked by default; auditors can view full credentials via a controlled path.
- Batch operations mount only to published pages of the current `(gameId, env)`; execution contexts are never reused across scopes.

## 8. Acceptance Scenarios

1. **Failure recovery**: the batch crashes after object 37/100; after restart the first 36 credentialed successes are not replayed, object 37 is queried first, and execution continues only after its non-terminal state is resolved — exactly 100 grants in total.
2. **List changes**: adding/removing/replacing any target or changing per-target amounts after approval blocks execution; a new snapshot and re-approval are required.
3. **Per-object reconciliation**: the game returns 98 succeeded, 1 failed, 1 unknown; the batch is partially completed, the report shows receipt/error/query position per item, and the total equals 98 actual successes.
4. **Lost response**: object 50 credited but its response was lost; the state is unknown; a same-operationId retry returns the original credential and converges to success — never granted twice.
5. **Budget guardrail**: the game returns an amount or count exceeding the approved budget; the executor pauses immediately, later objects are undispatched, and the alert includes the offending item and consumed budget.
6. **Pause and resume**: the 5 dispatched objects keep converging during the pause; after resume, execution continues from the 6th non-terminal object with rate and concurrency re-governed.
7. **Failure threshold**: consecutive eligibility changes reaching the policy threshold pause the batch; succeeded objects are not rolled back, and blocked objects can be exported.

## 9. Decisions Pending Review

1. The first business domains, maximum object counts, and default rate/concurrency/failure thresholds.
2. The storage form of `BatchOperation/BatchItem`, and whether per-object events go into `TaskEvent` or a separate business event.
3. The dry run's default validity, game version-based incremental verification capability, and expiry policy.
4. Who owns batch retry permissions: the initiator, on-duty ops, the approver, or an administrator.
5. Whether compensation drafts can be one-click created from failed/unknown objects, and which original batch fields the re-approval summary must show.
