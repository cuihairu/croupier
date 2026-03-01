#!/bin/bash
# scripts/e2e/run-all.sh
# Main E2E Acceptance Test Runner
# Run all E2E tests locally or in CI

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_URL="${SERVER_URL:-http://localhost:8080}"
DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:8000}"

# Test results
declare -A RESULTS
TOTAL=0
PASSED=0
FAILED=0

run_test() {
    local name="$1"
    local script="$2"

    echo ""
    echo -e "${BLUE}>>> Running: $name${NC}"
    echo "=========================================="

    if [ ! -f "$script" ]; then
        echo -e "${RED}✗ Script not found: $script${NC}"
        RESULTS[$name]="SKIP"
        return 1
    fi

    if bash "$script"; then
        echo -e "${GREEN}✓ $name PASSED${NC}"
        RESULTS[$name]="PASS"
        ((PASSED++))
    else
        echo -e "${RED}✗ $name FAILED${NC}"
        RESULTS[$name]="FAIL"
        ((FAILED++))
    fi
    ((TOTAL++))
    echo ""
}

# Header
echo "=========================================="
echo "  Croupier E2E Acceptance Test Suite"
echo "=========================================="
echo "Server:    $SERVER_URL"
echo "Dashboard: $DASHBOARD_URL"
echo "Time:      $(date '+%Y-%m-%d %H:%M:%S')"
echo ""

# Run all test suites
run_test "Health Check" "$SCRIPT_DIR/health-check.sh"
run_test "Performance Test" "$SCRIPT_DIR/test-performance.sh"
run_test "UI Configuration Test" "$SCRIPT_DIR/test-ui-configuration.sh"

# Summary
echo "=========================================="
echo "  Test Summary"
echo "=========================================="
echo "Total:   $TOTAL"
echo -e "Passed:  ${GREEN}$PASSED${NC}"
echo -e "Failed:  ${RED}$FAILED${NC}"
echo ""

# Detailed results
echo "Detailed Results:"
for name in "${!RESULTS[@]}"; do
    result="${RESULTS[$name]}"
    if [ "$result" = "PASS" ]; then
        echo -e "  ${GREEN}✓${NC} $name: $result"
    elif [ "$result" = "FAIL" ]; then
        echo -e "  ${RED}✗${NC} $name: $result"
    else
        echo -e "  ${YELLOW}○${NC} $name: $result"
    fi
done
echo ""

# Final result
echo "=========================================="
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}ALL TESTS PASSED!${NC}"
    echo "=========================================="
    exit 0
else
    echo -e "${RED}SOME TESTS FAILED!${NC}"
    echo "=========================================="
    exit 1
fi
