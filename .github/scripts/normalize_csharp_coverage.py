#!/usr/bin/env python3
"""Normalize C# Cobertura paths so Codecov can match repository files.

Merges all coverlet coverage reports into a single clean Cobertura XML
that only contains source files (no generated code). This avoids:
1. Codecov silently failing on large files with mostly generated code
2. Duplicate source files across multiple coverage reports confusing merging
"""

from __future__ import annotations

import re
from pathlib import Path


REPO_PREFIX = "sdks/csharp/"
REPORT_GLOB = "sdks/csharp/TestResults/*/coverage.cobertura.xml"
OUTPUT_DIR = Path("sdks/csharp/TestResults/codecov")
OUTPUT_FILE = OUTPUT_DIR / "coverage.xml"

# Files to exclude from the merged report
IGNORE_PATTERNS = ["/generated/", "/obj/"]


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


def should_ignore(filename: str) -> bool:
    for pat in IGNORE_PATTERNS:
        if pat in filename:
            return True
    if not filename.startswith(REPO_PREFIX):
        return True
    return False


def merge_reports(reports: list[Path], target: Path) -> None:
    """Merge multiple Cobertura XML reports into one clean report."""
    # {filename: [(class_name, line_rate, branch_rate, complexity, lines_xml), ...]}
    file_classes: dict[str, list[tuple[str, float, float, int, str]]] = {}

    for report in reports:
        text = report.read_text(encoding="utf-8")
        source_match = re.search(r"<source>(.*?)</source>", text)
        source = source_match.group(1) if source_match else ""

        class_pattern = re.compile(
            r'<class\s+name="([^"]+)"\s+filename="([^"]+)"\s+'
            r'line-rate="([^"]+)"\s+branch-rate="([^"]+)"\s+complexity="([^"]+)"'
            r'(.*?)(?=</class>)',
            re.DOTALL,
        )

        for m in class_pattern.finditer(text):
            class_name = m.group(1)
            raw_filename = m.group(2)
            line_rate = float(m.group(3))
            branch_rate = float(m.group(4))
            complexity = int(m.group(5))
            class_body = m.group(6)

            norm_fn = normalize_filename(raw_filename, source)
            if should_ignore(norm_fn):
                continue

            lines_section = re.search(r'<lines>(.*?)</lines>', class_body, re.DOTALL)
            lines_xml = lines_section.group(0) if lines_section else "<lines/>"

            if norm_fn not in file_classes:
                file_classes[norm_fn] = []
            file_classes[norm_fn].append((class_name, line_rate, branch_rate, complexity, lines_xml))

    if not file_classes:
        raise SystemExit("no source files found after filtering")

    # Build merged XML
    classes_xml_parts = []
    total_lines_covered = 0
    total_lines_valid = 0
    total_branches_covered = 0
    total_branches_valid = 0
    total_complexity = 0

    for norm_fn in sorted(file_classes.keys()):
        classes = file_classes[norm_fn]
        for class_name, lr, br, cx, lines_xml in classes:
            total_complexity += cx
            # Count lines
            line_hits = re.findall(r'number="\d+"\s+hits="(\d+)"', lines_xml)
            for h in line_hits:
                total_lines_valid += 1
                if int(h) > 0:
                    total_lines_covered += 1
            # Count branches
            conditions = re.findall(r'coverage="(\d+)%"', lines_xml)
            total_branches_valid += len(conditions)
            for cov in conditions:
                if int(cov) > 0:
                    total_branches_covered += 1

            classes_xml_parts.append(
                f'        <class name="{class_name}" filename="{norm_fn}" '
                f'line-rate="{lr}" branch-rate="{br}" complexity="{cx}">'
                f'{lines_xml}'
                f'</class>'
            )

    line_rate = total_lines_covered / total_lines_valid if total_lines_valid else 0
    branch_rate = total_branches_covered / total_branches_valid if total_branches_valid else 0

    classes_xml = "\n".join(classes_xml_parts)

    xml_content = f'''<?xml version="1.0" encoding="utf-8"?>
<coverage line-rate="{line_rate:.4f}" branch-rate="{branch_rate:.4f}" version="1.9" timestamp="0" lines-covered="{total_lines_covered}" lines-valid="{total_lines_valid}" branches-covered="{total_branches_covered}" branches-valid="{total_branches_valid}">
  <sources>
    <source>.</source>
  </sources>
  <packages>
    <package name="Croupier.Sdk" line-rate="{line_rate:.4f}" branch-rate="{branch_rate:.4f}" complexity="{total_complexity}">
      <classes>
{classes_xml}
      </classes>
    </package>
  </packages>
</coverage>
'''
    target.write_text(xml_content, encoding="utf-8")


def main() -> None:
    reports = sorted(Path(".").glob(REPORT_GLOB))
    if not reports:
        raise SystemExit("no C# Cobertura coverage reports found")

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    for stale in OUTPUT_DIR.glob("coverage*.xml"):
        stale.unlink()

    target = OUTPUT_DIR / "coverage.xml"
    merge_reports(reports, target)
    print(target)


if __name__ == "__main__":
    main()
