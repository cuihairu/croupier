---
title: C++ SDK
icon: file-code
order: 1
category:
  - SDK 文档
tag:
  - C++
  - SDK
---

# Croupier C++ SDK

本文档描述的是 C++ SDK 在主仓库中的统一设计基线，不再沿用历史 `gRPC`、SDK 本地监听或默认 `NNG` 依赖的写法。

## 当前定位

C++ SDK 的目标定位很明确：

- 作为业务程序内嵌依赖
- 主动连接 Agent 本地 gateway
- 在单条 session 上完成注册、心跳、调用与作业控制
- 默认使用 JSON payload
- 按需启用 TLS

这意味着：

- 不再要求 SDK 开本地端口
- 不再要求业务接入方维护 `rpc_addr`
- 不再把 `gRPC` 作为当前默认链路
- 不再把 `NNG` 写成默认必选依赖

## 当前设计基线

### 传输

- 默认 transport：独立 `tcp session`
- 默认安全策略：内网场景可明文，跨主机或合规场景启用 `tls`
- 首帧：`ProviderConnectRequest`
- 同一连接支持双向 request/response 复用

### 会话

- SDK 建连后建立 `provider session`
- Agent 基于既有 session 向 SDK 下发调用请求
- 断线后旧 session 立即失效
- SDK 自动重连后重新发起 `ProviderConnectRequest`

### 数据

- 平台协议：protobuf
- 业务 payload：UTF-8 JSON
- `input_schema` / `output_schema`：JSON Schema，可选

## C++ SDK 应向接入方暴露的核心能力

- 连接管理
  - `address`
  - `connect timeout`
  - `request timeout`
  - `tls`
- 会话管理
  - `ProviderConnect`
  - `heartbeat`
  - `drain`
  - 重连
- 注册与调用
  - 注册函数描述符
  - 处理 Agent 下发的 `InvokeRequest`
  - 处理作业型请求
- 业务编解码
  - 原生对象 <-> JSON

## 不再推荐的旧模型

以下内容如果仍出现在文档、示例或 README 中，都属于过时设计：

- SDK 本地 `gRPC server`
- SDK 本地 `NNGServer`
- `LocalControlService`
- `RegisterLocal`
- `rpc_addr`
- `local_listen`
- “Agent 回拨 SDK”

## 概念性接入流程

下面是概念流程，不代表某个具体 API 名字必须完全一致：

```cpp
ClientOptions options;
options.address = "127.0.0.1:19091";
options.tls.enabled = false;
options.reconnect.initialDelayMs = 1000;
options.reconnect.maxDelayMs = 30000;
options.reconnect.steadyStateDelayMs = 30000;

Provider provider(options);
provider.registerFunction(descriptor, handler);
provider.serve();
```

语义说明：

- `serve()` 表示维持 session 生命周期与处理下行请求
- `serve()` 不再表示“启动本地监听服务”

## 推荐文档边界

主仓库中的 C++ SDK 文档只应说明：

- 协议基线
- 传输基线
- 配置语义
- 禁止项

具体 API 名称、构建脚本、发布工件和示例代码，应以 `croupier-sdk-cpp` 仓库为准。

## 相关文档

- [C++ SDK 配置指南](./CONFIG_GUIDE.md)
- [SDK 规范](../../sdk/specification.md)
- [SDK Wire Protocol](../../architecture/sdk-wire-protocol.md)
- [SDK-Agent 传输重构设计](../../architecture/sdk-agent-transport-redesign.md)
