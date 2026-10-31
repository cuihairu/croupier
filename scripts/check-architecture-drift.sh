#!/usr/bin/env bash
# Check for architecture-doc drift vs a committed baseline.
#
# check-architecture.py validates SVG XML, scoped links, diagram references,
# TODO DAG and completion-evidence shape. It currently reports known
# pre-existing gaps (legacy ledger entries not yet migrated to the
# 完成证据/LOCAL VERIFIED shape). ARCH-020 deliberately does NOT turn on a
# strict full-site gate that would block legacy pages: instead this script
# fails only when NEW errors appear beyond the committed baseline (regression /
# drift), and reports stale baseline entries for refresh.
#
# Run from repo root:
#   ./scripts/check-architecture-drift.sh           # check
#   ./scripts/check-architecture-drift.sh --update  # rewrite baseline after review
# Exit codes:
#   0  no new architecture errors vs baseline
#   1  new architecture errors appeared (drift / regression)
#   2  baseline missing / usage error

set -u

cd "$(dirname "$0")/.." || exit 2
export LC_ALL=C

BASELINE="scripts/docs/architecture-check-baseline.txt"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

python3 scripts/docs/check-architecture.py 2>&1 | sort -u > "$TMP"

if [ "${1:-}" = "--update" ]; then
  cp "$TMP" "$BASELINE"
  echo "Updated $BASELINE ($(wc -l < "$BASELINE") entries)."
  exit 0
fi

if [ ! -f "$BASELINE" ]; then
  echo "Baseline $BASELINE missing. Generate it first:"
  echo "  $0 --update"
  exit 2
fi

new="$(comm -13 "$BASELINE" "$TMP")"
stale="$(comm -23 "$BASELINE" "$TMP")"

rc=0
if [ -n "$new" ]; then
  echo "FAIL: architecture drift / regression (not in baseline):"
  echo "$new" | sed 's/^/  /'
  echo ""
  echo "If these are legitimate (fixed / newly-documented), refresh the baseline:"
  echo "  $0 --update"
  rc=1
fi
if [ -n "$stale" ]; then
  echo "NOTE: baseline has stale entries (gaps closed); refresh with:"
  echo "  $0 --update"
fi

if [ "$rc" -eq 0 ]; then
  echo "OK: no architecture-doc drift ($(wc -l < "$TMP") baseline hits)."
fi
exit $rc
