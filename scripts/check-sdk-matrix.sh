#!/usr/bin/env bash
# Check SDK API surface against sdks/SDK_FEATURE_MATRIX.md.
#
# This script greps each language SDK for the L1 symbols declared in the
# matrix. It only checks for *presence*, not naming consistency — naming
# follows each language's idioms (Go/C# PascalCase, Java/JS camelCase,
# Python/C++ snake_case).
#
# Run from repo root:
#   ./scripts/check-sdk-matrix.sh
#
# Exit codes:
#   0  all required symbols found
#   1  at least one required symbol is missing
#
# When adding/removing an API:
#   1. Update sdks/SDK_FEATURE_MATRIX.md first (single source of truth).
#   2. Mirror the change in EXPECTED_* below.
#   3. Re-run this script locally before pushing.

set -u

cd "$(dirname "$0")/.." || exit 2

RED=$'\033[31m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
DIM=$'\033[2m'
RESET=$'\033[0m'

errors=0
warnings=0

# ---------------------------------------------------------------------------
# L1 Core Provider — required symbols per SDK.
# Each entry: "expected_substring|file_glob_hint|description"
# The checker greps the substring inside files matching the glob under the
# SDK root. Empty result => fail.
# ---------------------------------------------------------------------------

check_go() {
  local root="sdks/go/pkg/croupier"
  local entries=(
    "func.*NewClient|client.go|constructor"
    "func.*RegisterFunction|client.go|register"
    "func.*Connect|client.go|connect"
    "func.*Serve|client.go|serve"
    "func.*Stop|client.go|stop"
    "func.*Close|client.go|close"
    "AgentAddr|types.go|config.agent_addr"
    "ServiceID|types.go|config.service_id"
    "GameID|types.go|config.game_id"
    "Insecure|types.go|config.insecure"
  )
  run_checks "go" "$root" entries
}

check_python() {
  local root="sdks/python/croupier"
  local entries=(
    "class CroupierClient|__init__.py|constructor"
    "def register_function|__init__.py|register"
    "def connect|__init__.py|connect"
    "def disconnect|__init__.py|stop/close"
    "agent_addr: str|__init__.py|config.agent_addr"
    "service_id: str|__init__.py|config.service_id"
    "game_id: str|__init__.py|config.game_id"
    "insecure: bool|__init__.py|config.insecure"
  )
  run_checks "python" "$root" entries
}

check_java() {
  local root="sdks/java/src/main/java/io/github/cuihairu/croupier/sdk"
  local entries=(
    "createClient|CroupierSDK.java|constructor"
    "void registerFunction|CroupierClient.java|register"
    "CompletableFuture<Void> connect|CroupierClient.java|connect"
    "void serve|CroupierClient.java|serve"
    "void stop|CroupierClient.java|stop"
    "void close|CroupierClient.java|close"
    "private String agentAddr|ClientConfig.java|config.agent_addr"
    "private String serviceId|ClientConfig.java|config.service_id"
    "private String gameId|ClientConfig.java|config.game_id"
    "private boolean insecure|ClientConfig.java|config.insecure"
  )
  run_checks "java" "$root" entries
}

check_js() {
  local root="sdks/js/src"
  local entries=(
    "export function createClient|index.ts|constructor"
    "registerFunction|index.ts|register"
    "connect\\(\\): Promise<void>|index.ts|connect"
    "disconnect\\(\\): Promise<void>|index.ts|stop/close"
    "agentAddr\\?: string|index.ts|config.agent_addr"
    "serviceId\\?: string|index.ts|config.service_id"
    "gameId\\?: string|index.ts|config.game_id"
    "insecure\\?: boolean|index.ts|config.insecure"
  )
  run_checks "js" "$root" entries
}

