# 安装指南

本指南将帮助你在各种平台上安装和构建 Croupier C++ SDK。

## 方法一：使用预构建包（推荐）

### 下载预构建包

访问 [Releases 页面](https://github.com/cuihairu/croupier-sdk-cpp/releases) 下载：

**静态库包：**
- `croupier-cpp-sdk-static-{version}-windows-x64.zip`
- `croupier-cpp-sdk-static-{version}-linux-x64.tar.gz`
- `croupier-cpp-sdk-static-{version}-macos-arm64.tar.gz`

**动态库包：**
- `croupier-cpp-sdk-dynamic-{version}-windows-x64.zip`
- `croupier-cpp-sdk-dynamic-{version}-linux-x64.tar.gz`
- `croupier-cpp-sdk-dynamic-{version}-macos-arm64.tar.gz`

### 使用预构建包

解压后包含：
- 头文件 (`include/`)
- 库文件 (`lib/`)
- 示例程序 (`bin/`)

在你的 CMake 项目中：

```cmake
find_package(croupier-sdk REQUIRED)
target_link_libraries(your-target PRIVATE croupier-sdk::croupier-sdk)
```

## 方法二：从源代码构建

### Linux/macOS

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier-sdk-cpp.git
cd croupier-sdk-cpp

# 基础构建
./scripts/build.sh

# 清理重构建
./scripts/build.sh --clean

# 启用测试
./scripts/build.sh --tests ON
```

### Windows

```powershell
# 克隆仓库
git clone https://github.com/cuihairu/croupier-sdk-cpp.git
cd croupier-sdk-cpp

# 基础构建
.\scripts\build.ps1

# Debug 构建
.\scripts\build.ps1 -BuildType Debug
```

## 方法三：手动 CMake 构建

```bash
# 1. 配置构建
cmake -B build \
  -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \
  -DVCPKG_OVERLAY_PORTS=./vcpkg-overlays \
  -DVCPKG_OVERLAY_TRIPLETS=./vcpkg-overlays/triplets \
  -DCMAKE_BUILD_TYPE=Release

# 2. 构建
cmake --build build --parallel

# 3. 安装（可选）
cmake --install build --prefix /usr/local
```

## 依赖库

SDK 会自动安装以下依赖（通过 vcpkg）：

- **gRPC 1.71.x** - RPC 通信框架
- **Protobuf 4.25.x** - 协议缓冲区（**版本固定以确保 ABI 兼容性**）
- **nlohmann/json 3.12.x** - JSON 处理库

> **⚠️ 重要提示**：Protobuf 版本已固定为 **4.25.x** 系列以确保与 gRPC 1.71.x 的兼容性。请勿擅自升级到 5.x 版本，否则可能导致 ABI 不兼容问题。
>
> 📖 **详细版本管理策略**：查看 [仓库中的 `proto/README.md`](https://github.com/cuihairu/croupier-sdk-cpp/blob/main/proto/README.md) 了解完整的版本固定策略和升级流程。版本在三个层面保持一致：
> - `vcpkg.json` - C++ 编译库版本
> - `proto/buf.yaml` - Buf 依赖管理版本
> - `buf.gen.yaml` - 代码生成插件版本

## VS Code 配置

使用 `CMakePresets.json` 进行配置：

1. 选择 Configure Preset：`CMake: Select Configure Preset`
2. 根据平台选择：
   - macOS：`macos-arm64-*-vcpkg` 或 `macos-x64-*-vcpkg`
   - Linux：`linux-x64-*-vcpkg`
   - Windows：`windows-x64-*-vcpkg`

## 验证安装

```bash
# 运行示例程序
./build/bin/croupier-example

# 如果看到 "Connecting to agent..." 输出，说明安装成功
```

## 故障排除

### Protobuf 版本冲突

如果看到 `incompatible version of Protobuf` 错误：

1. 确保使用 vcpkg toolchain
2. 清理构建目录：`rm -rf build`
3. 重新配置构建

### vcpkg 下载失败

中国大陆用户可设置镜像：

```bash
export VCPKG_DOWNLOADS=https://mirrors.tuna.tsinghua.edu.cn/vcpkg
```

### Windows 上找不到 DLL

确保将以下目录添加到 PATH：
- `build\bin\` （你的构建输出目录）
- `vcpkg\installed\x64-windows\bin\`
