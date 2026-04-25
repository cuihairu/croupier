# 架构

当前 C++ SDK 文档以统一 session 模型为基线，不再以历史 gRPC 本地监听模型为默认前提。

## 关键点

- SDK 主动连接 Agent
- 单条连接上完成注册、心跳、调用和作业控制
- 业务 payload 默认使用 JSON
