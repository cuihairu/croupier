# Go SDK 重构清单

仓库：`croupier-sdk-go`

## 目标

Go SDK 作为首个参考实现，先把完整的 `sdk-agent subprotocol` 跑通，并作为其他语言 SDK 的协议样板。

## G0. 基线核对

- [x] 默认 transport 改为独立 `tcp session`
- [x] 默认地址语义改为 Agent 本地 gateway
- [x] 默认 `payload` 固定为 JSON
- [x] 默认不要求 SDK 监听端口

## G1. transport

- [x] 提供独立 `tcp transport`
- [x] 实现 framing、header、并发 request/response 复用
- [x] 支持读循环、写队列、关闭、状态查询
- [x] 让上层不依赖具体 `NNGTransport`

## G2. provider session

- [x] `connect()` 发送 `ProviderConnectRequest`
- [x] 接收 `ProviderConnectResponse`
- [x] 维护 `session_id`
- [x] 实现 heartbeat
- [x] 实现 drain 处理
- [x] 实现 Agent 下发 `Invoke/Job` 请求处理

## G3. JSON payload

- [x] 默认对象转 JSON bytes
- [x] 默认 JSON bytes 转对象
- [x] `input_schema/output_schema` 固定解释为 JSON Schema

## G4. 重连与背压

- [x] 指数退避重连
- [x] 上限后固定廉价周期重试
- [x] 处理 `overloaded/retry_after_ms/draining`
- [x] 支持优雅关闭

## G5. 清理旧模型

- [x] 清理默认原生传输依赖
- [x] 清理 `LocalControl/RegisterLocal` 主语义
- [x] 清理 `rpc_addr/local_listen`
- [x] 清理 README 中的旧接入方式

## 验收

- [ ] Go SDK 能作为其他语言的参考实现
- [ ] 用户只需注册函数、连接 Agent、传 JSON 对象
