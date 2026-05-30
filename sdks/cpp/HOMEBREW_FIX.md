# macOS Homebrew 冲突解决方案

## 问题描述

在 macOS 上使用 vcpkg 构建 Croupier SDK 时,会遇到 Homebrew 安装的 abseil 头文件与 vcpkg 版本冲突的问题:

```
error: "C++ versions less than C++17 are not supported."
```

这是因为 Apple Clang 会将 `/usr/local/include` 注入到头文件搜索路径的**最前面**,导致编译器优先找到 Homebrew 安装的 abseil,而不是 vcpkg 安装的版本。

## 解决方案

### 方案 1: 临时禁用 Homebrew 头文件 (推荐)

在构建时临时重命名 Homebrew 的 abseil 头文件目录:

```bash
sudo mv /usr/local/include/absl /usr/local/include/absl.disabled
./scripts/build.sh --clean
# 构建完成后恢复
sudo mv /usr/local/include/absl.disabled /usr/local/include/absl
```

### 方案 2: 使用 Docker 容器构建

在干净的 Linux 环境中构建,避免 Homebrew 干扰:

```bash
docker run -it --rm -v $(pwd):/workspace -w /workspace \
  ubuntu:22.06 bash -c "
    apt-get update &&
    apt-get install -y cmake git ninja-build &&
    ./scripts/build.sh --clean
  "
```

### 方案 3: 卸载 Homebrew 的 abseil

如果你不需要 Homebrew 的 abseil,可以直接卸载:

```bash
brew uninstall abseil
```

## 根本原因

- **Apple Clang 的行为**: 即使通过 `-isystem` 或 CMake 的 `include_directories` 添加其他路径,Apple Clang 也会在**最前面**插入 `/usr/local/include`
- **vcpkg 的限制**: vcpkg 的 port 构建系统无法通过常规 CMake 变量覆盖编译器的默认搜索路径
- **Homebrew abseil 版本**: Homebrew 安装的 abseil 可能是不同版本,并要求 C++17

## 长期解决方案

1. **优先使用 vcpkg 管理所有依赖**,尽量避免 Homebrew 和 vcpkg 混用
2. **使用 GitHub Actions 自动构建**: 项目已配置 CI,可以直接下载预构建的二进制包
3. **考虑使用 Conan 或其他包管理器**: 它们在处理这类冲突时可能更灵活

## 相关链接

- [vcpkg macOS 已知问题](https://learn.microsoft.com/vcpkg/contributing/maintainer-guide#known-issues)
- [Homebrew 与 vcpkg 共存指南](https://github.com/microsoft/vcpkg/discussions/29936)
