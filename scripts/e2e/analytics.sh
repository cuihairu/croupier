#!/bin/bash
# scripts/e2e/analytics.sh
# Analytics E2E: covers both inbound paths:
#   1. direct Redis XADD → worker → ClickHouse (worker-path regression, kept);
#   2. signed HTTP POST → ingest → Redis → worker → ClickHouse, including
#      batch validation (DATA-016/017), nonce replay, amount precision
#      (DATA-018) and cross-window aggregates.
#
# Prerequisites: docker, docker compose, Go toolchain, python3.
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
# docker compose wrapper: the repo default file is passed via -f; any other
# COMPOSE_FILE value is honored through the environment, where compose
# accepts a separator list of files (COMPOSE_PATH_SEPARATOR) — the -f flag
# does not split lists.
compose() {
  if [ "$COMPOSE_FILE" = "docker/docker-compose.yml" ]; then
    docker compose -f docker/docker-compose.yml "$@"
  else
    COMPOSE_PATH_SEPARATOR=':' docker compose "$@"
  fi
}
# Container names the script talks to via docker exec. CI gets the compose
# defaults; hosts running the self-hosted deploy stack side by side can point
# these at a separate compose project's containers.
REDIS_CONTAINER="${REDIS_CONTAINER:-croupier-redis}"
CH_CONTAINER="${CH_CONTAINER:-croupier-clickhouse}"

echo "==> Analytics E2E (Redis XADD + signed HTTP ingest → worker → ClickHouse)"
echo ""

# --- 1. Docker-compose up (Redis + ClickHouse) ---
# SKIP_COMPOSE_UP=1 reuses already-running containers (e.g. the self-hosted
# deploy stack) instead of starting the dev compose project.
if [ "${SKIP_COMPOSE_UP:-0}" = "1" ]; then
  echo "SKIP_COMPOSE_UP=1: reusing running containers ($REDIS_CONTAINER / $CH_CONTAINER)"
else
  echo "Starting Redis + ClickHouse..."
  compose up -d --wait redis clickhouse 2>&1 || { fail "docker-compose up"; exit 1; }
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
  compose down; exit 1
fi
ok "ClickHouse ready"

# Wait for ClickHouse system tables to fully initialize after readiness.
sleep 3

ok "Redis ready"

# --- 2. Schema lifecycle: fresh-create AND existing-table re-apply (DATA-032) ---
# DDL changes must be verifiable in both shapes: an empty database (first
# boot) and tables that already exist (idempotent re-apply / upgrade path).
# The initdb files are the single source of truth so the E2E schema can
# never drift from what a fresh deployment bootstraps (worker INSERT column
# order).
echo "Applying analytics schema lifecycle..."
if [ "${SKIP_COMPOSE_UP:-0}" != "1" ]; then
  # Script-managed project: prove the fresh-create path by dropping first.
  # Never done in SKIP_COMPOSE_UP=1 mode — that reuses a shared/self-hosted
  # stack whose analytics data must not be erased.
  if ! docker exec "$CH_CONTAINER" clickhouse-client --query "DROP DATABASE IF EXISTS analytics"; then
    fail "drop analytics database"
    compose down; exit 1
  fi
fi
for ddl in configs/clickhouse/initdb/001_init.sql configs/clickhouse/initdb/010_analytics.sql; do
  if ! docker exec -i "$CH_CONTAINER" clickhouse-client --multiquery < "$ddl"; then
    fail "apply $ddl"
    compose down; exit 1
  fi
done
TABLE_HITS=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM system.tables WHERE database = 'analytics' AND name IN ('events','payments','minute_online','daily_users','daily_revenue')" 2>/dev/null || echo "0")
if [ "$TABLE_HITS" -eq 5 ]; then
  ok "schema fresh-create + idempotent re-apply (5/5 core tables)"
else
  fail "schema incomplete after apply ($TABLE_HITS/5 core tables)"
  compose down >/dev/null 2>&1; exit 1
fi
if [ "${SKIP_COMPOSE_UP:-0}" != "1" ]; then
  # The single-use upgrade migration (scripts/clickhouse/011) must keep
  # executing against the current schema shape. The aggregate tables are
  # empty here (fresh-created above), so its row copy is a no-op and the
  # RENAME swap is lossless. Not applied against shared stacks: it requires
  # a stopped worker and re-running duplicates data.
  if ! docker exec -i "$CH_CONTAINER" clickhouse-client --multiquery \
    < scripts/clickhouse/011_fix_aggregate_tables.sql; then
    fail "upgrade migration 011"
    compose down >/dev/null 2>&1; exit 1
  fi
  ok "upgrade migration 011 executes over existing tables"
