# Installation

## Dependencies

- CMake
- Ninja or the platform's native build tool
- vcpkg

## Basic Workflow

```bash
cmake -B build -G Ninja \
  -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
```

## Recommendations

- Prefer a triplet already validated by the repository's CI
- On Windows, explicitly confirm the runtime library matches the triplet
