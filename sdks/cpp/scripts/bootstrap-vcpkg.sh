#!/usr/bin/env bash

set -euo pipefail

VCPKG_ROOT="${1:?missing vcpkg root}"
VCPKG_COMMIT="${2:?missing vcpkg commit}"

if [[ ! -f "$VCPKG_ROOT/bootstrap-vcpkg.sh" ]]; then
  rm -rf "$VCPKG_ROOT"
  git clone https://github.com/microsoft/vcpkg.git "$VCPKG_ROOT"
fi

git -C "$VCPKG_ROOT" fetch --depth 1 origin "$VCPKG_COMMIT"
git -C "$VCPKG_ROOT" checkout --force FETCH_HEAD

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
