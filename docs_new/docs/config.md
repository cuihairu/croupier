# Configuration (YAML, Includes, Profiles)

Croupier uses Cobra+Viper for configuration. You can combine flags, environment variables and YAML files. This document summarizes precedence and patterns.

## Precedence (low → high)
- Base YAML: --config base.yaml
- Include YAMLs: --config-include a.yaml --config-include b.yaml (later overrides earlier)
- Section select: server:/agent:/edge: (subtree of the merged YAML)
- Profile overlay: --profile <name> (applied from section.profiles.<name>)
- Environment: CROUPIER_SERVER_* / CROUPIER_AGENT_* / CROUPIER_EDGE_* (dots and dashes become underscores)
- Flags: highest precedence

## Examples

```yaml
# server.example.yaml
server:
  addr: ":8443"
  http_addr: ":8080"
  # Database (YAML preferred; flags/env can override per-env)
  db:
    driver: auto      # postgres | mysql | sqlite | auto
    dsn: ""          # DSN/URL. Examples:
    # Postgres: postgres://user:pass@host:5432/croupier?sslmode=disable
    # MySQL (URL): mysql://user:pass@host:3306/croupier?charset=utf8mb4
    # MySQL (DSN):  user:pass@tcp(host:3306)/croupier?parseTime=true&charset=utf8mb4
    # SQLite:       file:data/croupier.db  (defaults to data/croupier.db if empty)
  log: { level: debug, format: console }
  metrics:
    per_function: true
    per_game_denies: false
  profiles:
    prod:
      log: { level: info, format: json, file: logs/server.log }
      metrics: { per_function: true }
```

## Object Storage (uploads)
```yaml
server:
  storage:
    driver: s3     # s3 | cos | oss | file
    bucket: my-bucket
    region: ap-shanghai
    endpoint: https://cos.ap-shanghai.myqcloud.com   # s3/minio/cos endpoint (optional)
    access_key: {% raw %}${STORAGE_AK}{% endraw %}
    secret_key: {% raw %}${STORAGE_SK}{% endraw %}
    force_path_style: true
    signed_url_ttl: 15m
    # dev local:
    # driver: file
    # base_dir: data/uploads
```

### Notes:
- s3 覆盖 AWS/MinIO/腾讯 COS（S3 兼容模式）。COS 建议设置 `force_path_style=true`，并指定正确的 `region` 与 `endpoint`。
- 腾讯 COS 也提供官方 SDK 驱动（`driver: cos`），在 S3 兼容遇到边角不兼容时使用。
- 阿里云 OSS 使用官方 SDK 驱动（`driver: oss`）；Go Cloud 无原生 OSS 驱动。
- file 驱动仅用于本地开发，静态路径 `/uploads/` 会映射到 `base_dir`。

## Tencent COS（两种方式）
```yaml
server:
  storage:
    driver: s3  # 方式一：S3 兼容
    bucket: your-bucket
    region: ap-shanghai
    endpoint: https://cos.ap-shanghai.myqcloud.com
    access_key: {% raw %}${TENCENT_SECRET_ID}{% endraw %}
    secret_key: {% raw %}${TENCENT_SECRET_KEY}{% endraw %}
    force_path_style: true
    signed_url_ttl: 15m

# 或者使用官方 SDK 驱动：
server:
  storage:
    driver: cos  # 方式二：官方 SDK
    bucket: your-bucket-APPID
    region: ap-shanghai
    # endpoint 可选： https://cos.ap-shanghai.myqcloud.com
    access_key: {% raw %}${TENCENT_SECRET_ID}{% endraw %}
    secret_key: {% raw %}${TENCENT_SECRET_KEY}{% endraw %}
    signed_url_ttl: 15m
```

### 说明：
- 使用 `force_path_style: true` 避免虚拟主机名路由导致的兼容问题。
- `region` 需与 COS 控制台一致，否则签名可能失败。
- 如果使用 MinIO，请将 `endpoint` 指向 MinIO 地址（如 `http://minio:9000`），并保留 `force_path_style: true`。

## Start with overlay files and profile:
```bash
./croupier server \
  --config configs/server.example.yaml \
  --config-include configs/overrides.yaml \
  --profile prod
```

