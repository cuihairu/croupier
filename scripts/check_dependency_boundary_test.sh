#!/usr/bin/env bash
# 依赖边界守卫回归自测（ARCH-019）：干净树必须 PASS，注入合成违规必须
# 拒绝——证明守卫不是路径静默 no-op 的「全绿假象」。
set -euo pipefail

fixture_paths=(
  "core/dependency_boundary_fixture.go"
  "internal/platform/functioninvoke/dependency_boundary_fixture.go"
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

# 干净树必须 PASS
bash "scripts/check_dependency_boundary.sh"

expect_rejection() {
  local name="$1"
  local fixture_path="$2"
  local content="$3"

  printf '%s\n' "${content}" > "${fixture_path}"
  created_fixtures+=("${fixture_path}")
  if bash "scripts/check_dependency_boundary.sh"; then
    echo "dependency boundary guard accepted forbidden ${name}" >&2
    exit 1
  fi
  rm -f -- "${fixture_path}"
}

expect_rejection "core->internal import" \
  "core/dependency_boundary_fixture.go" \
  $'package dependency_boundary_fixture\n\nimport _ "github.com/cuihairu/croupier/internal/svc"'
expect_rejection "functioninvoke->internal/api import" \
  "internal/platform/functioninvoke/dependency_boundary_fixture.go" \
  $'package functioninvoke\n\nimport _ "github.com/cuihairu/croupier/internal/api/assignment"'

echo "PASSED: dependency boundary guard rejects synthetic violations"
