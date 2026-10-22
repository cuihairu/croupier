# 架构

当前 C++ SDK 文档以统一 session 模型为基线；旧版 gRPC local-listener 模型不再是默认假设。

## 要点

- SDK 主动连接 Agent
- 注册、心跳、调用与 job 控制都走同一条连接
- 业务 payload 默认使用 JSON
