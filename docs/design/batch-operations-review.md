---
title: Batch Operations Review
---

# Batch Operations Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. This page is
the GAME-011 方案决策 deliverable. It reviews
[batch-operations-compensation-design](batch-operations-compensation-design.md)
(still `pending design review`), confirms its already-drafted rulings as
binding, resolves its §9 "Decisions Pending Review" into concrete
recommendations, and closes the one gap the design leaves open (units and
multipliers). No code is changed by this page — implementation is gated on the
owner's approval.

Implementation reality (verified 2026-11-28): zero batch surface exists in
code — no `BatchOperation`/`BatchItem` models, tables, or handlers anywhere in
`internal/`. The design doc is the only artifact; everything below is
pre-implementation.

## 1. What the design already settles (confirmed binding, not re-decided)

GAME-011's review checklist maps onto the design doc as follows. These rulings
stand as written; this review adds no alternatives to them:

| GAME-011 ask                                 | Design ruling (binding)                                                                                                                                                                                |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Roster snapshot (名单快照)                   | §2.2: freeze object snapshots (stable identity + display info + target versions + execution parameters + exclusion reasons) **before** approval submission; saving only filter conditions is forbidden |
| Dry-run count/budget (预演)                  | §3: backend-generated immutable dry run (selection → validation → aggregation → snapshot → freeze); never a frontend count; validity period enforced                                                   |
| Post-approval changes (审批后变化)           | §2.2 + scenario 2: any target add/remove/replace or parameter change after approval blocks execution; new snapshot + re-approval required                                                              |
| Rate/concurrency (速率并发)                  | §4.2: explicit policy (global/per-game/per-plugin concurrency, RPS, burst, breaker, deadlines); never hold two leases of the same batch                                                                |
| Failure thresholds (失败阈值)                | §4.3: per-error-category default actions; thresholds state numerator+denominator; default pause over skip                                                                                              |
| Pause/resume (暂停恢复)                      | §5.3: pause stops new dispatch only; resume re-checks approval validity, budget, game health, snapshot hash; changes create a new batch                                                                |
| Per-object credentials (逐对象凭据)          | §6: every item reuses the high-risk execution contract (operationId + fingerprint dedup); final reconciliation counts successes/failures/unknowns separately                                           |
| Post-freeze re-approval (冻结后变更重新审批) | Same as "post-approval changes" — binding                                                                                                                                                              |

## 2. The five §9 decisions (recommendations for sign-off)

### D1 — First business domains, object caps, default thresholds

- **Domains: mail + compensation first.** Both are grant-shaped (one
  operationId semantic), exercise the full snapshot→dry-run→approval→dispatch
  chain, and match the design's §1 ordering. Batch ban is second (account
  disposal has its own review surface); batch order compensation last (needs
  per-order idempotency + credit verification from the payment side first).
- **Object cap: default 5,000 per batch, config-gated** (hard ceiling for v1;
  larger lists decompose into multiple batches — each independently approved).
  The cap is a dispatch-plane guardrail, not a business limit.
- **Default thresholds: adopt §4.3's table as the shipped default policy**
  (10 consecutive failures or 20% total failure rate → pause; parameter/
  permission/contract conflict and budget-exceeded → immediate pause+alert).

### D2 — Storage form: `BatchOperation`/`BatchItem` as dedicated tables; per-object events separate from `TaskEvent`

- New models `batch_operations` / `batch_items` (numbered migrations per the
  schema-migration contract; idempotent column catches modeled on 0015/0021/0025/0027).
- **Per-object events do NOT go into `TaskEvent`.** `TaskEvent` carries task
  progress/logs for the task runner's consumers; a batch item's state machine
  (§5.2: pending→ready→dispatching→running→…, cancel branch) has different
  states, different query shapes, and a different retention profile. Overloading
  it risks breaking task consumers and force-sharing indexes. Batch events get
  their own table keyed by (batchId, itemId, seq) — batch replay/recovery reads
  it, task pages never do.
