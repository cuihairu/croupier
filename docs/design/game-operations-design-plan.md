---
title: Game Console Business Design Backlog and Acceptance Scenarios
---

# Game Console Business Design Backlog and Acceptance Scenarios

Date: 2026-10-17. Status: **recommendation archive / pending design review**; this round's mandate is to organize recommendations, record tasks, and write documents — it is not approval to implement everything below. Priorities are a suggested ordering; concrete business semantics must be settled in the corresponding design reviews.

Settled matters keep following the existing docs: the four agent roles, explicit gameId/env, external probe target categories, plugins and OTel integrated by default yet disableable, and the seven default menus; this page does not reopen those decisions and does not duplicate the existing agent-reliability backlog.

## 1. TODO and Document Division of Labor

`todo.md` records delivery actions, dependencies, and verifiable completion conditions; design documents hold business boundaries, data truth, state and failure semantics, scenarios, and open questions. One requirement may have both a TODO and a design document — after delivery, keep the test/commit evidence, and keep maintaining the design.

| Suggested topic                        | Suggested priority | TODO deliverable                                                             | This document's design                       | Existing anchor                                                  |
| -------------------------------------- | ------------------ | ---------------------------------------------------------------------------- | -------------------------------------------- | ---------------------------------------------------------------- |
| Game data authority and ownership      | P0                 | The data-ownership matrix and the legacy-path closure plan                   | §2                                           | PlayerModel, function execution, CDC/query replicas              |
| High-risk operation execution contract | P0                 | Business operation identity, credentials, and unknown-outcome handling       | §3                                           | Idempotency, tasks, approvals, audit, page snapshots             |
| Batch operations and compensation      | P0                 | Object snapshots, dry runs, execution, and failure recovery                  | §4                                           | Async tasks, scheduling, mail/compensation extensions            |
| Player identity and business targets   | P1                 | An extensible identity reference and mapping spec                            | §5                                           | Player queries, tickets, functions, server targets               |
| Game maintenance and releases          | P1                 | Release preconditions, business verification, and the rollback flow          | §6                                           | Releases, the supervisor, config versions, external probes       |
| Standard business extensions and pages | P1                 | The five business workflows' integration and page acceptance                 | [Default menu design](default-game-menus.md) | Existing default business page TODOs carried over, no duplicates |
| Game integration and compatibility     | P1                 | The minimum capability list, reference implementations, and acceptance tools | §7                                           | Six-language SDKs, OpenAPI, function/probe plugins               |
| Incident tracing and recovery          | P1                 | Cross-system correlation and the recovery drill plan                         | §8                                           | OTel, execution records, alerts, CDC, backups, incident registry |

The designs above reuse the current platform foundation — "adding design" is never a reason to rebuild permissions, approvals, audit, task scheduling, or a game economy system. For data-ownership details see [game data authority and ownership design](game-authority-ownership-design.md); for high-risk execution semantics see [the high-risk business operation execution contract design](high-risk-operation-contract-design.md); for batch execution see [batch operations and compensation design](batch-operations-compensation-design.md).

## 2. Game Data Authority and Ownership

**Observed facts**: `internal/model/player_model.go` has local Player reads/writes, balance mutations, and ban-state updates; from this alone we cannot tell whether the data corresponds to real game assets. Core/Extension mapping has proposed delivering business capability through extensions, with boundaries still to be pinned down.

**Recommended approach**: annotate, per domain — account, character, asset, ban, order, mail, activity — the authority system, read/write entries, sync method, cache freshness, and recovery responsibility; the recommendation is that the game keeps game business truth while Croupier keeps operation intent, credentials, audit, and necessary query replicas, with any exception explicitly declared.

**To be settled**: the formal positioning of the local Player/Balance, data mapping, offline-query labeling, concurrency version checks, post-read verification, and failure compensation; the existing model cannot simply be deleted, and a local write success must not count as game execution success.

**Acceptance scenario**: one asset query shows the source and capture time; one asset mutation links the game-returned credential; when the game is unreachable no fabricated success is returned, and replicas never override business truth.

## 3. High-Risk Operation Execution Contract

**Existing capabilities**: the platform already has idempotency, approvals, tasks, audit, and published snapshots; the [execution main path](../architecture/data-flow.md) has frozen the page-binding contract. What follows adds business execution semantics without redefining the page contract.

**To be settled**: the relationship between the business-level operationId and taskId/requestId/traceId, parameter and object digests, game-side durable dedup and expiry policy, original-request conflict checks, execution credential queries, retry admission, cancellation boundaries, and the distinction between success/failure/unknown outcome.

Approvals should be able to freeze the actual objects, parameters, budget, and validity of the operation, and re-check permissions, contract versions, and preconditions before execution; what the approval page binding snapshot owns versus what one approval of a concrete business operation owns must be written out explicitly.

Undo, rollback, and compensation are modeled separately; once game items are claimed/consumed no universal rollback can be promised, and an unknown outcome must never auto-flip to failure and re-grant rewards.

**Acceptance scenario**: the game granted rewards but the response was lost — retries/queries never grant twice; the same operationId with changed parameters is refused; changing the object scope or exceeding the budget after approval is refused; canceling a completed task never claims asset reversal.

