#!/bin/bash
# scripts/e2e/test-performance.sh
# API Performance Testing

set -euo pipefail

SERVER_URL="${SERVER_URL:-http://localhost:8080}"

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
    local iterations=10

    printf "%-40s" "$name"

    # Warm up
    curl -s -o /dev/null "$url" > /dev/null 2>&1

    # Measure iterations
    total=0.0
    for i in $(seq 1 $iterations); do
        time=$(curl -s -o /dev/null -w "%{time_total}" "$url" 2>/dev/null || echo "0")
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

echo "--- Core API Endpoints ---"
test_endpoint "Health Check" "$SERVER_URL/healthz" 0.05 0.1
test_endpoint "Function Descriptors" "$SERVER_URL/api/v1/functions/descriptors" 0.2 0.5
test_endpoint "Function List" "$SERVER_URL/api/v1/functions" 0.2 0.5
test_endpoint "Agents Status" "$SERVER_URL/api/v1/agents" 0.1 0.3
test_endpoint "Games List" "$SERVER_URL/api/v1/games" 0.2 0.5

echo ""
echo "--- Function Detail Endpoints ---"
test_endpoint "Function OpenAPI" "$SERVER_URL/api/v1/functions/test.player.addCurrency/openapi" 0.1 0.3
test_endpoint "Function UI Config" "$SERVER_URL/api/v1/functions/test.player.addCurrency/ui" 0.1 0.3
test_endpoint "Function Route Config" "$SERVER_URL/api/v1/functions/test.player.addCurrency/route" 0.1 0.3

echo ""
echo "--- Dashboard Pages ---"
test_endpoint "Dashboard Home" "http://localhost:8000" 1.0 3.0

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
