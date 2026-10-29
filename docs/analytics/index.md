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

| Relationship                        | Implementation and limits                                                                                                                                                                                                                                                                                                                              |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Producers → ingest → Redis          | Routes `/api/ingest/events` and `/api/ingest/payments` and authentication in `cmd/ingest/cmd/root.go`; JSON stream publishing in `internal/analytics/mq/redis_pub.go`. HTTP 202 does not confirm ClickHouse durability. Keep HMAC secrets on trusted producers.                                                                                        |
| Server bridge → Redis               | The in-process buffer and publisher in `internal/telemetry/analytics_bridge.go` write directly to `analytics:events`, without ingest HTTP. Control-plane telemetry is not game DAU.                                                                                                                                                                    |
| Worker ↔ Redis; worker → ClickHouse | Consumer-group reads, pending reclaim, retry/dead letters and detail/aggregate batches in `internal/analytics/worker/worker.go`. Delivery is at-least-once: ACK is deferred until the ClickHouse batch `Send()` succeeds, so a crash between Send and ACK redelivers rather than loses (ARCH-011 resolved; ARCH-012 crash-recovery verification open). |
| Browser → Server → databases        | `web/src/services/api/analytics.ts`, `internal/handler/routes.go`, `internal/api/analytics/warehouse.go` and the relational report implementations in that directory. Warehouse endpoints read ClickHouse; relational reports do not automatically consume warehouse events.                                                                           |

This diagram shows the default Redis path. `internal/analytics/mq/factory.go` also offers Kafka/noop publishers, but the existing worker consumes Redis; Kafka is not a connected substitute in this diagram. The built-in worker refuses to start when the deployment declares `ANALYTICS_MQ_TYPE=kafka` (ARCH-013 startup preflight), so a Kafka publisher cannot be silently paired with the Redis-only worker; no Kafka consumer is built. Professional integrations and accepted core metrics remain [target deliverables](./core-and-external).

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
