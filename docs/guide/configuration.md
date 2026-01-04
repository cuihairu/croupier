---
title: 配置管理
icon: gears
order: 3
category:
  - 入门指南
tag:
  - 配置
  - YAML
---

# 配置管理

Croupier 使用 YAML 配置文件管理系统行为。本文档详细说明配置选项、优先级和最佳实践。

## 目录

[[toc]]

## 配置优先级

配置加载顺序（低 → 高）：

1. **YAML 文件** - 基础配置
2. **YAML includes** - 包含的配置文件
3. **YAML profiles** - 环境配置
4. **环境变量** - 运行时覆盖
5. **命令行参数** - 最高优先级

### 环境变量语法

- 环境变量前缀：`CROUPIER_SERVER_*`、`CROUPIER_AGENT_*`、`CROUPIER_EDGE_*`
- 点号和连字符转换为下划线
- 示例：`CROUPIER_SERVER_ADDR`、`CROUPIER_SERVER_HTTP_ADDR`

## Server 配置

### 完整配置示例

```yaml
# server.yaml
server:
  # gRPC 监听地址
  addr: ":8443"

  # HTTP REST API 监听地址
  http_addr: ":8080"

  # TLS 配置
  tls:
    enabled: true
    cert_file: "data/server.crt"
    key_file: "data/server.key"
    ca_file: "data/ca.crt"  # 用于验证客户端证书
    min_version: "TLS1.2"
    max_version: "TLS1.3"

  # 数据库配置
  db:
    driver: auto  # auto | postgres | mysql | sqlite
    datasource: ""  # DSN/URL
    # Postgres: postgres://user:pass@host:5432/croupier?sslmode=disable
    # MySQL: mysql://user:pass@host:3306/croupier?charset=utf8mb4
    # SQLite: file:data/croupier.db

  # 对象存储配置
  storage:
    driver: s3  # s3 | cos | oss | file
    bucket: "my-bucket"
    region: "ap-shanghai"
    endpoint: "https://cos.ap-shanghai.myqcloud.com"
    access_key: "${STORAGE_AK}"
    secret_key: "${STORAGE_SK}"
    force_path_style: true
    signed_url_ttl: "15m"

  # 日志配置
  log:
    level: "info"  # debug | info | warn | error
    format: "console"  # console | json
    file: ""  # 日志文件路径
    max_size: 100  # MB
    max_backups: 3
    max_age: 7  # days

  # 指标配置
  metrics:
    per_function: true
    per_game_denies: false
    enable_prometheus: true
    prometheus_addr: ":9090"

  # 审计配置
  audit:
    enabled: true
    sensitive_fields:
      - "password"
      - "token"
      - "secret"

  # 认证配置
  auth:
    jwt_secret: "${JWT_SECRET}"
    jwt_expiry: "24h"
    oidc:
      enabled: false
      issuer: "https://accounts.example.com"
      client_id: "${OIDC_CLIENT_ID}"
      client_secret: "${OIDC_CLIENT_SECRET}"

  # 环境配置（profiles）
  profiles:
    dev:
      log:
        level: "debug"
      db:
        driver: "sqlite"
        datasource: "file:data/dev.db"
    prod:
      log:
        level: "info"
        format: "json"
        file: "logs/server.log"
```

### 环境变量覆盖

```bash
# 覆盖数据库配置
export DB_DRIVER=postgres
export DATABASE_URL="postgres://user:pass@localhost:5432/croupier?sslmode=disable"

# 覆盖监听地址
export CROUPIER_SERVER_ADDR=":9443"
export CROUPIER_SERVER_HTTP_ADDR=":9080"

# 覆盖日志级别
export CROUPIER_SERVER_LOG_LEVEL="debug"
```

## Agent 配置

### 完整配置示例

```yaml
# agent.yaml
agent:
  # Server 连接配置
  server_addr: "localhost:8443"
  server_name: "croupier.server"

  # 本地监听地址
  local_addr: ":19090"

  # 游戏标识
  game_id: "my-game"
  env: "dev"  # dev | staging | prod

  # TLS 配置
  tls:
    ca_file: "data/ca.crt"
    cert_file: "data/agent.crt"
    key_file: "data/agent.key"
    server_name: "croupier.server"

  # 心跳配置
  heartbeat_interval: "30s"
  heartbeat_timeout: "5m"

  # 分配配置
  assignments_api: "http://localhost:8080"
  assignments_poll_sec: 30
  downlink_dir: "./packs/downlink"

  # 适配器配置（开发用）
  adapter_prom_cmd: "go run ./tools/adapters/prom"
  adapter_http_cmd: "go run ./tools/adapters/http"
  adapter_prom_health_url: "http://localhost:9091/-/healthy"
  adapter_http_health_url: "http://localhost:9092/-/healthy"
  adapter_health_interval_sec: 30
  adapter_log_dir: "logs"
  adapter_log_max_mb: 100
  adapter_log_backups: 3

  # 日志配置
  log:
    level: "info"
    format: "console"
```

