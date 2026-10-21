---
title: Configuration Management
icon: gears
order: 10
category:
  - Operations Manual
tag:
  - Configuration
---

# Configuration Management

Croupier currently uses YAML configuration files to drive runtime behavior. This document only describes configuration entry points that are still in effect in the repository.

## Configuration Precedence

YAML is the base; CLI flags and environment variables override per field (CLI > env > YAML).

The legacy file layout (`services/*/etc`, `includes`) has been removed; the `profiles` YAML key still parses and takes effect (the meta API exposes the current profile); `configs/*.yaml` is the single source of truth.

## Server Configuration

The most important configuration groups in `configs/server.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 18780
  timeout: 600000
  mode: dev

database:
  driver: mysql
  dataSource: "${DATABASE_URL}"

control:
  addr: ":19090"

auth:
  jwtSecret: "${JWT_SECRET}"
  rbacConfig: "configs/rbac.json"

storage:
  driver: file
  baseDir: "data/uploads"

log:
  level: info
  format: console
```

Per-database `dataSource` (DSN) examples — `croupier_meta` is the metadata database; with per-game database splitting, the platform creates game databases automatically:

| Key                     | Default | Description                                                                                                                                                                     |
| ----------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `database.multiGame`    | `false` | Single-database mode (default): all tables in one database with row-level isolation via the `game_id` column; when `true`, each `(game_id, env)` gets its own physical database |
| `database.gameDbPrefix` | `game_` | Physical database name prefix in multiGame mode (final name `game_<gameID>_<env>`)                                                                                              |

```yaml
# SQLite (development/testing)
database:
  driver: sqlite
  dataSource: "data/croupier.db"

# MySQL (recommended for production)
database:
  driver: mysql
  dataSource: "user:pass@tcp(host:3306)/croupier_meta?charset=utf8mb4&parseTime=True"

# PostgreSQL
database:
  driver: postgres
  dataSource: "host=localhost port=5432 user=postgres password=pass dbname=croupier_meta sslmode=disable"
```

### Common Overrides

```bash
export DATABASE_URL="postgres://user:***@localhost:5432/croupier?sslmode=disable"
export JWT_SECRET="change-me"
./bin/croupier-server --config configs/server.yaml --port 18780
```

## Agent Configuration

`configs/agent.yaml` currently corresponds to the Agent startup configuration:

```yaml
name: croupier-agent
host: 0.0.0.0
port: 19091

server:
  addr: "server:19090"
  insecure: false

agent:
  id: ""
  gameId: "demo" # Required; startup fails if left empty
  env: "prod" # Required; startup fails if left empty
  localAddr: "agent:19091"
  httpAddr: "agent:19091"

upstream:
  heartbeatInterval: 30
  retryInterval: 5
  maxRetries: 3
  timeout: 10000

outboundTLS:
  enabled: false
```

## Policy Configuration

`configs/default-policies.yaml` defines default policies for functions of different risk levels:

```yaml
low:
  require_approval: false
  require_audit: false
  allowed_roles:
    - user
    - operator

medium:
  require_approval: false
  require_audit: true
  allowed_roles:
    - operator

high:
  require_approval: true
  approval_workflow: single_admin
  require_audit: true
  allowed_roles:
    - admin

danger:
  require_approval: true
  approval_workflow: two_person
  require_audit: true
  allowed_roles:
    - super_admin
```

**Field descriptions:**

| Field               | Description                                      |
| ------------------- | ------------------------------------------------ |
| `require_approval`  | Whether approval is required                     |
| `approval_workflow` | Approval workflow type (single_admin/two_person) |
| `require_audit`     | Whether auditing is required                     |
| `allowed_roles`     | List of roles allowed to invoke                  |

**Two-layer policy architecture:**

1. **Default policies**: from `default-policies.yaml`, applied automatically based on the function's risk level
2. **Override policies**: stored in the database; override rules can be set for specific functions via the API

**Reloading configuration:**

```bash
# Via API
POST /api/v1/policies/reload

# Restarting the service loads automatically
./bin/croupier-server --config configs/server.yaml
```

## Configuration Validation

```bash
./bin/croupier-server --config configs/server.yaml validate
```

Common errors:

| Error                        | Cause                         | Solution                                                       |
| ---------------------------- | ----------------------------- | -------------------------------------------------------------- |
| `invalid address`            | Malformed address             | Check `Control.Addr`, `Server.Addr`                            |
| `certificate not found`      | Wrong certificate file path   | Check that the certificate files exist                         |
| `database connection failed` | DSN error                     | Check the database connection string                           |
| `permission denied`          | Insufficient file permissions | Check permissions on config, certificate, and data directories |

## Best Practices

### Use Environment Variables for Sensitive Information

```bash
DATABASE_URL=postgres://...              # Overrides database.dataSource (internal/svc/db.go)
STORAGE_ACCESS_KEY=your-access-key       # Object storage AK (internal/platform/objstore/store.go)
STORAGE_SECRET_KEY=your-secret-key       # Object storage SK (same as above)
```

The JWT secret does not use an environment variable; use the config key `auth.jwtSecret` (or `auth.jwtSecretFile`).

### Vault

| Key                                   | Default                     | Description                                                                                                     |
| ------------------------------------- | --------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `vault.enabled`                       | `false`                     | Off = vault not wired up (ctx.Vault is nil)                                                                     |
| `vault.provider`                      | `local`                     | Provider registry key: `local` \| `env`                                                                         |
| `vault.sessionTtlSeconds`             | 900                         | Unlock session idle timeout (local; sliding renewal)                                                            |
| `vault.providers.local.path`          | `data/vault.json`           | Encrypted storage file path (directory 0700, file 0600)                                                         |
| `vault.providers.local.passphraseEnv` | `CROUPIER_VAULT_PASSPHRASE` | Name of the environment variable holding the master passphrase (passphrase never lands in config files or logs) |
| `vault.providers.env.prefix`          | `CROUPIER_VAULT_ENV_`       | Environment variable name prefix (credential path `smtp/password` → `CROUPIER_VAULT_ENV_SMTP_PASSWORD`)         |

### Keep a Copy for Local Debugging

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.yaml configs/agent.local.yaml
```

### Validate After Changing

```bash
./bin/croupier-server --config configs/server.local.yaml validate
```

## Configuration Domains Not Covered Here

This page covers the commonly used keys. The following effective configuration domains are not expanded on this page; for the full key table see `internal/config/config.go` (the Config struct's yaml tags are the single source of truth):

- `auth.providers` (LDAP / OIDC / GitHub / WeChat / genericOAuth / register), `auth.loginLockout`
- `executionLog`, `taskLog`, `approval`, `cluster`, `registry.sdkVersionMinimums`
- `telemetry`, `sse`, `metrics`, `cache`, `pages`, `openapi`, `featureFlags`
- `herald`, `alertInbound`, `externalAPI`, `serverStatus`

## Next Steps

- [Deployment Guide (Chinese only)](/zh/operations/deploy-docker)
- [Development Documentation](/archive/legacy-en/development/index)
