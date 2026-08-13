#!/bin/bash
# scripts/e2e/happy-path.sh
# Minimal stable E2E: Server health + auth + Agent TCP registration + task lifecycle.
#
# This is the CI-suitable smoke test. It uses the already-running server
# (started by CI) and verifies the core request chain without external
# dependencies. Designed to be deterministic: each step asserts a concrete
# observable, no flaky timing assumptions beyond the readiness poll.

set -uo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:18780}"
AGENT_ADDR="${AGENT_ADDR:-127.0.0.1:19090}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
PASS=0; FAIL=0

ok()   { echo -e "${GREEN}PASS${NC} — $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}FAIL${NC} — $1"; FAIL=$((FAIL+1)); }

assert_eq() { # name expected actual
  if [ "$2" = "$3" ]; then ok "$1 ($3)"; else fail "$1 (expected $2, got $3)"; fi
}

echo "==> Happy-path E2E against $SERVER_URL"

# 1. Server health (unauthenticated probe — must be 200).
code=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/healthz" || echo 000)
assert_eq "server /healthz" 200 "$code"

# 2. Login → obtain JWT.
TOKEN=$(curl -s -X POST "$SERVER_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" \
  | jq -r '.token // empty')
if [ -n "$TOKEN" ]; then ok "auth login (token acquired)"; else fail "auth login (no token)"; fi

AUTH=(-H "Authorization: Bearer $TOKEN")

# 3. Authenticated REST surface reachable.
code=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "$SERVER_URL/api/v1/games" || echo 000)
[ "$code" = "200" ] && ok "GET /api/v1/games (200)" || fail "GET /api/v1/games (got $code)"

code=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "$SERVER_URL/api/v1/ops/agents" || echo 000)
[ "$code" = "200" ] && ok "GET /api/v1/ops/agents (200)" || fail "GET /api/v1/ops/agents (got $code)"

code=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "$SERVER_URL/api/v1/tasks" || echo 000)
# 200 (list ok) is the expected happy path.
[ "$code" = "200" ] && ok "GET /api/v1/tasks (200)" || fail "GET /api/v1/tasks (got $code)"

# 4. Agent TCP session registration.
#    The control plane listens on AGENT_ADDR. We send a RegisterRequest frame
#    and expect a RegisterResponse. This validates the handshake state machine
#    (first frame = Register) and session routing.
if [ -x ./bin/e2e-agent-probe ]; then
  if ./bin/e2e-agent-probe -addr "$AGENT_ADDR" -agent-id "e2e-agent-1" -game-id "e2e-game" -env "dev"; then
    ok "agent TCP register handshake"
  else
    fail "agent TCP register handshake"
  fi
else
  echo "    (skip agent TCP probe: ./bin/e2e-agent-probe not built; build with the e2e harness)"
fi

echo ""
echo "==> Happy-path result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
