#!/usr/bin/env bash
#
# sync-sdk-versions.sh - Synchronize SDK versions across all SDKs
#
# Usage:
#   ./scripts/sync-sdk-versions.sh [VERSION]
#
# If VERSION is not provided, reads from the VERSION file in the repo root.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_FILE="${REPO_ROOT}/VERSION"

# Determine target version
if [ $# -ge 1 ]; then
    VERSION="$1"
    echo "Setting all SDK versions to: ${VERSION}"
    echo "${VERSION}" > "${VERSION_FILE}"
else
    if [ ! -f "${VERSION_FILE}" ]; then
        echo "Error: VERSION file not found and no version argument provided"
        exit 1
    fi
    VERSION="$(cat "${VERSION_FILE}" | tr -d '[:space:]')"
    echo "Syncing all SDK versions to: ${VERSION} (from VERSION file)"
fi

# Validate version format (semver)
if ! echo "${VERSION}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$'; then
    echo "Error: Invalid version format '${VERSION}'. Expected semver (e.g., 0.1.0 or 1.0.0-beta.1)"
    exit 1
fi

echo ""
echo "=== Updating SDK versions to ${VERSION} ==="

# JavaScript SDK (package.json)
if [ -f "${REPO_ROOT}/sdks/js/package.json" ]; then
    echo "  - Updating JS SDK (package.json)..."
    sed -i.bak -E "s/\"version\": \"[^\"]+\"/\"version\": \"${VERSION}\"/" \
        "${REPO_ROOT}/sdks/js/package.json"
    rm -f "${REPO_ROOT}/sdks/js/package.json.bak"
fi

# Python SDK (setup.py)
if [ -f "${REPO_ROOT}/sdks/python/setup.py" ]; then
    echo "  - Updating Python SDK (setup.py)..."
    sed -i.bak -E "s/version=\"[^\"]+\"/version=\"${VERSION}\"/" \
        "${REPO_ROOT}/sdks/python/setup.py"
    rm -f "${REPO_ROOT}/sdks/python/setup.py.bak"
fi

# Java SDK (build.gradle)
if [ -f "${REPO_ROOT}/sdks/java/build.gradle" ]; then
    echo "  - Updating Java SDK (build.gradle)..."
    sed -i.bak -E "s/^version = '[^']+'/version = '${VERSION}'/" \
        "${REPO_ROOT}/sdks/java/build.gradle"
    rm -f "${REPO_ROOT}/sdks/java/build.gradle.bak"
fi

# C++ SDK (CMakeLists.txt)
if [ -f "${REPO_ROOT}/sdks/cpp/CMakeLists.txt" ]; then
    echo "  - Updating C++ SDK (CMakeLists.txt)..."
    sed -i.bak -E "s/VERSION [0-9]+\.[0-9]+\.[0-9]+/VERSION ${VERSION}/" \
        "${REPO_ROOT}/sdks/cpp/CMakeLists.txt"
    rm -f "${REPO_ROOT}/sdks/cpp/CMakeLists.txt.bak"
fi

# Go SDK (version.go) - create if doesn't exist
GO_VERSION_FILE="${REPO_ROOT}/sdks/go/version.go"
if [ ! -f "${GO_VERSION_FILE}" ]; then
    echo "  - Creating Go SDK version file (version.go)..."
    cat > "${GO_VERSION_FILE}" <<EOF
package croupier

// Version is the current version of the Croupier Go SDK
const Version = "${VERSION}"
EOF
else
    echo "  - Updating Go SDK (version.go)..."
    sed -i.bak -E "s/const Version = \"[^\"]+\"/const Version = \"${VERSION}\"/" \
        "${GO_VERSION_FILE}"
    rm -f "${GO_VERSION_FILE}.bak"
fi

echo ""
echo "✅ All SDK versions synchronized to ${VERSION}"
echo ""
echo "Modified files:"
echo "  - VERSION"
echo "  - sdks/js/package.json"
echo "  - sdks/python/setup.py"
echo "  - sdks/java/build.gradle"
echo "  - sdks/cpp/CMakeLists.txt"
echo "  - sdks/go/version.go"
echo ""
echo "⚠️  Don't forget to:"
echo "  1. Update pnpm-lock.yaml: cd sdks/js && pnpm install"
echo "  2. Review changes: git diff"
echo "  3. Commit changes: git add -A && git commit -m 'chore: bump SDK versions to ${VERSION}'"
