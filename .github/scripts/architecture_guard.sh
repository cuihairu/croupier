#!/usr/bin/env bash
set -euo pipefail

echo "Running architecture guard checks..."

fail() {
  echo "ARCH_GUARD_FAILED: $1" >&2
  exit 1
}

# Agent common runtime and role implementations must remain usable without
# importing the Server/sidecar internal application tree.
if rg -n '"github\.com/cuihairu/croupier/internal/' core agents --glob '*.go' --glob '!**/*_test.go'; then
  fail "core/ and agents/ production packages must not import internal/"
fi

if git ls-files --error-unmatch "configs/platforms.yaml" >/dev/null 2>&1; then
  fail "legacy config file configs/platforms.yaml must not exist"
fi

if git ls-files --error-unmatch "internal/platform/loader.go" >/dev/null 2>&1; then
  fail "legacy loader internal/platform/loader.go must not exist"
fi

if git ls-files --error-unmatch "internal/api/platform/legacy_gateway.go" >/dev/null 2>&1; then
  fail "legacy gateway internal/api/platform/legacy_gateway.go must not exist"
fi

if git ls-files --error-unmatch "internal/platform/migrationflags/flags.go" >/dev/null 2>&1; then
  fail "legacy migration flags internal/platform/migrationflags/flags.go must not exist"
fi

if rg -n "PlatformLoader|initPlatformLoader" internal --glob '!**/*_test.go' >/dev/null 2>&1; then
  fail "legacy platform loader symbols are still referenced in internal/"
fi

# Keep the architecture state table synchronized with production wiring. The
# ServiceContext must initialize approvals through the persistent SQL store;
# falling back to MemStore would silently remove restart/cross-instance state.
if ! rg -n 'approvalsStore, err := approvals\.NewSQLStore\(db\)' internal/svc/service_context.go >/dev/null; then
  fail "ServiceContext approval wiring must initialize approvals.NewSQLStore"
fi
if ! rg -n 'ApprovalsStore:\s+approvalsStore' internal/svc/service_context.go >/dev/null; then
  fail "ServiceContext must assign the initialized SQL approval store"
fi
if rg -n 'Function approvals\s*\|\s*MemStore in current ServiceContext wiring' docs/architecture/index.md; then
  fail "architecture state table still claims approval wiring uses MemStore"
fi

if rg -n "/out/packs|/app/packs" docker >/dev/null 2>&1; then
  fail "docker images must not depend on removed packs path"
fi

echo "Architecture guard checks passed."
