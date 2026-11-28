---
title: Managed-Process Config Update Review
---

# Managed-Process Config Update Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. This page is
the OPS-013 方案决策 deliverable. It resolves the five open questions for
remotely updating **managed-process configuration** (command/args/env/schedule
of `OpsConfig.ManagedProcesses`) into a concrete proposal, reusing the
`core/configsync` pull precedent and the §10.4 update machinery. No code is
changed by this page — implementation is gated on the owner's approval.

**Explicit non-claim (acceptance wording):** the existing probe-surface config
pull (`probe.policy` / `synthetics.targets` via `config_versions`, PROBE-004)
covers **probe-surface configuration only**. It does **not** distribute program
files or managed-process configuration. This proposal describes new work that
_reuses the same pull pattern_ — it must never be recorded as "already
supported by the existing probe config pull".

## 1. Current state (code facts)

- Managed-process config is **static yaml** loaded into `OpsConfig.ManagedProcesses`
  at agent startup. The schedule table is compiled once at construction
  (`compileProcessSchedules`; "config 构造后不变" — snapshots and the schedule
  loop share the compiled table). Live config update therefore requires a
  **recompile path that does not exist today** — this is real implementation
  cost, named up front.
- `ManagedProcessConfig.Validate()` exists (`internal/app/agent/ops_config.go`):
  closed-set checks for kind / schedule action / cron (`processcore.ParseCron`,
  shared with server-side task scheduling) / snapshotProfile. It is the
  format-validation seed for question 2.
- `core/configsync.Puller[T]` (PROBE-004 precedent): versioned pull loop with
  `WithOnError` (keep last-good on failure), `WithOnApply`, `PullOnce`,
  `LastVersion` skip-unchanged. Bad versions never clobber the running state.
- Payload size precedent: `OpsConfig.MaxPushFileBytes` (default **1 MiB**) —
  the push-file surface's existing size gate.
- Atomicity + rollback precedent: §10.4 `UpdateProcess` (hash pin → stop →
  backup → swap → start → stability window → rollback; event closed set;
  execlog audit `source=process_update`).
- Authorization precedent: §10.7 trio (double switch + allowlist fail-closed +
  execlog audit on every attempt).

## 2. The five questions (proposal)

### Q1 — Path sandbox

Config content travels as **versioned config payloads** (server-side
`config_versions`, new key e.g. `ops.managed_processes`), never as arbitrary
file paths — the agent writes what it received into an agent-owned config
directory, no server-specified paths. Constraints:

- The materialized file lives only in the agent's managed config directory
  (same trust root as `SupervisorLog`/`SnapshotDir` defaults: relative to the
  agent working directory). Absolute-path or `..` escape in any process
  working-dir field stays caller-owned and is **not** part of this surface.
- If a future surface needs to reference staged binaries, that is D1's
  staged-relay territory (agent-self-upgrade-review §2), not config update.

### Q2 — Format validation

Apply gate = `ManagedProcessConfig.Validate()` on the **candidate config as a
whole** before anything is touched, plus structural checks the pull payload
must satisfy (process name closure, unknown-field rejection). A candidate that
fails validation is dropped with an error record; **the last-good config keeps
running** (configsync precedent). Validation failure never restarts a process.

### Q3 — Atomicity, restart and rollback

1. Diff candidate vs running config → the **affected set** = processes whose
   effective config changed (added / removed / changed command, args, env,
   schedule, autorestart).
2. For each affected process: stop (explicit-action funnel, §10.6 semantics —
   desired-state and supervisor events flow as usual) → config swap (atomic
   rename within the config directory) → start.
3. Stability window (§10.4 default 5s): an exit within the window rolls the
   affected process back to the last-good config and restarts it.
4. Unaffected processes keep running untouched (no restart storm).
5. Events: the supervisor event closed set gains `config_applied` /
   `config_rolled_back` (message: old→new config version). Execlog audit
   `source=process_config_update` is the persistent-query main entry.
6. Desired-state semantics preserved: a manually stopped process (§10.3) stays
   stopped after a config apply — the new config is materialized but not
   started (explicit run intent required, same as §10.4).

### Q4 — Size and bandwidth

- Pull-based (agent dials on the existing configsync cadence): no push storm,
  bandwidth bounded by one versioned payload per agent per change.
- Payload cap: reuse the `MaxPushFileBytes` gate value (default 1 MiB) as the
  `config_versions` payload ceiling for this key — managed-process config is
  expected to be far smaller; the cap exists to make abuse impossible rather
  than to tune a budget.
- `LastVersion` skip-unchanged (configsync) prevents no-op apply loops.

### Q5 — Approval gate

- New independent ops gate on the agent (e.g. `allowProcessConfigUpdate`),
  default false, in the §10.7 double-switch family. Config update ≠ permission
  to run arbitrary commands (§10.7 hard boundary holds).
- Server side: the issuing endpoint lands in the existing ops REST family with
  its permission surface; **high-risk two-person approval is available for
  reuse** (existing approval workflow family) — whether config issuance
  requires a second approver is an owner sub-decision (D5 below).
- Every attempt (success / validation failure / agent unreachable) writes
  execlog audit — the §10.7 trio applies unchanged.

## 3. Acceptance mapping (design → verification scenarios)

| #   | Scenario                                             | Expected                                                                                     |
| --- | ---------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| A1  | Candidate config fails `Validate()`                  | Last-good config keeps running; error record; no restart of any process                      |
| A2  | Apply succeeds on N-of-M changed processes           | Only the affected set restarts; unaffected processes' uptime uninterrupted                   |
| A3  | New process config exits within the stability window | Rolls back to last-good config, restarts old config; `config_rolled_back` event + execlog    |
| A4  | Manually stopped process receives config change      | Config materialized, process stays stopped (desired state preserved)                         |
| A5  | Payload over the size cap                            | Rejected before apply; last-good retained; audit record                                      |
| A6  | Agent gate off (`allowProcessConfigUpdate=false`)    | Server-side issue fails; execlog records the denial                                          |
| A7  | Schedule change only                                 | Recompiled schedule table takes effect without process restart; `next_schedule_at` refreshes |

## 4. Decisions needed from the owner

- [ ] **D1** payload channel — approve `config_versions` pull (key `ops.managed_processes`) as the distribution channel
- [ ] **D2** validation strictness — approve full-`Validate()`-before-apply with last-good retention (unknown-field rejection included)
- [ ] **D3** atomicity shape — approve affected-set restart + stability-window rollback (no whole-fleet restart)
- [ ] **D4** size cap — approve `MaxPushFileBytes`-aligned ceiling (default 1 MiB) for the payload
- [ ] **D5** approval strength — is config issuance single-operator (gate + audit only) or two-person (reuse the approval workflow)?
- [ ] Acceptance mapping §3 (A1–A7) — confirm as the implementation's verification bar
