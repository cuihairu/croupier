#!/bin/bash
# Script to add integration build tags to test files

files=(
  "utilities_test.go"
  "type_system_test.go"
  "types_test.go"
  "types_extended_test.go"
  "specialized_cases_test.go"
  "security_error_recovery_test.go"
  "schema_operations_test.go"
  "retry_mechanisms_test.go"
  "realworld_usage_test.go"
  "protocol_transport_test.go"
  "monitoring_logging_test.go"
  "memory_management_test.go"
)

cd "$(dirname "$0")/pkg/croupier"

for file in "${files[@]}"; do
  if [ -f "$file" ]; then
    # Check if file already has integration build tag
    if ! head -5 "$file" | grep -q "^//go:build integration"; then
      # Add integration build tag after copyright header
      sed -i '4a\\n//go:build integration\n// +build integration\n' "$file"
      echo "Updated: $file"
    else
      echo "Skipped (already has tag): $file"
    fi
  else
    echo "Not found: $file"
  fi
done
