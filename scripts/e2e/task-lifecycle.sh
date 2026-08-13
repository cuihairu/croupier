#!/bin/bash
# scripts/e2e/task-lifecycle.sh
# Task lifecycle E2E: startTask → events (started/progress/completed) and
# startTask → cancelTask → events (cancel_requested/cancelled).
#
# Covers the full chain: REST task → Dispatcher (picks agent from reg.Store)
# → Agent TCP session → mock agent (e2e-agent-probe) runs TaskRunner-equivalent
# → TaskEvent streamed back via MuxConn.Send → server persists to DB →
# GET /tasks/:id/events returns them. This catches protocol drift between
# REST task, Dispatcher, Agent handler, and the TaskEvent schema.
#
# Prerequisites: a running server booted with the SQLite config, plus the two
# helper binaries built under ./bin:
#   - e2e-agent-probe (mock agent: registers the function, serves StartTask/Cancel)
#   - e2e-function-seed (inserts the function metadata row the REST entrypoint requires)

set -uo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:18780}"
AGENT_ADDR="${AGENT_ADDR:-127.0.0.1:19090}"
DSN="${DSN:-test-data/croupier.db}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
FUNCTION_ID="${FUNCTION_ID:-e2e.echo}"
GAME_ID="${GAME_ID:-e2e-game}"
ENV="${ENV:-dev}"
PROBE_BIN="${PROBE_BIN:-./bin/e2e-agent-probe}"
SEED_BIN="${SEED_BIN:-./bin/e2e-function-seed}"

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
PASS=0; FAIL=0
ok()   { echo -e "${GREEN}PASS${NC} — $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}FAIL${NC} — $1"; FAIL=$((FAIL+1)); }

echo "==> Task lifecycle E2E against $SERVER_URL (function=$FUNCTION_ID)"

# 0. Seed function metadata into the DB. Service.Start requires the function
#    row to exist (FindByFunctionID); there is no single-function create API.
if ! "$SEED_BIN" -dsn "$DSN" -function-id "$FUNCTION_ID" -game-id "$GAME_ID" >/tmp/e2e-seed.log 2>&1; then
  echo "::error::function-seed failed:"; cat /tmp/e2e-seed.log
  fail "seed function $FUNCTION_ID"
  echo ""; echo "==> result: $PASS passed, $FAIL failed"; exit 1
fi
ok "seed function $FUNCTION_ID"

# 1. Login.
TOKEN=$(curl -s -X POST "$SERVER_URL/api/v1/auth/login" -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r '.token // empty')
if [ -n "$TOKEN" ]; then ok "auth login"; else fail "auth login"; exit 1; fi
AUTH=(-H "Authorization: Bearer $TOKEN")

# 2. Start the mock agent in serve mode. It registers FUNCTION_ID and exits
#    after handling 2 StartTask requests (exit-after-tasks), so the harness
#    can wait deterministically instead of guessing timing.
"$PROBE_BIN" -addr "$AGENT_ADDR" -agent-id e2e-probe -game-id "$GAME_ID" -env "$ENV" \
  -mock-task "$FUNCTION_ID" -exit-after-tasks 2 -serve-duration 60s -ttl-seconds 120 \
  >/tmp/e2e-probe.log 2>&1 &
PROBE_PID=$!
PROBE_READY=0
for i in $(seq 1 40); do
  if grep -q "serving function" /tmp/e2e-probe.log 2>/dev/null; then PROBE_READY=1; break; fi
  if ! kill -0 "$PROBE_PID" 2>/dev/null; then break; fi
  sleep 0.25
done
if [ "$PROBE_READY" -eq 1 ]; then
  ok "probe serving $FUNCTION_ID"
else
  fail "probe serving"; cat /tmp/e2e-probe.log
  echo ""; echo "==> result: $PASS passed, $FAIL failed"; exit 1
fi

BODY='{"functionId":"'"$FUNCTION_ID"'","gameId":"'"$GAME_ID"'","env":"'"$ENV"'"}'

# 3. Task 1 — complete lifecycle: started → ... → completed.
T1=$(curl -s -X POST "$SERVER_URL/api/v1/tasks" "${AUTH[@]}" -H "Content-Type: application/json" -d "$BODY" | jq -r '.taskId // empty')
if [ -n "$T1" ]; then ok "startTask 1 → $T1"; else fail "startTask 1 (no task_id)"; fi
EV1=""
for i in $(seq 1 40); do
  EV1=$(curl -s "$SERVER_URL/api/v1/tasks/$T1/events" "${AUTH[@]}" | jq -r '[.items[].type] | join(",")' 2>/dev/null)
  echo "$EV1" | grep -q completed && break
  sleep 0.25
done
if echo "$EV1" | grep -q started && echo "$EV1" | grep -q completed; then
  ok "task1 lifecycle: $EV1"
else
  fail "task1 lifecycle (expected started+completed, got: $EV1)"
fi

# 4. Task 2 — cancel lifecycle: started → ... → cancel_requested → cancelled.
T2=$(curl -s -X POST "$SERVER_URL/api/v1/tasks" "${AUTH[@]}" -H "Content-Type: application/json" -d "$BODY" | jq -r '.taskId // empty')
if [ -n "$T2" ]; then ok "startTask 2 → $T2"; else fail "startTask 2 (no task_id)"; fi
sleep 0.3
curl -s -X POST "$SERVER_URL/api/v1/tasks/$T2/cancel" "${AUTH[@]}" -H "Content-Type: application/json" -d '{}' >/dev/null
EV2=""
for i in $(seq 1 40); do
  EV2=$(curl -s "$SERVER_URL/api/v1/tasks/$T2/events" "${AUTH[@]}" | jq -r '[.items[].type] | join(",")' 2>/dev/null)
  echo "$EV2" | grep -q cancelled && break
  sleep 0.25
done
if echo "$EV2" | grep -q cancel_requested && echo "$EV2" | grep -q cancelled; then
  ok "task2 cancel lifecycle: $EV2"
else
  fail "task2 cancel lifecycle (expected cancel_requested+cancelled, got: $EV2)"
fi

# 5. Probe must have handled both tasks and exited 0.
wait "$PROBE_PID" 2>/dev/null
PROBE_RC=$?
if [ "$PROBE_RC" -eq 0 ] && grep -q "served tasks=2" /tmp/e2e-probe.log; then
  ok "probe handled 2 tasks (exit 0)"
else
  fail "probe result (rc=$PROBE_RC)"; tail -3 /tmp/e2e-probe.log
fi

echo ""
echo "==> result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
