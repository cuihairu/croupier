#!/usr/bin/env bash

set -euo pipefail

e2e_dir="${1:-web/e2e}"
errors=0

# Prefer ripgrep; fall back to GNU grep -P (PCRE) which GitHub-hosted Ubuntu
# runners ship by default. Both support the patterns used below.
if command -v rg >/dev/null 2>&1; then
  search() {
    local pattern="$1" dir="$2"
    rg -n --pcre2 "${pattern}" "${dir}" --glob '*.spec.ts' --glob 'helpers/index.ts'
  }
elif command -v grep >/dev/null 2>&1 && grep -P '' /dev/null >/dev/null 2>&1; then
  search() {
    local pattern="$1" dir="$2"
    find "${dir}" \( -name '*.spec.ts' -o -name 'index.ts' \) -type f -print0 |
      xargs -0 grep -nP "${pattern}" 2>/dev/null
  }
else
  echo "ERROR: dashboard E2E guard requires rg (ripgrep) or GNU grep with PCRE support" >&2
  exit 1
fi

if [[ ! -d "${e2e_dir}" ]]; then
  echo "ERROR: dashboard E2E directory does not exist: ${e2e_dir}" >&2
  exit 1
fi

check_absent() {
  local label="$1"
  local pattern="$2"
  local output

  if output=$(search "${pattern}" "${e2e_dir}"); then
    echo "ERROR: ${label}" >&2
    printf '%s\n' "${output}" >&2
    errors=$((errors + 1))
  fi
}

check_absent "不得跳过 Dashboard E2E" 'test\.(skip|fixme)|describe\.skip|\.skip\s*\('
check_absent "不得通过 isVisible 布尔探测绕过硬断言" '\.isVisible\s*\('
check_absent "不得使用宽松布尔组合断言" '\.toBe(True|False|Truthy|Falsy)\s*\('
check_absent "不得接受零行作为数据成功" 'toBeGreaterThanOrEqual\s*\(\s*0\s*\)|rows\s*>=\s*0'
check_absent "不得在元素或按钮缺失时提前返回" 'if\s*\(\s*!.*(button|btn|locator|element).*(\)|\})[^\n]*return'
check_absent "不得只断言 body/main/container 等页面外壳" 'locator\([^\n]*\b(body|main|container)\b[^\n]*\)|getBy(Role|TestId)\([^\n]*\b(body|main|container)\b'

if [[ ${errors} -ne 0 ]]; then
  echo "Dashboard E2E guard failed with ${errors} rule violation(s)." >&2
  exit 1
fi

echo "Dashboard E2E guard passed."
