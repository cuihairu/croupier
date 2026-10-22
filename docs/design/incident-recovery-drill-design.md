---
title: Incident Correlation and Recovery Drills
---

# Incident Correlation and Recovery Drills

## Status and Scope

- Status: design pending review; `incidents` is the incident truth, `alerts/execution_logs/audit_records` are evidence sources, and backup/restore is the drill target.
- Goal: alerts, operations, affected objects, versions, and recovery actions can query each other; platform-database recovery and game-data recovery are assigned separately — alert recovery is not data recovery.
- Current state: `Incident` already has `GameID/Env/ExecLogIDs/RefType/RefID/Details`; `ExecutionLog` has scope, function, actor, status, and `TraceID`; `AuditRecord` has actor/resource/context/hash chain; `GameRelease`, `ConfigVersion`, supervisor events, and plugin/function versions can provide change snapshots; backup records already have checksum/status/location; the CDC journal supports ordered replay and requires transaction boundaries.

## Correlation Dimensions and Truth

| Dimension       | Truth/source                                                           | Correlation rule                                                          |
| --------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| scope           | Incident/Alert/ExecutionLog/approval `gameId/env`                      | Non-empty values must match; platform-level scopes are marked separately. |
| Business target | The unified target reference                                           | Stable IDs and `identityFingerprint` stored.                              |
| operationId     | High-risk contract, approvals, tasks, execution logs, game credentials | Used as the cross-system query key; never regenerated.                    |
| Task/approval   | `TaskID`, approval ID                                                  | Request/approval scope snapshots retained.                                |
| trace           | `ExecutionLog.TraceID`, AuditContext, OTel                             | The persistent trace string survives even with OTel off.                  |
| Agent           | AgentID, `agentVersion`, binary fingerprint                            | Read from registration, heartbeat, supervisor events/snapshots.           |
| Plugin/function | Manifest/contract version and digest                                   | Bound to the snapshot taken at execution.                                 |
| Game/config     | GameRelease, ConfigVersion, publish status and fingerprints            | Aligned to the incident time window.                                      |
| CDC             | source ID, position, journal event ID, transaction ID, rule ID         | References and hashes only; sensitive row images are never copied.        |

Incident details store "evidence references + digests"; full request/response payloads, credentials, or sensitive player payloads are never copied into the Incident. Reads expand per source under permissions.

## Incident Entry Points and Queries

1. Automatic conversion: alerts, probe windows, supervisor events, cicd builds, CDC rule hits, and controlled-operation anomalies can all become Incidents; the conversion idempotency key is composed of source type, source ID, and event fingerprint.
2. Correlated queries: expand from the Incident across approvals, operations, tasks, agent/plugin/game/config versions, alerts, and CDC windows; every reference records its fetch time, so later truth changes cannot drift the interpretation.
3. Attribution: `ResponsibleType=change` must bind a release/config/process-update/plugin-change ID; time ordering alone never attributes.
4. Recovery status: an Incident's resolved only means human closure; `data_recovered`, `game_side_effect_closed`, and `cdc_replay_closed` are separate booleans/states — missing means `unknown`.
5. OTel trimming: no spans are generated, but `traceId`, operationId, task/approval IDs, credentials, and the audit chain are still written; the console shows "no full trace view; query by persistent IDs".

## Backup Responsibility Matrix

| Object              | Platform responsibility                                                                    | Game/external responsibility                                                      |
| ------------------- | ------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| Meta database       | Built-in logical backups, checksums, recovery drills                                       | External PITR/snapshots (if chosen) are carried outside.                          |
| Game database       | The platform does not own game business truth by default; it may run dumps/record metadata | Backup policy, point-in-time restore, compliance retention, and data consistency. |
| Platform config     | Config versions, last-good, artifact checksums                                             | External IaC/release systems back up their own state.                             |
| Agent runtime state | supervisor/audit/config last-good                                                          | Local logs and crash artifacts kept per retention policy.                         |
| CDC journal         | Positions, transaction boundaries, event hashes, capacity and corruption detection         | Source database backups, schema history, and logical snapshots.                   |
| Alert spool         | Durable pending queue, retry counts, order and dedup keys                                  | External alerting systems own their receiver state.                               |

The platform must not claim it can restore game-database business truth it does not own; data sources without integrated backups must show `not_covered` in the plan.

## Recovery Order

```text
Isolated environment ready → platform meta/config restore → version compatibility check → agent/plugin re-registration
→ data recovery (game DB/CDC) → consistency verification → task convergence → operations resume
```

