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
- 可选：capture-agent（CDC 变更捕获，源码构建栈默认启用，见下文）

## Docker 部署

三份编排按场景分工，别混用：

| 编排                            | 镜像来源    | 适用                                        |
| ------------------------------- | ----------- | ------------------------------------------- |
| `docker-compose.quickstart.yml` | ghcr 预构建 | 快速体验/演示：一条命令最小栈，无需本地构建 |
| `docker-compose.yml`            | 源码构建    | 本地开发：改代码即改容器                    |
| `docker-compose.deploy.yml`     | ghcr 预构建 | 生产 HA：双实例 server/agent + haproxy      |

### 快速体验（quickstart，开箱即用）

预构建镜像（ghcr.io，匿名可拉）起最小栈：postgres + redis + server + agent + dashboard。

```bash
cd docker
docker compose -f docker-compose.quickstart.yml up -d
# 验证：五容器全 healthy；curl http://localhost:18780/healthz 应见
# "ok":true 且 registry.agentsHealthy=1（agent 已注册）；Dashboard 在 :8000
```

常用操作：

```bash
# 看日志 / 停止
docker compose -f docker-compose.quickstart.yml logs -f server
docker compose -f docker-compose.quickstart.yml down

# 升级全栈（pull + 重建）
docker compose -f docker-compose.quickstart.yml pull
docker compose -f docker-compose.quickstart.yml up -d

# 清数据（⚠️ 连数据卷一起删：业务库、上传文件全没）
docker compose -f docker-compose.quickstart.yml down -v
```

可选组件用 profile 拉起，不混进默认最小栈：

```bash
# SDK 六语言 + OpenAPI 示例——只想起示例时显式列服务名（见下「隐式全栈更新」坑）
docker compose -f docker-compose.quickstart.yml --profile sdk-examples \
  up -d sdk-demo-go sdk-demo-python sdk-demo-java \
        sdk-demo-js sdk-demo-cpp sdk-demo-csharp

# 分析管道（ClickHouse + ingestion + worker）；置 ANALYTICS_BRIDGE_ENABLED=true
# 后 server 事件才写入分析流
ANALYTICS_BRIDGE_ENABLED=true docker compose -f docker-compose.quickstart.yml \
  --profile analytics up -d
```

> **隐式全栈更新坑**：`--profile sdk-examples pull && up -d` 会把
> server/agent/dashboard 的 main 镜像一并拉新并级联 recreate，效果等同一次
> 升级（新功能 + 数据库迁移直接上线）。只想重建示例容器时，pull/up 都显式
> 限定服务名；接受级联就顺手验证新迁移已落库。secret/端口/多游戏单库切换
> 等说明见 `docker/docker-compose.quickstart.yml` 文件头注释。

### 本地开发（源码构建）

```bash
docker compose up -d
```

当前镜像构建入口：

- `docker/Dockerfile.server` -> `./cmd/server`
- `docker/Dockerfile.agent` -> `./cmd/sidecar-agent`（定名 `sidecar-agent`，方案 A 已落地：仅源码目录改名，产物名 `croupier-agent` 不变，见 [Agent 清单](../design/agents-inventory.md) §9）
- `docker/Dockerfile.capture-agent` -> `./cmd/capture-agent`（CDC，见下）
- `docker/Dockerfile.analytics-worker` -> `./cmd/analytics-worker`
- `docker/Dockerfile.ingest` -> `./cmd/ingest`

#### capture agent（CDC，源码构建栈默认启用）

出站连 `server:19090` 注册上线 + 订阅 postgres 逻辑复制流（`wal_level=logical` 已由 `postgres.command` 固化），订阅表白名单 `public.capture_demo`（示例表，生产改成道具相关表）。**默认启用**，开关是环境变量 `CAPTURE_AGENT_REPLICAS`（`deploy.replicas`，默认 `1`；`0` = 关闭）。

| 环境变量                     | 默认值                                                                             | 说明                                                                                                                         |
| ---------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `CAPTURE_AGENT_REPLICAS`     | `1`                                                                                | capture-agent 副本数。只允许 `0` 或 `1`——`capture_data` 是共享位点卷，多副本会互踩位点。`0` = 关闭服务                       |
| `CROUPIER_CAPTURE_PG_DSN`    | `postgres://croupier:croupier_dev_password@postgres:5432/croupier?sslmode=disable` | 源库连接串。**凭据红线**：明文 DSN 只走环境变量，配置文件 `docker/configs/capture-agent.yaml` 只写变量名（`capture.dsnEnv`） |
| `POSTGRES_IMAGE`             | `postgres:15-alpine`                                                               | 与全栈共用；CDC 需要 pgoutput 解码（PG 10+，逻辑复制）                                                                       |
| `GO_IMAGE` / `RUNTIME_IMAGE` | `golang:1.27.2` / `gcr.io/distroless/static:nonroot`                               | 与其他服务共用                                                                                                               |

