---
title: JavaScript SDK API
---

# JavaScript SDK API

当前 API 以 SDK 源码与 TypeScript 类型定义为准，文档会逐步补齐。

## 核心对象

- `CroupierClient`: 连接 Agent 并维持会话
- `ClientConfig`: 客户端连接与标识配置
- `registerFunction(...)`: 注册函数处理器
- `registerFromOpenAPI(...)`: 从 OpenAPI 3 spec 本地导入并注册函数（Descriptor v2）

## 建议阅读顺序

1. 先看 [JavaScript SDK 首页](/sdks/js/)
2. 再看 [指南](/sdks/js/guide/)
3. 最后结合 `sdks/js/src` 阅读类型定义与实现

## 补充页面

- [集成指南](/sdks/js/guide/integration)
- [约定规范](/sdks/js/guide/conventions)
- [主线程调度器](/sdks/js/guide/threading)
