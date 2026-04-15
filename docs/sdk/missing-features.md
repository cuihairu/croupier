# Croupier SDK 重构执行清单

更新时间：2026-04-14
状态：SDK 文档基线已更新，代码与各语言仓库待逐个落地

## 文档入口

- `docs/sdk/specification.md`
- `docs/sdk/go-checklist.md`
- `docs/sdk/python-checklist.md`
- `docs/sdk/js-ts-checklist.md`
- `docs/sdk/csharp-checklist.md`
- `docs/sdk/java-checklist.md`
- `docs/sdk/cpp-checklist.md`
- `docs/architecture/sdk-wire-protocol.md`
- `docs/architecture/sdk-agent-transport-redesign.md`
- `docs/sdks/sdk-parity-matrix.md`

## 目标

SDK 侧这轮重构只做四件事：

1. 所有 SDK 默认切到独立 `tcp session`
2. 所有 SDK 默认作为 `Agent` 的 session client，而不是本地 server
3. 所有 SDK 默认业务 `payload` 固定为 UTF-8 JSON
4. 所有 SDK 清理 `NNG` 主依赖、`LocalControl`、`rpc_addr`、本地监听模型

## 统一约束

- 首帧必须是 `ProviderConnectRequest`
- SDK 不监听端口
- Agent 不回拨 SDK
- `serve()` 只表示等待 session 生命周期，不表示启动本地监听
- 默认 `SDK <-> Agent` 不启用 TLS，但必须支持按需开启
- 默认只支持 JSON payload，不做多 codec 协商

## S0. 统一协议与配置基线

- [ ] 统一所有 SDK 的 `0x05xx` 语义为 `ProviderSession`
- [ ] 统一所有 SDK 的 header 编解码
- [ ] 统一 `FrameLength + Header + Body` framing
- [ ] 统一 `ProviderConnect/Heartbeat/Drain` 消息语义
- [ ] 统一 `invoke/startJob/streamJob/cancelJob` 语义
- [ ] 统一默认地址语义为 Agent 本地 gateway 地址
- [ ] 统一 TLS 字段命名
- [ ] 统一 reconnect / heartbeat / backpressure 字段命名

验收标准：

- 各语言 SDK 的配置与协议术语一致

## S1. 统一 transport 抽象

- [ ] 每个 SDK 都要有明确的 transport abstraction
- [ ] transport 默认实现固定优先为 `tcp`
- [ ] transport 不再把 `NNG` 写死在高层 API
- [ ] transport 支持独立读循环、写队列、并发请求复用
- [ ] transport 暴露关闭、重连、状态查询与错误映射

验收标准：

- 上层 Client / Invoker 不依赖具体 `NNG` 类型

## S2. 实现 provider session

- [ ] 实现 `connect() -> ProviderConnectRequest -> ProviderConnectResponse`
- [ ] 维护 `session_id`
- [ ] 实现 heartbeat
- [ ] 实现 Agent 下发调用请求的处理路径
- [ ] 实现 drain 通知处理
- [ ] 断线后旧 session 立即失效
- [ ] 重连后重新注册 provider session

验收标准：

- SDK 侧注册、心跳、调用、作业控制都复用同一条 session

## S3. 固化 JSON payload

- [ ] `InvokeRequest.payload` 默认编码 JSON
- [ ] `InvokeResponse.payload` 默认编码 JSON
- [ ] `JobEvent.payload` 默认编码 JSON
- [ ] 提供原生对象与 JSON bytes 的自动编解码
- [ ] `input_schema` / `output_schema` 固定解释为 JSON Schema
- [ ] 不要求 SDK 用户先定义自己的 `.proto`

验收标准：

- 各语言 SDK 的用户接入体验一致

## S4. 统一重连、背压、摘流

- [ ] 实现指数退避重连
- [ ] 达到上限后进入固定廉价周期重试
- [ ] 实现 `max_inflight_requests`
- [ ] 实现本地待处理队列上限
- [ ] 实现 `overloaded` / `retry_after_ms` / `draining` 反馈处理
- [ ] 实现优雅关闭与会话摘流

推荐默认值：

- `initialDelayMs = 1000`
- `maxDelayMs = 30000`
- `backoffMultiplier = 2.0`
- `jitterFactor = 0.2`
- `steadyStateDelayMs = 30000`

