#!/bin/bash
# scripts/e2e/demo-game.sh
# Demo-game E2E: real three-process stack (server + agent + demo-game) proving
# the PageSpec publish closure and the exactly-once store across restart.
#
# Chain under test (sdks/go/cmd/demo-game is a simulated integrator — it only
# imports the public Go SDK):
#   1. Boot server (sqlite) + agent (demo-game scope) + demo-game (seed 7).
#   2. Provider registration: demo-game's 34 functions surface in the scoped
#      registry via agent → server.
#   3. Publish closure: PUT draft (fixture exported from the player-manage
#      TSX template) → publish → published snapshot serves the console.
#   4. Binding execution over the published page: list / create / detail
#      against the demo store with authoritative re-reads.
#   5. Exactly-once across restart: kill demo-game, restart on the same DSN,
#      replay the SAME recharge payload + operationId → identical receipt,
#      balance unchanged; same operationId with a different payload →
#      fingerprint conflict rejected.
#
# Prerequisites: Go toolchain, jq, curl. No docker needed.
# CI: .github/workflows/ci-demo-game.yml runs this same script.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

SERVER_URL="${SERVER_URL:-http://localhost:18780}"
GAME_ID="${GAME_ID:-demo-game}"
GAME_ENV="${GAME_ENV:-demo}"
FIXTURE="$SCRIPT_DIR/fixtures/demo-game/player-manage.pagespec.json"
AGENT_CONFIG_SRC="$SCRIPT_DIR/fixtures/demo-game/agent.e2e.yaml"
PAGE_KEY="template--player-manage"
SEED="${SEED:-7}"
# Local stacks often share the machine with the deployed demo stack (its
# agent gateway holds 19091 and demo-game health 18090), so the E2E picks
# non-default local ports and rewrites the canonical fixture per run — the
# agent binary has no env override layer for config keys.
AGENT_PORT="${AGENT_PORT:-19093}"
DEMO_HTTP_PORT="${DEMO_HTTP_PORT:-18093}"

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
PASS=0; FAIL=0
ok()   { echo -e "${GREEN}PASS${NC} — $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}FAIL${NC} — $1"; FAIL=$((FAIL+1)); }
assert_eq() { # name expected actual
  if [ "$2" = "$3" ]; then ok "$1 ($3)"; else fail "$1 (expected $2, got $3)"; fi
}

