---
title: Risk Control Notes
---

# Risk Control Notes

This document describes only capabilities implemented in the current repository. Risk-control definition, publishing, execution, hits and disposition are split between the control plane and the Agent execution plane; rule definition and execution are separated, and the platform currently does not automatically block database writes, ban accounts, or call game-side disposition APIs when a rule hits.

## Capability Overview

| Control point             | Basis                                                                                                                                      | Status                                                                    |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------- |
| CDC event filter          | `event.ops` operation whitelist; listed-but-legit operations do not reach later gates, unknown ops are dropped with an error count         | Implemented; empty list allows everything by default                      |
| Reconcile gate            | Item table changes must match ledger changes within a time window; unmatched over-window changes alert                                     | Implemented; alerts only, no blocking                                     |
| CEP three templates       | Off-whitelist-hours changes, single or windowed cumulative bursts, repeated small-amount probing                                           | Implemented; alerts only, no blocking                                     |
| Lineage gate              | Writes to monitored tables must come from whitelisted accounts                                                                             | Degraded: CDC events carry no session account; always skipped and counted |
| Rule publishing           | Versioned `capture.gates` publishing; dry-run, scope, capture-table and connector-capability prechecks                                     | Implemented; no approval workflow                                         |
| Risk event handling       | Hits generate alerts; acknowledge, ignore, escalate with notes leave an audit trail                                                        | Implemented; auto ban/compensation/blocking not implemented               |
| Function risk rating      | Functions and operations are rated safe/warning/high/danger; declaration, normalization and governance resolution are three defined layers | Implemented                                                               |
| High-risk policy approval | high/danger invocations are forced into the approval flow with two-person review plus TOTP step-up, fully audited                          | Implemented; `capture.gates` publishing approval not implemented          |

## Risk Rules

### Event filter

- `agents/capture/filter/event.go:5` defines the `EventFilter`, which only makes cheap deterministic decisions; `agents/capture/filter/event.go:11` states that unknown operations are `ResultError` and must be dropped with a count, never silently swallowed.
- `agents/capture/filter/event.go:18` constructor accepts only `insert/update/delete`; an empty list keeps everything allowed; `agents/capture/filter/event.go:35` performs the decision.
- `agents/capture/config/gates_validate.go:43` builds the event filter during ruleset validation; `agents/capture/config/gates_validate.go:18` is the shared validation truth for server dry-run and Agent apply.

### Reconcile gate

- `agents/capture/gate/reconcile.go:41` defines "item changes must match ledger changes"; `agents/capture/gate/reconcile.go:46` documents that an unobtainable item key enters a never-match sentinel — prefer false positives over misses.
- `agents/capture/gate/reconcile.go:48` rule fields cover the two tables, both-side keys, `WindowSec`, `Severity`, exemption keys and `MaxPending`; `agents/capture/gate/reconcile.go:60` validates required fields, table shapes, severity and caps.
- `agents/capture/gate/reconcile.go:36` sets the default 300-second window and a 10000 in-window cap per rule; `agents/capture/gate/reconcile.go:91` and `agents/capture/gate/reconcile.go:99` apply explicit config or defaults.
- `agents/capture/gate/reconcile.go:107` forbids routing one table through two reconcile rules; `agents/capture/gate/reconcile.go:164` documents the bidirectional time-window clearing — over-window residue raises a `reconcile.missing_ledger` alert.

### CEP rules

- `agents/capture/gate/cep.go:13` fixes the template closed set to `cep.off_hours`, `cep.burst_delta`, `cep.probing`; `agents/capture/gate/cep.go:24` caps cumulative buckets and tracked keys at 10000 each.
- Off-hours rule: `agents/capture/gate/cep.go:34` defines table/operation × whitelisted hours; `agents/capture/gate/cep.go:80` validates hours, timezone offset and severity.
- Burst rule: `agents/capture/gate/cep.go:45` supports `single` and `window`; `agents/capture/gate/cep.go:50` requires `Threshold > 0`, window mode additionally `WindowSec > 0`; `agents/capture/gate/cep.go:105` completes static validation.
- Repeated probing: `agents/capture/gate/cep.go:62` detects consecutive small changes on the same table and key within a window; `agents/capture/gate/cep.go:67` degrades to row key when the account is unavailable; `agents/capture/gate/cep.go:137` requires `Count >= 2` and a positive window.

