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
  - 默认不启用 `tls`；Agent 侧可经 `agent.yaml` 的 `tls:` 段启用本地网关 TLS（TLS 在分帧之下，帧格式不变，SDK 拨入需带 TLS 拨号选项）
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

- `Version`：当前 `0x01`（接受窗口为 `[MinSupportedVersion, Version1]`，见下方版本策略）
- `MsgID`：24-bit 无符号，大端
- `RequestID`：32-bit 无符号，大端
- `Body`：protobuf 编码

## 版本策略

线路协议使用版本窗口而非单一固定字节，服务端升级不强迫每个长运行游戏进程同时发版。

- 当前版本：`0x01`（`protocol.Version1`）。
- 最旧接受版本：`0x01`（`protocol.MinSupportedVersion`）。这是 N-1 兼容下限；
  当 `Version1` 升版时，在过渡窗口内保留旧版本值。
- `Version` 在 `[MinSupportedVersion, Version1]` 之外时**逐请求拒绝**：接收方
  以 `InvokeResponse` 应答，payload 为
  `{"error":"unsupported protocol version: N","supportedVersions":[...]}`，
  并**保留会话**；不整体断开连接。
- 仅请求帧（`IsRequest`）得到拒绝响应；不兼容的响应/事件帧被跳过（没有可应答的请求）。
- `protocol.IsSupportedVersion` / `protocol.SupportedVersions` 向
  `internal/transport/tcp`（Agent/Server 入站）与 Go SDK 传输层暴露同一窗口。

### N-1 兼容与废止时间表

兼容窗口刻意窄但真实：平台部署 N 期间，跑上一版本线路协议（N-1）的对端继续工作。

**边界——窗口只在入站方向生效。** 每端出站帧一律写本端当前版本
（`protocol.Version1`），且 v1 线路没有可协商公共版本的握手。混合版本会话
因此只有单向可用：N 侧接受并应答 N-1 请求，但其响应（以 N 写出）落在
N-1 对端窗口之外、会被对端丢弃——拒绝响应同样以本端当前版本写出，也不例外。
在引入协商握手之前，升版要求游戏侧 SDK 至少与服务端同步滚动；窗口买到的是
入站路径上带 `supportedVersions` 诊断的优雅逐请求拒绝，而非透明的混合版本互操作。

