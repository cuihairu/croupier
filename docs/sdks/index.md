---
title: SDK 文档
---

# SDK 文档

本目录是官方 SDK 的站点文档入口，重点描述跨语言契约、能力矩阵、接入方式和语言使用指南。SDK 源目录 `sdks/<lang>/README.md` 只作为对应语言包的开发、构建和发布入口。

## 文档边界

| 位置 | 职责 |
| --- | --- |
| `docs/sdks/` | 使用者文档、能力矩阵、跨语言行为约束 |
| `docs/sdks/<lang>/` | 单语言接入指南、API 概览、示例说明 |
| `sdks/<lang>/README.md` | 源码目录入口、构建、测试、发布、包管理说明 |
| `sdks/<lang>/*_GUIDE.md` | 迁移期遗留文档；正式内容应逐步合并回 `docs/sdks/<lang>/` 或 README |

## 当前统一基线

- `SDK <-> Agent` 默认使用 Agent 本地 gateway 上的 TCP session。
- SDK 不应再暴露 `rpc_addr`、`local_listen` 或本地 server 回拨语义。
- 跨语言能力以 [SDK 能力矩阵](/sdks/sdk-parity-matrix) 为准。
- 协议细节以 [SDK Wire Protocol](/architecture/sdk-wire-protocol) 为准。

## 语言入口

| 语言 | 文档入口 | 源码入口 |
| --- | --- | --- |
| C++ | [C++ SDK](/sdks/cpp/) | `sdks/cpp/README.md` |
| Go | [Go SDK](/sdks/go/) | `sdks/go/README.md` |
| Java | [Java SDK](/sdks/java/) | `sdks/java/README.md` |
| JavaScript | [JavaScript SDK](/sdks/js/) | `sdks/js/README.md` |
| Python | [Python SDK](/sdks/python/) | `sdks/python/README.md` |
| C# | [C# SDK](/sdks/csharp/) | `sdks/csharp/README.md` |

## 迁移要求

- 新增使用者文档必须写入 `docs/sdks/<lang>/`。
- 新增构建、测试、发布说明必须写入 `sdks/<lang>/README.md` 或同目录开发文档。
- C++ 当前存在多份配置、插件、虚拟对象文档，迁移时应先选定 canonical 内容，再将重复页改成跳转或归档。
