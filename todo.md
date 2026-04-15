# Croupier Session 重构执行清单

更新时间：2026-04-14
状态：文档基线已基本冻结，代码重构待推进

## 目标

本清单只服务当前这一轮重构，不再继续沿用旧的扩展化主计划。

本轮最终目标只有五个：

1. `Agent <-> Server` 收敛为单连接、双向、多路复用的 `TCP session`
2. `SDK <-> Agent` 收敛为单连接、双向、多路复用的 `TCP session`
3. 两条链路共享同一套 `shared session runtime`
4. SDK 默认业务 `payload` 固定为 `JSON`
5. 清理 `NNG 主链路`、`LocalControl`、`rpc_addr`、`回拨式模型` 的历史依赖

## 约束

- `Agent <-> Server` 与 `SDK <-> Agent` 共用一套 session 心智模型
- `subprotocol` 只区分握手消息、注册内容和路由语义，不分叉底层 runtime
- `SDK <-> Agent` 默认不启用 TLS
- `Agent <-> Server` 默认启用 TLS
- SDK 不得开启本地监听端口
- Agent 本地监听只服务 `GameServer / SDK / 第三方本地应用`
- `payload` 默认只支持 JSON，不做多 codec 协商

## P0. 统一基线与消除设计冲突

- [ ] 统一仓库中的默认 TLS 语义
- [ ] 明确 `Agent <-> Server` 默认 `tls.enabled = true`
- [ ] 明确 `SDK <-> Agent` 默认 `tls.enabled = false`
- [ ] 统一 `shared session runtime`、`subprotocol`、`provider session`、`agent session` 术语
- [ ] 冻结 `Envelope v1`、`8-byte header`、`首帧识别`、`JSON payload` 规则
- [ ] 把 `0x05xx` 的目标语义固定为 `ProviderSession`

验收标准：

- 所有主文档、配置示例、README、SDK 规范对默认 TLS 与协议边界的描述一致

## P1. 抽取 shared session runtime

- [ ] 在主仓库抽取独立的 session runtime，而不是继续把主链路逻辑绑死在 `NNG`
- [ ] 固化 `4-byte FrameLength + 8-byte Header + Body` 的 framing 实现
- [ ] 实现统一的读循环、写队列、in-flight 请求表、`RequestID` 分配器
- [ ] 实现统一的 heartbeat、超时、关闭、drain、backpressure 机制
- [ ] 实现统一的连接状态机
- [ ] 实现统一的错误码与断链原因映射
- [ ] 为 runtime 增加基础观测指标

验收标准：

- runtime 可被 `Agent <-> Server` 与 `SDK <-> Agent` 共用
- runtime 不依赖 `NNG pattern`

## P2. 落地 Agent <-> Server session

- [ ] 在 Server 侧实现 `tcp session listener`
- [ ] 在 Agent 侧实现 `tcp session client`
- [ ] 让 `Register/Heartbeat` 跑在新 session 上
- [ ] 让 `Invoke/StartJob/CancelJob/Ops` 全部复用既有 session 下发
- [ ] 增加 `server_node_id + session_id` 路由信息
- [ ] 改造 registry，让 agent session 明确归属于某个 server node
- [ ] 改造 `Dispatcher`，不再根据 `rpc_addr` 直拨 Agent
- [ ] 为 Agent session 增加 drain、过载保护、会话摘流
- [ ] 清理 `rpc_addr` 作为运行时直连依赖

验收标准：

- `Server -> Agent` 不再走回拨链路
- 控制面与调用面统一复用同一条 session

## P3. 落地 SDK <-> Agent session

- [ ] 在 Agent 本地 gateway 实现 SDK 专用 `tcp listener`
- [ ] 首帧严格校验为 `ProviderConnectRequest`
- [ ] 建立 provider session store
- [ ] 落地 `ProviderConnectResponse`
- [ ] 落地 `ProviderHeartbeatRequest/Response`
- [ ] 落地 `ProviderDrainRequest/Response`
- [ ] 让 Agent 基于既有 provider session 向 SDK 下发 `Invoke/Job`
- [ ] 实现 SDK 侧重连后重新注册
- [ ] 实现 SDK 侧过载反馈处理
- [ ] 清理 `LocalControl`、`RegisterLocal`、`rpc_addr`、SDK 本地监听模型

