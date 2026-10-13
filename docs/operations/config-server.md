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

Croupier 当前使用 YAML 配置文件驱动运行时行为。本文档只描述仓库中仍然生效的配置入口。

## 配置优先级

YAML 为底，CLI 参数与环境变量按字段覆盖（CLI > env > YAML）。

旧文件布局（`services/*/etc`、`includes`）已移除；`profiles` YAML 键仍解析生效（meta API 暴露当前 profile），统一以 `configs/*.yaml` 为准。

## Server 配置

`configs/server.yaml` 中最关键的几组配置如下：

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

各库 `dataSource`（DSN）示例——`croupier_meta` 为元数据库，按游戏分库时由平台自动建游戏库：

| 键                      | 默认    | 说明                                                                                              |
| ----------------------- | ------- | ------------------------------------------------------------------------------------------------- |
| `database.multiGame`    | `false` | 单库模式（缺省）：所有表同库，`game_id` 列行级隔离；`true` 时每个 `(game_id, env)` 分配独立物理库 |
| `database.gameDbPrefix` | `game_` | multiGame 模式下物理库名前缀（最终名 `game_<gameID>_<env>`）                                      |

```yaml
# SQLite（开发/测试）
database:
  driver: sqlite
  dataSource: "data/croupier.db"

# MySQL（生产推荐）
database:
  driver: mysql
  dataSource: "user:pass@tcp(host:3306)/croupier_meta?charset=utf8mb4&parseTime=True"

# PostgreSQL
database:
  driver: postgres
  dataSource: "host=localhost port=5432 user=postgres password=pass dbname=croupier_meta sslmode=disable"
```

### 常见覆盖项

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/croupier?sslmode=disable"
export JWT_SECRET="change-me"
./bin/croupier-server --config configs/server.yaml --port 18780
```

## Agent 配置

`configs/agent.yaml` 当前对应的是 Agent 启动配置：

```yaml
name: croupier-agent
host: 0.0.0.0
port: 19091

server:
  addr: "server:19090"
  insecure: false

agent:
  id: ""
  gameId: "demo" # 必填，留空启动失败
  env: "prod" # 必填，留空启动失败
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

## 政策配置

`configs/default-policies.yaml` 定义了不同风险等级函数的默认政策：

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

| 字段                | 说明                                    |
| ------------------- | --------------------------------------- |
| `require_approval`  | 是否需要审批                            |
| `approval_workflow` | 审批流程类型（single_admin/two_person） |
| `require_audit`     | 是否需要审计                            |
| `allowed_roles`     | 允许调用的角色列表                      |

**双层政策架构：**

1. **默认政策**：来自 `default-policies.yaml`，根据函数风险等级自动应用
2. **覆盖政策**：存储在数据库中，可通过 API 为特定函数设置覆盖规则

**重新加载配置：**

```bash
# API 方式
POST /api/v1/policies/reload

# 重启服务自动加载
./bin/croupier-server --config configs/server.yaml
```

## 配置验证

```bash
./bin/croupier-server --config configs/server.yaml validate
```

常见错误：

| 错误                         | 原因             | 解决方法                           |
| ---------------------------- | ---------------- | ---------------------------------- |
| `invalid address`            | 地址格式错误     | 检查 `Control.Addr`、`Server.Addr` |
| `certificate not found`      | 证书文件路径错误 | 检查证书文件是否存在               |
| `database connection failed` | DSN 错误         | 检查数据库连接字符串               |
| `permission denied`          | 文件权限不足     | 检查配置、证书和数据目录权限       |

## 最佳实践

### 使用环境变量管理敏感信息

```bash
DATABASE_URL=postgres://...              # 覆盖 database.dataSource（internal/svc/db.go）
STORAGE_ACCESS_KEY=your-access-key       # 对象存储 AK（internal/platform/objstore/store.go）
STORAGE_SECRET_KEY=your-secret-key       # 对象存储 SK（同上）
```

JWT 密钥不走环境变量，用配置项 `auth.jwtSecret`（或 `auth.jwtSecretFile`）。

### 密码箱（vault）

| 键                                    | 默认                        | 说明                                                                            |
| ------------------------------------- | --------------------------- | ------------------------------------------------------------------------------- |
| `vault.enabled`                       | `false`                     | 关=密码箱不装配（ctx.Vault 为 nil）                                             |
| `vault.provider`                      | `local`                     | 提供商注册表键：`local` \| `env`                                                |
| `vault.sessionTtlSeconds`             | 900                         | 解锁会话空闲超时（local；滑动续期）                                             |
| `vault.providers.local.path`          | `data/vault.json`           | 加密存储文件路径（目录 0700、文件 0600）                                        |
| `vault.providers.local.passphraseEnv` | `CROUPIER_VAULT_PASSPHRASE` | 持主口令的环境变量名（口令不落配置文件、不落日志）                              |
| `vault.providers.env.prefix`          | `CROUPIER_VAULT_ENV_`       | 环境变量名前缀（凭据路径 `smtp/password` → `CROUPIER_VAULT_ENV_SMTP_PASSWORD`） |

### 为本地调试保留副本

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.yaml configs/agent.local.yaml
```

### 修改后做一次验证

```bash
./bin/croupier-server --config configs/server.local.yaml validate
```

## 未覆盖配置域

本页覆盖常用键。以下生效配置域未在此页展开，完整键表见 `internal/config/config.go`（Config 结构体 yaml tag 为唯一真相源）：

- `auth.providers`（LDAP / OIDC / GitHub / WeChat / genericOAuth / register）、`auth.loginLockout`
- `executionLog`、`taskLog`、`approval`、`cluster`、`registry.sdkVersionMinimums`
- `telemetry`、`sse`、`metrics`、`cache`、`pages`、`openapi`、`featureFlags`
- `herald`、`alertInbound`、`externalAPI`、`serverStatus`

## 下一步

- [部署指南](./deploy-docker.md)
- [开发文档](../development/)
