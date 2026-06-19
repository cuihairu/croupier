-- Core analytics tables for events/payments and aggregates
-- Database-per-game architecture: Each game has its own independent database.
-- The game_id and env are implicit from the database name.
-- This script creates example tables for game_demo_prod database.

-- Create game_demo_prod database
CREATE DATABASE IF NOT EXISTS game_demo_prod;

-- Use game_demo_prod database
USE game_demo_prod;

-- Events table (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS events (
  event_time DateTime DEFAULT now(),
  user_id String,
  session_id String,
  event LowCardinality(String),
  channel LowCardinality(String),
  platform LowCardinality(String),
  country FixedString(2),
  app_version String,
  event_id UUID,
  server_id LowCardinality(String), -- MMORPG multi-server support
  props_json String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (event, server_id, user_id, event_time)
TTL event_time + INTERVAL 6 MONTH;

-- Payments table (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS payments (
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
  server_id LowCardinality(String) -- MMORPG multi-server support
) ENGINE = MergeTree
PARTITION BY toYYYYMM(time)
ORDER BY (time, order_id)
TTL time + INTERVAL 12 MONTH;

-- Minute online (write-once per minute, game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS minute_online (
  m DateTime,
  server_id LowCardinality(String),
  online UInt32
) ENGINE = MergeTree
PARTITION BY toYYYYMM(m)
ORDER BY (server_id, m);

-- Daily peak online (MV)
CREATE TABLE IF NOT EXISTS daily_online_peak (
  d Date,
  server_id LowCardinality(String),
  peak_online AggregateFunction(max, UInt32)
) ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(d)
ORDER BY (server_id, d);

CREATE MATERIALIZED VIEW IF NOT EXISTS daily_online_peak_mv
TO daily_online_peak AS
SELECT toDate(m) AS d, server_id, maxState(online) AS peak_online
FROM minute_online
GROUP BY d, server_id;

-- Daily users (dau/new_users) with replacing upserts
CREATE TABLE IF NOT EXISTS daily_users (
  d Date,
  server_id LowCardinality(String),
  dau UInt64,
  new_users UInt64,
  version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(d)
ORDER BY (server_id, d);

-- Daily revenue with replacing upserts
CREATE TABLE IF NOT EXISTS daily_revenue (
  d Date,
  server_id LowCardinality(String),
  revenue_cents UInt64,
  refunds_cents UInt64,
  failed UInt64,
  version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(d)
ORDER BY (server_id, d);

-- Example: Create staging database for the same game
CREATE DATABASE IF NOT EXISTS game_demo_staging;

-- Use staging database
USE game_demo_staging;

-- Create same table structure in staging
CREATE TABLE IF NOT EXISTS events (
  event_time DateTime DEFAULT now(),
  user_id String,
  session_id String,
  event LowCardinality(String),
  channel LowCardinality(String),
  platform LowCardinality(String),
  country FixedString(2),
  app_version String,
  event_id UUID,
  server_id LowCardinality(String),
  props_json String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (event, server_id, user_id, event_time)
TTL event_time + INTERVAL 3 MONTH;

CREATE TABLE IF NOT EXISTS payments (
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
  server_id LowCardinality(String)
) ENGINE = MergeTree
PARTITION BY toYYYYMM(time)
ORDER BY (time, order_id)
TTL time + INTERVAL 6 MONTH;
