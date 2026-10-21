---
title: Ops Health Monitoring Notes
---

# Ops Health Monitoring Notes

This document verifies the implementation along the line of "endpoints and probes → metrics → alert thresholds → logs and integrations". The current state is a hybrid architecture of local and centralized observability: the server exposes public health endpoints and an optional Prometheus endpoint; the Agent collects system metrics and executes business probes; central rules, alert lifecycle, and external dispatch live in the server.

## Health Check Endpoints

| Endpoint                                                                          | Auth          | Purpose                                                                                                   |
| --------------------------------------------------------------------------------- | ------------- | --------------------------------------------------------------------------------------------------------- |
| `GET /healthz` or `GET /api/v1/monitoring/healthz`                                | Public        | Checks three components — database, Agent registry, and ops status; any failure makes the whole unhealthy |
| `GET /api/v1/monitoring/metrics`, `/status`                                       | Protected     | JSON snapshot including database latency, Agent health, function counts, and ops summary                  |
| `GET /metrics/prometheus`                                                         | Public scrape | Optional Prometheus exposition, disabled by default                                                       |
| `GET /api/v1/ops/health`, `POST /api/v1/ops/health/run`, `PUT /api/v1/ops/health` | Auth required | Ops-page health check list; currently only stores configuration and synthesized status                    |
| `POST /api/v1/ops/probe/diagnose`                                                 | Auth required | Manually executes one configured server probe                                                             |

- Routing: `internal/handler/routes.go:101` mounts the public monitoring group, `internal/handler/routes.go:104` conditionally mounts the Prometheus exposition; `internal/handler/routes.go:325` registers `/healthz`, `internal/handler/routes.go:332` registers the protected `metrics/status`.
- Health aggregation: `internal/api/monitoring/service.go:23` checks the three components; `internal/api/monitoring/service.go:97` executes `SELECT 1` and records latency; `internal/api/monitoring/service.go:121` counts total registered Agents, non-expired Agents, and enabled functions; `internal/api/monitoring/service.go:156` aggregates ops status. `internal/api/monitoring/service.go:179` combines the components' `ok` booleans into overall health.
- Prometheus endpoint: `internal/config/config.go:306` defines `telemetry.prometheus.enabled/path`, `internal/config/config.go:315` default path is `/metrics/prometheus`. The collector registers Go, process, and platform collectors at `internal/api/monitoring/prometheus.go:88`, with a 5-second database collection timeout (`internal/api/monitoring/prometheus.go:53`).
- **The ops-page health checks currently do not perform real probing**: `internal/api/ops/helpers.go:935` sets `ok=true` directly for all checks, and `internal/api/ops/helpers.go:942` notes that tcp/http/https only record synthesized success without making network connections or using `TimeoutMs`. It is not a production health probe.
- **Plan**: converge the `OpsHealthCheck` executor onto `core/healthprobe`, honoring the configured timeout, expected assertions, and interval; background scheduling reuses the Agent probe state machine, with failure windows converting to Alerts.

## Agent Health Probes

The Agent-side config `probe.targets[]` supports the types `http`, `tcp`, `db`, `protocol`.

- Target contract: `core/healthprobe/targets.go:25` defines the target fields; `core/healthprobe/targets.go:51` applies the explicit interval, otherwise defaulting to 30 seconds.
- Default thresholds: `core/healthprobe/probe.go:95` opens a window after 3 consecutive failures and closes it after 1 consecutive success; at a 30-second interval the worst case is about a 90-second window.
- Default timeouts: HTTP 5 seconds (`core/healthprobe/targets.go:69`, `core/healthprobe/healthprobe.go:48`; 2xx/3xx status codes pass, see `core/healthprobe/healthprobe.go:67`); TCP 2 seconds (`core/healthprobe/targets.go:78`); DB defaults to 5 seconds and executes `SELECT 1` or a custom read-only query (`core/healthprobe/db.go:11`, `core/healthprobe/db.go:19`, `core/healthprobe/db.go:42`); protocol 5 seconds per step, 2 seconds to connect (`core/healthprobe/protocol.go:24`).
- Execution location: `internal/app/agent/probe_manager.go:19` assembles `probe.targets[] → Manager → ProbeUplink → MetricsReport`; `internal/app/agent/probe_manager.go:25` makes explicit that probes only observe and never trigger process recovery. `configs/agent.yaml:78` is a target example.
- Sampling policy: `core/healthprobe/policy.go:29` sets the accelerated-reporting floor for anomalies at 5 seconds; `core/healthprobe/policy.go:192` computes the actual interval from the target and failure-window state. `internal/app/agent/probe_manager.go:62` holds the policy snapshot and enables acceleration during failure windows.

