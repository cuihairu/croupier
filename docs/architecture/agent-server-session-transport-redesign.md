# Agent-Server TCP Session 重构设计

## 状态
- 状态: Accepted Target Design
- 适用范围: `Agent <-> Server`
- 不在范围内: `SDK <-> Agent`

## 设计结论

本设计用于收敛 `Agent <-> Server` 主链路的传输模型，核心结论如下：

1. `Agent <-> Server` 默认传输从当前 `NNG REQ/REP` 迁移为独立的 `tcp` session，并按需启用 `tls`。
2. `Agent` 只保留到 `Server` 的主动出站长连接，不再要求 `Server` 反向直连 `Agent` 暴露的 `rpc_addr`。
3. `Server -> Agent` 的 `Invoke`、`StartJob`、`CancelJob`、`Ops` 请求，统一复用已有 `Agent-Server` session 下发。
4. `Agent` 本地监听地址只服务 `GameServer / SDK / 第三方应用`，不再承担 `Server -> Agent` 回拨职责。
5. 不再把 `NNG pattern` 作为主链路抽象中心，统一收敛为“单连接、双向、多路复用、可重连、可治理”的轻量 session 协议。

## shared session runtime 与 subprotocol

这里同样需要明确两个术语：

- `shared session runtime`
  - 指两条主链路共用的会话传输基座
  - 包括 `tcp/tls + framing + mux + reconnect + heartbeat + drain + backpressure`
- `subprotocol`
  - 指在该基座上运行的应用层子协议
  - 对 `Agent <-> Server` 而言，就是 `agent-server subprotocol`

`agent-server subprotocol` 的关键特征是：

- 首帧必须是 `RegisterRequest` 或其后续等价 connect 消息
- 默认启用 `tls`
- 面向 agent session，而不是 `rpc_addr` 回拨

## 背景

当前仓库里，`Agent <-> Server` 实际上混合了两条不同语义的链路：

- 控制面: `Agent` 通过 `NNG REQ/REP` 主动连接 `Server`，完成 `Register/Heartbeat`
- 调用面: `Server/Dispatcher` 再根据 `rpc_addr` 主动拨回 `Agent`，执行 `Invoke/Job/Ops`

这带来了几个问题：

- 传输模型分裂: 控制面一条连接，调用面另一条连接
- `rpc_addr` 语义混乱: 看起来像“Agent 的注册信息”，本质却是“供 Server 回拨的入口”
- 集群语义不清晰: `Agent` 实际连到了某个 `Server` 节点，但调用面又试图绕开该 session 直接拨 `Agent`
- `NNG REQ/REP` 本身不适合“在已有连接上由 Server 主动发新请求”

这不是因为 `NNG` 没有长连接能力，而是因为当前使用的 `REQ/REP` pattern 与目标会话模型不一致。

## 我们真正需要的东西

当前场景真正需要的不是“某个消息中间件 pattern”，而是一个轻量的应用层 session：

- 底层是一条可靠长连接
- 连接建立后先做身份与能力协商
- 双方都能在同一条连接上主动发起新请求
- 同一连接上允许多个并发 in-flight 请求
- 断线后旧 session 立即失效，重连后重建新 session
- 支持心跳、背压、drain、超时、取消、优雅摘流

换句话说，我们要的不是 `REQ/REP`，而是“单连接双向会话协议”。

## 为什么当前场景更适合 TCP Session

### 业务边界特点

当前 `Agent <-> Server` 场景有几个非常明确的约束：

- 主要部署在内网、同机房、同集群网络中
- 不需要 `libp2p` 那类打洞、对等发现、多节点 mesh 能力
- 不需要 `RSocket` 那种覆盖大量 transport 和交互模型的通用协议栈
- 不需要默认引入 `Aeron` 这种偏高性能消息基础设施
- 需要的是稳定、简单、可控、可跨版本演进的传输层

因此默认方案应优先满足：

- 简单
- 易于排错
- 便于跨服务统一
- 便于后续与 `SDK <-> Agent` 复用同一套 session 心智模型

### 为什么不是继续用 NNG pattern

`NNG` 本身并不是错误的，问题在于它提供的是“消息模式抽象”，而我们需要的是“会话协议抽象”。

