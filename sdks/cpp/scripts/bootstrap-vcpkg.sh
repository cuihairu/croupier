#!/usr/bin/env bash

set -euo pipefail

VCPKG_ROOT="${1:?missing vcpkg root}"
VCPKG_COMMIT="${2:-master}"  # Default to master if no version specified

# Ensure we have a clean vcpkg installation
# Re-clone if directory exists but is a submodule residue (.git is a file, not a directory)
needs_clone=false
if [[ -d "$VCPKG_ROOT" ]]; then
  if [[ -f "$VCPKG_ROOT/.git" ]]; then
    echo "Detected submodule residue (.git is a file), re-cloning..."
    rm -rf "$VCPKG_ROOT"
    needs_clone=true
  elif [[ ! -f "$VCPKG_ROOT/vcpkg" || ! -f "$VCPKG_ROOT/.vcpkg-root" ]]; then
    rm -rf "$VCPKG_ROOT"
    needs_clone=true
  fi
else
  needs_clone=true
fi

if [[ "$needs_clone" == "true" ]]; then
  mkdir -p "$VCPKG_ROOT"

  # Clone vcpkg with full history to get all git objects needed for port versioning
  echo "Cloning vcpkg (this may take a few minutes for full history)..."
  git clone https://github.com/microsoft/vcpkg.git "$VCPKG_ROOT"

  # If a specific version was requested, fetch and checkout
  if [[ "$VCPKG_COMMIT" != "master" ]]; then
    echo "Fetching vcpkg version $VCPKG_COMMIT..."
    git -C "$VCPKG_ROOT" fetch origin "refs/tags/$VCPKG_COMMIT" 2>/dev/null || \
    git -C "$VCPKG_ROOT" fetch origin "$VCPKG_COMMIT"

    echo "Checking out $VCPKG_COMMIT..."
    git -C "$VCPKG_ROOT" checkout "$VCPKG_COMMIT"
  fi

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
