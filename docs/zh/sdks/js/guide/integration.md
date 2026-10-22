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

`FunctionDescriptor` 的 `tags`/`summary`/`description`/`operationId`/`deprecated` 是展示层字段：注册时以 protobuf 编码上报，用于控制面的函数分组与页面生成。当多个语言 SDK 共享同一函数 ID 空间时，agent 对跨 provider 的空值注册做合并保护（空值不会覆盖其他 provider 已有的 tags/summary），但每个 SDK 仍应在注册时填写完整元数据——不要依赖合并兜底。

## 实例元数据（Provider 自描述）

`ClientConfig.instanceMetadata` 携带**用户自定义的多 KV 实例元数据**（如 `serverId`、`pod`）。它通过 `ProviderConnectRequest.metadata` 上报，控制台的 “SDK Version Distribution” 页面用它展示实例信息，并支持基于元数据的检索：

```typescript
const client = createClient({
  agentAddr: "127.0.0.1:19091",
  serviceId: "my-service",
  instanceMetadata: { serverId: "s1", pod: "game-7c4d" },
});
```

- 保留键 `sdkLanguage` / `sdkVersion` / `sdkName` / `protocol_version` / `gameId` / `env` 由平台固定字段生成；当用户元数据与保留键冲突时，agent 会丢弃该键，并在注册响应的 `warnings` 中给出告警。
- 元数据仅用于可观测性（展示/检索/诊断）；不参与路由与负载均衡。
- wire 语义与存储设计决策见 `docs/architecture/sdk-wire-protocol.md` 的 “Instance Metadata” 一节。

## 连接生命周期

transport 内部维护单 reader 的阻塞 read loop，因此空闲连接上的探测（ping/pong）与调用响应不会在并发读上竞争。连接断开或出错时：

- 所有 pending 调用立即以 `connection closed` 失败；
- `isConnected()` 变为 `false`，心跳失败后客户端按重连配置自动重连。

业务侧无需轮询连接状态；把 `call()` 的 rejection 当作普通的可重试失败即可。
