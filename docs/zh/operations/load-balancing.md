---
title: 负载均衡
icon: deployment
order: 7
category:
  - 运维手册
tag:
  - 部署
  - 高可用
---

# 负载均衡

Croupier 的 HA 多实例架构（Server 多实例 HA（本节仅中文））有两条流量入口，各自需要负载均衡：

- **Dashboard API（L7 / HTTP）**：Dashboard 前置网关把流量拆到各 Server 实例的 `:18780`
- **Agent 接入（L4 / TCP）**：agent 的自研 transport 长连接（`:19090`，长度前缀分帧 + 多路复用）分散到各 Server 实例

**为什么 agent 必须走 L4 LB 而不是直连**：直连时实例故障会让该实例名下所有 agent 掉线且需要人工干预；经 LB 分散后，重连自动落到存活实例 + 重新注册更新归属（架构文档 §6 failover 时间线，全程零人工）。

## 部署形态约束：主 + 冷备（当前推荐；双活前置已就绪，待切换验证）

**背景认知**：agent 会话表与函数注册注册表是 Server 实例的**进程内内存状态**；跨实例同步只覆盖部分路径（`/ops/nodes` 的 agent 列表与 `/functions/instances` 聚合经共享归属表——instances 已于 2026-09-11 完成）。invoke 执行路径已于 2026-09-11 完成归属表整合与转发，双活下的调用正确性不再依赖「请求恰好落到持有该 agent 的实例」：

- **候选集回落（RemoteAgentSource）**：本地候选为空或 failover 耗尽本地候选时，同步查询共享归属表 + `agent_sessions` 快照表（1s 预算；出错则降级回 `no live agent` 语义，不放大故障）补充远端候选；选中的远端候选经 mesh 转发到属主实例执行
- **三支转发**：同步 invoke、异步任务（start_task，与 invoke 同一 failover——lb 路由失败换下一候选重试，失败尝试把 task_runs 行标记 failed，耗尽映射 503）、任务取消（cancel_task，任务路由 miss 时从共享 `task_runs` 解析 agent）都走同一帧格式（`kind` 区分）；broadcast（候选集本地 ∪ 远端，按 AgentID 去重、本地优先）同样复用；属主侧直接投递并记录审计（不重查策略；信任边界见 `docs/architecture/server-ha-multi-instance.md` §5.3）
- `refreshRemoteSnapshots` 的 30s 周期回填从正确性依赖降级为性能优化层（减少热路径上的同步 DB 查询）
- **注册表内存会话生命周期与归属表对齐**（2026-09-11 完成，三级清理）：本地断连即清理（`RemoveAgentIfStale` 的 notAfter 检查挡住断连瞬间与重新注册的竞态）、重启恢复按归属表活跃全集过滤快照行、30s 对账趟清理「无归属行且 5min 无心跳」的孤儿副本——掉线 agent 不再作为僵尸候选在函数视图/lb/broadcast 候选集里滞留到 ExpireAt（24h）。DB `agent_sessions` 快照行不删不改、自然过期（归属表是跨实例查询的唯一入口；无归属行的快照行无消费者）；单实例（未开 cluster）行为不变：断连即清理仍然适用，重启恢复仍是全量

已知边界（切双活前先读）：

- mesh 互联是明文内网 TCP、无对端认证——互联端口必须只在集群内网可达
- hash 路由（`route: hash`）跨实例无稳定放置：仅在本地候选耗尽时扩展到远端候选
- 异步任务事件不经 mesh 回传：事件由属主落共享 DB（`task_events`），调用方 HTTP 轮询/SSE 读共享 DB（既有路径不变）

因此当前部署姿态仍是**主 + 冷备**（双活切换另行操作；前置条件已齐备）：

| 层                    | 配置                                       | 故障语义                                                                       |
| --------------------- | ------------------------------------------ | ------------------------------------------------------------------------------ |
| L4（haproxy.cfg）     | `server croupier-server2 ... check backup` | 所有 agent 连主库；主库摘除后 agent 重连自动落到备库并重新注册                 |
| L7（nginx-main.conf） | `split_clients ... 100% croupier-server`   | 所有 API 流量走主库；主库故障时在此手工提升比例切到 server2（改一个 split 值） |

**切换双活**（前置条件齐备，时机自定）：

