# Fixing Windows RuntimeLibrary Mismatches

A common Windows problem is a runtime library mismatch, e.g. mixing `/MD` with `/MT`.

## Root Causes

- The vcpkg triplet does not match the SDK compile options
- Dependency libraries and the target library use different CRT strategies

## Recommendations

- `x64-windows` maps to the dynamic runtime
- `x64-windows-static` maps to the static runtime
- Keep CMake, CI, and local presets consistent
