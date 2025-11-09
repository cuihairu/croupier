-- Core analytics tables for events/payments and aggregates
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

-- Minute online (write-once per minute)
CREATE TABLE IF NOT EXISTS analytics.minute_online (
  m DateTime,
  game_id LowCardinality(String),
  env LowCardinality(String),
  online UInt32
) ENGINE = MergeTree
PARTITION BY toYYYYMM(m)
ORDER BY (game_id, env, m);

-- Daily peak online (MV)
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

-- Daily users (dau/new_users) with replacing upserts
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

-- Daily revenue with replacing upserts
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
