---
title: 数据库监控
icon: database
order: 22
category:
  - 运维手册
tag:
  - 可观测性
  - 数据库
---

# 数据库监控

「运营中心 → 数据库监控」对游戏/平台的 MySQL 与 PostgreSQL 实例做**只读直连探测**（不是代理、不走 Agent）：登记数据源 → 一键探测 → 快照式展示连接/锁等待/死锁，超阈值自动进告警中心。

## 数据源登记

字段（`POST /api/v1/dbmon/sources`）：

| 字段             | 说明                                                                      |
| ---------------- | ------------------------------------------------------------------------- |
| `name`           | 显示名                                                                    |
| `driver`         | `mysql` / `postgres`（目前仅这两种）                                      |
| `kind`           | `self`（自建）/ `aliyun` / `huawei`（云 RDS）                             |
| DSN              | **必须只读账号**——登记时校验并拒绝 `root`/超级用户；回显脱敏为 `user:***` |
| `gameId` / `env` | 归属范围                                                                  |
| `enabled`        | 停用的源不探测                                                            |
| `lockWaitWarn`   | 锁等待告警阈值（秒，默认 5）                                              |
| `connWarnRatio`  | 连接水位告警阈值（百分比，默认 80）                                       |

## 探测指标（快照，不落历史）

| 域     | 指标                                                                                   |
| ------ | -------------------------------------------------------------------------------------- |
| 连接   | 当前连接数 / `max_connections` / 活跃数（水位超过 `connWarnRatio` 告警）               |
| 锁等待 | 阻塞链明细：等待方 ID / 被谁阻塞 / 表 / 等待秒数（超过 `lockWaitWarn` 告警）/ 截断 SQL |
| 死锁   | 累计死锁次数（自统计重置起）                                                           |
| 健康   | 探测时延、OK/degraded 状态                                                             |

- MySQL 用 `information_schema` / `performance_schema`；PostgreSQL 用 `pg_stat_*`
- 云 RDS 未暴露的指标标记 **degraded**（页面 tooltip 说明原因），绝不猜测
- 探测 5s 超时；不可达数据源返回明确错误而不是 500

## 阈值联动

探测时执行 `raiseAlertsIfNeeded`：锁等待超阈值 → 告警；连接水位超阈值 → 告警。产出的告警与其他告警一样走告警中心（同等级、同静默、同通知渠道）。

## 运营建议

- **用监控专号**：`SELECT` ON information_schema/performance_schema（PG 还需 `pg_stat_database`）即可；绝不用业务写账号
- 探测是快照不是趋势：容量规划请另起长效外部时序存储（Prometheus exporter）；本页回答的是「现在有没有锁/连接是否打满」的即时问题
- 游戏库逐个登记（`multiGame` 模式下每 `(game_id, env)` 一个库），配合归属范围过滤
