-- Aggregate tables for the analytics pipeline.
-- Column order matches worker flush statements exactly:
--   internal/analytics/worker/worker.go (minute_online / daily_users / daily_revenue).
--
-- minute_online: HLL minute-level online counts. Each (game, env, minute) is
-- rewritten with a fresh snapshot every flush; ReplacingMergeTree(version)
-- keeps the latest snapshot for a minute instead of Summing them (a HLL count
-- is an absolute value, not an increment).
-- daily_users / daily_revenue: worker rewrites the current day's row every
-- flush with version = unix-seconds timestamp; ReplacingMergeTree(version)
-- keeps the max version for each (game, env, day). version is the engine's
-- version parameter (NOT part of ORDER BY), so FINAL dedupes by day key.

CREATE TABLE IF NOT EXISTS analytics.minute_online (
    m        DateTime,
    game_id  LowCardinality(String),
    env      LowCardinality(String),
    online   UInt32,
    version  UInt64
) ENGINE = ReplacingMergeTree(version)
ORDER BY (game_id, env, m);

CREATE TABLE IF NOT EXISTS analytics.daily_users (
    d         Date,
    game_id   LowCardinality(String),
    env       LowCardinality(String),
    dau       UInt64,
    new_users UInt64,
    version   UInt64
) ENGINE = ReplacingMergeTree(version)
ORDER BY (game_id, env, d);

CREATE TABLE IF NOT EXISTS analytics.daily_revenue (
    d             Date,
    game_id       LowCardinality(String),
    env           LowCardinality(String),
    revenue_cents UInt64,
    refunds_cents UInt64,
    failed        UInt64,
    version       UInt64
) ENGINE = ReplacingMergeTree(version)
ORDER BY (game_id, env, d);
