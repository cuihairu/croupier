---
title: SDK 文档
---

# SDK 文档

这里汇总官方 SDK 的使用者文档、跨语言契约和能力边界。SDK 源码目录 `sdks/<lang>/README.md` 负责该语言包的开发、构建与发布入口。

## 文档边界

| 位置                    | 职责                                     |
| ----------------------- | ---------------------------------------- |
| `docs/zh/sdks/`         | 中文使用者文档、能力矩阵和跨语言约束。   |
| `sdks/<lang>/README.md` | 对应语言包的开发、构建、测试和发布入口。 |
| `docs/architecture/`    | Wire 协议、会话生命周期和 Agent 边界。   |
| `internal/proto` 与代码 | 协议字段和生效语义的最终依据。           |

## 统一基线

- L1/L2 Provider 连接 Agent 本地 gateway 的 TCP session。
- L3 Invoker 是独立调用方，只调用 Server HTTP API，不复用 Provider session。
- SDK 不暴露 `rpc_addr`、`local_listen` 或让 Agent 回拨 SDK 的本地 server 语义。
- 平台控制字段使用 protobuf；业务 payload 默认 UTF-8 JSON。
- 跨语言能力差异以 [SDK 能力矩阵](/zh/sdks/sdk-parity-matrix)为准。
- Wire 契约以 [SDK Wire 协议](/zh/architecture/sdk-wire-protocol)为准。

## 语言入口

| 语言       | 当前中文文档                                                                                | 源码入口                |
| ---------- | ------------------------------------------------------------------------------------------- | ----------------------- |
| C++        | [线程模型](./cpp/guide/threading.md)、[资源与操作](./cpp/guide/resources-and-operations.md) | `sdks/cpp/README.md`    |
| C#         | [概览](./csharp/guide/index.md)、[错误处理](./csharp/guide/error-handling.md)               | `sdks/csharp/README.md` |
| Go         | [SDK 能力矩阵](./sdk-parity-matrix.md)                                                      | `sdks/go/README.md`     |
| Java       | [SDK 能力矩阵](./sdk-parity-matrix.md)                                                      | `sdks/java/README.md`   |
| JavaScript | [SDK 能力矩阵](./sdk-parity-matrix.md)                                                      | `sdks/js/README.md`     |
| Python     | [SDK 能力矩阵](./sdk-parity-matrix.md)                                                      | `sdks/python/README.md` |

表格中出现英文站入口时，通过默认主题语言切换器可达对应内容；不复制英文正文顶替中文页。

## 接入原则

1. SDK 嵌入游戏进程后主动连接 Agent，不要等待 Agent 反向拨号。
2. 注册前准备稳定函数 ID、输入输出 schema、风险等级和业务语义。
3. 长耗时操作使用任务语义；高风险操作交给平台审批、审计和风控。
4. 心跳、drain、重连和函数版本变化要能幂等恢复。
5. 业务参数放 JSON payload；路由、身份、scope 和治理字段放协议层。

## 排障入口

先确认进程和 Agent session 已注册，再检查 provider session 心跳、函数 ID、schema digest、`gameId/env` 和可用实例。页面执行失败时，区分认证、RBAC、审批、绑定过期、函数离线和执行器未绑定这几类错误。
