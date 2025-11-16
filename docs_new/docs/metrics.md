# Metrics & Observability

This doc lists the built-in metrics endpoints and exported series for Server/Agent/Edge.

## Endpoints
- JSON metrics
  - Server: GET /metrics
  - Agent:  GET /metrics
  - Edge:   GET /metrics
- Prometheus text format
  - Server: GET /metrics.prom
  - Agent:  GET /metrics.prom
  - Edge:   GET /metrics.prom

## Server JSON (/metrics)
- uptime_sec
- invocations_total / invocations_error_total
- jobs_started_total / jobs_error_total
- rbac_denied_total / audit_errors_total
- logs: { debug, info, warn, error, total }
- lb_stats, conn_pool (when available)
- functions: per-function snapshot
  - invocations_total / errors_total / rbac_denied_total
  - latency_seconds: { buckets[], counts[], sum, count }

## Server Prometheus (/metrics.prom)
- croupier_invocations_total
- croupier_invocations_error_total
- croupier_jobs_started_total
- croupier_jobs_error_total
- croupier_rbac_denied_total
- croupier_audit_errors_total
- croupier_logs_total{level="debug|info|warn|error"}
- Per-function series
  - croupier_invocations_total{function_id="..."}
  - croupier_invocations_error_total{function_id="..."}
  - croupier_rbac_denied_total{function_id="..."}
  - croupier_invoke_latency_seconds_bucket{function_id="...",le="..."}
  - croupier_invoke_latency_seconds_sum{function_id="..."}
  - croupier_invoke_latency_seconds_count{function_id="..."}

## Agent JSON (/metrics)
- functions, instances, tunnel_reconnects
- logs

## Agent Prometheus (/metrics.prom)
- croupier_agent_instances
- croupier_tunnel_reconnects
- croupier_logs_total{level}

## Edge JSON (/metrics)
- tunnel metrics map + logs

## Edge Prometheus (/metrics.prom)
- croupier_logs_total{level}

## Notes
- Histogram buckets follow Prometheus defaults (0.005 .. 10 seconds). Values are best-effort for HTTP path and meant for dashboards/alerts.
- Series cardinality: per-function metrics may increase cardinality; keep function ids bounded.
- Toggles: you can disable per-function metrics via `--metrics.per_function=false` and enable per-game RBAC denied counters via `--metrics.per_game_denies=true`.

## Prometheus scrape example
```yaml
scrape_configs:
  - job_name: 'croupier-server'
    metrics_path: /metrics.prom
    static_configs: [ { targets: ['localhost:8080'] } ]
  - job_name: 'croupier-agent'
    metrics_path: /metrics.prom
    static_configs: [ { targets: ['localhost:19091'] } ]
  - job_name: 'croupier-edge'
    metrics_path: /metrics.prom
    static_configs: [ { targets: ['localhost:9080'] } ]
```

## Grafana quick panel ideas
- Query rate by function: `increase(croupier_invocations_total{function_id="$fid"}[5m])`
- Error ratio: `increase(croupier_invocations_error_total{function_id="$fid"}[5m]) / increase(croupier_invocations_total{function_id="$fid"}[5m])`
- P95 latency: `histogram_quantile(0.95, sum by (le,function_id) (rate(croupier_invoke_latency_seconds_bucket[5m])))`