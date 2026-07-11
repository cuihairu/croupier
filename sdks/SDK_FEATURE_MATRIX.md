# Croupier SDK 功能矩阵

本文件是各语言 SDK 的**单一事实来源**，定义"必备 / 可选 / 语言扩展"三层能力。
新增 SDK 或新能力时，先更新本表，再落到具体语言实现与 README。

参考协议基线：[`docs/architecture/sdk-wire-protocol.md`](../docs/architecture/sdk-wire-protocol.md)。

---

## 一、能力分层原则

| 层级 | 含义 | 跨语言一致性要求 |
| --- | --- | --- |
| **L1 Core Provider** | SDK 作为被调用方接入平台的最小集合 | **必须**，所有 SDK 一致实现 |
| **L2 Provider 扩展** | 增强注册/治理的可选项 | 可选，但若实现需遵循统一字段语义 |
| **L3 Invoker** | SDK 作为调用方的独立能力 | 独立模块、独立配置、独立示例 |
| **L4 语言/引擎扩展** | 与语言生态深度耦合的能力 | 不要求跨语言对齐，仅在所属 SDK 文档化 |

---

## 二、L1 Core Provider（必备）

所有 SDK **必须**实现以下 API 与语义：

### 2.1 生命周期 API

| 能力 | 说明 |
| --- | --- |
| `Client(config)` | 构造客户端，注入配置 |
| `registerFunction(descriptor, handler)` | 注册函数及其描述符 |
| `connect()` | TCP 拨号 + 发送 `ProviderConnectRequest` + 启动心跳 |
| `serve()` / `serveAsync()` | 持续接受入站调用直到 `stop` |
| `stop()` / `close()` | 优雅关闭：完成在途请求 → 发送 drain → 关闭连接 |

### 2.2 传输与会话

- 单条 TCP 长连接到 Agent（不监听本地端口）
- 4-byte FrameLength + 8-byte Header + protobuf body
- 首帧必须是 `ProviderConnectRequest`，回应 `ProviderConnectResponse`
- 多路复用请求/响应（按 `RequestID` 配对）
- 默认心跳（60s，可配置）
- 默认自动重连（指数退避 + jitter）

### 2.3 FunctionDescriptor（核心字段，所有 SDK 必须支持）

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | string | 是 | 函数 ID，例如 `player.ban` |
| `version` | string | 是 | 语义化版本 |

扩展字段（见 L2）：`tags` `summary` `description` `operation_id` `deprecated` `input_schema` `output_schema` `category` `risk` `entity` `operation`。

### 2.4 Handler 签名

`handler(context_metadata, payload) -> result`

- `context_metadata`：JSON 字符串（包含 trace、game_id、env、metadata 等平台控制字段）
- `payload`：UTF-8 JSON 字节序列
- `result`：UTF-8 JSON 字符串或字节序列

各语言本地化形态：

| 语言 | handler 签名 |
| --- | --- |
| Go | `func(ctx context.Context, payload []byte) ([]byte, error)` |
| Python | `Callable[[str, bytes], str \| bytes]` |
| Java | `FunctionHandler.handle(context: String, payload: byte[]) -> String` |
| JS/TS | `(context: string, payload: string) => Promise<string> \| string` |
| C++ | `std::function<std::string(const std::string&, const std::string&)>` |
| C# | `Func<string, string, Task<string>>` 或同步等价物 |

### 2.5 ClientConfig（核心字段）

| 字段（canonical snake_case） | 必填 | 说明 |
| --- | --- | --- |
| `agent_addr` | 是 | Agent TCP 地址 |
| `service_id` | 是 | Provider / 进程标识 |
| `service_version` | 否 | Provider 版本，默认 `1.0.0` |
| `game_id` | 否 | 游戏作用域标识（game scope） |
| `env` | 否 | `development` / `staging` / `production` |
| `insecure` | 否 | 是否跳过 TLS，默认 `true`（开发友好） |
| `auto_reconnect` | 否 | 默认 `true` |
| `heartbeat_interval_seconds` | 否 | 默认 60 |

