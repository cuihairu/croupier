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

if [[ -d "web/dist" ]] && rg -n "WorkspaceConfig|WorkspaceRenderer|workspaceConfig|/api/v1/workspaces|PageGenerator|WorkspaceEditor|Workspaces|objectKey|workspace_not_found|workspace_invalid_config" \
  web/dist >/dev/null 2>&1; then
  fail "legacy Workspace/PageGenerator artifacts are still present in web/dist"
fi

echo "Dashboard model guard checks passed."
