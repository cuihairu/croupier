# 入门指南

欢迎来到 Croupier C++ SDK！本指南将帮助你快速上手并掌握 SDK 的核心功能。

## 系统要求

### 必需组件

- **C++17** 编译器
  - GCC 8+
  - Clang 10+
  - MSVC 2019+
- **CMake 3.20+**
- **vcpkg**（推荐，用于依赖管理）

### 支持的平台

| 平台 | 架构 | 状态 |
|------|------|------|
| Windows | x64 | ✅ 支持 |
| Linux | x64, ARM64 | ✅ 支持 |
| macOS | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 目录结构

```
croupier-sdk-cpp/
├── include/           # 公共头文件
├── src/               # 源代码
├── examples/          # 示例程序
├── scripts/           # 构建脚本
├── cmake/             # CMake 模块
├── configs/           # 配置文件示例
└── vcpkg.json         # vcpkg 依赖清单
```

## 下一步

- [安装指南](./installation.md) - 详细安装步骤
- [快速开始](./quick-start.md) - 第一个示例程序
- [构建指南](./building.md) - 高级构建选项
- [主线程调度器](./threading.md) - gRPC 回调线程安全处理
