#!/usr/bin/env bash
# Run the cross-SDK golden vector suites (CR-FIX-05 behavioral contract).
#
# Every SDK test reads the single language-neutral vector file
# proto/contracts/golden_vectors.json and asserts identical bytes/semantics,
# so one proto behavior change that breaks the contract fails every suite.
# This script is the one-command entry point over those per-SDK suites.
#
# Usage:
#   scripts/contract/run-golden-vectors.sh [--only go,python,js,java,csharp,cpp]
#                                          [--skip <list>] [--with-cpp] [--strict] [--list]
#
# - Default set: go, python, js, java, csharp (cpp is opt-in: its suite needs
#   a full CMake+GTest build, see scripts/contract/README.md)
# - A suite whose toolchain is missing is SKIPPED (exit 0); --strict turns
#   skips into failures
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$ROOT" ]; then ROOT="$(cd "$(dirname "$0")/../.." && pwd)"; fi
CONTRACT="$ROOT/proto/contracts/golden_vectors.json"
ALL_SDKS="go python js java csharp"
STRICT=0
WITH_CPP=0
ONLY=""
SKIP=""

while [ $# -gt 0 ]; do
  case "$1" in
    --only) ONLY="$2"; shift 2 ;;
    --skip) SKIP="$2"; shift 2 ;;
    --with-cpp) WITH_CPP=1; shift ;;
    --strict) STRICT=1; shift ;;
    --list) echo "go python js java csharp"; [ "$WITH_CPP" = 1 ] && echo "cpp"; exit 0 ;;
    -h|--help) sed -n '2,14p' "$0"; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [ ! -f "$CONTRACT" ]; then
  echo "contract file missing: $CONTRACT" >&2
  exit 1
fi

TARGETS="$ALL_SDKS"
if [ -n "$ONLY" ]; then TARGETS=$(echo "$ONLY" | tr ',' ' '); fi
if [ "$WITH_CPP" = 1 ] && [ -z "$ONLY" ] && ! echo " $TARGETS " | grep -q " cpp "; then
  TARGETS="$TARGETS cpp"
fi
if [ -n "$SKIP" ]; then
  for s in $(echo "$SKIP" | tr ',' ' '); do
    TARGETS=$(echo "$TARGETS" | tr ' ' '\n' | grep -v "^$s$" | tr '\n' ' ')
  done
fi

have() { command -v "$1" >/dev/null 2>&1; }

run_go() {
  have go || return 9
  (cd "$ROOT/sdks/go" && go test ./pkg/croupier/protocol/ -run 'GoldenVectors' -count=1) >&2
}

run_python() {
  have python3 || return 9
  python3 -c 'import pytest' 2>/dev/null || { echo "pytest not installed" >&2; return 9; }
  (cd "$ROOT/sdks/python" && python3 -m pytest tests/test_golden_vectors.py -q) >&2
}

run_js() {
  have pnpm || return 9
  [ -d "$ROOT/sdks/js/node_modules" ] || { echo "sdks/js deps not installed (pnpm --dir sdks/js install)" >&2; return 9; }
  (cd "$ROOT/sdks/js" && pnpm exec jest src/golden_vectors.test.ts --ci) >&2
}

run_java() {
  have java || return 9
  (cd "$ROOT/sdks/java" && ./gradlew test --tests '*GoldenVectorsTest*' --console=plain -q) >&2
}

run_csharp() {
  have dotnet || return 9
  (cd "$ROOT/sdks/csharp" && dotnet test Croupier.Sdk.sln --filter 'FullyQualifiedName~GoldenVectorsTests' --nologo -v q) >&2
}

run_cpp() {
  have cmake || return 9
  B="$ROOT/sdks/cpp/build-contract"
  cmake -S "$ROOT/sdks/cpp" -B "$B" -DBUILD_TESTS=ON -DENABLE_VCPKG=OFF >&2 || return 9
  cmake --build "$B" --target croupier-sdk-tests >&2 || return 9
  (cd "$B" && ctest -R 'GoldenVectors' --output-on-failure) >&2
}

FAILED=""
SKIPPED=""
PASSED=""
for sdk in $TARGETS; do
  echo "=== golden vectors: $sdk ==="
  if "run_$sdk"; then
    PASSED="$PASSED $sdk"
    echo "--- $sdk: PASS"
  else
    rc=$?
    if [ "$rc" = "9" ]; then
      SKIPPED="$SKIPPED $sdk"
      echo "--- $sdk: SKIP (toolchain/deps unavailable)"
    else
      FAILED="$FAILED $sdk"
      echo "--- $sdk: FAIL (exit $rc)"
    fi
  fi
done

echo
echo "Contract file: proto/contracts/golden_vectors.json"
echo "Passed:$PASSED"
echo "Skipped:$SKIPPED"
echo "Failed:$FAILED"

if [ -n "$FAILED" ]; then exit 1; fi
if [ "$STRICT" = 1 ] && [ -n "$SKIPPED" ]; then
  echo "strict mode: skipped suites are failures" >&2
  exit 1
fi
exit 0
