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
  echo "and/or ensure $(go env GOPATH)/bin is in your PATH" >&2
  exit 1
fi

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
  output_dir="${service_dir}/openapi"
  base_name="$(basename "${api_path}" .api)"

  mkdir -p "${output_dir}"

  echo "Generating OpenAPI for ${rel_path} -> ${output_dir}/${base_name}.json"
  if ! goctl api swagger --api "${api_path}" --dir "${output_dir}" --filename "${base_name}" >/dev/null; then
    echo "  ⚠️  Failed to generate OpenAPI for ${rel_path}, skipping." >&2
    continue
  fi
done

echo "✅ OpenAPI generation complete (see per-service openapi/ directories)."
