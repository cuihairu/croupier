#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SERVICES_DIR="${SCRIPT_DIR}"

if ! command -v goctl >/dev/null 2>&1; then
  GOPATH_BIN="$(go env GOPATH 2>/dev/null)/bin"
  if [[ -x "${GOPATH_BIN}/goctl" ]]; then
    export PATH="${PATH}:${GOPATH_BIN}"
  fi
fi

if ! command -v goctl >/dev/null 2>&1; then
  echo "error: goctl is not installed or not in PATH" >&2
  echo "install via: go install github.com/zeromicro/go-zero/tools/goctl@latest" >&2
  exit 1
fi

style="go_zero"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --style)
      style="$2"
      shift 2
      ;;
    *)
      echo "usage: $0 [--style go_zero]" >&2
      exit 1
      ;;
  esac
done

api_files=()
while IFS= read -r file; do
  api_files+=("$file")
done < <(find "${SERVICES_DIR}" -name "*.api" -print | sort)

if [[ ${#api_files[@]} -eq 0 ]]; then
  echo "warning: no .api files found under ${SERVICES_DIR}" >&2
  exit 0
fi

for api_path in "${api_files[@]}"; do
  rel_path=${api_path#${REPO_ROOT}/}
  service_dir="$(dirname "${api_path}")"
  echo "Generating Go code for ${rel_path} -> ${service_dir}"
  goctl api go --api "${api_path}" --dir "${service_dir}" --style "${style}"
done

echo "✅ goctl api go generation complete."
