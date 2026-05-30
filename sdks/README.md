# Croupier SDKs

官方 SDK 仓库已独立维护，不再作为子模块跟踪；本目录主要保留聚合入口和规划文档。

## 主项目与协议

| 项目 | 描述 | 链接 |
| --- | --- | --- |
| **Croupier** | 游戏后端平台主项目 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |

## 官方 SDK

| 语言 | 仓库 | Nightly | Release | Docs | Coverage |
| --- | --- | --- | --- | --- | --- |
| Go | [croupier-sdk-go](https://github.com/cuihairu/croupier-sdk-go) | [![nightly](https://github.com/cuihairu/croupier-sdk-go/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-go/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-go)](https://github.com/cuihairu/croupier-sdk-go/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-go/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-go/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-go) |
| JS/TS | [croupier-sdk-js](https://github.com/cuihairu/croupier-sdk-js) | [![nightly](https://github.com/cuihairu/croupier-sdk-js/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-js/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-js)](https://github.com/cuihairu/croupier-sdk-js/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-js/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-js/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-js) |
| Python | [croupier-sdk-python](https://github.com/cuihairu/croupier-sdk-python) | [![nightly](https://github.com/cuihairu/croupier-sdk-python/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-python/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-python)](https://github.com/cuihairu/croupier-sdk-python/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-python/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-python/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-python) |
| Java | [croupier-sdk-java](https://github.com/cuihairu/croupier-sdk-java) | [![nightly](https://github.com/cuihairu/croupier-sdk-java/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-java/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-java)](https://github.com/cuihairu/croupier-sdk-java/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-java/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-java/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-java) |
| C# | [croupier-sdk-csharp](https://github.com/cuihairu/croupier-sdk-csharp) | [![nightly](https://github.com/cuihairu/croupier-sdk-csharp/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-csharp/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-csharp)](https://github.com/cuihairu/croupier-sdk-csharp/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-csharp/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-csharp/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-csharp) |
| C++ | [croupier-sdk-cpp](https://github.com/cuihairu/croupier-sdk-cpp) | [![nightly](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-cpp)](https://github.com/cuihairu/croupier-sdk-cpp/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-cpp/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-cpp/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-cpp) |
| Lua | [croupier-sdk-cpp](https://github.com/cuihairu/croupier-sdk-cpp) | - | - | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-cpp/) | - |

## 仓库内规划文档

- [docs/sdk/specification.md](../docs/sdk/specification.md)
- [docs/sdk/missing-features.md](../docs/sdk/missing-features.md)
- [docs/sdk/go-checklist.md](../docs/sdk/go-checklist.md)
- [docs/sdk/python-checklist.md](../docs/sdk/python-checklist.md)
- [docs/sdk/js-ts-checklist.md](../docs/sdk/js-ts-checklist.md)
- [docs/sdk/csharp-checklist.md](../docs/sdk/csharp-checklist.md)
- [docs/sdk/java-checklist.md](../docs/sdk/java-checklist.md)
- [docs/sdk/cpp-checklist.md](../docs/sdk/cpp-checklist.md)
- [docs/architecture/sdk-wire-protocol.md](../docs/architecture/sdk-wire-protocol.md)
- [docs/architecture/sdk-agent-transport-redesign.md](../docs/architecture/sdk-agent-transport-redesign.md)