The server also has a local probe surface, for dependencies or third-party APIs visible only to the server. `internal/svc/server_probe.go:3` documents that perspective; `cmd/server/root.go:461` assembles it, `cmd/server/root.go:473` starts it. Target contract and default thresholds match the Agent's (`configs/server.yaml:206`); an empty target set is not assembled (`internal/svc/server_probe.go:31`), and an invalid target fails startup (`internal/svc/server_probe.go:53`).

## Metrics

### Server Platform Metrics

| Metric                          | Type     | Meaning                                  |
| ------------------------------- | -------- | ---------------------------------------- |
| `croupier_db_up`                | Gauge    | Database connectivity, 1/0               |
| `croupier_db_latency_ms`        | Gauge    | Database ping latency in ms              |
| `croupier_agents_total`         | Gauge    | Number of registered Agents              |
| `croupier_agents_healthy`       | Gauge    | Number of Agents with unexpired sessions |
| `croupier_functions_registered` | Gauge    | Number of currently enabled functions    |
| `go_*`, `process_*`             | Standard | Go runtime and process metrics           |

Evidence: `internal/api/monitoring/prometheus.go:30` registers metric descriptors, `internal/api/monitoring/prometheus.go:53` collects database metrics at scrape time, `internal/api/monitoring/prometheus.go:69` sends the registry metrics, `internal/api/monitoring/prometheus.go:88` attaches the Go/process standard metrics.

### Agent System and Business Metrics

- `core/metrics/collector.go:88` collects CPU, memory, disk, network, managed processes, probe statistics, and Go runtime custom metrics.
- Reported every 30 seconds by default: `internal/app/agent/upstream.go:586` is the loop, `internal/app/agent/upstream.go:588` falls back to 30 seconds on zero values; the config entry is `cmd/sidecar-agent/root.go:648`.
- Metrics inbound: `internal/server/control_handler.go:1010` receives `MetricsReport`; `internal/platform/registry/metrics.go:143` writes the in-memory ring and persists asynchronously, `internal/platform/registry/metrics.go:171` triggers alert rules asynchronously. Defaults: 10 reports per Agent, 2000 total, 7 days of history, 1-hour cleanup (`internal/platform/registry/metrics.go:115`).
- Liveness is not a dedicated heartbeat channel but the registry session expiry time: `internal/api/monitoring/service.go:142` counts only Agents whose `ExpireAt` has not passed as healthy. The heartbeat loop is driven by `core/register/loops.go:76`, defaulting to 3 seconds.
- Managed-process resource thresholds only flag, never kill processes; automatic restart backoff defaults to 1 second initially, capped at 60 seconds, and trips a breaker after 5 consecutive failures requiring manual reset (`configs/agent.yaml:55`).

## Alert Rules and Thresholds

### Threshold Rules

Alert rules are stored in the database; metric paths support CPU, memory, disk, custom, and probe aggregate metrics. Model and evaluation semantics are in `internal/model/alert_rule.go:10`. Evaluation is hooked after every Agent metrics report: `internal/svc/service_context.go:592` constructs the evaluator and wires it into `MetricsStore.SetOnReport`; `internal/platform/alertrule/evaluator.go:49` iterates enabled rules, `internal/platform/alertrule/evaluator.go:63` extracts the current value and compares.

- Sustained triggering: `internal/platform/alertrule/evaluator.go:74` clamps `ForCount` below 1 up to 1; the trigger fires only after consecutive hits reach the threshold; misses reset at `internal/platform/alertrule/evaluator.go:83`.
- Default cooldown: `internal/platform/alertrule/evaluator.go:93` uses `CooldownSeconds`, falling back to 5 minutes for non-positive values.
- Configurable threshold/level/scope: rule CRUD at `internal/api/alert/rules.go:141`, `internal/api/alert/rules.go:176`, routing at `internal/handler/routes.go:744`. `internal/platform/alertrule/evaluator.go:7` lists supported paths; the four probe fields are at `internal/platform/alertrule/evaluator.go:15`.
- **The platform has no built-in alert thresholds**: thresholds, comparators, `ForCount`, cooldown, and scope are all fields on the rule row. The Prometheus expressions in `docs/operations/monitoring.md:199` are deployment-side examples only, not an alert configuration auto-loaded by the repo; `croupier_server_requests_total` there is unimplemented.

### Probe Window Alerts

