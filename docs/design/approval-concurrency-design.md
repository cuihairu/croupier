# Approval Concurrency & Execution Claim Semantics

> ARCH-001 · P1 · 方案决策 · 审批并发与执行领取语义
>
> Scope: the function-invoke approval gate (`internal/api/approval/service.go`
> `Service.Approve`/`Reject` → `continueApprovedFunction`) and its persistence
> (`internal/platform/approvals/store.go` `Store` interface, `SQLStore` and
> `MemStore`). The workflow engine (`internal/platform/approvals/workflow.go`,
> multi-step workflows with expiry/escalation) is a separate feature and is
> deliberately out of scope here.
>
> Status: **PROPOSED — NOT IMPLEMENTED** (reviewed 2026-10-29).
> ARCH-001 remains open until durable execution claim/recovery is resolved;
> this document does not establish production readiness.
> Prerequisite for: ARCH-002 (接入审批 SQL 存储).

## 1. Problem

The invoke-approval gate has no concurrency protection. Both the SQL and the
in-memory store convert `pending → approved|rejected` by an unconditional
read-then-write, and the service triggers the continuation run for **every**
caller of `Approve`, not just the one that performed the state transition.

Evidence:

- `internal/platform/approvals/sql_store.go` `Approve`: `First(id)` then
  unconditional `Save(state="approved")`. `Reject` identical. No
  `WHERE state='pending'`, no row lock, no affected-rows check.
- `internal/platform/approvals/store.go` `MemStore.Approve`: under the lock,
  unconditional `a.State = "approved"` overwrite — no state guard either.
- `internal/api/approval/service.go` `Approve`: after
  `ApprovalsStore.Approve(...)` returns, it unconditionally calls
  `continueApprovedFunction(ctx, record)`. The continuation invoke carries no
  idempotency key, so each successful caller can dispatch another invocation. Actual game-side
  effects depend on dispatch success and the game implementation.

Static findings (the following interleavings follow from the code; real multi-instance fault injection is NOT RUN):

| #   | Scenario                                                                                                    | Current behavior                                                                                                                 | Harm                                                                                                                                           |
| --- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Two operators approve the same pending request concurrently                                                 | Both read `pending`, both write `approved`, both run the continuation                                                            | **Function executes twice** — double side effect (e.g. double recharge / double ban).                                                          |
| 2   | Approve races Reject                                                                                        | Both read `pending`; last writer wins; if Approve's write lands last, the continuation runs even though a rejection was recorded | Undecidable final state; audit chain shows both `approved` and `rejected` rows for one request; execution proceeds against a rejected request. |
| 3   | Duplicate approve (same operator double-click, or retry)                                                    | Second `Approve` overwrites `Approver`/`ReviewedAt` and re-runs the continuation                                                 | **Function executes twice**; reviewer attribution and timestamps corrupted.                                                                    |
| 4   | Approve succeeds but the continuation fails / server crashes between the approve-save and the result Update | `state="approved"` with no `ResultKind`/`TaskID`/`Result`; `Reason` may carry `approved but continuation failed`                 | "Approved but result unknown" — no verified automatic recovery protocol; the outcome may require reconciliation.                               |

## 2. Decisions

### D1 — Atomic pending→terminal conversion (CAS)

`Approve` and `Reject` must be **single-statement conditional updates**:

```sql
UPDATE approval SET state = :new, approver = :operator, reviewed_at = :now, reason = :reason
WHERE id = :id AND state = 'pending'
```

- `SQLStore` implements with `db.Model(&ApprovalModel{}).Where("id = ? AND state = 'pending'", id).Updates(...)`
  and inspects `RowsAffected`. Supported database behavior still requires integration verification; it
  needs no `SELECT ... FOR UPDATE` (single-statement atomicity is sufficient —
  there is no read-then-write window to lock).
- `MemStore` implements the same rule under its existing `mu` lock: reject the
  transition if `a.State != "pending"`.
