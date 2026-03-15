#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INPUT_SPEC="${ROOT_DIR}/docs/contracts/extensions-openapi-v1.yaml"
OUT_FILE="${1:-${ROOT_DIR}/.tmp/contracts/extensions.types.ts}"

mkdir -p "$(dirname "${OUT_FILE}")"

echo "Generating TypeScript types from: ${INPUT_SPEC}"
echo "Output: ${OUT_FILE}"

npx --yes openapi-typescript@7.10.1 "${INPUT_SPEC}" -o "${OUT_FILE}"

echo "Done."