验收标准：

- SDK 不监听端口
- Agent 不回拨 SDK
- 注册、心跳、调用、作业控制都复用同一条 session

## P4. 统一 protobuf 与协议常量

- [ ] 统一主仓库中的 MsgID 常量定义
- [ ] 将 `0x05xx` 常量名切到 `ProviderConnect/Heartbeat/Drain`
- [ ] 历史 `RegisterLocal*` 常量仅保留兼容别名
- [ ] 审视 `agent/v1/register.proto` 中的命名与语义
- [ ] 为 `RegisterRequest` 明确“session connect/register”语义
- [ ] 评估后续是否引入 `AgentConnectRequest` / `AgentConnectResponse`
- [ ] 清理协议实现中对 `LocalControlService` 的硬编码命名

验收标准：

- 主仓库协议命名与当前文档一致
- 历史别名不再成为新的代码入口

## P5. 固化 JSON payload 边界

- [ ] 固定 `InvokeRequest.payload` 为 UTF-8 JSON
- [ ] 固定 `InvokeResponse.payload` 为 UTF-8 JSON
- [ ] 固定 `JobEvent.payload` 为 UTF-8 JSON
- [ ] SDK 默认提供“原生对象 <-> JSON bytes”自动编解码
- [ ] Agent 不默认解析业务 payload 内部结构
- [ ] 把平台治理字段保留在 protobuf 协议层
- [ ] 把 `input_schema` / `output_schema` 固定为 JSON Schema 语义

验收标准：

- SDK 用户不需要先定义 `.proto` 才能接入
- payload 边界在所有语言中一致

## P6. 统一重连、背压、摘流策略

- [ ] 实现统一重连配置面
- [ ] 支持可选快速重试阶段
- [ ] 支持指数退避
- [ ] 达到上限后进入固定廉价周期重试
- [ ] 实现 `max_inflight_requests`
- [ ] 实现待处理队列上限
- [ ] 实现 `overloaded` / `retry_after_ms` / `draining` 反馈
- [ ] 为 Server 与 Agent 两侧都加上会话级 drain

推荐默认值：

- `initial_delay_ms = 1000`
- `max_delay_ms = 30000`
- `backoff_multiplier = 2.0`
- `jitter_factor = 0.2`
- 达到上限后固定 `30000ms` 周期持续重试

验收标准：

- 短暂断线能快速恢复
- 长时间故障不会惊群
- drain 后不再接收新流量

## P7. 以 Go 作为参考实现打通全链路

- [ ] 主仓库先完成 Go 侧 `Agent <-> Server` session 实现
- [ ] 主仓库完成 Go 侧 `SDK <-> Agent` session 实现
- [ ] 打通 `provider register -> invoke -> job -> cancel -> drain`
- [ ] 打通 `agent register -> invoke -> ops -> drain`
- [ ] 完成明文与 TLS 两条路径验证

验收标准：

- Go 参考实现可作为其他语言 SDK 的协议样板

## P8. 各语言 SDK 逐个对齐

### Go SDK

- [x] 切到独立 `tcp session`
- [x] 移除默认 `NNG` 依赖
- [x] 对齐 JSON payload、TLS、重连、drain 语义
- [x] 抽取 shared session runtime（session 包）

### Python SDK

- [x] 切到独立 `tcp session`
- [x] 去掉对 `pynng` 的默认依赖（nng.py 保留但未导入）
- [x] 对齐 JSON payload、TLS、重连、drain 语义

### JS/TS SDK

- [x] 切到独立 `tcp session`
- [x] 去掉对 `@rustup/nng` 的依赖（package.json 已移除）
- [x] 对齐 JSON payload、TLS、重连、drain 语义

### C# SDK

