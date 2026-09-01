---
title: JavaScript SDK 指南
---

# JavaScript SDK 指南

## 集成建议

- 优先在 Node.js 24+ 环境运行
- 将业务处理器按领域拆分，避免单个入口过重
- 与 `proto/**` 同步升级，避免 SDK 与服务端协议漂移

## 并发与线程模型

JavaScript SDK 运行在单进程事件循环模型上。阻塞型 CPU 任务应移交给 Worker 或外部服务，避免影响 Agent 回调处理时延。

## 约定

- 使用稳定的函数 ID，例如 `player.ban`
- 版本号与变更级别对应，不要把破坏性变更塞进补丁版本
- 对输入输出 JSON 结构保持显式约束

## 继续阅读

- [主线程调度器](./threading)
- [约定规范](./conventions)
- [集成指南](./integration)
- [API 参考](../api/)