- When `RowsAffected == 0` (or the MemStore state guard fails), the store
  re-reads the record (missing IDs remain not-found, database errors remain errors) and returns it together with a typed sentinel
  `ErrAlreadyReviewed` (carrying the current state/approver/reviewedAt).
  The caller (service) maps this to an HTTP 409 and does **not** continue.
- The store must not mutate the record on a failed CAS; a failed CAS is
  read-only.

**Decision:** the first review to flip `pending → terminal` wins. The losing
request (approve-vs-approve, approve-vs-reject, duplicate approve/reject) gets
a stable conflict; it performs no continuation, writes no audit line, sends no
notification.

### D2 — Continuation is claimed by the transition winner only

`Service.Approve` must treat a successful CAS (`RowsAffected == 1`) as the winning decision required, after mandatory authorization, to run `continueApprovedFunction`. A `ErrAlreadyReviewed` result
must short-circuit before any continuation / audit / notify side effect.

The continuation invoke (`FunctionInvoke` with
`metadata["approvalBypass"]="approved"`, `runtime_api=approval.continue`) keeps
its current shape, but **only the winner calls it**. This prevents losing reviews from dispatching a continuation. CAS alone
does not guarantee durable dispatch or exactly-once game effects after a crash.

### D3 — Unknown-execution-result semantics

An error or timeout does not establish that the game operation failed. Keep the
approval decision final, preserve every known operation/task identifier and
receipt, and report execution as unknown until reconciled. Do not erase known
result fields or turn an unknown outcome into a fabricated failure.

- Query the original task when its `TaskID` was durably recorded. The current
  service writes it only after continuation returns; a crash may leave no ID.
- For synchronous operations, query the game-side durable receipt using the
  original operation identity. Without such a contract the result remains
  unknown and requires manual reconciliation.
- Never treat a fresh approval or a fresh invocation as a safe retry. Retry is
  permitted only under a separately verified durable idempotency contract using
  the same operation identity (GAME-009/010).
- ARCH-001 must decide the durable execution intent/claim and recovery protocol
  before ARCH-002 claims crash-safe dispatch. Persist the original identity and
  payload fingerprint before dispatch, define ownership/fencing and recovery,
  and reconcile dispatch/result updates. Storage migrations may be required.

Current evidence: `Service.Approve` ignores result `Update` failures and writes
`Reason` on continuation errors; `recordApprovalAudit` is best effort, so even
an approved decision does not prove the audit write succeeded. The current
console's exact unknown-state presentation remains to be verified.

### D4 — Duplicate-request rule

A duplicate `Approve`/`Reject` (same or different operator, on a record already
in a terminal state) is **not** a re-execution trigger:

- `ErrAlreadyReviewed` → HTTP 409 `approval_already_reviewed` with the current
  `state`/`approver`/`reviewedAt` in `details`. The caller sees the existing
  decision and can reconcile (e.g. render "already approved by X").
- Idempotency is **not** "second request succeeds silently" — the first
  decision stands, the duplicate is rejected with a conflict so the client
  never mistakes it for a fresh approval. Rationale: silent success on a
  duplicate would hide that a continuation was already dispatched; the conflict
  makes the double-execution risk explicit at the UI layer.

### D5 — Mandatory authorization before the transition

Scope, actor identity, self-approval rejection and step-up OTP checks remain
mandatory. CAS chooses the winning decision; it does not replace authorization.
If scope or approved payload can change between pre-read and transition, bind
the transition to the authorized revision/fingerprint or revalidate atomically.

### D6 — Compatibility and open storage decisions

Preserve the continuation metadata and winner-only decision audit/notification
intent. Conflicting requests must not emit a successful decision event; security
logging of rejected attempts remains allowed. Existing payload compatibility is
a goal, not a promise that no schema migration is needed. Durable claim fields,
transaction boundaries and recovery remain ARCH-001 design work, with
GAME-009/010 providing the game-side operation/receipt boundary.

## 3. Review table (the ARCH-001 deliverable)

