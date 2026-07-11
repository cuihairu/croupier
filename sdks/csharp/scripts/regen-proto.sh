#!/usr/bin/env bash
# Regenerate C# SDK protobuf code from the root proto/ definitions.
#
# The handwritten layer (src/Croupier.Sdk/CroupierInvoker.cs) was migrated to
# canonical Task/ProviderConnect naming, but the generated code under
# generated/ (Invocation.cs, Provider.cs) is STALE — it still defines
# StartJobResponse/JobEvent/CancelJobRequest and legacy RegisterLocal types,
# and embeds protobuf reflection descriptors that cannot be safely hand-edited.
#
# After regenerating, the CroupierInvoker.cs NOTE markers referencing
# StartJobResponse / CancelJobRequest / JobEvent should be updated to the
# regenerated Task types (StartTaskResponse / CancelTaskRequest / TaskEvent).
#
# Requirements: buf CLI (https://buf.build). Run from the repo root:
#
#   ./sdks/csharp/scripts/regen-proto.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SDK_DIR="$REPO_ROOT/sdks/csharp"

if ! command -v buf >/dev/null 2>&1; then
  echo "ERROR: buf CLI not found. Install from https://buf.build." >&2
  exit 1
fi

cd "$SDK_DIR"
echo "Regenerating C# protobuf code into $SDK_DIR/../generated ..."
buf generate

echo "Done. Now update src/Croupier.Sdk/CroupierInvoker.cs to reference the"
echo "regenerated Task types (StartTaskResponse, CancelTaskRequest, TaskEvent)"
echo "and remove the NOTE markers."
echo
echo "Verify with:"
echo "  rg 'StartJob|JobEvent|CancelJob|RegisterLocal' sdks/csharp/"
echo "  (expected: no matches outside this script and generated descriptors)"
