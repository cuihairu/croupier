---
title: 部署拓扑概览
icon: rocket
order: 2
category:
  - 运维手册
tag:
  - 部署
---

# 部署拓扑概览

Croupier 是「单一控制面 + sidecar Agent」架构：Server 本身没有本地持久状态（全部状态在数据库/Redis），因此部署拓扑的选择归结为两个问题——**Server 起几个实例**、**入口层怎么搭**。

## 拓扑对比

| 拓扑                                          | 适用场景                | 可用性       | 入口层                                 | 参考文档                      |
| --------------------------------------------- | ----------------------- | ------------ | -------------------------------------- | ----------------------------- |
| 单机 Compose（`docker/docker-compose.yml`）   | 开发、测试、小规模试点  | 单点         | 无（直连 18780/19090）                 | Docker Compose 部署（仅中文） |
| 单机 HA（`docker/docker-compose.deploy.yml`） | 单机生产                | 99.9%+       | nginx（L7）+ HAProxy（L4）             | Docker Compose 部署（仅中文） |
| 多机                                          | 跨机生产，LB 不允许单点 | 99.99%       | HAProxy 主备（keepalived VIP）或云 NLB | 负载均衡                      |
| Kubernetes                                    | 已有 K8s 基础设施       | 取决于副本数 | Service（ClusterIP）+ 探针             | Kubernetes 部署               |
| 二进制 + systemd                              | 无容器约束的环境        | 取决于拓扑   | 自管 nginx/HAProxy                     | 二进制部署                    |

## 组件清单

控制面最小可用组件：

- **Server**（必需）：HTTP API `:18780` + transport 入口 `:19090`
- **数据库**（必需）：MySQL / PostgreSQL（生产推荐）；SQLite 仅限开发
- **Redis**（生产必需）：缓存、集群成员表、注册中心共享状态；单机 `memory` 模式可用但不推荐
- **Agent**（按游戏网络部署）：每个游戏网络至少一个，向 Server 主动外连
- 可选：`analytics-worker` / `ingest` / ClickHouse（数据分析域）、`dashboard`（前端静态资源）

## 决策指引

```
Is high availability needed?
├─ No → single-host Compose (accept platform downtime during Server restarts)
└─ Yes → what is the hosting environment?
    ├─ Single host → docker-compose.deploy.yml (two Servers + two LB containers)
    ├─ Multiple hosts → HAProxy active/standby + keepalived VIP (or cloud NLB)
    └─ K8s → Deployment with 2 replicas + Service (kube-proxy spreads L4 naturally)
```

关键事实（详见 Server 多实例 HA（仅中文））：

- Server 是**无状态**的：任一实例宕机不丢数据，恢复后自动重新加入集群（成员表租约自动续期/被淘汰）
- Agent 重连重注册后会把归属迁移到存活实例；**切换入口层不需要动 Agent**（只改 `server.addr`）
- HA 不是「双写」：多实例靠共享存储的成员表 + owner 转发协调；`cluster.enabled` 是唯一开关