1. L7 split 值从 `100% croupier-server` 改成比例（如 `50%`/`50%`），`nginx -s reload`
2. L4：去掉 server2 的 `backup` 标记（agent 连接开始分散）
3. 观察：两个实例的 `/ops/nodes` 都出现 agent 归属；从 dashboard 触发 invoke/异步/broadcast 调用，落点混合在两实例且成功（对端实例出现属主侧审计记录）；`server2` 日志出现 `cluster: refreshed remote agent snapshot` / 属主侧审计
4. 回滚 = 恢复 `100%` + `backup`（agent 重连自然回到主库；期间跨实例调用由转发兜底）

## 背景概念：L4 / L7 / VRRP

### 按协议分层定义的 L4 / L7

L4/L7 指 OSI 模型的协议层——**负载均衡器工作在哪一层，决定了它能看到什么、按什么分发**：

| 层     | OSI 名称    | LB 看到什么                                 | 分发依据                           | 典型实现                                          |
| ------ | ----------- | ------------------------------------------- | ---------------------------------- | ------------------------------------------------- |
| **L4** | transport   | IP + 端口（TCP/UDP 头）；**不解析**应用内容 | 五元组 hash、最少连接              | LVS、HAProxy（`mode tcp`）、nginx stream、云 NLB  |
| **L7** | application | 完整 HTTP 请求（method/path/Header/Cookie） | 路径路由、Header、会话粘滞、金丝雀 | nginx http、HAProxy（`mode http`）、Envoy、云 ALB |

两者的取舍：

- **L4 透传字节流**：协议无关（自研协议也能用）、对长连接天然友好、开销低；但看不到请求内容，只能按「连接」粒度分发，做不了基于内容的路由
- **L7 理解应用语义**：可按 path/Header 拆分/改写/重试/金丝雀；代价是必须解析（甚至终结）应用层协议

对应到 Croupier：Dashboard API 是标准 HTTP，天然适合 **L7**；agent 用自研二进制分帧协议（长度前缀 + protobuf），LB 既无法也无必要理解——**L4 透传**即可——这就是本文两条入口分层部署的依据。

> 顺带说明 **LVS**（Linux Virtual Server）：内核态 L4 转发（IPVS），绕过用户态协议栈；性能是所有方案里最高的；代价是配置与生态复杂——通常在单公司千余节点以下没必要。

### VRRP：解决「LB 本身是单点」

**VRRP（Virtual Router Redundancy Protocol，RFC 5798）**是**网络层高可用协议**，与负载均衡本身无关：

- 多台主机组成一个虚拟路由器共享**虚拟 IP（VIP）**；对外只暴露 VIP，客户端永远连 VIP
- 协议自动选举 **Master**（优先级最高者）持有 VIP 并应答流量；其余为 **Backup** 待命
- Master 周期广播 advertisement（默认每 1s 一次）；Backup 连续约 3 个周期收不到即判 Master 死亡，按优先级接管 VIP——**秒级切换，客户端无感**
- **keepalived** 是 VRRP 在 Linux 上的守护进程实现，通常与 nginx/HAProxy 成对部署：LB 进程负责分发，keepalived 保证「跑 LB 的那台机器」挂掉后 VIP 漂移到备机

注意 VRRP 依赖二层组播/单播可达，**多数公有云 VPC 内不可用**——云上等价物是云 NLB（跨 AZ + 健康检查 + 零运维）。

## 概念澄清：keepalived 不是负载均衡器

三者的关系常被混淆：

- **nginx（stream 模块）**与 **HAProxy**：负载均衡器，真正分发流量的角色
- **keepalived**：VRRP 守护进程，只负责 **VIP 漂移**（主备高可用），自身不做负载均衡；通常与 nginx/HAProxy 成对部署，解决「LB 本身挂了」的问题
- 第三种自建高可用 LB 形态是 **keepalived + LVS**（内核态转发，性能最高）

## 方案对比

### L4 能力对比（nginx stream vs HAProxy）

