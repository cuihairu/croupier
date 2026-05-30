# C++ SDK 集成指南

本文档说明如何在现有工程中集成 C++ SDK。

## Windows 集成建议

- 明确包含目录和库目录
- 运行时库设置必须与 triplet 匹配
- Debug/Release 下依赖库分别核对

## 重点

- 不再把历史 gRPC 本地监听模型作为默认前提
- 优先围绕统一 session 模型做集成
