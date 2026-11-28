# 真实部署与外部依赖配置盘点（DEPLOY-001，2026-11-28）

对部署宿主（内网 Linux 主机，下称 deploy-host，即 CI 部署 workflow 的目标机）上
真实运行的 croupier 平台栈做只读盘点。本报告是 DEPLOY-001 的交付物：
核对实际镜像摘要/角色/端口/TLS/协调后端/备份、Herald 与外部平台开关、OIDC/Vault/
观测出口，并给出与仓库模板的差异。

证据方法（2026-11-28 采集，全部只读）：

- `docker ps -a` / `docker inspect`（容器、镜像 ID、compose project 标签、挂载、重启策略）
- `docker images --digests`（registry 摘要）
- 部署侧 `docker-compose.yml` / `.env` / `configs/*` 与仓库模板逐文件 `diff`/md5
- `psql`（prod 库行数计数，只读 SELECT）
- `crontab -l`（宿主定时任务）

不把 Compose 存在当部署证据：下表所有「运行中」状态均来自 live `docker ps`
（Up 11 days + healthcheck healthy），数据面另有 113 张表 / 4 个管理员 / 5 个
游戏的 prod 库计数佐证。

## 1. 拓扑总览（compose project `croupier`）

部署目录 deploy-host:`/data/deploy/croupier/`（compose 文件 + configs/ + .env）。
全部容器 `restart=unless-stopped`，单 docker bridge 网络 `croupier_croupier-network`。

| 服务                  | 容器                                             | 镜像                                              | 镜像 ID (sha256 前 12) | 宿主端口                     | 状态                           |
| --------------------- | ------------------------------------------------ | ------------------------------------------------- | ---------------------- | ---------------------------- | ------------------------------ |
| server (HA-1)         | croupier-server                                  | ghcr.io/cuihairu/croupier-server:main             | 77ab343d4e69           | 8443→19090, 18780            | Up 11d (healthy)               |
| server (HA-2)         | croupier-server2                                 | 同上（同镜像同 ID）                               | 77ab343d4e69           | 无宿主发布（18780 仅集群内） | Up 11d (healthy)               |
| agent-1               | croupier-agent                                   | ghcr.io/cuihairu/croupier-agent:main              | 1d328681704f           | 19091                        | Up 11d (healthy)               |
| agent-2               | croupier-agent-2                                 | 同上                                              | 1d328681704f           | 无宿主发布                   | Up 11d (healthy)               |
| dashboard             | croupier-dashboard                               | ghcr.io/cuihairu/croupier-dashboard:main          | 038936b3567a           | 8000→80                      | Up 11d (healthy)               |
| postgres              | croupier-postgres                                | public.ecr.aws/docker/library/postgres:15-alpine  | 23799e521dee           | **5432 对宿主发布**          | Up 11d (healthy)               |
| redis                 | croupier-redis                                   | public.ecr.aws/docker/library/redis:7-alpine      | 7f7d30b7fdd0           | **6379 对宿主发布**          | Up 11d (healthy)               |
| clickhouse            | croupier-clickhouse                              | clickhouse/clickhouse-server:23                   | —                      | **8123 / 19010 对宿主发布**  | Up 12d (healthy)               |
| haproxy               | croupier-haproxy                                 | haproxy:2.9-alpine                                | —                      | 8404 (stats)                 | Up 24h (healthy)               |
| otel-collector        | croupier-otel-collector                          | otel/opentelemetry-collector-contrib:0.91.0       | —                      | 无宿主发布                   | Up 12d                         |
| jaeger                | croupier-jaeger                                  | jaegertracing/all-in-one:1.62.0                   | —                      | 17686→16686 (UI)             | Up 12d                         |
| ingestion             | croupier-ingestion                               | ghcr.io/cuihairu/croupier-ingest:main             | c3583aa05e6e           | 18081→8088                   | Up 11d（analytics profile）    |
| analytics-worker      | croupier-analytics-worker                        | ghcr.io/cuihairu/croupier-analytics-worker:main   | d329dd4632c8           | 无宿主发布                   | Up 11d（analytics profile）    |
| sdk-demo ×6           | croupier-sdk-demo-{go,python,java,js,cpp,csharp} | ghcr.io/cuihairu/croupier-sdk-*-example-demo:main | —                      | 无宿主发布                   | Up 11d（sdk-examples profile） |
| openapi-provider-demo | croupier-openapi-provider-demo                   | …croupier-openapi-provider-demo:main              | —                      | —                            | **Created（未启动）**          |

