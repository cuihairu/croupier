#!/usr/bin/env python3
"""Fix build tag placement in Go test files."""

import os
import re

files_to_fix = [
    "protocol_transport_test.go",
    "realworld_usage_test.go",
    "retry_mechanisms_test.go",
    "schema_operations_test.go",
    "security_error_recovery_test.go",
    "specialized_cases_test.go",
    "type_system_test.go",
    "types_extended_test.go",
]

os.chdir("D:/croupier/croupier-sdk-go/pkg/croupier")

for filename in files_to_fix:
    filepath = filename
    if not os.path.exists(filepath):
        print(f"File not found: {filepath}")
        continue

    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    # Fix pattern: move build tags before package declaration
    # Pattern 1: build tags after "package croupier"
    pattern1 = r'(// Copyright.*?\n)+(// Licensed under.*?\n)+package croupier\n(//go:build integration\n// \+build integration\n)\n+import'
    replacement1 = r'\1\2\3\5\n\npackage croupier'

    # Pattern 2: build tags after "package croupier" (no extra blank lines)
    pattern2 = r'package croupier\n(//go:build integration\n// \+build integration)\n+import'
    replacement2 = r'//go:build integration\n// +build integration\n\npackage croupier\n\nimport'

    if re.search(pattern1, content):
        content = re.sub(pattern1, replacement1, content, flags=re.DOTALL)
        print(f"Fixed pattern 1: {filename}")
    elif re.search(pattern2, content):
        content = re.sub(pattern2, replacement2, content, flags=re.DOTALL)
        print(f"Fixed pattern 2: {filename}")
    else:
        print(f"No fix needed (or unknown pattern): {filename}")

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)

print("\nDone!")
