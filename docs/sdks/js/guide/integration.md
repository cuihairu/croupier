# Croupier JS/TS SDK 集成指南

## 安装

```bash
npm install croupier-js-sdk
```

## 最小示例

```typescript
import {
  createClient,
  FunctionDescriptor,
  ClientConfig,
} from "croupier-js-sdk";

async function main() {
  const config: ClientConfig = {
    agentAddr: "127.0.0.1:19091",
    serviceId: "my-service",
  };

  const client = createClient(config);
  const descriptor: FunctionDescriptor = {
    id: "game.action",
    version: "1.0.0",
    resource: "game",
    capability: "action",
    risk: "safe",
  };

  client.registerFunction(descriptor, async () =>
    JSON.stringify({ status: "success" }),
  );
  await client.connect();
  await client.serve();
}
```
