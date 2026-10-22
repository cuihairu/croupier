---
title: Game Maintenance and Release Process
---

# Game Maintenance and Release Process

## Status and Scope

- Status: design pending review; this page defines the platform orchestration boundary and does not claim automatic operation of game logins, matchmaking, or databases.
- Reused capabilities: `GameRelease` version states, announcements bound to games, `ConfigVersion` versioned configuration, backup records, task scheduling, the Agent supervisor's update/suspend scheduling, and healthprobe.
- No generic workflow engine is added; one maintenance/release run is a single task governed by approval and audit, with steps orchestrated by the platform, concrete game actions executed via functions/plugins, and external release systems adapted through callbacks or tasks.
- `gameId/env` is a hard boundary; batches may only be split by server, canary group, or target references — an environment is never a batch.

## State Machine

```text
draft → approved → preparing → announced → draining → maintaining
      → updating → verifying → canary_open → full_open → completed
```

- `failed` can be entered after `preparing`; `paused` only means a human paused the remaining steps and does not change already-effective announcements or login restrictions.
- `canceled` is allowed only before irreversible changes execute.
- Every state transition records the operator, time, reason, snapshot fingerprint, and approval/task IDs; events are appended, never overwritten.
- The platform provides no "skip probing and force open". If a business must open in an emergency, use a separate high-risk approval and retain the failed-probe evidence afterwards.

## Division of Responsibilities

| Step                  | Platform responsibility                                            | Game/function responsibility                                         | External system responsibility                       |
| --------------------- | ------------------------------------------------------------------ | -------------------------------------------------------------------- | ---------------------------------------------------- |
| Precheck              | Validate scope, version matrix, backups, approvals, target batches | Report online counts, save progress, dependency status               | Return artifacts or pipeline results                 |
| Announcement          | Create/publish the announcement and bind it to the game            | None                                                                 | May sync to the support system                       |
| Drain                 | Distribute login-restriction intent, show queue/maintenance status | Reject new logins, stop matchmaking, finish existing matches         | Gateway/load balancer executes the access-plane side |
| Save-and-exit         | Track saves, session counts, timeouts, and manual intervention     | Save world state and exit safely                                     | May provide a host drain API                         |
| Backup & update       | Start tasks, verify artifact fingerprints and backup readability   | Produce application/database backups, run update scripts             | Carry storage or database backups                    |
| Business verification | Run read-only probes, check versions/config/metrics/audit          | Provide login, character, order, and activity verification functions | None                                                 |
| Batched reopen        | Control batches and the open gates                                 | Restore logins/matchmaking and report business results               | Gateway/load balancer restores traffic               |
| Failure handling      | Record evidence, pause/fail, and offer the allowed recovery paths  | Report side effects and rollbackability                              | Execute controlled recovery actions                  |

## Key Gates

1. **Precheck gate**: approval passed, version/config/database compatibility matrix complete, target batches non-empty, backup strategy explicit, and every executing function's contract available.
2. **Announcement gate**: maintenance window, impact scope, expected duration, and support copy published, and the announcement's game binding hits the current scope.
3. **Drain gate**: the new-login restriction is in effect; online count below threshold or max wait reached; saving transactions at zero; after timeout only a human may choose to keep waiting, force-stop, or cancel.
4. **Update gate**: backup succeeded and verifiable; irreversible database changes individually flagged with a recovery point; artifact SHA256 matches the release record; the managed process's desired state, suspended scheduling, and update fingerprint agree.
5. **Verification gate**: a running process is not an open condition; login/key business functions/probe results must pass. When the probe is unavailable but business functions prove healthy, only a manual high-risk decision may proceed.
6. **Canary gate**: first-batch size, observation duration, and error-rate/latency/login-success thresholds all met; a failure automatically pauses subsequent batches and never auto-rolls back unknown side effects.
7. **Completion gate**: full restoration, announcement updated, scheduling restored, config/version actual state matching the plan, and the task and audit chains closed.

## Online Counts and Draining

