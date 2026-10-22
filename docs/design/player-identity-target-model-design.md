---
title: Player Identity and Business Target Model
---

# Player Identity and Business Target Model

## Status and Scope

- Status: design pending review; this page does not promote the existing `internal/api/player` demo CRUD into a player authority store.
- Scope: accounts, characters, servers, regions, channels, platforms, external accounts, merge mappings, and the unified target reference.
- Authority boundary: player assets, bans, orders, mail, and activities remain governed by [game data authority and ownership](./game-authority-ownership-design.md); this model only defines identification and referencing, never copying business truth.
- Current state: the `Player` of `internal/api/player/dto.go` only has the platform auto-increment `id`, `gameId`, a nickname, and display attributes — no `env`, account ID, character ID, server ID, channel, platform, or old-ID mapping; it cannot serve as a safe execution target. Approval records already carry `GameID/Env/FunctionID/TaskID/IdempotencyKey/Payload/Metadata`, but target references do not yet form a unified closure.

## Design Principles

1. `gameId/env` is the isolation boundary; servers, characters, accounts, channels, and platforms are business targets — never stuffed into env, and physical hosts never stand in for servers.
2. Display names, nicknames, emails, phone numbers, and platform nicknames are search aids only; initiation, approval, execution, and audit must use stable IDs.
3. A target reference is a complete closure; vague requests with only a character ID or only an account ID are not accepted.
4. When game capability is insufficient, fields may be optional, but a missing field must surface as "not executable" or degrade to a search result — never defaulted to `0`, `default`, or the current server.
5. All references resolve to the current snapshot before execution; a target changed after approval follows the high-risk operation contract's re-review rules.

## Identity Objects

| Object    | Suggested fields                              | Description and boundaries                                                                                                                          |
| --------- | --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Account   | `accountId`, `platform`, `externalAccountId`  | The player's login principal, which may own multiple characters; platform/external IDs describe origin only and are not user accounts of this repo. |
| Character | `roleId`, `serverId`, `accountId?`            | The in-game controlled object; single-character games without an account system may declare character = account, but never fabricate an account.    |
| Server    | `serverId`, `regionId?`, `clusterId?`         | A logical server; after merges, multiple historical servers may remain, pointing at the current logical server.                                     |
| Region    | `regionId`                                    | An operations region/cluster used only for grouping, routing, and statistics — never a substitute for `serverId`.                                   |
| Channel   | `channelId`, `platform?`                      | The login/user-acquisition channel, for attribution and display; it does not affect target uniqueness.                                              |
| Mapping   | `fromRef`, `toRef`, `mappingType`, `mappedAt` | Server merges, account migration, and external system mappings; historical references remain traceable forever.                                     |

`platform` uses a closed set (e.g. `android/ios/steam/ps/xbox/switch/web/third_party`); new values require a declaration from the game data source. `channelId` is a game-side string and is not assumed globally unique.

## Unified Target Reference

All functions, tasks, approvals, tickets, audit records, and probes store the following canonical shape:

```json
{
  "targetType": "player_role",
  "scope": { "gameId": "game-a", "env": "prod" },
  "target": {
    "accountId": "account-001",
    "roleId": "role-001",
    "serverId": "s1001",
    "regionId": "cn-east",
    "channelId": "channel-x",
    "platform": "ios"
  },
  "display": { "nickname": "display name", "accountName": "account name" },
  "resolvedAt": "2026-10-19T10:00:00Z",
  "identityFingerprint": "sha256:..."
}
```

- The initial `targetType` closed set: `player_role`, `player_account`, `server`, `player_cohort`. Operations on a player's personal assets must resolve to `player_role`; account-level bans, notifications, or queries may use `player_account`.
- `identityFingerprint` is the canonicalized hash of scope, targetType, and the stable target fields; `display`, search conditions, and extension fields are excluded.
- `player_cohort` must carry its generation rules, snapshot version, and count/quota summary, and reuse the batch-operation design — dynamic queries silently widening the approval scope are forbidden.
- Target references keep the same JSON shape wherever they enter approval payloads, task parameters, function invocations, audit records, and tickets — no per-layer private abbreviations.

