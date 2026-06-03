# Croupier SDKs

本目录包含 Croupier 的官方 SDK，作为 monorepo 的一部分维护。

## 架构

```
sdks/
├── go/          # Go SDK
├── js/          # JavaScript/TypeScript SDK
├── python/      # Python SDK
├── java/        # Java SDK
├── csharp/      # C# SDK
├── cpp/         # C++ SDK
└── README.md    # 本文件
```

## 协议共享

所有 SDK 共享根目录的 protobuf 定义：

```
proto/
└── v1/
    ├── invocation.proto    # 函数调用协议
    └── provider.proto      # Provider 注册协议
```

## CI 状态

| SDK | CI | Docs |
| --- | --- | --- |
| [Go](./go/) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml) | [docs](./go/) |
| [JS/TS](./js/) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml) | [docs](./js/) |
| [Python](./python/) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [docs](./python/) |
| [Java](./java/) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml) | [docs](./java/) |
| [C#](./csharp/) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [docs](./csharp/) |
| [C++](./cpp/) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml) | [docs](./cpp/) |

## 规划文档

- [docs/sdk/specification.md](../docs/sdk/specification.md)
- [docs/sdk/missing-features.md](../docs/sdk/missing-features.md)
- [docs/architecture/sdk-wire-protocol.md](../docs/architecture/sdk-wire-protocol.md)
