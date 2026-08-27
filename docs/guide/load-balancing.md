---
title: 负载均衡
icon: deployment
order: 5
category:
  - 入门指南
tag:
  - 部署
  - 高可用
---

# 负载均衡

Croupier HA 多实例架构（[Server 多实例 HA](../architecture/server-ha-multi-instance.md)）有两个流量入口，各自需要负载均衡：

- **Dashboard API（L7 / HTTP）**：Dashboard 前置网关分流到各 Server 实例 `:18780`
- **Agent 接入（L4 / TCP）**：Agent 的自研 transport 长连接（`:19090`，长度前缀分帧 + 多路复用）打散到各 Server 实例

**为什么 Agent 必须经 L4 LB 而不是直连**：直连会导致实例故障时该实例名下所有 Agent 失联需人工干预；经 LB 打散后，断连重连自动分发到存活实例 + 重新注册更新 owner（架构文档 §6 故障转移时间线，全程无人工干预）。

## 背景概念：L4 / L7 / VRRP

### L4 / L7 按协议分层定义

L4/L7 指的是 OSI 模型中的协议层次——负载均衡器**工作在哪一层，决定了它看得见什么、能按什么分发**：

| 层     | OSI 名称 | LB 看得见什么                               | 分发依据                         | 典型实现                                          |
| ------ | -------- | ------------------------------------------- | -------------------------------- | ------------------------------------------------- |
| **L4** | 传输层   | IP + 端口（TCP/UDP 头），**不解析**应用内容 | 五元组哈希、最少连接             | LVS、HAProxy（`mode tcp`）、nginx stream、云 NLB  |
| **L7** | 应用层   | 完整 HTTP 请求（方法/路径/Header/Cookie）   | 路径路由、Header、会话粘滞、灰度 | nginx http、HAProxy（`mode http`）、Envoy、云 ALB |

两者的取舍：

- **L4 透传字节流**：协议无关（自研协议也能跑）、长连接天然友好、开销低；但看不见请求内容，只能按「连接」粒度分发，也无法做基于内容的路由
- **L7 理解应用语义**：能按路径/Header 分流、改写、重试、灰度；代价是必须解析（甚至终结）应用层协议

对 Croupier 的映射：Dashboard API 是标准 HTTP，天然适合 **L7**；Agent 用的是自研二进制分帧协议（长度前缀 + protobuf），LB 无法也无需理解，**L4 透传**即可——这也是本文两个入口分层选型的依据。

> 顺带一提 **LVS**（Linux Virtual Server）：工作在内核态的 L4 转发（IPVS），不经过用户态协议栈，性能是所有方案中最高的；代价是配置与生态复杂度，单公司千级节点规模内通常用不到。

### VRRP：解决「LB 自己是单点」

**VRRP（Virtual Router Redundancy Protocol，RFC 5798）** 是一个**网络层高可用协议**，与负载均衡本身无关：

- 多台主机组成一个虚拟路由器，共享一个**虚拟 IP（VIP）**；对外只暴露 VIP，客户端永远连 VIP
- 协议自动选举 **Master**（按 priority 最高者）持有 VIP 并应答流量，其余为 **Backup** 待命
- Master 周期性广播通告（默认 1s 一次）；Backup 连续约 3 个周期收不到通告即认定 Master 死亡，按 priority 接管 VIP——**秒级故障转移，客户端无感知**
- **keepalived** 是 Linux 上 VRRP 的守护进程实现，常与 nginx/HAProxy 成对部署：LB 进程做分发，keepalived 保证「跑 LB 的这台机器」挂了 VIP 能漂到备机

注意 VRRP 依赖二层组播/单播可达，**多数公有云 VPC 内不可用**——云上的等价物是云 NLB（跨可用区 + 健康检查 + 免运维）。

## 概念澄清：keepalived 不是负载均衡器

三者的关系经常被混淆：

- **nginx（stream 模块）**、**HAProxy**：负载均衡器，真正分发流量
- **keepalived**：VRRP 守护进程，只负责 **VIP 漂移**（主备高可用），本身不负载均衡；通常与 nginx/HAProxy 成对部署，解决「LB 自己挂了」的问题
- 第三种自建高可用 LB 形态是 **keepalived + LVS**（内核态转发，性能最高）

## 方案对比

### L4 能力对比（nginx stream vs HAProxy）

| 维度              | nginx stream                                                                                          | HAProxy                                                                       | 对 Croupier 的影响                                                                                           |
| ----------------- | ----------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| 运行时 DNS 重解析 | ❌ upstream 域名只在启动/reload 时解析；实例重建换 IP 后旧 IP 仍被使用（502/连接失败），需手动 reload | ✅ `resolvers` 块 + `resolve-prefer ipv4`，容器 IP 变动自动跟随               | **最关键差异**：compose 重建 server 容器后，nginx 需 `--force-recreate dashboard` 兜底，HAProxy 无需任何动作 |
| 主动健康检查      | ❌ 开源版被动探测（`max_fails`/`fail_timeout`，连接失败才累计标记）                                   | ✅ `option tcp-check` / `http-check` 主动探活，摘除半死实例                   | Agent 是长连接多路复用：Server 进程僵死但 TCP 连接未断时，被动探测摘不干净，调用会持续打到死实例             |
| 优雅下线          | ❌ reload 直接断存量连接                                                                              | ✅ stats socket 运行时 `set server ... state drain`：存量连接排空、新连接停入 | Server 实例滚动升级时，HAProxy 可零断连切换                                                                  |
| 可观测性          | access log 为主                                                                                       | ✅ 内置 stats 页：每后端会话数/队列/错误率实时可见                            | 排查「agent 连不上/连不均」时直接看图                                                                        |
| 性能              | epoll，数万并发连接                                                                                   | epoll，同量级；均低于 LVS（内核态）                                           | 单公司多游戏（千级节点内）**两者都不是瓶颈**，不必纠结                                                       |
| 组件成本          | ✅ 复用 dashboard 镜像，零新增                                                                        | ❌ 新增组件（镜像/配置/监控面）                                               | 小团队 nginx 的核心优势                                                                                      |
| L7 能力           | ✅ SPA 静态资源 + API 反代 + SSE 调优一体                                                             | ✅ 但静态资源服务弱于 nginx                                                   | Dashboard 已经需要 nginx 服务静态资源                                                                        |