| 维度              | nginx stream                                                                                     | HAProxy                                                                             | 对 Croupier 的影响                                                                                                   |
| ----------------- | ------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| 运行时 DNS 重解析 | ❌ upstream 主机名只在启动/reload 时解析；实例重建换 IP 后仍用旧 IP（502/连不上）；需手工 reload | ✅ `resolvers` 块 + `resolve-prefer ipv4`；容器 IP 变化自动跟随                     | **最关键的差异**：compose 重建 server 容器后，nginx 需要 `--force-recreate dashboard` 这种拐杖，HAProxy 完全无需动作 |
| 主动健康检查      | ❌ 开源版是被动（`max_fails`/`fail_timeout`；只在连接失败时计数）                                | ✅ `option tcp-check` / `http-check` 主动探测；能摘掉半死实例                       | agent 是长连接多路复用：Server 进程卡死但 TCP 未断时，被动探测摘不干净，调用会持续打到死实例                         |
| 优雅下线          | ❌ reload 直接切断既有连接                                                                       | ✅ 运行时经 stats socket `set server ... state drain`：既有连接排空、新连接不再进入 | Server 滚动升级时 HAProxy 可以零掉线切换                                                                             |
| 可观测性          | 主要是 access log                                                                                | ✅ 内置 stats 页：每后端会话数/队列/错误率实时可见                                  | 排查「agent 连不上/连得不均」时直接看图                                                                              |
| 性能              | epoll，数万并发连接                                                                              | epoll，同一量级；两者都低于 LVS（内核态）                                           | 单公司多游戏（~1k 节点以下）**都不是瓶颈**；无需纠结                                                                 |
| 组件成本          | ✅ 复用 dashboard 镜像，零新增                                                                   | ❌ 新增组件（镜像/配置/监控面）                                                     | nginx 对小团队的核心优势                                                                                             |
| L7 能力           | ✅ SPA 静态资源 + API 反代 + SSE 调优一体                                                        | ✅ 但静态资源服务弱于 nginx                                                         | Dashboard 本就需要 nginx 服务静态资源                                                                                |

### 结论

> **L4 负载均衡角色 HAProxy 全面胜出**（运行时重解析 + 主动健康检查 + 优雅下线 + 可观测性）；nginx stream 的唯一优势是**复用既有 dashboard 容器、零新增组件**。

按部署形态的推荐选择：

| 部署形态                                    | 推荐                                                              | 理由                                                                        |
| ------------------------------------------- | ----------------------------------------------------------------- | --------------------------------------------------------------------------- |
| 单机 compose（`docker-compose.deploy.yml`） | **HAProxy**（现状，独立 `haproxy` 服务）                          | 一种形态统一：完整 L4 能力（重解析/健康检查/stats）；单机与多机不用两套配置 |
| 多机容器化生产                              | **HAProxy**（agent 的 L4 入口）                                   | IP 变化自动跟随 + 主动摘除卡死实例 + stats 排障；dashboard 的 L7 仍归 nginx |
| LB 本身不允许单点                           | **keepalived + HAProxy**（主备 VIP 漂移）；云上等价物是**云 NLB** | VRRP 需要网络层支持；公有云内用 NLB 零运维                                  |
| Kubernetes                                  | **Service（ClusterIP）**                                          | kube-proxy 是天然负载均衡器 + Endpoints 就绪探针；无需自建 LB 层            |

## 配置示例

### nginx stream（备选：复用 dashboard 容器）

```nginx
# docker/nginx-main.conf (part of the deployment artifacts)
stream {
    upstream croupier_agent_lb {
        least_conn;                          # by active session count; fits long connections
        server croupier-server:19090;        # active + cold standby (see "Deployment Mode Constraint"):
        server croupier-server2:19090 backup; # agents reconnect to the backup after a primary failure
    }
    server {
        listen 19090;
        proxy_pass croupier_agent_lb;
        proxy_timeout 1h;                    # agent heartbeat keepalive at 30s; 1h idle fallback
        proxy_connect_timeout 3s;            # fast failover
    }
}
```

注意事项：

- upstream 主机名只在启动/reload 时解析；**server 实例重建换 IP 后**：`docker compose up -d --force-recreate dashboard`
- 被动探测：连接仍挂着的那种卡死实例摘不掉；靠 agent 侧心跳超时重连兜底

### HAProxy（本仓库默认，docker/configs/haproxy.cfg）

```haproxy
# /etc/haproxy/haproxy.cfg (the agent L4 entry point)
resolvers docker_dns
    nameserver dns1 127.0.0.11:53    # docker's embedded DNS; use the environment's DNS for bare metal
    resolve_retries 3
    timeout resolve 1s
    hold valid 10s                   # re-resolve every 10s; follows instance rebuilds changing IPs automatically

defaults
    mode tcp
    timeout connect 3s
    timeout client 1h                # agent long-lived sessions
    timeout server 1h

listen croupier_agent_lb
    bind *:19090
    balance leastconn
    # Active health check: TCP connect probe (the server's control listener)
    option tcp-check
    default-server inter 2s fall 3 rise 2 resolvers docker_dns init-addr libc,none
    # Active + cold standby (see "Deployment Mode Constraint"): server2 takes reconnections only after the primary is removed
    server server1 croupier-server:19090 check
    server server2 croupier-server2:19090 check backup

listen stats                        # debugging the connection distribution
    bind *:8404
    stats enable
    stats uri /stats
```

