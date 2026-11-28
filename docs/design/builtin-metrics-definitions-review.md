---
title: Builtin Core Metrics Definitions Review
---

# Builtin Core Metrics Definitions Review

Date: 2026-11-28. Status: **proposal awaiting owner decision**. This page is
the DATA-001 方案决策 deliverable. The product boundary is already owner-set
(2026-10-29, [core-and-external](../analytics/core-and-external.md)): croupier
ships reusable **core game-operations metrics**; professional analytics
integrates external systems. What remained open is the **definitions** — this
page proposes them per metric group, grounded in current code. No analytics
capability is implemented or accepted by this page; until DATA-002–008 land
the computation, every definition here is paper (`口径未验收不称已可用`).

## 1. Current code facts (grounding, 2026-11-28)

| Surface                                                                                                            | Shape                                                                                | Consequence for definitions                                                                                                     |
| ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------- |
| `analytics.events` (CH, MergeTree)                                                                                 | `user_id` flat String, `event_id UUID`, `session_id`, `event_time` UTC, TTL 6 months | No account-vs-role distinction exists in the event stream; identity must be pinned by definition                                |
| `analytics.payments` (CH, MergeTree)                                                                               | `order_id` String, `amount_cents UInt64`, `currency` present, `status`               | Table-level order dedup does not exist — replay duplicates rows; dedup must be defined at query/rollup grain                    |
| `analytics.daily_users` / `daily_revenue` / `minute_online` (ReplacingMergeTree(version), key `game_id, env, d/m`) | Worker-produced rollups                                                              | `daily_revenue` key omits `currency` (DATA-004) — definitions must require currency in the grain                                |
| `retention_cohorts` (relational model) + `UpsertCohort`                                                            | Model exists; no production computation call found                                   | Cohort shape exists; definition fixes semantics before DATA-002 wires the producer                                              |
| Warehouse read endpoints (`internal/api/analytics/warehouse.go`)                                                   | DAU/new-users, minute online, revenue queries; absent DSN = disabled                 | Read shape exists; aggregate semantics inherit these definitions                                                                |
| Platform `Player` model                                                                                            | Account-level (username/nickname/game_id/status), no role/server column              | Platform identity = account; `ServerID` in tickets/cohort models is game-side and can only be a dimension, not the identity key |

## 2. Definitions (proposal per metric group)

### D1 — Identity key

`user_id` = the platform **Player ID** (account-level), scoped by mandatory
`(game_id, env)`. Role/character and server are **dimensions, never the
identity key** — an account active on two servers is one DAU. Events without a
resolvable `user_id` are excluded from activity metrics and counted under data
health (§ D9), not silently dropped.

### D2 — Active event set, DAU/WAU/MAU

- An **active day** = ≥1 event with a valid `user_id` on that business day
  (login, heartbeat, and player-action events all count; system-sourced events
  are excluded by an explicit allowlist of player-event names, not by
  `props`-field guessing).
- **DAU** = distinct `user_id` per business day. **WAU/MAU** = rolling
  trailing 7 / 28-day distinct (not ISO calendar weeks — rolling windows match
  ops usage and avoid week-boundary artifacts).
- Approximation: dashboard queries may use HLL (`uniq`) with the declared
  error; audit-grade exports use exact (`uniqExact`). The chosen mode is
  recorded per query response.

### D3 — New users (新增)

First-ever active day per `(game_id, env, user_id)` — **not** `Player` row
creation. Player CRUD is platform registration and is explicitly _not_
authoritative game registration (already established in
[core-and-external](../analytics/core-and-external.md) §1). Requires a
first-seen watermark; a user's first event after a data gap is still their
true first-seen only if the watermark covers that gap — the watermark is part
of DATA-002's computation contract.

### D4 — Business day and timezone

- Per-`(game_id, env)` **business timezone** configuration, default UTC; all
  daily aggregates bucket by business-local day; storage stays UTC
  (`event_time`). No metric mixes timezones; the timezone in effect is
  reported with every aggregate response.

### D5 — Near-real-time online (近实时在线)

- Heartbeat-based: a user is **online at minute m** if they emitted a
  heartbeat within the trailing window `W = 90s` of `m`. Minute bucket =
  `event_time` truncation; daily peak = max per-minute value.
- Semantics are declared **approximate concurrent users (HLL)** — never
  presented as an exact connection count. `W` is config-gated per game.

### D6 — Retention (D1/D7/D30)

- **Cohort day** = first-active day (D3 alignment). **Return** = active on
  cohort day + N. **Denominator** = cohort size (distinct users first-active
  that day). Cohort membership is immutable once written for a mature cohort.