| 阶段                | 策略                                                                                                                                              |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Version1` 升到 `N` | 设 `MinSupportedVersion = N-1`，在一个发布周期内继续接受旧版本                                                                                    |
| N-1 窗口期间        | 入站：不兼容帧逐请求拒绝、会话存活（`supportedVersions` 告知对端精确窗口）。出站：帧一律以本端当前版本写出，N-1 对端无法读取 N 侧响应——见上方边界 |
| 废止（窗口结束）    | 从窗口移除旧版本；旧对端此后收到逐请求拒绝                                                                                                        |
| 废止时间表          | 当前无废止（`MinSupportedVersion == Version1`）；N-1 策略适用于下一次升版，届时在此记录                                                           |

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

### 同步调用超时：metadata `timeoutMs` 约定

调用方声明的一次同步调用预算（毫秒，字符串十进制），放在 invoke 请求的
metadata map 中端到端传播，各跳取 **min(本跳配置, 声明值)** 生效（Go
context deadline 的天然 min 语义）：

| 层          | 行为                                                                                                    |
| ----------- | ------------------------------------------------------------------------------------------------------- |
| HTTP API    | 请求体 `timeoutMs`（可选）→ 注入 `metadata["timeoutMs"]`                                                |
| Server 派发 | `requestTimeoutBudget`：clamp [1000, 60000]，收紧 ctx；全局默认 15s 只作上限                            |
| Agent       | `providerCallDeadline`：clamp [1000, Agent 配置上限]，与默认（`agent.invokeTimeoutMs`，默认 15000）取小 |

- 垃圾值/缺失 → 各跳沿用自身默认，不报错（零侵入）
- 同步通道硬上限 60s：更长操作应走异步任务（事件流语义）
- 语义修正：Agent 侧旧实现硬编码 10s 与 Server 默认 15s 倒挂，现已对齐并可配

## 过载反馈与背压现状

按连接角色分层，当前实现状态如下（双车道已落地，见下节）：

| 路径                                                        | 机制                                                                                                                             | 背压语义                                                                                                                                                            |
| ----------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Server → Agent/Provider（`transport/tcp/server.go`）        | 双车道有界派发（CR-FIX-09）：业务车道=有界 worker 池（`MaxInflightRequests` 封顶），控制车道=专用队列+单 worker，逐请求 deadline | Fail-fast：业务队列满立即内联回 busy 错误帧（仅 Invoke）；控制帧永不拒绝；逐请求 deadline 约束慢 handler                                                            |
| SDK → Agent（各语言 SDK 入站，Go 基准 `tcp_client.go`）     | 有界 worker 池（默认 `NumCPU`，队列 `workers×4`）+ Go 控制车道                                                                   | fail-fast：业务队列满立即回 `inbound queue full, retry on another instance` 错误帧，原样回给调用方；内存不积累。busy 是终态——不自动换实例重试（见下方投递语义矩阵） |
| Agent↔Server、Agent↔Provider（`transport/tcp/mux_conn.go`） | **双车道有界派发**（`dispatchInbound`）：系统车道专用队列(64)+单 worker；业务车道 NumCPU worker + workers×4 有界队列             | 业务车道满则内联回 busy 错误帧（原样回给调用方，不自动换实例）；控制车道永不 reject（满则阻塞读循环=自然控制面反压）                                                |

已知边界：

- `overloaded` / `retry_after_ms` / `too_many_inflight` 等显式过载信号字段尚未定义 wire 消息；当前唯一过载信号是业务队列满时的错误 payload 文案
- 双车道已落地：Go（`MuxConn` + Go SDK）、Python（`transport/tcp.py` 控制车道单 worker，永不拒绝）、JS（`tcp_transport.ts` 控制队列 + 单并发 loop）。Java（`controlPool`）、C++（`control_pool_`）、C#（`_controlLimiter`）已于 2026-11 落地同款永不拒绝控制车道，六语言入站均按控制/业务车道分类

### 双车道：控制消息优先级

过载治理的一个结构性前提：**控制消息不能和业务消息共用车道**。
业务洪峰打满共享队列时，心跳被 fail-fast 拒绝 → 对端判定会话死亡 →
过载升级为连接雪崩；drain 摘流信号被淹没 → 过载时无法优雅下线。

因此入站派发按 MsgID 显式分类为双车道（`protocol.IsControlRequest()`，
显式集合而非裸字节前缀——0x06 集群族中 Hello 是控制、ForwardInvoke 是业务）：

| 车道         | 消息                                                                                        | 队列语义                                                |
| ------------ | ------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| 系统（控制） | `0x01xx` Register/Heartbeat、`0x05xx` Provider 会话控制（含 Drain）、`0x060101` ServerHello | 小专用队列，**永不 reject**                             |
| 业务         | `0x03xx` Invoke/Task、`0x04xx` Ops/Metrics、`0x060103` ForwardInvoke                        | 有界队列，满则 fail-fast 回错帧给调用方（不自动换实例） |
| 事件         | `TaskEvent`/`MetricEvent` 等单向帧                                                          | 读循环内联，不排队（现状保持）                          |

行为保证：

- 业务车道打满时，心跳/drain/register 照常可达——**会话存活与摘流不受业务过载影响**
- 系统车道自身也有界（防控制面被打爆），但容量独立且永不向对端回 reject
- fail-fast 的 reject 语义只作用于业务请求，`retry on another instance` 文案不变

实现位置：`MuxConn.dispatchInbound`（`internal/transport/tcp/mux_conn.go`，
Config `DispatchWorkers`/`BusinessQLen`/`ControlQLen`，默认 NumCPU / workers×4 / 64）
与 Go SDK `handleInboundRequest`（`sdks/go/pkg/croupier/transport/tcp_client.go`，
`ctrlInbox` 容量 64 + 单 worker）。测试：`mux_conn_dual_lane_test.go`、`dual_lane_test.go`。

## 投递语义矩阵

按消息类别给出平台当前的投递保证、幂等键与背压行为。dispatch 层
（`internal/platform/dispatch`）是唯一决定「失败能否换实例重试」的地方，
其规则刻意收窄：**仅 `errAgentUnreachable` 才 failover**——即投递可证明
未到达上一候选（无会话、转发亦失败）。凡完成了一次协议往返的响应——包括
busy 拒绝与应用层错误 payload——都原样回给调用方，**绝不自动换实例重试**。

原因是非幂等安全：「实例已饱和」的 busy 拒绝与「实例不可达」不是同一事件。
把一条变更型 Ops 命令静默重发给下一候选，等于打开隐式重复执行入口——命令
可能已在第一个实例上开始执行。重试是否安全由调用方依据自身的幂等方案决定，
busy 文案 `retry on another instance` 是给调用方的建议，不是平台行为的描述。

| 消息类别         | wire 消息                                                           | 投递语义                                                                                                        | 幂等键                                                                             | 业务队列饱和时                            |
| ---------------- | ------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ----------------------------------------- |
| 控制（会话）     | Register / Heartbeat / ProviderConnect / Drain、ServerHello         | 天然幂等；周期性重发即机制（at-least-once 安全）                                                                | 对端身份 + 消息类别；无业务 payload                                                | 永不拒绝——专用控制车道                    |
| 函数调用         | `InvokeRequest`                                                     | 每次派发尝试 at-most-once；不自动换实例重试                                                                     | 调用方 `metadata["idempotencyKey"]` 端到端透传；平台不去重——重放安全由契约作者负责 | busy 错误帧原样回给调用方                 |
| 异步任务启动     | `StartTaskRequest`                                                  | 每候选 at-most-once 投递（仅投递可证明未达上一候选才 failover）；每次尝试独立 `task_runs` 行，失败尝试标 failed | 服务端逐尝试生成 `taskId`；调用方 `metadata["taskId"]` 标记 drain 安全的任务延续   | 空 body 响应帧原样返回；不换实例          |
| 任务取消         | `CancelTaskRequest`                                                 | best-effort at-most-once，仅路由到归属 agent（不重选）                                                          | `taskId`                                                                           | 空 body 响应帧；结果仅以归属 agent 为权威 |
| 任务事件         | `TaskEvent`                                                         | at-least-once 读取：服务端事件存储赋每任务序号；消费方经 `StreamTaskAfterSeq` 重放并按其去重                    | 服务端每任务序号（`afterSeq`），非 wire 字段                                       | n/a（服务端事件存储，非入站队列）         |
| Ops / 变更型命令 | 以 invoke/task 请求派发到 Ops 角色函数（capture/synthetics/devops） | 默认 at-most-once；高风险操作在到达 dispatch 前另过平台 approval workflow（双人规则）门                         | approval 记录 + `taskId`                                                           | 与 invoke/task 相同：busy 绝不自动换实例  |

已知边界（明示而非隐藏）：

- wire 层无平台级去重：`metadata["idempotencyKey"]` 只是透传约定；业务侧 exactly-once 副作用仍需游戏方回执/去重作为独立契约
- `TaskEvent` 的排序是服务端存储序号，不是 wire 字段；单向帧本身不带 `seq`，事件消费方必须依赖 `StreamTaskAfterSeq` 重放而非帧内排序
- 非Invoke 业务消息的 busy 响应是空 body 帧（无错误 payload）；调用方只能凭响应完整性区分，而非过载标志位——显式 `overloaded` / `retry_after_ms` wire 字段仍未定义
- 钉住 busy-never-fails-over 规则的回归测试：`internal/platform/dispatch/dispatcher_delivery_semantics_test.go`

## Provider 握手鉴权

SDK → Agent 的 `ProviderConnectRequest` 在 `metadata` map 中以平台保留键
`authToken` 携带握手鉴权令牌。该键由 Agent 消费用于握手校验，绝不入会话
元数据（不可能泄漏到 SDK 分发页）；用户声明的同名键按保留键丢弃。

- SDK 侧：各语言 client 配置均有 `authToken` 字段；配置后连接帧携带
  `metadata["authToken"]`（trim 后）。六语言行为一致。
- Agent 侧：agent 配置 `agent.providerAuth.require` + `agent.providerAuth.token`。
  `require=false`（默认）完全忽略 token——握手行为与从前一致；`require=true`
  时缺 token 或 token 不匹配的握手被拒：连接关闭、不注册会话（fail-closed）。
  `require=true` 且 token 为空属无效配置，启动即报错——否则所有握手都会被拒。
- token 比较为常量时间比较；校验在两条连接路径（TCP 本地网关与 mux 路径
  handler）上都生效，且都在会话注册之前。

测试：agent 强制侧 `internal/agent/provider_auth_test.go`；各 SDK 发送侧
用例在各语言测试套件内。

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

实现状态（ARCH-016，2026-10-30）：

- Agent 在 `ProviderSession`（`agents/sidecar/provider_session.go`）上实现 `draining` 状态：`Draining`/`SetDraining`/`Inflight`/`AddInflight`。`callProvider`（`internal/agent/local_handler.go`）对新业务请求跳过 draining 会话；目标会话或全部函数匹配会话都在 drain 时显式拒绝 `provider <id> is draining (retry after <N>ms)`。任务延续（drain 前已接受的 `StartTask`）仍路由到其 draining provider——drain 不得截断已指派的工作。
- 共享 `LocalHandler.DrainProviderSession` 标记目标 draining、摘除其 agentlocal 实例（legacy 地址拨号不再路由），并等待在途归零或宽限到点；`ProviderDrainResponse` ACK 仅在该点之后返回。`grace` 默认 `defaultDrainGrace`（30s），`retry_after_ms` 优先。
- Server 侧路由门：`reg.AgentSession.Draining` 由 `opsNodeDrain`/`opsNodeUndrain` 经 `SetAgentDraining` 置/清；`Dispatcher.agentCanInvoke` 对 draining agent 返回 false，不再给该节点派新请求。`UpsertAgent` 在会话刷新时保留 `Draining`（心跳/重连不静默撤 drain）。
- undrain 用 `ProviderDrainRequest.reason` 哨兵 `__undrain__`（`protocol.ProviderDrainReasonUndrain`）表达，由 `opsNodeUndrain` 下发、两条 drain 路径都识别。

如实边界：

- 宽限到点后的强制关闭（约束 5）**未实现**：剩余请求不取消；仍在跑的任务经 ARCH-015 D3 `unknown`/终态事件路径收敛，drain ACK 在宽限到点时返回。
- undrain 只清 `draining` 标记，不重建 agentlocal 实例表；被 drain 的 provider 需重新注册或心跳刷新函数后，legacy 地址拨号路由才恢复。
- `reason == "__undrain__"` 哨兵是因为系统 protoc 3.21.12 产出坏 gencode 且无 34.x 可用；待 protoc 34.x 可用时以独立 `undrain` proto 字段跟进。

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
- `warnings`：非阻断告警字符串列表。当前两类来源——作用域漂移检测（provider 上报的 `game_id` / `env` 与 Agent 配置不一致时写入 `game_id mismatch` / `env mismatch`，便于控制台定位多服务共享 Agent 时的作用域错配），以及实例元数据保留键冲突（见下节，写入 `metadata key %q is reserved and dropped`）

### 实例元数据：`ProviderConnectRequest.metadata`

provider 注册时可随首帧携带**用户自定义多 KV 实例元数据**（如 `serverId`、`pod`），用于 SDK 版本分布页展示与按元数据搜索实例（对应 REST `GET /api/v1/providers/sdk-stats?metaKey=&metaValue=`）。

数据链路：

```
SDK 配置（Go InstanceMetadata / JS providerMetadata）
  → ProviderConnectRequest.metadata（map<string,string>, field 13）
  → Agent 合并：保留键冲突丢弃 + warnings 告警；空键跳过
  → agentlocal.Instance.Metadata
  → AgentProcess.metadata（field 11）随 RegisterRequest 上报 server
  → registry ProviderSession / ProviderSessionSnapshot
  → sdk-stats REST API（服务端子串过滤）→ web SDK 版本分布页
