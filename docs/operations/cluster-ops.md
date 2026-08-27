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

「运维中心 → 集群拓扑」是 Server 多实例 HA 的运维观察窗（需 `cluster.enabled`）。单实例部署时页面显示一条 `standalone` 记录。

## 页面结构

`GET /api/v1/ops/cluster` 返回：

| 字段                            | 说明                                                                             |
| ------------------------------- | -------------------------------------------------------------------------------- |
| `enabled`                       | 集群是否开启                                                                     |
| `self` / `aliveCount` / `total` | 当前实例 / 在线数 / 总数                                                         |
| `items[].instanceId`            | 实例标识                                                                         |
| `items[].advertiseAddr`         | 实例互联地址                                                                     |
| `items[].epoch`                 | fencing 代次（防脑裂，见[HA 架构](../architecture/server-ha-multi-instance.md)） |
| `items[].startedAt`             | 启动时间                                                                         |
| `items[].agentCount`            | 该实例名下 Agent 连接数（归属表统计）                                            |
| `items[].alive`                 | 在线 = 租约未过期                                                                |

## 成员状态机

```
实例启动 → 自注册成员表（advertiseAddr + 租约）
        → 心跳续期（cluster.heartbeatInterval，默认 5s）
        → 租约 TTL（cluster.leaseTtl，默认 15s ≈ 3 倍心跳）内无心跳 → 展示离线
        → 恢复回来 → 重新注册（epoch + 1，旧代次请求被 fencing 拒绝）
```

Agent 归属：`cluster.ownerTtl`（默认 3m）内 Agent 心跳续期归属；Agent 断连即释放，重连到哪个实例归属就跟着走——**页面上的 agent 分布是实时归属，不是静态配置**。

## 常见运维场景

### 实例离线了怎么办

1. 拓扑页确认离线实例（`alive=false` + 租约过期 Tooltip）
2. 该实例名下 Agent 已自动重连到存活实例（对比前后 agentCount 分布）
3. 修复实例（看日志：迁移/端口/网络）后重启即自动回归，无需手工「摘除成员」——租约机制天然清理
4. 在途任务：实例死亡时其名下 running 任务由 reconciliation 标记失败/超时，需按业务语义补偿重试

### 滚动升级

见[版本升级与回滚](./upgrade-rollback)——逐台摘流量 → 替换 → 健康 → 回流量，拓扑页全程可观察成员进出。

### 脑裂疑虑

epoch fencing 保证分区恢复的旧主不再下发调用（对端拒绝低 epoch 请求）。若拓扑页出现两个实例互报 online 但调用报 `ErrNotOwner`/epoch 错误，检查共享存储（Redis/DB）连通性与系统时钟偏移。

### 分布不均

Agent 打散由 L4 LB 决定（leastconn），不均是正常的（长连接存量）；只在「某实例归零但 Agent 总数正常」时才需要排查 LB 健康检查。

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