- [x] 切到独立 `tcp session`
- [x] 去掉对 `nng.NET` 的依赖
- [x] 对齐 JSON payload、TLS、重连、drain 语义

### Java SDK

- [x] 打通真实 transport，不再保留 TODO / mock 调用路径
- [x] 切到独立 `tcp session`
- [x] 对齐 JSON payload、TLS、重连、drain 语义

### C++ SDK

- [x] 清理 README 与实现中的历史 `gRPC/NNG server` 心智
- [x] 切到独立 `tcp session`（NNG 代码条件编译保留）
- [x] 对齐 JSON payload、TLS、重连、drain 语义

验收标准：

- 六个 SDK 的默认接入方式一致
- 没有语言再要求 SDK 开本地监听端口

## P9. 配置面统一

- [ ] 统一 `address`
- [ ] 统一 `connectTimeoutMs`
- [ ] 统一 `requestTimeoutMs`
- [ ] 统一 `tls.enabled`
- [ ] 统一 `tls.certFile`
- [ ] 统一 `tls.keyFile`
- [ ] 统一 `tls.caFile`
- [ ] 统一 `tls.serverName`
- [ ] 统一 `tls.insecureSkipVerify`
- [ ] 统一 `reconnect.*`
- [ ] 统一 `heartbeat.*`
- [ ] 统一 `backpressure.*`
- [ ] 废弃 `local_listen`
- [ ] 废弃 `local_addr`
- [ ] 废弃 `rpc_addr`
- [ ] 废弃仅用于本地监听的 `ipcAddress`

验收标准：

- 各语言配置字段语义一致
- 文档、代码、示例使用同一套字段名

## P10. 清理旧实现与旧依赖

- [ ] 清理主链路默认 `NNG` 依赖
- [ ] 清理 `LocalControlService` 历史入口
- [ ] 清理 `RegisterLocal*` 作为主语义的实现
- [ ] 清理 `Dispatcher` 直拨 Agent 代码路径
- [ ] 清理 SDK 本地 `NNGServer` / `RequestServer`
- [ ] 清理 `rpc_addr` 运行时依赖
- [ ] 清理过时示例、注释、README、CI 描述

验收标准：

- 新实现默认路径不再依赖 `NNG`
- 新接入文档不再出现回拨式模型

## P11. 测试与验收

- [ ] framing 单元测试
- [ ] header 编解码测试
- [ ] 首帧识别测试
- [ ] request/response 并发复用测试
- [ ] reconnect 测试
- [ ] drain 测试
- [ ] backpressure 测试
- [ ] TLS / mTLS 测试
- [ ] `Agent <-> Server` 集群路由测试
- [ ] `SDK <-> Agent` 端到端调用测试
- [ ] 六语言 SDK 协议一致性测试

验收标准：

- 所有主链路都有自动化测试覆盖
- CI 不再以历史 `NNG/gRPC` 主链路作为前提

## P12. 文档与发布收尾

- [ ] 清理剩余过时文档
- [ ] 清理剩余 README 中的 `NNG/gRPC/LocalControl/rpc_addr` 旧表述
- [ ] 更新配置示例
- [ ] 更新 Docker / 部署文档
- [ ] 更新联调清单
- [ ] 为各 SDK 仓库同步新的接入 README
- [ ] 发布迁移说明

验收标准：

- 主仓库与 SDK 仓库文档都只描述当前设计
- 接入方不会再被旧文档误导到回拨式模型

## 当前建议执行顺序

1. 先完成 `P0 + P1`
2. 再完成 `P2`
3. 再完成 `P3`
4. 随后完成 `P4 + P5 + P6`
5. 用 `P7` 打样
6. 再推进 `P8 + P9`
7. 最后完成 `P10 + P11 + P12`

## 明确不再继续做的事

- 不再继续强化 `NNG REQ/REP` 作为主链路
- 不再继续围绕 `LocalControl` 补新语义
- 不再让 SDK 暴露本地监听端口
- 不再把 `rpc_addr` 当成长期运行时核心字段
- 不再为业务 payload 引入多 codec 协商
