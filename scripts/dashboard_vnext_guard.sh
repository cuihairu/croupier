#!/bin/bash
# dashboard_vnext_guard.sh - Guard against non-vNext model regression
#
# This script checks that the codebase only uses vNext models:
# - FunctionContract (not FunctionSpec)
# - PageProposal (not GeneratedPageSpec)
# - PageSpecV2 (not old PageSpec with FormilySchema)
# - SelectorAST (not raw inputMapping/outputMapping)
#
# Exit code:
#   0 = all checks pass
#   1 = non-vNext model detected

set -e

echo "=== Dashboard vNext Guard ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

# Function to check for forbidden patterns
check_forbidden() {
    local pattern="$1"
    local description="$2"
    local exclude_dirs="$3"

    local find_cmd="grep -rn \"$pattern\" --include=\"*.go\" --include=\"*.ts\" --include=\"*.tsx\""

    # Add exclude directories
    if [ -n "$exclude_dirs" ]; then
        for dir in $exclude_dirs; do
            find_cmd="$find_cmd --exclude-dir=$dir"
        done
    fi

    local results
    results=$(eval "$find_cmd" . 2>/dev/null | grep -v "internal/model" | grep -v "internal/dashboard/spec" | grep -v "internal/dashboard/generator" | grep -v "internal/api/console" | grep -v "internal/api/page" | grep -v "internal/api/function" | grep -v "internal/logic/function" | grep -v "internal/dashboard/normalizer" | grep -v "internal/dashboard/freshness" | grep -v "internal/api/resource" || true)

    if [ -n "$results" ]; then
        echo -e "${RED}ERROR: $description${NC}"
        echo "$results" | head -5
        if [ $(echo "$results" | wc -l) -gt 5 ]; then
            echo "  ... and $(($(echo "$results" | wc -l) - 5)) more"
        fi
        echo ""
        ERRORS=$((ERRORS + 1))
    fi
}

# Function to check for deprecated patterns (warnings only)
check_deprecated() {
    local pattern="$1"
    local description="$2"

    local results
    results=$(grep -rn "$pattern" --include="*.go" . 2>/dev/null || true)

    if [ -n "$results" ]; then
        echo -e "${YELLOW}WARNING: $description${NC}"
        echo "$results" | head -3
        echo ""
        WARNINGS=$((WARNINGS + 1))
    fi
}

echo "Checking for non-vNext models..."

# Check for old FormilySchema usage (except in spec/types.go where it's deprecated, frontend code, and model/page_spec.go)
check_forbidden "FormilySchema" "Old FormilySchema type usage" "vendor web internal/dashboard/spec internal/model internal/dashboard/generator internal/api/console internal/api/page internal/api/function internal/logic/function"

# Check for old FunctionSpec (except in spec/types.go, normalizer, generator, freshness)
check_forbidden "spec\.FunctionSpec" "Old FunctionSpec type usage" "vendor web internal/dashboard/spec internal/dashboard/normalizer internal/dashboard/generator internal/dashboard/freshness"

# Check for old GeneratedPageSpec (frontend code excluded, spec/types.go excluded)
check_forbidden "GeneratedPageSpec" "Old GeneratedPageSpec type usage" "vendor web internal/dashboard/spec internal/dashboard/generator"

# Check for old inputMapping/outputMapping JSON objects (frontend code excluded, spec/types.go excluded)
check_forbidden "InputMapping.*json\.RawMessage" "Old inputMapping JSON object" "vendor web internal/dashboard/spec internal/dashboard/generator internal/model"
check_forbidden "OutputMapping.*json\.RawMessage" "Old outputMapping JSON object" "vendor web internal/dashboard/spec internal/dashboard/generator internal/model"

# Check for old page schema validator
check_forbidden "pageComponentContracts" "Old page schema validator" "vendor"
check_forbidden "validatePageSchema" "Old page schema validator function" "vendor"

# Check for old Formily component references (frontend code excluded, test files excluded)
check_forbidden "x-component.*Input" "Formily component reference" "vendor web internal/api/function"
check_forbidden "x-component.*Select" "Formily component reference" "vendor web internal/api/function"
check_forbidden "x-component.*Switch" "Formily component reference" "vendor web internal/api/function"

echo ""
echo "Checking for deprecated patterns..."

# Check for deprecated patterns (warnings)
check_deprecated "formily-page:1" "Old Formily page schema version"

echo ""
echo "=== Summary ==="

if [ $ERRORS -gt 0 ]; then
    echo -e "${RED}FAILED: $ERRORS non-vNext patterns detected${NC}"
    echo "Please migrate to vNext models before merging."
    exit 1
fi

if [ $WARNINGS -gt 0 ]; then
    echo -e "${YELLOW}WARNINGS: $WARNINGS deprecated patterns detected${NC}"
    echo "Consider migrating these patterns to vNext."
fi

echo -e "${GREEN}PASSED: All vNext checks passed${NC}"
exit 0
