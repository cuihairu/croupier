---
title: Go SDK 指南
---

# Go SDK 指南

## 构建模式

Go SDK 当前有两类构建关注点：本地开发与 CI/生产。文档与实现都应以 monorepo 中的协议定义为准；不再需要从外部 SDK 仓库同步。

## 集成建议

- 按业务域将 handler 拆分到独立 package
- 在同一个 PR 中同时评审协议变更与 SDK 升级
- 为 timeout、retry 与错误映射构建统一封装

## 并发模型

Go SDK 适合并发服务场景，但 handler 仍需显式的边界约束，避免 goroutine 泄漏失控。

## 延伸阅读
