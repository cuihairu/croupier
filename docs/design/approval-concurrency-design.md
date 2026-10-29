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
> Status: **DECIDED** (2026-10-29). Drives ARCH-002 implementation.
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
  idempotency key, so every caller that reaches this point executes the
  function once.

Consequences (all real, all reachable):

| #   | Scenario                                                                                                    | Current behavior                                                                                                                 | Harm                                                                                                                                           |
| --- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Two operators approve the same pending request concurrently                                                 | Both read `pending`, both write `approved`, both run the continuation                                                            | **Function executes twice** — double side effect (e.g. double recharge / double ban).                                                          |
| 2   | Approve races Reject                                                                                        | Both read `pending`; last writer wins; if Approve's write lands last, the continuation runs even though a rejection was recorded | Undecidable final state; audit chain shows both `approved` and `rejected` rows for one request; execution proceeds against a rejected request. |
| 3   | Duplicate approve (same operator double-click, or retry)                                                    | Second `Approve` overwrites `Approver`/`ReviewedAt` and re-runs the continuation                                                 | **Function executes twice**; reviewer attribution and timestamps corrupted.                                                                    |
| 4   | Approve succeeds but the continuation fails / server crashes between the approve-save and the result Update | `state="approved"` with no `ResultKind`/`TaskID`/`Result`; `Reason` may carry `approved but continuation failed`                 | "Approved but result unknown" — no recovery path: the operator cannot tell whether the function ran.                                           |

## 2. Decisions

### D1 — Atomic pending→terminal conversion (CAS)

`Approve` and `Reject` must be **single-statement conditional updates**:

```sql
UPDATE approval SET state = :new, approver = :operator, reviewed_at = :now, reason = :reason
WHERE id = :id AND state = 'pending'
```

- `SQLStore` implements with `db.Model(&ApprovalModel{}).Where("id = ? AND state = 'pending'", id).Update(...)`
  and inspects `RowsAffected`. This is portable across sqlite/mysql/postgres and
  needs no `SELECT ... FOR UPDATE` (single-statement atomicity is sufficient —
  there is no read-then-write window to lock).
- `MemStore` implements the same rule under its existing `mu` lock: reject the
  transition if `a.State != "pending"`.
- When `RowsAffected == 0` (or the MemStore state guard fails), the store
  re-reads the record and returns it together with a typed sentinel
  `ErrAlreadyReviewed` (carrying the current state/approver/reviewedAt).
  The caller (service) maps this to an HTTP 409 and does **not** continue.
- The store must not mutate the record on a failed CAS; a failed CAS is
  read-only.

**Decision:** the first review to flip `pending → terminal` wins. The losing
request (approve-vs-approve, approve-vs-reject, duplicate approve/reject) gets
a stable conflict; it performs no continuation, writes no audit line, sends no
notification.

### D2 — Continuation is claimed by the transition winner only

`Service.Approve` must treat a successful CAS (`RowsAffected == 1`) as the sole
authorization to run `continueApprovedFunction`. A `ErrAlreadyReviewed` result
must short-circuit before any continuation / audit / notify side effect.

The continuation invoke (`FunctionInvoke` with
`metadata["approvalBypass"]="approved"`, `runtime_api=approval.continue`) keeps
its current shape, but **only the winner calls it**. This is the fix for
double-execution: the invoke gate must not be reachable from a losing review.

### D3 — Unknown-execution-result semantics

If the winner's `continueApprovedFunction` errors:

- `state` stays `approved` (the decision is final — already recorded in the
  audit chain via `EventApprovalApproved`).
- `Reason` is set to `approved but continuation failed: <err>` (existing
  behavior, preserved).
- The record keeps `ResultKind`/`TaskID`/`Result` empty, so the console
  renders "approved, result unknown" rather than a fabricated result.
- Recovery is **not** via re-approval (the state is no longer `pending`, so a
  re-approve hits the CAS and returns 409). Recovery is either:
  - task mode: the operator queries the task by `TaskID` for the real result
    (task-mode continuations return `TaskID`; the approval row persists it);
  - sync mode: the operator re-submits the original function invocation (a
    fresh approval if the risk gate still requires one) — the decision and the
    execution result stay decoupled.

This intentionally keeps "decision" and "execution result" as two phases: a
review decision is never rolled back because the downstream run failed.

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

### D5 — Pre-checks stay on the pre-read; CAS is the final gate

