---
title: Analytics overview
---

# Analytics overview

> **Product boundary/status correction, 2026-10-29:** built-in shared core operations metrics, external professional analytics. Existing screens/models/pipelines are not end-to-end metric acceptance. Retention computation, source consistency and aggregate semantics still have gaps. See [scope, source evidence and delivery plan](./core-and-external). Metric groups below are a catalog, not a statement that all are implemented.

The analytics stack supports game behavior, retention, revenue, and warehouse reporting. Queries converge in the Analytics console; ingestion and storage are intentionally separated from the control plane.

## Pipelines

| Pipeline             | Source                   | Path                                                                             | Best suited for                                                                                      |
| -------------------- | ------------------------ | -------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| ClickHouse pipeline  | Game clients and servers | HMAC-authenticated `ingest` -> Redis Streams -> `analytics-worker` -> ClickHouse | High-volume events, warehouse detail, and pre-aggregated reports.                                    |
| Server GORM          | Server business models   | Per-game relational tables                                                       | Business-entity joins and moderate-volume overview, retention, behavior, payment, and level reports. |
| OpenTelemetry bridge | Server telemetry         | In-process bridge -> Redis Streams -> ClickHouse pipeline                        | Server-side automatic events without game-client changes.                                            |

<div class="analytics-diagram">

![Actual analytics topology: ingest, worker and Server processes, in-process bridge, and separate data stores](/architecture/analytics-en.svg)

</div>

[Open the full SVG architecture diagram](/architecture/analytics-en.svg) · [Chinese SVG](/architecture/analytics-zh.svg)

Arrows represent writes or query requests/responses, not a timeline. Dashed lines are optional; crossings without dots do not join. The bridge and Analytics API are Server modules. Browsers call Server APIs rather than connecting to databases. Warehouse and relational reports use separate sources.

## Current wiring and source evidence

| Relationship                        | Implementation and limits                                                                                                                                                                                                                                                    |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Producers → ingest → Redis          | Routes `/api/ingest/events` and `/api/ingest/payments` and authentication in `cmd/ingest/cmd/root.go`; JSON stream publishing in `internal/analytics/mq/redis_pub.go`. HTTP 202 does not confirm ClickHouse durability. Keep HMAC secrets on trusted producers.              |
| Server bridge → Redis               | The in-process buffer and publisher in `internal/telemetry/analytics_bridge.go` write directly to `analytics:events`, without ingest HTTP. Control-plane telemetry is not game DAU.                                                                                          |
| Worker ↔ Redis; worker → ClickHouse | Consumer-group reads, pending reclaim, retry/dead letters and detail/aggregate batches in `internal/analytics/worker/worker.go`. ACK and warehouse persistence are not atomic; ARCH-011/012 remain open.                                                                     |
| Browser → Server → databases        | `web/src/services/api/analytics.ts`, `internal/handler/routes.go`, `internal/api/analytics/warehouse.go` and the relational report implementations in that directory. Warehouse endpoints read ClickHouse; relational reports do not automatically consume warehouse events. |

This diagram shows the default Redis path. `internal/analytics/mq/factory.go` also offers Kafka/noop publishers, but the existing worker consumes Redis; Kafka is not a connected substitute in this diagram. Professional integrations and accepted core metrics remain [target deliverables](./core-and-external).

### Batch validation and retry semantics

