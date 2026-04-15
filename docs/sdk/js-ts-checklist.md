# JS/TS SDK 重构清单

仓库：`croupier-sdk-js`

## 目标

移除对 `@rustup/nng` 的默认依赖，切到独立 `tcp session`，统一 Node 侧默认接入体验。

## J0. 基线核对

- [ ] 默认 transport 改为独立 `tcp session`
- [ ] 默认地址语义改为 Agent 本地 gateway
- [ ] 默认 `payload` 固定为 JSON
- [ ] 默认不监听端口

## J1. transport

- [ ] 新增独立 `tcp transport`
- [ ] 不再把 `@rustup/nng` 作为默认主路径
- [ ] transport 与高层 Client / Invoker 解耦
- [ ] 支持 framing、header、读循环、写队列

## J2. provider session

- [ ] `connect()` 打通 `ProviderConnectRequest/Response`
- [ ] 维护 `session_id`
- [ ] 实现 heartbeat
- [ ] 实现 drain 处理
- [ ] 处理 Agent 下发调用请求

## J3. JSON payload

- [ ] 默认使用对象 <-> JSON bytes
- [ ] 保持 TypeScript 类型定义与运行时行为一致
- [ ] `input_schema/output_schema` 解释为 JSON Schema

## J4. 重连与背压

- [ ] 实现指数退避重连
- [ ] 上限后固定廉价周期重试
- [ ] 实现过载反馈处理
- [ ] 实现优雅关闭

## J5. 清理旧模型

- [ ] 清理 `LocalControl/RegisterLocal`
- [ ] 清理 `rpc_addr/local_listen`
- [ ] 清理 README 与 Node/Bun 适配说明中的旧描述

## 验收

- [ ] 不安装 `@rustup/nng` 也能跑默认链路
- [ ] Node 用户只需传 JS 对象