Scope check, self-approval rejection, and step-up OTP enforcement all read the
record via `Get` before the CAS (current behavior). They are advisory and stay
as-is. The CAS is the **only** gate that decides "who won". No new pre-check is
introduced; the ordering (pre-read → CAS → winner-only continuation) is the
invariant.

### D6 — What stays unchanged

- `Create`/`List`/`Get`/`Update` shapes and the `Approval` fields
  (`State`, `Approver`, `ReviewedAt`, `Reason`, `ResultKind`, `TaskID`,
  `Result`).
- The continuation payload/metadata (approvalBypass, approvalId,
  approvalActor, runtime_api).
- The audit events (`EventApprovalApproved`/`EventApprovalRejected`), the
  extension upsert, and the notification events — but all fire only on the
  winner's path (a loser fires none).

## 3. Review table (the ARCH-001 deliverable)

| Case                                                                            | Pre-condition                              | Winner behavior                                                                                          | Loser / duplicate behavior                                                                               | Post-state                            | Execution count                                                                                                        |
| ------------------------------------------------------------------------------- | ------------------------------------------ | -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| Approve vs Approve (two operators, concurrent)                                  | two requests read `pending`                | first CAS wins; audit `approved`, continuation runs, `ResultKind`/`TaskID` recorded                      | second CAS `RowsAffected==0` → 409 `approval_already_reviewed`; **no continuation, no audit, no notify** | `approved`                            | **exactly 1**                                                                                                          |
| Approve vs Reject (concurrent)                                                  | one approve + one reject on `pending`      | first CAS wins; its decision + continuation (if approve won)                                             | loser → 409 `approval_already_reviewed`; no side effect                                                  | `approved` or `rejected` (first-wins) | 1 if approve won, else 0                                                                                               |
| Duplicate approve (same operator, double-click / retry)                         | record already `approved`                  | first request's continuation already dispatched                                                          | second → 409; returns existing `approver`/`reviewedAt`                                                   | `approved`                            | 1 (total)                                                                                                              |
| Duplicate reject                                                                | record already `rejected`                  | —                                                                                                        | second → 409; returns existing reason/approver                                                           | `rejected`                            | 0                                                                                                                      |
| Approve succeeds, continuation fails (agent down / contract stale / task error) | CAS won, `continueApprovedFunction` errors | `state=approved`, `Reason=approved but continuation failed: ...`, `ResultKind` empty                     | —                                                                                                        | `approved` (decision final)           | 0 (continuation failed); result recoverable via `TaskID` (task mode) or re-submit (sync mode)                          |
| Approve succeeds, server crashes before result Update                           | CAS won, no `ResultKind`/`TaskID` written  | on restart the record is `approved` with empty result fields; console renders "approved, result unknown" | —                                                                                                        | `approved`                            | 0..1 (indeterminate by design — the decision is final; the execution outcome is recovered via task query or re-submit) |

## 4. Implementation notes for ARCH-002

1. Add `ErrAlreadyReviewed = errors.New("approval already reviewed")` to
   `internal/platform/approvals/store.go` (exported, with a helper to extract
   the conflicting record, e.g. `AsAlreadyReviewed(err) (*Approval, bool)`).
2. Change `SQLStore.Approve`/`Reject` to the conditional UPDATE +
   `RowsAffected` check (re-read on 0 rows).
3. Change `MemStore.Approve`/`Reject` to guard `a.State == "pending"` under the
   existing lock.
4. `Service.Approve`: on `ErrAlreadyReviewed`, return
   `errorx.NewConflict("approval_already_reviewed", ...)` with `details` =
   current state/approver/reviewedAt; **skip** continuation, audit, extension
   upsert, event, and notify. Only the `RowsAffected==1` path proceeds to
   `continueApprovedFunction`.
5. `Service.Reject`: same conflict mapping; no side effects on a losing reject.
6. Tests: a concurrency test that fires N concurrent `Approve` on one record
   and asserts exactly one continuation (e.g. via a fake that counts
   `continueApprovedFunction` calls or a `FunctionInvoke` spy), plus
   approve-vs-reject and duplicate-approve cases asserting 409 and exactly-one
   execution. `MemStore` and `SQLStore` both need the CAS unit tests.
7. The two-person/multi-approval accumulation (if a workflow requires N
   approvals before `approved`) is **not** part of this gate — the invoke-gate
   `Store` is single-approve. If a future workflow needs quorum, that is a new
   `workflow_store.go` concern, not a change to `Store.Approve`.
