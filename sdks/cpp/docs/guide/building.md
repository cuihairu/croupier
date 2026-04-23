# 构建指南

本指南详细介绍了各种构建选项和配置。

## 构建脚本

### Linux/macOS: `build.sh`

```bash
./scripts/build.sh [OPTIONS]

选项:
  --clean          清理构建目录后重新构建
  --tests ON/OFF   启用/禁用测试 (默认: OFF)
  --examples ON/OFF 启用/禁用示例 (默认: ON)
  --help           显示帮助信息
```

### Windows: `build.ps1`

```powershell
.\scripts\build.ps1 [OPTIONS]

选项:
  -BuildType <Debug|Release>  构建类型 (默认: Release)
  -Clean                     清理构建目录
  -Tests <ON|OFF>            启用/禁用测试
  -Examples <ON|OFF>         启用/禁用示例
```

## CMake 选项

### 基础选项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `CMAKE_BUILD_TYPE` | `Release` | 构建类型 |
| `BUILD_TESTING` | `OFF` | 构建测试 |
| `BUILD_EXAMPLES` | `ON` | 构建示例 |

### vcpkg 相关

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `CROUPIER_BOOTSTRAP_VCPKG` | `ON` | 自动下载 vcpkg |
| `VCPKG_OVERLAY_PORTS` | `./vcpkg-overlays` | vcpkg overlay ports |
| `VCPKG_OVERLAY_TRIPLETS` | `./vcpkg-overlays/triplets` | vcpkg overlay triplets |

### 示例配置

```bash
cmake -B build \
  -DCMAKE_BUILD_TYPE=Debug \
  -DBUILD_TESTING=ON \
  -DBUILD_EXAMPLES=ON \
  -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake
```

## CMake Presets

项目提供了跨平台的 CMake Presets：

### 可用 Presets

**macOS:**
- `macos-arm64-debug-vcpkg`
- `macos-arm64-release-vcpkg`
- `macos-x64-debug-vcpkg`
- `macos-x64-release-vcpkg`

**Linux:**
- `linux-x64-debug-vcpkg`
- `linux-x64-release-vcpkg`

**Windows:**
- `windows-x64-debug-vcpkg`
- `windows-x64-release-vcpkg`

### 使用 Presets

```bash
# 列出所有预设
cmake --list-presets

# 使用预设配置
cmake --preset linux-x64-release-vcpkg

# 构建
cmake --build build --config Release
```

## 自定义构建

### 禁用自动下载 vcpkg

```bash
cmake -B build \
  -DCROUPIER_BOOTSTRAP_VCPKG=OFF \
  -DCMAKE_TOOLCHAIN_FILE=/path/to/your/vcpkg/scripts/buildsystems/vcpkg.cmake
```

### 使用全局 vcpkg

```bash
cmake -B build \
  -DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake
```

### 指定安装前缀

```bash
cmake -B build -DCMAKE_INSTALL_PREFIX=/usr/local
cmake --install build
```

## 编译器特定选项

### GCC

```bash
cmake -B build \
  -DCMAKE_CXX_COMPILER=g++ \
  -DCMAKE_CXX_FLAGS="-Wall -Wextra"
```

### Clang

```bash
cmake -B build \
  -DCMAKE_CXX_COMPILER=clang++ \
  -DCMAKE_CXX_FLAGS="-Wall -Wextra"
```

### MSVC (Visual Studio)

```powershell
cmake -B build `
  -G "Visual Studio 17 2022" `
  -A x64
```

## 性能优化

### Release 构建优化

```bash
cmake -B build \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INTERPROCEDURAL_OPTIMIZATION=ON
```

### 特定架构优化

```bash
# 本机架构优化
cmake -B build -DCMAKE_CXX_FLAGS="-march=native"

# ARM64 NEON 优化
cmake -B build -DCMAKE_CXX_FLAGS="-mfpu=neon"
```

## 静态分析

### 使用 clang-tidy

```bash
cmake -B build \
  -DCMAKE_CXX_CLANG_TIDY="clang-tidy;-checks=*"
```

### 使用 cppcheck

```bash
cmake -B build \
  -DCMAKE_CXX_CPPCHECK="cppcheck;--enable=all"
```

## 调试构建

### 启用调试符号

```bash
cmake -B build \
  -DCMAKE_BUILD_TYPE=Debug \
  -DCMAKE_CXX_FLAGS_DEBUG="-g -O0"
```

### 启用地址清理器 (ASAN)

```bash
cmake -B build \
  -DCMAKE_CXX_FLAGS="-fsanitize=address -fno-omit-frame-pointer" \
  -DCMAKE_EXE_LINKER_FLAGS="-fsanitize=address"
```

## 构建产物

构建完成后，产物位于：

| 内容 | 路径 |
|------|------|
| 可执行文件 | `build/bin/` |
| 库文件 | `build/lib/` |
| 头文件 | `include/` |

## 故障排除

### 构建失败

```bash
# 清理后重试
rm -rf build
./scripts/build.sh --clean
```

### vcpkg 依赖安装失败

```bash
# 单独安装依赖
cd vcpkg
./vcpkg install ggrpc grpc cpprestsdk nlohmann-json --triplet=x64-linux
```

### Protobuf 版本冲突

确保使用 vcpkg toolchain：
```bash
cmake -B build -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake
```
