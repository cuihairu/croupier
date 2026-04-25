#!/usr/bin/env bash

set -euo pipefail

VCPKG_ROOT="${1:?missing vcpkg root}"
VCPKG_COMMIT="${2:?missing vcpkg commit}"

if [[ ! -f "$VCPKG_ROOT/bootstrap-vcpkg.sh" ]]; then
  rm -rf "$VCPKG_ROOT"
  mkdir -p "$VCPKG_ROOT"

  # Initialize a fresh git repository for vcpkg
  git init "$VCPKG_ROOT"
  git -C "$VCPKG_ROOT" remote add origin https://github.com/microsoft/vcpkg.git

  # Fetch the specific tag/commit
  git -C "$VCPKG_ROOT" fetch --depth 1 origin "refs/tags/$VCPKG_COMMIT" 2>/dev/null || \
  git -C "$VCPKG_ROOT" fetch --depth 1 origin "$VCPKG_COMMIT"

  # Checkout the fetched content
  git -C "$VCPKG_ROOT" checkout FETCH_HEAD

  # Replace .git directory with a file to prevent git from using this as a repo
  # and prevent git from searching parent directories
  rm -rf "$VCPKG_ROOT/.git"
  touch "$VCPKG_ROOT/.git"
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
