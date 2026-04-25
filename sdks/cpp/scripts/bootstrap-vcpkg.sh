#!/usr/bin/env bash

set -euo pipefail

VCPKG_ROOT="${1:?missing vcpkg root}"
VCPKG_COMMIT="${2:?missing vcpkg commit}"

# Ensure we have a clean vcpkg installation
if [[ ! -f "$VCPKG_ROOT/vcpkg" || ! -f "$VCPKG_ROOT/.vcpkg-root" ]]; then
  rm -rf "$VCPKG_ROOT"
  mkdir -p "$VCPKG_ROOT"

  # Use a temporary directory for git operations to avoid any interference
  TMP_DIR="$(mktemp -d)"
  trap "rm -rf '$TMP_DIR'" EXIT

  # Clone vcpkg to a temporary location first
  git clone --depth 1 --branch "$VCPKG_COMMIT" https://github.com/microsoft/vcpkg.git "$TMP_DIR/vcpkg" 2>/dev/null || \
  git clone --depth 1 --single-branch --shallow-submodules https://github.com/microsoft/vcpkg.git "$TMP_DIR/vcpkg"

  # Copy vcpkg files including the .git directory
  rsync -a "$TMP_DIR/vcpkg/" "$VCPKG_ROOT/"

  # Detach from the original remote to prevent any accidental fetching/pushing
  # This keeps vcpkg's internal git working while isolating it from project git
  git -C "$VCPKG_ROOT" remote remove origin 2>/dev/null || true
  git -C "$VCPKG_ROOT" config user.name "vcpkg"
  git -C "$VCPKG_ROOT" config user.email "vcpkg@localhost"

  # Mark this as a valid vcpkg root
  touch "$VCPKG_ROOT/.vcpkg-root"
fi

# Build vcpkg
chmod +x "$VCPKG_ROOT/bootstrap-vcpkg.sh"
"$VCPKG_ROOT/bootstrap-vcpkg.sh" -disableMetrics

echo "VCPKG_ROOT=$VCPKG_ROOT" >> "$GITHUB_ENV"
echo "VCPKG_FORCE_SYSTEM_BINARIES=1" >> "$GITHUB_ENV"

for name in VCPKG_DEFAULT_TRIPLET VCPKG_BUILD_TYPE; do
  value="${!name:-}"
  if [[ -n "$value" ]]; then
    echo "$name=$value" >> "$GITHUB_ENV"
  fi
done

echo "$VCPKG_ROOT" >> "$GITHUB_PATH"