Reference: [PlayFab's idempotency and retry contract](https://learn.microsoft.com/en-us/gaming/playfab/economy-monetization/economy-v2/tutorials/idempotent-transactions-and-retries); reference its division of responsibilities without copying specific retention periods.

## 4. Batch Operations and Compensation

**Recommended reuse**: batch operations as a business execution scheme on top of the existing async tasks, prioritizing mail, compensation, and batch bans, with pages provided through business extensions.

**To be settled**: selection and exclusion conditions, the executed-object snapshot, duplicate-object handling, dry-run counts and asset totals, approval summaries, per-batch quotas, concurrency/rate, failure-stop thresholds, per-object operationIds, task checkpoints, pause/resume, and final reconciliation.

If the target list changes over time, define when it freezes and whether it is recomputed after approval; the dry run must state whether it calls game verification, whether it produces side effects, and its validity — frontend counters never substitute for backend snapshots.

**Acceptance scenario**: run compensation against a fixed target set; after a failure, recovery executes only the unfinished items, with completed/failed/unknown queryable per item, and the totaled amounts matching the game credentials; changing the list requires re-approval.

## 5. Player Identity and Business Targets

**Constraints**: follow the [scope spec](../architecture/game-environment-scope.md) — gameId/env expresses the isolation boundary while servers/characters/channels express business targets; physical servers or channels are never stuffed into env.

**To be settled**: account vs character, one-account-many-characters, server/region/channel/platform identifiers, post-merge identity mapping, external account mapping, old-ID retention, and unambiguous addressing; fields are adopted per game capability — not every game is forced into the same world model.

A unified target reference must be correlatable across functions, tasks, approvals, tickets, audit, and probes; displayed targets must match the actually executed reference — nicknames are never stable identity.

**Acceptance scenario**: two servers with the same character ID, one account with multiple characters, and old IDs across a merge are all located accurately without crossing scopes.

## 6. Game Maintenance and Releases

**Existing capabilities**: version records, config, supervisor process updates, and the synthetics plan are reusable; a process starting successfully never automatically means the game business recovered.

Suggested flow:

```text
precheck → publish announcement → restrict new logins/matchmaking → wait for saves/safe exit
→ backup and update → protocol/business probes → small-scope opening → full restoration
```

**To be settled**: the source of real online counts, maintenance and drain conditions, timeouts and human intervention, server batches, the version/config/database compatibility matrix, opening gates, and failure handling; when irreversible data changes exist, automatic rollback is explicitly forbidden and the available recovery paths stated.

Reuse the existing task execution and approval mechanisms; no new generic workflow engine by default. Which steps the platform controls, which the game implements itself, and which external release systems carry must be divided item by item.

**Acceptance scenario**: new logins do not open early during maintenance; when the new version's process is healthy but the login probe fails, it does not open; when a change touches the database, the compatible choice among rollback, fix-forward, or restore can be made.

Reference: the game-side graceful-exit cooperation of [Agones Fleet Updates](https://agones.dev/site/docs/guides/fleet-updates/); this project makes no Agones integration promise because of it.

## 7. Game Integration and Compatibility

**To be settled**: the applicable scenarios, minimum required capabilities, optional capabilities, and unsupported states of SDK/OpenAPI/external function plugins; a reference game service and explicit integration steps — plugin metadata is never "the business is integrated".

The compatibility matrix covers platform, SDK, Agent, plugin contracts, function contracts, and published page versions; reuse the existing version floors and stale checks, and specify the upgrade order, mixed versions, protocol incompatibilities, fallbacks, and diagnostics.

Acceptance tools should cover registration, schemas, scope, authorization, business invocation and dedup, unknown-outcome queries, disconnect recovery, task cancellation, and probe failures; external plugins prefer the Go/Python reference implementations, with other languages added per actual integration.

**Acceptance scenario**: a developer completes one game's query and controlled write through the integration checklist; wrong scope/old contracts/unsupported capabilities give locatable errors, and the extra hops and latency of the SDK path vs the external adapter path are observable.

## 8. Incident Tracing and Recovery

**Suggested correlation dimensions**: gameId/env, business target, operationId, task/approval/trace, agent identity, binary fingerprint, plugin version, game version, and config version; each system correlates along its existing truth without duplicating full sensitive payloads.

**To be settled**: the query path from alert to release/operation/affected objects, the backup responsibilities for game assets vs the platform database, recovery order and time objectives, CDC positions/rule windows and pending-alert recovery, post-recovery task and credential replay rules, and the retention periods and permissions of logs/recordings/credentials.

With OTel off or builds trimmed, basic operation credentials and audit remain traceable; recovery drills use isolated environments and state which external systems' data the platform cannot restore — alert recovery is never counted as data-recovery completion.

**Acceptance scenario**: a wrong compensation can be traced from the alert to the operator, the approved scope, the actually executed objects, and the versions; after restoring the platform from backup and recovering the Agent, no rewards are re-granted, and unknown-outcome items and records needing manual review are identifiable.

## 9. Recommended Landing Order and Document Split

Complete §2/§3 first, validate with §4's single compensation chain, and in parallel land §5's minimal target identity; then wire the default menu business pages, maintenance releases, integration tooling, and incident recovery drills. The existing P0 scope/Agent/CDC reliability work continues — business design never substitutes for those fixes.

For now this page centrally holds the seven design briefs; when one enters an implementation batch, split its formal detailed design and contract documents along its upstream/downstream dependencies, avoiding creating multiple overlapping specs at once. A design review may only mark design-complete after stating the adopted approach, open questions, code impact, data migration, and positive/negative acceptance cases.
