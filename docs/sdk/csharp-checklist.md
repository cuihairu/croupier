# C# SDK 重构清单

仓库：`croupier-sdk-csharp`

## 目标

移除对 `nng.NET` 的默认依赖，切到独立 `tcp session`，保证 .NET 8 下行为与 Go 参考实现一致。

## C0. 基线核对

- [ ] 默认 transport 改为独立 `tcp session`
- [ ] 默认地址语义改为 Agent 本地 gateway
- [ ] 默认 `payload` 固定为 JSON
- [ ] 默认不监听端口

## C1. transport

- [ ] 新增独立 `tcp transport`
- [ ] 不再把 `nng.NET` 作为默认主路径
- [ ] 清理反射加载 `nng.NET` 的主路径依赖
- [ ] 支持 framing、header、读循环、写队列

## C2. provider session

- [ ] `ConnectAsync()` 打通 `ProviderConnectRequest/Response`
- [ ] 维护 `session_id`
- [ ] 实现 heartbeat
- [ ] 实现 drain 处理
- [ ] 处理 Agent 下发调用请求

## C3. JSON payload

- [ ] 默认使用对象 <-> JSON bytes
- [ ] 对齐 `System.Text.Json` 行为与错误处理
- [ ] `input_schema/output_schema` 解释为 JSON Schema

## C4. 重连与背压

- [ ] 实现指数退避重连
- [ ] 上限后固定廉价周期重试
- [ ] 实现过载反馈处理
- [ ] 实现优雅关闭

## C5. 清理旧模型

- [ ] 清理 `LocalControl/RegisterLocal`
- [ ] 清理 `rpc_addr/local_listen`
- [ ] 清理 README、NuGet 说明、CI 中的旧接入方式

## 验收

- [ ] .NET 8 默认链路不依赖 `nng.NET`
- [ ] C# 用户只需传 POCO / JSON 对象

