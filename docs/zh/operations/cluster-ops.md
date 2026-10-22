---
title: 集群拓扑与 HA 运维
icon: cluster
order: 32
category:
  - 运维手册
tag:
  - 高可用
  - 集群
---

# 集群拓扑与 HA 运维

「运营中心 → 集群拓扑」是多实例 Server HA（需 `cluster.enabled`）的运维观测窗口。单实例部署时页面显示一条 `standalone` 记录。

## 页面结构

`GET /api/v1/ops/cluster` 返回：

| 字段                            | 说明                                         |
| ------------------------------- | -------------------------------------------- |
| `enabled`                       | 是否启用集群                                 |
| `self` / `aliveCount` / `total` | 当前实例 / 在线数 / 总数                     |
| `items[].instanceId`            | 实例标识                                     |
| `items[].advertiseAddr`         | 实例互联地址                                 |
| `items[].epoch`                 | fencing 代际（防脑裂，见 HA 架构（仅中文）） |
| `items[].startedAt`             | 启动时间                                     |
| `items[].agentCount`            | 本实例持有的 Agent 连接数（归属表统计）      |
| `items[].alive`                 | 在线 = 租约未过期                            |

## 成员状态机

```
Instance starts → self-registers in the member table (advertiseAddr + lease)
        → heartbeat renewal (cluster.heartbeatInterval, default 5s)
        → no heartbeat within the lease TTL (cluster.leaseTtl, default 15s ≈ 3 heartbeats) → shown as offline
        → comes back → re-registers (epoch + 1; requests from the old generation are rejected by fencing)
```

Agent 归属：`cluster.ownerTtl`（默认 3m）内 Agent 通过心跳续归属；Agent 断开即释放归属，重连到哪台就归属哪台——**页面上的 agent 分布是实时归属，不是静态配置**。

## 常见运维场景

### 实例离线怎么办

1. 拓扑页确认离线实例（`alive=false` + 租约过期 tooltip）
2. 该实例名下的 Agent 已自动重连到存活实例（对比前后 agentCount 分布）
3. 修复实例（查日志：迁移/端口/网络）后重启即可自动重新加入——不需要手工「摘除成员」，租约机制会自然清理
4. 在途任务：实例死亡时其在跑任务由对账机制标记失败/超时，按业务语义补偿重试

### 滚动升级

见版本升级与回滚——逐实例 drain 流量 → 替换 → 健康 → 还流；拓扑页全程围观成员进出。

### 脑裂担忧

epoch fencing 保证分区前的旧主不再下发调用（对端拒绝低 epoch 请求）。如果拓扑页显示两个实例互报在线而调用报 `ErrNotOwner`/epoch 错误，检查共享存储（Redis/DB）的连通性与系统时钟偏移。

### 分布不均

Agent 扎堆由 L4 LB（leastconn）决定，不均属正常（长连接库存）。只有「一个实例 0 agent 而 Agent 总数正常」才需要查 LB 健康检查。

## 相关配置

| 键                          | 默认     | 说明                                |
| --------------------------- | -------- | ----------------------------------- |
| `cluster.enabled`           | `false`  | HA 总开关（单实例零开销）           |
| `cluster.advertiseAddr`     | 必填*    | 对端可达的互联地址（K8s 用 POD_IP） |
| `cluster.instanceId`        | 自动生成 | 多实例建议显式命名                  |
| `cluster.heartbeatInterval` | `5s`     | 成员心跳                            |
| `cluster.leaseTtl`          | `15s`    | 租约 TTL（建议 3 倍心跳）           |
| `cluster.ownerTtl`          | `3m`     | Agent 归属存活窗口                  |
| `cluster.peerPollInterval`  | `10s`    | 对端发现轮询                        |
