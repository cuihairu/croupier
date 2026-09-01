---
title: 传输层决策 — 不使用 gRPC
---

# 传输层决策:不使用 gRPC

## 状态

Accepted(已落地,不可逆)。**不要再推荐 gRPC。**

## 决策

Croupier 内部 RPC(Server ↔ Agent ↔ SDK Provider)**不使用 gRPC**,采用自研 TCP transport + protobuf 强契约。

## 背景:从 gRPC 的坑里重构出来

项目早期曾用 gRPC 作内部传输,后经历 `gRPC → NNG → TCP-only` 的重构,最终移除 gRPC。git 历史:

- `7b3b955d2` chore(deps): bump google.golang.org/grpc
- `07d0f9faa` refactor(proto): remove gRPC service definitions from example proto files
- `573b94038` refactor: remove NNG dependency, migrate to TCP-only transport
- `4d0f1ffe4` fix: complete TCP-only migration (P0/P1/P2 issues)
- `775398748` Remove croupier-proto references

## 为什么不推荐 gRPC

1. **体积过重**:gRPC debug 版本约 **1.7GB**,对游戏后端是不可接受的负担。
2. **依赖难治理**:gRPC 依赖链复杂,曾花**一周**仍未搞定依赖冲突。
3. **游戏后端不需要**:gRPC 的核心优势(跨语言 IDL、HTTP/2 多路复用、丰富生态)对单公司多游戏后端非必需;自研 TCP+proto 已满足强契约,且可控、可调试。
4. **历史教训**:从 gRPC 重构出来的代价很大,不应重蹈覆辙。

## 替代方案(已落地)

- 自研 TCP transport:length-prefix framing(`internal/transport/tcp`),4 字节 size + payload,单帧上限 32MB。
- protobuf 消息定义在 `proto/`,生成代码在 `pkg/pb`。
- 强契约由 proto IDL 保证;传输层轻量、可控、易调试。
- 如需 gRPC 才有的能力(例如统一 error 协议),在自研 RPC 层补齐(定义 `Status`/`RpcError` proto 消息),**而非引入 gRPC**。

## 为什么「简单 TCP」对 SDK ↔ Agent 足够

这条链路的**部署拓扑**决定了传输选型:SDK(游戏服务器进程)→ Agent 几乎总是
同机或同游戏网络——不走公网、不穿 CDN/代理链。在该前提下,gRPC/HTTP2 换来的
跨广域网生态(负载均衡、重试传播、per-stream 流控)收益趋近于零,而六语言 SDK
各绑一套 gRPC runtime 的依赖税是实打实的(C++/C# 尤甚)。

自研 TCP 上「难做的那部分」已全部落地:

| 需求           | 实现                                                                    |
| -------------- | ----------------------------------------------------------------------- |
| 单连接并发请求 | reqID 多路复用;对端可在等自己响应的同时处理本端回调(双向隧道死锁已修)   |
| 存活检测       | 心跳(0x01/0x05 消息族)                                                  |
| 过载保护       | 双车道派发 + 有界 worker 池 + 队列满 fail-fast(见 sdk-wire-protocol.md) |
| 优雅摘流       | drain 语义(会话级,协议已定义)                                           |
| 安全           | TLS 按配置启用;同机/游戏内网明文是合理缺省                              |
| 重连           | 指数退避 + 自动重注册                                                   |
| 演进           | 协议版本字节(0x01)预留                                                  |

### 已知边界(诚实清单)

- **帧级队头阻塞**:单连接所有帧串行传输,一个接近 32MB 上限的大响应在写时,
  排在后面的小帧(如心跳响应)会被延迟。双车道解决的是**派发**优先级,不是
  **线路**优先级;当前 GM 流量(小 payload、低 QPS)下无实感,大帧场景应
  收紧业务最大帧而非换传输。
- **流控粒度为连接级**:TCP 窗口 + 应用层 fail-fast 够用;单连接大量并发长
  任务流的场景才需要 per-stream 流控,当前不存在该形态。
- **OTel 跨边界传播未接**:trace context 尚未 invoker → server → agent → SDK
  全链路透传(见 sdk-otel-propagation.md,规划中)——可观测性缺口,非传输缺口。

### 重新评估传输层的触发条件

出现任一情况再评估 QUIC/HTTP2,否则继续在现有协议上补齐 drain 与五语言双车道:

1. SDK 跨广域网连 Agent(丢包+高延迟下 QUIC 才有意义)
2. 浏览器/不可信环境直连(应改走 Server HTTP,而非此链路)
3. 单连接持续大流量流式场景(per-stream 流控成为刚需)
4. 接入 service mesh 等要求 HTTP 系协议的基础设施

## 约束

- **不要再推荐 gRPC**。新增 RPC 需求一律走自研 TCP+proto。
- `go.mod` 中的 `google.golang.org/grpc` 若为某库间接引入,应标注并定期评估能否移除;**不得新增直接 gRPC 用法**。
- 检测:`rg "google.golang.org/grpc" internal/` 应零命中业务传输代码(只允许 `go.mod` 间接依赖或 `docs/archive/` 归档代码)。
- 评估任何新传输方案时,**体积与依赖复杂度是硬约束**,优先级高于"生态丰富度"。