- The source of real online counts is declared by the game data source (in-memory metrics, login service, matchmaking service, or business events), with fields `serverId/currentSessions/savingSessions/draining/newLoginBlocked`.
- When no source is declared, the platform may only display "online status unverifiable" — TCP connection counts, process CPU, or agent heartbeats are never player counts.
- The drain policy must state the maximum wait, save confirmation, session-disconnect policy, and the human escalation path.
- Gateway/load-balancer traffic removal is an external action and must have an execution credential or callback result; changing platform config alone is not "drained".

## Compatibility and Irreversible Changes

A version plan declares at least the following matrix:

| Component          | Required fields                                     | Rollback policy                                                                         |
| ------------------ | --------------------------------------------------- | --------------------------------------------------------------------------------------- |
| Client             | version, platform, channel, minSDK                  | A legacy-version entry may be switched in; upgraded clients need a compatibility policy |
| Server             | version, config version, process fingerprint        | The supervisor rolls back to the old artifact                                           |
| Database           | migration version, prior backup, reversibility flag | Reversible migrations recover via backup/script; irreversible ones only roll forward    |
| Protocol           | schema version, compatibility window                | Old protocols stay usable within the mixed-version window                               |
| Dependent services | name, minimum version, network policy               | Automatic full-open is forbidden when dependencies cannot roll back                     |

Irreversible changes must be shown at the top of the approval summary: affected objects, recovery points, whether writes are lost, and expected recovery RTO/RPO. Without the declaration, entering `maintaining` is refused.

## Failure Handling

| Phase                       | Default action                                      | Allowed recovery paths                                                                        |
| --------------------------- | --------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Failure before announcement | Cancel, no business impact                          | A new plan after fixes                                                                        |
| Drain timeout               | Pause and escalate to a human                       | Keep waiting, safe force-stop, cancel                                                         |
| Backup failure              | Stop the update                                     | Retry the backup; never update without a backup                                               |
| Update failure              | Pause, collect process/function/database evidence   | Roll back artifacts/config if reversible; blind rollback of unknown side effects is forbidden |
| Probe failure               | Pause the opening                                   | Verify after fixes; may open after a manual high-risk decision, with the trail retained       |
| Canary failure              | Stop subsequent batches, keep opened batches' state | Roll back per the compatibility matrix, fix-and-roll-forward, or controlled rate limiting     |
| Database unknown            | Flag `unknown_side_effect`                          | Query/repair only; substitute replay of rewards is forbidden                                  |

Rollback is not one action: config rollback, process rollback, client-entry rollback, database restore, and traffic restore must be recorded separately, with the final state converged by the plan snapshot and audit.

## Acceptance Scenarios

1. `s1001` and `s1002` maintained in batches: the announcement affects only the target batches, and neither drain nor opening crosses the scope.
2. While the new-login restriction is not yet effective, the flow stays in `draining` and cannot enter backup & update.
3. The new-version process is running, but the login probe fails: the canary opening is refused and the page shows the failure evidence.
4. Any of config version, server artifact, or database migration failing the compatibility matrix: precheck fails and lists the gaps.
5. An irreversible migration fails: the platform does not auto-rollback the database; it only presents the backup recovery point and the roll-forward path.
6. The canary error rate exceeds threshold: subsequent batches auto-pause, and completed batches' state and audit remain traceable.
7. The external gateway's traffic-removal callback is lost: the drain gate does not pass merely on task-dispatch success — it requires credential/query confirmation of the result.
8. After a manual emergency open, the audit still links the original probe failure, the approver, and the compensation decision.

## Landing Batches

1. Add the maintenance plan snapshot and state-transition model, first covering the state machine, scope, batches, and compatibility-matrix validation.
2. Wire the task-step adapters for announcements, config versions, tasks, and supervisor operations.
3. Wire healthprobe and the game-function verification gates, completing the drain data-source contract.
4. Build failure-injection tests for backup/update/canary failures before enabling automatic advancement.
5. The console provides the maintenance timeline, evidence, batch progress, and pause/escalate-to-human actions.
