# C++ SDK 集成指南

本文说明如何把 C++ SDK 集成进现有项目。

## Windows 集成要点

- 显式设置 include 目录与库目录
- runtime library 设置必须与 triplet 匹配
- Debug 与 Release 构建分别验证依赖

## 要点

- 旧版 gRPC local-listener 模型不再是默认假设
- 优先围绕统一 session 模型做集成
