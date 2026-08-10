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

# Check for rg (ripgrep) availability, fallback to grep if not found
if ! command -v rg &> /dev/null; then
    echo "WARNING: rg (ripgrep) not found, using grep as fallback"
    rg() {
        grep -r "$@"
    }
fi

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

assert_dir_has_no_files() {
    local path="$1"
    local label="$2"
    if [ -d "$path" ] && find "$path" -type f | grep -q .; then
        fail "$label must not contain files: $path"
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
    local pattern="$1"
    local label="$2"
    shift 2
    if command -v rg &> /dev/null; then
        if rg -n "$pattern" "$@" >/tmp/croupier-dashboard-guard-match.txt 2>/dev/null; then
            fail "$label"
            sed -n '1,20p' /tmp/croupier-dashboard-guard-match.txt
        else
            ok "$label"
        fi
    else
        # Fallback to grep when rg is not available
        # Exclude test files and common non-production paths
        local grep_args="--include=*.go --include=*.ts --include=*.tsx --include=*.proto --exclude=*_test.go --exclude=*.test.ts --exclude=*.test.tsx --exclude-dir=test --exclude-dir=tests --exclude-dir=vendor --exclude-dir=node_modules"
        if grep -rn $grep_args "$pattern" "$@" >/tmp/croupier-dashboard-guard-match.txt 2>/dev/null; then
            fail "$label"
            sed -n '1,20p' /tmp/croupier-dashboard-guard-match.txt
        else
            ok "$label"
        fi
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
assert_file_absent "web/src/components/FunctionFormManager" "legacy FunctionForm manager"
assert_file_absent "web/src/components/formily" "legacy Formily component runtime"
assert_dir_has_no_files "internal/dashboard/descriptors" "legacy request-time descriptor collector"

echo ""
echo "Checking legacy Page protocol terms do not return..."
assert_not_contains "\\b(PageSpecV2|GeneratedPageSpecV2|PageTypeV2|PageFunctionBindingV2|PageSchemaEditor|PageStudioV2)\\b|formily-page:1|formily-page/v1|dashboard-vnext" "legacy split/page-schema identifiers absent" "internal/dashboard" "internal/api/page" "internal/model" "web/src/types" "web/src/pages/PageStudio" "web/src/pages/Console" "web/src/components/PageRenderer" "web/src/services/dashboard.ts"
assert_not_contains "\\b(inputMapping|outputMapping|SchemaJSON|PageSpecJSON|FormPresentationJSON|PageSpecVersion)\\b" "legacy page storage/mapping fields absent" "internal/dashboard" "internal/api/page" "internal/model" "web/src/types" "web/src/pages/PageStudio" "web/src/pages/Console" "web/src/components/PageRenderer" "web/src/services/dashboard.ts"
assert_not_contains "\\b(WorkspaceRenderer|PageGenerator|FunctionUIManager|FunctionFormRenderer|XUISchema)\\b" "legacy page runtime references absent" "web/src" "web/package.json"
assert_not_contains "dashboard/descriptors|descriptors\\.Collect" "legacy descriptor collector imports absent" "internal" "web/src"
if command -v rg &> /dev/null; then
    if rg -nP "(?<![\"'])\\bany\\b(?![\"'])" "web/src/types/dashboard.ts" "web/src/components/PageRenderer" "web/src/pages/PageStudio" "web/src/pages/Console" "web/src/services/dashboard.ts" >/tmp/croupier-dashboard-guard-match.txt 2>/dev/null; then
        fail "core Dashboard TypeScript code has no any"
        sed -n '1,20p' /tmp/croupier-dashboard-guard-match.txt
    else
        ok "core Dashboard TypeScript code has no any"
    fi
else
    # Fallback to grep - exclude test files
    if grep -rn --include="*.ts" --include="*.tsx" --exclude="*.test.ts" --exclude="*.test.tsx" "any" "web/src/types/dashboard.ts" "web/src/components/PageRenderer" "web/src/pages/PageStudio" "web/src/pages/Console" "web/src/services/dashboard.ts" 2>/dev/null | grep -v "test" >/tmp/croupier-dashboard-guard-match.txt 2>/dev/null; then
        fail "core Dashboard TypeScript code has no any"
        sed -n '1,20p' /tmp/croupier-dashboard-guard-match.txt
    else
        ok "core Dashboard TypeScript code has no any"
    fi
fi

echo ""
echo "Checking form runtime boundary..."
assert_not_contains "@formily/|form-render|components/formily|FunctionFormManager|generateFormily|validateFormily|BuildFallbackFormSchema" "legacy Formily/FunctionForm runtime absent" "web/src" "web/package.json"
assert_not_contains "\\bFunctionForm|BuildFallbackFormSchema|fallbackFormilyComponent|generateFormily|validateFormily" "legacy backend FunctionForm APIs absent" "internal/logic" "internal/api" "internal/dashboard" "pkg" "cmd"
assert_contains "web/src/components/SchemaFormRenderer/index.tsx" "@rjsf/antd" "SchemaFormRenderer uses @rjsf/antd adapter"
assert_contains "web/src/components/SchemaFormRenderer/index.tsx" "@rjsf/validator-ajv8" "SchemaFormRenderer uses ajv8 validator"

echo ""
echo "Checking execution and approval boundary..."
assert_not_contains "FunctionExecutionApproval|GenerateApprovalPageForOperation|isApprovalOperation|execution must be one of sync, task, approval|x-execution must be one of sync, task, approval|sync\\|task\\|approval|sync/task/approval|execution === 'approval'" "approval is not a FunctionContract execution mode" "internal/dashboard" "internal/api/openapi" "internal/platform/openapi" "internal/model" "proto/croupier" "web/src/types/dashboard.ts" "web/src/pages/OpenAPISources" "sdks/go/pkg/croupier" "sdks/java/src/main" "sdks/cpp/include"

echo ""
echo "Checking registration-side UI fields stay rejected..."
if command -v rg &> /dev/null; then
    if rg -n "category_display|entity_display|operation_display|operation_kind|page_hint|x-labels" \
        proto/croupier sdks/go sdks/js/src sdks/python sdks/java/src sdks/csharp/src sdks/cpp/include sdks/cpp/src \
        internal/agent internal/app/agent internal/api/openapi internal/platform/openapi internal/function internal/platform/registry \
        --glob "*.proto" --glob "*.go" --glob "*.ts" --glob "*.tsx" --glob "*.py" --glob "*.java" --glob "*.cs" --glob "*.h" --glob "*.cpp" \
        --glob "!**/test/**" --glob "!**/tests/**" --glob "!**/*Test.java" --glob "!**/*.test.ts" \
        --glob "!**/build/**" --glob "!**/obj/**" --glob "!**/bin/**" \
        --glob "!internal/function/registrationguard/reject.go" >/tmp/croupier-dashboard-guard-match.txt 2>/dev/null; then
        fail "registration-side UI/display fields must not reappear in SDK/OpenAPI registration sources"
        sed -n '1,20p' /tmp/croupier-dashboard-guard-match.txt
    else
        ok "registration-side UI/display fields absent from SDK/OpenAPI registration sources"
    fi
else
    # Fallback to grep - exclude test files
    local grep_args="--include=*.proto --include=*.go --include=*.ts --include=*.tsx --include=*.py --include=*.java --include=*.cs --include=*.h --include=*.cpp --exclude=*_test.go --exclude=*.test.ts --exclude=*.test.tsx --exclude-dir=test --exclude-dir=tests --exclude-dir=build --exclude-dir=obj --exclude-dir=bin"
    if grep -rn $grep_args "category_display\|entity_display\|operation_display\|operation_kind\|page_hint\|x-labels" \
        proto/croupier sdks/go sdks/js/src sdks/python sdks/java/src sdks/csharp/src sdks/cpp/include sdks/cpp/src \
        internal/agent internal/app/agent internal/api/openapi internal/platform/openapi internal/function internal/platform/registry \
        2>/dev/null | grep -v "registrationguard/reject.go" >/tmp/croupier-dashboard-guard-match.txt 2>/dev/null; then
        fail "registration-side UI/display fields must not reappear in SDK/OpenAPI registration sources"
        sed -n '1,20p' /tmp/croupier-dashboard-guard-match.txt
    else
        ok "registration-side UI/display fields absent from SDK/OpenAPI registration sources"
    fi
fi

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
