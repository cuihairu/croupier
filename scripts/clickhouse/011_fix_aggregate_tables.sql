-- DATA-003: rebuild existing analytics aggregate tables with versioned
-- replacement keys (single-use migration for already-initialized volumes).
--
-- docker-entrypoint-initdb.d only replays on FIRST-TIME volume init, so a
-- volume created before this fix keeps the old engines. ClickHouse cannot
-- MODIFY ENGINE (and shrinking ORDER BY is rejected when the old primary key
-- is longer), so each table is rebuilt: create the corrected table, copy rows
-- (filling the new minute_online.version with now()), swap via RENAME, then
-- drop the old table.
--
-- STOP the analytics worker before running this, then run it once:
--   docker exec -i croupier-clickhouse clickhouse-client --multiquery < \
--     scripts/clickhouse/011_fix_aggregate_tables.sql
-- (It is NOT idempotent: re-running re-copies rows and may duplicate data.)

CREATE TABLE IF NOT EXISTS analytics.minute_online_new (
    m        DateTime,
    game_id  LowCardinality(String),
    env      LowCardinality(String),
    online   UInt32,
    version  UInt64
) ENGINE = ReplacingMergeTree(version)
ORDER BY (game_id, env, m);

INSERT INTO analytics.minute_online_new (m, game_id, env, online, version)
SELECT m, game_id, env, online, toUnixTimestamp(now()) FROM analytics.minute_online;

RENAME TABLE analytics.minute_online TO analytics.minute_online_old,
             analytics.minute_online_new TO analytics.minute_online;
DROP TABLE analytics.minute_online_old;

CREATE TABLE IF NOT EXISTS analytics.daily_users_new (
    d         Date,
    game_id   LowCardinality(String),
    env       LowCardinality(String),
    dau       UInt64,
    new_users UInt64,
    version   UInt64
) ENGINE = ReplacingMergeTree(version)
ORDER BY (game_id, env, d);

INSERT INTO analytics.daily_users_new (d, game_id, env, dau, new_users, version)
SELECT d, game_id, env, dau, new_users, version FROM analytics.daily_users;

RENAME TABLE analytics.daily_users TO analytics.daily_users_old,
             analytics.daily_users_new TO analytics.daily_users;
DROP TABLE analytics.daily_users_old;

CREATE TABLE IF NOT EXISTS analytics.daily_revenue_new (
    d             Date,
    game_id       LowCardinality(String),
    env           LowCardinality(String),
    revenue_cents UInt64,
    refunds_cents UInt64,
    failed        UInt64,
    version       UInt64
) ENGINE = ReplacingMergeTree(version)
ORDER BY (game_id, env, d);

INSERT INTO analytics.daily_revenue_new (d, game_id, env, revenue_cents, refunds_cents, failed, version)
SELECT d, game_id, env, revenue_cents, refunds_cents, failed, version FROM analytics.daily_revenue;

RENAME TABLE analytics.daily_revenue TO analytics.daily_revenue_old,
             analytics.daily_revenue_new TO analytics.daily_revenue;
DROP TABLE analytics.daily_revenue_old;