### Lineage gate

- `agents/capture/gate/lineage.go:12` defines writes to monitored tables from non-whitelisted accounts as anomalous.
- `agents/capture/gate/lineage.go:15` states that current binlog/WAL events carry no session account; `agents/capture/gate/lineage.go:38` requires a non-empty `allowAccounts`, but at runtime `agents/capture/gate/lineage.go:89` skips empty accounts and increments the `Degraded` counter.
- The lineage gate therefore currently produces no effective risk events; enabling it only makes the whitelist config and degradation counter observable.

## Risk Rating

Function and operation risk has four levels with semantics fixed at `internal/dashboard/spec/types.go:78`: `safe` is read-only with no side effects, `warning` moderate side effects, `high` significant side effects requiring confirmation, and `danger` destructive or irreversible and requiring approval (`internal/dashboard/spec/types.go:87`).

- The declaration entry is the OpenAPI `x-risk` extension: `internal/drivers/openapi/provider.go:192` defines the field uploaded with the function contract.
- Normalization lives in `internal/dashboard/normalizer/normalizer.go:300`: legacy aliases `low/medium/critical` are accepted (kept in sync with `ParseRisk` at `internal/dbenum/p1_enums.go:264`); unknown values normalize to empty and persist as safe.
- Governance resolution lives in `internal/platform/functioninvoke/policy.go:35`: priority is the in-memory OpenAPI `x-risk` → the contract row's non-safe risk (a safe zero value is not trusted) → fallback medium; `ResolvePolicyRisk` at `internal/platform/functioninvoke/policy.go:72` exposes the same semantics to the approval path so packages never copy the logic.
- The rating directly drives generated pages: `internal/dashboard/generator/resource_generator.go:851` renders danger-level buttons for high/danger, and `internal/dashboard/generator/resource_generator.go:860` forces confirmation for high/danger and whenever the approval policy is Required.
- Runtime policy can be overridden per function: `internal/model/function.go:72` distinguishes `default` (auto-generated from the risk level) from `manual`, with overrides covering `RequireApproval`, `ApprovalWorkflow` and `RequireAudit`.

## Policy Approval

High-risk invocations do not execute directly: `internal/platform/functioninvoke/invoke.go:125` routes the call into the approval flow when the function policy has `RequireApproval` and the approval store is ready; the caller receives a ticket number. The approval surface and rules:

- Approval API: `internal/handler/routes.go:229` mounts `/approvals`; `internal/handler/routes.go:856` through `:859` register the List/Get/Approve/Reject endpoints, implemented at `internal/api/approval/service.go:38`, `:159`, `:198` and `:273`.
- Two-person rule: `internal/api/approval/service.go:120` rejects self-approval and logs an audit entry (`:138`); `ApprovalConfig.AllowSelfApprove` at `internal/config/config.go:865` defaults to false and is recommended only for explicitly single-admin deployments.
- High-risk step-up TOTP: `internal/api/approval/stepup_otp.go:23` requires second-factor verification to approve governance risk high/danger (sharing the `ResolvePolicyRisk` semantics with the invoke path, `:66`); accounts with TOTP enrolled must submit a valid otp, accounts without are rejected outright with `otp_not_enrolled` (`internal/api/approval/stepup_otp.go:123`) — high-risk approval must never pass silently.
- Post-approval continuation: `internal/api/approval/service.go:494` continues the approved function invocation; `:536` verifies the published page snapshot is still fresh — stale snapshots after an approval-policy change must not pass.
- Scope isolation and audit: `internal/api/approval/service.go:369` forces approval operations to carry game/env scope; `:325` writes every action to the audit chain.
- Pending model: `PendingFunction` at `internal/model/function.go:84` stores function registrations awaiting approval.

Not currently implemented: reusing this approval model for `capture.gates` risk-rule publishing (see the plan in the next section).

## Publishing and Validation

