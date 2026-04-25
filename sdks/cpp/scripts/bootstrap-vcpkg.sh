#!/usr/bin/env bash

set -euo pipefail

VCPKG_ROOT="${1:?missing vcpkg root}"
VCPKG_COMMIT="${2:?missing vcpkg commit}"

# Ensure we have a clean vcpkg installation
if [[ ! -f "$VCPKG_ROOT/vcpkg" || ! -f "$VCPKG_ROOT/.vcpkg-root" ]]; then
  rm -rf "$VCPKG_ROOT"
  mkdir -p "$VCPKG_ROOT"

  # Clone vcpkg with full history to get all git objects needed for port versioning
  # This is required because vcpkg's internal versioning system uses git tree objects
  echo "Cloning vcpkg (this may take a few minutes for full history)..."
  git clone https://github.com/microsoft/vcpkg.git "$VCPKG_ROOT"

  # Fetch the specific tag/commit to ensure we have all objects
  echo "Fetching vcpkg version $VCPKG_COMMIT..."
  git -C "$VCPKG_ROOT" fetch origin "refs/tags/$VCPKG_COMMIT" 2>/dev/null || \
  git -C "$VCPKG_ROOT" fetch origin "$VCPKG_COMMIT"

  # Checkout the desired version
  git -C "$VCPKG_ROOT" checkout "$VCPKG_COMMIT"

  # Remove the origin remote to prevent any accidental operations on upstream
  git -C "$VCPKG_ROOT" remote remove origin

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
