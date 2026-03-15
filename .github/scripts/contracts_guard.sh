#!/usr/bin/env bash
set -euo pipefail

echo "Running contracts guard checks..."

fail() {
  echo "CONTRACT_GUARD_FAILED: $1" >&2
  exit 1
}

SPEC_FILE="docs/contracts/extensions-openapi-v1.yaml"
MAPPING_FILE="docs/contracts/frontend-error-mapping-v1.json"

if command -v rg >/dev/null 2>&1; then
  SEARCH_TOOL="rg"
else
  SEARCH_TOOL="grep"
fi

has_line() {
  local pattern="$1"
  local file="$2"
  if [[ "${SEARCH_TOOL}" == "rg" ]]; then
    rg -n --pcre2 "$pattern" "$file" >/dev/null 2>&1
  else
    grep -nE "$pattern" "$file" >/dev/null 2>&1
  fi
}

[[ -f "${SPEC_FILE}" ]] || fail "missing ${SPEC_FILE}"
[[ -f "${MAPPING_FILE}" ]] || fail "missing ${MAPPING_FILE}"

required_paths=(
  "/extensions/catalog:"
  "/extensions/catalog/{id}:"
  "/extensions/catalog/{id}/releases:"
  "/extensions/installations:"
  "/extensions/installations/{id}:"
  "/extensions/install:"
  "/extensions/{id}/enable:"
  "/extensions/{id}/disable:"
  "/extensions/{id}/upgrade:"
  "/extensions/{id}/uninstall:"
  "/extensions/{id}/capabilities:"
  "/extensions/{id}/health-check:"
  "/extensions/{id}/reconcile:"
  "/extensions/{id}/events:"
)

for p in "${required_paths[@]}"; do
  escaped="$(printf '%s' "$p" | sed -e 's/[.[\*^$()+?{|]/\\&/g')"
  has_line "^  ${escaped}$" "${SPEC_FILE}" || fail "missing required path in spec: ${p}"
done

required_error_codes=(
  "extension_already_installed"
  "dependency_blocked"
  "missing_dependency"
  "version_mismatch"
  "dependency_cycle"
  "forbidden"
  "not_found"
)

for code in "${required_error_codes[@]}"; do
  has_line "\"${code}\"" "${MAPPING_FILE}" || fail "missing error code mapping: ${code}"
done

echo "Contracts guard checks passed."
