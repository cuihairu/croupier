#!/bin/bash
# Sync root VERSION to all SDK manifests and version constants.
# Root VERSION is the canonical version source; run after bumping it:
#   make version-sync
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION_FILE="VERSION"
if [[ ! -f "$VERSION_FILE" ]]; then
    echo "[version] ERROR: $VERSION_FILE not found" >&2
    exit 1
fi

VERSION=$(tr -d '[:space:]' < "$VERSION_FILE")
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "[version] ERROR: invalid version '$VERSION' (expect MAJOR.MINOR.PATCH)" >&2
    exit 1
fi

MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)

sed_i() {
    if sed --version >/dev/null 2>&1; then
        sed -i "$@"
    else
        sed -i '' "$@"
    fi
}

# Go SDK
sed_i 's/^const Version = ".*"/const Version = "'"$VERSION"'"/' sdks/go/version.go
echo "[version] sdks/go/version.go -> $VERSION"

# JS SDK
node -e '
const fs = require("fs");
const p = "sdks/js/package.json";
const pkg = JSON.parse(fs.readFileSync(p, "utf8"));
pkg.version = process.argv[1];
fs.writeFileSync(p, JSON.stringify(pkg, null, 2) + "\n");
' "$VERSION"
echo "[version] sdks/js/package.json -> $VERSION"

sed_i 's/^export const SDK_VERSION = ".*";/export const SDK_VERSION = "'"$VERSION"'";/' sdks/js/src/version.ts
echo "[version] sdks/js/src/version.ts -> $VERSION"

# Python SDK
sed_i 's/^version = ".*"/version = "'"$VERSION"'"/' sdks/python/pyproject.toml
echo "[version] sdks/python/pyproject.toml -> $VERSION"

# Java SDK
sed_i "s/^String baseVersion = '.*'/String baseVersion = '$VERSION'/" sdks/java/build.gradle
echo "[version] sdks/java/build.gradle -> $VERSION"

sed_i 's/^    public static final String VERSION = ".*";/    public static final String VERSION = "'"$VERSION"'";/' \
    sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/SdkInfo.java
echo "[version] sdks/java SdkInfo.VERSION -> $VERSION"

# C# SDK
sed_i 's|<Version>.*</Version>|<Version>'"$VERSION"'</Version>|' sdks/csharp/src/Croupier.Sdk/Croupier.Sdk.csproj
echo "[version] sdks/csharp Croupier.Sdk.csproj -> $VERSION"

# C++ SDK
sed_i 's/^set(CROUPIER_SDK_VERSION_MAJOR .*)/set(CROUPIER_SDK_VERSION_MAJOR '"$MAJOR"')/' sdks/cpp/VERSION.cmake
sed_i 's/^set(CROUPIER_SDK_VERSION_MINOR .*)/set(CROUPIER_SDK_VERSION_MINOR '"$MINOR"')/' sdks/cpp/VERSION.cmake
sed_i 's/^set(CROUPIER_SDK_VERSION_PATCH .*)/set(CROUPIER_SDK_VERSION_PATCH '"$PATCH"')/' sdks/cpp/VERSION.cmake
sed_i 's/^#define CROUPIER_SDK_VERSION ".*"/#define CROUPIER_SDK_VERSION "'"$VERSION"'"/' sdks/cpp/src/croupier_client.cpp
echo "[version] sdks/cpp -> $VERSION"

echo "[version] All SDK versions synchronized to $VERSION"