- Ingest validates the whole batch before publishing anything (DATA-016): an invalid item returns `400` with `details.index` and enqueues nothing.
- A publish failure mid-batch returns `500 queue_write_failed` with `details.enqueued` / `details.failed_index` / `details.total` (DATA-017): items before the failure are already in the stream and are disclosed, not hidden behind a bare batch-level error.
- Retry contract: re-submit the same batch unchanged. Events without a producer `event_id` get a deterministic UUIDv5 derived from the payload fingerprint in the worker (`internal/analytics/worker/worker_insert.go`, `deriveEventID`), so re-submission and worker redelivery collapse onto the same ClickHouse natural key instead of inserting random new ids. A producer-supplied valid UUID `event_id` is honored as-is.
- Amount precision (DATA-018): `amount_cents` must be a positive integer within the uint64 range. Ingest decodes JSON numbers with `UseNumber` and parses digit-for-digit — fractional, negative and oversized values are rejected `400` with the item index; the worker parses the amount the same way and dead-letters a bad row with a reason instead of truncating through float64.
- Retry retention (DATA-019): failed messages requeue into the **source stream without any retention cap of their own** — source-stream trimming (e.g. `ANALYTICS_REDIS_MAXLEN`) belongs to the publisher's configuration. A retry copy can no longer evict unread backlog entries once the backlog exceeds the old 1000-entry cap; retry volume is bounded by `retry_count` (dead-lettered after `maxPendingRetries`).
- Aggregate recovery (DATA-020): every touched minute/day carries a **durable flush marker** in Redis (`ANALYTICS_AGG_DIRTY_SET`), so a crash after the raw event was ACKed but before the aggregate row landed still leaves a flush obligation after restart. When a best-effort HLL write fails, a separate rebuild marker (`ANALYTICS_AGG_REBUILD_SET`) makes the next flush re-derive that HLL from the persisted `analytics.events` rows (HLL union is idempotent). A rebuild failure keeps the marker and does not persist a false zero; a healthy HLL write incurs no rebuild query.
- Pipeline alert recovery (DATA-022): `event_stream_stalled` / `event_volume_drop` alerts close only on **positive evidence** — a scope row actually read in the current round that satisfies the recovery rule. A scope missing from the discovery window (fully silent = the very stall being alerted), a failed query, or a partially read result set is treated as unknown and keeps the alert firing (`events_query_failed` marks the unknown round separately). Recent and baseline rates are each divided by their own window length, so a non-default window pair does not skew the comparison.
- Consumer group initialization (DATA-023): a freshly created group starts at `WORKER_GROUP_START` — default `0`, so a worker started after the first inbound events consumes the retained backlog; `$` opts into latest-only. An **existing group is never repositioned at startup**: the Redis-native cursor keeps the consumption position across restarts and a stale application checkpoint cannot replay ACKed data. The persisted checkpoint is applied only when creating a group (resume after Redis lost the group) or through the explicit `WORKER_GROUP_RESET=true` operator reset. Group creation failures are logged, never silently discarded; a `BUSYGROUP` race against another worker keeps that worker's cursor.
- Window-wide behavior aggregation (DATA-024): summary, heatmap, adoption breakdown, funnel, paths and level/episode/map reports aggregate the **entire query window** by paging through every matching event, trusting the model's authoritative `count` to know when a type is exhausted (requested page sizes are re-normalized by `ListEvents`). Windows larger than the aggregation bound (`maxWindowEvents`, 50000) return a bounded sample and disclose it in the response `sample` object (`totalEvents`/`scannedEvents`/`sampled`/`coverage`), so a partial read is never presented as complete; the paged events endpoint keeps its own page-size contract.
- Realtime metric sources (DATA-025): `onlineUsers` counts **distinct users** with a `heartbeat`, `session_start` or dotted `session.start` event inside the 15-minute realtime window, and `activeSessions` counts `session_start`/`session.start` events in the same window. Business events (`login`, `purchase`, ...) feed throughput (`eventsPerMinute`, all behavior events / 15 minutes) but no longer inflate online or session counts. The `/analytics/realtime` response includes a `source` object naming each signal and the window length, and the realtime online series uses the same liveness events. Boundary: there is no player-session/heartbeat table, so "online" is an activity liveness proxy derived from analytics events, not a count of live sockets; a user with no liveness event in the window is offline by this definition even if still connected.
- Boundary: the payments table has no dedup column; identical payment re-submission still appends a second raw row and relies on `order_id`-keyed read semantics for reconciliation (tracked by DATA-004/051/056).

## Metric groups

- **Foundation:** DAU, WAU, MAU, stickiness, and D1/D7/D30 retention.
- **Progression and economy:** level completion, conversion, source sinks and sources, balance signals.
- **Engagement and social:** session length, feature use, team and guild activity, messaging.
- **Combat and content:** win-rate distribution, character balance, ranked distribution, PVP/PVE participation.
- **Campaign and operations:** participation, completion, incremental revenue, push delivery and conversion.
- **Experience:** page dwell time, interaction and error rates, discovery, latency, and network quality.

## Operational shape

1. Start Redis and ClickHouse.
2. Run `ingest` with the configured HMAC secret.
3. Run `analytics-worker`.
4. Run the Server and open the Analytics console.

Schema and pipeline changes should be additive where practical. Existing events must remain queryable while new dimensions or aggregates are introduced.

## Reading path

- Start with the game metrics map for definitions.
- Use the collection architecture page for event ownership.
- Use the ClickHouse schema page before adding queries or aggregates.
- Use best practices and troubleshooting pages for rollout and failure handling.