### 结论

> **就 L4 负载均衡职能而言，HAProxy 全面占优**（运行时重解析 + 主动健康检查 + 优雅下线 + 可观测性）；nginx stream 唯一优势是**复用现有 dashboard 容器、零新增组件**。

推荐按部署形态选择：

| 部署形态                                    | 推荐                                                                | 理由                                                                             |
| ------------------------------------------- | ------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| 单机 compose（`docker-compose.deploy.yml`） | **HAProxy**（现状，独立 `haproxy` 服务）                            | 统一形态：L4 能力完整（重解析/健康检查/stats），单机与多宿主无需两套配置         |
| 多宿主容器化生产                            | **HAProxy**（agent L4 入口）                                        | IP 变动自动跟随 + 主动摘除僵死实例 + stats 排查；dashboard 的 L7 仍由 nginx 承担 |
| LB 自身不允许单点                           | **keepalived + HAProxy**（主备 VIP 漂移）；公有云等价物是**云 NLB** | VRRP 需要网络层支持，公有云内用 NLB 免运维                                       |
| Kubernetes                                  | **Service（ClusterIP）**                                            | kube-proxy 天然负载均衡 + Endpoints 就绪探针，无需自建 LB 层                     |

## 配置示例

### nginx stream（备选：复用 dashboard 容器）

```nginx
# docker/nginx-main.conf（部署产物的一部分）
stream {
    upstream croupier_agent_lb {
        least_conn;                          # 按活跃会话数，贴合长连接
        server croupier-server:19090;
        server croupier-server2:19090;
    }
    server {
        listen 19090;
        proxy_pass croupier_agent_lb;
        proxy_timeout 1h;                    # agent 心跳 30s 保活，1h 空闲兜底
        proxy_connect_timeout 3s;            # 故障快速切换
    }
}
```

注意事项：

- upstream 域名只在启动/reload 解析；**server 实例重建换 IP 后**：`docker compose up -d --force-recreate dashboard`
- 被动探测：实例僵死但连接未断时不会被摘除，依赖 agent 侧心跳超时重连兜底

### HAProxy（本仓库默认，docker/configs/haproxy.cfg）

```haproxy
# /etc/haproxy/haproxy.cfg（agent L4 入口）
resolvers docker_dns
    nameserver dns1 127.0.0.11:53    # docker 内嵌 DNS；裸机部署换成环境 DNS
    resolve_retries 3
    timeout resolve 1s
    hold valid 10s                   # 10s 重解析，实例重建换 IP 自动跟随

defaults
    mode tcp
    timeout connect 3s
    timeout client 1h                # agent 长连接会话
    timeout server 1h

listen croupier_agent_lb
    bind *:19090
    balance leastconn
    # 主动健康检查：TCP 建连探活（server 的 control listener）
    option tcp-check
    default-server inter 2s fall 3 rise 2 resolvers docker_dns init-addr libc,none
    server server1 croupier-server:19090 check
    server server2 croupier-server2:19090 check

listen stats                        # 排查连接分布
    bind *:8404
    stats enable
    stats uri /stats
```

Dashboard 的 L7（API 反代 + SPA + SSE）仍由 nginx 承担——两层各司其职：nginx 管「人」（HTTP），HAProxy 管「机器」（agent TCP）。

### keepalived + HAProxy（LB 自身高可用）

```conf
# /etc/keepalived/keepalived.conf（主 LB 宿主；备机 priority 略低、其余相同）
vrrp_instance VI_CROUPIER {
    state MASTER
    interface eth0
    virtual_router_id 51
    priority 100
    advert_int 1
    virtual_ipaddress {
        10.0.0.100/24                # VIP：agent/Dashboard 统一连这个地址
    }
}
```

两台宿主各跑一个 HAProxy（配置完全相同），keepalived 维持 VIP 在主上；主挂了 VIP 秒级漂移到备，agent 断连重连自动到备——**agent 侧依然零改动**。

注意：VRRP 依赖网络层组播/单播支持，多数公有云 VPC 内不可用——云上等价方案是**云 NLB**（跨可用区 + 健康检查 + 免运维），直接替代 keepalived+HAProxy 整层。

## Agent 侧零改动原则

无论入口层怎么换型，Agent 不需要任何改动：单地址连接 + 断线退避重连 + 重新注册（owner 随之更新到新实例）。换 LB 只需改 `agent.yaml` 的 `server.addr` 指向新入口。

## 迁移路径（nginx stream → HAProxy）

1. 起新 HAProxy（指向现有 croupier-server/croupier-server2:19090）
2. 灰度：先改一个 agent 的 `server.addr` 指向 HAProxy，验证注册与调用正常
3. 全量 agent 切换；观察 HAProxy stats 会话分布
4. nginx stream 块下线（dashboard 回归纯 L7）

整个过程 Server 集群无感知（成员表按连接归属自动更新 owner）。