注：镜像 tag 全部为 `main` 滚动 tag（.env 显式固定 `CROUPIER_*_TAG=main`），
镜像 ID 即当日部署拉取的摘要；滚动更新由部署 workflow 的 pull+up 级联完成。

## 2. 协调后端与模板差异

协调后端：**docker compose（单宿主，无 k8s/swarm）**，compose project 名
`croupier`，配置文件 deploy-host:`/data/deploy/croupier/docker-compose.yml`。

与仓库模板逐文件比对结果（md5 / diff）：

| 部署侧文件                    | 仓库对应文件                         | 差异                                                                                                                                 |
| ----------------------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------ |
| docker-compose.yml            | docker/docker-compose.deploy.yml     | **逐字节一致**                                                                                                                       |
| docker-compose.telemetry.yaml | docker/docker-compose.telemetry.yaml | **逐字节一致**                                                                                                                       |
| configs/server.yaml           | docker/configs/server.yaml           | **逐字节一致**                                                                                                                       |
| configs/agent2.yaml           | docker/configs/agent2.yaml           | **逐字节一致**                                                                                                                       |
| configs/haproxy.cfg           | docker/configs/haproxy.cfg           | **逐字节一致**                                                                                                                       |
| configs/otel-collector.yaml   | configs/otel-collector-config.yaml   | **逐字节一致**（仅文件名不同）                                                                                                       |
| configs/prometheus.yml        | configs/prometheus.yml               | **逐字节一致**                                                                                                                       |
| configs/agent.yaml            | docker/configs/agent.yaml            | 仅缺仓库侧后来新增的 telemetry 注释说明块（无配置语义差异）                                                                          |
| .env                          | —（部署侧私有）                      | 极简：`GHCR_OWNER` / 四个 `CROUPIER_*_TAG=main` / `ENABLE_SDK_EXAMPLES=true` / `ANALYTICS_BRIDGE_ENABLED=true`，其余全部走模板默认值 |

结论：部署栈运行的是 `docker-compose.deploy.yml` 这一代模板（HA + 观测 +
analytics profile），仓库 `docker/docker-compose.yml` 是旧一代单实例模板，
部署未使用。模板漂移≈0（仅 agent.yaml 注释滞后与 .env 私有覆盖）。

## 3. HA 形态

- `CROUPIER_CLUSTER_ENABLED=true`，双实例 `croupier-server` / `croupier-server2`，
  实例 ID/advertiseAddr 按容器名注入（成员表 + owner 转发生效）。
- 集群互联：`allowPlaintextInterconnect: true`（单宿主 docker 内网模板的显式
  声明；无跨机链路，未配 interconnectTls 证书面）。
- agent 接入：两实例 agent 均经 `haproxy:19090`（HAProxy L4）。haproxy 形态为
  **server 单活 + server2 backup 冷备**（registry 是 server 实例内存态、无跨
  实例同步，双活会产生悬尸 session——2026-09-11 线上事故后的修正形态）。
- agent 上游连接 `insecure: true`、`tls.enabled=false`、`outboundTLS.enabled=false`；
  server 侧 `control` 无证书配置 → **agent↔server transport 全程明文**（单宿主
  内网语义）。
- `server_data` named volume 双实例共享（assignments/上传文件跨实例可见）。

## 4. TLS / 安全面（如实记录）

| 面                               | 状态                                                                                                         |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| agent↔server transport（19090）  | 明文（control cert/key/ca 空，agent tls/outboundTLS 关）                                                     |
| server↔server 集群互联           | 明文（allowPlaintextInterconnect=true 显式确认）                                                             |
| server↔postgres/redis/clickhouse | `sslmode=disable` / 无 TLS（栈内 bridge 网络）                                                               |
| dashboard↔server HTTP            | 明文 HTTP（8000→nginx→18780）                                                                                |
| 对外入口                         | 无 L7 TLS 终结；dashboard 8000 / HTTP API 18780 / transport 8443 均按宿主端口直发                            |
| server `mode`                    | `dev`                                                                                                        |
| 数据库端口                       | postgres 5432、redis 6379、clickhouse 8123/19010 **均对宿主 0.0.0.0 发布**，且 `.env` 未覆盖模板默认开发口令 |
| 鉴权                             | 仅本地账号（users.json）；LDAP/OIDC 在部署 server.yaml 中整节注释禁用                                        |
| Vault（密码箱）                  | `VaultConfig` 存在但缺省 false，部署配置无该节 → 未装配                                                      |
| analytics ingest                 | `ANALYTICS_INGEST_SECRET` 走模板默认值                                                                       |

