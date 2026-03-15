#!/usr/bin/env bash
set -euo pipefail

echo "Running contracts guard checks..."

fail() {
  echo "CONTRACT_GUARD_FAILED: $1" >&2
  exit 1
}

SPEC_FILE="docs/contracts/extensions-openapi-v1.yaml"
MAPPING_FILE="docs/contracts/frontend-error-mapping-v1.json"

has_line() {
  local pattern="$1"
  local file="$2"
  grep -nF "$pattern" "$file" >/dev/null 2>&1
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
  has_line "  ${p}" "${SPEC_FILE}" || fail "missing required path in spec: ${p}"
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
