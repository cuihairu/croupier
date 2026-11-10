#!/usr/bin/env bash
# 四层依赖守卫回归自测（CR-FIX-21）：干净树必须 PASS，注入合成违规必须
# 拒绝——证明守卫不是路径静默 no-op 的「全绿假象」。
set -euo pipefail

fixture_paths=(
  "internal/transport/zzz_layer_fixture_test.go"
  "internal/core/zzz_layer_fixture_test.go"
  "internal/platform/zzz_layer_fixture_test.go"
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
bash "scripts/check_layer_dependency.sh"

expect_rejection() {
  local name="$1"
  local fixture_path="$2"
  local content="$3"

  printf '%s\n' "${content}" > "${fixture_path}"
  created_fixtures+=("${fixture_path}")
  if bash "scripts/check_layer_dependency.sh"; then
    echo "layer dependency guard accepted forbidden ${name}" >&2
    exit 1
  fi
  rm -f -- "${fixture_path}"
}

# 注意：合成违规放 _test.go（守卫排除测试文件）不会触发——故各层合成
# 违规必须走生产文件名，才能证明守卫真实拒绝。文件名不以 _test.go 结尾。
expect_rejection "transport->core import" \
  "internal/transport/zzz_layer_fixture.go" \
  $'package transport\n\nimport _ "github.com/cuihairu/croupier/internal/core"'
expect_rejection "core->platform import" \
  "internal/core/zzz_layer_fixture.go" \
  $'package core\n\nimport _ "github.com/cuihairu/croupier/internal/platform"'
expect_rejection "platform->api import" \
  "internal/platform/zzz_layer_fixture.go" \
  $'package platform\n\nimport _ "github.com/cuihairu/croupier/internal/api/page"'

echo "PASSED: layer dependency guard rejects synthetic violations"
