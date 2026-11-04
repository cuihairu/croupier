#!/bin/bash
# check-version-sync.sh — version consistency guard (single source of truth: VERSION).
#
# Rules:
#   1. VERSION file must exist and be valid MAJOR.MINOR.PATCH.
#   2. All six SDK manifests must equal VERSION (strict equality):
#        - Go:     sdks/go/version.go                          (const Version)
#        - JS:     sdks/js/package.json                        (version)
#                  sdks/js/src/version.ts                      (SDK_VERSION)
#        - Python: sdks/python/pyproject.toml                  (project.version)
#        - Java:   sdks/java/build.gradle                      (baseVersion)
#        - C#:     sdks/csharp/src/Croupier.Sdk/Croupier.Sdk.csproj (<Version>)
#        - C++:    sdks/cpp/VERSION.cmake                      (MAJOR/MINOR/PATCH)
#                  sdks/cpp/src/croupier_client.cpp            (CROUPIER_SDK_VERSION)
#   3. Latest release tag (vX.Y.Z, "nightly" ignored):
#        - default (CI / local): VERSION must NOT be lower than the latest tag
#          (being ahead of the tag is the normal between-releases state).
#        - --require-tag-match (release pipeline): VERSION must EQUAL the latest tag.
#
# Usage:
#   scripts/check-version-sync.sh                       # CI guard
#   scripts/check-version-sync.sh --require-tag-match   # release gate
#   scripts/check-version-sync.sh --expect-tag vX.Y.Z   # verify against a given tag
#
# Note: no version value is ever modified here — this is a read-only check.
# To align manifests after bumping VERSION, run `make version-sync`.
set -euo pipefail

cd "$(dirname "$0")/.."

FAILURES=()
REQUIRE_TAG_MATCH=0
EXPECT_TAG=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --require-tag-match) REQUIRE_TAG_MATCH=1; shift ;;
        --expect-tag) EXPECT_TAG="${2:-}"; REQUIRE_TAG_MATCH=1; shift 2 ;;
        *) echo "[version-guard] unknown option: $1" >&2; exit 2 ;;
    esac
done

fail() { FAILURES+=("$1"); echo "[version-guard] FAIL: $1"; }
ok()   { echo "[version-guard] ok: $1"; }

# --- 1. VERSION file ---
if [[ ! -f VERSION ]]; then
    fail "VERSION file not found at repo root"
    VERSION=""
else
    VERSION=$(tr -d '[:space:]' < VERSION)
    if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        fail "VERSION file content '$VERSION' is not MAJOR.MINOR.PATCH"
        VERSION=""
    else
        ok "VERSION = $VERSION"
    fi
fi

check_eq() { # <label> <file> <actual> 
    local label="$1" file="$2" actual="$3"
    if [[ "$actual" == "$VERSION" ]]; then
        ok "$label = $actual"
    else
        fail "$label = '${actual:-<missing>}' != VERSION $VERSION ($file)"
    fi
}

