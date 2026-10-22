---
title: Game Data Authority and Ownership Design
---

# Game Data Authority and Ownership Design

Date: 2026-10-17. Status: **pending design review**. This page completes the data ownership, read/write entries, replica freshness, and legacy `Player/Balance` closure plan of §2 of the [Game console business design backlog](game-operations-design-plan.md). Until the review passes, this page defines boundaries only — it authorizes no refactoring of local tables, no new game economy systems, and no data migration.

## 1. Design Principles

- The game service is the authority for player runtime state, characters, assets, bans, order credits, mail, and activity effectiveness. Croupier is the operations surface and never becomes the source of game business truth.
- Croupier's authoritative data is limited to the platform control plane: function contracts, function installs and version floors, pages and menus, permissions, approvals, task execution records, execution logs, audit, published snapshots, probe results, and config versions.
- Every game business read model must declare `sourceSystem`, the read entry, capture time, data freshness, and its failure display. When undeclared or stale, it displays unavailable — empty values, old replicas, or platform defaults are never rendered as truth.
- Every game business write must execute through a game function/plugin. A successful Croupier local transaction only means the operation intent or control-plane record succeeded; it is never merged into "game execution succeeded".
- All reads, writes, and execution records are scoped to `(gameId, env)`. Game function contracts must declare the target data domain and business target; the platform never guesses servers, characters, or accounts.

## 2. Authority Systems and the Read/Write Matrix

The table below is the minimal field requirement of the integration contract. `Authority` answers "which side wins on conflict"; `Croupier replica` exists only after explicit integration.

| Business domain                  | Authority system and write entry                                                                                                                                                | Croupier read entry                                                                                                               | Croupier replica and freshness                                                                                                                     | Recovery responsibility                                                                                                                                 |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Player accounts and login state  | The game account service. Creation, profiles, password changes, and login-state maintenance may only call game functions; the account service may call back into Croupier audit | Query functions return account identifiers, character summaries, status, profile version, and capture time                        | No persistence of passwords, tokens, or full account profiles; only short-TTL query results or audit summaries                                     | The game account service owns account recovery; Croupier restores only approvals, execution records, and audit                                          |
| Player characters and progress   | The game character service. Character creation, renames, levels, VIP, and server migration are written by game functions                                                        | Query functions return `gameId/env/zoneId/playerRef`, character status, version, and capture time                                 | No generic character replica. The workbench may store execution object snapshots for audit, never for live display overriding truth                | The game side restores characters from backups; Croupier restores object snapshots and execution history                                                |
| Assets and currency balances     | The game asset ledger system. Grants, deductions, freezes, and compensations all go through functions carrying game-side credentials                                            | Query functions return balances, resource items, ledger version, capture time, and query credentials                              | Platform tables are never the balance truth; compensation/mail object snapshots may record intended changes, never credited amounts                | The game ledger owns reconciliation, compensation, and rollback; Croupier keeps intent, approvals, and execution credentials                            |
| Bans and dispositions            | The game risk-control/account service. Bans, suspensions, and unbans are written via game functions, returning disposition credentials                                          | Query functions return disposition status, reason, duration, effective time, and credential                                       | Approval and execution logs keep object snapshots, reasons, and results; platform fields are never long-term ban truth                             | The game side restores disposition state; Croupier restores history for traceability                                                                    |
| Orders and credits               | The payment channel holds channel orders; the game payment/ledger service holds credit state. The compensation function is the only controlled write entry                      | Channel orders, game orders, and credit results queried per source; every record carries source, transaction id, and capture time | No order ledger rebuilt on the platform; the review page may stage query snapshots for cross-source comparison                                     | Channel and game each restore their own ledgers; Croupier restores review records, approvals, tasks, and execution logs                                 |
| Mail and compensation delivery   | The game mail/compensation service. Creation, withdrawal, claiming, and failure redelivery go through game functions; claim results return via game events or queries           | Query functions return delivery status, claim status, targets, batches, game credentials, and times                               | The platform may store tasks, per-object results, failure lists, and last-query times; replicas never drive credit judgments                       | The game service restores mail; when Croupier restores task states and replayable approval contexts it must re-run the game-side prechecks              |
| Activities and ops configuration | The game activity runtime. Config parsing, scheduled activation, canary, rollback, and player-visible state are executed by the game                                            | Query functions return activity config versions, publish batches, effective scope, game-side state, and capture time              | PageSpec stores page presentation only; platform tasks/approvals store publish intent and object scope, never replacing the activity runtime state | The game side's backups and versioned configs own activity recovery; Croupier restores publish records and re-validates version and scope before replay |

