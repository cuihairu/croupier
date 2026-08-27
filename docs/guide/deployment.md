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

### HA 双实例部署（docker-compose.deploy.yml）

生产编排 `docker/docker-compose.deploy.yml` 按 [Server 多实例 HA 架构](../architecture/server-ha-multi-instance.md) 部署双实例拓扑：

```
                          ┌─► croupier-server  ── HTTP API 18780 ─┐
浏览器 ──► dashboard nginx ┤                                       ├─► 共享 postgres/redis
（L7 分流）                └─► croupier-server2 ── HTTP API 18780 ─┘
        │
        └─► nginx stream :19090（L4 TCP 透传，least_conn）
                ├─► croupier-server  :19090 ──┐
                └─► croupier-server2 :19090 ──┴─► 集群互联转发 + 共享目录

agent/agent2 ── TCP session ──► croupier-dashboard:19090（L4 LB 打散）
sdk-examples ── HTTP ──► agent:19091
```

- **双 Server**（`server`/`server2`，YAML anchor 共享配置）：集群成员表 + owner 转发自动协同；任一实例故障，另一实例接管调用（Agent 断连重连经 LB 分发至存活实例，架构文档 §6 故障语义）
- **双 Agent**（`agent`/`agent2`）：上游统一走 `croupier-dashboard:19090` L4 LB；`configs/agent2.yaml` 区分 Agent ID 与 httpAddr
- **dashboard nginx 双层负载均衡**：
  - L7（HTTP `/api/`）：`split_clients` 按请求哈希分流到两实例 18780 + docker DNS resolver 运行时解析（10s，实例重建换 IP 不 502）；SSE 已关缓冲
  - L4（TCP 19090 stream）：Agent 自研 transport 长连接透传，`least_conn` 按活跃会话数打散
- 宿主端口只由每组实例 1 发布（server: 8443/18780、agent: 19091）；实例 2 仅集群内可达

```bash
cd docker
docker compose -f docker-compose.deploy.yml up -d
# 验证：Dashboard「运维中心 → 集群拓扑」应显示两实例在线 + agent 归属分布
```

> **stream upstream 重解析注意**：nginx（开源版）`upstream` 块只在启动/reload 时解析域名。Server 实例重建换 IP 后需 `docker compose up -d --force-recreate dashboard`（或容器内 `nginx -s reload`）。

### 负载均衡方案选型

Agent 的 L4 入口与 Dashboard 的 L7 入口均可按部署环境替换，三种常见方案的取舍：

| 方案                           | 层级      | 优点                                                                                     | 缺点                                                             | 适用                                        |
| ------------------------------ | --------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------- | ------------------------------------------- |
| **nginx stream**（本仓库默认） | L4 TCP    | 复用 dashboard 镜像零新增组件；`least_conn` 适配长连接                                   | upstream 域名仅启动时解析（实例重建需 reload）；单容器承载 L7+L4 | 单机 compose / 中小规模                     |
| **HAProxy**                    | L4/L7     | 运行时 DNS 重解析（`resolvers` + `resolve-headers`）天然适配容器 IP 变动；TCP 健康检查细 | 新增组件与配置面                                                 | 实例频繁重建的环境（如 K8s 之外的自建编排） |
| **keepalived + LVS/nginx**     | L4 + VRRP | 解决 **LB 自身高可用**（VIP 漂移）；机房级容灾                                           | 需多台宿主 + 网络层支持 VRRP/组播；配置复杂                      | 多宿主生产部署，LB 不能是单点               |

选型原则：

1. **单机双实例**（compose 形态）：nginx stream 足够——LB 与 dashboard 同容器，故障域一致。
2. **多宿主容器化**：HAProxy 或云厂商 NLB（L4）打散 agent 连接；LB 自身 2 副本 + 前置 DNS 轮询。
3. **LB 必须自身高可用**：keepalived VRRP VIP 漂移（主备 nginx/HAProxy），或直接用云 NLB 免运维。
4. K8s 环境：直接用 Service（ClusterIP）+ Endpoints 天然负载均衡，无需自建 LB 层。

无论哪种方案，**Agent 侧无需任何改动**：单地址连接 + 断线重连 + 重新注册（架构文档 §6.2 故障转移时间线），入口层换型对 Agent 透明。

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

- [监控指南](./operations/monitoring.md)
- [安全配置](./operations/security.md)
- [故障排查](./operations/troubleshooting.md)
