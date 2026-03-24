# SDK Parity Matrix

本文档记录各语言 SDK 与主仓库协议标准之间的当前对齐情况。

规范来源：

- 协议标准：`docs/architecture/sdk-wire-protocol.md`
- 行为规范：`docs/sdk/specification.md`

状态说明：

- `Yes`: 已实现且链路已接通
- `Partial`: 有部分代码或协议常量，但行为未完全可用
- `No`: 当前未实现或不可用

## 当前总体判断

截至当前代码状态：

- Go 是最接近完整参考实现的 SDK
- C# 基本可用，但 transport 依赖质量仍弱于 Go/C++
- Python 和 JS 在 NNG 层面可用，但对原生依赖敏感
- C++ transport 本身较完整，但高层实现仍混有大量占位逻辑
- Java 当前最不对齐，主要问题不是协议常量，而是调用层仍有明显未接通路径

## 能力矩阵

| Capability | Go | JS/TS | Python | Java | C# | C++ |
|---|---|---|---|---|---|---|
| 协议 header 8B | Yes | Yes | Yes | Yes | Yes | Yes |
| 基础 MsgID 常量 | Yes | Yes | Yes | Partial | Partial | Yes |
| LocalControl 0x05xx | Yes | Yes | Yes | Partial | Yes | Partial |
| Ops 0x04xx 常量 | Partial | Yes | Yes | No | No | Yes |
| NNG client transport | Yes | Yes | Yes | Partial | Yes | Yes |
| NNG server transport | Yes | Yes | Yes | No | Yes | Yes |
| Client register local | Yes | Yes | Yes | Partial | Yes | Partial |
| Local heartbeat | Yes | Yes | Yes | No | Yes | Partial |
| 同步 invoke | Yes | Yes | Yes | No | Yes | Partial |
| StartJob | Yes | Partial | Yes | No | Yes | Partial |
| StreamJob | Partial | Partial | Partial | Partial | Partial | Partial |
| CancelJob | Yes | Yes | Yes | No | Yes | Partial |
| 自动重连 | Yes | Partial | Partial | Partial | Partial | Partial |
| 重试策略 | Yes | Partial | Partial | Partial | Partial | Partial |
| Schema 校验 | Yes | Yes | Partial | Partial | Partial | Partial |
| 多地址 / IPC 回退 | Yes | No | No | No | No | No |
| 可选 TLS 配置 | Partial | No | No | Partial | No | Partial |
| 独立 TCP transport | No | No | No | No | No | No |

## 按语言分析

## Go SDK

现状：

- 拥有较完整的 transport 抽象，而不是单纯把 NNG 写死在调用器里
- 支持多地址、IPC 优先、`tls+tcp://` 归一化
- Client 与 Invoker 链路都已经接入 transport
- 协议常量覆盖最完整的一批

问题：

- 主仓库协议定义反而比 Go SDK 常量覆盖更少，存在“SDK 比规范更完整”的倒挂
- `StreamJob` 语义是否真正与服务端对齐，仍需端到端确认
- 当前仍是 NNG transport，不是独立 TCP transport

结论：

- 适合作为未来 `tcp` transport 的首个参考实现

## JS/TS SDK

现状：

- 有独立 `protocol.ts` 与 `transport.ts`
- 支持基础 invoke、register local、heartbeat
- transport/server 结构清晰

问题：

- 严重依赖 `@rustup/nng`
- 多地址回退、IPC 优先、TLS 抽象没有像 Go 那样系统化
- Job streaming 更像“有消息定义”，不等于真正稳定的事件流能力

结论：

- 协议层清晰，但默认可用性受 NNG 依赖影响

## Python SDK

现状：

- `protocol.py` 覆盖面较大
- `pynng` transport 基本清晰
- invoker 基础 invoke/start_job/cancel_job 已接入

问题：

