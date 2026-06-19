---
title: ClickHouse 表结构与物化聚合
---
# 表结构（DDL）

**数据库架构**：按游戏分库，每个游戏独立 ClickHouse 数据库。

```
croupier_meta (元数据库)
├─ games
└─ game_envs

game_demo_prod (游戏数据库)
├─ events
├─ payments
├─ minute_online
├─ daily_users
├─ daily_revenue
└─ daily_online_peak

game_demo_staging
└─ (同样的表结构)

game_rpg_prod
└─ ...
```

**注意**：`game_id` 和 `env` 已在数据库/表名称中体现，不需要作为字段存储。

## 事件明细（events）

```sql
-- game_demo_prod.events (game_id 和 env 在数据库名称中)
CREATE TABLE IF NOT EXISTS game_demo_prod.events (
  event_time DateTime DEFAULT now(),
  user_id String,
  session_id String,
  event LowCardinality(String),
  channel LowCardinality(String),
  platform LowCardinality(String),
  country FixedString(2),
  app_version String,
  event_id UUID,
  server_id LowCardinality(String), -- MMORPG 多服务器支持
  props_json String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (event, server_id, user_id, event_time);
```

## 支付明细（payments）

```sql
-- game_demo_prod.payments (game_id 和 env 在数据库名称中)
CREATE TABLE IF NOT EXISTS game_demo_prod.payments (
  time DateTime DEFAULT now(),
  user_id String,
  order_id String,
  amount_cents UInt64,
  currency FixedString(3),
  status LowCardinality(String),
  channel LowCardinality(String),
  platform LowCardinality(String),
  country FixedString(2),
  region LowCardinality(String),
  city String,
  product_id LowCardinality(String),
  reason String,
  server_id LowCardinality(String) -- MMORPG 多服务器支持
) ENGINE = MergeTree
PARTITION BY toYYYYMM(time)
ORDER BY (time, order_id);
```

## 分钟在线（minute_online）

```sql
-- game_demo_prod.minute_online
CREATE TABLE IF NOT EXISTS game_demo_prod.minute_online (
  m DateTime,
  server_id LowCardinality(String),
  online UInt32
) ENGINE = MergeTree
PARTITION BY toYYYYMM(m)
ORDER BY (server_id, m);
```

## 日活/新增（daily_users，ReplacingMergeTree）

```sql
-- game_demo_prod.daily_users
CREATE TABLE IF NOT EXISTS game_demo_prod.daily_users (
  d Date,
  server_id LowCardinality(String),
  dau UInt64,
  new_users UInt64,
  version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(d)
ORDER BY (server_id, d);
```

## 日收入（daily_revenue，ReplacingMergeTree）

```sql
-- game_demo_prod.daily_revenue
CREATE TABLE IF NOT EXISTS game_demo_prod.daily_revenue (
  d Date,
  server_id LowCardinality(String),
  revenue_cents UInt64,
  refunds_cents UInt64,
  failed UInt64,
  version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(d)
ORDER BY (server_id, d);
```

## 日峰值在线（物化视图）

```sql
-- game_demo_prod.daily_online_peak
CREATE TABLE IF NOT EXISTS game_demo_prod.daily_online_peak (
  d Date,
  server_id LowCardinality(String),
  peak_online AggregateFunction(max, UInt32)
) ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(d)
ORDER BY (server_id, d);

CREATE MATERIALIZED VIEW IF NOT EXISTS game_demo_prod.daily_online_peak_mv
TO game_demo_prod.daily_online_peak AS
SELECT toDate(m) AS d, server_id, maxState(online) AS peak_online
FROM game_demo_prod.minute_online
GROUP BY d, server_id;
```

# Ingestion 字段映射

由于按游戏分库，SDK 上报时仍然需要携带 `game_id` 和 `env` 用于路由，但在写入 ClickHouse 时这些信息在数据库/表层级已体现：

- 事件写入（game_demo_prod.events）
  - 从上报 JSON 中映射：`ts -> event_time (RFC3339)`、`user_id`、`session_id`、`event`、`channel`、`platform`、`country`、`app_version`、`event_id`、`server_id`；其余作为 `props_json`
  - `game_id` 和 `env` 用于路由到正确的数据库，不作为字段存储

- 支付写入（game_demo_prod.payments）
  - `ts -> time`、`user_id`、`order_id`、`amount_cents`、`currency`、`status`、`channel`、`platform`、`country`、`region`、`city`、`product_id`、`reason`、`server_id`
  - `game_id` 和 `env` 用于路由到正确的数据库，不作为字段存储

- 分钟在线与日活/新增
  - Worker 使用 Redis HyperLogLog 统计分钟在线（heartbeat/session_start）和 DAU/新增（login/register/first_active），周期性落入对应游戏数据库

# 示例查询

**注意**：由于按游戏分库，查询时不需要过滤 `game_id` 和 `env`。

- 最近 7 天 DAU/New
```sql
SELECT d, server_id, dau, new_users
FROM game_demo_prod.daily_users
WHERE d >= today() - 7
ORDER BY d, server_id;
```

- 最近 7 天收入（元）
```sql
SELECT d, server_id, revenue_cents/100.0 AS revenue, refunds_cents/100.0 AS refunds
FROM game_demo_prod.daily_revenue
WHERE d >= today() - 7
ORDER BY d, server_id;
```

- 峰值在线（聚合状态求值）
```sql
SELECT d, server_id, maxMerge(peak_online) AS peak_online
FROM game_demo_prod.daily_online_peak
WHERE d >= today() - 7
GROUP BY d, server_id
ORDER BY d, server_id;
```

- 事件漏斗示例（进入->完成）
```sql
WITH
  (SELECT count() FROM game_demo_prod.events
   WHERE event_time >= now() - INTERVAL 7 DAY
   AND event='level.start') AS starts,
  (SELECT count() FROM game_demo_prod.events
   WHERE event_time >= now() - INTERVAL 7 DAY
   AND event='level.complete') AS completes
SELECT starts, completes, completes/starts AS cr;
```

- 按 server_id 统计 DAU（MMORPG 多服务器场景）
```sql
SELECT server_id, avg(dau) AS avg_dau
FROM game_demo_prod.daily_users
WHERE d >= today() - 30
GROUP BY server_id
ORDER BY avg_dau DESC;
```

# 性能与治理建议

- 低基数字段使用 LowCardinality（已在 DDL 使用）
- 按月分区、合理 ORDER BY（已在 DDL 使用）
- ReplacingMergeTree + version 字段用于"幂等/更新"写入
- 高基数字段优先放入 props_json，避免维度爆炸；对分析常用字段正式列化
- **按游戏分库的优势**：
  - 物理隔离，完全独立
  - 独立的容量规划和扩展
  - 简化查询（不需要 WHERE game_id = 'xxx' AND env = 'prod'）
  - 便于游戏迁移和归档
  - 符合"通用平台 + 独立游戏数据"的理念

# 数据库路由

网关需要根据 `game_id + env` 路由到对应的 ClickHouse 数据库：

```yaml
路由规则:
  - game_id: "demo"
    env: "prod"
    database: "game_demo_prod"
  - game_id: "demo"
    env: "staging"
    database: "game_demo_staging"
  - game_id: "rpg"
    env: "prod"
    database: "game_rpg_prod"
```

SDK 上报时仍然需要携带 `game_id` 和 `env` 用于路由，但在存储层这些信息在数据库/表层级已体现。
