# SDK Wire Protocol

> **状态**：Current — 当前线协议权威规范，主仓库/Agent/各 SDK 共同遵循。

本文档定义 Croupier 当前 `shared session runtime` 之上的线协议约定，供主仓库、Agent 与各语言 SDK 共同遵循。

## 文档定位

本文档描述的是：

- framing
- header
- request/response 复用规则
- `subprotocol` 识别方式
- protobuf 协议层与 JSON payload 层的边界

本文档不再把历史 `gRPC` 或 `历史消息模式` 作为标准基线。

## shared session runtime

两条内部链路共享一套传输运行时：

- `Agent <-> Server`
- `SDK <-> Agent`

共享能力至少包括：

- `tcp`
- 可选 `tls`
- `4-byte frame length + 8-byte croupier header + protobuf body`
- 单连接双向 request/response
- 多个 in-flight 请求复用
- heartbeat
- reconnect
- drain
- backpressure

这个共享基座就是 `shared session runtime`。

## subprotocol

这里的 `subprotocol` 不是个性化配置，而是“跑在同一套 session runtime 上、但握手与业务语义不同的应用层子协议”。

当前主仓库有两套核心 `subprotocol`：

- `sdk-agent subprotocol`
  - 首帧必须是 `ProviderConnectRequest`
  - 默认不启用 `tls`
  - 面向 provider session
- `agent-server subprotocol`
  - 首帧必须是 `RegisterRequest`
  - 默认启用 `tls`
  - 面向 agent session

## v1 首帧识别规则

v1 不引入独立 `Magic`，而是直接用首条应用层消息识别子协议。

原因：

- 监听端口边界已经明确，不需要做多协议探测
- `Version + MsgID` 足以识别首帧是否合法
- 一旦 framing 损坏，最安全的策略是断开重连，而不是在坏流中继续猜边界

规则：

1. 先按 `FrameLength` 读完整首帧
2. 解析 8 字节 header
3. 校验 `Version`
4. 根据 `MsgID` 判断首帧是否符合当前监听边界
5. 解析 protobuf body
6. 任一条件不满足，立即关闭连接

## Frame 格式

```text
+--------------+------------------+-----------+
| FrameLength  | Croupier Header  | Body      |
| 4 bytes      | 8 bytes          | N bytes   |
+--------------+------------------+-----------+
```

约束：

- `FrameLength` 为大端无符号 32-bit
- `FrameLength` 表示后续 `Header + Body` 总长度
- 单帧长度必须受双方最大帧配置限制
- framing 只负责分帧，不负责业务协议探测

## Header 格式

当前沿用统一 8 字节头：

```text
+---------+------------+-----------------+
| Version | MsgID      | RequestID       |
| 1 byte  | 3 bytes    | 4 bytes         |
+---------+------------+-----------------+
```

约束：

- `Version`：当前固定 `0x01`
- `MsgID`：24-bit 无符号，大端
- `RequestID`：32-bit 无符号，大端
- `Body`：protobuf 编码

## 请求响应规则

统一规则：

- 奇数 `MsgID` 表示 request
- 偶数 `MsgID` 表示 response
- 响应必须回填原请求的 `RequestID`
- 每个主动方维护本端 `RequestID` 递增计数器
- 同一连接允许多个并发 in-flight 请求

说明：

- `responseMsgID = requestMsgID + 1` 仍是默认约定
- 像 `TaskEvent` 这样的单向事件消息不属于标准 request/response 配对

### 同步调用超时：metadata `timeout_ms` 约定

调用方声明的一次同步调用预算（毫秒，字符串十进制），放在 invoke 请求的
metadata map 中端到端传播，各跳取 **min(本跳配置, 声明值)** 生效（Go
context deadline 的天然 min 语义）：

| 层          | 行为                                                                                                    |
| ----------- | ------------------------------------------------------------------------------------------------------- |
| HTTP API    | 请求体 `timeoutMs`（可选）→ 注入 `metadata["timeout_ms"]`                                               |
| Server 派发 | `requestTimeoutBudget`：clamp [1000, 60000]，收紧 ctx；全局默认 15s 只作上限                            |
| Agent       | `providerCallDeadline`：clamp [1000, Agent 配置上限]，与默认（`agent.invokeTimeoutMs`，默认 15000）取小 |

- 垃圾值/缺失 → 各跳沿用自身默认，不报错（零侵入）
- 同步通道硬上限 60s：更长操作应走异步任务（事件流语义）
- 语义修正：Agent 侧旧实现硬编码 10s 与 Server 默认 15s 倒挂，现已对齐并可配

## 过载反馈与背压现状

按连接角色分层，当前实现状态如下（双车道已落地，见下节）：

