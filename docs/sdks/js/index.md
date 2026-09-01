---
title: JavaScript SDK
---

# JavaScript SDK

TypeScript 优先的 Node.js SDK，用于连接 Croupier Agent、注册函数并处理调用。

## 代码位置

- `sdks/js`

## 特性

- TypeScript 优先，提供完整类型定义
- 支持现代 Node.js 运行时
- 提供函数描述符与处理器注册能力
- 面向 monorepo 统一协议演进

## 安装

```bash
npm install croupier-js-sdk
```

## 快速开始

```ts
import { CroupierClient } from "croupier-js-sdk";

const client = new CroupierClient({
  agentAddr: "127.0.0.1:19091",
  gameId: "my-game",
  env: "development",
  insecure: true,
});

await client.connect();
await client.serve();
```

## 继续阅读

- [指南](/sdks/js/guide/)
- [API 参考](/sdks/js/api/)
- [仓库 README](https://github.com/cuihairu/croupier/tree/main/sdks/js)
