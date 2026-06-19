# SDK-Agent 传输重构设计

## 状态

- 状态: Accepted Target Design
- 适用范围: `SDK <-> Agent`
- 不在范围内: `Agent <-> Server`

## 设计结论

本设计用于统一后续 SDK 接入方向，核心结论如下：

1. SDK 与 Agent 的默认传输从 SDK 侧 `旧传输` 迁移为独立的 `tcp` transport，并按需启用 `tls`。
2. SDK 不再开启本地监听端口，不再向 Agent 暴露 `rpc_addr` / `local_listen`。
3. SDK 与 Agent 之间采用 **单条由 SDK 主动发起的长连接**，在同一连接上完成注册、心跳、函数调用、作业控制和响应回传。
4. `Agent <-> Server` 链路不在本次重构实现范围内，但设计目标已统一到共享 session runtime。
5. SDK 侧旧的 `NNGServer` / `LocalControl` / “本地 server 回调” 模型进入废弃状态。

## shared session runtime 与 subprotocol

这里需要明确两个术语：

- `shared session runtime`
  - 指两条主链路共享的传输与会话基座
  - 包括 `tcp/tls + framing + request/response mux + reconnect + heartbeat + drain`
- `subprotocol`
  - 指运行在该基座上的具体应用层子协议
  - 对 `SDK <-> Agent` 而言，就是 `sdk-agent subprotocol`

`sdk-agent subprotocol` 的关键特征是：

- 首帧必须是 `ProviderConnectRequest`
- 默认不启用 `tls`
- 面向 provider session，而不是回拨地址注册

## 背景

当前多语言 SDK 在 SDK-Agent 连接层存在以下问题：

- Java / C# / JS / Python 对 旧传输 绑定质量和本地运行时分发高度敏感。
- SDK 作为被业务程序嵌入的依赖时，引入原生 旧传输 运行时会增加接入复杂度、产物体积和 CI 不稳定性。
- 当前仓库仍残留“SDK 启动本地监听端口，由 Agent 回调 SDK”的历史设计，这与当前目标不一致。
- 对 SDK 使用者而言，最稳定的接入模型应是:
  - SDK 主动连接 Agent
  - SDK 不暴露监听端口
  - Agent 只维护与 SDK 的会话和调度

本设计不讨论 Agent 与 Server 之间的交互协议，只收敛 SDK 与 Agent 的边界。

## 范围

### In Scope

- SDK 与 Agent 之间的连接模型
- SDK 与 Agent 之间的 transport 类型
- SDK Provider / Invoker 在 SDK-Agent 边界的消息承载方式
- 重连、心跳、背压、过载处理
- SDK 配置面统一建议

### Out Of Scope

- Agent 与 Server 的通信协议
- Server 侧函数路由、作业调度和控制面协议
- Dashboard / HTTP API / gRPC API
- 各语言 SDK 的具体代码实现细节

## 目标架构

### 目标拓扑

```text
+----------------------+             +----------------------+
| Business Process     |             | Agent                |
|  - Game Server       |             |  - Session manager   |
|  - Embedded SDK      |             |  - Dispatch / queue  |
|                      |             |  - Backpressure      |
|  SDK opens one       | TCP (+TLS)  |                      |
|  outbound connection +-----------> |  accepts SDK session |
|  and keeps it alive  |             |                      |
+----------------------+             +----------------------+
```

### 关键要求

- SDK 只能主动发起连接，不能要求业务程序开放监听端口。
- Agent 必须支持在同一条连接上双向收发请求与响应。
- SDK 与 Agent 必须支持连接断开后的自动重连与重新注册。
- SDK 与 Agent 必须支持过载反馈和背压控制。

## 传输层设计

### 默认 transport

- 默认 transport: `tcp`
- 默认安全模式: `tls.optional`
- `tls` 为可选增强项，而不是默认必选项

### TLS 策略

SDK 嵌入业务进程的典型场景以同机、同机房、同内网为主，默认不强制要求 `tls` 更符合接入成本最小化原则。

推荐规则：

- 默认使用明文 `tcp`
- 跨主机、跨网段、零信任网络或有合规要求时启用 `tls`
- 需要双向身份校验时启用 `mTLS`
- `tls` 是 `tcp` transport 的安全配置，不单独定义为新的 transport kind

### 非目标

本设计的默认路径不再依赖 SDK 侧 `旧传输`。如后续出于本地高性能场景保留 `旧传输`，也只能作为非默认、明确 opt-in 的兼容选项，不能继续作为 SDK 默认依赖。

### TCP 信封协议（Envelope v1）