当前主链路继续依赖 `NNG` 的问题主要有：

- `REQ/REP` 不适合双向主动发请求
- 即使用 `PAIR` 等其他模式，session、mux、重连、背压语义仍然要自己补
- 最终复杂度落在我们自己实现的协议层，而不是 `NNG` 帮我们解决
- 既然协议层仍然要自己做，继续绑定 `NNG` 只会让模型更绕

结论不是“NNG 不能长连接”，而是“NNG pattern 不是我们当前主链路最合适的抽象”。

### 为什么不是 RSocket / libp2p / Aeron / HTTP2

这几个方向都能提供启发，但都不适合作为当前默认基础设施：

- `RSocket`
  - 优点是 session、双向、多路复用都很完整
  - 问题是协议层过重，header 和交互模型都超出当前项目需要
- `libp2p`
  - 优点是协议协商和 stream multiplexing 很成熟
  - 问题是它主要服务点对点网络与多节点覆盖网络，不是当前诉求
- `Aeron`
  - 优点是同机和低延迟场景很强，后续可作为本地优化参考
  - 问题是它更像高性能消息基础设施，不适合作为当前默认控制面
- `HTTP/2`
  - 优点是 stream/mux 心智模型值得参考
  - 问题是完整 HTTP 语义和头部体系并不是我们要的

因此最合适的落点是：

- 参考 `HTTP/2` 的“单连接多路复用”思想
- 保持比 `RSocket` 更轻
- 保持比 `Aeron` 更通用、更易接入
- 直接基于 `TCP` 自己实现最小 session 协议

## 目标架构

### 目标拓扑

```text
+----------------------+             +----------------------+
| Agent                |             | Server Node          |
|  - Session client    |             |  - Session manager   |
|  - Local dispatch    |  TCP(+TLS)  |  - Registry          |
|  - SDK/Game bridge   +-----------> |  - Invoke/Job/Ops    |
|                      |             |                      |
+----------------------+             +----------------------+
```

关键变化：

- `Agent` 主动连接 `Server`
- `Server` 持有 `Agent session`
- `Server -> Agent` 的请求全部走这条现有 session
- `Agent` 本地监听只留给本地接入方，不再暴露给 `Server`

### 会话语义

连接建立后，`Agent` 必须在该连接上完成会话注册：

1. `Agent` 建立 `tcp` 连接，按配置可附加 `tls`
2. `Agent` 发送 `AgentConnectRequest` 或等价注册消息
3. `Server` 返回 `session_id`、能力协商结果和告警信息
4. 双方开始 heartbeat
5. `Server` 后续在同一连接上向 `Agent` 下发 `Invoke/Job/Ops`
6. 连接断开后 session 立即失效
7. `Agent` 重连后必须重新注册，拿到新的 `session_id`

## 传输层设计

### 默认 transport

- 默认 transport: `tcp`
- 默认安全模式: `tls.disabled`
- `tls` 是 `tcp` 之上的安全配置，不单独定义为新 transport

### framing

沿用最小 framing：

```text
+--------------+------------------+-----------+
| FrameLength  | Croupier Header  | Body      |
| 4 bytes      | 8 bytes          | N bytes   |
+--------------+------------------+-----------+
```

约束：

- `FrameLength` 表示后续 `Header + Body` 总长度
- `Header` 继续复用现有 `Version + MsgID + RequestID`
- `Body` 为 protobuf 编码
- 单帧大小受双方配置的最大帧长限制

## 双向多路复用模型

### 基本原则

同一条 `Agent-Server` 连接允许双方主动发起请求：

- `Agent -> Server`
  - Connect/Register
  - Heartbeat
  - Capabilities update
  - Metrics / 状态上报
- `Server -> Agent`
  - Invoke
  - StartJob
  - CancelJob
  - Ops / control
  - Drain / shutdown

### 请求响应规则

- 请求使用奇数 `MsgID`
- 响应使用偶数 `MsgID`
- 双方各自维护本端 `RequestID` 递增计数器
- 响应必须回填原请求的 `RequestID`
- 同一连接上允许多个并发 in-flight 请求

这意味着 `Agent <-> Server` 不再是“单向 register/heartbeat + 另一条连接回拨”，而是一个真正双向复用的 session。

