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
# Container names the script talks to via docker exec. CI gets the compose
# defaults; hosts running the self-hosted deploy stack side by side can point
# these at a separate compose project's containers.
REDIS_CONTAINER="${REDIS_CONTAINER:-croupier-redis}"
CH_CONTAINER="${CH_CONTAINER:-croupier-clickhouse}"

echo "==> Analytics E2E (simplified: Redis XADD → worker → ClickHouse)"
echo ""

# --- 1. Docker-compose up (Redis + ClickHouse) ---
# SKIP_COMPOSE_UP=1 reuses already-running containers (e.g. the self-hosted
# deploy stack) instead of starting the dev compose project.
if [ "${SKIP_COMPOSE_UP:-0}" = "1" ]; then
  echo "SKIP_COMPOSE_UP=1: reusing running containers ($REDIS_CONTAINER / $CH_CONTAINER)"
else
  echo "Starting Redis + ClickHouse..."
  docker compose -f "$COMPOSE_FILE" up -d --wait redis clickhouse 2>&1 || { fail "docker-compose up"; exit 1; }
fi

# Verify ClickHouse is responding (HTTP port 8123 is more reliable than
# clickhouse-client which may not be in PATH in some image variants).
for i in $(seq 1 60); do
  if docker exec "$CH_CONTAINER" wget -qO- "http://localhost:8123/?query=SELECT%201" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
if ! docker exec "$CH_CONTAINER" wget -qO- "http://localhost:8123/?query=SELECT%201" >/dev/null 2>&1; then
  fail "ClickHouse not ready"
  echo "--- ClickHouse container logs (last 20 lines) ---"
  docker logs "$CH_CONTAINER" 2>&1 | tail -20
  docker compose -f "$COMPOSE_FILE" down; exit 1
fi
ok "ClickHouse ready"

# Wait for ClickHouse system tables to fully initialize after readiness.
sleep 3

ok "Redis ready"

# --- 2. Create analytics tables (single source of truth: initdb files) ---
# Reuse configs/clickhouse/initdb/*.sql so the E2E schema can never drift
# from what a fresh deployment bootstraps (worker INSERT column order).
echo "Creating analytics tables from initdb files..."
for ddl in configs/clickhouse/initdb/001_init.sql configs/clickhouse/initdb/010_analytics.sql; do
  if ! docker exec -i croupier-clickhouse clickhouse-client --multiquery < "$ddl"; then
    fail "apply $ddl"
    docker compose -f "$COMPOSE_FILE" down; exit 1
  fi
done
ok "analytics tables created (initdb 001 + 010)"

# --- 3. Build worker (or reuse a prebuilt binary via WORKER_BIN) ---
if [ -n "${WORKER_BIN:-}" ] && [ -x "$WORKER_BIN" ]; then
  echo "Using prebuilt worker: $WORKER_BIN"
  mkdir -p bin && cp "$WORKER_BIN" bin/analytics-worker
else
  echo "Building analytics-worker..."
  if ! go build -o bin/analytics-worker ./cmd/analytics-worker 2>&1; then
    fail "build analytics-worker"
    docker compose -f "$COMPOSE_FILE" down; exit 1
  fi
fi
ok "analytics-worker ready"

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
  if docker exec "$REDIS_CONTAINER" redis-cli XINFO GROUPS analytics:events 2>/dev/null | grep -q "analytics-worker"; then
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
import json, subprocess, sys, uuid

def xadd(stream, obj):
    payload = json.dumps(obj)
    cmd = ['docker', 'exec', 'croupier-redis', 'redis-cli', 'XADD', stream, '*', 'data', payload]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        print(f'XADD failed for {stream}: {r.stderr.strip()}', file=sys.stderr)
        sys.exit(1)
    print(r.stdout.strip())

import time as _time
_now = _time.strftime('%Y-%m-%dT%H:%M:%SZ', _time.gmtime())
xadd('analytics:events', {
    'game_id': 'e2e-game', 'env': 'dev', 'event': 'login',
    'user_id': 'test-user', 'session_id': 's1',
    'channel': 'direct', 'platform': 'linux', 'country': 'US',
    'event_id': str(uuid.uuid4()),
    'ts': _now,
    'props': {'action': 'login'}
})
xadd('analytics:events', {
    'game_id': 'e2e-game', 'env': 'dev', 'event': 'session_start',
    'user_id': 'agg-user', 'session_id': 's2',
    'channel': 'direct', 'platform': 'linux', 'country': 'US',
    'event_id': str(uuid.uuid4()),
    'ts': _now,
    'props': {}
})
xadd('analytics:events', {
    'game_id': 'e2e-game', 'env': 'dev', 'event': 'purchase',
    'user_id': 'test-user', 'session_id': 's1',
    'channel': 'direct', 'platform': 'linux', 'country': 'US',
    'event_id': str(uuid.uuid4()),
    'props': {'item': 'sword', 'amount': 100}
})
xadd('analytics:payments', {
    'game_id': 'e2e-game', 'env': 'dev', 'user_id': 'test-user',
    'order_id': 'ORD-001', 'amount_cents': 999, 'currency': 'USD',
    'status': 'success', 'channel': 'direct', 'platform': 'linux',
    'country': 'US', 'region': 'us-west', 'city': 'SF', 'product_id': 'sword',
    'ts': _now
})
"
if [ $? -ne 0 ]; then
  fail "XADD failed"
  kill $WORKER_PID 2>/dev/null; docker compose -f "$COMPOSE_FILE" down >/dev/null 2>&1; exit 1
