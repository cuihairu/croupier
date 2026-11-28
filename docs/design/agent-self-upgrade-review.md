---
title: Agent Self-Upgrade Review
---

# Agent Self-Upgrade Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. This page is the
OPS-012 方案决策 deliverable. It consolidates the v0.1 draft in
[agent-supervisor-design §10.5](agent-supervisor-design.md) into a decision
sheet with recommendations, and drafts the acceptance plan for the two required
outcomes: **a failed self-upgrade must not brick the agent**, and **rollback
must have an out-of-band path**. No code is changed by this page —
implementation is gated on the owner's approval of D1–D4 below.

## 1. What already exists (build surface, code facts)

The self-upgrade question is not greenfield. Three independent pieces landed
2026-10-15 and are decision-independent (they serve triage regardless of
whether self-upgrade is ever built):

| Piece                                                                                                                                                                                                                                            | Where                                                                    | Role for self-upgrade                                    |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------ | -------------------------------------------------------- |
| Run-instance fingerprint uplink (`SystemInfo.agent_binary_sha256`, proto appended, wire-compatible)                                                                                                                                              | agent → `GET /ops/agents/:agentId/system-info`                           | Know _what is running_ before and after an upgrade       |
| Last-known fingerprint anchor (registration label `agentBinarySha256`, platform-key exempt; DB `LastKnownIdentity` read Unscoped)                                                                                                                | `agent_sessions.labels`; anchor survives session expiry + server restart | Triage when the agent is down (lost contact mid-upgrade) |
| Managed-process update machinery (`UpdateProcess`: sha256 fail-closed pin → stop → `.croupier-bak` backup → atomic swap → start → 5s stability window → rollback; `update_applied`/`update_rolled_back` events; execlog `source=process_update`) | `internal/app/agent/ops_update.go`                                       | The pattern self-upgrade must adapt — not reuse as-is    |
| Authorization trio (§10.7: double switch + allowlist fail-closed + execlog audit on every attempt)                                                                                                                                               | `allowUpdate`/`allowExec` family                                         | The gate family self-upgrade joins                       |

Why `UpdateProcess` cannot be reused as-is: its rollback assumes **some role
still observes the new binary** after the swap (the agent's own monitor
closes `waitDone` and rolls back). During a _self_-upgrade that observer is the
process being replaced — there is nothing above the agent. A failed
self-upgrade = loss of contact; the invariant is therefore:

> **The worst case of a failed self-upgrade is the agent restarting on the old
> version — never a bricked, unrelaunchable agent.**

## 2. The four decisions (recommendations from §10.5, restated for sign-off)

### D1 — Upgrade source

