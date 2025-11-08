# Flink SQL Samples (Local Dev)

This folder can host Flink SQL scripts for local pipelines.

Example rough steps:

1) Define Kafka source tables (events/payments)

```
CREATE TABLE events (
  event_time TIMESTAMP(3),
  game_id STRING, env STRING,
  user_id STRING, session_id STRING,
  event STRING,
  properties STRING,
  WATERMARK FOR event_time AS event_time - INTERVAL '5' SECOND
) WITH (
  'connector' = 'kafka',
  'topic' = 'events',
  'properties.bootstrap.servers' = 'kafka:9092',
  'format' = 'json',
  'scan.startup.mode' = 'latest-offset'
);
```

2) Derive minute_online and sink to ClickHouse (via JDBC or custom sink)

```
CREATE TABLE minute_online (
  m TIMESTAMP(0),
  game_id STRING, env STRING,
  online BIGINT,
  PRIMARY KEY (m, game_id, env) NOT ENFORCED
) WITH (
  'connector'='jdbc',
  'url'='jdbc:clickhouse://clickhouse:8123/analytics',
  'table-name'='minute_online',
  'driver'='com.clickhouse.jdbc.ClickHouseDriver'
);

INSERT INTO minute_online
SELECT
  TUMBLE_START(event_time, INTERVAL '1' MINUTE) AS m,
  game_id, env,
  COUNT(DISTINCT user_id) AS online
FROM events
WHERE event IN ('heartbeat','session_start')
GROUP BY TUMBLE(event_time, INTERVAL '1' MINUTE), game_id, env;
```

Note: In production, prefer a dedicated connector (ClickHouse sink) or Debezium/JDBC with upsert semantics.