> 字段在 Go/C# 用 PascalCase 导出字段、在 Java/JS 用 camelCase、在 Python/C++ 用 snake_case，这是**语言本地规范**，不是命名漂移。

---

## 三、L2 Provider 扩展（可选但语义统一）

| 能力 | 触发条件 | 统一字段/语义 |
| --- | --- | --- |
| JSON Schema 校验 | 描述符含 `input_schema` / `output_schema` | 默认 JSON Schema 格式，校验失败返回标准错误 |
| 平台 drain 处理 | Agent 发送 `ProviderDrainRequest` | 停止接收新请求 → 完成在途 → 返回 `ProviderDrainResponse` |
| 控制面 manifest 上传 | 配置 `control_addr` | 通过 `RegisterCapabilitiesRequest` 推送压缩 manifest |
| TLS | `insecure=false` | `ca_file` / `cert_file` / `key_file` / `server_name` |
| 鉴权 | 配置 `auth_token` | Bearer token，附加到握手 metadata |
| 文件传输 | `enable_file_transfer=true` | 受白名单（`allowed_extensions` / `allowed_mime_types`）与上限（`max_file_size`）约束 |

---

## 四、L3 Invoker（独立调用方能力）

当 SDK 同时提供调用方时，**必须**独立成模块，禁止与 Provider Client 共享配置入口。

统一最小 API：

| 能力 | 说明 |
| --- | --- |
| `Invoker(config)` | 独立构造，使用 HTTP/gRPC 调 Server，不再走 Provider session |
| `invoke(function_id, payload, options)` | 同步调用，返回 payload 或错误 |
| `startTask(function_id, payload, options)` | 异步作业，返回 `task_id` |
| `streamTask(task_id)` | 流式订阅 `TaskEvent` |
| `cancelTask(task_id)` | 取消运行中作业 |

当前实现状态：

| SDK | Invoker 实现 |
| --- | --- |
| Go | ✅ `pkg/croupier/invoker.go` |
| Python | ✅ `croupier/invoker.py` |
| Java | ✅ `invoker/Invoker.java` |
| JS | ✅ `src/invoker.ts` |
| C++ | ✅ `CroupierInvoker` |
| C# | ✅ `CroupierInvoker` |

---

## 五、L4 语言/引擎扩展（不要求跨语言对齐）

仅在所属 SDK 内文档化，不进入跨语言一致性矩阵：

| SDK | 扩展能力 | 入口 |
| --- | --- | --- |
| C++ | VirtualObject 注册 | `RegisterVirtualObject` |
| C++ | Component 注册 | `RegisterComponent` / `LoadComponentFromFile` |
| C++ | 动态插件 | `plugin/dynamic_loader` |
| C++ | Lua / Sol2 绑定 | `bindings/lua_binding_sol2` |
| C++ | Skynet 集成 | `skynet/` |
| C# | DI 集成 | `Extensions/ServiceCollectionExtensions` |
| C# | Unity 适配 | `Unity/CroupierUnityBehaviour` |
| Java | Spring Boot starter | `spring-boot-starter/` |

---

## 六、README 描述规范

所有 SDK 的 README **必须**：

1. 描述传输层时使用"`sdk-agent subprotocol` 单条 TCP session 客户端"措辞，**不再**写"两层注册系统"、"LocalControlService"、"control.proto"。
2. "核心特性"段落按 L1 能力清单展开，扩展能力单列"可选能力"或"语言扩展"。
3. 涉及 Invoker 必须显式标注是"独立调用方能力"，禁止与 Provider 主流程混在同一个 quick start 里。
4. 字段示例保持本地语言命名规范（Go/C# PascalCase、Java/JS camelCase、Python/C++ snake_case），但**语义**对应本表 `ClientConfig` 字段。
5. 链接到本文件作为功能对照单一事实来源。

---

## 七、各语言 L1 API 表面映射

> 命名风格遵循各语言本地规范；矩阵校验只要求**符号存在**，不要求命名一致。