SERVER_PID=""; AGENT_PID=""; GAME_PID=""
WORK="$(mktemp -d /tmp/croupier-demo-game-e2e.XXXXXX)"
cleanup() {
  [ -n "$GAME_PID" ]   && kill "$GAME_PID"   2>/dev/null
  [ -n "$AGENT_PID" ]  && kill "$AGENT_PID"  2>/dev/null
  [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null
  wait 2>/dev/null
}
trap cleanup EXIT

start_demo_game() { # $1 = log suffix
  ./bin/demo-game \
    -agent "127.0.0.1:$AGENT_PORT" \
    -game-id "$GAME_ID" -env "$GAME_ENV" \
    -dsn "$WORK/demo.db" -seed "$SEED" \
    -http-addr "127.0.0.1:$DEMO_HTTP_PORT" \
    >> "$WORK/demo-game-$1.log" 2>&1 &
  GAME_PID=$!
}

wait_readyz() { # $1 = name, $2 = max seconds
  for _ in $(seq 1 "${2:-30}"); do
    if curl -sf "http://127.0.0.1:$DEMO_HTTP_PORT/readyz" > /dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "::demo-game $1 not ready; last log:"; tail -20 "$WORK"/demo-game-*.log
  return 1
}

echo "==> Demo-game E2E (workdir $WORK)"

# --- 1. Build the three processes ---
export GOTMPDIR="${GOTMPDIR:-/tmp}"
mkdir -p bin "$GOTMPDIR"
echo "Building server / agent / demo-game..."
go build -o bin/croupier-server ./cmd/server || { fail "build server"; exit 1; }
go build -o bin/croupier-agent ./cmd/sidecar-agent || { fail "build agent"; exit 1; }
( cd sdks/go && go build -o "$REPO_ROOT/bin/demo-game" ./cmd/demo-game ) || { fail "build demo-game"; exit 1; }
ok "binaries built"

# --- 2. Boot server (sqlite, default ports) ---
: > "$WORK/server.log"
DB_DRIVER=sqlite \
DATABASE_URL="$WORK/croupier.db" \
STORAGE_DRIVER=file \
STORAGE_BASE_DIR="$WORK/uploads" \
./bin/croupier-server --config configs/test-sqlite.yaml >> "$WORK/server.log" 2>&1 &
SERVER_PID=$!
for i in $(seq 1 60); do
  curl -sf "$SERVER_URL/healthz" > /dev/null 2>&1 && break
  kill -0 "$SERVER_PID" 2>/dev/null || { fail "server exited early"; tail -30 "$WORK/server.log"; exit 1; }
  sleep 1
done
curl -sf "$SERVER_URL/healthz" > /dev/null 2>&1 && ok "server ready" || { fail "server not ready"; tail -30 "$WORK/server.log"; exit 1; }

# --- 3. Login ---
TOKEN=$(curl -s -X POST "$SERVER_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token // empty')
[ -n "$TOKEN" ] && ok "auth login" || { fail "auth login"; exit 1; }
AUTH=(-H "Authorization: Bearer $TOKEN" -H "X-Game-ID: $GAME_ID" -H "X-Env: $GAME_ENV")

# --- 4. Boot agent (demo-game scope) + demo-game ---
sed -e "s/19091/$AGENT_PORT/g" "$AGENT_CONFIG_SRC" > "$WORK/agent.e2e.yaml"
: > "$WORK/agent.log"
./bin/croupier-agent agent --config "$WORK/agent.e2e.yaml" >> "$WORK/agent.log" 2>&1 &
AGENT_PID=$!

start_demo_game "1"
wait_readyz "first boot" 30 && ok "demo-game ready (seed $SEED)" || { fail "demo-game not ready"; tail -30 "$WORK"/demo-game-*.log; exit 1; }

# --- 5. Provider registration surfaces in the scoped registry ---
registered=""
for i in $(seq 1 45); do
  if curl -s "${AUTH[@]}" "$SERVER_URL/api/v1/functions" | jq -e '.items[]? | select(.id == "player.list")' > /dev/null 2>&1; then
    registered=1; break
  fi
  sleep 2
done
if [ -n "$registered" ]; then ok "provider registration (player.list in scoped registry)"; else
  fail "player.list not registered"; tail -20 "$WORK/agent.log"; tail -20 "$WORK"/demo-game-1.log; exit 1
fi
count=$(curl -s "${AUTH[@]}" "$SERVER_URL/api/v1/functions" | jq '.items | length')
echo "    registered functions in scope: $count"

# --- 6. Proposals rebuild covers the players resource ---
code=$(curl -s -o "$WORK/rebuild.json" -w "%{http_code}" -X POST "${AUTH[@]}" \
  "$SERVER_URL/api/v1/pages/proposals/rebuild" -H "Content-Type: application/json" -d '{}')
assert_eq "POST /pages/proposals/rebuild" 200 "$code"

# --- 7. Publish closure: draft → publish → console snapshot ---
# New page: optimistic lock starts at revision 0 (response bumps to 1).
jq -c '. + {draftRevision: 0}' "$FIXTURE" > "$WORK/save-body.json"
code=$(curl -s -o "$WORK/save.json" -w "%{http_code}" -X PUT "${AUTH[@]}" \
  "$SERVER_URL/api/v1/pages/$PAGE_KEY" -H "Content-Type: application/json" \
  --data-binary "@$WORK/save-body.json")
assert_eq "PUT draft (fixture pagespec)" 200 "$code"
REV=$(jq -r '.draftRevision // empty' "$WORK/save.json")
[ -n "$REV" ] && ok "draft saved (revision $REV)" || { fail "no draftRevision in save response"; cat "$WORK/save.json"; exit 1; }

code=$(curl -s -o "$WORK/publish.json" -w "%{http_code}" -X POST "${AUTH[@]}" \
  "$SERVER_URL/api/v1/pages/$PAGE_KEY/publish" -H "Content-Type: application/json" \
  -d "{\"draftRevision\": $REV}")
assert_eq "POST publish" 200 "$code"
jq -e '.published == true' "$WORK/publish.json" > /dev/null \
  && ok "page published" || { fail "publish did not report published=true"; cat "$WORK/publish.json"; }

# Published snapshot serves the console (spec fields really landed in DB).
curl -s "${AUTH[@]}" "$SERVER_URL/api/v1/console/pages/$PAGE_KEY" > "$WORK/console-page.json"
jq -e '.page.pageKey == "template--player-manage"' "$WORK/console-page.json" > /dev/null \
  && ok "console snapshot pageKey" || { fail "console snapshot pageKey"; cat "$WORK/console-page.json"; }
BINDINGS=$(jq '.page.bindings | length' "$WORK/console-page.json")
assert_eq "console snapshot bindings" 11 "$BINDINGS"
jq -e '.page.title["zh-CN"] == "玩家管理"' "$WORK/console-page.json" > /dev/null \
  && ok "console snapshot localized title" || fail "console snapshot localized title"

# --- 8. Binding execution over the published page ---
curl -s -X POST "${AUTH[@]}" \
  "$SERVER_URL/api/v1/console/pages/$PAGE_KEY/bindings/list/execute" \
  -H "Content-Type: application/json" -d '{"context":{}}' > "$WORK/exe-list.json"
LIST_TOTAL=$(jq '.result.data.total // 0' "$WORK/exe-list.json")
[ "$LIST_TOTAL" -ge 1 ] 2>/dev/null && ok "binding list.execute (total=$LIST_TOTAL)" \
  || { fail "binding list.execute"; cat "$WORK/exe-list.json"; }

curl -s -X POST "${AUTH[@]}" \
  "$SERVER_URL/api/v1/console/pages/$PAGE_KEY/bindings/create/execute" \
  -H "Content-Type: application/json" \
  -d '{"context":{"form":{"nickname":"e2e-binding-player","level":10,"vip":1}}}' > "$WORK/exe-create.json"
jq -e '.result.data.id | length > 0' "$WORK/exe-create.json" > /dev/null \
  && ok "binding create.execute" || { fail "binding create.execute"; cat "$WORK/exe-create.json"; }

# Authoritative re-read: the created player exists via direct invoke.
NEW_ID=$(curl -s -X POST "${AUTH[@]}" "$SERVER_URL/api/v1/functions/player.list/invoke" \
  -H "Content-Type: application/json" \
  -d '{"params":{"nickname":"e2e-binding-player"},"mode":"sync"}' \
  | jq -r '.result.items[0].id // empty')
[ -n "$NEW_ID" ] && ok "created player visible via invoke ($NEW_ID)" || { fail "created player not found"; }

curl -s -X POST "${AUTH[@]}" \
  "$SERVER_URL/api/v1/console/pages/$PAGE_KEY/bindings/detail/execute" \
  -H "Content-Type: application/json" \
  -d "{\"context\":{\"row\":{\"id\":\"$NEW_ID\"}}}" > "$WORK/exe-detail.json"
jq -e '.result.data.nickname == "e2e-binding-player"' "$WORK/exe-detail.json" > /dev/null \
  && ok "binding detail.execute (authoritative re-read)" || { fail "binding detail.execute"; cat "$WORK/exe-detail.json"; }

# --- 9. Exactly-once across restart (same DSN, replayed operationId) ---
OP_ID="e2e-replay-op-1"
recharge() { # $1 = amount
  curl -s -X POST "${AUTH[@]}" "$SERVER_URL/api/v1/functions/player.recharge/invoke" \
    -H "Content-Type: application/json" \
    -d "{\"params\":{\"playerId\":\"$NEW_ID\",\"amount\":$1,\"channel\":\"e2e\",\"operationId\":\"$OP_ID\"},\"mode\":\"sync\"}"
}
balance() {
  curl -s -X POST "${AUTH[@]}" "$SERVER_URL/api/v1/functions/player.get/invoke" \
    -H "Content-Type: application/json" \
    -d "{\"params\":{\"id\":\"$NEW_ID\"},\"mode\":\"sync\"}" | jq -r '.result.balanceGold'
}

BAL0=$(balance)
recharge 648 > "$WORK/recharge-1.json"
R1=$(jq -c '.result | {success, amount}' "$WORK/recharge-1.json")
BAL1=$(balance)
assert_eq "balance after recharge" "$((BAL0 + 648))" "$BAL1"

kill "$GAME_PID"; wait "$GAME_PID" 2>/dev/null; GAME_PID=""
ok "demo-game stopped (restart replay phase)"
start_demo_game "2"
wait_readyz "second boot" 30 && ok "demo-game restarted on same DSN" || { fail "demo-game restart"; tail -30 "$WORK"/demo-game-2.log; exit 1; }
for i in $(seq 1 30); do
  R=$(recharge 648 | jq -r '.result.success // empty' 2>/dev/null)
  [ "$R" = "true" ] && break
  sleep 2
done
recharge 648 > "$WORK/recharge-2.json"
R2=$(jq -c '.result | {success, amount}' "$WORK/recharge-2.json")
[ -n "$R1" ] && [ "$R1" = "$R2" ] && ok "replay after restart returns identical receipt ($R2)" \
  || { fail "replay receipt mismatch ($R1 vs $R2)"; cat "$WORK/recharge-2.json"; }
BAL2=$(balance)
assert_eq "balance unchanged after replay" "$BAL1" "$BAL2"

# Same operationId + different payload → fingerprint conflict.
recharge 999 > "$WORK/recharge-conflict.json"
if jq -e '.result' "$WORK/recharge-conflict.json" > /dev/null 2>&1 \
   && [ "$(jq -r '.result.success // empty' "$WORK/recharge-conflict.json")" != "true" ]; then
  ok "same operationId with different payload rejected"
else
  fail "fingerprint conflict not rejected"; cat "$WORK/recharge-conflict.json"
fi

# --- Summary ---
echo ""
echo "==> Demo-game E2E result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
exit 0
