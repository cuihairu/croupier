#!/bin/bash
# dashboard_vnext_guard.sh - Guard the canonical Dashboard PageSpec model.
#
# This script intentionally keeps the historical filename because CI and
# handoff docs already call it, but the checked model is no longer a parallel
# next-version path. Dashboard has one page contract:
# PublishedPageSpec[] -> ConsoleMenuSpec -> ProLayout -> PageRenderer.

set -e

echo "=== Dashboard PageSpec Guard ==="
echo ""

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

ERRORS=0

fail() {
    echo -e "${RED}ERROR: $1${NC}"
    ERRORS=$((ERRORS + 1))
}

ok() {
    echo -e "${GREEN}OK: $1${NC}"
}

assert_file_exists() {
    local path="$1"
    local label="$2"
    if [ -f "$path" ]; then
        ok "$label exists"
    else
        fail "$label missing: $path"
    fi
}

assert_file_absent() {
    local path="$1"
    local label="$2"
    if [ -e "$path" ]; then
        fail "$label must be physically removed: $path"
    else
        ok "$label absent"
    fi
}

assert_contains() {
    local path="$1"
    local pattern="$2"
    local label="$3"
    if grep -q "$pattern" "$path"; then
        ok "$label"
    else
        fail "$label not found in $path"
    fi
}

assert_not_contains() {
    local scope="$1"
    local pattern="$2"
    local label="$3"
    if rg -n "$pattern" $scope >/tmp/croupier-dashboard-guard-match.txt 2>/dev/null; then
        fail "$label"
        sed -n '1,20p' /tmp/croupier-dashboard-guard-match.txt
    else
        ok "$label"
    fi
}

echo "Checking canonical model entry points..."
assert_file_exists "internal/dashboard/spec/types.go" "Go Dashboard spec"
assert_file_exists "web/src/types/dashboard.ts" "TypeScript Dashboard spec"
assert_file_exists "web/src/components/PageRenderer/index.tsx" "PageRenderer"
assert_file_exists "web/src/pages/PageStudio/index.tsx" "PageStudio"
assert_file_exists "web/src/pages/Console/Page.tsx" "Console page runtime"
assert_file_exists "web/src/utils/consoleMenu.ts" "Console menu builder"

echo ""
echo "Checking current routing and renderer contract..."
assert_contains "web/config/routes.ts" "component: './PageStudio'" "PageStudio route uses canonical editor"
assert_contains "web/config/routes.ts" "path: '/console/:categoryKey/:pageKey'" "Console dynamic page route is registered"
assert_contains "web/src/pages/Console/Page.tsx" "PageRenderer" "Console runtime renders PageRenderer"
assert_contains "web/src/utils/consoleMenu.ts" "ConsoleMenuSpec" "Menu builder consumes ConsoleMenuSpec"
assert_contains "internal/api/page/service.go" "page-spec:1" "Page API publishes page-spec:1"

echo ""
echo "Checking removed legacy files stay removed..."
assert_file_absent "internal/dashboard/generator/generator_v2.go" "legacy split generator"
assert_file_absent "web/src/types/dashboard-vnext.ts" "legacy split-model types"
assert_file_absent "web/src/services/dashboard-vnext.ts" "legacy split-model service"
assert_file_absent "web/src/pages/PageStudioV2" "legacy PageStudioV2"
assert_file_absent "web/src/pages/PageStudio/PageSchemaEditor.tsx" "legacy PageSchemaEditor"
assert_file_absent "web/src/components/FormilyPageRenderer" "legacy Formily page renderer"

echo ""
echo "Checking legacy Page protocol terms do not return..."
assert_not_contains "internal/dashboard internal/api/page internal/model web/src/types web/src/pages/PageStudio web/src/pages/Console web/src/components/PageRenderer web/src/services/dashboard.ts" "\\b(PageSpecV2|GeneratedPageSpecV2|PageTypeV2|PageFunctionBindingV2|PageSchemaEditor|PageStudioV2)\\b|formily-page:1|formily-page/v1|dashboard-vnext" "legacy split/page-schema identifiers absent"
assert_not_contains "internal/dashboard internal/api/page internal/model web/src/types web/src/pages/PageStudio web/src/pages/Console web/src/components/PageRenderer web/src/services/dashboard.ts" "\\b(inputMapping|outputMapping|SchemaJSON|PageSpecJSON|FormPresentationJSON|PageSpecVersion)\\b" "legacy page storage/mapping fields absent"
assert_not_contains "web/src/types/dashboard.ts web/src/components/PageRenderer web/src/pages/PageStudio web/src/pages/Console web/src/services/dashboard.ts" "\\bany\\b" "core Dashboard TypeScript code has no any"

echo ""
echo "Checking dependencies..."
if grep -q '"@formily/' web/package.json; then
    fail "Formily runtime dependency must not be used"
else
    ok "Formily runtime dependency absent"
fi

if grep -q '"@ant-design/charts"' web/package.json; then
    ok "@ant-design/charts dependency present"
else
    fail "@ant-design/charts dependency missing for ReportPageRenderer"
fi

echo ""
echo "=== Summary ==="
if [ $ERRORS -gt 0 ]; then
    echo -e "${RED}FAILED: $ERRORS issue(s) detected${NC}"
    exit 1
fi

echo -e "${GREEN}PASSED: Dashboard PageSpec guard passed${NC}"