| 路径                                                        | 机制                                                                                                                 | 背压语义                                                                                                                 |
| ----------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Server → Agent/Provider（`transport/tcp/server.go`）        | 读循环同步调 handler                                                                                                 | 天然背压：handler 慢则读停，TCP 窗口收紧端到端反压                                                                       |
| SDK → Agent（各语言 SDK 入站，Go 基准 `tcp_client.go`）     | 有界 worker 池（默认 `NumCPU`，队列 `workers×4`）+ Go 控制车道                                                       | fail-fast：业务队列满立即回 `inbound queue full, retry on another instance` 错误帧，Agent 侧 failover 换实例；内存不积累 |
| Agent↔Server、Agent↔Provider（`transport/tcp/mux_conn.go`） | **双车道有界派发**（`dispatchInbound`）：系统车道专用队列(64)+单 worker；业务车道 NumCPU worker + workers×4 有界队列 | 业务车道满则内联回 busy 错误帧（对端 failover）；控制车道永不 reject（满则阻塞读循环=自然控制面反压）                    |

已知边界：

- `overloaded` / `retry_after_ms` / `too_many_inflight` 等显式过载信号字段尚未定义 wire 消息；当前唯一过载信号是业务队列满时的错误 payload 文案
- 双车道目前落地于 Go 侧（`MuxConn` + Go SDK）；Python/Java/JS/C++/C# 的入站仍为单队列有界 worker 池（心跳与业务共队列），待按 Go 基准迁移

### 双车道：控制消息优先级

过载治理的一个结构性前提：**控制消息不能和业务消息共用车道**。
业务洪峰打满共享队列时，心跳被 fail-fast 拒绝 → 对端判定会话死亡 →
过载升级为连接雪崩；drain 摘流信号被淹没 → 过载时无法优雅下线。

因此入站派发按 MsgID 显式分类为双车道（`protocol.IsControlRequest()`，
显式集合而非裸字节前缀——0x06 集群族中 Hello 是控制、ForwardInvoke 是业务）：

| 车道         | 消息                                                                                        | 队列语义                                         |
| ------------ | ------------------------------------------------------------------------------------------- | ------------------------------------------------ |
| 系统（控制） | `0x01xx` Register/Heartbeat、`0x05xx` Provider 会话控制（含 Drain）、`0x060101` ServerHello | 小专用队列，**永不 reject**                      |
| 业务         | `0x03xx` Invoke/Task、`0x04xx` Ops/Metrics、`0x060103` ForwardInvoke                        | 有界队列，满则 fail-fast 回错帧（对端 failover） |
| 事件         | `TaskEvent`/`MetricEvent` 等单向帧                                                          | 读循环内联，不排队（现状保持）                   |

行为保证：

- 业务车道打满时，心跳/drain/register 照常可达——**会话存活与摘流不受业务过载影响**
- 系统车道自身也有界（防控制面被打爆），但容量独立且永不向对端回 reject
- fail-fast 的 reject 语义只作用于业务请求，`retry on another instance` 文案不变

实现位置：`MuxConn.dispatchInbound`（`internal/transport/tcp/mux_conn.go`，
Config `DispatchWorkers`/`BusinessQLen`/`ControlQLen`，默认 NumCPU / workers×4 / 64）
与 Go SDK `handleInboundRequest`（`sdks/go/pkg/croupier/transport/tcp_client.go`，
`ctrlInbox` 容量 64 + 单 worker）。测试：`mux_conn_dual_lane_test.go`、`dual_lane_test.go`。

## drain 语义

`drain` 是会话级的优雅摘流语义，不是立即断连语义。

统一定义：

- 不再向该 session 分配新的业务请求
- 允许已在途请求在宽限时间内继续完成
- 排空完成或宽限期结束后，再关闭连接或使 session 失效

最小行为约束：

1. session 进入 `draining` 状态
2. 新请求应被拒绝、重路由，或返回明确的 `draining` / `retry_after_ms`
3. 已在途请求可继续执行
4. heartbeat 继续保持
5. 宽限时间到达后，可强制关闭剩余请求

边界说明：

- `drain` 不是普通背压
- `drain` 不等于取消所有在途请求
- `drain` 期间仍可收发与会话治理相关的控制消息

## 消息族

### `0x01xx` Agent Session Control

用于 `agent-server subprotocol` 的会话控制消息，当前主 proto 命名仍为：

- `RegisterRequest`
- `RegisterResponse`
- `HeartbeatRequest`
- `HeartbeatResponse`
- `RegisterCapabilitiesRequest`
- `RegisterCapabilitiesResponse`

语义上，这一组已经应按“agent session connect/register”理解，而不是历史回拨模型。

其中 `agent-server subprotocol` 还应具备会话级 `drain` 能力：

- `Server` 可将某个 `Agent session` 置为 `draining`
- `Dispatcher` 不再向该 session 分配新请求
- 已在途请求在宽限时间内继续完成
- 排空完成后再关闭连接或移除 session

### `0x02xx` Client / Result 查询

保留给 client 注册与作业结果查询等 SDK 可见控制消息。

### `0x03xx` Invocation / Task

用于同步调用、异步任务与取消：