验收标准：

- 断线恢复与过载保护语义一致

## S5. 清理旧模型

- [ ] 清理 `LocalControl`
- [ ] 清理 `RegisterLocal`
- [ ] 清理 `rpc_addr`
- [ ] 清理 `local_listen`
- [ ] 清理 SDK 本地 `NNGServer`
- [ ] 清理 SDK 本地 `gRPC server`
- [ ] 清理 README / 示例 / CI 中的旧接入描述

验收标准：

- 新的 SDK 文档里不再出现回拨式模型

## G1. Go SDK

- [ ] 作为首个参考实现完成独立 `tcp session`
- [ ] 对齐 `ProviderSession`
- [ ] 对齐 JSON payload
- [ ] 对齐 TLS / reconnect / drain 语义
- [ ] 作为其他语言 SDK 的协议样板

## P1. Python SDK

- [ ] 用独立 `tcp session` 替换默认 `pynng`
- [ ] 打通 provider session
- [ ] 对齐 JSON payload、TLS、重连、drain
- [ ] 更新示例与打包说明

## J1. JS/TS SDK

- [ ] 用独立 `tcp session` 替换默认 `@rustup/nng`
- [ ] 打通 provider session
- [ ] 对齐 JSON payload、TLS、重连、drain
- [ ] 更新 README、Node/Bun 浏览器边界说明

## C1. C# SDK

- [ ] 用独立 `tcp session` 替换默认 `nng.NET`
- [ ] 打通 provider session
- [ ] 对齐 JSON payload、TLS、重连、drain
- [ ] 清理反射加载 `nng.NET` 的主路径依赖

## J2. Java SDK

- [ ] 去掉 TODO / mock / 占位调用路径
- [ ] 接入真实 `tcp session transport`
- [ ] 打通 provider session
- [ ] 对齐 JSON payload、TLS、重连、drain
- [ ] 完善 Java 17 下的集成测试

## C2. C++ SDK

- [ ] 清理 README 与代码中的历史 `gRPC/NNG server` 心智
- [ ] 接入独立 `tcp session`
- [ ] 打通 provider session
- [ ] 对齐 JSON payload、TLS、重连、drain
- [ ] 清理“启动本地服务”的旧 API 语义

## S6. 统一配置字段

- [ ] `address`
- [ ] `connectTimeoutMs`
- [ ] `requestTimeoutMs`
- [ ] `tls.enabled`
- [ ] `tls.certFile`
- [ ] `tls.keyFile`
- [ ] `tls.caFile`
- [ ] `tls.serverName`
- [ ] `tls.insecureSkipVerify`
- [ ] `heartbeat.intervalMs`
- [ ] `reconnect.enabled`
- [ ] `reconnect.initialDelayMs`
- [ ] `reconnect.maxDelayMs`
- [ ] `reconnect.backoffMultiplier`
- [ ] `reconnect.jitterFactor`
- [ ] `reconnect.steadyStateDelayMs`
- [ ] `backpressure.maxConcurrency`
- [ ] `backpressure.maxQueueSize`

明确废弃：

- [ ] `local_listen`
- [ ] `local_addr`
- [ ] `rpc_addr`
- [ ] 仅用于本地监听的 `ipcAddress`

## S7. SDK 测试

- [ ] framing 单元测试
- [ ] header 编解码测试
- [ ] 首帧识别测试
- [ ] provider session 建连测试
- [ ] invoke / startJob / cancelJob 测试
- [ ] reconnect 测试
- [ ] drain 测试
- [ ] backpressure 测试
- [ ] TLS / mTLS 测试
- [ ] JSON payload 编解码一致性测试

验收标准：

- 六语言 SDK 都有协议一致性与基本端到端测试

## 当前建议推进顺序

1. 先做 `S0 + S1`
2. 再做 Go SDK 参考实现
3. 再做 `S2 + S3 + S4`
4. 随后逐个推进 Python / JS/TS / C# / Java / C++
5. 最后做 `S5 + S6 + S7`

## 一句话验收目标

用户只需要：

1. 注册函数
2. 连接 Agent
3. 传入 JSON 对象

不需要：

- 启动本地服务
- 配置 `rpc_addr`
- 预先定义自己的 `.proto`
- 安装或分发 `NNG` 运行时
