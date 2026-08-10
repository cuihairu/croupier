#!/usr/bin/env bash
set -euo pipefail

fixture_paths=(
  "web/src/dashboard-vnext-guard-fixture.ts"
  "internal/agent/dashboard_vnext_guard_fixture.go"
)
created_fixtures=()

cleanup() {
  if [[ ${#created_fixtures[@]} -gt 0 ]]; then
    rm -f -- "${created_fixtures[@]}"
  fi
}
trap cleanup EXIT

for fixture_path in "${fixture_paths[@]}"; do
  if [[ -e "${fixture_path}" ]]; then
    echo "guard fixture path already exists: ${fixture_path}" >&2
    exit 1
  fi
done

bash "scripts/dashboard_vnext_guard.sh"

expect_rejection() {
  local name="$1"
  local fixture_path="$2"
  local content="$3"

  printf '%s\n' "${content}" > "${fixture_path}"
  created_fixtures+=("${fixture_path}")
  if bash "scripts/dashboard_vnext_guard.sh"; then
    echo "Dashboard vNext guard accepted forbidden ${name}" >&2
    exit 1
  fi
  rm -f -- "${fixture_path}"
}

expect_rejection "Formily runtime import" "web/src/dashboard-vnext-guard-fixture.ts" "import '@formily/core';"
expect_rejection "form-render runtime import" "web/src/dashboard-vnext-guard-fixture.ts" "import 'form-render';"
expect_rejection "legacy Function Form reference" "web/src/dashboard-vnext-guard-fixture.ts" "const legacy = 'FunctionFormManager';"
expect_rejection "legacy page runtime reference" "web/src/dashboard-vnext-guard-fixture.ts" "const legacy = 'WorkspaceRenderer';"
expect_rejection "registration-side UI field" "internal/agent/dashboard_vnext_guard_fixture.go" $'package agent\n\nconst dashboardVNextGuardFixture = "category_display"'

echo "Dashboard vNext guard rejection test passed."
