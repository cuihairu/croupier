# SDK 对齐矩阵

本文档是使用者文档入口。跨语言功能点的单一事实来源是源码目录中的
[`sdks/SDK_FEATURE_MATRIX.md`](https://github.com/cuihairu/croupier/blob/main/sdks/SDK_FEATURE_MATRIX.md)；本文只摘录关键基线和当前必须避免误解的能力状态。

不要把 Server 侧 OpenAPI Source 上传能力等同于每个 SDK 都支持本地 OpenAPI 解析。当前只有 Go SDK 提供并验证了 `RegisterFromOpenAPI` 等价 helper；JS/TS、Python、Java、C#、C++ 只能按 Descriptor v2 字段注册函数，不能在文档或示例中宣称已支持本地 OpenAPI helper。

## 评估口径

统一使用以下状态：

- `Required`: 所有 SDK 必须支持
- `Optional`: 可选增强项，但语义必须一致
- `Forbidden`: 新设计中禁止继续实现或继续宣传

## 统一基线

| Capability                                           | 基线        |
| ---------------------------------------------------- | ----------- |
| SDK 主动连接 Agent 本地 gateway                      | `Required`  |
| 单条长连接复用注册、心跳、调用、作业控制             | `Required`  |
| `ProviderConnectRequest` 作为首帧                    | `Required`  |
| 默认 transport 为独立 `tcp session`                  | `Required`  |
| `tls` 作为 `tcp` 的可选安全配置                      | `Required`  |
| `InvokeRequest/Response/TaskEvent` 默认 JSON payload | `Required`  |
| `input_schema/output_schema` 描述 JSON Schema        | `Required`  |
| 自动重连 + 指数退避 + 上限后廉价周期重试             | `Required`  |
| drain / overload / backpressure 语义                 | `Required`  |
| JSON payload 自动编解码                              | `Required`  |
| 多地址回退                                           | `Optional`  |
| IPC / 本地专用优化 transport                         | `Optional`  |
| mTLS                                                 | `Optional`  |
| SDK 本地监听端口                                     | `Forbidden` |
| `rpc_addr` / 回拨式注册                              | `Forbidden` |
| SDK 侧 `NNGServer`                                   | `Forbidden` |
| 以 `gRPC` 作为 SDK 默认主链路                        | `Forbidden` |

## Descriptor v2 / OpenAPI Helper 状态

| SDK | Descriptor v2 字段 | 本地 OpenAPI helper | 说明 |
| --- | --- | --- | --- |
| Go | 已接入 | 已实现并可验证 | 当前基准实现 |
| JS/TS | 待验收 | 未实现 | 不得宣称支持本地 OpenAPI 解析 |
| Python | 待验收 | 未实现 | 不得宣称支持本地 OpenAPI 解析 |
| Java | 待验收 | 未实现 | 不得宣称支持本地 OpenAPI 解析 |
| C# | 待验收 | 未实现 | 生成 proto 已含字段，手写 API/示例仍需验收 |
| C++ | 待验收 | 未实现 | 不得宣称支持本地 OpenAPI 解析 |

OpenAPI helper 只能解析函数能力契约字段：`operationId/tags/summary/description/requestBody/responses` 和 `x-resource/x-operation/x-capability/x-execution/x-approval/x-risk/x-enabled/x-permission`。遇到页面 schema、组件树、菜单、路由、页面分类、显示文案、页面 mapping 或布局 DSL 必须报错或产生 diagnostics。

## 协议边界

所有 SDK 必须遵守以下分层：

### 协议层

- protobuf message
- 8 字节 header
- request / response 对应规则
- `ProviderConnectRequest` / `ProviderHeartbeatRequest`
- `InvokeRequest` / `StartTaskRequest` / `CancelTaskRequest`

### 业务 payload 层

- 默认固定为 UTF-8 JSON
- SDK 负责把语言原生对象编码为 JSON
- Agent 不默认解析业务 JSON 内部结构

## 统一配置面

所有 SDK 建议至少暴露以下统一配置语义：

| 字段                           | 说明                     |
| ------------------------------ | ------------------------ |
| `address`                      | Agent 本地 gateway 地址  |
| `connectTimeoutMs`             | 建连超时                 |
| `requestTimeoutMs`             | 单请求超时               |
| `tls.enabled`                  | 是否启用 TLS             |
| `tls.caFile`                   | CA 文件                  |
| `tls.certFile`                 | 客户端证书               |
| `tls.keyFile`                  | 客户端私钥               |
| `tls.serverName`               | TLS SNI / 校验名         |
| `tls.insecureSkipVerify`       | 仅开发环境跳过校验       |
| `heartbeat.intervalMs`         | 心跳间隔                 |
| `reconnect.enabled`            | 自动重连                 |
| `reconnect.initialDelayMs`     | 初始退避                 |
| `reconnect.maxDelayMs`         | 最大退避                 |
| `reconnect.multiplier`         | 退避倍率                 |
| `reconnect.jitter`             | 抖动比例                 |
| `reconnect.steadyStateDelayMs` | 到达上限后的廉价重试周期 |

## 命名要求

为了减少跨语言认知成本，建议统一术语：

- `shared session runtime`
- `sdk-agent subprotocol`
- `provider session`
- `payload`
- `drain`
- `backpressure`

不建议继续把新的实现文档写成：

- `LocalControl`
- `RegisterLocal`
- `local_listen`
- `rpc_addr`
- `session client/server`

## 为什么这里不再维护另一份详细状态表

过去那种静态矩阵有两个问题：

1. 很快过期
2. 会把历史实现细节误写成目标能力

主仓库现在更适合维护：

- 统一协议与配置基线
- 统一术语
- 统一禁止项

详细功能状态以 [`sdks/SDK_FEATURE_MATRIX.md`](https://github.com/cuihairu/croupier/blob/main/sdks/SDK_FEATURE_MATRIX.md) 为准；每个 SDK 的 README、测试矩阵和 CI 结果只能补充语言本地使用说明，不能覆盖该矩阵。

## SDK 仓库

- Go: `croupier-sdk-go`
- C++: `croupier-sdk-cpp`
- Java: `croupier-sdk-java`
- JS/TS: `croupier-sdk-js`
- Python: `croupier-sdk-python`
- C#: `croupier-sdk-csharp`

## 相关文档

- [SDK 概览](../sdks/)
- [SDK-Agent 传输重构设计](../architecture/sdk-agent-transport-redesign.md)
- [SDK Wire Protocol](../architecture/sdk-wire-protocol.md)
