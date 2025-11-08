-- Initial ClickHouse bootstrap for local dev
CREATE DATABASE IF NOT EXISTS analytics;
-- You can add tables/materialized views here, e.g.:
-- CREATE TABLE IF NOT EXISTS analytics.events (
--   event_time DateTime,
--   game_id LowCardinality(String), env LowCardinality(String),
--   user_id String, session_id String,
--   event LowCardinality(String),
--   channel LowCardinality(String), platform LowCardinality(String),
--   country FixedString(2), app_version String,
--   event_id UUID, props_json String
--) ENGINE=MergeTree
-- PARTITION BY toYYYYMM(event_time)
-- ORDER BY (game_id, env, event, user_id, event_time);

