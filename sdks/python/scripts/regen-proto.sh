#!/usr/bin/env bash
# Regenerate Python SDK protobuf code from the root proto/ definitions.
#
# The handwritten layer (croupier/__init__.py, croupier/invoker.py) references
# the generated *_pb2 modules. After the wire-protocol rename (RegisterLocal →
# ProviderConnect, StartJob/JobEvent → StartTask/TaskEvent, etc.) the generated
# code MUST be regenerated from proto/ so it matches the canonical names.
#
# Requirements: buf CLI (https://buf.build). Run from the repo root:
#
#   ./sdks/python/scripts/regen-proto.sh
#
# This regenerates sdks/python/generated/ in place. After regenerating, the
# handwritten layer's renamed references (ProviderConnectRequest, StartTaskResponse,
# TaskEvent, ...) resolve against the fresh generated code.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SDK_DIR="$REPO_ROOT/sdks/python"

if ! command -v buf >/dev/null 2>&1; then
  echo "ERROR: buf CLI not found. Install from https://buf.build." >&2
  exit 1
fi

cd "$SDK_DIR"
echo "Regenerating Python protobuf code into $SDK_DIR/generated ..."
buf generate

echo "Done. Verify with:"
echo "  rg 'RegisterLocal|StartJob|JobEvent|HeartbeatLocal|ListLocal' sdks/python/generated/"
echo "  (expected: no matches)"
