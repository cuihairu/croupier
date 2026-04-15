---
title: 术语与分层
icon: tags
order: 3
category:
  - 系统架构
tag:
  - 架构
  - 术语
---

# 术语与分层

本文档用于统一 Croupier 当前 session 方向的术语，避免把不同层面的概念混在一起讨论。

当前最推荐的总称是：

- `session runtime`

如果需要更完整地表达“底层可替换传输承载”的方向，可以使用：

- `pluggable transport session runtime`

这个名字比“传输层”更准确，因为我们要的并不只是换一个 socket 实现，而是一整套连接、复用、重连、摘流与治理语义。

## 术语层次

建议统一按下面五层理解：

1. `media kind`
2. `transport adapter`
3. `session runtime`
4. `subprotocol`
5. `application semantics`

这五层职责不同，不应混用。

## media kind

`media kind` 指底层通信介质或链路类别。

当前默认且唯一必须支持的是：

- `tcp`

未来如果确有必要，可以扩展为：

- `unix-domain-socket`
- `quic`
- `shared-memory`

但这些都不属于 v1 必需范围。

要点：

- `tls` 不是 `media kind`
- `tls` 是附着在可靠字节流之上的安全层
- `tcp` 只是承载，不等于完整 session 语义

## transport adapter

`transport adapter` 指把某种底层承载接入统一 session runtime 的适配层。

例如：

- `tcp transport adapter`
- `quic transport adapter`
- `shared-memory transport adapter`

它的职责是：

- 建连与关连
- 提供可靠读写或明确的可靠性语义
- 暴露最大帧、半关闭、错误分类等基础能力
- 向上提供统一的收发抽象

它不负责：

- request/response 配对
- session 级重连策略
- provider / agent 握手语义

因此不要把 `transport adapter` 和 `subprotocol` 混为一谈。

## session runtime

`session runtime` 是当前设计的核心概念。

它位于 `transport adapter` 之上，负责统一会话语义：

- framing
- header encode/decode
- request id 分配
- 双向 request/response 复用
- in-flight 跟踪
- heartbeat
- reconnect
- drain
- backpressure
- session 生命周期管理

可以把它理解为：

- 一个轻量双向 session 框架
- 一个可复用的连接运行时
- 一个比“裸 TCP”更高一层、但比“完整 RPC 框架”更轻的公共基座

这也是为什么当前最合适的名称不是单纯的 `transport layer`，而是 `session runtime`。

## drain

`drain` 是 `session runtime` 的核心运行时语义之一，建议统一翻译为：

- 摘流
- 排空流量

它的定义是：

- 停止向该 session 分配新的业务请求
- 允许已经在途的请求在宽限时间内继续完成
- 在排空完成或宽限时间到达后，再关闭连接或移除 session

它不是：

- 立刻断开连接
- 普通限流
- 一般性的请求超时

最简理解可以写成：

- `close` 是直接掐断
- `drain` 是先停新单，再把手上的单处理完

### drain 与 backpressure 的区别

二者都属于运行时治理能力，但目标不同：

- `backpressure`
  - 表示当前过忙，需要减速、限流或延迟重试
- `drain`
  - 表示该 session 正在优雅退出，不再接新流量

因此：

- `backpressure` 更偏“还要继续服务，但要降速”
- `drain` 更偏“准备下线，先排空再退出”

### drain 的最小行为约束

v1 建议统一遵循下面的最小语义：

1. session 进入 `draining` 状态
2. 不再接收新的业务请求分配
3. 继续处理已在途的请求
4. 心跳继续保持，避免对端误判已死
5. 若在宽限时间内排空完成，则优雅关闭
6. 若宽限时间到达仍未排空，则按策略取消或强制关闭剩余请求

### drain 的典型使用场景

- 节点发布前优雅下线
- SDK / Agent 进程准备退出
- 连接切换到新 session 前的平滑摘流
- 节点过载后不再接新请求

### drain 的设计边界

`drain` 是会话级语义，不是单个请求级语义。

也就是说：

- 单个请求的取消仍由 `CancelJobRequest` 等消息承担
- `drain` 负责的是“这条连接还要不要继续接新流量”

## subprotocol

`subprotocol` 指运行在共享 `session runtime` 上的应用层子协议。

当前有两套核心子协议：

- `sdk-agent subprotocol`
- `agent-server subprotocol`

二者共享：

- 同样的 framing
- 同样的 header
- 同样的 request/response 复用规则
- 同样的重连、心跳、drain 思维

二者不同的是：

- 首帧握手消息
- 注册内容
- 路由语义
- 默认 TLS 策略

所以 `subprotocol` 的专业含义是：

- 同一 runtime 上的不同应用层协议变体

它不是：

- 个性化配置
- 环境 profile
- 传输类型别名

## application semantics

`application semantics` 指具体业务消息与平台消息的语义层。

例如：

- `ProviderConnectRequest`
- `RegisterRequest`
- `InvokeRequest`
- `JobEvent`

以及这些消息承载的：

- protobuf 平台字段
- JSON 业务 payload

这层才是真正定义“这条消息代表什么”的地方。

## profile 的推荐用法

`profile` 在工程语境里更适合表示“命名配置预设”，而不是协议层概念。

推荐用法：

- `deployment profile`
- `runtime profile`
- `connection profile`

例如：

- `local-dev`
- `intra-cluster`
- `cross-zone-tls`
- `cross-zone-mtls`

不推荐把下面这些概念命名成 `profile`：

- `sdk-agent subprotocol`
- `agent-server subprotocol`
- `frame format`

因为这些属于协议与运行时定义，不属于配置预设。

## v1 的明确边界

v1 当前明确收敛为：

- `media kind = tcp`
- `transport adapter = tcp transport adapter`
- `session runtime = shared session runtime`
- `subprotocol = sdk-agent / agent-server`
- `payload = JSON`

这符合 KISS 和 YAGNI：

- 不先抽象一堆并不存在的 transport
- 不提前做多 codec 协商
- 不把协议讨论复杂化

## 后续扩展的正确位置

如果未来要支持：

- 同机共享内存
- 跨机 QUIC
- 特定低延迟链路

正确扩展点应是：

1. 新增 `media kind`
2. 新增对应 `transport adapter`
3. 复用既有 `session runtime`
4. 尽量不改 `subprotocol`

也就是说：

- 先扩 transport adapter
- 再复用 runtime
- 尽量不动 application semantics

这才符合当前分层目标。

## 与历史概念的区别

以下词在旧讨论里容易混淆：

- `NNG pattern`
- `gRPC`
- `LocalControl`
- `rpc_addr`

它们的问题不是“不能工作”，而是没有准确表达当前所需的 session 语义。

当前文档应尽量统一使用：

- `session runtime`
- `transport adapter`
- `subprotocol`
- `provider session`
- `agent session`

## 结论

如果要给当前方向起一个最准确、最不误导的名字，推荐：

- `shared session runtime`

如果要表达中长期愿景，推荐：

- `pluggable transport session runtime`

前者适合当前实现与文档，后者适合架构演进讨论。
