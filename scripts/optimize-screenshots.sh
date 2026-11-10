#!/usr/bin/env bash
# Screenshot asset optimizer (CR-FIX-30): convert docs/screenshots/ui PNG
# captures to WebP and drop the PNG originals.
#
# Run after web/scripts/capture-interface-preview.mjs (which emits PNG).
# Quality 82 keeps UI text crisp; the reference numbers in
# docs/guide/interface-preview.md are desktop 1600x900 @2x captures.
#
# Idempotent: PNGs that no longer exist are skipped; a PNG whose WebP
# already exists is re-encoded from the current PNG (regeneration flow).
set -euo pipefail
cd "$(dirname "$0")/.."

DIR="${1:-docs/screenshots/ui}"
QUALITY="${WEBP_QUALITY:-82}"

if [[ ! -d "$DIR" ]]; then
  echo "FAIL: screenshot dir missing: $DIR"
  exit 1
fi

shopt -s nullglob
pngs=("$DIR"/*.png)
if [[ ${#pngs[@]} -eq 0 ]]; then
  echo "optimize-screenshots: no PNG in $DIR (already converted?)"
  exit 0
fi

total_before=0
total_after=0
for png in "${pngs[@]}"; do
  webp="${png%.png}.webp"
  before=$(stat -c%s "$png")
  python3 - "$png" "$webp" "$QUALITY" <<'PYEOF'
import sys
from PIL import Image

src, dst, quality = sys.argv[1], sys.argv[2], int(sys.argv[3])
img = Image.open(src)
img.save(dst, "WEBP", quality=quality, method=6)
PYEOF
  after=$(stat -c%s "$webp")
  total_before=$((total_before + before))
  total_after=$((total_after + after))
  rm -f "$png"
  printf '%s: %d -> %d bytes (%d%%)\n' "$(basename "$webp")" "$before" "$after" \
    $((after * 100 / before))
done

echo "optimize-screenshots: total ${total_before} -> ${total_after} bytes ($((total_after * 100 / total_before))%)"
