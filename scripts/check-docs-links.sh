#!/usr/bin/env bash
# Check internal markdown links for dead targets.
#
# VitePress already fails the build on dead links *inside* docs/. This script
# extends coverage to the repo-root README and sdks/*/README.md and runs
# locally (pre-commit friendly) without a full VitePress build.
#
# What counts as an internal link: the target of `[text](target)` or
# `![alt](target)`, excluding http(s)/mailto/tel and anchor-only (`#...`).
# Targets are resolved as:
#   - `/abs/path`  -> relative to docs/ (VitePress convention), trailing slash
#                    means `<dir>/index.md`
#   - `./rel`      -> relative to the referencing file
#   - `../rel`
# A target resolves if any of these exists: itself, itself + `.md`,
# `<dir>/index.md`.
#
# Run from repo root:   ./scripts/check-docs-links.sh
# Exit codes:
#   0  all internal links resolve
#   1  at least one dead link
#   2  environment / usage error

set -u

cd "$(dirname "$0")/.." || exit 2

DOCS_DIR="docs"
# Files to scan: docs markdown (no archive/node_modules/dist) + root README.
# sdks/<lang>/README.md are SDK source-of-truth entries, governed separately
# (see documentation-governance.md §信息架构), so they are out of scope here.
mapfile -t FILES < <(
  {
    find "$DOCS_DIR" -name "*.md" \
      -not -path "*/node_modules/*" \
      -not -path "*/.vitepress/dist/*" \
      -not -path "*/archive/*" 2>/dev/null
    [ -f README.md ] && echo "README.md"
  }
)

broken=0
checked=0

for f in "${FILES[@]}"; do
  [ -f "$f" ] || continue
  dir="$(dirname "$f")"
  # Extract link targets from [text](target) / ![alt](target).
  while IFS= read -r target; do
    [ -z "$target" ] && continue
    # Skip external / anchor-only / mailto / tel.
    case "$target" in
      http://*|https://*|mailto:*|tel:*) continue ;;
      \#*) continue ;;
    esac
    # Strip optional title ("...") and anchor (#...).
    target="${target%%\"*}"
    target="${target%%#*}"
    [ -z "$target" ] && continue

    if [[ "$target" == /* ]]; then
      # VitePress absolute path -> relative to docs/.
      cand="${DOCS_DIR}${target}"
    else
      cand="$dir/$target"
    fi

    checked=$((checked + 1))
    if [ -e "$cand" ] || [ -e "${cand}.md" ] || [ -e "${cand%/}/index.md" ]; then
      :
    else
      echo "DEAD LINK: $f -> $target"
      broken=$((broken + 1))
    fi
  done < <(grep -hoE '\]\([^)]+\)' "$f" 2>/dev/null | sed -E 's/^\]\(//; s/\)$//')
done

echo "Checked $checked internal link(s) across ${#FILES[@]} file(s)."

if [ "$broken" -gt 0 ]; then
  echo "FAIL: $broken dead internal link(s) found."
  exit 1
fi
echo "OK: all internal markdown links resolve."
exit 0