1. The platform restores first to a snapshot compatible with the binary/migrations; old/new binary compatibility follows the versioned migration gates.
2. After agents recover, they verify with the old config last-good and fingerprints; config or artifact mismatch with the restored version enters human review.
3. The game database restores per its RPO/RTO; the platform records only the recovery point, verification result, and responsible party.
4. CDC recovers from the journal or source positions, prioritizing complete transaction boundaries and snapshots earlier than the start; missing transaction metadata must refuse recovery.
5. The alert spool replays in original order after recovery; already-acknowledged/recovered alerts do not re-escalate.
6. Consistency checks run before operations resume: audit chains, task terminal states, approvals, game credentials, version fingerprints, and the recovery-point list.

## Post-Recovery Tasks and Credentials

- Status `succeeded` with a game credential: no replay — verify business truth only.
- Status `unknown`: query the game outcome only by the original `operationId`; a found side effect converges the terminal state, and a missing one forbids substitute compensation.
- Status `retryable_failed`: retry after recovery with the original snapshot and window; changed contracts, targets, or budgets require re-approval.
- `canceled/failed`: old tasks are never resurrected; compensation generates a new operationId, a new approval, and a new target snapshot.
- Tasks created after the platform database's recovery point are marked `orphaned_execution` for human review even when run logs exist — never auto-trusted as success.

## Drill Design

| Drill                       | Input                                                 | Success criteria                                                                                                          |
| --------------------------- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Audit chain closure         | Wrong compensation + alert + approval + execution log | The operator, approved scope, target fingerprint, versions, and game credentials are reconstructible.                     |
| Platform database restore   | Meta dump + same-version binary                       | healthz, login, function catalog, approval/audit chains queryable; post-recovery-point records flagged as orphans.        |
| Agent recovery              | Config last-good + fingerprint + crash artifacts      | Registration succeeds, supervisor events traceable, config version silently undrifted.                                    |
| CDC recovery                | Journal + consistent snapshot                         | Transactions complete and ordered, target idempotent; missing-marker scenarios are refused.                               |
| Alert recovery              | Durable spool + old alert states                      | No duplicate escalations, order preserved, recovered alerts never reopen.                                                 |
| Unknown-outcome convergence | Lost response + game credential query                 | Only the original operationId is queried, no re-grants, unknown is never fabricated into success.                         |
| Irreversible change failure | Database migration + backup point                     | No automatic rollback of irreversible migrations; recovery point, roll-forward path, and human decision records provided. |

Drills run in isolated environments, independent scopes, and the same version or a declared mixed matrix; production drills may only do read-only verification.

## Retention and Permissions

| Data                       | Suggested hot retention        | Long-term anchors                                 |
| -------------------------- | ------------------------------ | ------------------------------------------------- |
| ExecutionLog               | 30–90 days                     | operationId, credentials, audit hashes, digests.  |
| Audit records              | Per compliance period          | hash chain + signature.                           |
| CDC journal                | Per recovery window            | source position, transaction ID, event hashes.    |
| Alerts/notifications       | 90 days                        | dedup key, state transitions, escalation records. |
| Crash artifacts/recordings | 14–30 days                     | fingerprint, Incident references, access audit.   |
| Drill reports              | Permanent or compliance period | versions, snapshot IDs, RTO/RPO, and gaps.        |

Exact days are covered by deployment compliance policy; this page's defaults are upper bounds and minimum query anchors. Sensitive payloads are masked by default; reading raw values requires dedicated permission and second-pass audit.

## Acceptance Scenarios

1. A wrong-compensation alert resolves within 5 hops to the operator, approved scope, actual `role/server`, operationId, game credential, game/config versions, and the agent fingerprint.
2. After restoring the platform from a meta backup, approval/audit chains before the recovery point are complete; orphan tasks after it are clearly visible and never replayed.
3. After the game database restores to a backup point, `unknown` operations query by the original operationId: a credited game converges to success; an unfindable one stays unknown — never compensated.
4. A CDC journal missing transaction-end markers refuses recovery; a complete journal + consistent snapshot replays in order, and repeated replays never double-apply.
5. An agent recovering with a stale fingerprint enters human review; with a matching fingerprint it auto-registers and keeps the event chain.
6. Spool replay produces no duplicate critical escalations, and resolved source alerts are not reopened by old data.
7. With OTel off, audit reports still reconstruct the key chain by operationId + traceId + approval/task/credentials.
8. An irreversible migration failure yields the backup point, roll-forward path, affected objects, and human decision records — never an automatic rollback.

## Landing Batches

1. Add the unified recovery status, version fingerprint references, and orphan-task flags to Incident; model and query tests first.
2. Add an evidence-aggregation API to incident details, returning approval/task/execution/version/agent/CDC references per source under permissions.
3. Wire CDC recovery coordination into Incidents, recording recovery plans, audit IDs, rejection reasons, and closure states.
4. The recovery drill runner supports read-only verification of platform meta and agent config first, then isolated databases.
5. The console provides the recovery timeline, missing evidence, unknown outcomes, and human decision entry points.
