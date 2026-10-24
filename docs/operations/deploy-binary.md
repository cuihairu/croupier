---
title: Binary Deployment (systemd)
icon: server
order: 4
category:
  - Operations Manual
tag:
  - Deployment
---

# Binary Deployment (systemd)

Suitable for bare-metal/VM environments without container constraints. Artifacts come from [Releases](https://github.com/cuihairu/croupier/releases) or a local build (`make build` → `bin/croupier-server`, `bin/croupier-agent`).

> If you only need the agent, use the one-click install script (auto-detects architecture, anonymous download, idempotent upgrades, optional service registration); this page covers the full manual deployment (directory layout, server configuration, production-grade systemd units).

## Prerequisites

- Linux x86_64, Go 1.27.3+ (only needed for local builds)
- MySQL 8.x / PostgreSQL 16 (production) — SQLite is only suitable for single-machine pilots
- Redis 7+ (required for multi-instance; a single instance can degrade to `cache.type: memory`)

## Directory Layout

```
/opt/croupier/
├── bin/            # croupier-server, croupier-agent
├── etc/            # server.yaml, agent.yaml, rbac.json, ...
├── data/           # assignments.json, uploaded files, and other runtime data
└── logs/           # (optional) file logs; by default stdout goes to journald
```

## Configuration

Start from the repository templates and adjust following the Full Server Configuration Reference:

```bash
cp configs/server.yaml /opt/croupier/etc/server.yaml
# Must change for production: auth.jwtSecret, database.dataSource, cache password, server.mode: prod
```

Configuration validation (without starting the service):

```bash
/opt/croupier/bin/croupier-server --config /opt/croupier/etc/server.yaml validate
```

## systemd Units

`/etc/systemd/system/croupier-server.service`:

```ini
[Unit]
Description=Croupier Server
After=network-online.target mysql.service redis.service
Wants=network-online.target

[Service]
Type=simple
User=croupier
Group=croupier
WorkingDirectory=/opt/croupier
ExecStart=/opt/croupier/bin/croupier-server --config /opt/croupier/etc/server.yaml
Restart=on-failure
RestartSec=5
# Recommended for sensitive values: write a $(VAR) placeholder in YAML, expanded from environment variables at load time
# (config_loader's os.ExpandEnv), so secrets never touch disk:
#   auth:
#     jwtSecret: "$(CROUPIER_JWT_SECRET)"
# Environment=CROUPIER_JWT_SECRET=xxx
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/croupier/data

[Install]
WantedBy=multi-user.target
```

`croupier-agent.service` has the same shape; the differences:

```ini
[Unit]
After=network-online.target
[Service]
ExecStart=/opt/croupier/bin/croupier-agent --config /opt/croupier/etc/agent.yaml
# The Agent has no local state directory requirements; the upstream address points at the Server entry point (LB or direct)
```

Start and enable at boot:

```bash
systemctl daemon-reload
systemctl enable --now croupier-server
systemctl enable --now croupier-agent
```

## Verification

```bash
systemctl status croupier-server
curl -fsS http://127.0.0.1:18780/healthz          # {"status":"ok"}
curl -fsS http://127.0.0.1:18780/api/v1           # version + features metadata
journalctl -u croupier-server -f                  # startup migration logs (goose catch-up)
```

Agent-side verification: a session registration appearing in the Server logs means it works; the Dashboard "Node Maintenance" page shows the agent online.

## Multi-Instance (Bare-Metal HA)

1. One `server.yaml` per Server instance, with `cluster.enabled: true` and `advertiseAddr` set to a **peer-reachable address**
2. Point the Agent `server.addr` at the LB entry point, not a specific instance

## Common Pitfalls

- **Slow first startup**: versioned migrations catch up at startup (a fresh database runs the baseline AutoMigrate), which can take tens of seconds with many tables — it is not a hang
- **`data/` permissions**: under `ProtectSystem=strict` an explicit `ReadWritePaths` is required, otherwise assignments/uploads fail to write
- **Time zone**: the MySQL DSN needs `loc=Local`, otherwise audit/task timestamps are shifted
