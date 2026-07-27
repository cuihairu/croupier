#!/usr/bin/env bash
set -euo pipefail

echo "Running dashboard model guard checks..."

fail() {
  echo "DASHBOARD_GUARD_FAILED: $1" >&2
  exit 1
}

removed_paths=(
  "web/docs/DEMO.md"
  "web/docs/WORKSPACE_API.md"
  "web/docs/WORKSPACE_DEV_GUIDE.md"
  "web/docs/WORKSPACE_USER_GUIDE.md"
  "web/docs/workspace-error-contract.md"
  "web/docs/workspace-governance-playbook.md"
  "web/docs/workspace-logging-standard.md"
  "web/docs/workspace-product-delivery-guide.md"
  "web/docs/workspace-regression-checklist.md"
  "web/src/components/FunctionFormRenderer"
  "web/src/components/PageGenerator"
  "web/src/components/WorkspaceRenderer"
  "web/src/components/XUISchema.tsx"
  "web/src/pages/WorkspaceEditor"
  "web/src/pages/Workspaces"
  "web/src/pages/Entities"
  "web/src/services/workspaceConfig.ts"
  "web/src/services/api/users.ts"
  "web/src/services/api/roles.ts"
  "web/src/types/workspace.ts"
  "web/tests/workspace"
)

for path in "${removed_paths[@]}"; do
  if [[ -e "${path}" ]]; then
    fail "removed dashboard compatibility path must not exist: ${path}"
  fi
done

if rg -n "WorkspaceConfig|WorkspaceRenderer|workspaceConfig|/api/v1/workspaces|canWorkspace|PageGenerator|Compatibility wrapper|Legacy compatibility|向后兼容" \
  web/src web/tests web/config \
  --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "legacy Workspace/PageGenerator symbols are still referenced in dashboard frontend"
fi

if rg -n "FunctionFormRenderer|XUISchema|X_UI_SCHEMA|FUNCTION_FORM_RENDERER|已废弃|deprecated wrapper" \
  web/src web/tests web/config \
  --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "removed function UI compatibility wrappers are still referenced in dashboard frontend"
fi

if rg -n "from ['\"]@/services/api/users['\"]|from ['\"]@/services/api/roles['\"]|from ['\"]\\./users['\"]|from ['\"]\\./roles['\"]" \
  web/src web/tests \
  --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "legacy users/roles API wrappers are still imported"
fi

if [[ -d "web/docs" ]] && rg -n "WorkspaceConfig|WorkspaceRenderer|workspaceConfig|/api/v1/workspaces|PageGenerator|WorkspaceEditor|Workspaces|objectKey|对象工作台|实体管理" \
  web/docs --glob "*.md" >/dev/null 2>&1; then
  fail "legacy Workspace/PageGenerator documentation still exists under web/docs"
fi

if [[ -d "docs" ]] && rg -n "workspace\\.md|object-workspace\\.md|WorkspaceConfig|/api/v1/workspaces|对象工作台|实体管理" \
  docs --glob "*.md" --glob "!archive/**" >/dev/null 2>&1; then
  fail "legacy dashboard documentation still exists outside docs/archive"
fi

if rg -n "objectKey" \
  web/src/pages/Console web/src/services internal/api/page internal/api/console \
  --glob "*.ts" --glob "*.tsx" --glob "*.go" >/dev/null 2>&1; then
  fail "objectKey must not be used in Page/Console runtime"
fi

if rg -n "menu\\.ControlConsole\\.category\\." web/src --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "dynamic console categories must not use static locale keys"
fi

if rg -n "x-operation.*custom|CRUD operation type" proto sdks web/src \
  --glob "*.proto" --glob "*.ts" --glob "*.tsx" --glob "*.go" --glob "*.md" \
  --glob "!**/node_modules/**" --glob "!**/build/**" --glob "!**/obj/**" >/dev/null 2>&1; then
  fail "SDK/OpenAPI docs or sources still describe operation as CRUD/custom"
fi

if rg -n "PageFunctionBinding.*Role|binding\\.Role|Role:.*Placement" \
  internal/dashboard internal/api/page internal/model/page_spec.go \
  --glob "*.go" >/dev/null 2>&1; then
  fail "Page binding must use usage, not placement-derived role"
fi

if rg -n "/api/v1/workspaces/pages|/api/v1/workspaces/:objectKey/config" \
  internal web/src --glob "*.go" --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "legacy workspace routes must not be referenced"
fi

if rg -n "ImportFromOpenAPI|importOpenAPISpec|/metadata/functions/import/openapi|/api/v1/metadata/functions/import/openapi" \
  internal web/src --glob "*.go" --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "OpenAPI uploads must use OpenAPI Source APIs, not metadata import shortcuts"
fi

if rg -n "category_display|entity_display|operation_display|operation_kind|page_hint|x-labels" \
  proto/croupier sdks/go sdks/js/src sdks/python sdks/java/src sdks/csharp/src sdks/cpp/include sdks/cpp/src internal web/src \
  --glob "*.proto" --glob "*.go" --glob "*.ts" --glob "*.tsx" --glob "*.py" --glob "*.java" --glob "*.cs" --glob "*.h" --glob "*.cpp" \
  --glob "!**/test/**" --glob "!**/tests/**" --glob "!**/*Test.java" --glob "!**/*.test.ts" \
  --glob "!internal/function/uicontract/reject.go" >/dev/null 2>&1; then
  fail "SDK/OpenAPI registration sources still contain forbidden page/display fields"
fi

if rg -n '"domain"\s*:\s*"entity"|domain=entity|Domain:\s*"entity"' \
  configs internal web/src \
  --glob "*.json" --glob "*.go" --glob "*.ts" --glob "*.tsx" \
  --glob "!internal/api/terms/handler_test.go" \
  --glob "!internal/model/model_test.go" \
  --glob "!internal/svc/service_context_test.go" >/dev/null 2>&1; then
  fail "term dictionary domain must not use entity in runtime config"
fi

if rg -n "ProvidersEntities|openAPIDocEntities|aggregateEntities|/providers/.*/entities|/:id/entities|x-entities" \
  internal/api/provider internal/handler internal/router docs/api/provider.md \
  --glob "*.go" --glob "*.md" >/dev/null 2>&1; then
  fail "provider API must expose resources, not legacy entities"
fi

if rg -n "\\bany\\b" web/src/types web/src/services/console.ts web/src/services/api/resources.ts \
  --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "dashboard frontend core types/services must not use TypeScript any"
fi

if rg -n "interface\\{\\}|map\\[string\\]interface\\{" \
  internal/dashboard/spec internal/dashboard/generator internal/api/page/dto.go internal/api/console/dto.go internal/model/page_spec.go \
  --glob "*.go" >/dev/null 2>&1; then
  fail "dashboard core DTO/model packages must not expose Go interface{} maps"
fi

if [[ -d "web/dist" ]] && rg -n "WorkspaceConfig|WorkspaceRenderer|workspaceConfig|/api/v1/workspaces|PageGenerator|WorkspaceEditor|Workspaces|objectKey|workspace_not_found|workspace_invalid_config" \
  web/dist >/dev/null 2>&1; then
  fail "legacy Workspace/PageGenerator artifacts are still present in web/dist"
fi

echo "Dashboard model guard checks passed."
