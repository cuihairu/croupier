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

Effective config snapshot
- Validate and print merged config (strict):
```bash
./croupier config test --config configs/server.example.yaml --section server --profile prod
```

Notes
- Flags always win; prefer YAML + env for deploy, flags for local dev tweaks.
- You can keep secrets (JWT, TLS paths) in environment or external secret managers; YAML supports file paths, not secret storage.
