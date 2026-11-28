---
title: Player Ticket Identity Review
---

# Player Ticket Identity Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. This page is
the AUD-041 方案决策 deliverable. It reviews the public player-ticket identity
gap (`internal/api/support/player.go`), compares the two candidate identity
sources (SDK signature vs player session proof), and proposes the enforcement
surface for both create and list. No code is changed by this page —
implementation is gated on the owner's contract decision.

## 1. Current state (code facts, confirmed 2026-11-28)

- `CreateTicket` (anonymous): `playerId` is taken from the request body with
  **no proof** — anyone can file tickets in any player's name.
- `ListMyTickets` (anonymous): filters **only by the submitted `playerId`**
  query parameter — ticket titles/statuses of arbitrary players are enumerable
  by scanning playerId values. No rate limit on this surface.
- `gameId`/`env` come from query parameters on both endpoints.
- The platform has **no player authentication**: player identity is owned by
  the game by construction (single-company scope model). `internal/security/jwtutil`
  provides HS256 `Sign`/`Parse` for admin tokens only; there is no JWKS/RSA
  machinery and no player token surface. The game registry (`internal/model/game.go`)
  has no per-game secret field.

## 2. The contract decision: identity source

| Option | Shape                                                                                                                                                            | Pros                                                                                                              | Cons                                                                                                                  |
| ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| (a)    | **SDK signature** — game server co-signs each request (HMAC over playerId+nonce+timestamp with a per-game secret)                                                | No token semantics to share; per-request proof                                                                    | Game client must route every ticket call through its server; no expiry semantics; list needs a signature per page too |
| (b)    | **Player session proof** — game server issues a short-lived support token (HS256 JWT: playerId+gameId+env+exp+aud) after its own player login; platform verifies | One token covers create+list+follow-ups; expiry bounded; reuses `jwtutil` HS256 parse; game keeps login authority | Needs per-game secret registration + rotation; token issuance is new game-side work                                   |

**Recommendation: (b)** — a `player-support-token` profile, issued by the game
server after its own player login, verified by the platform:

- Claims: `playerId`, `gameId`, `env`, `exp` (≤15 min), `aud: "support"`.
  Platform derives identity **from claims only** — submitted `playerId` on
  create/list is ignored (and rejected when it conflicts).
- Verification reuses the existing HS256 path (`jwtutil`), with the **per-game
  secret** as the key. No new JWT library, no JWKS.
- Option (a) remains available per-game as a fallback profile only if a game
  cannot issue tokens — same secret registry, different verification shape
  (HMAC over the canonical request). Not built in v1.

This does **not** introduce platform player authentication: the token proves
"the game vouches for this player, recently". Login, account merge, and
identity ownership stay with the game.

## 3. Enforcement surface

| Endpoint                       | Today                                   | Proposed                                                                                                                             |
| ------------------------------ | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `POST /public/support/tickets` | anonymous, playerId from body           | Requires valid player-support token; `playerId`/`gameId`/`env` from claims; body values that conflict with claims are rejected (400) |
| `GET /public/support/tickets`  | anonymous, filter by submitted playerId | Requires valid token; list filters by the token's `playerId` — **enumeration by submitted playerId is dead**                         |
| Game scope                     | query parameters, unvalidated           | From token claims; `X-Game-ID`/`X-Env` style query values ignored when a token is present                                            |

Both endpoints leave the `/public` route family (no admin session) but gain
the token gate; failure responses follow the unified error contract
(`401 unauthorized` for missing/expired tokens, `403 forbidden` for
cross-game claims).

## 4. Secret registration and rotation

- Per-game support secret registered on the game registry as a **write-only
  field** (set via admin API, never returned by any read path; masked in list
  responses by the existing sensitive-field helpers).
- Storage: the games table gains a secret column → **numbered migration**
  (schema-migration contract: column-catch pattern 0015/0021/0025/0027; model
  change and migration in the same PR).
- Rotation: at most two active versions per game (current + previous); tokens
  carry a `kid`-style version number; both versions verify during the overlap
  window. Rotation is an admin action, audited.
- When Vault is enabled (`VaultConfig`), the secret may be referenced instead
  of stored — the reference path follows the extension `SecretRefsJSON`
  precedent. Vault stays optional; the registry column is the baseline.

## 5. Acceptance mapping (design → verification scenarios)

| #   | Scenario                                                     | Expected                                                                              |
| --- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------- |
| A1  | Create with a valid token                                    | Ticket created with `playerId`/`gameId`/`env` from claims; body conflict rejected 400 |
| A2  | Create without / with expired token                          | 401 `unauthorized`                                                                    |
| A3  | List with a submitted playerId but no token                  | 401 — enumeration is impossible without a token                                       |
| A4  | List with valid token                                        | Only that player's tickets, public status only (no assignee/internal comments)        |
| A5  | Token for game A used on game B's endpoint                   | 403 `forbidden` (claims/game binding)                                                 |
| A6  | Token signed with the previous secret version during overlap | Accepted (rotation window); after the window, 401                                     |
| A7  | Admin read path                                              | Secret never appears in any game list/detail response (masked)                        |

## 6. Decisions needed from the owner

- [ ] **D1** identity source — approve option (b): game-issued HS256 `player-support-token` (≤15 min, `aud: support`), identity from claims only
- [ ] **D2** enforcement — approve token gates on both create and list; submitted playerId ignored
- [ ] **D3** secret registry — approve write-only per-game secret column + numbered migration + two-version rotation
- [ ] **D4** anonymous create — confirm there is **no** anonymous ticket path in v1 (a per-game opt-in for anonymous filing is deferred until a game asks)
- [ ] Acceptance mapping §5 (A1–A7) — confirm as the implementation's verification bar

## 7. Known boundaries

- Games must integrate token issuance (server-side, after their own player
  login). Until a game ships it, its public support surface is effectively
  disabled for players — operator-side ticket handling is unaffected.
- This proof does not authenticate the player to the platform for anything
  beyond support tickets; it must not be reused as a general player session.
- Rate limiting on the public surface is not part of this decision (identity
  first; throttle policy is a separate item).
