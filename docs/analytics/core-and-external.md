---
title: Shared core metrics and external professional analytics
---

# Shared core metrics and external professional analytics

Date: 2026-10-29. The user set the product boundary: **Croupier provides reusable core game-operations metrics; professional analytics integrates external systems**. The metric list below is target scope pending DATA-001 definitions. This review changes documentation/TODO only, implements no new analytics capability and selects no external vendor.

## Existing assets versus an accepted product

| Reusable asset                                                 | Evidence                                                                           | Current limit                                                                                                                                                                                       |
| -------------------------------------------------------------- | ---------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Event/payment ingest and Redis→worker→ClickHouse               | cmd/ingest/cmd/root.go, internal/analytics/mq, internal/analytics/worker/worker.go | Real pipeline exists; batch durability/ACK gaps remain (ARCH-011/012)                                                                                                                               |
| Relational overview/payments/behavior/retention APIs           | internal/api/analytics and internal/model/analytics_*                              | Overview uses relational models and local Player counts; retention reads cohort rows, with no production UpsertCohort computation call found. These APIs do not automatically read warehouse events |
| Read-only warehouse endpoints                                  | internal/api/analytics/warehouse.go                                                | DAU/new users, minute online and revenue queries exist; absent DSN is disabled. Aggregate semantics still need repair                                                                               |
| Overview/realtime/retention/payments/warehouse UI              | web/src/pages/Analytics, web/src/services/api/analytics.ts                         | Overview adapter hardcodes d1/d7/d30 to null; overview and warehouse use different sources. Screens are not end-to-end acceptance                                                                   |
| Extension configuration/install/disable and export foundations | internal/api/extension, internal/core/extension, internal/extensions               | Reusable integration mechanisms, not proof of an integrated professional BI product                                                                                                                 |

Control-plane function telemetry is not player activity. function.call counts are not game DAU, and local Player CRUD rows are not authoritative game registrations.

## Built-in minimum shared scope — target design

| Group                | Proposed built-in metrics                                                   | Required definitions                                                                                                                          |
| -------------------- | --------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Acquisition/activity | New users, DAU/WAU/MAU, trends                                              | Registration/first-active events, account versus role identity, game/env, business day/timezone, missing versus real zero                     |
| Online               | Near-real-time online, daily peak/trends                                    | Heartbeat/offline window, sampling lag and HLL error; minute activity is not automatically an exact concurrent-connection count               |
| Retention            | D1/D7/D30 cohorts/trends                                                    | Cohort membership, return event, denominator, calendar boundaries, mature observation windows, late data/recomputation                        |
| Payments             | Payers/orders, gross/refunds/net, payer rate, ARPU/ARPPU                    | Unique orders/state transitions, currency/integer minor unit, accounting time, denominators and deduplication; no raw cross-currency addition |
| Shared exploration   | Date/scope and supported-dimension filters, comparisons, controlled exports | Source, window, freshness, unit, approximation/unavailability per metric; do not invent absent region/channel dimensions                      |
| Data health          | Ingest lag, backlog, dead letters, missing samples                          | Reuse pipeline_monitor; silence may mean missing data rather than a real zero                                                                 |

Relational storage and ClickHouse may implement queries at different scales, but one logical metric needs one versioned definition and an explicit source. Do not silently fall back to a semantically different table when a warehouse fails. Core metrics must not require a professional external product.

## Confirmed source gaps

1. `configs/clickhouse/initdb/010_analytics.sql` uses unparameterized ReplacingMergeTree and includes version in ORDER BY. Distinct versions therefore have distinct sorting keys; FINAL cannot replace them by day/game/env. The comment promising maximum-version selection does not match the definition. Existing tables require migration, not just an edited init SQL (DATA-003).
2. worker.flush clears revAgg after each daily_revenue write; touchRevenue accumulates interval deltas, not a full-day snapshot. Its key omits currency and it does not deduplicate by order_id. Replay/refunds and currency semantics need repair (DATA-004).
3. minute_online writes full HLL snapshots to SummingMergeTree. Late events can flush the same minute again and add a full snapshot twice. Snapshot/delta and approximate-online semantics need an explicit contract (DATA-003).
4. Cohort production computation and a unified metric source remain missing parts of the end-to-end flow; existing models/read APIs do not establish this (DATA-002/005–008).

Items 1/3 are source-based conclusions checked against upstream semantics; no real ClickHouse result validation ran. Official check date **2026-10-29**: ReplacingMergeTree identifies duplicates by ORDER BY and uses an explicit ver parameter for version selection; SummingMergeTree sums numeric values for matching keys. References: [ReplacingMergeTree](https://clickhouse.com/docs/reference/engines/table-engines/mergetree-family/replacingmergetree), [SummingMergeTree](https://clickhouse.com/docs/reference/engines/table-engines/mergetree-family/summingmergetree). Repository driver: v2.43.0; Compose tag: 23-alpine, floating. Deployed version/migration behavior remain NOT RUN.

## External professional analytics — target design

Complex self-service querying/drill-down, advanced funnels/paths, ad attribution/ROI, predictive LTV, professional segmentation and large-scale modeling belong to external professional analytics. Existing behavior/level pages remain available with honest limitations; this review does not delete them. No vendor or vendor authorization scheme has been selected; DATA-010 registers the investigation.

Croupier should reuse its extension foundations for:

- Versioned, sourced metric/event/read-only dataset outputs constrained by authorized gameId/env, time window and supported dimensions.
- Connection testing, configuration, field/metric mapping, controlled export/synchronization and report links scoped to the current context.
- External identity/short-lived credentials, authorization mapping, auditing, revocation/disable and degradation. Enforce scope server-side or through restricted datasets; browser filters are not isolation, and warehouse credentials must not reach browsers.
- Failure isolation: an external analytics outage must not block core operations or built-in metrics. A dashboard result does not authorize a game write.

Read-only datasets, export, APIs or controlled embedding depend on the selected product's actual capabilities and require official checks and a reviewed contract. A reserved interface or link button is not integration acceptance. No new analytics control service is introduced.

## Delivery and acceptance

1. Define common metrics and semantics (DATA-001); investigate professional integration in parallel (DATA-010).
2. Repair reliability/aggregation and connect events→computation→query→UI using existing assets (DATA-002–008, ARCH-011/012).
3. Reconcile fixed cross-scope/timezone, duplicate/out-of-order, multicurrency/refund and immature-cohort samples per metric. Record source/freshness; missing real environments remain NOT RUN.
4. Deliver one real external adapter, then validate scope, revocation, timeout and outage isolation (DATA-011–013).

Stable task IDs, dependencies and conditions live in the root todo.md. These are future deliverables, not completed implementations.
