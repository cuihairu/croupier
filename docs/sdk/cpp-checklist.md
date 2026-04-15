# C++ SDK 重构清单

仓库：`croupier-sdk-cpp`

## 目标

清理 C++ SDK 目前最容易误导接入方的历史心智，把实现和 README 一起收敛到独立 `tcp session`。

## C0. 基线核对

- [ ] 默认 transport 改为独立 `tcp session`
- [ ] 默认地址语义改为 Agent 本地 gateway
- [ ] 默认 `payload` 固定为 JSON
- [ ] 默认不监听端口

## C1. transport

- [ ] 实现独立 `tcp transport`
- [ ] 不再把 `NNG` 作为默认主路径
- [ ] transport 与高层 Client / Invoker 解耦
- [ ] 支持 framing、header、读循环、写队列

## C2. provider session

- [ ] `connect()` 打通 `ProviderConnectRequest/Response`
- [ ] 维护 `session_id`
- [ ] 实现 heartbeat
- [ ] 实现 drain 处理
- [ ] 处理 Agent 下发调用请求

## C3. JSON payload

- [ ] 默认使用对象 <-> JSON bytes
- [ ] 明确 `nlohmann/json` 或等价 JSON 实现策略
- [ ] `input_schema/output_schema` 解释为 JSON Schema

## C4. 重连与背压

- [ ] 实现指数退避重连
- [ ] 上限后固定廉价周期重试
- [ ] 实现过载反馈处理
- [ ] 实现优雅关闭

## C5. 清理旧模型

- [ ] 清理 README 中的历史 `gRPC` 表述
- [ ] 清理 README 中的历史 `NNG server` 表述
- [ ] 清理 `LocalControl/RegisterLocal`
- [ ] 清理 `rpc_addr/local_listen`
- [ ] 清理“启动本地服务”的旧 API 语义

## 验收

- [ ] C++ SDK 文档与实现都不再传播旧模型
- [ ] 用户只需注册函数、连接 Agent、传 JSON 对象