Read entries go uniformly through registered, authorized game functions or external function plugins; no generic direct-to-game-database query channel is added. Query function contracts must include the data domain, target, return fields, sensitive-field policy, cache freshness, and error codes; when a query is impossible, pages display the source and time rather than fabricating an empty object.

## 3. Replicas, Snapshots, and Cache Freshness

| Type                      | Purpose                                                             | Lifecycle and expiry                                                                                                        | Conflict rule                                                                                              |
| ------------------------- | ------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Short-TTL query results   | Workbench, order review, and other displays                         | Not persisted by default; when persistence is needed, write `fetchedAt` and `expiresAt`, and on read mark stale or re-query | Display cache only; never participates in business decisions                                               |
| Approval object snapshots | Proving the person/character/assets/budget seen at approval time    | Immutable after approval; the original snapshot is kept at execution and the actual execution response recorded             | Execution is blocked and re-approval required when the actual object or version changed                    |
| Per-object task results   | Progress and failure recovery of batch compensation, mail, and bans | Kept per task, including game credentials, status, failure codes, and times                                                 | The game query/receipt is authoritative; duplicate platform records must be explicable by credential dedup |
| Execution logs and audit  | Tracing intent, request/response, operator, and trace               | Cleaned per Croupier retention; sensitive payloads masked                                                                   | Never game business truth; on conflict, game-side queries/credentials win                                  |
| Probe and health results  | Network and service availability judgments                          | Kept per probe window                                                                                                       | Never used to judge asset or order business outcomes                                                       |

Every replica write must be able to answer: which `sourceSystem`, which function, which `operationId`/credential, which `(gameId, env, business target)`, when captured, when it expires, and who authorized it. A persisted replica missing any of these does not satisfy this design.

## 4. Positioning the Legacy Local `Player/Balance`

### 4.1 Current Code Facts

- `internal/model/player.go` defines `Player` with `Username/Nickname/Email/Phone/GameID/Status/Balance/Level/VIP/Password`; `Status` directly expresses active/banned/suspended and `Balance` represents game currency.
- `internal/model/player_model.go` provides generic reads/writes like `Create/FindOne/FindByUsername/Update/Delete/List`; `UpdateBalance` accumulates balance in a local transaction and refuses negative balances; `BanPlayer/SuspendPlayer/ActivatePlayer` update local state directly.
- `internal/handler/routes.go` registers the player routes, including create, update, delete, and `POST /:id/balance`.
- `internal/model/migration.go` puts `Player` into every `(gameId, env)` game database model set; there are no server, region, channel, platform, character-mapping, or external-player-ID fields.
- The execution log `ExecutionLog` already records `gameId/env/functionId/actor/status/traceId/requestPayload/responseBody`, but has no business `operationId` or game execution credential fields. This design does not modify the log wire; item 3's execution-contract document must define the extension or mapping plan.

Therefore the local table actually carries the platform's built-in player profile and local state/balance model; nothing in the repo proves it automatically maps to any real game asset. The old table cannot simply be deleted, and `UpdateBalance` cannot continue to be treated as granting assets.

### 4.2 Formal Positioning

1. During the compatibility period, the local `players` table is classified as a **Croupier local demo/auxiliary account model** — not the truth of game players, characters, assets, or bans.
2. `Password` may only serve platform demo login or local extension credential checks; real game login credentials must never migrate in and must never appear in logs or execution payloads.
3. `Balance` is a local model field and does not represent game currency. Any real compensation, grant, or deduction must move to game functions; continuing to change real assets through `POST /:id/balance` is forbidden.
4. `Status` is a local account state; real bans, suspensions, and unbans must go through game functions with disposition credentials recorded. If a game side wants local fields to speed up display, an explicit sync task must be configured, with replica freshness and conflict direction declared.
5. The player workbench may keep the local table as a history/demo data source, but the UI must distinguish "Croupier local model" from "authoritative game query"; without integration, no real asset operations are offered.

