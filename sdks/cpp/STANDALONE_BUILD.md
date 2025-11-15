# Croupier C++ SDK - 独立构建指南

> **核心答案：是的！C++ SDK 现在支持完全独立构建，无需克隆父项目。**

## 🎯 构建模式对比

| 构建模式 | 是否需要父项目 | 网络需求 | 构建速度 | 推荐场景 |
|---------|-------------|---------|---------|----------|
| **📦 预构建模式** | ❌ 否 | ❌ 无 | ⚡ 最快 | **生产构建（推荐）** |
| **🌐 在线模式** | ❌ 否 | ✅ 需要 | 🐌 中等 | 开发测试 |
| **🔄 Mock 模式** | ❌ 否 | ❌ 无 | ⚡ 最快 | 快速原型 |
| **🏠 本地模式** | ✅ 是 | ❌ 无 | 🚀 快 | 父项目开发 |

## 🚀 快速开始（推荐方式）

### 1. 克隆 SDK（无需父项目）

```bash
git clone https://github.com/cuihairu/croupier-sdk-cpp.git
cd croupier-sdk-cpp
```

### 2. 选择构建模式

#### 📦 预构建模式（推荐生产使用）
```bash
# 使用预提交的 protobuf 生成文件
./standalone-build.sh configure --with-grpc
./standalone-build.sh build
```

#### 🌐 在线模式（自动下载 proto 文件）
```bash
# 自动从 GitHub 下载最新 proto 文件并生成代码
./standalone-build.sh configure --with-grpc --online
./standalone-build.sh build
```

#### 🔄 Mock 模式（快速原型开发）
```bash
# 使用 gRPC mock 实现，无需 protobuf 文件
./standalone-build.sh configure --mock-grpc
./standalone-build.sh build
```

## 📋 详细构建选项

### 使用构建脚本（推荐）

```bash
# 查看所有选项
./standalone-build.sh --help

# 常用构建命令
./standalone-build.sh configure --system-libs  # 使用系统库（Linux/macOS）
./standalone-build.sh configure --vcpkg        # 使用 vcpkg（Windows）
./standalone-build.sh build                    # 编译项目
./standalone-build.sh test                     # 运行测试
./standalone-build.sh clean                    # 清理构建文件
```

### 直接使用 CMake

```bash
# 预构建模式
cmake -B build -DCROUPIER_STANDALONE_BUILD=ON -DCROUPIER_PREBUILT_PROTO=ON -DENABLE_GRPC=ON

# 在线模式
cmake -B build -DCROUPIER_STANDALONE_BUILD=ON -DCROUPIER_ONLINE_BUILD=ON -DENABLE_GRPC=ON

# Mock 模式
cmake -B build -DCROUPIER_STANDALONE_BUILD=ON -DENABLE_GRPC=OFF

cmake --build build
```

## 🏗️ 技术实现细节

### 问题与解决方案

#### 原始问题：
1. **Protobuf 依赖**：SDK 需要父项目的 `proto/` 目录
2. **生成文件缺失**：`gen/` 目录被 gitignore
3. **构建脚本依赖**：CMake 假设父项目结构

#### 我们的解决方案：

#### 1. 智能模式检测
```cpp
// CMake 自动检测最优构建模式
if(预构建文件存在)
    使用预构建模式  // 最快，无网络需求
else if(网络可用 && protoc可用)
    使用在线模式    // 自动下载+生成
else
    使用Mock模式   // 无依赖开发
```

#### 2. 预构建文件策略
- **CI 自动生成**：GitHub Actions 定期生成最新的 protobuf 代码
- **版本控制提交**：将生成文件提交到 SDK 仓库
- **智能缓存**：避免重复生成

#### 3. 在线下载机制
```bash
# 自动下载策略
proto_files=(
    "croupier/agent/local/v1/local.proto"
    "croupier/control/v1/control.proto"
    "croupier/function/v1/function.proto"
    # ... 其他文件
)

# 从 GitHub 下载最新版本
base_url="https://raw.githubusercontent.com/cuihairu/croupier/main/proto"
```

## 🔧 高级配置

### 依赖管理

