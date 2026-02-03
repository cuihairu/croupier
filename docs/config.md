---
title: 配置管理
icon: gears
order: 4
category:
  - 入门指南
tag:
  - 配置
  - YAML
---

# Configuration (YAML, Includes, Profiles)

This repo contains multiple Go entrypoints. For go-zero services under `services/*` (including `services/server`), configuration is loaded via `github.com/zeromicro/go-zero/core/conf` from a YAML file passed by `-f/--config`.

Best practice in this repo:
- Put **non-secret defaults** in YAML (ports, feature toggles, relative paths).
- Put **secrets / per-environment DSNs** in environment variables and either:
  - set them directly (e.g. `DATABASE_URL=...`), or
  - reference them from YAML using `${VAR}` (env expansion is enabled for `services/server`).

Notes for `services/server`:
- DB config keys are `Database.Driver` and `Database.DataSource` in YAML.
- `DB_DRIVER` and `DATABASE_URL` (if set) override the YAML DB values at runtime.
- Relative paths like `data/...`, `configs/...`, `packs/...` are resolved from the process working directory; when developing locally, run with `cwd=server/` (see `server/.vscode/launch.json`).

`services/server` load order (low → high)
- YAML file: `-f services/server/etc/server.yaml`
- YAML `${VAR}` expansion (env expansion)
- Explicit env overrides: `DB_DRIVER`, `DATABASE_URL`
- Flags (e.g. `--port`, `--host`)

Legacy CLI precedence (low → high)
- Base YAML: `--config base.yaml`
- Include YAMLs: `--config-include a.yaml --config-include b.yaml` (later overrides earlier)
- Section select: `server:/agent:/edge:` (subtree of the merged YAML)
- Profile overlay: `--profile <name>` (applied from section.profiles.`<name>`)
- Environment: `CROUPIER_SERVER_* / CROUPIER_AGENT_* / CROUPIER_EDGE_*` (dots and dashes become underscores)
- Flags: highest precedence

Examples
```yaml
# server.example.yaml
# HTTP REST API (go-zero RestConf)
Name: croupier-api
Host: 0.0.0.0
Port: 18780
Timeout: 600000  # 10 minutes (in milliseconds) - for SSE and long-lived connections

# Database configuration
Database:
  Driver: auto      # postgres | mysql | sqlite | auto
  DataSource: ""    # DSN/URL. Examples:
  # Postgres: postgres://user:pass@host:5432/croupier?sslmode=disable
  # MySQL (URL): mysql://user:pass@host:3306/croupier?charset=utf8mb4
  # MySQL (DSN):  user:pass@tcp(host:3306)/croupier?parseTime=true&charset=utf8mb4
  # SQLite:       file:data/croupier.db  (defaults to data/croupier.db if empty)

# gRPC server configuration (control plane)
GRPC:
  Addr: ":19090"
  Cert: ""
  Key: ""
  CA: ""

CroupierLog:
  Level: debug
  Format: console

Metrics:
  PerFunction: true
  PerGameDenies: false

# SSE (Server-Sent Events) configuration
SSE:
  UpdateInterval: 2       # 消息更新间隔（秒），默认 2
  KeepAliveInterval: 30   # Keep-alive 间隔（秒），默认 30

Profiles:
  prod:
    log: { level: info, format: json, file: logs/server.log }
    metrics: { per_function: true }
```

Object Storage (uploads)
```yaml
Storage:
  Driver: s3     # s3 | cos | oss | obs | file
  Bucket: my-bucket
  Region: ap-shanghai
  Endpoint: https://cos.ap-shanghai.myqcloud.com   # s3/minio/cos endpoint (optional)
  AccessKey: ${STORAGE_AK}
  SecretKey: ${STORAGE_SK}
  ForcePathStyle: true
  SignedURLTTL: 15m
  # dev local:
  # Driver: file
  # BaseDir: data/uploads
```

