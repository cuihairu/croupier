#!/bin/bash
# scripts/e2e/health-check.sh
# E2E Health Check Script for Croupier

set -euo pipefail

# Configuration
SERVER_URL="${SERVER_URL:-http://localhost:8080}"
DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:8000}"
GAME_ID="${GAME_ID:-test-game}"

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

    echo -n "Testing $name... "
    code=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")

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

echo "=========================================="
echo "  Croupier E2E Health Check"
echo "=========================================="
echo "Server:    $SERVER_URL"
echo "Dashboard: $DASHBOARD_URL"
echo "Game ID:   $GAME_ID"
echo ""

# Service health checks
echo "--- Service Health ---"
test_service "Server Health" "$SERVER_URL/healthz"
test_service "Agent Status" "$SERVER_URL/api/v1/agents"
test_service "Function Descriptors" "$SERVER_URL/api/v1/functions/descriptors"
test_service "Dashboard" "$DASHBOARD_URL"

# API endpoint checks
echo ""
echo "--- API Endpoints ---"
test_service "Function List" "$SERVER_URL/api/v1/functions"
test_service "Games List" "$SERVER_URL/api/v1/games"
test_service "Audit Logs" "$SERVER_URL/api/v1/audit"

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