独立 TCP transport 必须实现自己的帧边界，不能直接把 protobuf 裸写入 socket。

v1 信封格式：

```text
+--------------+------------------+-----------+
| FrameLength  | Croupier Header  | Body      |
| 4 bytes      | 8 bytes          | N bytes   |
+--------------+------------------+-----------+
```

约束：

- `FrameLength`：大端无符号 32-bit，表示后续 `Croupier Header + Body` 的总长度，不包含自身 4 字节
- `Croupier Header`：继续沿用现有 8 字节业务头
- `Body`：protobuf 序列化字节
- 单帧长度必须受 Agent 与 SDK 双方配置的最大帧大小限制

这个设计刻意保持最小化：

- 不引入额外 `Magic`
- 不引入独立的 transport version 字段
- framing 只负责分帧，不负责协议探测

### 首条消息识别

SDK-Agent 的 `tcp` listener 是专用边界，不承担多协议复用职责；如启用 `tls`，其外层只是 `tcp` 连接上的安全封装。  
因此 v1 通过“首条应用层消息必须为 provider connect”完成协议识别，而不是依赖额外魔数。

连接建立后的规则：

1. Agent 先按 `FrameLength` 读取完整首帧
2. 解析 `Croupier Header`
3. 校验首条消息必须为 `ProviderConnectRequest`（建议 `MsgID = 0x050101`）
4. 校验 header `Version` 是否受支持，并校验 protobuf body 可解码
5. 任一条件不满足时，Agent 直接关闭连接，并记录 `protocol_mismatch`、`unsupported_protocol_version` 或等价原因

设计约束：

- 首条消息不是注册消息，直接断开
- framing 非法、长度越界、消息体不可解码，直接断开
- 不尝试在同一连接上重同步后续帧
- 不在同一端口上做“也许是别的协议”的探测式兼容

### 为什么 v1 不使用 Magic

`Magic` 不是完全没有价值，但在当前边界里收益很低，不应作为 v1 默认规则。

`Magic` 通常解决的问题是：

- 一个 listener 上复用多种协议，需要快速识别
- 希望在抓包时更容易肉眼定位帧起点
- 出现流损坏后尝试重新同步

而 SDK-Agent v1 不满足这些前提：

- 这是独立的 SDK-Agent 专用 listener，不是多协议复用端口
- 首条 `ProviderConnectRequest` 已经能完成协议识别
- `Croupier Header.Version` 已经承担 wire 版本演进语义
- 一旦 framing 损坏，最简单且最安全的策略就是关闭连接并重连，而不是在损坏流中继续猜测边界

因此 v1 采用：

- 最小长度前缀 framing
- 首条注册消息识别协议
- 不识别就断开

如果未来确实出现“单端口复用多协议”或“需要 transport preface”的场景，再通过新的 envelope/profile 版本显式引入，而不是现在预埋复杂度。

### 业务头保持不变

SDK-Agent transport 重构不要求推翻现有业务头。业务头继续沿用：

```text
+---------+------------+-----------------+
| Version | MsgID      | RequestID       |
| 1 byte  | 3 bytes    | 4 bytes         |
+---------+------------+-----------------+
```

这样可以最大限度复用现有协议常量、请求响应匹配规则和 protobuf 消息体。

## 双向连接模型

### 总体原则

同一条 SDK-Agent 长连接允许双向发起请求：

- SDK -> Agent
  - 建连注册
  - 心跳
  - 能力上报
  - 调用请求
  - 作业流读取 / 取消请求
- Agent -> SDK
  - 函数调用请求
  - 作业控制请求
  - 降载 / drain / 关闭通知

### 请求响应规则

- 请求仍使用奇数 `MsgID`
- 响应仍使用偶数 `MsgID`
- 同一连接上的每个主动方各自维护本端 `RequestID` 递增计数器
- 响应必须回填原请求的 `RequestID`
- SDK 与 Agent 都必须支持并发 in-flight 请求

### 历史设计废弃

以下概念在 SDK-Agent 目标架构中应被废弃：

- `local_listen`
- `local_addr`
- `rpc_addr`
- SDK 本地 `NNGServer` / `RequestServer`
- “Agent 通过回调本地监听地址访问 SDK” 的模型

兼容期内可以保留字段和 API 外壳，但其语义必须标记为 deprecated，不能再作为新实现的基础。

## 注册与会话模型

### 新语义

SDK 与 Agent 建连后，应在该连接上建立 provider session，而不是注册一个“可回调地址”。

推荐会话流程：