| Option | Shape                                                                                                 | Consequence                                                     |
| ------ | ----------------------------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| (a)    | Server-side staged relay (server stages the package, distributes; same shape as §10.4's `stagedPath`) | Reuses the existing wire + audit surface; no second trust chain |
| (b)    | Agent fetches an external URL directly                                                                | Needs its own TLS + signature trust chain                       |
| (c)    | Config-version pinning (server issues `targetVersion`, agent fetches itself)                          | Distributed trust decisions; harder to audit                    |

**Recommendation: (a).** The upgrade surface is naturally covered by the
authorization and audit surfaces (§10.7 trio); introducing a second trust chain
for the highest-risk action is the wrong trade.

### D2 — Trigger model

| Option | Shape                                                                        | Consequence                                                             |
| ------ | ---------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| (a)    | Manual API, independent key alongside `allowUpdate` (e.g. `allowSelfUpdate`) | Explicit, auditable, no silent upgrades                                 |
| (b)    | ConfigVersion hot-update carries an upgrade field                            | Upgrades happen silently with config — unacceptable for this risk class |
| (c)    | §10.6 cron scheduling                                                        | Deferred maintenance windows, but silent by default                     |

**Recommendation: (a) to start.** Self-upgrade is the highest-risk action in
the system; it must never fire silently. Batch upgrades layer batch
orchestration _on top of_ (a) (operator-driven), not as a separate trigger.

### D3 — Restart handover (the core difficulty)

The agent must replace its own running binary while holding the control
connection. Options:

| Option | Shape                                                                                            | Consequence                                                                                                                            |
| ------ | ------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| (a)    | External supervisor relaunches (systemd/k8s restarts on exit)                                    | Depends on the deployment surface guaranteeing restart-on-exit                                                                         |
| (b)    | fork+exec handover (fork a child that waits for the swap, then execs the new path; parent exits) | Self-consistent even with no supervisor; with a supervisor, the parent exit degrades into exit-restart — one code path for both shapes |
| (c)    | Dual-process alternation (shadow process)                                                        | Highest complexity; two live agents risk split-brain on managed processes                                                              |

**Recommendation: (b).** Self-upgrades stay self-consistent on bare hosts
without a supervisor; under systemd/k8s the parent exit triggers the existing
restart policy and the new binary comes up either way. On reconnection the
existing registration path refreshes the fingerprint label + anchor (§1), so
"which version is running" is queryable across the swap.

### D4 — Failure posture and rollback

A failed self-upgrade = loss of contact; rollback can only be out-of-band.
Design constraints (carried from §10.5):

1. The old binary is **renamed as a backup, never deleted** (same
   last-known-good semantics as §10.4's `.croupier-bak`).
2. The new version must **survive a stability window** before success is
   declared — longer than the managed-process 5s default is recommended for
   self-upgrade (the agent has no external observer; the window is the only
   self-declared health gate before the old binary becomes the rollback
   target). Concrete value: **10s, config-gated**.
3. A death within the window = the supervisor restarts per its existing policy
   (the backup is there → the old version returns), or a human intervenes
   out-of-band.
4. Fingerprint records follow §10.4's three landing surfaces: event closed set
   (new members `self_upgrade_applied` / `self_upgrade_rolled_back`) + execlog
   audit `source=agent_self_upgrade` + report surface.

## 3. Acceptance plan (what implementation will be verified against)

### A1 — Failed self-upgrade does not brick (required outcome)

| Scenario                                              | Expected                                                                                                                                                   |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| New binary exits within the stability window          | Backup `.croupier-bak` still present → supervisor/manual relaunch runs the **old** version; agent reconnects and re-registers with the **old** fingerprint |
| New binary fails after the window (run-state failure) | Existing recovery semantics apply; `.croupier-bak` remains for manual rollback                                                                             |
| Staged content hash mismatch                          | Reject before touching the running binary (fail-closed pin, §10.4 first gate)                                                                              |
| Install fails mid-swap (EXDEV, permission, disk)      | Old binary untouched; explicit failure record in execlog; agent keeps running                                                                              |
| First install (no old version to roll back to)        | No rollback target: explicit no-event boundary (§10.4 honest boundary ④ applies); execlog is the record                                                    |

### A2 — Out-of-band rollback is real (required outcome)

- The backup path is deterministic (`<target>.croupier-bak`, same directory),
  so a human (or a runbook script) can restore without platform tooling.
- The DB last-known fingerprint anchor (§1) lets an operator confirm _which
  version was last known running_ even while the agent is down.
- Rollback via the backup is an explicitly documented runbook step, not an
  undocumented internal detail.

### A3 — Observability across the swap

- Every attempt (success / business failure / agent unreachable) leaves an
  execlog record (`source=agent_self_upgrade`), aligned by
  (agentID, time, old→new fingerprint) with the event stream.
- Server-side metrics-stale alerting distinguishes "agent upgrading" from
  "agent lost contact" (§10.5 known boundary ③) — at minimum via the execlog
  in-flight record; a dedicated "upgrading" state is out of scope for v1.

## 4. Known boundaries (carried from §10.5, unchanged)

- Windows has no `fork` — D3(b) needs its own plan there (subprocess spawn +
  wait).
- Multi-agent batch upgrade ordering (standbys before primaries, in batches)
  is orchestration-layer work, not part of the single-agent primitive.
- During self-upgrade all of the agent's managed programs share the blind
  window (recovery loop / probes stop with the agent).

## 5. Decisions needed from the owner

- [ ] **D1** upgrade source — approve option (a) staged relay (or pick b/c)
- [ ] **D2** trigger model — approve option (a) manual API with independent key (or pick b/c)
- [ ] **D3** restart handover — approve option (b) fork+exec (or pick a/c)
- [ ] **D4** failure posture — approve rename-not-delete backup + 10s stability window + event closed-set extension
- [ ] Acceptance plan §3 (A1–A3) — confirm as the implementation's verification bar