fi

# --- 3. Build worker (or reuse a prebuilt binary via WORKER_BIN) ---
if [ -n "${WORKER_BIN:-}" ] && [ -x "$WORKER_BIN" ]; then
  echo "Using prebuilt worker: $WORKER_BIN"
  mkdir -p bin && cp "$WORKER_BIN" bin/analytics-worker
else
  echo "Building analytics-worker..."
  if ! go build -o bin/analytics-worker ./cmd/analytics-worker 2>&1; then
    fail "build analytics-worker"
    compose down; exit 1
  fi
fi
ok "analytics-worker ready"

# --- 4. Start worker (background) ---
# REDIS_URL / CLICKHOUSE_DSN honor caller-provided values (they are the two
# binaries' canonical env vars) so hosts where the compose-published ports
# collide (e.g. a native Redis on 6379) can republish the services via a
# COMPOSE_FILE override and point these at the alternate ports. CI keeps the
# defaults.
export CLICKHOUSE_DSN="${CLICKHOUSE_DSN:-clickhouse://localhost:9000/analytics}"
export REDIS_URL="${REDIS_URL:-redis://localhost:6379/0}"
./bin/analytics-worker > /tmp/e2e-analytics-worker.log 2>&1 &
WORKER_PID=$!
sleep 1
if ! kill -0 $WORKER_PID 2>/dev/null; then
  fail "worker start"; cat /tmp/e2e-analytics-worker.log
  compose down; exit 1
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
  compose down >/dev/null 2>&1; exit 1
fi
ok "worker consumer group ready"

# --- 5b. Build ingest (or reuse a prebuilt binary via INGEST_BIN) ---
INGEST_E2E_PORT="${INGEST_E2E_PORT:-18088}"
INGEST_E2E_SECRET="${INGEST_E2E_SECRET:-e2e-ingest-secret}"
# Per-run game id: the exact-count assertions in 7b must never collide with
# rows left by a previous run (named compose volumes persist across runs,
# and SKIP_COMPOSE_UP=1 reuses a shared stack entirely).
INGEST_E2E_GAME="e2e-ingest-$(date +%s)"
if [ -n "${INGEST_BIN:-}" ] && [ -x "$INGEST_BIN" ]; then
  echo "Using prebuilt ingest: $INGEST_BIN"
  mkdir -p bin && cp "$INGEST_BIN" bin/ingest
else
  echo "Building ingest..."
  if ! go build -o bin/ingest ./cmd/ingest 2>&1; then
    fail "build ingest"
    compose down; exit 1
  fi
fi
ok "ingest binary ready"

# --- 5c. Ingest chain: signed HTTP publish (the full inbound path) ---
# Fixed samples go through cmd/ingest with test credentials: accepted batch,
# duplicate re-submit (same payload, fresh nonce), invalid batch (DATA-016:
# 400 + details.index, nothing enqueued), header/game_id mismatch, fractional
# amount_cents rejection (DATA-018) and nonce replay (429). Downstream rows
# are verified in ClickHouse in section 7b. The direct-XADD worker test in
# section 6 is retained on purpose: it isolates worker behavior from ingest.
echo "Starting ingest (127.0.0.1:${INGEST_E2E_PORT}, signed)..."
./bin/ingest --addr "127.0.0.1:${INGEST_E2E_PORT}" --secret "$INGEST_E2E_SECRET" \
  > /tmp/e2e-ingest.log 2>&1 &
INGEST_PID=$!
sleep 1
INGEST_READY=0
for i in $(seq 1 20); do
  if ! kill -0 $INGEST_PID 2>/dev/null; then break; fi
  if python3 -c "
import os, sys, urllib.request
try:
    with urllib.request.urlopen('http://127.0.0.1:${INGEST_E2E_PORT}/healthz', timeout=2) as r:
        sys.exit(0 if r.status == 200 else 1)
except Exception:
    sys.exit(1)
" 2>/dev/null; then
    INGEST_READY=1; break
  fi
  sleep 0.5
done
if [ "$INGEST_READY" -ne 1 ]; then
  fail "ingest start"; cat /tmp/e2e-ingest.log
  kill $WORKER_PID 2>/dev/null
  compose down >/dev/null 2>&1; exit 1
