# JS/TS SDK 重构清单

目录：`sdks/js/`

## 目标

移除对 `JS 原生传输` 的默认依赖，切到独立 `tcp session`，统一 Node 侧默认接入体验。

## J0. 基线核对

- [x] 默认 transport 改为独立 `tcp session`
- [x] 默认地址语义改为 Agent 本地 gateway
- [x] 默认 `payload` 固定为 JSON
- [x] 默认不监听端口

## J1. transport

- [x] 新增独立 `tcp transport`
- [x] 不再把 `JS 原生传输` 作为默认主路径
- [x] transport 与高层 Client / Invoker 解耦
- [x] 支持 framing、header、读循环、写队列

## J2. provider session

- [x] `connect()` 打通 `ProviderConnectRequest/Response`
- [x] 维护 `session_id`
- [x] 实现 heartbeat
- [x] 实现 drain 处理
- [x] 处理 Agent 下发调用请求

## J3. JSON payload

- [x] 默认使用对象 <-> JSON bytes
- [x] 保持 TypeScript 类型定义与运行时行为一致
- [x] `input_schema/output_schema` 解释为 JSON Schema

## J4. 重连与背压

- [x] 实现指数退避重连
- [x] 上限后固定廉价周期重试
- [x] 实现过载反馈处理
- [x] 实现优雅关闭

## J5. 清理旧模型

- [x] 清理 `LocalControl/RegisterLocal`
- [x] 清理 `rpc_addr/local_listen`
- [ ] 清理 README 与 Node/Bun 适配说明中的旧描述

## 验收

- [x] 不安装 `JS 原生传输` 也能跑默认链路
- [x] Node 用户只需传 JS 对象
