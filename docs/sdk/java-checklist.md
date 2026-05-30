# Java SDK 重构清单

目录：`sdks/java/`

## 目标

Java SDK 当前重点不是“微调”，而是把真实链路彻底接通，去掉 TODO、mock 和占位逻辑。

## J0. 基线核对

- [x] 默认 transport 改为独立 `tcp session`
- [x] 默认地址语义改为 Agent 本地 gateway
- [x] 默认 `payload` 固定为 JSON
- [x] 默认不监听端口

## J1. transport

- [x] 实现真实 `tcp session transport`
- [x] 不再把 `NNGTransport` 作为默认主路径
- [x] transport 与高层 Client / Invoker 解耦
- [x] 支持 framing、header、读循环、写队列

## J2. provider session

- [x] `connect()` 真实发送 `ProviderConnectRequest`
- [x] 真实处理 `ProviderConnectResponse`
- [x] 维护 `session_id`
- [x] 实现 heartbeat
- [x] 实现 drain 处理
- [x] 处理 Agent 下发调用请求

## J3. 清理占位逻辑

- [x] 清理 TODO
- [x] 清理 mock / fake invoke 返回
- [x] 清理模拟 `startJob/cancelJob` 路径
- [x] 让 Client / Invoker 全部接入真实 transport

## J4. JSON payload

- [x] 默认使用对象 <-> JSON bytes
- [x] 明确 `Jackson` 或等价 JSON 实现策略
- [x] `input_schema/output_schema` 解释为 JSON Schema

## J5. 重连与背压

- [x] 实现指数退避重连
- [x] 上限后固定廉价周期重试
- [x] 实现过载反馈处理
- [x] 实现优雅关闭

## J6. 清理旧模型

- [x] 清理 `LocalControl/RegisterLocal`
- [x] 清理 `rpc_addr/local_listen`
- [ ] 清理 README、Gradle/Maven 使用说明中的旧模型

## J7. 测试

- [ ] Java 17 集成测试
- [ ] provider session 端到端测试
- [ ] JSON payload 编解码测试

## 验收

- [x] Java SDK 不再是占位实现
- [ ] Java 17 下能跑通完整默认链路