Dashboard 的 L7（API 反代 + SPA + SSE）仍归 nginx——两层分工：nginx 管「人」（HTTP），HAProxy 管「机器」（agent TCP）。

### keepalived + HAProxy（LB 本身的高可用）

```conf
# /etc/keepalived/keepalived.conf (the primary LB host; the backup has slightly lower priority, everything else identical)
vrrp_instance VI_CROUPIER {
    state MASTER
    interface eth0
    virtual_router_id 51
    priority 100
    advert_int 1
    virtual_ipaddress {
        10.0.0.100/24                # VIP: the single address agents/Dashboard connect to
    }
}
```

两台主机各跑一个 HAProxy（配置一致），keepalived 让 VIP 常驻主机；主机挂掉后 VIP 秒级漂到备机，agent 掉线重连自动落到备机——**agent 侧依然零改动**。

注意：VRRP 依赖网络层组播/单播支持，多数公有云 VPC 内不可用——云上等价物是**云 NLB**（跨 AZ + 健康检查 + 零运维），直接替代整套 keepalived+HAProxy 层。

## Agent 侧零改动原则

入口层无论怎么换形态，agent 都不需要改：单地址外连 + 断线退避重连 + 重新注册（归属随连接更新到新实例）。换 LB 只需把 `agent.yaml` 的 `server.addr` 指向新入口。

## 迁移路径（nginx stream → HAProxy）

1. 起新 HAProxy（指向既有 croupier-server/croupier-server2:19090）
2. 金丝雀：先指一台 agent 的 `server.addr` 到 HAProxy，验证注册与调用
3. 全量切换所有 agent；观察 HAProxy stats 的会话分布
4. 退役 nginx stream 块（dashboard 回归纯 L7）

全程 Server 集群无感（成员表按连接归属自动更新 owner）。

## LB 监控（设计决策）

> 状态：Accepted（已定，分期落地）。核心原则：**链路用开源生态，呈现走平台原生；数据源可配置、不绑定 LB 品牌。**

### 设计演进与结论

LB 监控方案经三轮讨论收敛：

1. ~~server 侧自研 LB stats 解析适配器（拉 haproxy CSV）~~——**否决**：重复造轮子，等价于重写一部分 haproxy exporter；且硬绑 haproxy——换 LB 即死
2. ~~Grafana 外链/iframe 内嵌~~——**否决为主方案**：体验割裂（第二个 UI）、权限绕过平台 RBAC（Grafana 要另配匿名/auth-proxy）
3. **定论**：采集与存储用 Prometheus exporter 生态（开源）；呈现层平台原生渲染（直读 Prometheus API + Ant Design Charts）；Grafana 仅作深度分析的外链兜底

### 分层架构

```text
HAProxy 2.4+ built-in exporter (zero new containers, one line in the cfg)
        → Prometheus (storage/history/aggregation/alerting, open-source TSDB)
        → Server read-only proxy endpoint (metric allowlist + platform RBAC auth consolidation)
        → Dashboard native chart page /ops/lb (the same pattern as dbmon: the server pulls the data, native rendering)
```

自建与不自建的边界：

| 层                 | 自建或开源           | 理由                                                   |
| ------------------ | -------------------- | ------------------------------------------------------ |
| 采集（exporter）   | 开源                 | 每种 LB 都有官方/社区 exporter；逐一跟进不现实也不应该 |
| 存储/查询/告警引擎 | 开源（Prometheus）   | 自研 TSDB 是纯重复劳动                                 |
| 查询代理 + 图表    | **自建（两层薄壳）** | 鉴权收口 + 体验一致；dbmon 页是同模式先例              |
| 深度分析面板       | 开源（Grafana 外链） | 平台只做日常对账，不克隆 Grafana                       |

### 开源 Exporter 对比