Notes:
- **s3**: AWS S3 / MinIO / 腾讯 COS（S3 兼容模式）。COS 建议设置 `ForcePathStyle=true`，并指定正确的 `Region` 与 `Endpoint`。
- **cos**: 腾讯云 COS 官方 SDK 驱动，在 S3 兼容遇到边角不兼容时使用。需要 `Region` 或 `Endpoint`。
- **oss**: 阿里云 OSS 官方 SDK 驱动。需要 `Endpoint`、`AccessKey`、`SecretKey`。
- **obs**: 华为云 OBS 官方 SDK 驱动。需要 `Endpoint`、`AccessKey`、`SecretKey`。
- **file**: 本地文件系统，仅用于本地开发，静态路径 `/uploads/` 会映射到 `BaseDir`。

Tencent COS（两种方式）
```yaml
Storage:
  Driver: s3  # 方式一：S3 兼容
  Bucket: your-bucket
  Region: ap-shanghai
  Endpoint: https://cos.ap-shanghai.myqcloud.com
  AccessKey: ${TENCENT_SECRET_ID}
  SecretKey: ${TENCENT_SECRET_KEY}
  ForcePathStyle: true
  SignedURLTTL: 15m

# 或者使用官方 SDK 驱动：
Storage:
  Driver: cos  # 方式二：官方 SDK
  Bucket: your-bucket-APPID
  Region: ap-shanghai
  # Endpoint 可选： https://cos.ap-shanghai.myqcloud.com
  AccessKey: ${TENCENT_SECRET_ID}
  SecretKey: ${TENCENT_SECRET_KEY}
  SignedURLTTL: 15m
```
说明：
- 使用 `ForcePathStyle: true` 避免虚拟主机名路由导致的兼容问题。
- `Region` 需与 COS 控制台一致，否则签名可能失败。
- 如果使用 MinIO，请将 `Endpoint` 指向 MinIO 地址（如 `http://minio:9000`），并保留 `ForcePathStyle: true`。
```

华为云 OBS
```yaml
Storage:
  Driver: obs
  Bucket: your-bucket
  Endpoint: https://obs.cn-north-1.myhuaweicloud.com
  AccessKey: ${HUAWEI_ACCESS_KEY}
  SecretKey: ${HUAWEI_SECRET_KEY}
  SignedURLTTL: 15m
```

`services/server` quickstart:
```bash
cd server

# SQLite (default)
go run ./services/server -f services/server/etc/server.yaml

# Postgres
DB_DRIVER=postgres DATABASE_URL="postgres://croupier:croupier_dev_password@localhost:5432/croupier?sslmode=disable" \
  go run ./services/server -f services/server/etc/server.yaml
```

Environment overrides (`services/server`)
- `DB_DRIVER`: `postgres|mysql|sqlite|sqlserver|auto` (default `auto`)
- `DATABASE_URL`: DSN/URL, e.g. `postgres://...` or `file:data/croupier.db?...`

Environment overrides (legacy CLI)
- Server: `CROUPIER_SERVER_ADDR`, `CROUPIER_SERVER_HTTP_ADDR`, `CROUPIER_SERVER_LOG_LEVEL`, ...
- Agent:  `CROUPIER_AGENT_SERVER_ADDR`, `CROUPIER_AGENT_LOCAL_ADDR`, ...

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
  adapter_prom_cmd: "go run ./tools/adapters/prom"
  adapter_http_cmd: "go run ./tools/adapters/http"
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
- Validate configuration (go-zero services):
```bash
# Validate server config
./bin/croupier-server -f services/server/etc/server.yaml --mode test

# Or use the built-in validate command
./bin/croupier-server -f services/server/etc/server.yaml validate
```

Notes
- Flags always win; prefer YAML + env for deploy, flags for local dev tweaks.
- The server binary reads `server.*` section. In CLI mode (`croupier server`), the same section applies.
- You can keep secrets (JWT, TLS paths) in environment or external secret managers; YAML supports file paths, not secret storage.

