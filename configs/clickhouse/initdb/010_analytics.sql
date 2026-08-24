-- Aggregate tables for the analytics pipeline.
-- Column order matches worker flush statements exactly:
--   internal/analytics/worker/worker.go (minute_online / daily_users / daily_revenue).
--
-- minute_online: HLL minute-level online counts, summed per (game, env, minute).
-- daily_users / daily_revenue: worker rewrites the current day's row every
-- flush with version = unix-seconds timestamp; ReplacingMergeTree keeps the max.

CREATE TABLE IF NOT EXISTS analytics.minute_online (
    m        DateTime,
    game_id  LowCardinality(String),
    env      LowCardinality(String),
    online   UInt32
) ENGINE = SummingMergeTree
ORDER BY (game_id, env, m);

CREATE TABLE IF NOT EXISTS analytics.daily_users (
    d         Date,
    game_id   LowCardinality(String),
    env       LowCardinality(String),
    dau       UInt64,
    new_users UInt64,
    version   UInt64
) ENGINE = ReplacingMergeTree
ORDER BY (game_id, env, d, version);

CREATE TABLE IF NOT EXISTS analytics.daily_revenue (
    d             Date,
    game_id       LowCardinality(String),
    env           LowCardinality(String),
    revenue_cents UInt64,
    refunds_cents UInt64,
    failed        UInt64,
    version       UInt64
) ENGINE = ReplacingMergeTree
ORDER BY (game_id, env, d, version);
