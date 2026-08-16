# Croupier JS/TS SDK 集成指南

## 安装

```bash
npm install @croupier/sdk
```

## 最小示例

```typescript
import {
  CroupierClient,
  FunctionDescriptor,
  ClientConfig,
} from "@croupier/sdk";

async function main() {
  const config: ClientConfig = {
    agentAddr: "127.0.0.1:19091",
    serviceId: "my-service",
  };

  const client = new CroupierClient(config);
  const descriptor: FunctionDescriptor = {
    id: "game.action",
    version: "1.0.0",
    category: "gameplay",
    risk: "low",
  };

  client.registerFunction(descriptor, async () =>
    JSON.stringify({ status: "success" }),
  );
  await client.connect();
  await client.serve();
}
```
