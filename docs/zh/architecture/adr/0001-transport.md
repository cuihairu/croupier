---
title: "ADR 0001: 传输选型——自研 TCP + protobuf"
---

# ADR 0001: 传输选型——自研 TCP + protobuf

**状态：** Accepted（2026-11-10）。本页是传输决策的现行记录，取代归档备注
[`docs/archive/legacy-en/architecture/transport-no-grpc.md`](../../../archive/legacy-en/architecture/transport-no-grpc.md)（仅作历史保留）。

**范围：** 所有 Croupier 长连会话链路——Server ↔ Agent 控制（TCP 19090）与
SDK ↔ sidecar provider（TCP 19091）。不含：浏览器 → Server 的 HTTP
REST/SSE、集群互联（独立链路）、以及本地插件 IPC（后者有范围豁免，见下）。

## 背景

Croupier 早期原型用 gRPC 作内部传输。项目经历了 `gRPC → NNG → TCP-only`
重构，最终从游戏接入链路移除 gRPC（git 历史：`7b3b955d2`、`07d0f9faa`、
`573b94038`、`4d0f1ffe4`、`775398748`）。驱动本决策的部署拓扑自那时起未变：

- SDK（游戏服务器进程）→ Agent 几乎总是同机或同游戏网络——不走公网、
  不穿 CDN/代理链。
- Agent 主动拨号 Server；一条出站会话承载双向流量。
- 单游戏公司多游戏；无 service mesh、无多组织联邦、无按租户端点扇出。

六语言 SDK（Go / JS / Python / Java / C# / C++）都要内嵌传输层。每引入一个
运行时，依赖税就乘以六；游戏工作室对二进制体积与依赖冲突的敏感度远高于
服务端项目。

## 决策

Croupier 内部 RPC 一律使用**自研 TCP 传输 + protobuf 载荷**
（`core/transport/tcp`；`internal/transport` 为兼容 shim）。上述链路
**否决 gRPC**，游戏 SDK 链路不得直接或传递引入 gRPC。契约见
[sdk-wire-protocol.md](../sdk-wire-protocol.md)，并由 Go agent 传输与全部
六语言 SDK 传输的回归测试钉住。

**范围豁免：** 外部探测/插件宿主与其插件之间允许使用 HashiCorp go-plugin
的本地 gRPC IPC，仅限插件宿主子树（`agents/synthetics/pluginhost/`、
`agents/sidecar/funcplugin/`、`core/pluginhost/`）。其依赖绝不得进入游戏
SDK 的构建、链接、生成代码或分发产物，也不得经公共包传递给游戏。无论豁免
与否，Agent ↔ Server 远程链路继续共用现有 TCP/TLS 会话。

两个可选集成都带裁剪开关：`no_go_plugin` build tag 裁掉插件宿主，
`no_otel` build tag 裁掉遥测 provider 代码
（`internal/telemetry/provider_sdk_trim.go`）。

## 为什么否决 gRPC（逐项对比）

| gRPC 优势               | 对本链路的裁决                                                                                                                                               |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 跨语言 IDL              | 需求真实，但 protobuf 本身就提供 IDL。线上帧是 4 字节长度前缀 + 载荷——各语言无需 gRPC runtime 即可轻松实现，消息经 `proto/` 生成到 `pkg/pb`。                |
| HTTP/2 多路复用         | 拓扑抵消收益。gRPC 擅长的广域网特性（per-stream 流控、LB 感知的重试传播、逐流 deadline）在丢包高延迟网络才有意义；SDK → Agent 是同机或游戏内网。             |
| 单链路请求并发          | 单条 TCP 连接上的 `reqID` 多路复用已交付；历史双向隧道死锁已修并有回归测试。                                                                                 |
| 生态（LB、xDS、拦截器） | 单公司部署没有 mesh 可集成。拦截器类需求（鉴权、追踪）直接实现：握手鉴权（`agent.providerAuth`）、OTel trace context 传播。                                  |
| 工具链成熟度            | 历史代价具体：一次依赖冲突治理耗时超过一周；真实接入案例中 gRPC debug 版本约 1.7 GB——比游戏本体还重（历史场景记录，不是对所有版本/构建方式的通用体积断言）。 |
| 双向流                  | 所需形态是带进度事件的请求/响应（任务）与控制帧——在现有协议里都是一等帧，包括饱和时 fail-fast 的双车道有界派发。                                             |

