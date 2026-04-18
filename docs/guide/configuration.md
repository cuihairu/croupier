---
title: 配置管理
icon: gears
order: 3
category:
  - 入门指南
tag:
  - 配置
---

# 配置管理

Croupier 当前使用 YAML 配置文件驱动运行时行为。本文档只描述仓库中仍然生效的配置入口。

## 配置优先级

1. YAML 文件
2. 环境变量
3. 命令行参数

当前仓库不再维护 `services/*/etc`、`includes`、`profiles` 这套旧布局，统一以 `configs/*.yaml` 为准。

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
  addr: "server:8443"
  insecure: false

agent:
  id: ""
  gameId: ""
  env: ""
  localAddr: "agent:19090"
  httpAddr: "agent:19091"

upstream:
  heartbeatInterval: 30
  retryInterval: 5
  maxRetries: 3
  timeout: 10000

tls:
  enabled: false
```

## 配置验证

```bash
./bin/croupier-server --config configs/server.yaml validate
```

常见错误：

| 错误 | 原因 | 解决方法 |
|------|------|----------|
| `invalid address` | 地址格式错误 | 检查 `Control.Addr`、`Server.Addr` |
| `certificate not found` | 证书文件路径错误 | 检查证书文件是否存在 |
| `database connection failed` | DSN 错误 | 检查数据库连接字符串 |
| `permission denied` | 文件权限不足 | 检查配置、证书和数据目录权限 |

## 最佳实践

### 使用环境变量管理敏感信息

```bash
JWT_SECRET=your-jwt-secret
DATABASE_URL=postgres://...
STORAGE_AK=your-access-key
STORAGE_SK=your-secret-key
```

### 为本地调试保留副本

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.yaml configs/agent.local.yaml
```

### 修改后做一次验证

```bash
./bin/croupier-server --config configs/server.local.yaml validate
```

## 下一步

- [部署指南](./deployment.md)
- [开发文档](../development/README.md)
