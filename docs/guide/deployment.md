---
title: 部署指南
icon: rocket
order: 4
category:
  - 入门指南
tag:
  - 部署
---

# 部署指南

本文档只保留当前仓库已有产物对应的部署方式：Docker 镜像、`docker compose` 和二进制部署。

## 推荐组件

- Server
- Agent
- MySQL 或 PostgreSQL
- Redis
- 可选：analytics-worker、ingest、ClickHouse

## Docker 部署

### 直接使用 compose

```bash
docker compose up -d
```

当前镜像构建入口：

- `docker/Dockerfile.server` -> `./cmd/server`
- `docker/Dockerfile.agent` -> `./cmd/agent`
- `docker/Dockerfile.analytics-worker` -> `./cmd/analytics-worker`
- `docker/Dockerfile.ingest` -> `./cmd/ingest`

### 单独构建镜像

```bash
docker build -t croupier-server:latest -f docker/Dockerfile.server .
docker build -t croupier-agent:latest -f docker/Dockerfile.agent .
```

## 二进制部署

### Server

```bash
make server
./bin/croupier-server --config configs/server.yaml
```

### Agent

```bash
make agent
./bin/croupier-agent --config configs/agent.yaml
```

### Analytics 链路

```bash
make worker
make ingest
./bin/analytics-worker
ANALYTICS_INGEST_SECRET=dev-secret ./bin/ingest --addr :8088 --secret dev-secret
```

## 服务安装脚本

仓库已提供：

- `scripts/install-systemd.sh`
- `scripts/install-launchd.sh`
- `scripts/install-windows-service.ps1`

## 健康检查

```bash
curl http://localhost:18780/api/v1/monitoring/health
curl http://localhost:8088/healthz
```

## 部署建议

1. 数据库和 Redis 使用外部托管实例，不与应用容器共存储。
2. 生产环境不要直接使用仓库默认的 `configs/*.yaml`，应复制后按环境落地。
3. 通过环境变量注入 `DATABASE_URL`、`JWT_SECRET`、对象存储密钥等敏感项。
4. Server 对外暴露 HTTP 入口，Agent 走内网或专线接入。

## 下一步

- [监控指南](./operations/monitoring.md)
- [安全配置](./operations/security.md)
- [故障排查](./operations/troubleshooting.md)