- When consecutive failures reach `failThreshold` a window opens, and the Agent reports the window event immediately; the server persists it idempotently with `alertID=probe:<agentId>:<target>`. `internal/server/control_handler.go:917` dispatches window events, `internal/server/probe_lifecycle.go:90` handles firing; `internal/server/probe_lifecycle.go:105` handles recovery, with replays deduplicated.
- Probe alerts are fixed at warning severity (`internal/server/probe_lifecycle.go:120`).
- Anomaly details are reported immediately, up to 10 per failure window per target by default (`internal/app/agent/probe_manager.go:42`); the server keeps the most recent 5 details in the alert details (`internal/server/probe_lifecycle.go:29`).
- On the metrics side there are also `probe.<target>.successRate/failed/checked/latencyP95Ms`, aggregated over the 30-second reporting window; `internal/platform/alertrule/evaluator.go:233` fetches values by target.

### Notification Dispatch

- `internal/platform/alertrule/evaluator.go:110` creates the firing Alert; `internal/platform/alertrule/evaluator.go:140` only persists when the notifier is nil. The production assembly at `internal/svc/service_context.go:594` currently passes nil, then `internal/svc/service_context.go:597` registers the outbound chain.
- Alert outbound channels, herald, and maintenance-window suppression are out of scope here; the platform has no general mechanism that auto-resolves metric-threshold alerts on recovery.

## Logs and Monitoring Integration

### Logs

- Config structure: `core/logx/logging.go:17` supports `debug/info/warn/error`, `console/json`, stdout/stderr, and rotating-file fields.
- Server defaults: `cmd/server/root.go:153` sets an empty level to `info`, `cmd/server/root.go:157` sets an empty format to `console`, `cmd/server/root.go:160` initializes logging.
- JSON and rotation: `core/logx/logging.go:71` sets the level, `core/logx/logging.go:72` selects the JSON handler, `core/logx/logging.go:51` uses lumberjack file rotation, `core/logx/logging.go:54` writes console and file synchronously.
- **Automatic log forwarding to Loki/OTLP is currently unimplemented**: there is no built-in collector client. Integration is via container stdout collection or mounting the rotating file, then collecting externally with Promtail/Fluent Bit/Vector. The plan is to add an optional OTLP logs exporter while keeping local rotation as the fallback.

### Prometheus / Grafana

1. Enable it in the server config:

```yaml
telemetry:
  prometheus:
    enabled: true
    path: /metrics/prometheus
```

2. Prometheus scrapes that path. The repo ships a local telemetry stack config that scrapes the OTel Collector's 8889 exporter; the 15-second global scrape interval is at `configs/prometheus.yml:3`, the Collector target at `configs/prometheus.yml:11`.
3. Grafana dashboards must be imported manually; the repo ships no auto-provisioned dashboards.

### OpenTelemetry

OTLP uses HTTP and can enable tracing and metrics simultaneously. Config mapping is at `internal/telemetry/provider.go:28`; `internal/telemetry/provider.go:83` initializes tracing, `internal/telemetry/provider.go:92` initializes metrics. The default collector is `http://localhost:4318` (`internal/telemetry/provider.go:247`), overridable via `OTEL_EXPORTER_OTLP_ENDPOINT`; tracing batches every 5 seconds with a configurable sampling rate (`internal/telemetry/provider.go:150`), metrics push every 30 seconds (`internal/telemetry/provider.go:190`).

`configs/telemetry.example.yaml:5` provides examples of service identity, collector, sampling rate, and the Prometheus switch; `configs/otel-collector-config.yaml:8` exposes OTLP gRPC/HTTP, `configs/otel-collector-config.yaml:19` exports to Jaeger and Prometheus respectively.

**Currently unimplemented**: server request totals, request-duration histograms, and automatic HTTP middleware metrics; probe/Agent metrics are not automatically converted into Prometheus metrics. The plan is to add OTel HTTP metrics in the request middleware and dispatch layer, and extend the platform collector to probe windows, alert counts, and status-source adjudication counts.

## Current Gaps Summary

| Gap                                | Current state                                                  | Plan                                                                    |
| ---------------------------------- | -------------------------------------------------------------- | ----------------------------------------------------------------------- |
| Ops-page health checks             | Manual run always succeeds (`internal/api/ops/helpers.go:942`) | Move to `core/healthprobe` and wire in failure windows                  |
| HTTP service metrics               | Only Go/process and five platform Gauges                       | Add request counters, duration, and error rate                          |
| Log forwarding                     | No built-in Loki/OTLP logs outlet                              | Add an optional exporter, keeping stdout/file as fallback               |
| Metric alert auto-recovery         | Rules no longer matching do not update old Alert states        | Add AlertID binding, recovery detection, and a resolved lifecycle       |
| Third-party monitoring SLO summary | No SLO/error-budget objects                                    | Export probe windows and alert counts first, then build SLO computation |
