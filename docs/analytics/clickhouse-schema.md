---
title: ClickHouse 表结构与物化聚合
---
# 表结构（DDL）

事件明细（events）
```sql
CREATE TABLE IF NOT EXISTS analytics.events (
  event_time DateTime DEFAULT now(),
  game_id LowCardinality(String),
  env LowCardinality(String),
  user_id String,
  session_id String,
  event LowCardinality(String),
  channel LowCardinality(String),
  platform LowCardinality(String),
  country FixedString(2),
  app_version String,
  event_id UUID,
  props_json String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (game_id, env, event, user_id, event_time);
```

支付明细（payments）
```sql
CREATE TABLE IF NOT EXISTS analytics.payments (
  time DateTime DEFAULT now(),
  game_id LowCardinality(String),
  env LowCardinality(String),
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
  reason String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(time)
ORDER BY (game_id, env, time, order_id);
```

分钟在线（minute_online）
```sql
CREATE TABLE IF NOT EXISTS analytics.minute_online (
  m DateTime,
  game_id LowCardinality(String),
  env LowCardinality(String),
  online UInt32
) ENGINE = MergeTree
PARTITION BY toYYYYMM(m)
ORDER BY (game_id, env, m);
```

日活/新增（daily_users，ReplacingMergeTree）
```sql
CREATE TABLE IF NOT EXISTS analytics.daily_users (
  d Date,
  game_id LowCardinality(String),
  env LowCardinality(String),
  dau UInt64,
  new_users UInt64,
  version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(d)
ORDER BY (game_id, env, d);
```

日收入（daily_revenue，ReplacingMergeTree）
```sql
CREATE TABLE IF NOT EXISTS analytics.daily_revenue (
  d Date,
  game_id LowCardinality(String),
  env LowCardinality(String),
  revenue_cents UInt64,
  refunds_cents UInt64,
  failed UInt64,
  version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(d)
ORDER BY (game_id, env, d);
```

日峰值在线（物化视图）
```sql
CREATE TABLE IF NOT EXISTS analytics.daily_online_peak (
  d Date,
  game_id LowCardinality(String),
  env LowCardinality(String),
  peak_online AggregateFunction(max, UInt32)
) ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(d)
ORDER BY (game_id, env, d);

CREATE MATERIALIZED VIEW IF NOT EXISTS analytics.daily_online_peak_mv
TO analytics.daily_online_peak AS
SELECT toDate(m) AS d, game_id, env, maxState(online) AS peak_online
FROM analytics.minute_online
GROUP BY d, game_id, env;
```

# Ingestion 字段映射
- 事件写入（analytics.events）
  - 从上报 JSON 中映射：`ts -> event_time (RFC3339)`、`game_id`、`env`、`user_id`、`session_id`、`event`、`channel`、`platform`、`country`、`app_version`、`event_id`；其余作为 `props_json`
- 支付写入（analytics.payments）
  - `ts -> time`、`game_id`、`env`、`user_id`、`order_id`、`amount_cents`、`currency`、`status`、`channel`、`platform`、`country`、`region`、`city`、`product_id`、`reason`
- 分钟在线与日活/新增
  - Worker 使用 Redis HyperLogLog 统计分钟在线（heartbeat/session_start）和 DAU/新增（login/register/first_active），周期性落入 ClickHouse

# 示例查询
- 最近 7 天 DAU/New
```sql
SELECT d, dau, new_users
FROM analytics.daily_users
WHERE game_id = 'demo' AND env = 'prod' AND d >= today() - 7
ORDER BY d;
```
- 最近 7 天收入（元）
```sql
SELECT d, revenue_cents/100.0 AS revenue, refunds_cents/100.0 AS refunds
FROM analytics.daily_revenue
WHERE game_id = 'demo' AND env = 'prod' AND d >= today() - 7
ORDER BY d;
```
- 峰值在线（聚合状态求值）
```sql
SELECT d, maxMerge(peak_online) AS peak_online
FROM analytics.daily_online_peak
WHERE game_id = 'demo' AND env = 'prod' AND d >= today() - 7
GROUP BY d ORDER BY d;
```
- 事件漏斗示例（进入->完成）
```sql
WITH
  (SELECT count() FROM analytics.events
   WHERE game_id='demo' AND env='prod'
     AND event_time >= now() - INTERVAL 7 DAY
     AND event='level.start') AS starts,
  (SELECT count() FROM analytics.events
   WHERE game_id='demo' AND env='prod'
     AND event_time >= now() - INTERVAL 7 DAY
     AND event='level.complete') AS completes
SELECT starts, completes, completes/starts AS cr;
```

# 性能与治理建议
- 低基数字段使用 LowCardinality（已在 DDL 使用）
- 按月分区、合理 ORDER BY（已在 DDL 使用）
- ReplacingMergeTree + version 字段用于“幂等/更新”写入
- 高基数字段优先放入 props_json，避免维度爆炸；对分析常用字段正式列化