- The control-plane key is `capture.gates`: `internal/api/ops/capture_gates.go:30` defines the constant, `internal/api/ops/capture_gates.go:226` requires non-empty game/env, then runs JSON structure lint, semantic validation and Agent-fact prechecks.
- Publish prechecks: `internal/api/ops/capture_gates.go:189` dry-run does not persist; `internal/api/ops/capture_gates.go:209` calls the same `ValidateGates` the Agent uses.
- Capture-fact prechecks: `internal/api/ops/capture_gates.go:379` collects the table union and capability intersection from online capture Agents' labels under the same scope; `internal/api/ops/capture_gates.go:484` rejects versions whose reconcile or CEP burst tables fall outside capture scope or lack after images. When Agent facts are unknowable, `internal/api/ops/capture_gates.go:485` skips this layer rather than blocking the publish.
- Versions and rollback: `internal/api/ops/capture_gates.go:266` — rollback takes historical content and publishes it as a new version; `internal/api/ops/capture_gates.go:286` re-runs the current lint. The frontend at `web/src/pages/Risk/Strategies/index.tsx:253` warns that publish success is not effectiveness — effectiveness follows the Agents' self-reported reconciliation, roughly a 30-second polling cycle plus lag.
- Not currently implemented: an independent approval step for risk-policy publishing, two-person review, publish circuit breaking, and "auto-replay of the previous version after rolling back a bad one".

**Plan**: wire `capture.gates` publishing into the existing approval model, requiring approval snapshots for high-risk scope or large ruleset changes; add a first-cycle failure-rate circuit breaker after publishing so a bad revision does not keep waiting on Agent polling.

## Alerts and Disposition

### Hit reporting

- `agents/capture/runner/runner.go:573` packs the three gates' `Hit` into `CaptureRuleHitReport`; `internal/server/control_handler.go:492` normalizes `critical/warn` into fixed severities, and `internal/server/control_handler.go:536` persists idempotently via `AlertModel.UpsertByAlertID`.
- `internal/model/alert_model.go:51` dedupes on the unique `alert_id`; `internal/model/alert_model.go:157` merges platform-level alerts into the display of alerts in the current game/env scope.

### State transitions

- `internal/api/ops/dto.go:221` defines acknowledge, ignore and escalate; `internal/api/ops/helpers.go:589` maps target states and `internal/api/ops/helpers.go:595` validates the action and alert existence.
- `internal/api/ops/helpers.go:620` persists state, assignee, note and DB-clock time in one statement; `internal/model/alert_model.go:116` keeps audit fields updated in the same statement. Route at `internal/handler/routes.go:579`.
- Not currently implemented: an approval hook on escalate and downstream system actions. `internal/api/ops/helpers.go:586` already marks the "high-risk transition approval hook" as a later batch.

**Plan**: add a configurable approval chain to `escalate`; expand hit evidence with table name, row key, rule snapshot and related events; provide optional disposition adapters — read-only verification first, then bans, compensation or game-side actions behind approval.

## Explicitly Not Implemented

- Execution blocking: CDC rule hits do not roll back or block source-database changes; probing and execution are isolated — `internal/app/agent/probe_manager.go:25` states probes never trigger recovery actions.
- Risk actions such as account bans, freezes, rollbacks or compensation: today there is only manual alert state transition.
- Risk scoring and adaptive thresholds: no cross-rule composite score, historical baseline or ML model.
- Full session lineage: CDC event accounts are always empty, so the lineage gate is degraded.
- Auto-recovery / auto-escalation loops: hits do not create tasks, tickets or external system calls.

## Minimal Operating Advice

1. Before publishing, fill in `captureTables` and connector-capability labels for the same game/env so the dependency precheck is not in the skipped state.
2. Cover item tables and ledger tables with reconcile rules and set an understandable `WindowSec`; do not compress the window just to reduce noise.
3. Use a single-event threshold plus sliding window for CEP bursts; probing rules need at least 2 hits.
4. Use the lineage gate only for audit degradation counts — it is not a protective control today.
5. Handle hits with acknowledge/ignore/escalate plus notes; real disposition after escalation must go through a controlled ops procedure.