以上是单宿主内网演示栈的真实形态；模板内注释已声明「跨机生产必须配
interconnectTls 并移除明文确认行」。本报告只记录事实，拓扑风险汇总见 §8。

## 5. 备份（关键发现：零备份）

- 平台侧备份能力**已实现**：`/ops` 备份 API（`internal/api/ops/backups.go`）+
  `internal/platform/backupexec` 真实执行器 + `ObjectStore`（部署侧
  `storage.driver=file` → 文件对象存储，落 `server_data` 卷）。
- **prod 库 `backups` 表 0 行**（2026-11-28 只读 SELECT）——部署以来从未执行过
  任何一次备份。
- 宿主 crontab 仅两条：磁盘看门狗与 runner 缓存清理，**无任何数据库备份任务**；
  postgres/redis/clickhouse 数据卷无快照机制。
- 推论：DEPLOY-002（备份可恢复演练）的「复用 backupexec 和既有备份」前提在
  当前部署上不存在既有备份，需先在 prod 栈上通过既有备份 API 创建一次真实
  备份（平台内既有能力，不新增服务），再谈隔离恢复演练。

## 6. 观测出口

- OTel：server/agent `OTEL_ENABLED=true` → 栈内 `croupier-otel-collector`
  (OTLP :4318) → 栈内 `croupier-jaeger`（UI 宿主发布 :17686）。出口全部在栈内，
  无外部观测平台外发。
- LB 监控源：`CROUPIER_LB_PROMETHEUS_URL=http://host.docker.internal:19092`
  指向 **croupier-telemetry** 独立 compose 项目（prometheus + jaeger + otel），
  但该项目三个容器全部处于 `Created`（从未启动）→ **:19092 是死源**，
  `/ops/lb` 页面监控数据源不可用（页面按设计隐藏）。
- haproxy stats：宿主发布 :8404（内置 Prometheus exporter，`/metrics` 可抓），
  telemetry 栈未运行即无人抓取。
- `CROUPIER_GRAFANA_EXPLORE_URL` 未配置。
- 遗留物：孤儿卷 `croupier_prometheus-storage`（现行 deploy.yml 无 prometheus
  服务，系旧一代栈残留）。

## 7. Herald 与外部平台开关

- croupier→herald 告警出口：部署 server.yaml **无 herald 节**（模板中该节本就
  注释、缺省关）→ 未接入。监督熔断等告警仅走 croupier 自身告警面。
- deploy-host 上确实运行独立栈：`herald`（compose project `herald`，
  /opt/herald，宿主 :18060/:18061，Up 12d healthy）与 `rustfs`
  （compose project `rustfs`，/data/deploy/rustfs，宿主 :9000/:9001，S3 兼容
  对象存储）。两者**均未与 croupier 配置级连接**（croupier 无 herald/baseUrl、
  无 s3 对象存储配置），属宿主上其他产品线/预留设施。
- 外部身份源：LDAP/OIDC 禁用（注释整节）；无 Vault；无外部 SMTP/webhook
  出口配置。平台当前对外网的外发依赖仅 ghcr.io 镜像拉取。

## 8. 风险与边界清单（盘点结论，供后续派单）

1. **零备份**（§5）：平台备份能力已实现但从未执行；数据卷无快照。DEPLOY-002
   的前置动作是「先创建一次真实备份」。
2. **模板默认凭据 + 数据库/缓存端口对宿主发布**（§4）：postgres/redis/
   clickhouse 使用模板默认开发口令且 0.0.0.0 发布；内网暴露面大于必要。
3. **全明文链路**（§3/§4）：agent transport、集群互联、DB、dashboard HTTP 均
   无 TLS；单宿主内网可接受，跨机/公网前必须逐项开 TLS。
4. **LB 监控死源**（§6）：19092 指向从未启动的 telemetry 栈；要么启动
   telemetry 项目，要么显式清空该变量。
5. `openapi-provider-demo` Created 未启动（sdk-examples profile 容器运行 6 个、
   该 1 个未启）——与「运行时导入」区块数据源相关，按需启动。
6. `mode: dev` + 滚动 `main` tag：无版本固定，生产语义缺失（每日交付面如此
   设计，仅记录）。

## 脱敏说明

本报告不含口令、secret、token、JWT 密钥与宿主 IP；镜像摘要为公开 registry
摘要可保留。容器名/端口/路径为部署事实，保留。