| L1 能力 | Go | Python | Java | JS/TS | C++ | C# |
| --- | --- | --- | --- | --- | --- | --- |
| 构造 | `NewClient` | `CroupierClient` | `CroupierSDK.createClient` | `createClient` | `CroupierClient` ctor | `CroupierClient` ctor |
| 注册 | `RegisterFunction` | `register_function` | `registerFunction` | `registerFunction` | `RegisterFunction` | `RegisterFunction` |
| 建立会话 | `Connect` | `connect` | `connect` | `connect` | `Connect` | `ConnectAsync` |
| 服务循环 | `Serve` | `connect` 内联 | `serve` / `serveAsync` | `serve` / `serveAsync` | `Serve` | `ServeAsync` |
| 停止 | `Stop` | `disconnect` | `stop` | `disconnect` | `Stop` | `Stop` |
| 关闭 | `Close` | `disconnect` | `close` | `disconnect` | `Close` | `Close` |

### L1 配置字段映射

| 字段（canonical snake_case） | Go | Python | Java | JS | C++ | C# |
| --- | --- | --- | --- | --- | --- | --- |
| `agent_addr` | `AgentAddr` | `agent_addr` | `agentAddr` | `agentAddr` | `agent_addr` | `AgentAddr` |
| `service_id` | `ServiceID` | `service_id` | `serviceId` | `serviceId` | `service_id` | `ServiceId` |
| `service_version` | `ServiceVersion` | `service_version` | `serviceVersion` | `serviceVersion` | `service_version` | `ServiceVersion` |
| `game_id` | `GameID` | `game_id` | `gameId` | `gameId` | `game_id` | `GameId` |
| `env` | `Env` | `env` | `env` | `env` | `env` | `Env` |
| `insecure` | `Insecure` | `insecure` | `insecure` | `insecure` | `insecure` | `Insecure` |
| `auto_reconnect` | `Reconnect.Enabled` | `auto_reconnect` | `reconnect` | `autoReconnect` | `auto_reconnect` | `AutoReconnect` |
| `heartbeat_interval_seconds` | `HeartbeatInterval` | `heartbeat_interval` | `heartbeatInterval` | `heartbeatIntervalSeconds` | `heartbeat_interval` | `HeartbeatIntervalSeconds` |

### L3 Invoker 能力映射

> 命名统一为 `task` 系列（`startTask` / `streamTask` / `cancelTask`），与 `proto/croupier/sdk/v1/invocation.proto` 对齐。
> 本阶段不以向后兼容为约束：`Job` 系列旧命名已删除，不留 deprecated 别名。

| SDK | 入口文件 | 同步调用 | 异步任务 | 流式事件 | 取消 |
| --- | --- | --- | --- | --- | --- |
| Go | `pkg/croupier/invoker.go` | `Invoker.Invoke` | `StartTask` | `StreamTask` | `CancelTask` |
| Python | `croupier/invoker.py` | `Invoker.invoke` | `start_task` | `stream_task` | `cancel_task` |
| Java | `invoker/Invoker.java` | `invoke` | `startTask` | `streamTask` | `cancelTask` |
| JS | `src/invoker.ts` | `invoke` | `startTask` | `streamTask` | `cancelTask` |
| C++ | `CroupierInvoker` | `Invoke` | `StartTask` | `StreamTask` | `CancelTask` |
| C# | `CroupierInvoker` | `InvokeAsync` | `StartTaskAsync` | `StreamTaskAsync` | `CancelTaskAsync` |

**命名规则**：
- 所有 SDK 必须使用 `Task` 系列，禁止保留 `Job` 系列入口。
- `scripts/check-sdk-matrix.sh` 的 wire 检查将任何 `Job` 命名视为失败（仅 allowlist 内的协议别名模块除外，且这些模块也已迁移为纯 `Task`）。

---

## 八、版本与变更管理

- 协议层变更：先改 `proto/croupier/sdk/v1/*.proto` + `docs/architecture/sdk-wire-protocol.md`，再更新本表。
- 能力新增：先在本表登记所属层级，再落地到 SDK 实现与 README。
- 跨 SDK 一致性回归：CI 中按本表自动校验各 SDK 是否暴露对应 API（脚本：`scripts/check-sdk-matrix.sh`）。
