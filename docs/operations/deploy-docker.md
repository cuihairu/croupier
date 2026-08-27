---
title: 部署指南
icon: rocket
order: 3
category:
  - 运维手册
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

### HA 双实例部署（docker-compose.deploy.yml）

生产编排 `docker/docker-compose.deploy.yml` 按 [Server 多实例 HA 架构](../architecture/server-ha-multi-instance.md) 部署双实例拓扑：

```
                          ┌─► croupier-server  ── HTTP API 18780 ─┐
浏览器 ──► dashboard nginx ┤                                       ├─► 共享 postgres/redis
（L7 分流）                └─► croupier-server2 ── HTTP API 18780 ─┘

agent/agent2 ── TCP ──► haproxy :19090（L4：leastconn + tcp-check + 运行时重解析）
                          ├─► croupier-server  :19090 ──┐
                          └─► croupier-server2 :19090 ──┴─► 集群互联转发 + 共享目录

sdk-examples ── HTTP ──► agent:19091      haproxy stats ──► :8404（连接分布排查）
```

- **双 Server**（`server`/`server2`，YAML anchor 共享配置）：集群成员表 + owner 转发自动协同；任一实例故障，另一实例接管调用（Agent 断连重连经 LB 分发至存活实例，架构文档 §6 故障语义）
- **双 Agent**（`agent`/`agent2`）：上游统一走 `haproxy:19090` L4 LB；`configs/agent2.yaml` 区分 Agent ID 与 httpAddr
- **两层负载均衡各司其职**（nginx 管人，HAProxy 管机器）：
  - dashboard nginx（L7）：`split_clients` 按请求哈希分流到两实例 18780 + docker DNS resolver 运行时解析（10s，实例重建换 IP 不 502）；SSE 已关缓冲
  - haproxy（L4）：Agent 自研 transport TCP 长连接 `leastconn` 打散 + `tcp-check` 主动健康检查 + `resolvers` 运行时重解析（实例重建自动跟随）+ stats 页（:8404）
- 宿主端口只由每组实例 1 发布（server: 8443/18780、agent: 19091）；实例 2 仅集群内可达

```bash
cd docker
docker compose -f docker-compose.deploy.yml up -d
# 验证：Dashboard「运维中心 → 集群拓扑」应显示两实例在线 + agent 归属分布
```

> **历史注意**：曾用 dashboard nginx 的 stream 块承担 Agent L4（upstream 仅启动时解析，实例重建换 IP 需 `--force-recreate dashboard`）；现由独立 haproxy 服务承担（运行时重解析 + 主动健康检查），该限制不再存在。

### 负载均衡方案选型

Agent 的 L4 入口与 Dashboard 的 L7 入口均可按部署环境替换，详细对比（nginx stream vs HAProxy vs keepalived+LVS）、配置示例与迁移路径见独立篇 **[负载均衡](./load-balancing.md)**。速记：

| 方案                               | 层级      | 适用                                                                                               |
| ---------------------------------- | --------- | -------------------------------------------------------------------------------------------------- |
| **HAProxy**（本仓库默认）          | L4        | Agent 接入 LB：运行时 DNS 重解析 + 主动健康检查 + stats 可观测                                     |
| nginx stream                       | L4 TCP    | 备选：复用 dashboard 镜像零新增组件，但 upstream 仅启动时解析（实例重建需 reload）、无主动健康检查 |
| **keepalived + LVS/nginx/HAProxy** | VRRP + L4 | LB 自身高可用（VIP 漂移）；公有云等价物是云 NLB                                                    |
| Kubernetes Service                 | L4        | K8s 环境无需自建 LB 层                                                                             |

无论哪种方案，**Agent 侧无需任何改动**：单地址连接 + 断线重连 + 重新注册（架构文档 §6.2 故障转移时间线）。

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
4. Server 对外暴露 HTTP 入口；Agent 经 L4 负载均衡入口接入（见上文「负载均衡方案选型」），不要直连单个 Server 实例。

## 下一步

- [监控指南](./monitoring.md)
- [安全配置](./security.md)
- [故障排查](./troubleshooting.md)
