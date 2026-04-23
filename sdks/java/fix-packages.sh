#!/bin/bash

# Fix package names in generated files
echo "Fixing package names in generated files..."

# Fix java files
find build/generated -name "*.java" -type f -exec sed -i '' 's/^package croupier\./package com.croupier\./g' {} \;

# Fix import statements
find build/generated -name "*.java" -type f -exec sed -i '' 's/import croupier\./import com.croupier\./g' {} \;

# Fix class references
find build/generated -name "*.java" -type f -exec sed -i '' 's/croupier\.\([a-z]\)/com.croupier.\1/g' {} \;

echo "Package names fixed!"