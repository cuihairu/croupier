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

Effective config snapshot
- Validate and print merged config (strict):
```bash
./croupier config test --config configs/server.example.yaml --section server --profile prod
```

Notes
- Flags always win; prefer YAML + env for deploy, flags for local dev tweaks.
- You can keep secrets (JWT, TLS paths) in environment or external secret managers; YAML supports file paths, not secret storage.
