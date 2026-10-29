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

```text
Game event -> ingest -> Redis Streams -> analytics worker -> ClickHouse -> console warehouse page
```

Redis absorbs bursts and decouples game traffic from warehouse writes. The worker consumes streams, validates and normalizes events, and writes detail and aggregate tables. Console queries do not read game databases directly for the ClickHouse path.

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