| LB         | 方案                                                                                                         | 接入成本       |
| ---------- | ------------------------------------------------------------------------------------------------------------ | -------------- |
| HAProxy    | 2.4+ 内置 Prometheus exporter（`http-request use-service prometheus-exporter`）——本仓库 haproxy 2.9 原生支持 | 极低（改 cfg） |
| nginx      | `nginx-prometheus-exporter`（nginxinc 官方）                                                                 | 低（sidecar）  |
| keepalived | `keepalived-exporter`（VRRP 状态/漂移）                                                                      | 低             |
| LVS/IPVS   | `prometheus-ipvs-exporter`（需宿主机权限）                                                                   | 中             |
| 云 NLB     | 云厂商 exporter（AWS yace / 阿里云 aliyun-exporter）                                                         | 中（云凭据）   |
| 通用兜底   | `blackbox_exporter`（TCP 探测入口可达性/时延——没有管理 API 也能用）                                          | 极低           |

官方 Grafana 面板：HAProxy 2（dashboard ID 12693）、NGINX（12708）——导入即用。

### 核心平台侧图表（原生渲染）

| 图表            | PromQL（现成）                                                         | 对账含义                     |
| --------------- | ---------------------------------------------------------------------- | ---------------------------- |
| 每后端会话分布  | `haproxy_backend_current_sessions`                                     | 与归属表 agentCount 交叉核对 |
| 后端健康状态    | `haproxy_server_status`                                                | UP/DOWN/NMA 实时可见         |
| 请求速率/错误率 | `haproxy_backend_http_requests_total` / `haproxy_backend_errors_total` | 流量异常发现                 |
| 健康检查失败    | `haproxy_server_check_failures`                                        | 半死实例发现                 |

**对账列（僵尸探测）**：`/ops/cluster` 每实例展示「LB 会话数 vs 归属表 agent 数」——两者长期不一致（连接活着、心跳停了）即半开连接信号，正是排查 HA 双实例时最缺的那一屏可见性。

### 可配置（不写死地址）

```yaml
# server.yaml — deployment topology facts, overridable via environment variables (the same pattern as the cluster block)
ops:
  prometheusUrl: http://prometheus:9090 # empty = the /ops/lb page is hidden (graceful degradation for single instance / no telemetry stack)
  grafanaUrl: http://grafana:3000 # optional; when set, a "deep analysis" external link appears
```

- 未配置 → 页面不出现，零依赖
- 换 LB / 加 keepalived → 只改 prometheus.yml 的 scrape job；平台侧零改动
- 配置归属：支撑层（ops 块），不是站点配置（业务运营开关）——沿用信号分层的决策

### 网络注意（部署）

遥测栈（`docker-compose.telemetry.yaml`）与部署栈（`docker-compose.deploy.yml`）是**两个独立 compose 网络**。Server 要访问 Prometheus，二选一：

1. 共享外部网络（声明 `croupier-telemetry` 为 external 并挂到部署栈）
2. `prometheusUrl` 指向宿主机映射端口（`http://<host>:19092`）

HAProxy exporter 端点只需集群内可达（scrape 走容器网络）；**不需要暴露宿主机端口**——顺带解决 stats :8404 暴露问题（stats 页可加 basic auth 或完全收回内网；平台代理走 exporter 通道）。

### 与 Agent 视角三方对账的关系（并行推进）

Prometheus 生态覆盖的是**支撑层**（LB 管理面数据）；**业务数据面事实**平台仍需自建（生态不覆盖）：

```text
Three-way reconciliation:
  LB view        : the agent's TCP session is on instance X (Prometheus / exporter)
  Ownership view : instance X claims this agent, heartbeat fresh (cluster_agent_owners)
  Agent view     : the agent self-reports "I'm connected to instance X" (the registration response carries instanceId, echoed by the heartbeat)
```

三者任一不一致的组合都能精确定位故障形态（半开连接 / 路由漂移 / 注册丢失）。agent 视角是零配置的通用实现（不依赖任何 LB 类型），与本节 Prometheus 链路互补。

### 落地阶段

| 阶段 | 内容                                                                                              | 依赖               |
| ---- | ------------------------------------------------------------------------------------------------- | ------------------ |
| P0   | haproxy.cfg 启用 exporter + telemetry prometheus.yml 加 scrape + 文档                             | 无（纯配置）       |
| P1   | `ops.prometheusUrl` 配置 + server 只读代理端点（指标 allowlist）+ `/ops/lb` 原生图表 + 集群对账列 | P0                 |
| P2   | agent 视角三方对账（注册响应带 instanceId + 心跳自报属主 + nodes 页对账列）                       | 无（独立于 P0/P1） |
| P3   | Grafana 外链配置 + blackbox_exporter 作为数据面探测标准                                           | P1                 |
