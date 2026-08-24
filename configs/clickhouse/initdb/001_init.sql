-- Initial ClickHouse bootstrap for analytics pipeline.
-- Column order matches worker INSERT statements exactly:
--   internal/analytics/worker/worker.go insertEventsSQL / insertPaymentsSQL.
-- Single analytics database with game_id/env columns (multi-tenant rows,
-- not database-per-game). The E2E script (scripts/e2e/analytics.sh) reuses
-- these files as the single source of truth for table DDL.

CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.events (
    event_time   DateTime DEFAULT now(),
    game_id      LowCardinality(String),
    env          LowCardinality(String),
    user_id      String,
    session_id   String,
    event        LowCardinality(String),
    channel      LowCardinality(String),
    platform     LowCardinality(String),
    country      FixedString(2),
    app_version  String,
    event_id     UUID,
    props_json   String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (game_id, env, event, user_id, event_time)
TTL toDateTime(event_time) + INTERVAL 6 MONTH;

CREATE TABLE IF NOT EXISTS analytics.payments (
    time         DateTime,
    game_id      LowCardinality(String),
    env          LowCardinality(String),
    user_id      String,
    order_id     String,
    amount_cents UInt64,
    currency     String,
    status       LowCardinality(String),
    channel      LowCardinality(String),
    platform     LowCardinality(String),
    country      FixedString(2),
    region       String,
    city         String,
    product_id   String,
    reason       String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(time)
ORDER BY (game_id, env, user_id, time)
TTL toDateTime(time) + INTERVAL 12 MONTH;