1. SDK 打开 `tcp` 连接，按配置可附加 `tls`
2. SDK 发送会话注册请求
3. Agent 返回 `session_id`
4. SDK 启动心跳
5. Agent 基于该连接直接向 SDK 下发调用请求
6. 断线后 session 失效，SDK 重连后重新注册

### MsgID 方向

当前 `0x05xx LocalControlService` 是历史语义，后续实现时应改为“连接内 provider session 语义”。

推荐保留 `0x05xx` 分组，但重定义为：

| MsgID | 建议名称 | 说明 |
|------:|-----------|------|
| `0x050101` | `ProviderConnectRequest` | SDK 建立 provider session |
| `0x050102` | `ProviderConnectResponse` | 返回 `session_id` / 能力协商结果 |
| `0x050103` | `ProviderHeartbeatRequest` | SDK 心跳 |
| `0x050104` | `ProviderHeartbeatResponse` | Agent 心跳响应 |
| `0x050105` | `ProviderDrainRequest` | Agent 主动要求 SDK 停止接新请求 |
| `0x050106` | `ProviderDrainResponse` | SDK 确认 drain 状态 |

说明：

- 这是目标设计建议，不是当前代码状态说明。
- 具体 protobuf 消息定义和最终 MsgID 分配，应在实现前同步回写 `docs/architecture/sdk-wire-protocol.md`。

## Agent 实现要求

### 边界职责

Agent 需要新增并维护 SDK-Agent 专用的 `tcp` listener，并按配置支持 `tls` 握手，但这只作用于 `SDK <-> Agent` 边界，不影响 `Agent <-> Server` 现有链路。

### 连接与会话

Agent 至少需要实现：

- 接受 SDK 主动建立的长连接
- 为每条连接建立独立 provider session
- 将 `session_id` 与底层连接生命周期绑定
- 连接断开时立即使该 `session_id` 失效
- SDK 重连后要求重新发送 `ProviderConnectRequest`

### 收包与发包

Agent 至少需要实现：

- 一个基于 `FrameLength` 的读循环
- 一个按连接维度隔离的写队列
- in-flight 请求跟踪与超时回收
- 单连接内双向 request/response 复用
- 最大帧大小、最大并发、最大排队长度控制

### 首帧校验

Agent 必须把首帧校验作为协议入口：

- 首帧必须是 `ProviderConnectRequest`
- `Version` 不受支持则拒绝
- `MsgID` 不匹配则拒绝
- protobuf body 不可解码则拒绝
- 失败后直接关闭连接，不进入业务分发

### 运行期控制

Agent 至少需要实现：

- 心跳超时检测
- drain / overload 反馈
- 连接关闭前的有限度 drain
- 旧 session 上未完成请求的失效处理
- SDK 重连后的新旧 session 隔离

其中 `drain` 的统一语义是：

- `Agent` 将 provider session 标记为 `draining`
- 不再向该 session 分配新的 `Invoke / Task`
- 已在途请求允许在宽限时间内继续完成
- provider 保持 heartbeat 与必要控制消息处理
- 排空完成或宽限时间结束后，再关闭连接或使 session 失效

### 明确不做的事

本次实现不要求：

- 修改 `Agent <-> Server` 传输协议
- 让 SDK 开启本地监听端口
- 为历史 `LocalControl` 回调模型继续扩展新语义

## 调用与作业语义

### 保持不变的部分

`0x03xx InvokerService` 的业务语义应尽量保持不变：

- `InvokeRequest` / `InvokeResponse`
- `StartTaskRequest` / `StartTaskResponse`
- `StreamTaskRequest` / `TaskEvent`
- `CancelTaskRequest` / `CancelTaskResponse`

### 变化点

变化不在消息体，而在消息传输方向：

- 旧模型: Agent 连接 SDK 本地监听地址
- 新模型: Agent 直接在现有 SDK session 连接上向 SDK 发请求

因此协议迁移重点应放在 transport 层和 session 管理层，而不是重新发明一套函数调用模型。

### 平台协议与业务负载解耦

需要明确区分两层数据：

- 平台协议消息
  - 例如 `ProviderConnectRequest`、`InvokeRequest`、`InvokeResponse`、`TaskEvent`
  - 这层继续使用 protobuf，便于跨语言统一、版本演进和固定头部路由
- 用户业务 payload
  - 例如 `InvokeRequest.payload`、`InvokeResponse.payload`、`TaskEvent.payload`
  - 这层不应强制要求 SDK 用户定义 protobuf schema

设计结论：

- protobuf 只用于 Croupier 平台协议
- SDK 用户的业务 payload 固定使用 UTF-8 JSON
- SDK 必须提供“原生对象/结构体 <-> JSON bytes”的默认自动编解码
- 用户接入 SDK 时，不需要先定义 `.proto`
- 凡是 Agent 需要理解、路由、治理或协商的字段，不得藏在 JSON payload 里