## 集群与路由

这是 `Agent <-> Server` 改造里最关键的差异点。

### 现状问题

当前 `Dispatcher` 可以绕过 `Agent -> Server` 的控制面 session，直接根据 `rpc_addr` 去拨 `Agent`。  
一旦改为 session 模型，这种做法必须废弃。

### 新模型

应引入“session 所属节点”概念：

- 每个 `Agent session` 归属于某个 `Server node`
- registry 必须记录：
  - `agent_id`
  - `session_id`
  - `server_node_id`
  - `expire_at`
  - 功能与 provider 摘要
- `Dispatcher` 选中 `Agent` 后，不再直拨 `Agent`
- 而是把请求路由到持有该 `session` 的 `Server node`
- 由该节点通过现有 session 下发给 `Agent`

### 路由后果

这样做之后：

- `rpc_addr` 不再是 `Server -> Agent` 的运行时依赖
- `Agent` 可以只有一条到某个 `Server` 节点的出站连接
- 集群内其他 `Server` 节点只需要知道“这个 session 在哪台节点上”

## Agent 本地监听的边界

重构完成后，`Agent` 本地监听地址只服务本地接入方：

- `GameServer -> Agent`
- `SDK -> Agent`
- 第三方本地应用 -> `Agent`

明确不再用于：

- `Server -> Agent` 回拨

因此以下概念应进入废弃状态：

- `rpc_addr` 作为 `Server -> Agent` 调用入口
- `Dispatcher` 直接拨 `Agent`
- 依赖 `Agent` 暴露控制面回调端口的模型

## 重连、背压与摘流

### 重连

建议默认：

- `enabled = true`
- `initial_delay_ms = 1000`
- `max_delay_ms = 30000`
- `backoff_multiplier = 2.0`
- `jitter_factor = 0.2`
- 上限后固定在廉价周期持续重试

### 背压

`Server` 应至少维护：

- 每个 agent session 的 `max_inflight_requests`
- 每个 agent session 的待处理队列上限
- 最大帧大小
- session 当前 `draining` 状态

### drain

`Server` 应支持对单个 `Agent session` 发出 `drain`：

- 停止向其分配新请求
- 允许已发请求在超时窗口内完成
- 超时后关闭连接并使 session 失效

## 协议与消息演进建议

当前 `agent/v1/register.proto` 仍带有明显的旧模型痕迹，例如：

- `rpc_addr`
- 将 `RegisterRequest` 仅视为控制面注册，而非 session 建立

后续建议：

1. 保留现有消息做过渡兼容
2. 明确其语义切换为“session connect/register”
3. 逐步引入更准确的命名，例如：
   - `AgentConnectRequest`
   - `AgentConnectResponse`
   - `AgentHeartbeatRequest`
   - `AgentDrainRequest`
4. 在过渡完成后废弃 `rpc_addr`

## 实施顺序

建议按以下顺序推进：

1. 冻结本文档
2. 在 `Server` 侧引入 `tcp session listener`
3. 在 `Agent` 侧引入 `tcp session client`
4. 让 `Register/Heartbeat` 跑在新 session 上
5. 将 `Invoke/StartJob/CancelJob/Ops` 改为走现有 session 下发
6. 在 registry 中增加 `server_node_id + session_id` 路由信息
7. 改造 `Dispatcher`，不再直拨 `Agent`
8. 清理 `rpc_addr` 直连依赖
9. 将 `NNG` 从默认主链路中移除

## 最终判断

对于当前项目，`Agent <-> Server` 的核心诉求不是：

- 多种 transport 大而全
- P2P 发现与打洞
- 极致本机低延迟消息总线

而是：

- 单连接
- 双向主动请求
- 多路复用
- 简单可控
- 易于排错
- 易于与 `SDK <-> Agent` 统一模型

因此 `Agent <-> Server` 更适合收敛为：

- **轻量 TCP session**
- **按需 TLS**
- **Server 持有 Agent session**
- **Invoke/Job/Ops 全部复用既有 session**
- **Agent 本地监听只服务本地接入方**

这比继续围绕 `NNG pattern` 修补当前主链路，更符合当前系统真实需求。
