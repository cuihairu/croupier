#!/bin/bash
# scripts/e2e/health-check.sh
# E2E Health Check Script for Croupier

set -euo pipefail

# Configuration
SERVER_URL="${SERVER_URL:-http://localhost:18780}"
DASHBOARD_URL="${DASHBOARD_URL:-}"
GAME_ID="${GAME_ID:-test-game}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
TOKEN=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counters
PASSED=0
FAILED=0

# Test function
test_service() {
    local name="$1"
    local url="$2"
    local expected_code="${3:-200}"
    local requires_auth="${4:-}"

    echo -n "Testing $name... "
    if [ -n "$requires_auth" ]; then
        code=$(curl -s -H "Authorization: Bearer $TOKEN" -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    else
        code=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    fi

    if [ "$code" = "$expected_code" ]; then
        echo -e "${GREEN}PASS${NC} ($code)"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}FAIL${NC} (expected $expected_code, got $code)"
        ((FAILED++))
        return 1
    fi
}

login() {
    TOKEN=$(curl -s -X POST "$SERVER_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r '.token // empty')

    if [ -z "$TOKEN" ]; then
        echo -e "${RED}Failed to obtain auth token${NC}"
        exit 1
    fi
}

echo "=========================================="
echo "  Croupier E2E Health Check"
echo "=========================================="
echo "Server:    $SERVER_URL"
if [ -n "$DASHBOARD_URL" ]; then
    echo "Dashboard: $DASHBOARD_URL"
else
    echo "Dashboard: (skip)"
fi
echo "Game ID:   $GAME_ID"
echo ""

login

# Service health checks
echo "--- Service Health ---"
test_service "Server Health" "$SERVER_URL/healthz"
test_service "Agent Status" "$SERVER_URL/api/v1/ops/agents" 200 auth
test_service "Function Descriptors" "$SERVER_URL/api/v1/functions/descriptors" 200 auth
if [ -n "$DASHBOARD_URL" ]; then
    test_service "Dashboard" "$DASHBOARD_URL"
fi

# API endpoint checks
echo ""
echo "--- API Endpoints ---"
test_service "Function List" "$SERVER_URL/api/v1/functions" 200 auth
test_service "Games List" "$SERVER_URL/api/v1/games" 200 auth
test_service "Audit Logs" "$SERVER_URL/api/v1/audit" 200 auth

echo ""
echo "=========================================="
echo "Results"
echo "=========================================="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
