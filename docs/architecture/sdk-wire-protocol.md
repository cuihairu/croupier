# SDK Wire Protocol

本文档定义 Croupier Server、Agent、各语言 SDK 之间共享的 SDK 线协议标准。

目标：

- 将协议标准收敛到 `croupier` 主仓库
- 让 Agent、Server、所有 SDK 以同一份文档为准
- 明确区分“NNG over TCP”与“独立的 TCP transport”
- 为未来新增 transport 和能力协商提供稳定基线

## 文档定位

本文档是 SDK 传输与消息语义的规范源。

- 协议格式、MsgID、兼容性规则，以本文档为准
- 各 SDK 当前实现状态，以 `docs/sdks/sdk-parity-matrix.md` 为准
- 语言级 API 风格、行为约束，以 `docs/sdk/specification.md` 为准

各 SDK 仓库不应复制本协议正文，只应引用主仓库文档。

## 当前结论

当前系统的真实状态是：

- 主链路已经从 gRPC 迁移到 NNG REQ/REP 协议
- `agent` 已经支持 `tcp://`、`ipc://`，本质仍是 NNG transport
- “支持 TCP”如果只是指 `tcp://127.0.0.1:19090`，那当前已经支持
- “支持 TCP”如果指不依赖 NNG 运行时、直接通过自定义 TCP framing 通信，则当前尚未实现

因此后续设计必须显式区分两类 transport：

- `nng`
  - 协议编码相同
  - 底层依赖 NNG 运行时/绑定
  - 地址样式通常为 `tcp://...`、`ipc://...`
- `tcp`
  - 仍使用同一份 Croupier message header + protobuf body
  - 但底层不依赖 NNG，而是自定义裸 TCP framing
  - 地址样式应单独定义，例如 `tcp://host:port`

注意：`NNG over TCP` 不等于独立的 `TCP transport`。

## 标准分层

SDK 通信标准建议拆成三层：

1. Wire protocol
   - 消息头
   - MsgID
   - 请求/响应匹配
   - 错误表示
2. Transport abstraction
   - `nng`
   - `tcp`
   - 未来可能的 `ws`、`quic`
3. Capability contract
   - Client 注册
   - Invoker 调用
   - Job 生命周期
   - Heartbeat
   - Local function hosting

只要 transport 能承载同一份 wire protocol，SDK 对上层 API 的行为就应保持一致。

## Header 格式

当前各 SDK 和 Go 服务端基本一致，消息头为 8 字节：

```text
+---------+------------+-----------------+
| Version | MsgID      | RequestID       |
| 1 byte  | 3 bytes    | 4 bytes         |
+---------+------------+-----------------+
```

约束：

- `Version`：当前固定为 `0x01`
- `MsgID`：24-bit，无符号，大端
- `RequestID`：32-bit，无符号，大端
- Body：protobuf 序列化字节

## 请求响应规则

当前规则：

- 奇数 `MsgID` 表示 request
- 偶数 `MsgID` 表示 response
- 特殊事件消息如 `JobEvent` 不属于 request/response 配对
- 响应消息的 `RequestID` 必须回填原请求的 `RequestID`
- 通用规则为 `responseMsgID = requestMsgID + 1`

## MsgID 标准

以下内容以主仓库与大多数 SDK 的当前实现为基准。

### 0x01xx ControlService

| MsgID | 名称 |
|------:|------|
| `0x010101` | RegisterRequest |
| `0x010102` | RegisterResponse |
| `0x010103` | HeartbeatRequest |
| `0x010104` | HeartbeatResponse |
| `0x010105` | RegisterCapabilitiesRequest |
| `0x010106` | RegisterCapabilitiesResponse |

### 0x02xx ClientService

| MsgID | 名称 |
|------:|------|
| `0x020101` | RegisterClientRequest |
| `0x020102` | RegisterClientResponse |
| `0x020103` | ClientHeartbeatRequest |
| `0x020104` | ClientHeartbeatResponse |
| `0x020105` | ListClientsRequest |
| `0x020106` | ListClientsResponse |
| `0x020107` | GetJobResultRequest |
| `0x020108` | GetJobResultResponse |

### 0x03xx InvokerService

| MsgID | 名称 |
|------:|------|
| `0x030101` | InvokeRequest |
| `0x030102` | InvokeResponse |
| `0x030103` | StartJobRequest |
| `0x030104` | StartJobResponse |
| `0x030105` | StreamJobRequest |
| `0x030106` | JobEvent |
| `0x030107` | CancelJobRequest |
| `0x030108` | CancelJobResponse |

### 0x04xx OpsService

这一组目前并非所有 SDK 都实现，但主仓库、Python、C++、JS 等实现中已出现：

| MsgID | 名称 |
|------:|------|
| `0x040101` | GetSystemInfoRequest |
| `0x040102` | GetSystemInfoResponse |
| `0x040103` | ListProcessesRequest |
| `0x040104` | ListProcessesResponse |
| `0x040105` | ReportMetricsRequest |
| `0x040106` | ReportMetricsResponse |
| `0x040107` | StreamMetricsRequest |
| `0x040108` | MetricEvent |

扩展消息在不同实现中还包含：

- RestartProcess
- StopProcess
- StartProcess
- ExecuteCommand
- ListServices
- GetServiceStatus

这些扩展消息尚未在所有 SDK 完全对齐，后续应以主仓库 proto/handler 实际支持范围为准统一收口。

### 0x05xx LocalControlService

| MsgID | 名称 |
|------:|------|
| `0x050101` | RegisterLocalRequest |
| `0x050102` | RegisterLocalResponse |
| `0x050103` | HeartbeatLocalRequest |
| `0x050104` | HeartbeatLocalResponse |
| `0x050105` | ListLocalRequest |
| `0x050106` | ListLocalResponse |

