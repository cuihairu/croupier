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
for i in $(seq 1 300); do
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

# --- 5. Wait for worker's Redis consumer group to be created ---
# worker creates the group in Run() goroutine (XGROUP CREATE MKSTREAM).
# XReadGroup starts from "$" — only messages added AFTER group creation
# are consumed. Wait for the group before writing events.
echo "Waiting for worker consumer group..."
GROUP_READY=0
for i in $(seq 1 20); do
  if docker exec croupier-redis redis-cli XINFO GROUPS analytics:events 2>/dev/null | grep -q "analytics-worker"; then
    GROUP_READY=1; break
  fi
  sleep 0.5
done
if [ "$GROUP_READY" -ne 1 ]; then
  fail "worker consumer group not created"
  kill $WORKER_PID 2>/dev/null
  docker compose -f "$COMPOSE_FILE" down >/dev/null 2>&1; exit 1
fi
ok "worker consumer group ready"

# --- 6. Write test events via Redis Streams (bypassing ingest HMAC) ---
# Worker processMessage expects a single "data" field containing JSON.
# Use python to safely serialize JSON and pass to redis-cli via docker exec.
echo "Writing test events..."
python3 -c "
import json, subprocess, sys

def xadd(stream, obj):
    payload = json.dumps(obj)
    cmd = ['docker', 'exec', 'croupier-redis', 'redis-cli', 'XADD', stream, '*', 'data', payload]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        print(f'XADD failed for {stream}: {r.stderr.strip()}', file=sys.stderr)
        sys.exit(1)
    print(r.stdout.strip())

xadd('analytics:events', {
    'game_id': 'e2e-game', 'env': 'dev', 'event': 'login',
    'user_id': 'test-user', 'session_id': 's1',
    'channel': 'direct', 'platform': 'linux',
    'props_json': json.dumps({'action': 'login'})
})
xadd('analytics:events', {
    'game_id': 'e2e-game', 'env': 'dev', 'event': 'purchase',
    'user_id': 'test-user', 'session_id': 's1',
    'channel': 'direct', 'platform': 'linux',
    'props_json': json.dumps({'item': 'sword', 'amount': 100})
})
xadd('analytics:payments', {
    'game_id': 'e2e-game', 'env': 'dev', 'user_id': 'test-user',
    'order_id': 'ORD-001', 'amount_cents': 999, 'currency': 'USD',
    'status': 'paid', 'channel': 'direct', 'platform': 'linux',
    'country': 'US', 'region': 'us-west', 'city': 'SF', 'product_id': 'sword'
})
"
if [ $? -ne 0 ]; then
  fail "XADD failed"
  kill $WORKER_PID 2>/dev/null; docker compose -f "$COMPOSE_FILE" down >/dev/null 2>&1; exit 1
fi

# Verify events are in Redis stream
EVENTS_LEN=$(docker exec croupier-redis redis-cli XLEN analytics:events 2>/dev/null || echo "0")
if [ "$EVENTS_LEN" -ge 2 ]; then
  ok "events in Redis stream (count=$EVENTS_LEN)"
else
  fail "events NOT in Redis stream (count=$EVENTS_LEN, expected >= 2)"
  kill $WORKER_PID 2>/dev/null
  docker compose -f "$COMPOSE_FILE" down >/dev/null 2>&1; exit 1
fi

# --- 6. Wait for worker to consume events and flush to ClickHouse ---
# Worker XReadGroup blocks up to 2s per poll, then batches inserts to
# ClickHouse every 15s. Sleep 25s to cover: consume latency (2s) +
# batch flush interval (15s) + safety margin (8s).
echo "Waiting for worker to consume and flush (25s)..."
sleep 25

# --- 7. Verify in ClickHouse ---
echo "Verifying in ClickHouse..."

EVENTS_COUNT=$(docker exec croupier-clickhouse clickhouse-client --query \
  "SELECT count() FROM analytics.events WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$EVENTS_COUNT" -ge 2 ]; then
  ok "events in ClickHouse (count=$EVENTS_COUNT)"
else
  fail "events NOT in ClickHouse (count=$EVENTS_COUNT, expected >= 2)"
  echo "--- worker log (last 20 lines) ---"
  tail -20 /tmp/e2e-analytics-worker.log 2>/dev/null || echo "(no worker log)"
  # XReadGROUP direct test
  XREADGROUP_OUT=$(docker exec croupier-redis redis-cli XREADGROUP GROUP analytics-worker c1 STREAMS analytics:events ">" COUNT 2 2>&1)
  echo "--- XReadGROUP direct result ---"
  echo "$XREADGROUP_OUT"
  echo "--- Redis stream analytics:events (XLEN) ---"
  docker exec croupier-redis redis-cli XLEN analytics:events 2>/dev/null || echo "(redis query failed)"
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
