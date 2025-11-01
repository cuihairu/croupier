# Configuration (YAML, Includes, Profiles)

Croupier uses Cobra+Viper for configuration. You can combine flags, environment variables and YAML files. This document summarizes precedence and patterns.

Precedence (low → high)
- Base YAML: --config base.yaml
- Include YAMLs: --config-include a.yaml --config-include b.yaml (later overrides earlier)
- Section select: server:/agent:/edge: (subtree of the merged YAML)
- Profile overlay: --profile <name> (applied from section.profiles.<name>)
- Environment: CROUPIER_SERVER_* / CROUPIER_AGENT_* / CROUPIER_EDGE_* (dots and dashes become underscores)
- Flags: highest precedence

Examples
```yaml
# server.example.yaml
server:
  addr: ":8443"
  http_addr: ":8080"
  # Optional: database (use environment for secrets)
  # DATABASE_URL: postgres://user:pass@host:5432/db?sslmode=verify-full
  log: { level: debug, format: console }
  metrics:
    per_function: true
    per_game_denies: false
  profiles:
    prod:
      log: { level: info, format: json, file: logs/server.log }
      metrics: { per_function: true }
```

Start with overlay files and profile:
```bash
./croupier server \
  --config configs/server.example.yaml \
  --config-include configs/overrides.yaml \
  --profile prod
```

Environment overrides
- Server: CROUPIER_SERVER_ADDR, CROUPIER_SERVER_HTTP_ADDR, CROUPIER_SERVER_LOG_LEVEL, ...
- Agent:  CROUPIER_AGENT_SERVER_ADDR, CROUPIER_AGENT_LOCAL_ADDR, ...

Metrics env toggles (server)
- METRICS_PER_FUNCTION=true|false to enable per-function latency histogram and counters.
- METRICS_PER_GAME_DENIES=true|false to enable per-game RBAC deny counters.

Agent Assignments & Downlink (dev)
```yaml
agent:
  assignments_api: http://localhost:8080   # poll assignments and pack export from this server
  assignments_poll_sec: 30                 # polling interval seconds
  downlink_dir: ./packs/downlink           # save/export current pack here on updates
  # optional adapter process demo (dev-only)
  adapter_prom_cmd: "go run ./adapters/prom"
  adapter_http_cmd: "go run ./adapters/http"
```

Adapter supervisor (dev)
- Agent will supervise optional adapters with graceful restart and backoff.
- Environment passed to adapter process includes: `CROUPIER_AGENT_ID`, `CROUPIER_GAME_ID`, `CROUPIER_ENV`, and passthrough `PROM_URL`/`ASSIGNMENTS_API` if present.
- Desired adapters are inferred from assignments: `prom.*` → prom adapter, `http.*|grafana.*|alertmanager.*` → http adapter. Empty assignments means allow all → start both if configured.
- After downlink import/reload, Agent polls `/api/packs/list` briefly to verify server responds.

Adapter health & logs (dev)
- Health (optional): set `adapter_prom_health_url` / `adapter_http_health_url` to an HTTP endpoint that returns 2xx when healthy; tune `adapter_health_interval_sec`.
- Logs: set `adapter_log_dir` (default `logs/`), `adapter_log_max_mb`, and `adapter_log_backups` for size-based rotation of stdout/stderr per adapter.
- Metrics: `/metrics.prom` exposes `croupier_adapter_running{adapter}`, `croupier_adapter_restarts_total{adapter}`, `croupier_adapter_healthy{adapter}`, `croupier_adapter_last_health_ts{adapter}`, `croupier_adapter_last_start_ts{adapter}`, `croupier_adapter_health_failures_total{adapter}`.
- Optional auto-restart: set `adapter_health_restart_threshold`>0 to restart adapter after N consecutive failed health checks (dev only, default disabled).

Packs endpoints & ETag
- GET `/api/packs/list` returns `{ manifest, counts, etag }` where `etag` is a content hash of the current pack (manifest/descriptors/ui/web-plugin/js/root *.pb).
- GET `/api/packs/export` streams a tar.gz of the current pack and sets `ETag` header to the same value. Set `PACKS_EXPORT_REQUIRE_AUTH=true` to require JWT + RBAC (`packs:export`) for this endpoint (default open for Agent downlink demo).
- POST `/api/packs/import` (RBAC: `packs:import`) imports a tar.gz and reloads descriptors/FDS.
- POST `/api/packs/reload` (RBAC: `packs:reload`) rescans the pack directory.
- Agent uses the `ETag` from export to confirm readiness via `/api/packs/list`.

Registry API RBAC
- GET `/api/registry` requires `registry:read` permission; UI 页面会依据角色隐藏或禁用受限操作（后端仍强校验）。

Audit API RBAC
- GET `/api/audit` requires `audit:read` permission; 支持 `game_id`、`env`、`actor`、`kind`、`limit` 过滤；可选 `offset` 或 `page`+`size` 分页（最新在前）。UI 支持自动刷新、导出 CSV。

Assignments audit
- POST `/api/assignments` 会写入审计事件（kind=`assignments.update`，target=`<game>|<env>`，meta 包含 `functions` 和 `unknown`）。可通过 `/api/audit?kind=assignments.update` 查看。

Effective config snapshot
- Validate and print merged config (strict):
```bash
./croupier config test --config configs/server.example.yaml --section server --profile prod
```

Notes
- Flags always win; prefer YAML + env for deploy, flags for local dev tweaks.
- You can keep secrets (JWT, TLS paths) in environment or external secret managers; YAML supports file paths, not secret storage.
