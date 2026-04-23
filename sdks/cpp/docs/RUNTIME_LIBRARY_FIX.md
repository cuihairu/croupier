# Windows CI RuntimeLibrary 不匹配问题修复

## 问题描述

Windows CI 构建失败，出现链接错误：
```
error LNK2038: mismatch detected for 'RuntimeLibrary':
value 'MD_DynamicRelease' doesn't match value 'MT_StaticRelease'
```

**根本原因**：
- vcpkg 的 `x64-windows` triplet 编译的依赖（gRPC、protobuf、absl）使用动态运行时库 (`/MD`)
- Croupier SDK 默认配置使用静态运行时库 (`/MT`)
- 两者不匹配导致链接失败

## 修复方案

### 1. CMakeLists.txt (第 183-192 行)

**修改前**：
```cmake
option(CROUPIER_USE_STATIC_RUNTIME "Use static runtime library (MT/MTd) instead of dynamic (MD/MDd)" ON)
```

**修改后**：
```cmake
# 根据 vcpkg triplet 自动选择匹配的运行时库
if(VCPKG_TARGET_TRIPLET MATCHES "static$")
    # Static triplet (x64-windows-static) 默认使用静态运行时
    option(CROUPIER_USE_STATIC_RUNTIME "Use static runtime library (MT/MTd) instead of dynamic (MD/MDd)" ON)
else()
    # Standard triplet (x64-windows) 默认使用动态运行时
    option(CROUPIER_USE_STATIC_RUNTIME "Use static runtime library (MT/MTd) instead of dynamic (MD/MDd)" OFF)
endif()
```

**效果**：
- 自动根据 vcpkg triplet 选择匹配的运行时库
- `x64-windows` → 动态运行时 (`/MD`)
- `x64-windows-static` → 静态运行时 (`/MT`)
- `x64-windows-static-md` → 动态运行时 (`/MD`)

### 2. CI 配置 (.github/workflows/ci.yml 第 138-151 行)

**修改前**：
```yaml
- name: Setup vcpkg
  uses: lukka/run-vcpkg@v11
  with:
    # ...
    extraCacheKeys: ${{ matrix.runtime }}
  env:
    VCPKG_DEFAULT_TRIPLET: ${{ matrix.runtime == 'static' && 'x64-windows-static' || 'x64-windows-static-md' }}
```

**修改后**：
```yaml
- name: Setup vcpkg
  uses: lukka/run-vcpkg@v11
  with:
    # ...
    triplet: ${{ matrix.runtime == 'static' && 'x64-windows-static' || 'x64-windows-static-md' }}
  env:
    VCPKG_ROOT: '${{ github.workspace }}/vcpkg'
```

**效果**：
- 使用 `triplet` 参数明确指定 vcpkg triplet
- 确保依赖使用正确的运行时库编译

### 3. CMakePresets.json (第 118-169 行)

**修改前**：
```json
{
  "name": "windows-x64-debug-vcpkg",
  "cacheVariables": {
    "VCPKG_TARGET_TRIPLET": "x64-windows"
  }
}
```

**修改后**：
```json
{
  "name": "windows-x64-debug-vcpkg",
  "displayName": "Windows x64 Debug (vcpkg, VS2022, Dynamic Runtime)",
  "cacheVariables": {
    "VCPKG_TARGET_TRIPLET": "x64-windows",
    "CROUPIER_USE_STATIC_RUNTIME": "OFF"
  }
}
```

**效果**：
- 明确配置使用动态运行时
- 与 vcpkg 的 `x64-windows` triplet 保持一致

## 运行时库说明

| Triplet              | 运行时库 | 说明                           |
| -------------------- | -------- | ------------------------------ |
| `x64-windows`        | `/MD`    | 标准 triplet，动态运行时 (DLL)  |
| `x64-windows-static` | `/MT`    | 静态运行时 (静态链接 CRT)      |
| `x64-windows-static-md` | `/MD`  | 静态链接依赖 + 动态运行时      |

## 验证步骤

### 本地验证（Windows）

```powershell
# 清理构建目录
Remove-Item -Recurse -Force build

# 配置项目（标准 triplet）
cmake -B build `
  -G "Visual Studio 17 2022" `
  -A x64 `
  -DCMAKE_BUILD_TYPE=Release `
  -DCMAKE_TOOLCHAIN_FILE="vcpkg/scripts/buildsystems/vcpkg.cmake" `
  -DVCPKG_TARGET_TRIPLET=x64-windows

# 检查配置输出
# 应该看到：Croupier SDK: Using dynamic runtime library (MD/MDd)

# 构建
cmake --build build --config Release --parallel
```

### CI 验证

检查 CI 构建日志，确认：
1. CMake 配置输出显示 "Using dynamic runtime library (MD/MDd)"
2. 链接阶段不再出现 `LNK2038` 错误
3. 所有 4 个 Windows matrix 构建通过：
   - Release + static
   - Release + dynamic
   - Debug + static
   - Debug + dynamic

## 影响范围

### 默认行为变化

**之前**：所有 Windows 构建默认使用静态运行时 (`/MT`)

**现在**：
- 标准 triplet (`x64-windows`) → 动态运行时 (`/MD`)
- Static triplet (`x64-windows-static`) → 静态运行时 (`/MT`)

### 兼容性

✅ **向后兼容**：
- 如果之前使用 `-DCROUPIER_USE_STATIC_RUNTIME=ON` 显式指定，行为不变
- CI 配置已经明确指定运行时类型，不受影响

✅ **vcpkg 集成改进**：
- 自动匹配 vcpkg triplet 的默认运行时
- 减少手动配置需求

## 相关问题

如果遇到其他链接错误，检查：
1. vcpkg 依赖是否完整安装：`vcpkg install --triplet=x64-windows`
2. CMake 缓存是否清理：`rm -rf build`
3. Visual Studio 版本是否匹配（需要 VS2022）

## 修复时间

2025-02-02

## 修复作者

Claude Code (Engineer-Professional Mode)
