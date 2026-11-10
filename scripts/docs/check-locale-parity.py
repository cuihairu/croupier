#!/usr/bin/env python3
"""Locale parity gate (CR-FIX-17): the published zh tree must mirror the
published EN tree page-for-page, and the zh sidebar in config.mjs must carry
a /zh counterpart for every EN sidebar link. Excludes mirror the VitePress
srcExclude (archive/**, review/**, zh/review/**). Pure stdlib, seconds."""

import re
import sys
from pathlib import Path

R = Path(__file__).resolve().parents[2]
DOCS = R / "docs"
errors = []


def route(p: Path, strip_prefix: str) -> str:
    rel = str(p.relative_to(strip_prefix))[:-3]  # strip .md
    if rel.endswith("index"):
        rel = rel[:-5]
    return rel.rstrip("/")


# 1) Page-set parity ---------------------------------------------------------
excl = ("archive", "review")
SKIP_PARTS = {"node_modules", "dist", "cache"}


def published(p: Path, root: Path) -> bool:
    rel = p.relative_to(root)
    if rel.parts[0] in excl:
        return False
    return not any(
        part in SKIP_PARTS or part.startswith(".") for part in rel.parts
    )


en_pages = {
    route(p, DOCS)
    for p in (DOCS).glob("**/*.md")
    if published(p, DOCS) and p.relative_to(DOCS).parts[0] != "zh"
}
zh_pages = {
    route(p, DOCS / "zh")
    for p in (DOCS / "zh").glob("**/*.md")
    if published(p, DOCS / "zh")
}
en_only = sorted(en_pages - zh_pages)
zh_only = sorted(zh_pages - en_pages)
for r in en_only:
    errors.append(f"locale parity: EN page without zh counterpart: {r}")
for r in zh_only:
    errors.append(f"locale parity: zh page without EN counterpart: {r}")

# 2) Sidebar mirror ----------------------------------------------------------
cfg = (DOCS / ".vitepress" / "config.mjs").read_text(encoding="utf-8")
links = re.findall(r'link: "(/[^"]+)"', cfg)
en_links = [x for x in links if not x.startswith("/zh/")]
zh_links = {x for x in links if x.startswith("/zh/")}

for link in en_links:
    counterpart = "/zh" + link
    if counterpart not in zh_links:
        errors.append(f"locale parity: sidebar EN link {link} has no zh counterpart")

# Every sidebar link must resolve inside its locale tree.
def resolves(link: str, base: Path) -> bool:
    rel = link[len("/zh/"):] if link.startswith("/zh/") else link[1:]
    rel = rel.rstrip("/")
    if not rel:
        return (base / "index.md").exists()
    for cand in (base / (rel + ".md"), base / rel / "index.md"):
        if cand.exists():
            return True
    return False


for link in en_links:
    if not resolves(link, DOCS):
        errors.append(f"locale parity: sidebar link does not resolve: {link}")
for link in sorted(zh_links):
    if not resolves(link, DOCS / "zh"):
        errors.append(f"locale parity: sidebar zh link does not resolve: {link}")

if errors:
    print("\n".join(errors))
    print(f"locale parity check FAILED ({len(errors)} problem(s))")
    sys.exit(1)
print(
    f"locale parity check PASSED "
    f"({len(en_pages)} EN pages mirrored, {len(en_links)} EN sidebar links)"
)