## 当前协议偏差

这是当前代码里已经能看到的不一致点：

### 主仓库 `internal/transport/nng/protocol`

主仓库该实现当前只覆盖到：

- `0x01xx` 基础注册/心跳
- `0x02xx` 基础 client register/heartbeat
- `0x03xx` invoker 基础调用

缺少：

- `0x05xx LocalControlService`
- `0x04xx OpsService`
- `0x02xx` 中部分扩展消息

而 Go SDK、Python、JS、C++、C# 中的协议常量范围明显更完整。后续应先把主仓库协议定义补齐，再要求 SDK 跟进。

### Java SDK

Java 已有 `Protocol` 和 `NNGTransport`，但调用层仍未真正接入 transport，导致协议常量存在但主能力不可用。

### C# 协议面

C# 当前协议常量覆盖面比 Java 更接近可用状态，但仍未完整覆盖 0x04xx 扩展。

## Transport 抽象标准

后续所有 SDK 应统一暴露 transport 概念，而不是把 NNG 硬编码在高层 API 中。

建议标准字段：

```yaml
transport:
  kind: nng   # nng | tcp
  address: tcp://127.0.0.1:19090
  connect_timeout_ms: 5000
  request_timeout_ms: 30000
  tls:
    enabled: false
```

或在语言对象中表达为：

- `transportKind`
- `address`
- `connectTimeout`
- `requestTimeout`
- `tls`

### `nng` transport

要求：

- 使用相同 8 字节 header + protobuf body
- 支持 `tcp://` 和本地可用时的 `ipc://`
- 允许多地址回退
- 允许自动重连配置

### `tcp` transport

建议新增，但要注意这不是“把现有 NNG 地址换个名字”。

要求：

- 不依赖 NNG 原生库或语言绑定
- 使用独立 framing，建议：
  - 固定前缀魔数
  - 版本号
  - 总帧长度
  - 8 字节 Croupier header
  - protobuf body
- 支持半包/粘包解析
- 支持超时、连接复用、心跳与关闭语义

推荐 framing：

```text
+---------+---------+--------------+------------------+-----------+
| Magic   | Version | FrameLength  | Croupier Header  | Body      |
| 4 bytes | 1 byte  | 4 bytes      | 8 bytes          | N bytes   |
+---------+---------+--------------+------------------+-----------+
```

推荐魔数示例：`CRP1`

## 为什么要加独立 TCP transport

当前 NNG 的主要问题不是协议头，而是各语言绑定质量不一致：

- Java 依赖 JNA 手写 native binding，维护成本高
- C# 通过反射适配 `nng.NET`，可用但脆弱
- JS 依赖第三方 `@rustup/nng`
- Python 依赖 `pynng`
- C++ 和 Go 更接近原生生态，质量相对更稳

如果希望所有 SDK 都默认可用，独立 `tcp` transport 的收益很明确：

- Java/C#/JS/Python 不再受 NNG 本地库分发影响
- 默认安装路径更简单
- 测试和 CI 更容易稳定
- 更容易做协议抓包和调试

因此建议：

- `agent` 与 `server` 继续保留 `nng`
- 新增标准 `tcp` transport
- SDK 默认优先使用 `tcp`
- 本地高性能/本地 IPC 场景保留 `nng` 作为可选 transport

## 默认策略建议

建议后续统一为：

- 默认 transport：`tcp`
- 可选 transport：`nng`
- 本地同机部署且明确安装了 NNG 时，可手动切换到 `nng`

推荐配置示例：

```yaml
sdk:
  transport:
    kind: tcp
    address: tcp://127.0.0.1:19090
```

本地优化场景：

```yaml
sdk:
  transport:
    kind: nng
    address: ipc:///tmp/croupier-agent.sock
```

## Capability 协商

为避免“协议常量存在但功能并未真正实现”的情况，后续应引入能力协商。

建议在注册或心跳时显式上报：

- `supported_transports`
  - `nng`
  - `tcp`
- `supported_capabilities`
  - `invoke`
  - `start_job`
  - `stream_job`
  - `cancel_job`
  - `register_local`
  - `heartbeat_local`
  - `list_local`
  - `ops_read`
  - `ops_control`
- `sdk_language`
- `sdk_version`
- `protocol_version`

这样 server/agent 可以：

- 拒绝不支持必需能力的 SDK
- 按 transport 能力做回退
- 将不对齐变成显式信号，而不是运行时踩坑

## 兼容性规则

后续协议演进应遵守：

1. `Version` 不变时：
   - 不能重定义既有 `MsgID`
   - 不能改变 header 长度
   - protobuf 字段只能做向后兼容扩展
2. 新增消息：
   - 新增未使用 `MsgID`
   - 更新主仓库协议文档
   - 更新 parity matrix
3. transport 新增：
   - 不改变业务语义
   - 同一 `MsgID` 对应同一 protobuf 请求/响应
4. 删除能力：
   - 先标记 deprecated
   - 至少经历一个稳定版本周期

## 实施优先级

建议按这个顺序推进：

1. 主仓库补齐协议常量与文档
2. 定义统一 transport 抽象
3. 为 `agent` 和 `server` 增加独立 `tcp` transport
4. Go SDK 先实现 `tcp`，作为参考实现
5. Python / JS / C# / Java 跟进 `tcp`
6. 将默认 transport 切换为 `tcp`
7. 保留 `nng` 作为高性能可选项

## 本文档维护规则

任何以下改动都必须先更新本文档：

- 新增或修改 `MsgID`
- 新增 transport 类型
- 修改 framing
- 修改请求/响应语义
- 引入新的能力协商字段

协议变更完成后，必须同步更新：

- `docs/sdks/sdk-parity-matrix.md`
- 相关 SDK README 或配置文档
