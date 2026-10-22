---
title: 配置管理
icon: gears
order: 10
category:
  - 运维手册
tag:
  - 配置
---

# 配置管理

Croupier 目前使用 YAML 配置文件驱动运行时行为。本文只描述仓库中仍然生效的配置入口。

## 配置优先级

YAML 为基底；CLI 标志与环境变量按字段覆盖（CLI > env > YAML）。

旧的文件布局（`services/*/etc`、`includes`）已移除；`profiles` YAML 键仍解析并生效（meta API 暴露当前 profile）；`configs/*.yaml` 是唯一事实源。

## Server 配置

`configs/server.yaml` 中最重要的配置组：

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

各数据库 `dataSource`（DSN）示例——`croupier_meta` 是元数据库；启用按游戏分库时平台自动创建游戏库：

| 键                      | 默认    | 说明                                                                                                 |
| ----------------------- | ------- | ---------------------------------------------------------------------------------------------------- |
| `database.multiGame`    | `false` | 单库模式（默认）：所有表在同一库，靠 `game_id` 列做行级隔离；`true` 时每 `(game_id, env)` 独立物理库 |
| `database.gameDbPrefix` | `game_` | multiGame 模式下物理库名前缀（最终名 `game_<gameID>_<env>`）                                         |

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

### 常见覆盖

```bash
export DATABASE_URL="postgres://user:***@localhost:5432/croupier?sslmode=disable"
export JWT_SECRET="change-me"
./bin/croupier-server --config configs/server.yaml --port 18780
```

## Agent 配置

`configs/agent.yaml` 目前对应 Agent 启动配置：

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

## 策略配置

`configs/default-policies.yaml` 定义不同风险级别函数的默认策略：

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

**字段说明：**

| 字段                | 说明                                  |
| ------------------- | ------------------------------------- |
| `require_approval`  | 是否需要审批                          |
| `approval_workflow` | 审批流类型（single_admin/two_person） |
| `require_audit`     | 是否需要审计                          |
| `allowed_roles`     | 允许调用的角色列表                    |

**两层策略架构：**

1. **默认策略**：来自 `default-policies.yaml`，按函数风险级别自动套用
2. **覆盖策略**：存数据库；可通过 API 为特定函数设置覆盖规则

**重载配置：**

```bash
# Via API
POST /api/v1/policies/reload

# Restarting the service loads automatically
./bin/croupier-server --config configs/server.yaml
```

## 配置校验

```bash
./bin/croupier-server --config configs/server.yaml validate
```

常见错误：

| 错误                         | 原因             | 处理                               |
| ---------------------------- | ---------------- | ---------------------------------- |
| `invalid address`            | 地址格式错误     | 检查 `Control.Addr`、`Server.Addr` |
| `certificate not found`      | 证书文件路径错误 | 检查证书文件是否存在               |
| `database connection failed` | DSN 错误         | 检查数据库连接串                   |
| `permission denied`          | 文件权限不足     | 检查配置、证书、数据目录权限       |

## 最佳实践

### 敏感信息用环境变量

```bash
DATABASE_URL=postgres://...              # Overrides database.dataSource (internal/svc/db.go)
STORAGE_ACCESS_KEY=your-access-key       # Object storage AK (internal/platform/objstore/store.go)
STORAGE_SECRET_KEY=your-secret-key       # Object storage SK (same as above)
```

JWT secret 不走环境变量，用配置键 `auth.jwtSecret`（或 `auth.jwtSecretFile`）。

### Vault

| 键                                    | 默认                        | 说明                                                                            |
| ------------------------------------- | --------------------------- | ------------------------------------------------------------------------------- |
| `vault.enabled`                       | `false`                     | 关 = vault 未接线（ctx.Vault 为 nil）                                           |
| `vault.provider`                      | `local`                     | provider 注册键：`local` \| `env`                                               |
| `vault.sessionTtlSeconds`             | 900                         | 解锁会话空闲超时（local；滑动续期）                                             |
| `vault.providers.local.path`          | `data/vault.json`           | 加密存储文件路径（目录 0700，文件 0600）                                        |
| `vault.providers.local.passphraseEnv` | `CROUPIER_VAULT_PASSPHRASE` | 存放主口令的环境变量名（口令绝不落配置文件与日志）                              |
| `vault.providers.env.prefix`          | `CROUPIER_VAULT_ENV_`       | 环境变量名前缀（凭据路径 `smtp/password` → `CROUPIER_VAULT_ENV_SMTP_PASSWORD`） |

### 本地调试保留副本

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.yaml configs/agent.local.yaml
```

### 改完先校验

```bash
./bin/croupier-server --config configs/server.local.yaml validate
```

## 本文未覆盖的配置域

本页只列常用键。以下生效的配置域不在本页展开，全量键表以 `internal/config/config.go` 为准（Config 结构体的 yaml tag 是唯一事实源）：

- `auth.providers`（LDAP / OIDC / GitHub / WeChat / genericOAuth / register）、`auth.loginLockout`
- `executionLog`、`taskLog`、`approval`、`cluster`、`registry.sdkVersionMinimums`
- `telemetry`、`sse`、`metrics`、`cache`、`pages`、`openapi`、`featureFlags`
- `herald`、`alertInbound`、`externalAPI`、`serverStatus`

## 下一步
