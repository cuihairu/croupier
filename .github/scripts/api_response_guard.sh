#!/usr/bin/env bash
set -euo pipefail

echo "Running API response guard checks..."

fail() {
  echo "API_RESPONSE_GUARD_FAILED: $1" >&2
  exit 1
}

ROOT_DIR="${1:-internal/api}"

[[ -d "${ROOT_DIR}" ]] || fail "missing directory: ${ROOT_DIR}"

raw_json_hits="$(grep -RInE '\bc\.(JSON|IndentedJSON|String|Data|PureJSON|XML|YAML|ProtoBuf)\(' "${ROOT_DIR}" --include='*.go' --exclude='*_test.go' || true)"

if [[ -n "${raw_json_hits}" ]]; then
  filtered_hits="$(printf '%s\n' "${raw_json_hits}" | grep -vE 'text/event-stream|status": "ok"|gin\.H\{"status": "ok"\}' || true)"
  if [[ -n "${filtered_hits}" ]]; then
    printf '%s\n' "${filtered_hits}"
    fail "found raw response writes in API handlers"
  fi
fi

pkg2_hits="$(grep -RIn 'internal/pkg2/response' internal --include='*.go' --exclude='*_test.go' || true)"
if [[ -n "${pkg2_hits}" ]]; then
  printf '%s\n' "${pkg2_hits}"
  fail "found forbidden internal/pkg2/response import"
fi

echo "API response guard checks passed."