#### 系统包（Linux/macOS 推荐）
```bash
# Ubuntu/Debian
sudo apt-get install libgrpc++-dev libprotobuf-dev nlohmann-json3-dev

# macOS
brew install grpc protobuf nlohmann-json

# 构建
./standalone-build.sh configure --system-libs
```

#### vcpkg（Windows 推荐）
```bash
# 设置 vcpkg
export VCPKG_ROOT=/path/to/vcpkg

# 构建
./standalone-build.sh configure --vcpkg
```

### 自定义配置

```bash
# 指定构建和安装目录
./standalone-build.sh configure --build-dir custom_build --install-dir /usr/local

# Debug 构建
./standalone-build.sh configure --debug

# 仅构建静态库
cmake -B build -DBUILD_SHARED_LIBS=OFF -DBUILD_STATIC_LIBS=ON
```

## 📦 CI/CD 集成

### GitHub Actions 示例

```yaml
name: C++ SDK Build
on: [push, pull_request]
jobs:
  build:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - name: Build SDK
        run: |
          ./standalone-build.sh configure --with-grpc
          ./standalone-build.sh build
          ./standalone-build.sh test
```

### Docker 构建

```dockerfile
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y \
    build-essential cmake ninja-build \
    libgrpc++-dev libprotobuf-dev nlohmann-json3-dev

COPY . /sdk
WORKDIR /sdk
RUN ./standalone-build.sh build --system-libs
```

## 🎯 最佳实践建议

### 1. 生产环境
- ✅ 使用**预构建模式**：最稳定，构建速度最快
- ✅ 启用 **Release 构建**：`--release`
- ✅ 使用**静态链接**：方便部署

### 2. 开发环境
- ✅ 使用**在线模式**：获取最新 proto 定义
- ✅ 启用 **Debug 构建**：`--debug`
- ✅ 使用**动态链接**：开发调试方便

### 3. 原型开发
- ✅ 使用 **Mock 模式**：无依赖，启动最快
- ✅ 专注业务逻辑，后续切换到真实实现

## 🆚 与其他 SDK 语言对比

| 语言 | 独立构建 | 预构建文件 | 在线模式 | Mock 模式 |
|------|----------|------------|----------|-----------|
| **C++** | ✅ 是 | ✅ 支持 | ✅ 支持 | ✅ 支持 |
| Go | ✅ 是 | ❌ 否 | ✅ 支持 | ✅ 支持 |
| Java | 🚧 部分 | ❌ 否 | ✅ 支持 | ✅ 支持 |
| Python | 🚧 部分 | ❌ 否 | ✅ 支持 | ✅ 支持 |
| JavaScript | 🚧 部分 | ❌ 否 | ✅ 支持 | ✅ 支持 |

## 🔍 故障排除

### 常见问题

#### Q: 编译时找不到 proto 文件
```bash
# 解决方案：使用在线模式重新下载
./standalone-build.sh clean
./standalone-build.sh configure --with-grpc --online
```

#### Q: vcpkg 依赖安装失败
```bash
# 解决方案：使用系统包
sudo apt-get install libgrpc++-dev  # Linux
brew install grpc                   # macOS
./standalone-build.sh configure --system-libs
```

#### Q: 网络连接失败
```bash
# 解决方案：使用 Mock 模式进行开发
./standalone-build.sh configure --mock-grpc
```

### 调试技巧

```bash
# 查看详细构建信息
cmake -B build -DCMAKE_VERBOSE_MAKEFILE=ON -DCROUPIER_STANDALONE_BUILD=ON

# 检查生成文件状态
find generated -name "*.cc" | wc -l  # 应该 > 0

# 验证依赖
pkg-config --exists grpc++  # 检查 gRPC 安装
```

## 🎉 总结

**回答你的问题：不需要！** C++ SDK 现在支持三种独立构建模式：

1. **📦 预构建模式**：零依赖，最快构建
2. **🌐 在线模式**：自动下载最新定义
3. **🔄 Mock 模式**：快速原型开发

选择适合你的模式，享受独立构建的便利！

---

需要帮助？查看 [完整文档](./COMPLETE_SDK_README.md) 或提交 [Issue](https://github.com/cuihairu/croupier-sdk-cpp/issues)。