#!/bin/bash
# scripts/e2e/analytics.sh
# Simplified Analytics E2E: validates the worker → ClickHouse pipeline by
# writing directly to Redis Streams (bypassing ingest HMAC auth). This
# verifies the core data path without the complexity of the ingest layer.
#
# Prerequisites: docker, docker compose, Go toolchain.
# Runs against docker/docker-compose.yml (Redis + ClickHouse).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
PASS=0; FAIL=0
ok()   { echo -e "${GREEN}PASS${NC} — $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}FAIL${NC} — $1"; FAIL=$((FAIL+1)); }

COMPOSE_FILE="${COMPOSE_FILE:-docker/docker-compose.yml}"

echo "==> Analytics E2E (simplified: Redis XADD → worker → ClickHouse)"
echo ""

# --- 1. Docker-compose up (Redis + ClickHouse) ---
echo "Starting Redis + ClickHouse..."
docker compose -f "$COMPOSE_FILE" up -d redis clickhouse 2>&1 || { fail "docker-compose up"; exit 1; }

# Wait for ClickHouse
for i in $(seq 1 180); do
  docker exec croupier-clickhouse clickhouse-client --query "SELECT 1" >/dev/null 2>&1 && break
  sleep 1
done
if ! docker exec croupier-clickhouse clickhouse-client --query "SELECT 1" >/dev/null 2>&1; then
  fail "ClickHouse not ready"; docker compose -f "$COMPOSE_FILE" down; exit 1
fi
ok "ClickHouse ready"

# Wait for ClickHouse system tables to fully initialize after readiness.
# Immediately after readiness the system tables may still be loading,
# causing ATTEMPT_TO_READ_AFTER_EOF on CREATE DATABASE/DATA statements.
sleep 5

# Wait for Redis
for i in $(seq 1 15); do
  docker exec croupier-redis redis-cli ping 2>/dev/null | grep -q PONG && break
  sleep 1
done
ok "Redis ready"

# --- 2. Create analytics tables ---
echo "Creating analytics tables..."
docker exec -i croupier-clickhouse clickhouse-client --multiquery <<'SQL'
CREATE DATABASE IF NOT EXISTS analytics;

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
  ORDER BY (game_id, env, event, user_id, event_time)
  TTL event_time + INTERVAL 6 MONTH;

CREATE TABLE IF NOT EXISTS analytics.payments (
  time DateTime DEFAULT now(),
  game_id LowCardinality(String),
  env LowCardinality(String),
  user_id String,
  order_id String,
  amount_cents UInt64,
  currency String,
  status LowCardinality(String),
  channel LowCardinality(String),
  platform LowCardinality(String),
  country FixedString(2),
  region String,
  city String,
  product_id String,
  reason String
) ENGINE = MergeTree
  PARTITION BY toYYYYMM(time)
  ORDER BY (game_id, env, user_id, time)
  TTL time + INTERVAL 12 MONTH;
SQL
if [ $? -ne 0 ]; then
  fail "create analytics tables"
  docker compose -f "$COMPOSE_FILE" down; exit 1
fi
ok "analytics tables created"

# --- 3. Build worker ---
echo "Building analytics-worker..."
if ! go build -o bin/analytics-worker ./cmd/analytics-worker 2>&1; then
  fail "build analytics-worker"
  docker compose -f "$COMPOSE_FILE" down; exit 1
fi
ok "analytics-worker built"

# --- 4. Start worker (background) ---
export CLICKHOUSE_DSN="clickhouse://localhost:9000/analytics"
export REDIS_URL="redis://localhost:6379/0"
./bin/analytics-worker > /tmp/e2e-analytics-worker.log 2>&1 &
WORKER_PID=$!
sleep 1
if ! kill -0 $WORKER_PID 2>/dev/null; then
  fail "worker start"; cat /tmp/e2e-analytics-worker.log
  docker compose -f "$COMPOSE_FILE" down; exit 1
fi
ok "worker started (pid=$WORKER_PID)"

# --- 5. Write test events via Redis Streams (bypassing ingest HMAC) ---
echo "Writing test events..."
EVENT_ID_1=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || python3 -c "import uuid; print(uuid.uuid4())")
EVENT_ID_2=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || python3 -c "import uuid; print(uuid.uuid4())")

docker exec croupier-redis redis-cli XADD analytics:events '*' \
  game_id=e2e-game env=dev user_id=test-user session_id=s1 \
  event=login channel=direct platform=linux country=US \
  app_version=1.0.0 "event_id=$EVENT_ID_1" 'props_json={"action":"login"}' >/dev/null

docker exec croupier-redis redis-cli XADD analytics:events '*' \
  game_id=e2e-game env=dev user_id=test-user session_id=s1 \
  event=purchase channel=direct platform=linux country=US \
  app_version=1.0.0 "event_id=$EVENT_ID_2" 'props_json={"item":"sword","amount":100}' >/dev/null

docker exec croupier-redis redis-cli XADD analytics:payments '*' \
  time="$(date -u +%Y-%m-%dT%H:%M:%SZ)" game_id=e2e-game env=dev \
  user_id=test-user order_id=ORD-001 amount_cents=999 currency=USD \
  status=paid channel=direct platform=linux country=US region=us-west \
  city=SF product_id=sword reason= >/dev/null

ok "events + payment written"

# --- 6. Wait briefly for worker to read events into its batch ---
sleep 1

# --- 7. Kill worker (SIGTERM triggers defer flushBatches, writing to ClickHouse) ---
echo "Stopping worker to flush event batches..."
kill $WORKER_PID 2>/dev/null
# Wait for worker to flush and exit (defer flushBatches runs on SIGTERM).
for i in $(seq 1 15); do
  kill -0 $WORKER_PID 2>/dev/null || break
  sleep 0.5
done
sleep 1

# --- 8. Verify in ClickHouse ---
echo "Verifying in ClickHouse..."

EVENTS_COUNT=$(docker exec croupier-clickhouse clickhouse-client --query \
  "SELECT count() FROM analytics.events WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$EVENTS_COUNT" -ge 2 ]; then
  ok "events in ClickHouse (count=$EVENTS_COUNT)"
else
  fail "events NOT in ClickHouse (count=$EVENTS_COUNT, expected >= 2)"
fi

PAYMENTS_COUNT=$(docker exec croupier-clickhouse clickhouse-client --query \
  "SELECT count() FROM analytics.payments WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$PAYMENTS_COUNT" -ge 1 ]; then
  ok "payments in ClickHouse (count=$PAYMENTS_COUNT)"
else
  fail "payments NOT in ClickHouse (count=$PAYMENTS_COUNT, expected >= 1)"
fi

# Verify event content
EVENT_TYPES=$(docker exec croupier-clickhouse clickhouse-client --query \
  "SELECT DISTINCT event FROM analytics.events WHERE game_id = 'e2e-game' ORDER BY event" 2>/dev/null | tr '\n' ',')
if echo "$EVENT_TYPES" | grep -q "login" && echo "$EVENT_TYPES" | grep -q "purchase"; then
  ok "event types correct: $EVENT_TYPES"
else
  fail "event types wrong (got: $EVENT_TYPES)"
fi

# --- 8. Cleanup ---
kill $WORKER_PID 2>/dev/null
docker compose -f "$COMPOSE_FILE" down >/dev/null 2>&1

echo ""
echo "==> Analytics E2E result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
