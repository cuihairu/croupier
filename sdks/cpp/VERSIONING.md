# Version Management

This document explains how versioning works in the Croupier C++ SDK project.

## Version Source of Truth

**The canonical version is defined in [`VERSION.cmake`](VERSION.cmake)**.

All other places (CMake, vcpkg, CI, documentation) derive their version from this file.

## Current Version

```
0.1.0
```

## Version Format

We follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** (0): Incompatible API changes
- **MINOR** (1): Backwards-compatible functionality additions
- **PATCH** (0): Backwards-compatible bug fixes

## How to Update Version

### 1. Update VERSION.cmake

Edit [`VERSION.cmake`](VERSION.cmake):

```cmake
set(CROUPIER_SDK_VERSION_MAJOR 0)
set(CROUPIER_SDK_VERSION_MINOR 1)
set(CROUPIER_SDK_VERSION_PATCH 0)
```

### 2. Update vcpkg.json

The version in [`vcpkg.json`](vcpkg.json) should match VERSION.cmake:

```json
{
  "name": "croupier-cpp-sdk",
  "version-string": "0.1.0",
  ...
}
```

### 3. Create Git Tag (for releases)

For official releases, create and push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This will trigger the release workflow and create a GitHub Release.

## Version Components

### VERSION.cmake Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `CROUPIER_SDK_VERSION_MAJOR` | Major version number | `0` |
| `CROUPIER_SDK_VERSION_MINOR` | Minor version number | `1` |
| `CROUPIER_SDK_VERSION_PATCH` | Patch version number | `0` |
| `CROUPIER_SDK_VERSION` | Full version string | `0.1.0` |
| `CROUPIER_SDK_ABI_VERSION` | ABI version for soname | `0.1` |

### CMake Integration

The version is used in:

```cmake
project(croupier-cpp-sdk
    VERSION ${CROUPIER_SDK_VERSION}
    ...
)
```

This sets:
- `PROJECT_VERSION` → `0.1.0`
- `PROJECT_VERSION_MAJOR` → `0`
- `PROJECT_VERSION_MINOR` → `1`
- `PROJECT_VERSION_PATCH` → `0`

## Build Artifacts

### Nightly Builds

Triggered on:
- Every push to `main` branch
- Daily schedule (UTC 02:00)
- Manual workflow dispatch

Version format: `0.1.0-nightly.YYYYMMDD.HHMM`

Example: `0.1.0-nightly.20250103.1430`

### Official Releases

Triggered on:
- Git tag push matching `v*` pattern (e.g., `v0.1.0`, `v1.2.3`)

Version format: Uses tag version (e.g., `0.1.0`)

## Release Workflow

### Making a New Release

1. **Update version numbers**:
   ```bash
   # Edit VERSION.cmake
   vim VERSION.cmake
   # Edit vcpkg.json
   vim vcpkg.json
   ```

2. **Commit changes**:
   ```bash
   git add VERSION.cmake vcpkg.json
   git commit -m "chore: bump version to 0.2.0"
   ```

3. **Create and push tag**:
   ```bash
   git tag v0.2.0
   git push origin main
   git push origin v0.2.0
   ```

4. **GitHub Actions will**:
   - Build for all platforms (Windows x64/x86, Linux x64/arm64, macOS x64/arm64)
   - Create GitHub Release with artifacts
   - Mark as stable (non-pre-release)

### Nightly Builds

No manual action required. Nightly builds are automatically created:
- On every push to `main`
- Daily at UTC 02:00
- Marked as pre-release

## Version Compatibility

### API Stability

- **0.x.x**: Development phase, API may change
- **1.x.x**: Stable API, backwards-compatible changes only

### ABI Compatibility

The ABI version (`MAJOR.MINOR`) is used for:
- Shared library soname (Linux: `libcroupier-sdk.so.0.1`)
- macOS library versioning (macOS: `libcroupier-sdk.0.1.dylib`)

When MINOR version changes, ABI may change:
- Rebuild dependent applications
- Update linker flags

## Checking Version

### From Build

```bash
cmake --build build --target help
grep VERSION build/CMakeCache.txt
```

### From Installed Library

```bash
# Linux
objdump -p /usr/local/lib/libcroupier-sdk.so.0.1 | grep SONAME

# macOS
otool -L /usr/local/lib/libcroupier-sdk.0.1.dylib
```

### From Source

```cpp
#include <croupier/sdk/config.h>  // If generated
// Or check VERSION.cmake directly
```

## Examples

### Patch Release (Bug Fix)

```cmake
# VERSION.cmake
set(CROUPIER_SDK_VERSION_MAJOR 0)
set(CROUPIER_SDK_VERSION_MINOR 1)
set(CROUPIER_SDK_VERSION_PATCH 1)  # 0.1.0 → 0.1.1
```

### Minor Release (New Feature)

```cmake
# VERSION.cmake
set(CROUPIER_SDK_VERSION_MAJOR 0)
set(CROUPIER_SDK_VERSION_MINOR 2)  # 0.1.x → 0.2.0
set(CROUPIER_SDK_VERSION_PATCH 0)
```

### Major Release (Breaking Change)

```cmake
# VERSION.cmake
set(CROUPIER_SDK_VERSION_MAJOR 1)  # 0.x.x → 1.0.0
set(CROUPIER_SDK_VERSION_MINOR 0)
set(CROUPIER_SDK_VERSION_PATCH 0)
```

## References

- [Semantic Versioning 2.0.0](https://semver.org/)
- [CMake version documentation](https://cmake.org/cmake/help/latest/variable/PROJECT_VERSION.html)
- [vcpkg versioning](https://learn.microsoft.com/en-us/vcpkg/users/versioning)
