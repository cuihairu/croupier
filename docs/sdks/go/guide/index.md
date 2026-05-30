---
title: Go SDK 指南
---

# Go SDK 指南

## 构建模式

Go SDK 当前存在本地开发与 CI/生产环境两类构建关注点。文档和实现都应以 monorepo 中的协议定义为准，不再依赖外部 SDK 仓库同步。

## 集成建议

- 将处理器按业务域拆到独立 package
- 把协议变更和 SDK 升级放在同一个 PR 中评审
- 对超时、重试和错误映射建立统一封装

## 并发模型

Go SDK 适合并发服务场景，但处理器仍然需要明确边界，避免无控制 goroutine 泄漏。

## 继续阅读

- [并发与调度](./threading)
- [约定规范](./conventions)
- [集成指南](./integration)
- [E2E 流程](../e2e-flow)
