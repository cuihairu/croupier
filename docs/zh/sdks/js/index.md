---
title: JavaScript SDK
---

# JavaScript SDK

TypeScript-first 的 Node.js SDK，用于连接 Croupier Agent、注册函数并处理调用。

## 代码位置

- `sdks/js`

## 特性

- TypeScript-first，类型定义完整
- 支持现代 Node.js 运行时
- 函数描述符与 handler 注册
- 与 monorepo 的统一协议演进保持对齐

```ts
import { setFieldWidget, setFieldHint } from "croupier-sdk";

let desc = setFieldWidget(
  { id: "player.ban", version: "1.0.0" },
  "id",
  "Select",
);
desc = setFieldHint(desc, "id", "x-options-source", {
  functionId: "player.list",
  labelPath: "/items/*/name",
  valuePath: "/items/*/id",
});
```

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

## 延伸阅读

- [指南](/zh/sdks/js/guide/)
- [API 参考](/zh/sdks/js/api/)
- [仓库 README](https://github.com/cuihairu/croupier/tree/main/sdks/js)
