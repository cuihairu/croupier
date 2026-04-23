---
home: true
title: Croupier JavaScript SDK
titleTemplate: false
heroImage: /logo.png
heroText: Croupier JavaScript SDK
tagline: JavaScript/TypeScript SDK for Croupier
actions:
  - text: 快速开始
    link: /guide/quick-start.html
    type: primary
features:
  - title: TypeScript 支持
    details: 完整的 TypeScript 类型定义
  - title: 现代化
    details: 支持 ES2022+ 特性
  - title: 易于使用
    details: 简洁的 API 设计

footer: Apache License 2.0 | Copyright © 2024 Croupier
---

## 简介

Croupier JavaScript SDK 是 [Croupier](https://github.com/cuihairu/croupier) 的 JavaScript/TypeScript 客户端实现。

## 安装

```bash
npm install @croupier/sdk
# 或
pnpm add @croupier/sdk
# 或
yarn add @croupier/sdk
```

## 快速开始

```typescript
import { CroupierClient, ClientConfig } from "@croupier/sdk";

const config: ClientConfig = {
  agentAddr: "localhost:19090",
  gameId: "my-game",
  env: "development",
  insecure: true,
};

const client = new CroupierClient(config);

client.registerFunction(
  {
    id: "hello.world",
    version: "0.1.0",
  },
  async (ctx, payload) => {
    return { message: "Hello from JavaScript!" };
  },
);

await client.connect();
await client.serve();
```