前置（postgres 侧）：

- **`wal_level=logical`**：compose 的 `postgres.command` 已固化；存量栈需给 postgres 加同款 `command` 后重建容器（`command` 变更会触发 recreate）。
- **publication 必须预建**（connector 只建槽不建发布）：`docker/postgres-init/01-capture-publication.sql` 在**首次初始化空卷**时创建 `public.capture_demo` 表 + `croupier_capture` 发布。存量卷不会重放 initdb 脚本，需手工执行：

```bash
docker exec croupier-postgres psql -U croupier -d croupier <<'SQL'
CREATE TABLE IF NOT EXISTS public.capture_demo (id int PRIMARY KEY, note text, updated_at timestamptz NOT NULL DEFAULT now());
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'croupier_capture') THEN
    CREATE PUBLICATION croupier_capture FOR TABLE public.capture_demo;
  END IF;
END $$;
SQL
```

- **REPLICATION 权限**：本栈 `POSTGRES_USER=croupier` 是超管，天然具备；生产请用独立复制账号并授 `REPLICATION`。

一键验证（启用态）：

```bash
docker compose up -d                          # capture-agent 默认起
docker compose ps capture-agent               # STATUS 应变为 healthy
docker compose logs --tail=30 capture-agent | grep -iE 'registered upstream|postgres|event'
# 造一条真实变更 → 日志出现解码事件
docker exec croupier-postgres psql -U croupier -d croupier \
  -c "INSERT INTO public.capture_demo VALUES (1, 'compose-verify', now())"
```

关闭验证：

```bash
CAPTURE_AGENT_REPLICAS=0 docker compose up -d capture-agent   # 缩到 0 个容器
docker compose ps capture-agent                                # 无输出
```

健康检查走**心跳文件新鲜度**而非端口：capture agent 无本地监听端口（出站-only），healthcheck 是 `find /app/data/capture -name agent-heartbeat -mmin -2`（supervisable 心跳打点，`restart: unless-stopped` 兜底）。

已知边界：

- `docker-compose.deploy.yml`（ghcr 预构建，生产 HA 栈）暂不含 capture-agent：CI 只构建 server/agent/ingest/analytics-worker/openapi-demo 镜像，未发布 `croupier-capture-agent` 镜像。要在 HA 栈启用，需先给 `.github/workflows/docker.yml` 加 build job 并补 `x-capture-agent-common` anchor。
- `docker-compose.quickstart.yml`（最小体验栈）不含 capture-agent，避免给演示路径引入 CDC 前置（wal_level / publication）。
- capture agent 连不上 server 时**只影响注册与策略拉取，不影响捕获**（runner 的注册循环与捕获循环独立）；反向地，`capture.enabled=false` 时进程仍注册上线并打心跳，只是不落捕获（registration only）。

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

openapi-provider-demo ── TCP ──► haproxy :19090（内嵌 Agent，providers.yaml 注册函数）
```

- **双 Server**（`server`/`server2`，YAML anchor 共享配置）：集群成员表 + owner 转发自动协同；任一实例故障，另一实例接管调用（Agent 断连重连经 LB 分发至存活实例，架构文档 §6 故障语义）
- **双 Agent**（`agent`/`agent2`）：上游统一走 `haproxy:19090` L4 LB；`configs/agent2.yaml` 区分 Agent ID 与 httpAddr（该文件不入库，部署时复制 `configs/agent.yaml` 修改差异项生成）
- **两层负载均衡各司其职**（nginx 管人，HAProxy 管机器）：
  - dashboard nginx（L7）：`split_clients` 按请求哈希分流到两实例 18780 + docker DNS resolver 运行时解析（10s，实例重建换 IP 不 502）；SSE 已关缓冲
  - haproxy（L4）：Agent 自行开发 transport TCP 长连接 `leastconn` 打散 + `tcp-check` 主动健康检查 + `resolvers` 运行时重解析（实例重建自动跟随）+ stats 页（:8404）
- 宿主端口只由每组实例 1 发布（server: 8443→19090 transport / 18780 HTTP、agent: 19091）；实例 2 仅集群内可达
- **openapi-provider-demo**（`sdk-examples` profile，随 `enable_sdk_examples=true` 部署）：`examples/openapi-provider` 的常驻容器（镜像 `croupier-openapi-provider-demo`），内嵌 Agent 经 providers.yaml 把 players API 注册进 server——Dashboard「OpenAPI Sources → 运行时导入」区块的数据源，scope 跟随 `CROUPIER_SDK_EXAMPLE_GAME_ID/ENV`

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
curl http://localhost:18780/healthz
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
