#!/bin/bash
# scripts/e2e/test-performance.sh
# API Performance Testing

set -euo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:18780}"
DASHBOARD_URL="${DASHBOARD_URL:-}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
TOKEN=""

echo "=========================================="
echo "  API Performance Test"
echo "=========================================="
echo "Server: $SERVER_URL"
echo ""

# Check if bc is available
if ! command -v bc &> /dev/null; then
    echo "Error: 'bc' command not found. Please install bc."
    echo "  Ubuntu/Debian: sudo apt-get install bc"
    echo "  macOS: bc is pre-installed"
    exit 1
fi

test_endpoint() {
    local name="$1"
    local url="$2"
    local threshold_warn="${3:-0.5}"
    local threshold_slow="${4:-1.0}"
    local requires_auth="${5:-}"
    local iterations=10

    printf "%-40s" "$name"

    # Warm up
    if [ -n "$requires_auth" ]; then
        curl -s -H "Authorization: Bearer $TOKEN" -o /dev/null "$url" > /dev/null 2>&1
    else
        curl -s -o /dev/null "$url" > /dev/null 2>&1
    fi

    # Measure iterations
    total=0.0
    for i in $(seq 1 $iterations); do
        if [ -n "$requires_auth" ]; then
            time=$(curl -s -H "Authorization: Bearer $TOKEN" -o /dev/null -w "%{time_total}" "$url" 2>/dev/null || echo "0")
        else
            time=$(curl -s -o /dev/null -w "%{time_total}" "$url" 2>/dev/null || echo "0")
        fi
        total=$(echo "$total + $time" | bc 2>/dev/null || echo "999")
    done

    avg=$(echo "scale=3; $total / $iterations" | bc 2>/dev/null || echo "0")

    # Color output based on performance
    if (( $(echo "$avg < $threshold_warn" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "\033[0;32m${avg}s\033[0m (Excellent <${threshold_warn}s)"
    elif (( $(echo "$avg < $threshold_slow" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "\033[0;32m${avg}s\033[0m (Good <${threshold_slow}s)"
    elif (( $(echo "$avg < 2.0" | bc -l 2>/dev/null || echo "0") )); then
        echo -e "\033[1;33m${avg}s\033[0m (Acceptable <2.0s)"
    else
        echo -e "\033[0;31m${avg}s\033[0m (Slow >=2.0s)"
    fi
}

TOKEN=$(curl -s -X POST "$SERVER_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r '.token // empty')

if [ -z "$TOKEN" ]; then
    echo "Error: Failed to obtain auth token"
    exit 1
fi

echo "--- Core API Endpoints ---"
test_endpoint "Health Check" "$SERVER_URL/healthz" 0.05 0.1
test_endpoint "Function Descriptors" "$SERVER_URL/api/v1/functions/descriptors" 0.2 0.5 auth
test_endpoint "Function List" "$SERVER_URL/api/v1/functions" 0.2 0.5 auth
test_endpoint "Agents Status" "$SERVER_URL/api/v1/ops/agents" 0.1 0.3 auth
test_endpoint "Games List" "$SERVER_URL/api/v1/games" 0.2 0.5 auth

echo ""
echo "--- Function Detail Endpoints ---"
test_endpoint "Function OpenAPI" "$SERVER_URL/api/v1/functions/player.ban/openapi" 0.1 0.3 auth
test_endpoint "Function UI Config" "$SERVER_URL/api/v1/functions/player.ban/ui" 0.1 0.3 auth
test_endpoint "Function Route Config" "$SERVER_URL/api/v1/functions/player.ban/route" 0.1 0.3 auth

echo ""
if [ -n "$DASHBOARD_URL" ]; then
    echo "--- Dashboard Pages ---"
    test_endpoint "Dashboard Home" "$DASHBOARD_URL" 1.0 3.0
fi

echo ""
echo "=========================================="
echo "  Performance Test Complete"
echo "=========================================="
echo ""
echo "Notes:"
echo "- Green: Excellent performance"
echo "- Yellow: Acceptable performance"
echo "- Red: Needs optimization"
echo ""
echo "Run multiple times to get consistent results."
