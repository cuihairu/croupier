#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INPUT_SPEC="${ROOT_DIR}/docs/contracts/extensions-openapi-v1.yaml"
OUT_DIR="${1:-${ROOT_DIR}/.tmp/contracts/client}"

mkdir -p "${OUT_DIR}"

echo "Generating TypeScript client from: ${INPUT_SPEC}"
echo "Output dir: ${OUT_DIR}"

npx --yes openapi-typescript-codegen@0.29.0 \
  --input "${INPUT_SPEC}" \
  --output "${OUT_DIR}" \
  --client fetch \
  --useUnionTypes

echo "Done."