## Server Merges and Old IDs

1. A merge import generates `server_merge` mappings: `fromRef=(old serverId, roleId)` → `toRef=(current serverId, roleId)`; on character-ID conflicts, the game authority's newly generated or canonical character key wins.
2. Historical tickets and audit records are never rewritten. Old references resolve read-only to the current `toRef`, while the execution context keeps both `originalRef` and `resolvedRef`.
3. New operations execute the current reference by default; replaying or compensating against an old ID must resolve through the mapping and enter a new `operationId`/approval chain.
4. Mapping conflicts, multiple current characters, or missing mappings return `ambiguous_target` for human handling — never "take the first by sort order".
5. The merge mapping itself is a high-risk config change: it must be versioned, audited, query-rollbackable, and declare its import window; a mapping is never proof that the game side completed the merge.

## One Account, Multiple Characters

- Account queries may return multiple characters; executing an operation requires the operator to explicitly pick one `player_role`.
- Account-level operations must declare their effect surface (notification, login ban, query) and whether it applies per character; silently broadcasting an account operation to all characters with business side effects is forbidden.
- Approval summaries show the account and target characters; batch-granting rewards to multiple characters under one account generates per-item references and quota checks from the batch target list.

## Approval, Execution, and Audit Linkage

- The approval payload carries at least `operationId`, the function contract fingerprint, the unified target reference, the request digest, and the snapshot time.
- `IdempotencyKey` follows the high-risk operation contract and equals `operationId`; the target reference is part of the request fingerprint, so changing the target under the same ID must conflict.
- Tasks and execution logs store `identityFingerprint`; audit additionally stores `originalRef/resolvedRef`, the approval ID, task ID, traceId, and the game execution credential.
- Display nicknames may assist, but request bodies and confirmation copy must show stable IDs; log details can jump from the target reference to the original approval and task.
- Probe targets representing a player or server reference only `server` or a dedicated probe point — probe targets are never disguised as player business targets.

## Validation Rules

1. Scope missing `gameId/env`, target missing required stable IDs, an unknown `targetType`, an undeclared platform, or an account/character shape mismatch: refuse to publish or execute.
2. The same `roleId` may exist on different servers; uniqueness is guaranteed at least by `(scope, serverId, roleId)`, plus `accountId` ownership checks when accounts exist.
3. Mapping resolution must return 0 or 1 current target; multiple results are a conflict.
4. Old references may only read mappings — never auto-rewritten by nickname or fuzzy search.
5. Search interfaces return candidate lists; they never implicitly select a candidate.

## Acceptance Scenarios

1. `s1001/role-1` and `s1002/role-1` coexist; page selection, function arguments, approval summaries, tasks, audit, and replay all keep complete references — no cross-wiring.
2. `account-001` owns `s1001/role-a` and `s1002/role-b`; without picking a character, account-level operations allow only account-level capabilities; character assets can be granted only after a character is picked.
3. An approval referencing old `s1001/role-1` before a merge still shows the original reference after the merge, alongside the resolved current `s2000/role-1`; a new compensation operation uses a new operationId and records both references.
4. After a merge, two same-character candidates with conflicting mappings return `ambiguous_target` — no execution, no default selection.
5. With only a nickname or email, the result is a candidate search only; no controlled write can be initiated.
6. With OTel disabled, audit still chains the full trail by `operationId + identityFingerprint + approvalId/taskId`.

## Landing Batches

1. Define the Go/JSON target reference and validators, first covering the rejection and fingerprint rules above; existing Player CRUD is untouched.
2. PageSpec forms and function contracts adopt the unified reference, with display layers keeping read-only nicknames.
3. Approvals, tasks, execution logs, and audit gain the fields and migrations; legacy records keep their original payloads — no backfilled fabricated identities.
4. Build the merge-mapping import/query/conflict case set; mapping capability is declared by the game data source.
5. Wire the target selectors of the player workbench, mail & compensation, bans, orders, and activities page by page against the default game pages.
