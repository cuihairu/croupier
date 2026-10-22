---
title: JavaScript SDK 指南
---

# JavaScript SDK 指南

## 集成建议

- 优先运行在 Node.js 24+ 上
- 按业务域拆分业务 handler，避免单一入口过重
- 与 `proto/**` 同步升级，避免 SDK 与服务端之间出现协议漂移

## 并发与线程模型

JavaScript SDK 运行在单进程 event loop 上。把阻塞的 CPU 工作移到 Workers 或外部服务，避免影响 Agent 回调延迟。

## 约定

- 使用稳定的函数 ID，如 `player.ban`
- 版本号与变更层级匹配；绝不把破坏性变更夹带进 patch 版本
- 对输入/输出 JSON 结构保持显式约束

## 延伸阅读

- [API 参考](../api/)
