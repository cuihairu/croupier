---
title: Session Runtime 参考实现
icon: compass
order: 6
category:
  - 系统架构
tag:
  - 架构
  - 参考
---

# Session Runtime 参考实现

本文档用于回答两个问题：

1. 市面上有没有和 Croupier 当前目标相近的开源实现
2. 为什么当前不直接选其中某一个作为默认基础设施

结论先行：

- 有相近实现
- 没有一个与当前目标完全重合
- 最合理的路径是借鉴它们的分层思想，自建轻量 `session runtime`

## 我们当前真正要的是什么

Croupier 当前真正要的是一套：

- 单连接
- 双向主动请求
- 多路复用
- 可重连
- 可 heartbeat
- 可 drain
- 可背压
- 默认跨语言易接入

它不是：

- 完整 P2P 网络
- 大而全 RPC 框架
- 高性能消息总线产品
- 专门为共享内存或极致低延迟优化的基础设施

所以我们的目标更接近：

- `session runtime`

而不是单纯某种 transport 库。

## 参考对象概览

| 项目 | 借鉴点 | 不直接采用的主因 |
| --- | --- | --- |
| `RSocket` | 双向、多路复用、交互模型清晰 | 协议栈偏重，超出当前最小需求 |
| `Aeron` | 同机/低延迟/媒体驱动思维 | 更像高性能消息基础设施，不适合作为默认控制面 |
| `eCAL` | 多传输层抽象、工程化强 | 面向数据分发系统，模型与当前 session 不完全重合 |
| `Zenoh` | 跨网络传输与统一抽象 | 体系更大，超出当前平台边界 |
| `libp2p` | secure channel + stream mux 思维 | 更偏 P2P、多节点网络能力 |
| `IceRPC / Ice` | 传输抽象、RPC 工程化成熟 | 整体 RPC 框架语义偏重 |
| `yamux` | 轻量 stream multiplexing | 只解决 mux，不解决上层 session 语义 |
| `kcp-go` | UDP 可靠传输实现参考 | 只是一种 transport 选择，不是完整 session runtime |

## RSocket

官方资料：

- https://rsocket.io/about/protocol/
- https://rsocket.io/about/motivations/
- https://rsocket.io/about/implementations/

适合借鉴的点：

- 明确把连接视为长期 session
- 支持双向交互模型
- request/stream/channel 等交互语义清晰
- 跨语言实现经验成熟

为什么不直接采用：

- 协议头、帧类型和交互模型比当前需求更重
- transport 适配面比较大，容易把实现复杂度拉高
- 当前只需要最小 request/response + event 能力，不需要完整 RSocket 模型

适合借鉴的结论：

- 学它的 session 心智和双向模型
- 不直接引入整套协议

## Aeron

官方资料：

- https://aeron.io/docs/aeron/overview/
- https://aeron.io/docs/aeron/media-driver/

适合借鉴的点：

- 很强的 session / channel / media driver 心智
- 对同机高性能、低延迟和媒体承载区分非常明确
- 对未来 `shared-memory` 或更高性能 media 很有启发

为什么不直接采用：

- 当前主诉求不是极致吞吐或极低延迟
- 当前更重视跨语言接入成本、可控性和排障简单性
- 作为默认控制面基础设施会过重

适合借鉴的结论：

- 学它对 `media` 与 `session` 的分层思路
- 不把它作为当前默认依赖

## eCAL

官方资料：

- https://eclipse-ecal.github.io/ecal/stable/advanced/transport_layers.html
- https://github.com/eclipse-ecal/ecal

适合借鉴的点：

- 对多 transport 层的工程化表达比较清晰
- 能帮助我们思考 `media kind` 与 `transport adapter` 的命名
- 对“未来可插拔传输承载”很有参考价值

为什么不直接采用：

- 它更偏数据分发和系统集成框架
- 当前 Croupier 关注的是 session 语义、注册语义与平台治理
- 直接引入会把问题域从“轻量 runtime”扩成“完整通信框架选型”

