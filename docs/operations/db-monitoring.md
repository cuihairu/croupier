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

「运维中心 → 数据库监控」对游戏/平台的 MySQL、PostgreSQL 实例做**只读直连探测**（不是 proxy、无 Agent 旁路）：登记数据源 → 一键探测 → 快照式展示连接/锁/死锁，超阈值自动联动[告警中心](./alerts)。

## 数据源登记

字段（`POST /api/v1/dbmon/sources`）：

| 字段             | 说明                                                                       |
| ---------------- | -------------------------------------------------------------------------- |
| `name`           | 展示名                                                                     |
| `driver`         | `mysql` / `postgres`（当前仅这两种）                                       |
| `kind`           | `self`（自建）/ `aliyun` / `huawei`（云 RDS）                              |
| DSN              | **强制只读账号**——注册时校验拒绝 `root`/superuser，返回时脱敏为 `user:***` |
| `gameId` / `env` | 作用域                                                                     |
| `enabled`        | 停用后不参与探测                                                           |
| `lockWaitWarn`   | 锁等待告警阈值（秒，默认 5）                                               |
| `connWarnRatio`  | 连接水位告警阈值（百分比，默认 80）                                        |

## 探测指标（快照，无历史落库）

| 域     | 指标                                                                                 |
| ------ | ------------------------------------------------------------------------------------ |
| 连接   | 当前连接数 / `max_connections` / 活跃数（水位超 `connWarnRatio` 告警）               |
| 锁等待 | 阻塞链明细：等待 ID / 被谁阻塞 / 表 / 等待秒数（超 `lockWaitWarn` 告警）/ 截断的 SQL |
| 死锁   | 累计死锁数（自统计重置起）                                                           |
| 健康   | 探测延迟、OK/降级状态                                                                |

- MySQL 走 `information_schema` / `performance_schema`；PostgreSQL 走 `pg_stat_*`
- 云 RDS 不暴露的指标记 **degraded**（页面 Tooltip 说明），不猜测
- 探测 5s 超时；不可达的数据源返回明确错误而非报 500

## 阈值联动

探测时 `raiseAlertsIfNeeded`：锁等待超阈值 → 告警；连接水位超阈值 → 告警。产生的告警进入告警中心统一流转（等级、静默、通知渠道与告警规则一致）。

## 运维建议

- **给监控专用账号**：`SELECT` ON information_schema/performance_schema（PG 另需 `pg_stat_database`）即可，永远不要用业务写账号
- 探测是快照不是趋势：容量规划用外部时序库（Prometheus exporter）长跑，本页解决「现在是不是有锁/连接打满」的即时判断
- 游戏库逐个登记（`multiGame` 模式下每 `(game_id, env)` 一库），配合作用域过滤
