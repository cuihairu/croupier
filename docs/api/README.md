---
title: API 概览
icon: code
order: 1
category:
  - API 参考
tag:
  - API
  - 接口
---

# API 概览

Croupier 当前对外主要分为两类接口：

| 类型 | 说明 |
| --- | --- |
| REST API | 面向 Dashboard 与外部管理调用 |
| Session Wire API | 面向 `SDK <-> Agent` 与 `Agent <-> Server` 的内部会话协议 |

## REST API

- 协议：HTTP / HTTPS
- 典型用途：Dashboard、管理接口、查询与配置

## Session Wire API

Session Wire API 不是 gRPC API，而是运行在轻量 session runtime 之上的内部协议。

共享能力：

- `tcp`
- 可选 `tls`
- framing
- request/response 复用
- heartbeat
- reconnect
- drain
- backpressure

子协议：

- `sdk-agent subprotocol`
- `agent-server subprotocol`

## 相关文档

- [REST API 详情](./rest.md)
- [SDK-Agent 传输重构设计](../architecture/sdk-agent-transport-redesign.md)
- [Agent-Server TCP Session 重构设计](../architecture/agent-server-session-transport-redesign.md)
- [SDK Wire Protocol](../architecture/sdk-wire-protocol.md)