check_cpp() {
  local root="sdks/cpp/include/croupier/sdk"
  local entries=(
    "bool RegisterFunction|croupier_client.h|register"
    "bool Connect\\(\\)|croupier_client.h|connect"
    "void Serve\\(\\)|croupier_client.h|serve"
    "void Stop\\(\\)|croupier_client.h|stop"
    "void Close\\(\\)|croupier_client.h|close"
    "std::string agent_addr|croupier_client.h|config.agent_addr"
    "std::string service_id|croupier_client.h|config.service_id"
    "std::string game_id|croupier_client.h|config.game_id"
    "bool insecure|croupier_client.h|config.insecure"
  )
  run_checks "cpp" "$root" entries
}

check_csharp() {
  local root="sdks/csharp/src/Croupier.Sdk"
  local entries=(
    "public.*RegisterFunction|CroupierClient.cs|register"
    "public.*Task ConnectAsync|CroupierClient.cs|connect"
    "public.*Task ServeAsync|CroupierClient.cs|serve"
    "public string AgentAddr|Models/ClientConfig.cs|config.agent_addr"
    "public string ServiceId|Models/ClientConfig.cs|config.service_id"
    "public string GameId|Models/ClientConfig.cs|config.game_id"
    "public bool Insecure|Models/ClientConfig.cs|config.insecure"
  )
  run_checks "csharp" "$root" entries
}

# ---------------------------------------------------------------------------
# L3 Invoker presence (informational; JS is currently empty).
# ---------------------------------------------------------------------------

check_invoker_presence() {
  echo "${DIM}[invoker]${RESET} presence across SDKs"
  local row=""
  for sdk in go python java js cpp csharp; do
    local found=""
    case "$sdk" in
      go)     found=$(find sdks/go/pkg/croupier -maxdepth 1 -name 'invoker*.go' ! -name '*_test.go' 2>/dev/null | head -1) ;;
      python) found=$(find sdks/python/croupier -maxdepth 1 -name 'invoker*.py' 2>/dev/null | head -1) ;;
      java)   found=$(find sdks/java/src/main -name 'Invoker.java' 2>/dev/null | head -1) ;;
      js)     found=$(find sdks/js/src -name 'invoker*.ts' 2>/dev/null | head -1) ;;
      cpp)    found=$(grep -l 'class CroupierInvoker' sdks/cpp/include/croupier/sdk/croupier_client.h 2>/dev/null | head -1) ;;
      csharp) found=$(find sdks/csharp/src/Croupier.Sdk -name 'CroupierInvoker.cs' 2>/dev/null | head -1) ;;
    esac
    if [ -n "$found" ]; then
      row+="  ${GREEN}${sdk}: yes${RESET}"
    else
      row+="  ${YELLOW}${sdk}: no${RESET}"
      warnings=$((warnings + 1))
    fi
  done
  echo "$row"
}

# ---------------------------------------------------------------------------
# L3 Invoker naming conformance.
#
# Matrix target: StartTask / StreamTask / CancelTask (with language-idiomatic
# casing). Each SDK that ships an invoker MUST expose the canonical *Task*
# methods. The legacy *Job* spellings are tolerated only as deprecated
# aliases that delegate to the canonical names — see
# sdks/SDK_FEATURE_MATRIX.md §四 "迁移规则".
#
# Report:
#   - canonical present   → OK
#   - only legacy present → warning (migration pending)
#   - neither present     → error (broken contract)
# ---------------------------------------------------------------------------