fi

# Verify events are in Redis stream
EVENTS_LEN=$(docker exec "$REDIS_CONTAINER" redis-cli XLEN analytics:events 2>/dev/null || echo "0")
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

EVENTS_COUNT=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.events WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$EVENTS_COUNT" -ge 2 ]; then
  ok "events in ClickHouse (count=$EVENTS_COUNT)"
else
  fail "events NOT in ClickHouse (count=$EVENTS_COUNT, expected >= 2)"
  echo "--- worker log (last 20 lines) ---"
  tail -20 /tmp/e2e-analytics-worker.log 2>/dev/null || echo "(no worker log)"
  # XReadGROUP direct test
  XREADGROUP_OUT=$(docker exec "$REDIS_CONTAINER" redis-cli XREADGROUP GROUP analytics-worker c1 COUNT 2 STREAMS analytics:events ">" 2>&1)
  echo "--- XReadGROUP direct result ---"
  echo "$XREADGROUP_OUT"
  echo "--- Redis stream analytics:events (XLEN) ---"
  docker exec "$REDIS_CONTAINER" redis-cli XLEN analytics:events 2>/dev/null || echo "(redis query failed)"
fi

PAYMENTS_COUNT=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.payments WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$PAYMENTS_COUNT" -ge 1 ]; then
  ok "payments in ClickHouse (count=$PAYMENTS_COUNT)"
else
  fail "payments NOT in ClickHouse (count=$PAYMENTS_COUNT, expected >= 1)"
fi

# Verify event content
EVENT_TYPES=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT DISTINCT event FROM analytics.events WHERE game_id = 'e2e-game' ORDER BY event" 2>/dev/null | tr '\n' ',')
if echo "$EVENT_TYPES" | grep -q "login" && echo "$EVENT_TYPES" | grep -q "purchase"; then
  ok "event types correct: $EVENT_TYPES"
else
  fail "event types wrong (got: $EVENT_TYPES)"
fi

# Verify aggregate tables (minute online / daily users / daily revenue).
# minute_online only flushes minutes that have fully elapsed (t < nowMin),
# so wait for the next minute boundary (+ one 15s flush cycle) before
# asserting; otherwise the test flakes depending on wall-clock position.
echo "Waiting for minute boundary + flush cycle (up to 80s)..."
python3 - <<'PYEOF'
import time
now = time.time()
wait = 60 - (now % 60) + 20
time.sleep(min(wait, 80))
PYEOF
ONLINE_COUNT=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.minute_online WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$ONLINE_COUNT" -ge 1 ]; then
  ok "minute_online aggregate (rows=$ONLINE_COUNT)"
else
  fail "minute_online aggregate empty (expected >= 1 row)"
fi

DAU_COUNT=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.daily_users WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$DAU_COUNT" -ge 1 ]; then
  ok "daily_users aggregate (rows=$DAU_COUNT)"
else
  fail "daily_users aggregate empty (expected >= 1 row)"
  echo "--- worker log tail (daily_users diagnosis) ---"
  grep -iE "daily|pfcount|ch " /tmp/e2e-analytics-worker.log | tail -10
fi

REVENUE_COUNT=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.daily_revenue WHERE game_id = 'e2e-game'" 2>/dev/null || echo "0")
if [ "$REVENUE_COUNT" -ge 1 ]; then
  ok "daily_revenue aggregate (rows=$REVENUE_COUNT)"
else
  fail "daily_revenue aggregate empty (expected >= 1 row; only written for success/refunded/failed payments)"
  echo "--- worker log tail (daily_revenue diagnosis) ---"
  grep -iE "daily|revenue|ch " /tmp/e2e-analytics-worker.log | tail -10
  echo "--- redis hll keys ---"
  docker exec "$REDIS_CONTAINER" redis-cli --scan --pattern 'hll:*' | head -10
fi

# --- 8. Cleanup ---
kill $WORKER_PID 2>/dev/null
if [ "${SKIP_COMPOSE_UP:-0}" != "1" ]; then
  docker compose -f "$COMPOSE_FILE" down >/dev/null 2>&1
fi

echo ""
echo "==> Analytics E2E result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
