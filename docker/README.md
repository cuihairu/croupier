# Docker 容器化部署

本目录包含 Croupier 各服务的 Docker 构建与编排文件。

## 目录结构

```text
docker/
├── README.md
├── docker-compose.yml
├── docker-compose.telemetry.yaml
├── Dockerfile.server
├── Dockerfile.agent
├── Dockerfile.web
├── Dockerfile.analytics-worker
├── Dockerfile.ingest
└── Dockerfile.demo
```

## 当前端口语义

当前推荐按“统一 session + 本地 gateway”理解端口，而不是历史 `gRPC` / `NNG` 命名：

| 端口 | 含义 |
| --- | --- |
| `18780` | Server REST API |
| `19090` | Server session/control 入口，供 Agent 主动连接 |
| `19091` | Agent 本地 gateway，供 SDK / GameServer / 第三方本地程序接入 |
| `8000` | Dashboard |
| `18081` | Analytics Ingestion |

说明：

- Docker 编排中仍可能保留部分历史兼容端口映射
- 但新的架构文档与接入文档，应统一以上述 session 语义为准

## 快速开始

### 启动核心服务

```bash
cd docker
docker-compose up -d
```

默认会启动：

- PostgreSQL
- Redis
- ClickHouse
- Server
- Agent
- Dashboard
- Analytics Ingestion

### 启动遥测监控栈

```bash
cd docker
docker-compose -f docker-compose.telemetry.yaml up -d
```

## 服务访问

| 服务 | 地址 | 说明 |
| --- | --- | --- |
| Dashboard | http://localhost:8000 | 控制台 |
| Server REST API | http://localhost:18780 | 管理与查询接口 |
| Server Session Control | localhost:19090 | Agent 上行 session 入口 |
| Agent Local Gateway | localhost:19091 | SDK / GameServer 本地接入 |
| Analytics Ingestion | http://localhost:18081 | 公网/DMZ 摄取入口 |

## 数据库

| 服务 | 地址 |
| --- | --- |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |
| ClickHouse | localhost:8123 / 9000 |

## 常用命令

```bash
# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f [service_name]

# 停止
docker-compose down

# 停止并删除卷
docker-compose down -v

# 重启单服务
docker-compose restart server

# 仅启动单服务
docker-compose up -d server

# 重新构建
docker-compose build [service_name]
```

## 开发建议

- 只需要数据库时，可先单独拉起 `postgres`、`redis`、`clickhouse`
- 开发主链路时，重点关注 `18780`、`19090`、`19091`
- SDK 接入时，应把目标地址指向 Agent 本地 gateway，而不是 Server

## 生产建议

1. 修改默认密码与默认密钥
2. `Agent <-> Server` 启用 TLS / mTLS
3. 对 `19090`、`19091` 做明确网络边界控制
4. Dashboard 与 REST API 走 HTTPS 与统一鉴权
5. 为证书、JWT、数据库凭据接入 Secret Manager