### 4.3 Closure Plan

| Phase                      | Goal                                                                                                                                                        | Boundary and acceptance                                                                                                                        |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| A. Freeze new dependencies | New real-game pages and functions no longer read/write `PlayerModel.UpdateBalance/BanPlayer/SuspendPlayer/ActivatePlayer`                                   | Checked via code guards and the integration checklist after review; old page behavior unchanged                                                |
| B. Label sources           | Player list/detail distinguish "local model" and "game function" sources; local asset writes stay demo-mode-only or canary-disabled                         | Production scopes by default never render local balances as game assets; error pages explain when game functions are unavailable               |
| C. Contract replacement    | Establish account/character/asset/ban query/write function contracts; writes use item 3's operationId and game credentials                                  | The same operationId is idempotent on the game side; a lost response never grants twice; offline failures can query "outcome unknown"          |
| D. Data mapping            | Configure external player/character identity mapping per game; local IDs are Croupier references only, never game primary keys                              | Same account, multiple characters, multiple servers addressable accurately; merged-server old-ID mapping comes from the game authority service |
| E. History closure         | Legacy local balance/state migration generates a reconciliation report per game; display and operation entries switch only after administrator confirmation | Data with no corresponding game record stays locally archived; auto-converting local balances into game assets is forbidden                    |

A one-shot deletion of `players` or `UpdateBalance` is not recommended: demos, tests, and history pages reference it, and direct deletion would break data and pages. The closure targets disabling real write paths, explicit sources, and contract replacement.

## 5. Offline, Failure, and Recovery

1. **Query failure**: return source-level errors and the last-known-good time; never return default values like zero balances, no bans, or empty activities. Cross-source queries label success/failure per source.
2. **Pre-write failure**: when a game function is unreachable, credentials are missing, the contract version mismatches, or the approval expired, submission is refused — never write "granted".
3. **Unknown outcome**: timeouts, disconnects, or lost responses must enter `unknown`, converging only via query functions or game receipts. Platform retries must carry the same business `operationId`; game-side credentials are the first basis for dedup.
4. **Partial success**: batch/multi-step operations record status per object; a batch failure must not be written as total failure, nor a platform record success as assets credited.
5. **Compensation boundary**: generic rollback is refused once game items are claimed or consumed; only the game side's compensation/reversal functions may execute, with re-approval and the original credential retained.
6. **Recovery order**: restore the game authority service first, then Croupier; platform replicas start in a stale state after startup and may rejoin operation display only after re-query confirmation. Replicas never overwrite game data in reverse.

## 6. Acceptance Scenarios

1. **Source visibility**: a player asset query returns the asset value, `sourceSystem`, game function/data source, capture time, and TTL; an expired cache shows as expired rather than masquerading as live.
2. **Controlled write**: initiating compensation generates a business `operationId`, the approval snapshot includes objects, parameters, and budget; after the game function returns a credential, execution records, audit, and task results all chain through the same credential.
3. **Lost response**: after an invocation times out, the task state is outcome-unknown; the operator's retry produces no second operationId, and once the game query confirms the first grant landed, it converges to success without double-granting.
4. **Game offline**: queries show the failed source, writes refuse submission; submitted-but-unknown operations are never marked successful — after recovery they are settled by game query or receipt.
5. **Local table isolation**: production scopes never display local balances as game assets; calling the local balance write API directly does not change game truth, and new real-business integration reviews can reject that path.
6. **Replicas never override truth**: when a tampered or delayed platform replica conflicts with the game query, pages and task states follow the game query/credential, and the conflict is recorded.

## 7. Decisions Pending Review

1. The retention period of the local `players` demo/history data, and whether the old workbench gains a read-only archive mode.
2. Whether game execution credentials extend `ExecutionLog` fields, adopt a structured response convention, or add a credential table; this decision couples with item 3's execution contract.
3. The minimal required contract and error-code set for account/character/asset query functions across games.
4. Per-domain default query TTL, batch task result retention, and sensitive-field masking policies.
5. Whether real game integrators allow the platform to persist any business replicas; the default recommendation is to persist only audit snapshots and task results.
