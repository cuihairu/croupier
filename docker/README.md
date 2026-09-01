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
└── Dockerfile.demo (deprecated; not used by the telemetry stack)
```

## 当前端口语义

当前推荐按“统一 session + 本地 gateway”理解端口，而不是历史 `gRPC` / `旧传输` 命名：

| 端口    | 含义                                                         |
| ------- | ------------------------------------------------------------ |
| `18780` | Server REST API                                              |
| `19090` | Server session/control 入口，供 Agent 主动连接               |
| `19091` | Agent 本地 gateway，供 SDK / GameServer / 第三方本地程序接入 |
| `8000`  | Dashboard                                                    |
| `18081` | Analytics Ingestion                                          |

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
docker compose -f docker/docker-compose.telemetry.yaml up -d

# 完整的 audit -> trace -> metric 验收（使用独立 Compose project）
bash scripts/test-telemetry.sh
```

## 服务访问

| 服务                   | 地址                   | 说明                      |
| ---------------------- | ---------------------- | ------------------------- |
| Dashboard              | http://localhost:8000  | 控制台                    |
| Server REST API        | http://localhost:18780 | 管理与查询接口            |
| Server Session Control | localhost:19090        | Agent 上行 session 入口   |
| Agent Local Gateway    | localhost:19091        | SDK / GameServer 本地接入 |
| Analytics Ingestion    | http://localhost:18081 | 公网/DMZ 摄取入口         |

遥测栈本机端口：OTLP HTTP `14318`、OTLP gRPC `14317`、Collector health
`13313`、Collector metrics `18889`、Jaeger `17686`、Prometheus `19092`、Grafana
`13000`。

## 数据库

| 服务       | 地址                  |
| ---------- | --------------------- |
| PostgreSQL | localhost:5432        |
| Redis      | localhost:6379        |
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

## HA 双实例拓扑（docker-compose.deploy.yml）

deploy 编排按 HA 多实例架构（`docs/architecture/server-ha-multi-instance.md`）部署：

- **server / server2**：双 Server 实例（YAML anchor 共享配置，`server_data` 卷共享）；集群成员表 + owner 转发自动协同，任一实例故障另一实例接管调用
- **agent / agent2**：双 Agent 实例，上游统一经 haproxy 的 L4 LB（`haproxy:19090`）打散到两台 Server；断连重连由 LB 分发至存活实例（`configs/agent2.yaml` 区分 Agent ID 与 httpAddr）
- **两层负载均衡**（nginx 管人，HAProxy 管机器）：
  - dashboard nginx（L7）：`split_clients` 按请求哈希分流到两实例 18780（resolver 运行时解析，实例重建换 IP 不会 502；SSE 已关缓冲）
  - haproxy（L4，`configs/haproxy.cfg`）：Agent TCP `leastconn` 打散 + `tcp-check` 主动健康检查 + `resolvers` 运行时重解析 + stats 页（:8404）
- 宿主端口只由每组实例 1 发布（server: 8443/18780，agent: 19091）；实例 2 仅集群内可达
- 方案选型（nginx stream / HAProxy / keepalived）见 `docs/operations/load-balancing.md`

## 开发建议

- 只需要数据库时，可先单独拉起 `postgres`、`redis`、`clickhouse`
- 开发主链路时，重点关注 `18780`、`19090`、`19091`
- SDK 接入时，应把目标地址指向 Agent 本地 gateway，而不是 Server

## IP 属地查询（审计日志）

审计日志（登录日志/操作日志）的「属地」列由 [IP2Location LITE](https://lite.ip2location.com) 数据库驱动（`internal/ipgeo`）。**未部署 BIN 数据文件时功能优雅降级：IP 正常记录，属地为空。**

### 方式一：构建进镜像（推荐）

1. 注册 [lite.ip2location.com](https://lite.ip2location.com) 免费账号，进入数据库下载页（推荐 DB3，含国家/省/市）；
2. 复制完整下载链接（形如 `https://download.ip2location.com/lite/?token=...`）；
3. 在仓库 Settings → Secrets → Actions 添加 `IP2LOCATION_BIN_URL`，值为该链接；
4. 之后 CI 构建的 `croupier-server` 镜像自动内置 BIN（`Dockerfile.server` 的 `ip2location` stage），无需其他配置。

本地构建同理：

```bash
docker build -f docker/Dockerfile.server \
  --build-arg IP2LOCATION_BIN_URL="https://download.ip2location.com/lite/?token=..." \
  -t croupier-server:with-ipdb .
```

### 方式二：运行时挂载（不重建镜像）

下载 BIN 后挂载进容器（ipgeo 自动探测 `./configs/IP2LOCATION-LITE-DB3.BIN`）：

```yaml
# docker-compose.override.yml
services:
  server:
    volumes:
      - ./data/IP2LOCATION-LITE-DB3.BIN:/app/configs/IP2LOCATION-LITE-DB3.BIN:ro
      # IPv6 可选：IP2LOCATION-LITE-DB3.IPV6.BIN
```

或用环境变量指定任意路径（`IP2LOCATION_BIN_PATH` / `IP2LOCATION_BIN_PATH_V6`）。

### 许可证注意

IP2Location LITE 遵循 **CC BY-SA 4.0**：分发包含该数据库的镜像（如推送到公开 registry）需保留对 IP2Location 的署名；内部使用无额外要求。共享（share-alike）条款意味着基于该数据库的衍生数据集需同协议开放——仅查询展示属地不属于衍生数据集。

## 生产建议

1. 修改默认密码与默认密钥
2. `Agent <-> Server` 保持 TLS 开启，生产环境优先 mTLS
3. 对 `19090`、`19091` 做明确网络边界控制
4. Dashboard 与 REST API 走 HTTPS 与统一鉴权
5. 为证书、JWT、数据库凭据接入 Secret Manager
6. 需要审计属地展示时，配置 IP2Location BIN（见上节），并为内网请求正确设置 `X-Forwarded-For`（反代/负载均衡场景）
