---
title: JavaScript SDK API
---

# JavaScript SDK API

当前的 API 参考以 SDK 源码与 TypeScript 类型定义为准；文档会逐步补齐。

## 核心对象

- `CroupierClient`：连接 Agent 并维持 session
- `ClientConfig`：客户端连接与身份配置
- `registerFunction(...)`：注册函数 handler
- `registerFromOpenAPI(...)`：从 OpenAPI 3 spec（Descriptor v2）本地导入并注册函数

## 建议阅读顺序

1. 先看 [JavaScript SDK 首页](/zh/sdks/js/)
2. 再看[指南](/zh/sdks/js/guide/)
3. 最后结合 `sdks/js/src` 阅读类型定义与实现

## 更多页面