## 自研 TCP 层已提供的能力（已落地，非愿景）

| 需求       | 实现                                                                                                                                                 |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 帧契约     | 4 字节大端长度前缀 + 载荷；`MaxFrameBytes` = 32 MB（`core/transport/tcp/framing.go`）                                                                |
| 演进       | 会话头的协议版本字节（`0x01`），为升级预留                                                                                                           |
| 单连接并发 | `reqID` 多路复用；对端可在等自己响应的同时处理本端回调                                                                                               |
| 存活检测   | 心跳消息族（`0x01`/`0x05`）                                                                                                                          |
| 过载保护   | 双车道（控制/业务）有界派发 + 有界 worker 池 + 队列饱和 fail-fast                                                                                    |
| 优雅摘流   | drain 语义（会话级定义）                                                                                                                             |
| 安全       | TLS 按配置启用（各链路同 `cluster.interconnectTls` 思路）；同机/游戏内网明文是合理缺省                                                               |
| 重连       | 指数退避 + 自动重注册                                                                                                                                |
| 可观测     | OTel trace context 跨 invoker → server → agent → SDK 传播（平台 + Go SDK 一期；见 `internal/telemetry/context.go`、`sdks/go/pkg/croupier/trace.go`） |

## 执行守卫

`scripts/check-no-grpc.sh`（CI）守的是**依赖边界**，不是一刀切禁令：

1. `google.golang.org/grpc` import 只允许出现在插件宿主子树
   （`agents/synthetics/pluginhost/`、`agents/sidecar/funcplugin/`、
   `core/pluginhost/`）；游戏 SDK、其他 agent、服务端一律禁止。
2. 顶层 `proto/` 禁止 `service`/`rpc` 定义——会话 wire 契约保持帧级；
   插件 IPC 契约在各插件宿主的 `protocol/` 目录内独立版本化。
3. 主生成链（Makefile、`scripts/gen-proto.sh`、`proto/buf.yaml`）禁止使用
   gRPC 代码生成插件。

`go.mod` 中 `google.golang.org/grpc` 的直接依赖是宿主插件运行时的必然结果，
守卫对此 fail-open 提示，不算违规。

## 已知边界（诚实清单）

- **帧级队头阻塞：** 单连接所有帧在网线上串行。一个接近 32 MB 的大响应在写
  时，排在后面的小帧（含心跳回复）会被延迟。双车道解决的是**派发**优先级，
  不是**线路**优先级。当前 GM 流量小载荷、低 QPS，实际无感；大帧场景应先
  收紧业务侧最大帧，而不是换传输。
- **流控粒度为连接级：** TCP 窗口 + 应用层 fail-fast。只有单连接持续大量
  并发长任务流才需要 per-stream 流控；该形态今天不存在。
- **HTTP/2 时代的便利是手工实现的，因而比 gRPC 白送的要窄**（例如跨跳的
  deadline 传播是显式 metadata，不是传输层托管）。

## 重新评估的触发条件

出现任一情况即重开传输决策（QUIC/HTTP-2 或同类）；否则继续在现有协议上
扩展：

1. SDK 跨公网连 Agent（丢包 + 高延迟下 QUIC 的收益才真实）。
2. 浏览器或不可信环境需要直连（正确答案是走 Server HTTP 面，不是这条链路）。
3. 单连接承载持续大流量流式（per-stream 流控成为硬需求）。
4. 这两条链路上引入要求 HTTP 系协议的基础设施（service mesh ingress）。

评估替代方案时，**体积与依赖复杂度是硬约束**，优先级高于生态丰富度。

## 参考

- Wire 契约：[sdk-wire-protocol.md](../sdk-wire-protocol.md)
- 被取代的历史备注：
  [`docs/archive/legacy-en/architecture/transport-no-grpc.md`](../../../archive/legacy-en/architecture/transport-no-grpc.md)
- 传输实现：`core/transport/tcp/`（shim：`internal/transport/`）
- 会话重构上下文：
  [agent-server-session-transport-redesign.md](../agent-server-session-transport-redesign.md)、
  [sdk-agent-transport-redesign.md](../sdk-agent-transport-redesign.md)
- 边界守卫：`scripts/check-no-grpc.sh`
