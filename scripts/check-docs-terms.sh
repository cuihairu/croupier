#!/usr/bin/env bash
# Check deprecated terms in docs (documentation-governance.md §过时术语处理).
#
# These terms may appear ONLY in compatibility / historical background /
# proposal-migration / decision context. To avoid false positives on existing
# legitimate mentions, this script uses a **baseline diff**: the current set
# of (file, term) mentions is committed as the baseline; any NEW mention not
# in the baseline fails the check (regression), and mentions removed from docs
# are reported as stale baseline entries to refresh.
#
# Deprecated terms (governance):
#   gRPC                 — as SDK/Agent default link
#   LocalControlService  — as new semantic entrypoint
#   rpc_addr             — as long-term runtime dependency
#   REQ/REP              — as the main link model
#
# Scope: docs/ markdown, excluding archive/, node_modules/, .vitepress/dist/.
# sdks/<lang>/README.md are SDK source-of-truth entries, out of scope.
#
# Run from repo root:
#   ./scripts/check-docs-terms.sh           # check
#   ./scripts/check-docs-terms.sh --update  # rewrite baseline after review
# Exit codes:
#   0  no regressions (baseline may have stale notes)
#   1  deprecated term(s) appeared in new places
#   2  baseline missing / usage error

set -u

cd "$(dirname "$0")/.." || exit 2

# Stable byte-order sorting for both sort and comm (avoids locale-dependent
# ordering mismatches between local runs and CI).
export LC_ALL=C

BASELINE="scripts/docs-terms-baseline.txt"
DOCS_DIR="docs"
TERMS=(gRPC LocalControlService rpc_addr 'REQ/REP')

scan_current() {
  for term in "${TERMS[@]}"; do
    grep -rl --include="*.md" -- "$term" "$DOCS_DIR" 2>/dev/null \
      | grep -vE "/archive/|/node_modules/|/\.vitepress/dist/" \
      | while IFS= read -r f; do echo "$f|$term"; done
  done | sort -u
}

tmp_current="$(mktemp)"
trap 'rm -f "$tmp_current"' EXIT
scan_current > "$tmp_current"

if [ "${1:-}" = "--update" ]; then
  cp "$tmp_current" "$BASELINE"
  echo "Updated $BASELINE ($(wc -l < "$BASELINE") entries)."
  exit 0
fi

if [ ! -f "$BASELINE" ]; then
  echo "Baseline $BASELINE missing. Generate it first:"
  echo "  $0 --update"
  exit 2
fi

new="$(comm -13 "$BASELINE" "$tmp_current")"
stale="$(comm -23 "$BASELINE" "$tmp_current")"

rc=0
if [ -n "$new" ]; then
  echo "FAIL: deprecated terms appeared in new places (not in baseline):"
  echo "$new" | sed 's/^/  /'
  echo ""
  echo "If these are legitimate (compatibility/historical/proposal context),"
  echo "review and refresh the baseline:"
  echo "  $0 --update"
  rc=1
fi
if [ -n "$stale" ]; then
  echo "NOTE: baseline has stale entries (terms cleaned up); refresh with:"
  echo "  $0 --update"
fi

if [ "$rc" -eq 0 ]; then
  echo "OK: no deprecated-term regressions ($(wc -l < "$tmp_current") baseline hits)."
fi
exit $rc