| Case                                                                            | Pre-condition                              | Winner behavior                                                                                                                          | Loser / duplicate behavior                                                                               | Post-state                            | Execution count                                                                                   |
| ------------------------------------------------------------------------------- | ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ------------------------------------- | ------------------------------------------------------------------------------------------------- |
| Approve vs Approve (two operators, concurrent)                                  | two requests read `pending`                | first CAS wins; audit `approved`, continuation runs, `ResultKind`/`TaskID` recorded                                                      | second CAS `RowsAffected==0` → 409 `approval_already_reviewed`; **no continuation, no audit, no notify** | `approved`                            | At most one continuation attempt; game result needs a receipt                                     |
| Approve vs Reject (concurrent)                                                  | one approve + one reject on `pending`      | first CAS wins; its decision + continuation (if approve won)                                                                             | loser → 409 `approval_already_reviewed`; no side effect                                                  | `approved` or `rejected` (first-wins) | At most one continuation attempt if approved; otherwise 0                                         |
| Duplicate approve (same operator, double-click / retry)                         | record already `approved`                  | first request's continuation already dispatched                                                                                          | second → 409; returns existing `approver`/`reviewedAt`                                                   | `approved`                            | No additional attempt from the duplicate                                                          |
| Duplicate reject                                                                | record already `rejected`                  | —                                                                                                                                        | second → 409; returns existing reason/approver                                                           | `rejected`                            | 0                                                                                                 |
| Approve succeeds, continuation fails (agent down / contract stale / task error) | CAS won, `continueApprovedFunction` errors | `state=approved`, `Reason=approved but continuation failed: ...`, `ResultKind` empty                                                     | —                                                                                                        | `approved` (decision final)           | Unknown; query original task/receipt when available; no blind retry                               |
| Approve succeeds, server crashes before result Update                           | CAS won, no `ResultKind`/`TaskID` written  | on restart the record is `approved` with empty result fields; console should render "approved, result unknown" (UI verification pending) | —                                                                                                        | `approved`                            | Unknown; durable intent/receipt reconciliation required; missing IDs require manual investigation |

## 4. Implementation notes for ARCH-002

1. Add `ErrAlreadyReviewed = errors.New("approval already reviewed")` to
   `internal/platform/approvals/store.go` (exported, with a helper to extract
   the conflicting record, e.g. `AsAlreadyReviewed(err) (*Approval, bool)`).
2. Change `SQLStore.Approve`/`Reject` to the conditional UPDATE +
   `RowsAffected` check (re-read on 0 rows).
3. Change `MemStore.Approve`/`Reject` to guard `a.State == "pending"` under the
   existing lock.
4. `Service.Approve`: on `ErrAlreadyReviewed`, return
   an HTTP 409 error with stable code `approval_already_reviewed` (using the actual error helper contract) with `details` =
   current state/approver/reviewedAt; **skip** continuation, audit, extension
   upsert, event, and notify. Only the `RowsAffected==1` path proceeds to
   `continueApprovedFunction`.
5. `Service.Reject`: same conflict mapping; no side effects on a losing reject.
6. Tests: a concurrency test that fires N concurrent `Approve` on one record
   and asserts exactly one continuation (e.g. via a fake that counts
   `continueApprovedFunction` calls or a `FunctionInvoke` spy), plus
   approve-vs-reject and duplicate-approve cases asserting 409 and exactly-one
   continuation attempt under a no-crash test. `MemStore` and `SQLStore` both need the CAS unit tests.
7. The two-person/multi-approval accumulation (if a workflow requires N
   approvals before `approved`) is **not** part of this gate — the invoke-gate
   `Store` is single-approve. If a future workflow needs quorum, that is a new
   `workflow_store.go` concern, not a change to `Store.Approve`.

Implementation acceptance also requires crash-before-dispatch, response-loss and result-write-failure cases, plus mandatory authorization failures. These are NOT RUN; this document changes no runtime behavior.