- 依赖 `pynng`
- reconnect/retry 结构存在，但完整性需要进一步核实
- `stream_job` 仍更接近轮询/占位能力，不应视为完全对齐
- Client hosting 能力虽然存在，但需要进一步与 LocalControl 真正支持范围核实

结论：

- 比 Java 强很多，但仍未达到“跨平台默认稳可用”

## Java SDK

现状：

- 已有 `Protocol`
- 已有 `NNGTransport`
- 已有配置对象和测试骨架

关键缺口：

- `InvokerImpl.connect()` 仍是 TODO
- `invoke()` 直接返回占位 JSON
- `startJob()` / `cancelJob()` 仍是模拟逻辑
- `CroupierClientImpl.connect()` 也仍是 TODO

结论：

- Java 当前不是“质量稍差”，而是功能未真正接通
- 在 parity 视角里，Java 应标记为 `Partial/No`，不能按接口存在就算对齐

## C# SDK

现状：

- `CroupierClient` 已接入 `NNGTransport`
- `CroupierInvoker` 已直接通过 transport 发 `InvokeRequest`、`StartJobRequest`、`CancelJobRequest`
- 有 Local heartbeat
- 有独立 `NNGServer`

问题：

- transport 通过反射加载 `nng.NET`，运行时稳定性受包版本和本地环境影响
- 协议覆盖没有 Go/Python/C++ 那么完整
- `GetJobStatusAsync()` 用 `StreamJobRequest -> JobEvent` 的方式近似实现，语义仍需统一

结论：

- 在“链路是否可用”上明显好于 Java
- 在“实现稳健性”上仍弱于 Go/C++

## C++ SDK

现状：

- `NNGTransport` / `NNGServer` 本身实现比较完整
- 协议常量覆盖广
- 有较多测试

问题：

- 高层 `croupier_client.cpp` 里仍有大量 TODO
- 代码同时混有 HTTP/JSON 占位路径
- Client / Invoker 对真实协议链路的接通程度不如 transport 层看上去那么完整
- `StartJob` / `StreamJob` / `CancelJob` 高层能力仍有明显未完成实现

结论：

- C++ 的“传输层质量”高于“整体 SDK 完成度”
- 不应仅凭 transport 文件质量判断 SDK 已完全对齐

## 对齐优先级

建议按以下顺序修正：

1. 主仓库先成为协议真源
2. Java 打通真实 transport 链路
3. 明确 C++ 高层 HTTP 占位路径的去留
4. 为所有 SDK 引入统一 transport 抽象
5. 实现独立 `tcp` transport
6. 将默认 transport 切换到 `tcp`

## 强制对齐项

后续各 SDK 必须对齐以下项目：

- 相同 MsgID 集合
- 相同 header 编码
- 相同 request/response 对应关系
- 相同 Job 事件语义
- 相同 Local registration 流程
- 相同 transport 配置字段命名语义
- 相同错误映射原则

## 建议的统一配置面

所有 SDK 建议统一支持：

| Field | 说明 |
|---|---|
| `transportKind` | `nng` 或 `tcp` |
| `address` | 主地址 |
| `addresses` | 多地址回退 |
| `ipcAddress` | 本地 IPC 首选地址 |
| `connectTimeoutMs` | 连接超时 |
| `requestTimeoutMs` | 请求超时 |
| `reconnect` | 自动重连策略 |
| `retry` | 调用重试策略 |
| `tls` | TLS 配置 |

## 结论

如果目标是“所有语言默认都稳定可用”，当前最大的结构性问题不是 MsgID 本身，而是：

- transport 实现质量强依赖各语言的 NNG 绑定质量
- SDK 高层接入深度不一致
- 主仓库尚未成为唯一协议源

因此后续最重要的事情不是继续给每个语言分别补 NNG 细节，而是：

- 先固定主仓库协议文档
- 再抽象统一 transport
- 再补一个独立 `tcp` transport 作为默认实现