if [[ -n "$VERSION" ]]; then
    # --- Go SDK ---
    go_ver=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' sdks/go/version.go | head -1)
    check_eq "go sdks/go/version.go" sdks/go/version.go "$go_ver"

    # --- JS SDK ---
    js_ver=$(node -e 'try{console.log(JSON.parse(require("fs").readFileSync("sdks/js/package.json","utf8")).version)}catch(e){}' 2>/dev/null || \
             sed -n 's/^  "version": "\(.*\)",$/\1/p' sdks/js/package.json | head -1)
    check_eq "js sdks/js/package.json" sdks/js/package.json "$js_ver"
    js_ts=$(sed -n 's/^export const SDK_VERSION = "\(.*\)";$/\1/p' sdks/js/src/version.ts | head -1)
    check_eq "js sdks/js/src/version.ts" sdks/js/src/version.ts "$js_ts"

    # --- Python SDK ---
    py_ver=$(awk '/^\[project\]/{f=1;next} /^\[/{f=0} f && match($0, /^version *= *"[^"]+"/){s=substr($0,RSTART,RLENGTH); sub(/^version *= *"/,"",s); sub(/"$/,"",s); print s; exit}' sdks/python/pyproject.toml)
    check_eq "python sdks/python/pyproject.toml" sdks/python/pyproject.toml "$py_ver"

    # --- Java SDK ---
    java_ver=$(sed -n "s/^String baseVersion = '\(.*\)'$/\1/p" sdks/java/build.gradle | head -1)
    check_eq "java sdks/java/build.gradle" sdks/java/build.gradle "$java_ver"
    java_const=$(sed -n 's/^    public static final String VERSION = "\(.*\)";$/\1/p' \
        sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/SdkInfo.java | head -1)
    check_eq "java SdkInfo.VERSION" sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/SdkInfo.java "$java_const"

    # --- C# SDK ---
    cs_ver=$(sed -n 's|^    <Version>\(.*\)</Version>|\1|p' sdks/csharp/src/Croupier.Sdk/Croupier.Sdk.csproj | head -1)
    check_eq "csharp Croupier.Sdk.csproj" sdks/csharp/src/Croupier.Sdk/Croupier.Sdk.csproj "$cs_ver"

    # --- C++ SDK ---
    cpp_major=$(sed -n 's/^set(CROUPIER_SDK_VERSION_MAJOR \([0-9]*\))$/\1/p' sdks/cpp/VERSION.cmake)
    cpp_minor=$(sed -n 's/^set(CROUPIER_SDK_VERSION_MINOR \([0-9]*\))$/\1/p' sdks/cpp/VERSION.cmake)
    cpp_patch=$(sed -n 's/^set(CROUPIER_SDK_VERSION_PATCH \([0-9]*\))$/\1/p' sdks/cpp/VERSION.cmake)
    if [[ -n "$cpp_major$cpp_minor$cpp_patch" ]]; then
        cpp_ver="${cpp_major}.${cpp_minor}.${cpp_patch}"
        check_eq "cpp sdks/cpp/VERSION.cmake" sdks/cpp/VERSION.cmake "$cpp_ver"
    else
        fail "cpp sdks/cpp/VERSION.cmake: MAJOR/MINOR/PATCH not parseable"
    fi
    cpp_define=$(sed -n 's/^#define CROUPIER_SDK_VERSION "\(.*\)"$/\1/p' sdks/cpp/src/croupier_client.cpp | head -1)
    check_eq "cpp croupier_client.cpp" sdks/cpp/src/croupier_client.cpp "$cpp_define"
fi

# --- 3. Tag consistency (nightly is a moving tag, ignore it) ---
latest_tag=$(git tag --list 'v[0-9]*' --sort=-v:refname | head -1 || true)
tag_version="${latest_tag#v}"

if [[ -n "$EXPECT_TAG" ]]; then
    expected="${EXPECT_TAG#v}"
    if [[ "$VERSION" == "$expected" ]]; then
        ok "VERSION == release tag $EXPECT_TAG"
    else
        fail "VERSION ($VERSION) != release tag $EXPECT_TAG — refusing to publish"
    fi
elif [[ "$REQUIRE_TAG_MATCH" == "1" ]]; then
    if [[ -z "$latest_tag" ]]; then
        fail "no vX.Y.Z tag found but --require-tag-match requested"
    elif [[ "$VERSION" == "$tag_version" ]]; then
        ok "VERSION == latest tag ${latest_tag}"
    else
        fail "VERSION ($VERSION) != latest tag ${latest_tag} (stripped: $tag_version)"
    fi
else
    if [[ -n "$latest_tag" ]]; then
        if [[ "$(printf '%s\n%s\n' "$tag_version" "$VERSION" | sort -V | head -1)" == "$tag_version" ]]; then
            ok "VERSION ($VERSION) >= latest tag ${latest_tag}"
        else
            fail "VERSION ($VERSION) is LOWER than already-released tag ${latest_tag} — version regression"
        fi
    else
        ok "no release tag yet, skipping tag comparison"
    fi
fi

if [[ ${#FAILURES[@]} -gt 0 ]]; then
    echo "[version-guard] ${#FAILURES[@]} check(s) failed. Run 'make version-sync' to align manifests to VERSION (do not hand-edit)."
    exit 1
fi
echo "[version-guard] all version declarations consistent."