## SSE (Server-Sent Events) Configuration

SSE is used for real-time message streaming and requires special timeout configuration to prevent premature disconnection.

### Key Configuration Parameters

**Timeout** (go-zero RestConf)
- Purpose: HTTP request timeout in milliseconds
- Default: 3000ms (3 seconds) - **too short for SSE**
- Recommended: `600000` (10 minutes) for SSE endpoints
- Location: Top-level `Timeout` field (inherited from `rest.RestConf`)

**SSE.UpdateInterval**
- Purpose: Interval between message snapshot updates (push to client)
- Default: `2` seconds
- Unit: seconds
- Typical range: 1-10 seconds depending on real-time requirements

**SSE.KeepAliveInterval**
- Purpose: Interval between `: ping` keep-alive messages
- Default: `30` seconds
- Unit: seconds
- Recommended range: 20-60 seconds

### Critical Timeout Relationship

⚠️ **Important**: `Timeout` MUST be significantly larger than `SSE.KeepAliveInterval` to avoid `context deadline exceeded` errors.

**Minimum Safe Timeout:**
```
Timeout ≥ SSE.KeepAliveInterval × 3
```

**Example Calculation:**
- KeepAliveInterval: 30 seconds
- Minimum Timeout: 30 × 3 = 90 seconds
- Recommended Timeout: 600 seconds (10 minutes) for safety margin

### Automatic Timeout Validation

The server automatically validates the timeout relationship at startup and will:

1. **Check if** `Timeout < KeepAliveInterval × 3`
2. **Auto-adjust** the timeout to safe minimum if needed
3. **Display warning** if adjustment was made:
   ```
   ⚠️  警告: go-zero Timeout (3秒) 小于 SSE KeepAliveInterval (30秒) 的 3 倍
      自动调整 Timeout 为 90 秒以防止 SSE 连接过早断开
   ```
4. **Show configuration info** on successful validation:
   ```
   ✅ SSE 配置验证通过:
      - go-zero Timeout: 600 秒
      - SSE UpdateInterval: 2 秒
      - SSE KeepAliveInterval: 30 秒
      - 安全系数: 20.0x (超时 / KeepAlive)
   ```

### Configuration Example

```yaml
# services/server/etc/server.yaml
Name: croupier-api
Host: 0.0.0.0
Port: 18780
Timeout: 600000  # 10 minutes - MUST be > 3x KeepAliveInterval

SSE:
  UpdateInterval: 2       # Push messages every 2 seconds
  KeepAliveInterval: 30   # Send :ping every 30 seconds
```

### Troubleshooting

**Error: `context deadline exceeded` after 3 seconds**
- Cause: Default go-zero timeout (3000ms) is too short
- Fix: Set explicit `Timeout: 600000` in server.yaml

**Error: `superfluous response.WriteHeader call`**
- Cause: Database operations failing after headers written due to timeout
- Fix: Same as above - increase Timeout

**SSE connection drops intermittently**
- Check: Timeout ≥ 3 × KeepAliveInterval
- Verify: No intermediate proxies with shorter timeouts (nginx, load balancers)
- Monitor: Client-side reconnection logic

### Client-Side Considerations

When connecting to SSE endpoints:
1. **Set appropriate Accept header**: `Accept: text/event-stream`
2. **Handle reconnection**: Implement exponential backoff
3. **Monitor keep-alive**: Expect `: ping` messages every `KeepAliveInterval` seconds
4. **Parse event types**: Watch for `event: messages` and `event: unread`

Example endpoint: `GET /api/v1/messages/stream`
```javascript
const eventSource = new EventSource('/api/v1/messages/stream');
eventSource.addEventListener('messages', (e) => {
  const data = JSON.parse(e.data);
  // Handle message updates
});
eventSource.addEventListener('unread', (e) => {
  const { count } = JSON.parse(e.data);
  // Handle unread count
});
```
