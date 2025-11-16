#!/usr/bin/env bash
# Local test runner for macOS/Linux with SQLite (pure Go driver), no CGO required.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

MODE="memory"        # memory | file
RACE=0
RUN_REGEX=""
# Default packages: whole repo
PKGS=(./...)

usage() {
  cat <<'USAGE'
Usage: scripts/test-local.sh [options]

Options:
  --memory            Use in-memory SQLite (default)
  --file              Use file-backed SQLite at data/croupier_test.db
  --race              Enable -race
  --run REGEX         Run tests matching regex (go test -run)
  --pkg "PATTERNS"    Space-separated package patterns (default: ./...)
  -h, --help          Show this help

Examples:
  scripts/test-local.sh
  scripts/test-local.sh --file
  scripts/test-local.sh --race --run TestFunctionManagementAPIs --pkg "./internal/app/server/http ./internal/service/games"
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --memory) MODE="memory"; shift;;
    --file) MODE="file"; shift;;
    --race) RACE=1; shift;;
    --run) RUN_REGEX="${2:-}"; shift 2;;
    --pkg) read -r -a PKGS <<< "${2:-}"; shift 2;;
    -h|--help) usage; exit 0;;
    *) echo "Unknown option: $1"; usage; exit 2;;
  esac
done

# Environment setup
export CGO_ENABLED=0
export DB_DRIVER=sqlite
export GIN_MODE=release

if [[ "$MODE" == "file" ]]; then
  mkdir -p data logs
  export DATABASE_URL="file:data/croupier_test.db?cache=shared&_pragma=foreign_keys(1)"
else
  mkdir -p logs
  export DATABASE_URL=":memory:"
fi

echo "[test] mode=$MODE db=${DATABASE_URL}"
echo "[test] pkgs: ${PKGS[*]}"

ARGS=(-count=1 -v)
if [[ $RACE -eq 1 ]]; then
  ARGS+=(-race)
fi
if [[ -n "$RUN_REGEX" ]]; then
  ARGS+=(-run "$RUN_REGEX")
fi

# Ensure modules are downloaded (some CI/dev envs have stale caches)
GOFLAGS= GOWORK=off go mod download

# Run tests
go test "${PKGS[@]}" "${ARGS[@]}"