适合借鉴的结论：

- 学它的 transport layering
- 不直接照搬它的整体系统模型

## Zenoh

官方资料：

- https://zenoh.io/docs/manual/quic/
- https://zenoh.io/blog/2024-10-21-zenoh-firesong/

适合借鉴的点：

- 对异构网络和多种传输承载支持丰富
- 对统一抽象和跨环境部署很有价值

为什么不直接采用：

- 体系过大
- 当前不需要把平台演进成通用数据网格
- 引入成本和心智负担都偏高

适合借鉴的结论：

- 作为中长期 transport 扩展参考
- 不作为当前 v1 默认路径

## libp2p

官方资料：

- https://docs.libp2p.io/concepts/secure-comm/overview/
- https://docs.libp2p.io/concepts/multiplex/overview/

适合借鉴的点：

- secure channel
- stream multiplexing
- 协议协商

为什么不直接采用：

- 当前没有打洞、节点发现、mesh 网络这些需求
- 把 libp2p 引进来会把问题域扩大太多

适合借鉴的结论：

- 学它的协商和多路复用分层
- 不采用它的网络系统边界

## Ice / IceRPC

官方资料：

- https://docs.zeroc.com/ice/3.8/cpp/transports
- https://zeroc.com/icerpc

适合借鉴的点：

- 工程化成熟
- transport 抽象明确
- 跨语言 RPC 经验丰富

为什么不直接采用：

- 我们当前不想引入完整 RPC 框架语义
- 当前设计更偏轻量 session 基座，而不是完整远程对象或 RPC 框架

适合借鉴的结论：

- 学它的抽象边界
- 不直接接纳整套框架

## yamux

官方资料：

- https://github.com/hashicorp/yamux

适合借鉴的点：

- 很轻的连接内多路复用思路
- 对“单连接多 stream”很有启发

为什么不直接采用：

- 它只解决 multiplexing
- 不解决注册、会话、心跳、重连、业务消息边界

适合借鉴的结论：

- 可参考其 mux 设计
- 不能把它当完整答案

## kcp-go

官方资料：

- https://github.com/xtaci/kcp-go

适合借鉴的点：

- 如果未来真要走 UDP / KCP，这类实现是可参考的 transport adapter

为什么不直接采用：

- 这只是某一种承载实现
- 当前默认场景下 `tcp` 已经足够
- 还不到为了潜在场景引入额外复杂度的时候

适合借鉴的结论：

- 将来若扩展 transport adapter，可参考
- 当前 v1 不需要

## 当前最合理的工程选择

结合当前场景，最合理的选择是：

- v1 固定 `tcp`
- 统一 `shared session runtime`
- 统一 framing / header / request-response 复用
- 用 `subprotocol` 区分 `sdk-agent` 与 `agent-server`
- 默认 payload 固定 `JSON`

这条路径的优点是：

- 比 `RSocket` 轻
- 比 `libp2p` 简单
- 比 `Aeron` 更通用
- 比“继续围绕 历史消息模式 修补”更贴近真实需求

## 中长期可借鉴路线

如果未来真的要做“可插拔 transport/media”的扩展，推荐借鉴顺序：

1. 从 `RSocket` 学 session 和交互模型
2. 从 `yamux` 学轻量 mux
3. 从 `Aeron` 学 media / channel 分层
4. 从 `eCAL` 学 transport layering
5. 在必要时再评估 `QUIC`、`KCP`、`shared-memory`

也就是说：

- 不是选一个项目全盘照搬
- 而是分别借鉴它们在不同层的优点

## 最终结论

当前最接近我们目标的专业描述不是：

- `历史消息模式`
- `RPC transport`
- `message bus`

而是：

- `shared session runtime`

如果强调未来的可扩展承载能力，可以表述为：

- `pluggable transport session runtime`

这也是当前文档体系建议统一采用的说法。