fi
ok "ingest started (pid=$INGEST_PID)"

echo "Publishing fixed samples via signed HTTP..."
INGEST_E2E_PORT="$INGEST_E2E_PORT" INGEST_E2E_SECRET="$INGEST_E2E_SECRET" INGEST_E2E_GAME="$INGEST_E2E_GAME" python3 - <<'PYEOF'
import base64, hashlib, hmac, json, os, subprocess, sys, time, urllib.error, urllib.request

PORT = os.environ["INGEST_E2E_PORT"]
SECRET = os.environ["INGEST_E2E_SECRET"].encode()
GAME = os.environ["INGEST_E2E_GAME"]
BASE = f"http://127.0.0.1:{PORT}"

def xlen(stream):
    r = subprocess.run(
        ["docker", "exec", "croupier-redis", "redis-cli", "XLEN", stream],
        capture_output=True, text=True,
    )
    try:
        return int(r.stdout.strip())
    except ValueError:
        return -1

def post(path, game, body_obj, ts=None, nonce=None):
    # Signature contract (cmd/ingest/cmd/root.go authMiddleware):
    # X-Signature = base64(HMAC_SHA256(secret, ts + "\n" + nonce + "\n" + sha256hex(body)))
    body = json.dumps(body_obj).encode()
    ts = ts if ts is not None else str(int(time.time()))
    nonce = nonce if nonce is not None else f"e2e-{time.time_ns()}"
    digest = hashlib.sha256(body).hexdigest()
    msg = (ts + "\n" + nonce + "\n" + digest).encode()
    sig = base64.b64encode(hmac.new(SECRET, msg, hashlib.sha256).digest()).decode()
    req = urllib.request.Request(BASE + path, data=body, method="POST", headers={
        "Content-Type": "application/json",
        "X-Game-Id": game,
        "X-Timestamp": ts,
        "X-Nonce": nonce,
        "X-Signature": sig,
    })
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            return r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()

def expect_accept(label, st, body):
    if st != 202:
        print(f"FAIL {label}: status={st} want=202 body={body[:300]}", file=sys.stderr)
        sys.exit(1)
    print(f"ok {label}: 202")

def expect_reject(label, st, body, want_status, want_error, want_index=None):
    good = st == want_status and want_error in body
    if want_index is not None:
        try:
            good = good and json.loads(body).get("details", {}).get("index") == want_index
        except Exception:
            good = False
    if not good:
        print(f"FAIL {label}: status={st} want={want_status}/{want_error} "
              f"index={want_index} body={body[:300]}", file=sys.stderr)
        sys.exit(1)
    print(f"ok {label}: {st} {want_error}")

now = int(time.time())

def iso(offset):
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(now + offset))

def sample(event, user, offset, props=None):
    return {
        "game_id": GAME, "env": "dev", "event": event,
        "user_id": user, "session_id": f"s-{user}",
        "channel": "direct", "platform": "linux", "country": "US",
        "ts": iso(offset), "props": props or {},
    }

# Cross-window liveness samples: session_start 100s apart, both >= 100s in
# the past, so they land in two distinct, fully elapsed minutes (the
# minute_online HLL only counts heartbeat/session_start).
batch = [
    sample("session_start", "iu1", -200),
    sample("session_start", "iu2", -100),
    sample("purchase", "iu1", 0, {"item": "potion", "amount": 1}),
]

st, body = post("/api/ingest/events", GAME, batch)
expect_accept("signed batch accepted", st, body)

# Duplicate re-submit: same content, fresh nonce -> accepted again; the
# worker re-inserts onto the same derived event_id (natural key).
st, body = post("/api/ingest/events", GAME, batch)
expect_accept("duplicate re-submit accepted", st, body)

# Invalid batch: item 1 misses required 'event' -> 400 invalid_payload with
# details.index=1 and NOTHING enqueued (validate-then-publish).
bad_batch = [dict(batch[0]), {"game_id": GAME, "env": "dev", "ts": iso(0)}, dict(batch[2])]
len_before = xlen("analytics:events")
st, body = post("/api/ingest/events", GAME, bad_batch)
expect_reject("invalid item rejected", st, body, 400, "invalid_payload", want_index=1)
len_after = xlen("analytics:events")
if len_before < 0 or len_after != len_before:
    print(f"FAIL invalid batch enqueued rows: XLEN {len_before} -> {len_after}", file=sys.stderr)
    sys.exit(1)
