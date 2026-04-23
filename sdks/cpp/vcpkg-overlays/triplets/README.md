# Croupier SDK vcpkg Triplets 说明

本目录包含 Croupier SDK 的自定义 vcpkg triplet 配置文件。

## Triplets 命名规范

格式：`<arch>-<platform>-<linkage>-<crt>`

- `arch`: 架构 (x64, arm64)
- `platform`: 平台 (windows, linux, osx)
- `linkage`: 链接方式 (static)
- `crt`: CRT 链接方式 (md, mt) - 仅 Windows

## 可用的 Triplets

### Windows x64

| Triplet | 库链接 | CRT | 适用场景 |
|---------|--------|-----|----------|
| `x64-windows-static-md` | 静态 | 动态 (/MD) | **推荐**：大多数 Windows 应用程序 |
| `x64-windows-static-mt` | 静态 | 静态 (/MT) | 独立应用程序，不依赖 msvcp*.dll |

### macOS

| Triplet | 架构 | 适用场景 |
|---------|------|----------|
| `x64-osx` | x64_64 | Intel Mac |
| `arm64-osx` | arm64 | Apple Silicon (M1/M2/M3) |

### Linux

| Triplet | 架构 | 适用场景 |
|---------|------|----------|
| `x64-linux` | x64_64 | 标准 Linux 发行版 |

## Windows 下的 MT vs MD 选择

### MD (动态 CRT) - x64-windows-static-md

**优点：**
- 可执行文件体积小
- 多个模块共享 CRT，内存效率高
- Windows 官方推荐
- 与大多数第三方库兼容

**缺点：**
- 依赖系统安装的 MSVC 运行时
- 部署时需要确保目标机器有正确的运行时

**适用场景：**
- 桌面应用程序
- 与其他 DLL 一起使用
- 大多数商业软件

### MT (静态 CRT) - x64-windows-static-mt

**优点：**
- 完全独立，不依赖 MSVC 运行时 DLL
- 简化部署
- 避免版本冲突

**缺点：**
- 可执行文件体积较大
- 每个模块都有自己的 CRT 副本，内存效率略低
- 跨 DLL 边界传递 CRT 对象（如文件句柄）可能有问题

**适用场景：**
- 独立可执行文件
- 嵌入式系统
- 无法保证目标机器安装了 MSVC 运行时

## 如何选择 Triplet

### Windows

如果你的项目使用 `/MD`（大多数情况）：
```bash
-DVCPKG_TARGET_TRIPLET=x64-windows-static-md
```

如果你的项目使用 `/MT`（需要完全独立）：
```bash
-DVCPKG_TARGET_TRIPLET=x64-windows-static-mt
```

### 检查你的项目设置

**Visual Studio:**
- 项目属性 → C/C++ → 代码生成 → 运行时库
- `多线程 DLL (/MD)` → 使用 `x64-windows-static-md`
- `多线程 (/MT)` → 使用 `x64-windows-static-mt`

**CMake:**
```cmake
# 检查 CMAKE_CXX_FLAGS_RELEASE 中的 /MD 或 /MT
# 或者检查 CMAKE_MSVC_RUNTIME_LIBRARY
```

## 常见问题

### Q: 我的程序链接时报错 "LNK2038: 检测到“RuntimeLibrary”的不匹配项"

**A:** 你的项目和 SDK 使用了不同的 CRT 链接方式。确保：
- 如果项目用 `/MD`，SDK 必须用 `x64-windows-static-md` 构建
- 如果项目用 `/MT`，SDK 必须用 `x64-windows-static-mt` 构建

### Q: 我该选择哪个版本？

**A:** 除非你有特殊需求，否则：
- **Windows**: 使用 `x64-windows-static-md`（默认推荐）
- **macOS**: 根据你的硬件选择 `x64-osx` 或 `arm64-osx`
- **Linux**: 使用 `x64-linux`

## 相关链接

- [vcpkg Triplets 文档](https://learn.microsoft.com/en-us/vcpkg/users/triplets)
- [MSVC 运行时库](https://learn.microsoft.com/en-us/cpp/build/reference/md-mt-ld-use-run-time-library)
