---
title: "ADR 0002: Game data authority ownership and the unified target reference"
---

# ADR 0002: Game data authority ownership and the unified target reference

**Status:** Proposed (2026-11-13). This ADR consolidates
[game data authority and ownership design](../../design/game-authority-ownership-design.md)
and the [player identity and business target model](../../design/player-identity-target-model-design.md)
(todo items GAME-007/GAME-008) into one decision record. It is **not accepted
yet** — the sign-off called for by both source designs ("pending design
review") belongs to the project owner. Until this ADR is flipped to Accepted,
it authorizes no implementation: no Player table change, no migration, and no
GAME-008 coding.

**Scope:** which system owns game business truth, how the legacy local
`Player/Balance` tables are positioned, where account/character/server/
channel/merge mappings come from, and the single target-reference shape that
GAME-008 implements for pages, approvals, tasks, and audit. Not covered: the
high-risk operation credential protocol (GAME-009/010) and batch/compensation
rules (GAME-011) — this ADR only fixes the interface seam to them.

## Context

The two source designs have been in review since 2026-10-17. Meanwhile the
blocked state is load-bearing:

- `internal/model/player.go` still exposes `Balance/Status` with local
  `UpdateBalance/BanPlayer` write paths; nothing marks them demo-only.
- New business work (GAME-002, GAME-009..012, every default game page) is
  sequenced behind GAME-008, which refuses to start without an approved
  target-reference decision plus real integrator identity-mapping samples.
- The risk of leaving this open is concrete: any new page wired to the local
  `players` table hardens the wrong authority, and later extraction costs a
  migration plus a trust repair.

## Decisions

Each decision below restates the source design's position with the open
points of its §7 (ownership) resolved to a concrete recommendation. Items
marked **[sign-off]** are the ones the project owner must confirm.

1. **Authority split.** The game service owns player runtime state,
   characters, assets, bans, order credits, mail, and activity effectiveness.
   Croupier owns the control plane only (contracts, installs, pages,
   permissions, approvals, tasks, execution logs, audit, snapshots, config
   versions) plus replicas explicitly declared per §2 of the ownership
   design. Croupier never becomes a game economy truth. _(not open — design
   principle, restated as decision baseline)_

2. **Local `Player/Balance` positioning.** During the compatibility period
   the local `players` table is the Croupier local demo/auxiliary account
   model. `Balance` never represents game currency; `Status` never represents
   a game ban. Real grants/bans go through game functions with credentials.
   The five-phase closure plan (freeze new dependencies → label sources →
   contract replacement → data mapping → history closure) is adopted as the
   migration boundary. **[sign-off]** on two details:
   - old workbench becomes a **read-only archive mode** (recommended), and
   - demo/history data retention is decided **per game in closure phase E**
     with admin confirmation (recommended; no global default period).

3. **Unified target reference.** The JSON shape from the identity model
   (`targetType` ∈ {`player_role`,`player_account`,`server`,`player_cohort`},
   `scope.gameId/env`, stable target IDs, `display`, `resolvedAt`,
   `identityFingerprint`) is the only cross-layer reference: approvals,
   tasks, function invocations, audit, and tickets all store the same shape —
   no per-layer abbreviations. Uniqueness is guaranteed at least by
   `(scope, serverId, roleId)`; `identityFingerprint` hashes scope + type +
   stable IDs, excluding display and search aids.

4. **Ambiguity blocking (GAME-008's core spec).** Mapping resolution returns
   0 or 1 current targets; multiple matches or missing mappings return
   `ambiguous_target` and refuse execution — no sort-order picks, no
   nickname/fuzzy auto-selection, no defaulting to `gameId/env` or host as a
   business server. Old references resolve read-only through mappings with
   both `originalRef` and `resolvedRef` recorded; they are never rewritten.

5. **Mapping sources.** Account/character/server/channel/platform identity
   mappings come from the **game authority service** (per-game data source
   capability declaration), and merge imports produce versioned, audited
   `server_merge` mappings (`fromRef → toRef`). The platform never derives
   identity mappings from display names or search results, and a mapping is
   never proof the game completed the merge. **[sign-off]**: real integrator
   must supply identity-mapping samples before GAME-008 lands its resolution
   path (already recorded as GAME-008's second blocker).

6. **Replica persistence default.** Persist only audit snapshots, approval
   object snapshots, and per-object task results. Display caches are
   short-TTL, optional, and must carry `fetchedAt/expiresAt` or render as
   stale. **[sign-off]** on the recommended default TTL of 60 seconds for
   display caches, overridable per game declaration.

7. **Execution-credential seam.** The ownership design's §7.2 (whether game
   credentials extend `ExecutionLog`, use a structured convention, or add a
   credential table) is **deferred to GAME-009/010** — the high-risk
   operation contract already owns it. GAME-008's target reference carries
   `identityFingerprint` + refs only and must not pre-commit a credential
   storage shape.

8. **Query contract minimum (GAME-008's deliverable for queries).** Every
   query function contract declares: data domain, target reference type,
   returned stable fields, `sourceSystem`, capture time, sensitive-field
   policy, cache freshness, and error codes; a failed query renders
   source-level failure + last-known-good time, never fabricated values.

## Consequences

- GAME-008 may implement, once this ADR is Accepted and mapping samples
  arrive: the Go `TargetRef` type + validators + canonical fingerprint +
  resolution/ambiguity rules + tests (landing batch 1 of the identity
  model). Player CRUD, PageSpec forms, and DB migrations are **out of scope**
  for that first batch.
- Writing game business values through local tables
  (`UpdateBalance`, local ban state) becomes a review failure for new code;
  the phase-A guard work is a follow-up, not part of GAME-008.
- If the owner amends any decision above, the change is made here first —
  the source designs stay as the detailed rationale, this ADR as the ruling
  record.

## Sign-off checklist

- [ ] Owner accepts/amends decisions 2, 5, 6 (the [sign-off] items).
- [ ] Integrator supplies identity-mapping samples (server, character,
      account, one merge case).
- [ ] Flip Status to Accepted with date; GAME-008 unblocks.