print(f"ok invalid batch enqueued nothing (XLEN stayed {len_after})")

# Header game vs payload game_id mismatch -> 400.
st, body = post("/api/ingest/events", "e2e-not-ingest", [dict(batch[0])])
expect_reject("game_id header mismatch rejected", st, body, 400, "invalid_payload")

# Payment with an exact cents amount through the whole chain.
payment = {
    "game_id": GAME, "env": "dev", "user_id": "iu1",
    "order_id": "ORD-ING-001", "amount_cents": 999, "currency": "USD",
    "status": "success", "channel": "direct", "platform": "linux",
    "country": "US", "region": "", "city": "", "product_id": "potion",
    "ts": iso(0),
}
st, body = post("/api/ingest/payments", GAME, [payment])
expect_accept("signed payment accepted", st, body)

# Fractional cents must be rejected with the item index (DATA-018), and
# nothing may be enqueued.
plen_before = xlen("analytics:payments")
st, body = post("/api/ingest/payments", GAME,
                [dict(payment, order_id="ORD-ING-BAD", amount_cents=99.5)])
expect_reject("fractional amount rejected", st, body, 400, "invalid_payload", want_index=0)
plen_after = xlen("analytics:payments")
if plen_before < 0 or plen_after != plen_before:
    print(f"FAIL rejected payment enqueued rows: XLEN {plen_before} -> {plen_after}", file=sys.stderr)
    sys.exit(1)
print(f"ok rejected payment enqueued nothing (XLEN stayed {plen_after})")

# Nonce replay: byte-identical signed request (same ts + nonce) must be
# refused after first use (429 duplicate_nonce).
ts0 = str(int(time.time()))
nonce0 = "e2e-replay-nonce-1"
st, body = post("/api/ingest/events", GAME, [sample("session_start", "iu3", -300)],
                ts=ts0, nonce=nonce0)
expect_accept("replay first use accepted", st, body)
st, body = post("/api/ingest/events", GAME, [sample("session_start", "iu3", -300)],
                ts=ts0, nonce=nonce0)
expect_reject("nonce replay refused", st, body, 429, "duplicate_nonce")

print("INGEST-CHAIN-OK")
PYEOF
if [ $? -ne 0 ]; then
  fail "ingest chain publish"
  kill $WORKER_PID $INGEST_PID 2>/dev/null
  compose down >/dev/null 2>&1; exit 1
