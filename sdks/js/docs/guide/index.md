# 入门指南

## 系统要求

- Node.js ≥ 22
- TypeScript ≥ 5.0（可选）

## 安装

```bash
npm install @croupier/sdk
```

## 快速开始

### TypeScript

```typescript
import { CroupierClient, ClientConfig } from "@croupier/sdk";

const config: ClientConfig = {
  agentAddr: "localhost:19090",
  gameId: "demo-game",
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
    return { success: true };
  },
);

await client.serve();
```

### JavaScript

```javascript
const { CroupierClient } = require("@croupier/sdk");

const client = new CroupierClient({
  agentAddr: "localhost:19090",
  gameId: "demo-game",
  env: "development",
});

client.registerFunction(
  {
    id: "hello.world",
    version: "0.1.0",
  },
  async (ctx, payload) => {
    return { success: true };
  },
);

client.serve();
```

## 下一步

- [主线程调度器](./threading.md) - 回调执行时机控制
- [API 参考](../api/) - 完整的 API 文档