### 字段边界判定

判断一个字段该放在 protobuf 协议层还是 JSON payload 层，按下面规则执行：

- Agent 必须理解这个字段才能完成路由、会话、鉴权、背压、重试、幂等、超时、审计或能力协商
  - 放 protobuf
- 这个字段是跨函数通用字段，而不是某个具体业务函数私有字段
  - 放 protobuf
- 这个字段只对具体函数实现有意义，Agent 只需要转发，不需要理解
  - 放 JSON payload

可以直接收敛成一句规则：

- 凡是 Agent 需要理解的字段，不得藏在 JSON payload 里

### 正反例

应放 protobuf / header / metadata 的字段：

- `function_id`
- `request_id`
- `idempotency_key`
- `session_id`
- `trace_id`
- `game_id`
- `env`
- `timeout_ms`
- `retry` / `priority` 这类平台治理字段

应留在 JSON payload 的字段：

- `player_id`
- `ban_reason`
- `duration`
- `guild_id`
- `item_count`
- 其他只被具体业务 handler 消费的参数

### 默认 payload 规则

SDK-Agent v1 的默认业务负载规则为：

- `InvokeRequest.payload` 默认承载 UTF-8 JSON 字节
- `InvokeResponse.payload` 默认承载 UTF-8 JSON 字节
- `TaskEvent.payload` 默认承载 UTF-8 JSON 字节
- Agent 负责转发和调度，不默认解析业务字段语义
- SDK 负责把语言原生对象编码为 JSON，并在回调侧解码为语言原生对象

这样设计的原因是：

- SDK 的主要接入场景是嵌入现有业务进程，而不是先建立完整 protobuf 工程体系
- JSON 是所有目标语言最稳定、零额外依赖的共同分母
- 绝大多数 SDK 用户更关心“传对象就能调”，而不是先生成多语言 proto 代码
- Agent 在 SDK-Agent 边界的职责应是连接、会话、背压、调度，而不是绑定用户业务模型

### Schema 规则

`input_schema` / `output_schema` 应定义为可选增强项，而不是默认前置条件。

v1 约束：

- 业务 payload 固定为 `json`
- 当函数提供 schema 时，默认使用 JSON Schema 描述 JSON payload
- 没有 schema 时，函数仍然可以注册和调用
- schema 的作用主要是文档、校验、UI 生成和调试提示
- 不能把“缺少 schema”视为 SDK 不可用

### 固定 JSON 的原因

v1 不实现多 codec 业务负载协商。

也就是说：

- 业务 payload 只支持 JSON
- 不定义 `payload_codec`
- 不做 `default_payload_codec` 能力协商
- 不把 protobuf/自定义二进制作为 v1 业务负载选项
- 如果未来确实需要其他业务负载格式，必须通过新的协议版本显式引入，而不是在 v1 中保留隐式分支

## 重连策略

### 目标

重连既要足够快地恢复常见 Agent 重启场景，也要避免惊群和高频空转。

### 推荐默认值

- `enabled = true`
- `max_attempts = 0`
  - `0` 表示无限重试
- `initial_delay_ms = 1000`
- `max_delay_ms = 30000`
- `backoff_multiplier = 2.0`
- `jitter_factor = 0.2`

### 推荐节奏

建议节奏：

- 可选快速重试阶段: `0` 到 `1` 次，延迟 `200ms`
- 常规指数退避阶段: `1s -> 2s -> 4s -> 8s -> 15s -> 30s`
- 达到上限后:
  - 固定以 `30s` 持续重试
  - 允许实现层将上限放宽到 `60s`
  - **不建议默认使用 10 分钟上限**

### 为什么不建议 10 分钟

对于 SDK-Agent 场景，Agent 通常与业务进程距离很近：

- 同机
- 同机房
- 同局域网

此时 10 分钟上限会显著拉长 Agent 重启后的恢复时间，收益远小于成本。需要控制的是惊群和日志噪音，而不是故意拖慢恢复。

### 退避重置

满足以下任一条件后应重置重连退避计数：

- 连接连续稳定存活 `60s`
- 连续 `3` 次 heartbeat 成功

### 错误分类

#### 可重试错误

- TCP 连接断开
- 连接超时
- TLS 握手超时
- Agent 主动关闭连接
- Agent 返回可重试过载信号

#### 不可重试错误

- 认证失败
- 协议版本不兼容
- 必需能力不被支持
- 明确的配置错误