fi
ok "ingest chain: signed/duplicate/invalid/mismatch/amount/replay cases verified"

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
# DATA-037 session-length samples: one underscore-spelling session_end with a
# duration prop ending now, one dotted-spelling session.end that started
# yesterday (cross-day raw shape). Both exercise the JSONHas(props_json,
# 'duration_ms') predicate family the warehouse/sessions endpoint queries.
xadd('analytics:events', {
    'game_id': 'e2e-game', 'env': 'dev', 'event': 'session_end',
    'user_id': 'sess-user', 'session_id': 's-sess-e2e-1',
    'channel': 'direct', 'platform': 'linux', 'country': 'US',
    'event_id': str(uuid.uuid4()),
    'ts': _now,
    'props': {'duration_ms': 90000, 'cause_end': 'normal'}
})
xadd('analytics:events', {
    'game_id': 'e2e-game', 'env': 'dev', 'event': 'session.end',
    'user_id': 'sess-user', 'session_id': 's-sess-e2e-2',
    'channel': 'direct', 'platform': 'linux', 'country': 'US',
    'event_id': str(uuid.uuid4()),
    'ts': _time.strftime('%Y-%m-%dT%H:%M:%SZ', _time.gmtime(_time.time() - 86400)),
    'props': {'duration_ms': 7200000, 'cause_end': 'disconnect'}
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
  kill $WORKER_PID 2>/dev/null; compose down >/dev/null 2>&1; exit 1
fi

# Verify events are in Redis stream
EVENTS_LEN=$(docker exec "$REDIS_CONTAINER" redis-cli XLEN analytics:events 2>/dev/null || echo "0")
if [ "$EVENTS_LEN" -ge 2 ]; then
  ok "events in Redis stream (count=$EVENTS_LEN)"
else
  fail "events NOT in Redis stream (count=$EVENTS_LEN, expected >= 2)"
  kill $WORKER_PID 2>/dev/null
  compose down >/dev/null 2>&1; exit 1
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

# --- 7b. Ingest chain results in ClickHouse (game $INGEST_E2E_GAME) ---
# All rows published via signed HTTP in section 5c. Expected shapes:
#   events:       3 (batch) + 3 (duplicate re-submit) + 1 (replay first use)
#                 = 7 raw rows collapsing onto 4 derived event_id natural keys
#                 (identical payloads -> identical worker UUIDv5 fingerprints);
#   payments:     exactly 1 row with amount_cents = 999 (99.5 was rejected);
#   minute_online: 3 distinct fully-elapsed minutes (session_start at
#                 -300s/-200s/-100s, pairwise >60s apart);
#   daily_revenue: revenue_cents = 999 (FINAL collapses rewrites).
ING_ROWS=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.events WHERE game_id = '$INGEST_E2E_GAME'" 2>/dev/null || echo "0")
ING_KEYS=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT uniqExact(event_id) FROM analytics.events WHERE game_id = '$INGEST_E2E_GAME'" 2>/dev/null || echo "0")
if [ "$ING_ROWS" -eq 7 ] && [ "$ING_KEYS" -eq 4 ]; then
  ok "ingest events: 7 rows collapse onto 4 natural keys (rows=$ING_ROWS, keys=$ING_KEYS)"
else
  fail "ingest event natural-key collapse wrong (rows=$ING_ROWS, keys=$ING_KEYS; expected 7 rows / 4 keys)"
  echo "--- ingest log tail ---"
  tail -20 /tmp/e2e-ingest.log 2>/dev/null || echo "(no ingest log)"
fi

ING_PAY_ROWS=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.payments WHERE game_id = '$INGEST_E2E_GAME'" 2>/dev/null || echo "0")
ING_PAY_SUM=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT sum(amount_cents) FROM analytics.payments WHERE game_id = '$INGEST_E2E_GAME'" 2>/dev/null || echo "0")
if [ "$ING_PAY_ROWS" -eq 1 ] && [ "$ING_PAY_SUM" -eq 999 ]; then
  ok "ingest payment amount intact through chain (rows=1, amount_cents=$ING_PAY_SUM)"
else
  fail "ingest payment wrong (rows=$ING_PAY_ROWS, amount_cents=$ING_PAY_SUM; expected 1 row / 999)"
fi

ING_MINUTES=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT uniqExact(m) FROM analytics.minute_online WHERE game_id = '$INGEST_E2E_GAME'" 2>/dev/null || echo "0")
if [ "$ING_MINUTES" -eq 3 ]; then
  ok "ingest cross-window online minutes (distinct=$ING_MINUTES)"
else
  fail "ingest cross-window minutes wrong (distinct=$ING_MINUTES; expected 3)"
fi

ING_REV=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT sum(revenue_cents) FROM analytics.daily_revenue FINAL WHERE game_id = '$INGEST_E2E_GAME'" 2>/dev/null || echo "0")
if [ "$ING_REV" -eq 999 ]; then
  ok "ingest daily_revenue aggregate (revenue_cents=$ING_REV)"
else
  fail "ingest daily_revenue wrong (revenue_cents=$ING_REV; expected 999)"
fi

# --- 7c. Session-length samples in ClickHouse (DATA-037, game e2e-game) ---
# The two session-end samples from section 6 must be queryable with exactly
# the predicate family the /warehouse/sessions endpoint uses: both event
# spellings and a JSONHas duration filter against the real engine.
SESS_ENDS=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT count() FROM analytics.events WHERE game_id = 'e2e-game' AND env = 'dev' AND event IN ('session_end', 'session.end') AND session_id != '' AND JSONHas(props_json, 'duration_ms')" 2>/dev/null || echo "0")
if [ "$SESS_ENDS" -eq 2 ]; then
  ok "session-length samples queryable via endpoint predicates (rows=$SESS_ENDS, both spellings)"
else
  fail "session-length samples wrong (rows=$SESS_ENDS; expected 2: session_end + session.end with duration_ms)"
fi

# --- 8. Cleanup ---
kill $WORKER_PID $INGEST_PID 2>/dev/null
if [ "${SKIP_COMPOSE_UP:-0}" != "1" ]; then
  compose down >/dev/null 2>&1
fi

echo ""
echo "==> Analytics E2E result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
