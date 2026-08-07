#!/usr/bin/env python3
"""Normalize C# Cobertura paths so Codecov can match repository files."""

from __future__ import annotations

import re
from pathlib import Path


REPO_PREFIX = "sdks/csharp/"
REPORT_GLOB = "sdks/csharp/TestResults/*/coverage.cobertura.xml"
OUTPUT_DIR = Path("sdks/csharp/TestResults/codecov")


def normalize_filename(filename: str, source: str) -> str:
    filename = filename.replace("\\", "/")
    source = source.replace("\\", "/")

    if filename.startswith(REPO_PREFIX):
        return filename
    if filename.startswith(("src/", "generated/", "examples/")):
        return REPO_PREFIX + filename
    if "/sdks/csharp/src/Croupier.Sdk/" in source and not filename.startswith("/"):
        return REPO_PREFIX + "src/Croupier.Sdk/" + filename
    if "/sdks/csharp/" in source and not filename.startswith("/"):
        return REPO_PREFIX + filename
    return filename


def normalize_report(report: Path, target: Path) -> None:
    text = report.read_text(encoding="utf-8")
    source_match = re.search(r"<source>(.*?)</source>", text)
    source = source_match.group(1) if source_match else ""

    text = re.sub(
        r'filename="([^"]+)"',
        lambda match: f'filename="{normalize_filename(match.group(1), source)}"',
        text,
    )
    text = re.sub(r"<source>.*?</source>", "<source>.</source>", text)
    target.write_text(text, encoding="utf-8")


def main() -> None:
    reports = sorted(Path(".").glob(REPORT_GLOB))
    if not reports:
        raise SystemExit("no C# Cobertura coverage reports found")

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    for stale_report in OUTPUT_DIR.glob("coverage-*.cobertura.xml"):
        stale_report.unlink()

    for index, report in enumerate(reports, start=1):
        target = OUTPUT_DIR / f"coverage-{index}.cobertura.xml"
        normalize_report(report, target)
        print(target)


if __name__ == "__main__":
    main()
