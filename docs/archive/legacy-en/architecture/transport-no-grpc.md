---
title: 传输层决策 — 游戏 SDK 使用 TCP，外部探测插件可使用本地 gRPC
---

# 传输层决策：游戏 SDK 使用 TCP，外部探测插件可使用本地 gRPC

## 状态

Accepted。2026-10-17 用户澄清适用范围：原禁令针对游戏 SDK 接入的体积与依赖负担，不是禁止独立外部探测程序使用 gRPC；本节与下列约束取代此前仓库级禁令表述。

## 决策

Croupier 内部 RPC(Server ↔ Agent ↔ SDK Provider)**不使用 gRPC**,采用自行开发 TCP transport + protobuf 强契约。

独立 `synthetics-agent` 与开发者探测插件之间允许采用 HashiCorp go-plugin 的本地 gRPC IPC；其依赖只属于外部探测宿主与插件，不得进入游戏 SDK 的构建、链接、生成代码或分发产物，也不得由公共包传递给游戏。Agent↔Server 的远程链路继续共用现有 TCP/TLS session。

2026-10-17 用户确定宿主侧 go-plugin 与 OTel 默认集成且支持禁用：运行时各有总开关，编译时分别规划 `no_go_plugin`、`no_otel` 裁剪选项，可组合构建；该默认策略不改变游戏 SDK 的依赖边界，具体装配、裁剪范围与验收列入 todo，尚未实现的开关不得标作已交付。

## 背景:从 gRPC 的坑里重构出来

项目早期曾用 gRPC 作内部传输,后经历 `gRPC → NNG → TCP-only` 的重构,最终移除 gRPC。git 历史:

- `7b3b955d2` chore(deps): bump google.golang.org/grpc
- `07d0f9faa` refactor(proto): remove gRPC service definitions from example proto files
- `573b94038` refactor: remove NNG dependency, migrate to TCP-only transport
- `4d0f1ffe4` fix: complete TCP-only migration (P0/P1/P2 issues)
- `775398748` Remove croupier-proto references

## 为什么游戏 SDK 接入不采用 gRPC

1. **体积过重**：用户历史接入场景中的 gRPC debug 版本约 **1.7GB**，引入体积超过游戏自身的依赖不合理；此处是历史场景记录，不是对所有语言、版本或构建方式的通用体积断言。
2. **依赖难治理**:gRPC 依赖链复杂,曾花**一周**仍未搞定依赖冲突。
3. **游戏后端不需要**:gRPC 的核心优势(跨语言 IDL、HTTP/2 多路复用、丰富生态)对单公司多游戏后端非必需;自行开发 TCP+proto 已满足强契约,且可控、可调试。
4. **历史教训**:从 gRPC 重构出来的代价很大,不应重蹈覆辙。

## 替代方案(已落地)

- 自行开发 TCP transport:length-prefix framing(`internal/transport/tcp`),4 字节 size + payload,单帧上限 32MB。
- protobuf 消息定义在 `proto/`,生成代码在 `pkg/pb`。
- 强契约由 proto IDL 保证;传输层轻量、可控、易调试。
- 游戏 SDK 接入链路如需统一 error 等能力，在自行开发 RPC 层补齐，不引入 gRPC runtime。

## 为什么「简单 TCP」对 SDK [Agent 足够]

这条链路的**部署拓扑**决定了传输选型:SDK(游戏服务器进程)→ Agent 几乎总是
同机或同游戏网络——不走公网、不穿 CDN/代理链。在该前提下,gRPC/HTTP2 换来的
跨广域网生态(负载均衡、重试传播、per-stream 流控)收益趋近于零,而六语言 SDK
各绑一套 gRPC runtime 的依赖税是实打实的(C++/C# 尤甚)。

自行开发 TCP 上「难做的那部分」已全部落地:

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
- **OTel 跨边界传播已落地（一期）**:trace context 经 invoker → server → agent → SDK 透传(见 sdk-otel-propagation.md)——平台侧 + Go SDK 一期已实现（含 traceparent 测试）。

### 重新评估传输层的触发条件

出现任一情况再评估 QUIC/HTTP2,否则继续在现有协议上补齐 drain 与五语言双车道:

1. SDK 跨广域网连 Agent(丢包+高延迟下 QUIC 才有意义)
2. 浏览器/不可信环境直连(应改走 Server HTTP,而非此链路)
3. 单连接持续大流量流式场景(per-stream 流控成为刚需)
4. 接入 service mesh 等要求 HTTP 系协议的基础设施

## 约束

- 游戏 SDK 与游戏接入链路禁止直接或传递引入 gRPC runtime；SDK 的协议、生成链、构建与分发继续采用现有 TCP+protobuf。
- 外部探测宿主及其插件允许使用 go-plugin 本地 gRPC，插件依赖包不得被游戏 SDK 或共用轻量探测内核导入；同一 monorepo/module 出现 gRPC 依赖不等于游戏产物实际引入，验收需检查 SDK 依赖闭包与构建产物。
- Agent↔Server 继续使用共用 TCP/TLS session，本次澄清不变更远程传输契约。
- `scripts/check-no-grpc.sh` 当前仍执行旧的全局直接依赖/import/service 禁令，需随插件宿主接入改为检查游戏 SDK 依赖边界及明确的插件宿主路径；该守卫调整已列入 todo，当前尚未修改实现。
- 评估任何新传输方案时,**体积与依赖复杂度是硬约束**,优先级高于"生态丰富度"。