不可重试错误必须立即终止自动重连，并向上层暴露确定性的错误原因。

## 背压与过载控制

### 原则

背压的主责任在 Agent，不应把复杂调度策略散落到每个语言 SDK 中。

### Agent 侧职责

Agent 应至少维护以下控制项：

- 每个 SDK session 的 `max_inflight_requests`
- 每个 SDK session 的待处理队列上限
- 单消息 / 单帧大小上限
- 连接级 drain 状态
- 过载时的拒绝与降载策略

### SDK 侧职责

SDK 只需暴露最小必要配置：

- 本地 handler 并发上限
- 本地待处理队列上限
- 队列溢出策略
  - `reject`
  - `drop_oldest`
  - `caller_backoff`

### 过载反馈

Agent 需要提供显式反馈，而不是只靠 SDK 盲猜：

- `overloaded`
- `retry_after_ms`
- `draining`
- `too_many_inflight`

SDK 收到这些信号后，可以：

- 暂停发送新请求
- 推迟重连
- 降低本地并发
- 向业务层暴露“Agent 正在过载保护中”

## SDK API 面建议

### 推荐配置字段

所有 SDK 建议统一收敛到以下字段语义：

| Field | 说明 |
|---|---|
| `transport.kind` | 固定优先为 `tcp` |
| `transport.address` | Agent 地址 |
| `transport.connect_timeout_ms` | 连接超时 |
| `transport.request_timeout_ms` | 请求超时 |
| `transport.tls` | TLS 配置 |
| `reconnect.enabled` | 是否自动重连 |
| `reconnect.initial_delay_ms` | 初始退避 |
| `reconnect.max_delay_ms` | 最大退避 |
| `reconnect.backoff_multiplier` | 指数退避倍率 |
| `reconnect.jitter_factor` | 抖动因子 |
| `backpressure.max_concurrency` | 本地并发上限 |
| `backpressure.max_queue_size` | 本地排队上限 |

### 废弃字段

后续实现中应废弃以下字段：

- `local_listen`
- `local_addr`
- `rpc_addr`
- `ipcAddress`（如果其唯一用途是 SDK 本地监听）

### 生命周期 API

推荐长期 API：

- `connect()`
- `close()`
- `registerFunction()`
- `registerFunctions()`

兼容期 API：

- `serve()` / `Serve()`
  - 可以暂时保留
  - 但不得再表示“启动本地监听 server”
  - 只能作为“阻塞等待连接生命周期结束”的兼容包装

## 能力协商

Provider session 建立时，SDK 应显式上报：

- `sdk_language`
- `sdk_version`
- `protocol_version`
- `supported_capabilities`
  - `provider_session`
  - `invoke`
  - `start_job`
  - `stream_job`
  - `cancel_job`
- `supported_transport`
  - `tcp`
- `transport_security_mode`
  - `plaintext`
  - `tls`

Agent 可以据此：

- 拒绝不满足要求的 SDK
- 根据能力关闭不支持的调用路径
- 将不对齐从“运行时踩坑”改为“连接期显式失败”

## 迁移顺序

建议实施顺序：

1. 在主仓库冻结本设计文档
2. 更新 `docs/architecture/sdk-wire-protocol.md`
   - 明确 `0x05xx` 从 `LocalControl` 迁移到 `ProviderSession`
3. 在 Agent 上实现 SDK-Agent 专用独立 `tcp` transport，并按配置支持 `tls`
4. 先以 Go 作为参考实现，验证单连接双向收发
5. 逐步迁移 Python / JS / C# / Java / C++
6. 清理 SDK 中的本地监听、`rpc_addr`、`NNGServer` 残留
7. 将 SDK 默认 transport 切换为 `tcp`

## 对现有文档的影响

本设计落地后，以下内容需要同步清理或改写：

- `docs/architecture/sdk-wire-protocol.md`
  - 删除或重写 `LocalControlService` 历史语义
- `docs/sdks/sdk-parity-matrix.md`
  - 不再把“是否支持本地 server / LocalControl”作为目标能力
- 各语言 SDK README / docs
  - 删除 `local_listen`、本地监听端口、`gRPC` / `NNGServer` 回调式描述

## 最终判断

对于“把 SDK 嵌进其他用户程序”的场景，SDK 侧继续依赖 旧传输 的收益已经明显低于成本。  
后续 SDK-Agent 方向应明确收敛为：

- **单连接**
- **纯 TCP，按需 TLS**
- **SDK 不监听端口**
- **Agent 负责 session、背压与调度**
- **Agent-Server 与 SDK-Agent 共享同一套 session runtime 心智模型**
