---
title: Database Monitoring
icon: database
order: 22
category:
  - Operations Manual
tag:
  - Observability
  - Database
---

# Database Monitoring

"Ops Center → Database Monitoring" performs **read-only direct-connection probing** of the game/platform's MySQL and PostgreSQL instances (not a proxy, no Agent detour): register data sources → probe with one click → snapshot-style display of connections/locks/deadlocks, with automatic integration into the [Alert Center](./alerts) when thresholds are exceeded.

## Data Source Registration

Fields (`POST /api/v1/dbmon/sources`):

| Field            | Description                                                                                                               |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `name`           | Display name                                                                                                              |
| `driver`         | `mysql` / `postgres` (only these two for now)                                                                             |
| `kind`           | `self` (self-hosted) / `aliyun` / `huawei` (cloud RDS)                                                                    |
| DSN              | **Read-only account mandatory** — registration validates and rejects `root`/superuser; responses are masked to `user:***` |
| `gameId` / `env` | Scope                                                                                                                     |
| `enabled`        | Disabled sources are not probed                                                                                           |
| `lockWaitWarn`   | Lock-wait alert threshold (seconds, default 5)                                                                            |
| `connWarnRatio`  | Connection watermark alert threshold (percentage, default 80)                                                             |

## Probed Metrics (snapshots, no history persisted)

| Domain      | Metrics                                                                                                             |
| ----------- | ------------------------------------------------------------------------------------------------------------------- |
| Connections | Current connections / `max_connections` / active (alert when watermark exceeds `connWarnRatio`)                     |
| Lock waits  | Blocking chain details: waiting ID / blocked by / table / wait seconds (alert above `lockWaitWarn`) / truncated SQL |
| Deadlocks   | Cumulative deadlock count (since statistics reset)                                                                  |
| Health      | Probe latency, OK/degraded status                                                                                   |

- MySQL uses `information_schema` / `performance_schema`; PostgreSQL uses `pg_stat_*`
- Metrics not exposed by cloud RDS are marked **degraded** (explained by a page tooltip), never guessed
- Probes time out at 5s; unreachable data sources return a clear error rather than a 500

## Threshold Integration

During probes `raiseAlertsIfNeeded` runs: lock waits above threshold → alert; connection watermark above threshold → alert. The resulting alerts flow through the Alert Center like any other (same levels, silencing, and notification channels as alert rules).

## Operational Recommendations

- **Use a monitoring-dedicated account**: `SELECT` ON information_schema/performance_schema (PG also needs `pg_stat_database`) suffices; never use a business write account
- Probing is a snapshot, not a trend: for capacity planning run a long-lived external time-series store (Prometheus exporter); this page answers the immediate question "are there locks right now / are connections maxed out"
- Register game databases one by one (one database per `(game_id, env)` in `multiGame` mode), combined with scope filtering
