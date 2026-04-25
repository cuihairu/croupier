#!/usr/bin/env bash

set -euo pipefail

VCPKG_ROOT="${1:?missing vcpkg root}"
VCPKG_COMMIT="${2:?missing vcpkg commit}"

if [[ ! -f "$VCPKG_ROOT/bootstrap-vcpkg.sh" ]]; then
  rm -rf "$VCPKG_ROOT"
  # Clone with depth 1 and checkout the specific tag/commit in one command
  git clone --depth 1 --branch "$VCPKG_COMMIT" https://github.com/microsoft/vcpkg.git "$VCPKG_ROOT" || {
    # If branch doesn't exist (for tags), try cloning then checkout
    rm -rf "$VCPKG_ROOT"
    git clone --depth 1 https://github.com/microsoft/vcpkg.git "$VCPKG_ROOT"
    git -C "$VCPKG_ROOT" fetch --tags --depth 1 origin tag "$VCPKG_COMMIT" || \
    git -C "$VCPKG_ROOT" fetch --depth 1 origin "$VCPKG_COMMIT"
    git -C "$VCPKG_ROOT" checkout --force "$VCPKG_COMMIT"
  }
fi

chmod +x "$VCPKG_ROOT/bootstrap-vcpkg.sh"
"$VCPKG_ROOT/bootstrap-vcpkg.sh" -disableMetrics

echo "VCPKG_ROOT=$VCPKG_ROOT" >> "$GITHUB_ENV"
echo "VCPKG_FORCE_SYSTEM_BINARIES=1" >> "$GITHUB_ENV"

for name in VCPKG_DEFAULT_TRIPLET VCPKG_BUILD_TYPE; do
  if [[ -n "${!name:-}" ]]; then
    echo "$name=${!name}" >> "$GITHUB_ENV"
  fi
done

echo "$VCPKG_ROOT" >> "$GITHUB_PATH"
