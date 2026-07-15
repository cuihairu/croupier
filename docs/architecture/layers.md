---
title: 分层设计
icon: layers
order: 2
category:
  - 系统架构
tag:
  - 架构
  - 分层
---

# 分层设计

> **状态**：Current — 当前实现/规范，可作为实现依据。

Croupier 当前采用四层结构，但传输模型已经统一为 session 思维，而不是旧的 `gRPC/旧传输 回拨` 思维。

## 四层结构

1. 展示层
   - Dashboard / 管理 UI
2. 控制层
   - Server / Registry / Dispatch / RBAC / Audit
3. 代理层
   - Agent / Session Client / Local Gateway
4. 业务层
   - Game Server / SDK / 第三方应用

## 层间通信

| 边界 | 默认协议 | 默认 TLS | 说明 |
| --- | --- | --- | --- |
| Dashboard -> Server | HTTP REST | 视部署决定 | 面向人和前端 |
| Agent -> Server | TCP session | 开启 | `agent-server subprotocol` |
| SDK -> Agent | TCP session | 关闭 | `sdk-agent subprotocol` |
| Game Server -> Agent | TCP session | 关闭 | 本地接入优先 |

## 关键变化

### 旧模型

- `Agent -> Server` 只做注册与心跳
- `Server/Dispatcher` 再根据 `rpc_addr` 回拨 `Agent`
- SDK 还可能暴露本地监听端口给 `Agent`

### 新模型

- `Agent` 只维护到 `Server` 的主动 session
- `Server -> Agent` 的请求复用既有 session
- `SDK` 不监听端口，只主动连 `Agent`
- `Agent` 本地监听只服务本地接入方

## transport / runtime / subprotocol

为避免概念混乱，当前文档统一使用下面三层术语：

### transport

指底层承载方式：

- `tcp`
- 可选 `tls`

### shared session runtime

指共享的连接与复用运行时：

- framing
- mux
- request/response 匹配
- heartbeat
- reconnect
- drain
- backpressure

### subprotocol

指跑在 session runtime 上的具体应用层子协议：

- `sdk-agent subprotocol`
- `agent-server subprotocol`

## Agent 的职责

Agent 当前应被视为“本地网关 + 上游 session client”：

- 向 `Server` 主动建立上游 session
- 维护本地 provider / process / function 视图
- 接收 `Server` 下发的 invoke/task/ops 请求
- 将本地 `SDK / GameServer / 第三方应用` 接入统一到本地 session 边界

## 不再推荐的术语

以下术语若在旧文档中出现，应理解为历史实现痕迹：

- `LocalControlService`
- `RegisterLocal`
- `rpc_addr` 作为回拨入口
- SDK 本地监听
- `Server -> Agent` 反向直拨

这些概念不应再作为新设计依据。