## Environment overrides
- Server: CROUPIER_SERVER_ADDR, CROUPIER_SERVER_HTTP_ADDR, CROUPIER_SERVER_LOG_LEVEL, ...
- DB selection (server): DB_DRIVER=postgres|mysql|sqlite|auto, DATABASE_URL=<dsn>  (derived automatically from server.db.* when YAML is present)
- Agent:  CROUPIER_AGENT_SERVER_ADDR, CROUPIER_AGENT_LOCAL_ADDR, ...

## Metrics env toggles (server)
- METRICS_PER_FUNCTION=true|false to enable per-function latency histogram and counters.
- METRICS_PER_GAME_DENIES=true|false to enable per-game RBAC deny counters.

## Agent Assignments & Downlink (dev)
```yaml
agent:
  assignments_api: http://localhost:8080   # poll assignments and pack export from this server
  assignments_poll_sec: 30                 # polling interval seconds
  downlink_dir: ./packs/downlink           # save/export current pack here on updates
  # optional adapter process demo (dev-only)
  adapter_prom_cmd: "go run ./tools/adapters/prom"
  adapter_http_cmd: "go run ./tools/adapters/http"
```

## Adapter supervisor (dev)
- Agent will supervise optional adapters with graceful restart and backoff.
- Environment passed to adapter process includes: `CROUPIER_AGENT_ID`, `CROUPIER_GAME_ID`, `CROUPIER_ENV`, and passthrough `PROM_URL`/`ASSIGNMENTS_API` if present.
- Desired adapters are inferred from assignments: `prom.*` → prom adapter, `http.*|grafana.*|alertmanager.*` → http adapter. Empty assignments means allow all → start both if configured.
- After downlink import/reload, Agent polls `/api/packs/list` briefly to verify server responds.

## Adapter health & logs (dev)
- Health (optional): set `adapter_prom_health_url` / `adapter_http_health_url` to an HTTP endpoint that returns 2xx when healthy; tune `adapter_health_interval_sec`.
- Logs: set `adapter_log_dir` (default `logs/`), `adapter_log_max_mb`, and `adapter_log_backups` for size-based rotation of stdout/stderr per adapter.
- Metrics: `/metrics.prom` exposes `croupier_adapter_running{adapter}`, `croupier_adapter_restarts_total{adapter}`, `croupier_adapter_healthy{adapter}`, `croupier_adapter_last_health_ts{adapter}`, `croupier_adapter_last_start_ts{adapter}`, `croupier_adapter_health_failures_total{adapter}`.
- Optional auto-restart: set `adapter_health_restart_threshold`>0 to restart adapter after N consecutive failed health checks (dev only, default disabled).

## Packs endpoints & ETag
- GET `/api/packs/list` returns `{ manifest, counts, etag }` where `etag` is a content hash of the current pack (manifest/descriptors/ui/web-plugin/js/root *.pb).
- GET `/api/packs/export` streams a tar.gz of the current pack and sets `ETag` header to the same value. Set `PACKS_EXPORT_REQUIRE_AUTH=true` to require JWT + RBAC (`packs:export`) for this endpoint (default open for Agent downlink demo).
- POST `/api/packs/import` (RBAC: `packs:import`) imports a tar.gz and reloads descriptors/FDS.
- POST `/api/packs/reload` (RBAC: `packs:reload`) rescans the pack directory.
- Agent uses the `ETag` from export to confirm readiness via `/api/packs/list`.

## Registry API RBAC
- GET `/api/registry` requires `registry:read` permission; UI 页面会依据角色隐藏或禁用受限操作（后端仍强校验）。

## Audit API RBAC
- GET `/api/audit` requires `audit:read` permission; 支持 `game_id`、`env`、`actor`、`kind`、`limit` 过滤；可选 `offset` 或 `page`+`size` 分页（最新在前）。UI 支持自动刷新、导出 CSV。

## Assignments audit
- POST `/api/assignments` 会写入审计事件（kind=`assignments.update`，target=`<game>|<env>`，meta 包含 `functions` 和 `unknown`）。可通过 `/api/audit?kind=assignments.update` 查看。

## Effective config snapshot
- Validate and print merged config (strict):
```bash
./croupier config test --config configs/server.example.yaml --section server --profile prod
```

## Notes
- Flags always win; prefer YAML + env for deploy, flags for local dev tweaks.
- The server binary reads `server.*` section. In CLI mode (`croupier server`), the same section applies.
- You can keep secrets (JWT, TLS paths) in environment or external secret managers; YAML supports file paths, not secret storage.