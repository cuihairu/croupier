#!/bin/bash

echo "Updating generated protobuf code..."

# Clean old generated files
rm -rf generated/*

# Copy and fix generated files
mkdir -p generated

# Copy all generated files to generated directory
cp -r build/generated/source/proto/main/java/* generated/ 2>/dev/null || true
cp -r build/generated/source/proto/main/grpc/* generated/ 2>/dev/null || true

# Fix package names in all files
find generated -name "*.java" -type f -exec sed -i '' 's/^package croupier\./package com.croupier\./g' {} \;
find generated -name "*.java" -type f -exec sed -i '' 's/import croupier\./import com.croupier\./g' {} \;
find generated -name "*.java" -type f -exec sed -i '' 's/croupier\.\([a-z]\)/com.croupier.\1/g' {} \;

echo "Generated code updated successfully!"