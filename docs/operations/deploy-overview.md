---
title: 部署形态总览
icon: rocket
order: 2
category:
  - 运维手册
tag:
  - 部署
---

# 部署形态总览

Croupier 是「单控制面 + 旁路 Agent」架构：Server 无本地持久状态（状态全部在数据库/Redis），因此部署形态的选择本质上只有两个问题——**几个 Server 实例**、**入口层怎么做**。

## 形态对比

| 形态                                          | 适用                    | 可用性       | 入口层                                 | 参考文档                               |
| --------------------------------------------- | ----------------------- | ------------ | -------------------------------------- | -------------------------------------- |
| 单机 Compose（`docker/docker-compose.yml`）   | 开发、测试、小团队试点  | 单点         | 无（直连 18780/19090）                 | [Docker Compose 部署](./deploy-docker) |
| 单机 HA（`docker/docker-compose.deploy.yml`） | 单宿主生产              | 99.9%+       | nginx（L7）+ HAProxy（L4）             | [Docker Compose 部署](./deploy-docker) |
| 多宿主                                        | 跨机生产、LB 不允许单点 | 99.99%       | HAProxy 主备（keepalived VIP）或云 NLB | [负载均衡](./load-balancing)           |
| Kubernetes                                    | 已有 K8s 基础设施       | 取决于副本数 | Service（ClusterIP）+ 探针             | [Kubernetes 部署](./deploy-kubernetes) |
| 二进制 + systemd                              | 无容器约束的环境        | 取决于拓扑   | 自建 nginx/HAProxy                     | [二进制部署](./deploy-binary)          |

## 组件清单

最小可用的控制面组件：

- **Server**（必须）：HTTP API `:18780` + transport 入口 `:19090`
- **数据库**（必须）：MySQL / PostgreSQL（生产推荐）；SQLite 仅开发
- **Redis**（生产必须）：缓存、集群成员表、注册表共享状态；单机 `memory` 模式可用但不推荐
- **Agent**（按游戏网络部署）：每个游戏网络至少一个，出站连接 Server
- 可选：`analytics-worker` / `ingest` / ClickHouse（数据分析域）、`dashboard`（前端静态资源）

## 选型决策

```
需要高可用吗？
├─ 否 → 单机 Compose（接受 Server 重启期间的平台中断）
└─ 是 → 宿主环境是什么？
    ├─ 单台宿主 → docker-compose.deploy.yml（双 Server + 双 LB 容器）
    ├─ 多台宿主 → HAProxy 主备 + keepalived VIP（或云 NLB）
    └─ K8s → Deployment 2 副本 + Service（kube-proxy 天然 L4 打散）
```

关键事实（详见 [Server 多实例 HA](../architecture/server-ha-multi-instance.md)）：

- Server **无状态**：任何实例崩溃不丢数据，恢复即回归集群（成员表租约自动续期/剔除）
- Agent 断线重连 + 重新注册即可把归属迁移到存活实例，**入口层换型 Agent 零改动**（只改 `server.addr`）
- HA 不是「双写」：多实例通过共享存储成员表 + owner 转发协同，`cluster.enabled` 是唯一开关
