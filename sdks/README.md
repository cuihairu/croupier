# Croupier SDKs

所有官方 SDK 已整合到 monorepo 的 `sdks/` 目录下统一维护。

## 主项目

| 项目 | 描述 | 链接 |
| --- | --- | --- |
| **Croupier** | 游戏后端平台主项目 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |

## 官方 SDK

| 语言 | 目录 | Docs | Coverage |
| --- | --- | --- | --- |
| Go | `sdks/go/` | [README](sdks/go/README.md) | [![codecov](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=go-sdk)](https://codecov.io/gh/cuihairu/croupier) |
| JS/TS | `sdks/js/` | [README](sdks/js/README.md) | [![codecov](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=js-sdk)](https://codecov.io/gh/cuihairu/croupier) |
| Python | `sdks/python/` | [README](sdks/python/README.md) | [![codecov](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=python-sdk)](https://codecov.io/gh/cuihairu/croupier) |
| Java | `sdks/java/` | [README](sdks/java/README.md) | [![codecov](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=java-sdk)](https://codecov.io/gh/cuihairu/croupier) |
| C# | `sdks/csharp/` | [README](sdks/csharp/README.md) | [![codecov](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=csharp-sdk)](https://codecov.io/gh/cuihairu/croupier) |
| C++ | `sdks/cpp/` | [README](sdks/cpp/README.md) | [![codecov](https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=cpp-sdk)](https://codecov.io/gh/cuihairu/croupier) |
| Lua | `sdks/cpp/skynet/` | [README](sdks/cpp/skynet/README.md) | - |

## 仓库内规划文档

- [docs/sdk/specification.md](../docs/sdk/specification.md) - SDK 规范
- [docs/sdk/missing-features.md](../docs/sdk/missing-features.md) - 待实现功能
- [docs/sdk/go-checklist.md](../docs/sdk/go-checklist.md) - Go SDK 检查清单
- [docs/sdk/python-checklist.md](../docs/sdk/python-checklist.md) - Python SDK 检查清单
- [docs/sdk/js-ts-checklist.md](../docs/sdk/js-ts-checklist.md) - JS/TS SDK 检查清单
- [docs/sdk/csharp-checklist.md](../docs/sdk/csharp-checklist.md) - C# SDK 检查清单
- [docs/sdk/java-checklist.md](../docs/sdk/java-checklist.md) - Java SDK 检查清单
- [docs/sdk/cpp-checklist.md](../docs/sdk/cpp-checklist.md) - C++ SDK 检查清单
- [docs/architecture/sdk-wire-protocol.md](../docs/architecture/sdk-wire-protocol.md) - SDK 线协议
- [docs/architecture/sdk-agent-transport-redesign.md](../docs/architecture/sdk-agent-transport-redesign.md) - SDK-Agent 传输层设计
- [docs/sdks/sdk-parity-matrix.md](../docs/sdks/sdk-parity-matrix.md) - SDK 功能矩阵
