#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <dashboard_root> [--force]" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DASHBOARD_ROOT="$1"
FORCE="${2:-}"

if [[ ! -d "${DASHBOARD_ROOT}" ]]; then
  echo "Dashboard root not found: ${DASHBOARD_ROOT}" >&2
  exit 1
fi

echo "Bootstrap dashboard contracts into: ${DASHBOARD_ROOT}"

bash "${ROOT_DIR}/scripts/contracts/gen-extensions-ts-types.sh" \
  "${DASHBOARD_ROOT}/src/services/contracts/extensions.ts"

bash "${ROOT_DIR}/scripts/contracts/gen-extensions-ts-client.sh" \
  "${DASHBOARD_ROOT}/src/services/generated/extensions-client"

TEMPLATE_ROOT="${ROOT_DIR}/docs/contracts/templates/dashboard"

copy_template_file() {
  local src="$1"
  local rel="${src#${TEMPLATE_ROOT}/}"
  local dst="${DASHBOARD_ROOT}/${rel}"
  mkdir -p "$(dirname "${dst}")"
  if [[ -f "${dst}" && "${FORCE}" != "--force" ]]; then
    echo "Skip existing: ${dst}"
    return
  fi
  cp "${src}" "${dst}"
  echo "Wrote: ${dst}"
}

while IFS= read -r -d '' file; do
  copy_template_file "${file}"
done < <(find "${TEMPLATE_ROOT}" -type f -print0)

echo "Bootstrap done."
