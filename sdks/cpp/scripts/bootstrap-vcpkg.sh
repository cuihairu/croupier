#!/usr/bin/env bash

set -euo pipefail

VCPKG_ROOT="${1:?missing vcpkg root}"
VCPKG_COMMIT="${2:?missing vcpkg commit}"

# Ensure we have a clean vcpkg installation
if [[ ! -f "$VCPKG_ROOT/vcpkg" || ! -f "$VCPKG_ROOT/.vcpkg-root" ]]; then
  rm -rf "$VCPKG_ROOT"
  mkdir -p "$VCPKG_ROOT"

  # Use a temporary directory for download
  TMP_DIR="$(mktemp -d)"
  trap "rm -rf '$TMP_DIR'" EXIT

  # Download vcpkg as a zip archive from GitHub
  # This avoids git shallow clone issues entirely
  VCPKG_URL="https://github.com/microsoft/vcpkg/archive/refs/tags/${VCPKG_COMMIT}.tar.gz"

  echo "Downloading vcpkg from: $VCPKG_URL"
  if ! curl -fsSL "$VCPKG_URL" -o "$TMP_DIR/vcpkg.tar.gz"; then
    # If tag download fails, try commit download
    VCPKG_URL="https://github.com/microsoft/vcpkg/archive/${VCPKG_COMMIT}.tar.gz"
    echo "Retrying from: $VCPKG_URL"
    curl -fsSL "$VCPKG_URL" -o "$TMP_DIR/vcpkg.tar.gz"
  fi

  # Extract the archive
  tar -xzf "$TMP_DIR/vcpkg.tar.gz" -C "$TMP_DIR"

  # Copy files to vcpkg root (the archive extracts to vcpkg-COMMIT or vcpkg-TAG)
  find "$TMP_DIR" -maxdepth 1 -type d -name "vcpkg-*" | while read dir; do
    rsync -a "$dir/" "$VCPKG_ROOT/"
  done

  # Initialize a fresh git repository for vcpkg's internal operations
  # This is needed for vcpkg's versioning system to work
  git -C "$VCPKG_ROOT" init -q
  git -C "$VCPKG_ROOT" config user.name "vcpkg"
  git -C "$VCPKG_ROOT" config user.email "vcpkg@localhost"
  git -C "$VCPKG_ROOT" add -A
  git -C "$VCPKG_ROOT" commit -q -m "initial commit"

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
