---
title: 传输术语表
---

# 传输术语表

> **状态**：Current —— 只描述已交付的会话传输层。仅覆盖 wire/会话层；
> 页面模型术语见[看板术语表](./dashboard-glossary)。

| 术语                          | 定义                                                                                | 不是什么                                                  |
| ----------------------------- | ----------------------------------------------------------------------------------- | --------------------------------------------------------- |
| 会话运行时（session runtime） | 所有会话共用的传输基座：`tcp/tls + framing + mux + reconnect + heartbeat + drain`   | 消息历史存储、队列，或按调用建连                          |
| 子协议（subprotocol）         | 跑在会话运行时之上的应用层协议变体；握手消息、注册内容与路由语义不同                | "自定义配置"、插件系统，或另一套传输                      |
| `sdk-agent` 子协议            | SDK → sidecar 的会话（sidecar 本地 TCP 监听 19091），用于函数注册与调用             | SDK 直连 Server；SDK 不监听本地端口                       |
| `agent-server` 子协议         | agent → server 的控制会话（主动外拨 19090），承载注册、健康、任务与转发请求         | server 反向接入 agent；agent 始终外拨                     |
| 帧（Frame）                   | 会话上的 wire 单元：4 字节帧长前缀 + 8 字节 Header + protobuf body；单帧上限 32 MiB | 一个完整请求（请求可跨多帧，如流式），或 HTTP 载荷        |
| 握手（Handshake）             | 会话的第一帧；协商身份与能力（provider 侧承载 `ProviderConnectRequest`）            | HTTP upgrade、纯 TLS 步骤，或可省略——每个会话都以握手开始 |
| reqID 多路复用                | 单连接上以请求 id 关联双向并发 in-flight 请求                                       | 一请求一连接、无队头阻塞的有序流，或 HTTP/2 stream        |
| Drain                         | 会话优雅收尾：停止接收新工作、等 in-flight 收敛后关连接                             | 中止、硬取消，或配置热加载                                |
| L1 Core Provider              | 每个 SDK 必须实现的最小被调方表面：构造 / 注册 / 会话 / 服务循环 / 停止             | 可选扩展或服务端特性                                      |
| L2 Provider 扩展              | 可选 provider 能力：JSON Schema、drain、控制面 manifest、TLS、文件传输              | L1 合规的必要条件；调用方能力                             |
| L3 Invoker                    | 调用方能力（调用已注册函数），独立模块交付                                          | 被调方的必要条件；只被调用时不涉及                        |
| L4 语言扩展                   | 语言/引擎附加件（DI、Unity、Lua、Spring Boot starter、Skynet）                      | wire 协议的一部分；每个 SDK 都保证有                      |

## 延伸阅读

- wire 协议约定：[SDK Wire 协议](./sdk-wire-protocol)
- 传输决策历史：[ADR 0001 — 传输](./adr/0001-transport)
- SDK 分层细节：[SDK 文档](../sdks/)
