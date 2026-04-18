# C++ SDK 重构清单

仓库：`croupier-sdk-cpp`

## 目标

清理 C++ SDK 目前最容易误导接入方的历史心智，把实现和 README 一起收敛到独立 `tcp session`。

## C0. 基线核对

- [x] 默认 transport 改为独立 `tcp session`
- [x] 默认地址语义改为 Agent 本地 gateway
- [x] 默认 `payload` 固定为 JSON
- [x] 默认不监听端口

## C1. transport

- [x] 实现独立 `tcp transport`
- [x] 不再把 `NNG` 作为默认主路径
- [x] transport 与高层 Client / Invoker 解耦
- [x] 支持 framing、header、读循环、写队列

## C2. provider session

- [x] `connect()` 打通 `ProviderConnectRequest/Response`
- [x] 维护 `session_id`
- [x] 实现 heartbeat
- [x] 实现 drain 处理
- [x] 处理 Agent 下发调用请求

## C3. JSON payload

- [x] 默认使用对象 <-> JSON bytes
- [x] 明确 `nlohmann/json` 或等价 JSON 实现策略
- [x] `input_schema/output_schema` 解释为 JSON Schema

## C4. 重连与背压

- [x] 实现指数退避重连
- [x] 上限后固定廉价周期重试
- [x] 实现过载反馈处理
- [x] 实现优雅关闭

## C5. 清理旧模型

- [x] 清理 README 中的历史 `gRPC` 表述
- [x] 清理 README 中的历史 `NNG server` 表述
- [x] 清理 `LocalControl/RegisterLocal`
- [x] 清理 `rpc_addr/local_listen`
- [ ] 清理“启动本地服务”的旧 API 语义

## 验收

- [x] C++ SDK 文档与实现都不再传播旧模型
- [x] 用户只需注册函数、连接 Agent、传 JSON 对象
