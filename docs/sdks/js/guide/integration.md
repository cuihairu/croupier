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

## 注册元数据

`FunctionDescriptor` 的 `tags`/`summary`/`description`/`operationId`/`deprecated` 是展示层字段：注册时会随 protobuf 编码上报，进入控制面的函数分组与页面生成。多语言 SDK 共享同一 function ID 空间时，agent 对跨 provider 的空值注册做合并保护（空值不覆盖他方已有 tags/summary），但每个 SDK 仍应在注册时填写完整元数据，不要依赖合并兜底。

## 连接生命周期

transport 内部维持单读者阻塞读循环，空闲连接上探针（pong）与调用响应不受并发读竞争影响。连接断开或出错时：

- 所有 pending 调用立即以 `connection closed` 失败；
- `isConnected()` 变为 `false`，心跳失败后按重连配置自动重连。

业务侧无需自行轮询连接状态；把 `call()` 的 rejection 当作普通的可重试失败处理即可。