check_invoker_naming() {
  echo "${DIM}[invoker]${RESET} naming conformance (target: *Task*)"

  #/sdk|canonical_regex|legacy_regex|file_glob
  declare -a specs=(
    "go|func.*StartTask\\(|func.*StartJob\\(|sdks/go/pkg/croupier/invoker*.go"
    "python|def start_task|def start_job|sdks/python/croupier/invoker.py"
    "java|startTask\\(|startJob\\(|sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/invoker/Invoker.java"
    "cpp|StartTask\\(|StartJob\\(|sdks/cpp/include/croupier/sdk/croupier_client.h"
    "csharp|StartTaskAsync\\(|StartJobAsync\\(|sdks/csharp/src/Croupier.Sdk/CroupierInvoker.cs"
  )

  local pending=0
  for spec in "${specs[@]}"; do
    local sdk="${spec%%|*}"
    local rest="${spec#*|}"
    local canonical="${rest%%|*}"
    rest="${rest#*|}"
    local legacy="${rest%%|*}"
    local glob="${rest#*|}"

    # Resolve the glob to the first matching file (ignore tests).
    local file
    file=$(ls $glob 2>/dev/null | grep -v '_test\|\.test\.' | head -1)
    if [ -z "$file" ]; then
      # JS has no invoker yet — already flagged by check_invoker_presence.
      continue
    fi

    if grep -qE "$canonical" "$file"; then
      echo "  ${GREEN}${sdk}: canonical${RESET}  ${DIM}(${file})${RESET}"
    elif grep -qE "$legacy" "$file"; then
      echo "  ${YELLOW}${sdk}: legacy *Job* — migration pending${RESET}  ${DIM}(${file})${RESET}"
      warnings=$((warnings + 1))
      pending=$((pending + 1))
    else
      echo "  ${RED}${sdk}: missing both *Task* and *Job* — contract broken${RESET}"
      errors=$((errors + 1))
    fi
  done

  if [ "$pending" -gt 0 ]; then
    echo "${DIM}[invoker] ${pending} SDK(s) still on legacy *Job* — see sdks/SDK_FEATURE_MATRIX.md §四${RESET}"
  fi
}

# ---------------------------------------------------------------------------
# Helper: run a batch of (pattern|file|desc) checks under one SDK root.
# ---------------------------------------------------------------------------

run_checks() {
  local sdk="$1"
  local root="$2"
  local -n entries_ref="$3"

  if [ ! -d "$root" ]; then
    echo "${RED}[${sdk}] missing root: ${root}${RESET}"
    errors=$((errors + 1))
    return
  fi

  local failed=0
  local total=0
  for entry in "${entries_ref[@]}"; do
    total=$((total + 1))
    local pattern="${entry%%|*}"
    local rest="${entry#*|}"
    local file="${rest%%|*}"
    local desc="${rest#*|}"
    local path="${root}/${file}"
    if [ ! -f "$path" ]; then
      echo "${RED}[${sdk}] missing file: ${path}${RESET}"
      failed=$((failed + 1))
      continue
    fi
    if ! grep -qE "$pattern" "$path"; then
      echo "${RED}[${sdk}] missing symbol: ${desc} — pattern /${pattern}/ in ${file}${RESET}"
      failed=$((failed + 1))
    fi
  done

  if [ "$failed" -eq 0 ]; then
    echo "${GREEN}[${sdk}] OK${RESET}  ${DIM}(${total} symbols)${RESET}"
  else
    errors=$((errors + failed))
  fi
}

# ---------------------------------------------------------------------------
# README hygiene: ensure no SDK README mentions deprecated concepts.
# ---------------------------------------------------------------------------

check_readme_hygiene() {
  echo "${DIM}[docs]${RESET} README hygiene"
  local deprecated_patterns=(
    "两层注册"
    "LocalControlService"
    "rpc_addr"          # legacy field, replaced by ProviderConnect (no rpc_addr)
    "RegisterLocalRequest"
    "HeartbeatLocalRequest"
  )
  local any_fail=0
  for readme in sdks/*/README.md; do
    for pat in "${deprecated_patterns[@]}"; do
      if grep -q "$pat" "$readme"; then
        echo "${RED}[docs] ${readme} still mentions deprecated concept: ${pat}${RESET}"
        any_fail=1
        errors=$((errors + 1))
      fi
    done
  done
  if [ "$any_fail" -eq 0 ]; then
    echo "${GREEN}[docs] OK${RESET}  ${DIM}(no deprecated concepts in sdks/*/README.md)${RESET}"
  fi
}

# ---------------------------------------------------------------------------
# Wire-name hygiene: warn (not fail) when source code uses legacy names
# outside the compatibility-alias allowlist. The allowlist captures the
# intentional bridge constants in each SDK's protocol module.
# ---------------------------------------------------------------------------