```

sdk-stats 实例明细（`SdkInstanceItem`）同时透出 `firstSeenUnix`（注册时间，#44；服务端归一：无观测值时回退 `lastSeenUnix`）与 `lastSeenUnix`。`firstSeenUnix` 为进程窗口语义——registry 会话是内存态，会话过期或 server 重启后从零重新累计（与下方 runtime-sources 的边界一致）。

规则：

- **保留键**：`sdkLanguage` / `sdkVersion` / `sdkName` / `protocol_version` / `gameId` / `env` 是平台固定字段（Agent 从请求固定字段生成）。用户元数据撞键时 Agent **丢弃该键并写入 warnings**，不得覆盖平台语义；空键跳过
- **只做观测，不做路由**：metadata 仅用于展示/搜索/诊断，负载均衡与函数路由不得读它（对应 proto 注释 "observability only, never routing"）
- 旧版 SDK 不发该字段完全兼容；未配置时 wire 上不发空 map（Go/JS 侧均为 nil/缺省）

存储与查询设计结论（持久化，#11 落地）：

- 在线 provider 会话是**易逝内存数据**（TTL 心跳维持）；实例元数据另以 **EAV 表持久化**（迁移 0033，`provider_metadata`）：`(game_id, env, service_id, meta_key, meta_value)` + `UNIQUE(game_id, env, service_id, meta_key)` + `INDEX(meta_key, meta_value)`——sqlite/mysql/postgres 三方言通用，不依赖任何 JSON 查询扩展。每次 agent 注册成功后整体刷新（upsert 当前值 + 清掉本次消失的 service 行；无变化的重复注册跳过写放大），会话过期清理时删除对应行；表行集合跟随在册会话，重启后下拉选项与聚合查询仍有源
- 聚合查询 = `GET /api/v1/providers/meta-options`（scoped 组）：服务端按当前游戏 scope 返回 distinct 键 → 值集合（含使用实例数，值按实例数降序），带 30s 进程内缓存、注册即失效——过滤下拉的选项由服务端提供（#2），前端不得从（已过滤的）列表数据推导
- 搜索实现 = 服务端对快照做内存线性过滤（`metaKey`/`metaValue` 子串、大小写不敏感，value 条件同时匹配键名与 `k=v` 整对）+ 前端实例关键字匹配，量级（单 agent 会话数）远不需要索引
- DB-less（内存 registry）退化：无 DB 时聚合从在线会话快照即时计算，重启丢失——见部署形态差异

历史别名如 `RegisterLocalRequest`、`RegisterLocalResponse`、`HeartbeatLocalRequest` 只属于兼容语义，不应再出现在新设计文档里。

`ProviderDrainRequest / Response` 的语义应统一为：

- `Agent` 不再向该 provider session 分配新请求
- provider 继续完成已接收的在途请求
- provider 回应 `ProviderDrainResponse` 仅表示“已接受 drain 状态”
- 真正关闭连接应发生在排空完成或宽限时间结束之后

### `0x04012D..0x040135` Devops 会话中继（AGENT-002）

面向面板的交互式 devops 终端会话（当前为 SSH），链路为 面板 → Server → devops
agent。下行请求走 agent 双向控制连接（`MuxConn`）；上行输出/收束帧是
**单向事件帧**（`IsEvent` 置位，reqID 为 0 —— 不回响应帧）：

|      MsgID | 名称                          | 方向         | 说明                                                       |
| ---------: | ----------------------------- | ------------ | ---------------------------------------------------------- |
| `0x04012D` | `DevopsSessionOpenRequest`    | server→agent | `operator` 为 server 鉴权后的身份（审计真值）              |
| `0x04012E` | `DevopsSessionOpenResponse`   | agent→server | `ok=false` + `error` 携带真实拒绝原因                      |
| `0x04012F` | `DevopsSessionInputRequest`   | server→agent | 键入；拒绝 `busy` = agent 输入队列已满                     |
| `0x040130` | `DevopsSessionInputResponse`  | agent→server | `ok=false` + `error="busy"` 映射为 REST `429 session_busy` |
| `0x040131` | `DevopsSessionResizeRequest`  | server→agent | pty 窗口变更（cols × rows）                                |
| `0x040132` | `DevopsSessionResizeResponse` | agent→server | —                                                          |
| `0x040133` | `DevopsSessionCloseRequest`   | server→agent | 面板主动关闭，`reason` 入审计                              |
| `0x040134` | `DevopsSessionCloseResponse`  | agent→server | —                                                          |
| `0x040135` | `DevopsSessionEvent`          | agent→server | 单向事件：`kind=output` 数据块（≤ CastChunk）或 `kind=end` |

契约不变量：

- **真值在 agent 侧**：白名单准入、审计先行门、asciicast 归档、输入背压
  全部在 agent 侧 Broker/Hub 链完成；中继不引入第二真值。open 请求中的
  `operator` 由 server 依据其鉴权会话填写，agent 审计链按原样落账 ——
  客户端永远不自我上报身份
- **`kind=end` 的 outcome 取值**：`closed`（正常关闭）、`eof`/`error`
  （通道拆除）、`cast-broken`（归档失败 —— 会话本身已运行）、以及
  `agent_disconnected`（控制面丢失时由 server 侧合成）
- **控制面丢失 = 会话面关闭**：控制连接断开时，agent 关闭全部在途会话
  （`OnConnClosed`），server 侧对称收束该 agent 的全部代理会话 ——
  失去可审计通道的面板不能继续持有交互式会话。重连后面板必须重新开会话；
  会话绝不跨连接存活
- **server 侧扇出有界**：回放环（最近 256 条事件）+ 每消费者有界队列；
  消费者跟不上即被摘除（流以 `overflow` end 帧收束）—— 会话本身、其归档
  与其他消费者均不受影响

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
- `traceId`
  - W3C trace 传播详见 `docs/architecture/sdk-otel-propagation.md`（SDK 只做传播不做导出）
- `gameId`
- `env`
- `timeoutMs`
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

- `Version` 不受支持时逐请求拒绝并保留会话（见版本策略）
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
