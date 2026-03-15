#!/usr/bin/env bash
set -euo pipefail

echo "Running architecture guard checks..."

fail() {
  echo "ARCH_GUARD_FAILED: $1" >&2
  exit 1
}

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

if rg -n "/out/packs|/app/packs" docker >/dev/null 2>&1; then
  fail "docker images must not depend on removed packs path"
fi

echo "Architecture guard checks passed."
