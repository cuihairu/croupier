# Python SDK 重构清单

仓库：`croupier-sdk-python`

## 目标

移除对 `pynng` 的默认依赖，切到独立 `tcp session`，保证 Python 在内网常见部署下开箱可用。

## P0. 基线核对

- [ ] 默认 transport 改为独立 `tcp session`
- [ ] 默认地址语义改为 Agent 本地 gateway
- [ ] 默认 `payload` 固定为 JSON
- [ ] 默认不监听端口

## P1. transport

- [ ] 新增独立 `tcp transport`
- [ ] 不再把 `pynng` 作为默认主路径
- [ ] transport 与高层 Client / Invoker 解耦
- [ ] 支持 framing、header、读循环、写队列

## P2. provider session

- [ ] `connect()` 打通 `ProviderConnectRequest/Response`
- [ ] 维护 `session_id`
- [ ] 实现 heartbeat
- [ ] 实现 drain 处理
- [ ] 处理 Agent 下发调用请求

## P3. JSON payload

- [ ] 默认使用 `dict/list` <-> JSON bytes
- [ ] 保持异常与解码错误可区分
- [ ] `input_schema/output_schema` 解释为 JSON Schema

## P4. 重连与背压

- [ ] 实现指数退避重连
- [ ] 上限后固定廉价周期重试
- [ ] 实现过载反馈处理
- [ ] 实现优雅关闭

## P5. 清理旧模型

- [ ] 清理 `LocalControl/RegisterLocal`
- [ ] 清理 `rpc_addr/local_listen`
- [ ] 清理 README、示例、打包说明中的旧接入方式

## 验收

- [ ] 不安装 `pynng` 也能跑默认链路
- [ ] Python 用户只需传原生 JSON 对象