- `InvokeRequest`
- `InvokeResponse`
- `StartTaskRequest`
- `StartTaskResponse`
- `StreamTaskRequest`
- `TaskEvent`
- `CancelTaskRequest`
- `CancelTaskResponse`

### `0x04xx` Ops / Telemetry

用于系统信息、指标与运维控制消息族。

### `0x05xx` Provider Session

用于 `sdk-agent subprotocol` 的 provider session 控制消息：

|      MsgID | 名称                        | 说明                                      |
| ---------: | --------------------------- | ----------------------------------------- |
| `0x050101` | `ProviderConnectRequest`    | 建立 provider session                     |
| `0x050102` | `ProviderConnectResponse`   | 返回 `session_id`、协商结果与 `warnings`  |
| `0x050103` | `ProviderHeartbeatRequest`  | provider 心跳                             |
| `0x050104` | `ProviderHeartbeatResponse` | 心跳响应                                  |
| `0x050105` | `ProviderDrainRequest`      | Agent 将 provider session 置为 `draining` |
| `0x050106` | `ProviderDrainResponse`     | provider 确认进入 drain 状态              |

`ProviderConnectResponse` 字段语义：

- `session_id`：provider session 标识，后续心跳与 invoke 复用
- `accepted_capabilities`：Agent 接受的能力列表
- `warnings`：非阻断告警字符串列表。当前用于作用域漂移检测——provider 上报的 `game_id` / `env` 与 Agent 配置不一致时（仅双方都非空才校验），写入 `game_id mismatch` / `env mismatch`，便于控制台定位多服务共享 Agent 时的作用域错配。空值兼容：任一侧为空则跳过该项校验

历史别名如 `RegisterLocalRequest`、`RegisterLocalResponse`、`HeartbeatLocalRequest` 只属于兼容语义，不应再出现在新设计文档里。

`ProviderDrainRequest / Response` 的语义应统一为：

- `Agent` 不再向该 provider session 分配新请求
- provider 继续完成已接收的在途请求
- provider 回应 `ProviderDrainResponse` 仅表示“已接受 drain 状态”
- 真正关闭连接应发生在排空完成或宽限时间结束之后

## 业务 payload 规则

需要明确区分两层：

- 协议消息层
  - protobuf
  - 用于 session、路由、能力协商、作业控制
- 业务 payload 层
  - 默认 UTF-8 JSON
  - 一般承载在 `bytes payload`

v1 默认规则：

- `InvokeRequest.payload` 是 JSON bytes
- `InvokeResponse.payload` 是 JSON bytes
- `TaskEvent.payload` 是 JSON bytes
- SDK 用户不需要先定义 `.proto` 才能接入
- SDK 应默认提供原生对象与 JSON bytes 的自动编解码

## 字段分层规则

字段放在哪一层，不按“像不像业务字段”判断，而按“谁需要理解它”判断。

规则：

- Agent 需要理解、路由、治理或协商的字段，必须放在 protobuf 协议层
- 仅由具体业务函数消费的字段，放在 JSON payload 层
- 不允许把平台控制字段藏在 JSON payload 中要求 Agent 去猜

典型放 protobuf 的字段：

- `function_id`
- `session_id`
- `idempotency_key`
- `trace_id`
  - W3C trace 传播详见 `docs/architecture/sdk-otel-propagation.md`（SDK 只做传播不做导出）
- `game_id`
- `env`
- `timeout_ms`
- `priority`
- 能力协商、限流、重试、审计相关字段

典型放 JSON payload 的字段：

- `player_id`
- `ban_reason`
- `duration`
- `guild_id`
- `item_count`

## Schema 规则

`inputSchema` / `outputSchema`（对应 proto 字段名 `input_schema` / `output_schema`）在默认路径下描述的是 JSON payload 的 JSON Schema。

规则：

- schema 是可选增强项
- 没有 schema 时函数仍可注册和调用
- schema 主要用于校验、文档、UI 生成和调试提示
- v1 不做多 payload codec 协商

## 兼容性规则

- `Version` 不受支持时，直接断开连接
- 首帧不符合对应 `subprotocol` 时，直接断开连接
- protobuf body 不可解码时，直接断开连接
- 连接断开后旧 `session_id` 必须立即失效
- 重连后必须重新建 session，不能假设旧 session 自动恢复

## 明确废弃的旧概念

以下概念不应再作为新的协议实现依据：

- `LocalControlService` 作为主语义入口
- `RegisterLocalRequest` 作为主语义入口
- `rpc_addr`
- SDK 本地监听 server
- `Server -> Agent` 回拨模型
- “历史消息封装 over TCP 等于独立 TCP transport”的说法

## 相关文档

- [架构总览](./)
- [SDK-Agent 传输重构设计](./sdk-agent-transport-redesign.md)
- [Agent-Server TCP Session 重构设计](./agent-server-session-transport-redesign.md)
- [SDK 文档](../sdks/)
