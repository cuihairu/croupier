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
  "internal/function/uicontract"
  "internal/function/converter/pack.go"
  "internal/validation/entity.go"
  "internal/validation/entity_test.go"
  "internal/validation/entity_extra_test.go"
  "internal/logic/function/function_u_i_history_logic.go"
  "internal/logic/function/function_u_i_history_rollback_test.go"
  "internal/logic/function/function_u_i_logic_v2.go"
  "internal/logic/function/function_u_i_rollback_logic.go"
  "internal/logic/function/function_u_i_update_logic.go"
  "internal/logic/function/function_ui_e2e_test.go"
  "internal/logic/function/function_ui_versioning.go"
  "internal/logic/function/ui_resolver.go"
  "internal/logic/function/ui_resolver_test.go"
  "web/src/components/FunctionUIManager"
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

if rg -n 'PackConverter|PackManifest|PackEntity|PackEntityOperation|ValidateEntityDefinition|validateUIConfig|validateOperations' \
  internal/function internal/validation internal/api internal/model \
  --glob "*.go" >/dev/null 2>&1; then
  fail "legacy Pack manifest or entity definition validators must not be restored"
fi

if rg -n "FunctionFormRenderer|XUISchema|X_UI_SCHEMA|FUNCTION_FORM_RENDERER|FunctionUIDesigner|FunctionUIManager|@/components/FunctionUIManager|ui-designer|函数 UI 设计器|已废弃|deprecated wrapper" \
  web/src web/tests web/config \
  --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "removed function UI compatibility wrappers or legacy UI designer routes are still referenced in dashboard frontend"
fi

if rg -n "fetchFunctionUiSchema|saveFunctionUiSchema|fetchFunctionUiHistory|rollbackFunctionUiSchema|FunctionUiSchemaDocument|FunctionUIHistoryItem|fetchFunctionUISchemaDocument" \
  web/src web/tests \
  --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "function form frontend must use FunctionForm service names, not legacy FunctionUi names"
fi

if rg -n '/api/v1/functions/.*/ui(/history|/rollback)?|/:id/ui($|")|/:id/ui/(history|rollback)|functionHandler\.UI|functionHandler\.UIUpdate|functionHandler\.UIHistory|functionHandler\.UIRollback' \
  internal/api/function internal/handler internal/router web/src web/tests docs/api docs/architecture docs/guide \
  --glob "*.go" --glob "*.ts" --glob "*.tsx" --glob "*.md" >/dev/null 2>&1; then
  fail "function form API must use /form routes, not legacy /ui routes"
fi

if rg -n 'FunctionUI(Request|Response|UpdateRequest|HistoryRequest|HistoryResponse|HistoryItem|RollbackRequest|RollbackResponse|V2Response|V2UpdateRequest|V2RollbackRequest|Diagnostic)\b|func \(h \*Handler\) FunctionUI\b|func \(s \*Service\) FunctionUI\b|func functionUI\b' \
  internal/api/function \
  --glob "*.go" >/dev/null 2>&1; then
  fail "function API layer must expose FunctionForm types and handlers, not FunctionUI names"
fi

if rg -n 'uicontract|function_ui_not_allowed|openapi_ui_field_forbidden|descriptorUIRegistrationKey|operationUIRegistrationKey|forbiddenOpenAPIUIKey|Function UI|函数 UI' \
  internal/api/function internal/api/openapi internal/function internal/logic/function internal/server \
  --glob "*.go" >/dev/null 2>&1; then
  fail "function registration and form code must not use legacy Function UI naming"
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
  web/src/pages/Console web/src/services web/tests internal/api/page internal/api/console \
  --glob "*.ts" --glob "*.tsx" --glob "*.go" >/dev/null 2>&1; then
  fail "objectKey must not be used in Page/Console runtime"
fi

if rg -n "menu\\.ControlConsole\\.category\\." web/src --glob "*.ts" --glob "*.tsx" >/dev/null 2>&1; then
  fail "dynamic console categories must not use static locale keys"
fi

if rg -n "x-operation.*custom|CRUD operation type|operation \\|\\| ['\\\"]custom['\\\"]|\\\"create\\\", \\\"read\\\", \\\"update\\\", \\\"delete\\\", \\\"custom\\\"" proto sdks web/src \
  --glob "*.proto" --glob "*.ts" --glob "*.tsx" --glob "*.go" --glob "*.md" --glob "*.py" --glob "*.java" --glob "*.cs" --glob "*.h" --glob "*.cpp" \
  --glob "!**/node_modules/**" --glob "!**/build/**" --glob "!**/obj/**" --glob "!**/bin/**" --glob "!sdks/cpp/vcpkg/**" >/dev/null 2>&1; then
  fail "SDK/OpenAPI docs or sources still describe operation as CRUD/custom"
fi

if rg -n "PageFunctionBinding.*Role|binding\\.Role|Role:.*Placement" \
  internal/dashboard internal/api/page internal/model/page_spec.go \
  --glob "*.go" >/dev/null 2>&1; then
  fail "Page binding must use usage, not placement-derived role"
fi

if rg -n "/api/v1/workspaces/pages|/api/v1/workspaces/published|/api/v1/workspaces/:objectKey/config" \
  internal web/src web/tests --glob "*.go" --glob "*.ts" --glob "*.tsx" --glob "*.jsx" >/dev/null 2>&1; then
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
  --glob "!**/build/**" --glob "!**/obj/**" --glob "!**/bin/**" \
  --glob "!internal/function/registrationguard/reject.go" >/dev/null 2>&1; then
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

if rg -n "Virtual Object|VirtualObject|RelationshipDef|ComponentDescriptor|RegisterVirtualObject|RegisterComponent|GetRegisteredObjects|GetRegisteredComponents|UnregisterVirtualObject|UnregisterComponent|LoadComponentFromFile|config_driven_loader|config_manager|virtual_object_demo|complete_example|comprehensive_demo|production_example|Function -> Entity|gRPC Integration|虚拟对象|组件系统|register_vo|unregister_vo|get_vo|list_vos|register_virtual_object|get_virtual_object|list_virtual_objects|unregister_virtual_object|create_vo_data|create_invoke_request|desc\\.category|desc\\.entity|test_desc\\.category|test_desc\\.entity" \
  sdks/cpp/include sdks/cpp/src sdks/cpp/examples sdks/cpp/tests sdks/cpp/lua sdks/cpp/skynet sdks/cpp/README.md sdks/cpp/CMakeLists.txt sdks/cpp/VERSION.cmake sdks/cpp/vcpkg.json \
  --glob "*.h" --glob "*.cpp" --glob "*.lua" --glob "*.md" --glob "*.cmake" --glob "*.json" --glob "CMakeLists.txt" \
  --glob "!**/build/**" --glob "!**/vcpkg/**" >/dev/null 2>&1; then
  fail "C++ SDK must not restore legacy VirtualObject/Component registration or Skynet VO APIs"
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