## Edge 配置

### 完整配置示例

```yaml
# edge.yaml
edge:
  # 监听地址
  addr: ":8443"

  # Server 连接配置
  server_addr: "internal.server:8443"

  # TLS 配置
  tls:
    cert_file: "data/edge.crt"
    key_file: "data/edge.key"
    ca_file: "data/ca.crt"
    server_name: "croupier.server"

  # 隧道配置
  tunnel:
    max_connections: 100
    idle_timeout: "5m"
    keepalive_interval: "30s"

  # 日志配置
  log:
    level: "info"
    format: "console"
```

## 配置验证

### 验证配置文件

```bash
# 使用 CLI 验证
./bin/croupier-server config test --config configs/server.yaml

# 输出示例
# ✓ Configuration is valid
# - server.addr: :8443
# - server.http_addr: :8080
# - server.db.driver: postgres
```

### 常见配置错误

| 错误 | 原因 | 解决方法 |
|------|------|----------|
| `invalid address` | 端口格式错误 | 使用 `:port` 或 `host:port` 格式 |
| `certificate not found` | 证书文件路径错误 | 检查证书文件是否存在 |
| `database connection failed` | DSN 格式错误 | 检查数据库连接字符串格式 |
| `permission denied` | 文件权限不足 | 检查证书和密钥文件权限 |

## 敏感信息处理

### 使用环境变量

```yaml
# 不推荐：直接写入配置
storage:
  access_key: "AKIDxxxxxxxx"
  secret_key: "xxxxxxxxxxxx"

# 推荐：使用环境变量
storage:
  access_key: "${STORAGE_AK}"
  secret_key: "${STORAGE_SK}"
```

### 环境变量展开

支持以下展开语法：

- `${VAR}` - 简单展开
- `${VAR:-default}` - 带默认值
- `${VAR:+replacement}` - 如果设置了则替换

## Profiles 使用

### 激活 Profile

```bash
# 使用 --profile 参数
./bin/croupier-server --config configs/server.yaml --profile prod

# 或使用环境变量
export CROUPIER_SERVER_PROFILE=prod
./bin/croupier-server --config configs/server.yaml
```

### Profile 配置示例

```yaml
server:
  log:
    level: "info"
  profiles:
    dev:
      log:
        level: "debug"
      db:
        driver: "sqlite"
    staging:
      log:
        level: "info"
      db:
        driver: "postgres"
    prod:
      log:
        level: "warn"
        format: "json"
      db:
        driver: "postgres"
```

## 对象存储配置

### S3 兼容存储

```yaml
server:
  storage:
    driver: s3
    bucket: "my-bucket"
    region: "us-east-1"
    endpoint: "https://s3.amazonaws.com"
    access_key: "${AWS_ACCESS_KEY_ID}"
    secret_key: "${AWS_SECRET_ACCESS_KEY}"
```

### MinIO

```yaml
server:
  storage:
    driver: s3
    bucket: "croupier"
    endpoint: "http://minio:9000"
    access_key: "${MINIO_ROOT_USER}"
    secret_key: "${MINIO_ROOT_PASSWORD}"
    force_path_style: true
```

### 腾讯云 COS

```yaml
server:
  storage:
    driver: s3  # 或 cos
    bucket: "bucket-APPID"
    region: "ap-shanghai"
    endpoint: "https://cos.ap-shanghai.myqcloud.com"
    access_key: "${TENCENT_SECRET_ID}"
    secret_key: "${TENCENT_SECRET_KEY}"
    force_path_style: true
```

### 本地文件存储

```yaml
server:
  storage:
    driver: file
    base_dir: "data/uploads"
```

## 最佳实践

### 1. 分离环境配置

```
configs/
├── base.yaml          # 基础配置
├── dev.yaml           # 开发环境
├── staging.yaml       # 预发布环境
└── prod.yaml          # 生产环境
```

### 2. 使用环境变量管理敏感信息

```bash
# .env.example
JWT_SECRET=your-jwt-secret-here
DATABASE_URL=postgres://...
STORAGE_AK=your-access-key
STORAGE_SK=your-secret-key
```

### 3. 配置文件模板

```yaml
# server.example.yaml
server:
  addr: ":8443"
  http_addr: ":8080"
  tls:
    cert_file: "data/server.crt"
    key_file: "data/server.key"
  db:
    driver: "postgres"
    datasource: "${DATABASE_URL}"
```

### 4. 配置验证

```bash
# CI/CD 中验证配置
./bin/croupier-server config test --config configs/server.prod.yaml
```

## 下一步

- [部署指南](./deployment.md) - 生产环境部署
- [安全配置](./operations/security.md) - 安全相关配置
- [运维指南](./operations/monitoring.md) - 监控和日志配置