- `batchId` links to `TaskRun` where a batch dispatches through the task
  plane, but batch identity and task identity remain distinct (a task retry
  must never alias a batch object's terminal state).

### D3 — Dry-run validity and expiry

- **Default validity 24h, config-gated per domain.** Expired → re-run dry run
  - re-approval (design §3 already mandates; this fixes the number).
- Incremental verification via game version numbers is enabled **only when
  the function contract declares version support** (design's own condition);
  the list is still never silently modified.

### D4 — Retry permission owner

- **On-duty ops with a dedicated permission key** (`batch.retry` in the
  existing RBAC surface) is the primary owner: they hold the operational
  context and are neither the initiator (self-approval pressure) nor the
  approver (segregation of duties).
- The initiator may retry **their own** batch only while it is non-expired;
  the approver is not auto-granted retry (approving and re-dispatching are
  different duties); administrators act through the existing admin family.
- Retry = dispatch of non-terminal objects only (§5.2 recovery rules;
  succeeded/failed/conflict/canceled are never replayed). Every retry writes
  audit with actor + scope.

### D5 — Compensation drafts from failed/unknown objects

- **Yes — one-click creates a DRAFT, never an approved batch.** The draft
  carries: source batchId, per-object original operationId + receipt/query
  position, the original snapshot digest, and fresh freeze + dry run of its own.
- The re-approval summary must show (design §6.6 + this addition): source
  batch, count of failed vs unknown objects, why each is safe to re-dispatch
  (failed with non-delivery evidence / unknown converged by query), total
  budget of the new batch, and the function contract version.
- Approvals are never inherited (binding, §6.6).

## 3. Units and multipliers (the design's open gap)

The design names "currency and decimal precision" in §4.1 but leaves
multiplier semantics open. Proposal:

- Amounts are stored as **integer minor units or exact decimal strings +
  currency code** — never floats (money in floats is a defect class, not a style).
- **Multipliers resolve before freeze**: a compensation package ×N, a per-VIP
  tier coefficient, etc. are computed into concrete per-object amounts at
  dry-run/snapshot time; the snapshot stores resolved amounts (with the
  multiplier recorded as provenance). The executor dispatches resolved amounts
  only — it never multiplies at dispatch time.
- Post-freeze multiplier change = parameter change = new snapshot +
  re-approval (the binding §2.2 rule applies unchanged).
- Currency conversion is out of scope for v1: a batch is single-currency;
  mixed-currency lists are excluded at freeze with explicit reasons.

## 4. Dependencies (implementation order)

1. **GAME-009 protocol** (operationId + fingerprint dedup + receipts) — batch
   §6 reuses it per-item; without it, exactly-once claims are impossible.
   GAME-010 implementation is the follow-on and is itself partially delivered
   ("仍 NOT RUN" against a real game per todo).
2. **Approval chain** (existing SQLStore CAS workflow) — unchanged consumer.
3. Batch tables + pages land only after this review is approved (design
   header's own gate).

## 5. Acceptance confirmation

Design §8's seven scenarios stand as the implementation's verification bar
(recovery, list changes, per-object reconciliation, lost response, budget
guardrail, pause/resume, failure threshold). This review adds one:

8. **Units/multiplier**: a package ×3 freeze resolves per-object amounts at
   snapshot time; a multiplier change after approval blocks execution and
   requires re-approval; the reconciliation total equals the sum of resolved
   amounts, in the batch's single currency.

## 6. Decisions needed from the owner

- [ ] **D1** domains/caps/thresholds — approve mail+compensation first, 5,000-object default cap, §4.3 table as default policy
- [ ] **D2** storage form — approve dedicated `batch_operations`/`batch_items` tables with a separate batch-event table (not `TaskEvent`)
- [ ] **D3** dry-run validity — approve 24h default, config-gated
- [ ] **D4** retry owner — approve on-duty ops with dedicated key; initiator own-batch within validity; approver not auto-granted
- [ ] **D5** compensation drafts — approve draft-only creation with the listed re-approval summary contents
- [ ] **D6** units/multipliers — approve integer minor units / decimal strings, resolve-before-freeze, single-currency batches
- [ ] Design §8 scenarios + §5 scenario 8 — confirm as the implementation's verification bar