check_wire_name_hygiene() {
  echo "${DIM}[wire]${RESET} legacy name usage (warnings)"

  # Files allowed to reference legacy aliases.
  # Two categories:
  #   1. Protocol modules that define the aliases (Protocol.java, protocol.go, etc).
  #   2. Tests that explicitly verify alias == canonical equivalence
  #      (e.g. message_test.go::TestLegacyAliases).
  # Any other file using these names should switch to the canonical name.
  local allowlist_patterns=(
    "sdks/go/pkg/croupier/protocol/message\\.go"
    "sdks/go/pkg/croupier/protocol/message_test\\.go"
    "sdks/python/croupier/protocol\\.py"
    "sdks/csharp/src/Croupier.Sdk/Transport/Protocol\\.cs"
    "sdks/cpp/include/croupier/sdk/protocol\\.h"
    "sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/transport/Protocol\\.java"
    "sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/wire/SdkWireMessages\\.java"
    "sdks/js/src/protocol\\.ts"
  )
  local allowlist_regex
  allowlist_regex=$(printf '|%s' "${allowlist_patterns[@]}")
  allowlist_regex="(${allowlist_regex:1})"

  local legacy_names=(
    "MsgRegisterLocalRequest"
    "MsgRegisterLocalResponse"
    "MsgHeartbeatLocalRequest"
    "MsgHeartbeatLocalResponse"
    "MSG_REGISTER_LOCAL_REQUEST"
    "MSG_REGISTER_LOCAL_RESPONSE"
    "MSG_HEARTBEAT_LOCAL_REQUEST"
    "MSG_HEARTBEAT_LOCAL_RESPONSE"
    "MSG_START_JOB_REQUEST"
    "MSG_CANCEL_JOB_REQUEST"
    "MSG_STREAM_JOB_REQUEST"
    "MSG_JOB_EVENT"
  )

  local found=0
  local seen_file=""
  for name in "${legacy_names[@]}"; do
    # Search in source files (exclude generated, build, .git, etc.).
    while IFS= read -r match; do
      [ -z "$match" ] && continue
      local file="${match%%:*}"
      if [[ ! "$file" =~ $allowlist_regex ]]; then
        # Dedup at file granularity (one warning per offending file).
        if [[ "$seen_file" == *"$file"* ]]; then
          continue
        fi
        seen_file="${seen_file}${file}|"
        echo "${YELLOW}[wire] ${file} still references legacy names${RESET}"
        found=$((found + 1))
        warnings=$((warnings + 1))
      fi
    done < <(grep -rnI --include='*.go' --include='*.py' --include='*.ts' --include='*.cs' --include='*.cpp' --include='*.h' --include='*.java' "$name" sdks/ 2>/dev/null || true)
  done

  if [ "$found" -eq 0 ]; then
    echo "${GREEN}[wire] OK${RESET}  ${DIM}(legacy names only in compatibility-alias modules)${RESET}"
  else
    echo "${DIM}[wire] ${found} file(s) need migration; see todo.md and sdks/SDK_FEATURE_MATRIX.md${RESET}"
  fi
}

# ---------------------------------------------------------------------------
# Run all checks.
# ---------------------------------------------------------------------------

echo "==> SDK matrix conformance check"
echo
check_go
check_python
check_java
check_js
check_cpp
check_csharp
echo
check_invoker_presence
echo
check_invoker_naming
echo
check_readme_hygiene
echo
check_wire_name_hygiene
echo

if [ "$warnings" -gt 0 ]; then
  echo "${YELLOW}Warnings: ${warnings} (informational only)${RESET}"
fi

if [ "$errors" -gt 0 ]; then
  echo "${RED}Failed: ${errors} issue(s) above.${RESET}"
  echo "Update sdks/SDK_FEATURE_MATRIX.md first, then mirror changes here."
  exit 1
fi

echo "${GREEN}All required L1 symbols present.${RESET}"
exit 0
