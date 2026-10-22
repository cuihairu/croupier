# Building

## Goals

- Repeatable local development builds
- CI/release environments configured as closely as possible to local ones

## Recommendations

- Use CMake presets or an explicit triplet
- On Windows, confirm the runtime library match first
- Leave dependency management to vcpkg; do not mix multiple dependency sources