- **Mature window**: a D-N cohort is reportable only after N days of business
  calendar have elapsed; immature cohorts report `immature` — **never 0%**
  (a 0% display fabricates churn).
- Multi-server (MMORPG) cohorts may add `ServerID` as an optional dimension;
  the platform-level cohort remains account-grain.

### D7 — Payments (payers/orders/gross/refunds/net/ payer rate/ARPU/ARPPU)

- **Order grain**: dedup at `(game_id, env, order_id)`, first record wins
  (table-level replay duplicates exist — the dedup rule is part of the
  rollup/query contract; DATA-004 carries the storage repair).
- **Status closed set (v1)**: `paid`, `refunded`. **Gross** = Σ paid amounts;
  **refunds** = Σ refunded amounts **attributed to the refund event's own
  business day** (no restatement of the purchase day in v1); **net** =
  gross − refunds, same day grain.
- **Payers** = distinct users with ≥1 `paid` order in the window; **payer
  rate** = payers / DAU (same window); **ARPU** = gross / DAU; **ARPPU** =
  gross / payers. All ratios report their denominator.
- **Units**: integer minor units + mandatory currency code. **No
  cross-currency addition** — aggregates are per-currency rows; conversion to
  a reporting currency is explicit future work with rate snapshots (not v1).
  `daily_revenue` must carry currency in its key (aligns with DATA-004).

### D8 — Late data and restatement (迟到更正)

- Per-`(game_id, env)` **watermark**: daily activity/revenue aggregates
  recompute until maturity (`T+3` business days default, config-gated);
  retention cohorts recompute until their window matures (D6).
- After maturity, corrections create a **versioned restatement** (the
  ReplacingMergeTree `version` column already supports this) with the
  restatement visible in data health — never a silent edit.

### D9 — Missing vs real zero (缺测 vs 真零)

- No ingest data ≠ zero: every aggregate response distinguishes "no data for
  window" from "genuinely zero" (the data-health group — ingest lag, backlog,
  dead letters — is the companion surface, reusing `pipeline_monitor`).
- A metric whose source table is unavailable reports **unavailable**, never
  falls back to a semantically different table (binding from
  [core-and-external](../analytics/core-and-external.md)).

## 3. Boundary confirmation (owner-set 2026-10-29, restated)

Built-in scope = the six groups above (acquisition/activity, online,
retention, payments, shared exploration, data health). Complex self-service
analysis, funnels/paths, attribution, predictive LTV, professional
segmentation = external professional analytics (DATA-010 investigation;
DATA-011 contract). Existing behavior/level pages remain with honest
limitations. This page selects no vendor and wires no integration.

## 4. Acceptance mapping (definition → verification scenarios for DATA-002–008)

| #   | Scenario                                                | Expected                                                             |
| --- | ------------------------------------------------------- | -------------------------------------------------------------------- |
| A1  | Duplicate event replay (same `event_id`) within one day | DAU counts the user once; ingest health records the duplicate        |
| A2  | Account active on two servers same day                  | One DAU; per-server split appears only as a dimension                |
| A3  | Event at 23:59 UTC for a UTC+8 game                     | Buckets into the next business day                                   |
| A4  | First event ever for a user                             | Counted in `new_users` on that business day                          |
| A5  | D7 cohort, 5 days after cohort day                      | Reported `immature`; D1 cohort same day reported with real value     |
| A6  | Order replay / duplicate `order_id`                     | Counted once; gross unchanged                                        |
| A7  | Refund 3 days after purchase                            | Purchase day gross unchanged; refund day net reduced                 |
| A8  | Two currencies same day                                 | Two per-currency rows; no summed total without explicit conversion   |
| A9  | Ingest down for 2 hours                                 | Affected day marks "no data" for the gap, not zero; health shows lag |
| A10 | Restatement after maturity                              | New version row; old value visible in health/audit trail             |

## 5. Decisions needed from the owner

- [ ] **D1** identity — approve account-level `user_id` (Player ID), role/server as dimensions only
- [ ] **D2** activity — approve any-player-event activity set + rolling WAU/MAU + HLL-with-declared-error defaults
- [ ] **D3** new users — approve first-active-day (not Player CRUD)
- [ ] **D4** timezone — approve per-(game,env) business timezone, default UTC
- [ ] **D5** online — approve heartbeat window 90s + approximate semantics
- [ ] **D6** retention — approve cohort = first-active day, `immature` not 0%
- [ ] **D7** payments — approve order dedup grain, refund-day attribution, per-currency no-sum
- [ ] **D8** restatement — approve T+3 maturity + versioned restatement
- [ ] Acceptance mapping §4 (A1–A10) — confirm as the DATA-002–008 verification bar
