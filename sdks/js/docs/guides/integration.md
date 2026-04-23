# Croupier JS/TS SDK 集成指南

本指南提供完整的 Croupier JavaScript/TypeScript SDK 集成步骤，帮助开发者快速接入游戏后端平台。

## 目录

- [快速开始](#快速开始)
- [安装](#安装)
- [核心概念](#核心概念)
- [完整接口参考](#完整接口参考)
- [配置说明](#配置说明)
- [生产部署](#生产部署)
- [故障排查](#故障排查)

---

## 快速开始

### 安装 SDK

```bash
# npm
npm install @croupier/sdk

# yarn
yarn add @croupier/sdk

# pnpm
pnpm add @croupier/sdk
```

### 最小集成示例

```typescript
import {
  CroupierClient,
  FunctionDescriptor,
  ClientConfig,
} from "@croupier/sdk";

// 定义函数处理器
async function myHandler(
  context: CallContext,
  payload: Uint8Array,
): Promise<string> {
  const data = JSON.parse(new TextDecoder().decode(payload));
  // 处理业务逻辑
  return JSON.stringify({ status: "success" });
}

async function main() {
  // 创建配置
  const config: ClientConfig = {
    agentAddr: "127.0.0.1:19090",
    serviceId: "my-service",
  };

  // 创建客户端
  const client = new CroupierClient(config);

  // 注册函数
  const descriptor: FunctionDescriptor = {
    id: "game.action",
    version: "1.0.0",
    category: "gameplay",
    risk: "low",
  };
  client.registerFunction(descriptor, myHandler);

  // 连接并启动服务
  await client.connect();
  await client.serve(); // 阻塞运行
}

main().catch(console.error);
```

---

## 安装

### 系统要求

| 平台        | 架构                       | Node.js 版本  | 状态    |
| ----------- | -------------------------- | ------------- | ------- |
| **Linux**   | x64, ARM64                 | 18+, 20+, 22+ | ✅ 支持 |
| **macOS**   | x64, ARM64 (Apple Silicon) | 18+, 20+, 22+ | ✅ 支持 |
| **Windows** | x64                        | 18+, 20+, 22+ | ✅ 支持 |

### 从源码安装

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier-sdk-js.git
cd croupier-sdk-js

# 安装依赖
npm install

# 构建
npm run build
```

### 验证安装

```bash
npm run example:basic
```

---

## 核心概念

### 客户端 (Client)

客户端负责注册和管理游戏函数，接收来自 Agent 的调用请求。

```typescript
import { CroupierClient } from "@croupier/sdk";

const client = new CroupierClient(config);
```

### 函数描述符 (FunctionDescriptor)

描述函数的元数据：

```typescript
import { FunctionDescriptor } from "@croupier/sdk";

const descriptor: FunctionDescriptor = {
  id: "player.ban", // 函数唯一标识
  version: "1.0.0", // 版本号
  category: "moderation", // 业务分类
  risk: "high", // 风险等级: low, medium, high
  entity: "player", // 关联实体
  operation: "update", // 操作类型: create, read, update, delete
  enabled: true, // 是否启用
};
```

### 函数处理器 (Handler)

函数处理器是处理具体业务逻辑的函数：

```typescript
type Handler = (
  context: CallContext,
  payload: Uint8Array,
) => string | Promise<string>;

async function handler(
  context: CallContext,
  payload: Uint8Array,
): Promise<string> {
  /**
   * Args:
   *   context: 调用上下文，包含调用者信息
   *   payload: 请求负载，JSON 格式的 Uint8Array
   *
   * Returns:
   *   string | Promise<string>: JSON 格式的响应字符串
   */

  const data = JSON.parse(new TextDecoder().decode(payload));

  // 处理业务逻辑
  return JSON.stringify({ status: "success" });
}
```

---

## 完整接口参考

### CroupierClient

#### 初始化

```typescript
import { CroupierClient, ClientConfig } from "@croupier/sdk";

const config: ClientConfig = {
  // 连接配置
  agentAddr: "127.0.0.1:19090",
  controlAddr: "127.0.0.1:18080",
  localAddr: "127.0.0.1:0",

  // 身份配置
  serviceId: "my-service",
  serviceVersion: "1.0.0",
  gameId: "my-game",
  env: "production",

  // TLS 配置
  insecure: false,
  certFile: "/path/to/cert.pem",
  keyFile: "/path/to/key.pem",
  caFile: "/path/to/ca.pem",
  serverName: "agent.croupier.io",

  // 超时配置
  timeoutSeconds: 30,
};

const client = new CroupierClient(config);
```

#### 方法

| 方法                                    | 说明                 | 返回值          |
| --------------------------------------- | -------------------- | --------------- |
| `registerFunction(descriptor, handler)` | 注册函数             | `void`          |
| `unregisterFunction(functionId)`        | 取消注册函数         | `Promise<void>` |
| `connect()`                             | 连接到 Agent         | `Promise<void>` |
| `disconnect()`                          | 断开连接             | `Promise<void>` |
| `serve()`                               | 启动服务循环（阻塞） | `Promise<void>` |
| `isConnected()`                         | 检查连接状态         | `boolean`       |

### Invoker

用于主动调用远程函数（可选）：

```typescript
import { CroupierInvoker, InvokerConfig } from "@croupier/sdk";

const invokerConfig: InvokerConfig = {
  address: "localhost:8080",
  insecure: true,
  timeoutSeconds: 30,
};

const invoker = new CroupierInvoker(invokerConfig);

// 连接
await invoker.connect();

// 调用函数
const result = await invoker.invoke("player.get", '{"player_id":"123"}');

// 启动异步作业
const jobId = await invoker.startJob("item.create", '{"type":"sword"}');

// 流式获取作业事件
for await (const event of invoker.streamJob(jobId)) {
  console.log(`事件: ${event.eventType}, 数据: ${event.payload}`);
}

// 取消作业
await invoker.cancelJob(jobId);

// 关闭
await invoker.close();
```

---

## 配置说明

### ClientConfig 完整参数

```typescript
import { ClientConfig } from "@croupier/sdk";

const config: ClientConfig = {
  // === 连接配置 ===
  agentAddr: "127.0.0.1:19090", // Agent 地址
  controlAddr: "127.0.0.1:18080", // Control 平台地址（可选）
  localAddr: "127.0.0.1:0", // 本地监听地址

  // === 身份配置 ===
  serviceId: "my-service", // 服务标识（必填）
  serviceVersion: "1.0.0", // 服务版本
  gameId: "my-game", // 游戏标识
  env: "production", // 环境: development, staging, production

  // === TLS 配置 ===
  insecure: false, // 是否禁用 TLS
  certFile: "/path/to/cert.pem", // 客户端证书
  keyFile: "/path/to/key.pem", // 客户端私钥
  caFile: "/path/to/ca.pem", // CA 证书
  serverName: "agent.croupier.io", // SNI 名称

  // === 超时配置 ===
  timeoutSeconds: 30,

  // === 重连配置 ===
  autoReconnect: true,
  reconnectIntervalSeconds: 5,
  reconnectMaxAttempts: 0, // 0 = 无限重试
};
```

### 环境变量

可通过环境变量覆盖配置：

```bash
export CROUPIER_AGENT_ADDR="127.0.0.1:19090"
export CROUPIER_SERVICE_ID="my-service"
export CROUPIER_INSECURE="false"
```

---

## 生产部署

### Docker 部署

创建 `Dockerfile`:

```dockerfile
FROM node:20-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY package*.json ./
RUN npm ci --only=production

# 复制源码
COPY . .

# 构建 TypeScript
RUN npm run build

# 运行时镜像
FROM node:20-alpine

WORKDIR /app

COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY package*.json ./

# 暴露健康检查端口
EXPOSE 8080

# 运行服务
CMD ["node", "dist/server.js"]
```

创建 `docker-compose.yml`:

```yaml
version: "3.8"

services:
  game-service:
    build: .
    environment:
      - CROUPIER_AGENT_ADDR=agent:19090
      - CROUPIER_ENV=production
      - NODE_ENV=production
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Kubernetes 部署

创建 `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: croupier-game-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: game-service
  template:
    metadata:
      labels:
        app: game-service
    spec:
      containers:
        - name: game-service
          image: your-registry/croupier-game-service:latest
          env:
            - name: CROUPIER_AGENT_ADDR
              value: "croupier-agent:19090"
            - name: CROUPIER_ENV
              value: "production"
            - name: NODE_ENV
              value: "production"
          ports:
            - containerPort: 8080
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: croupier-game-service
spec:
  selector:
    app: game-service
  ports:
    - port: 80
      targetPort: 8080
```

---

## 故障排查

### 连接失败

**问题**: 无法连接到 Agent

```typescript
// 检查配置
console.log(`Agent 地址: ${config.agentAddr}`);

// 检查网络连通性
import net from "net";
const socket = new net.Socket();
socket
  .connect(19090, "127.0.0.1")
  .on("connect", () => {
    console.log("连接测试成功");
    socket.destroy();
  })
  .on("error", (err) => {
    console.log(`连接测试失败: ${err.message}`);
  });
```

**解决方案**:

1. 确认 Agent 服务正在运行
2. 检查防火墙规则
3. 验证地址格式

### 函数未注册

**问题**: 函数注册失败

```typescript
// 检查描述符
const descriptor: FunctionDescriptor = {
  id: "player.ban",
  version: "1.0.0",
};

// 验证必填字段
if (!descriptor.id) {
  console.error("函数 ID 不能为空");
}
if (!descriptor.version) {
  console.error("版本号不能为空");
}

// 注册前检查
if (!client.isConnected()) {
  console.error("客户端未连接");
}
```

### 性能问题

**优化建议**:

1. 使用异步处理

```typescript
async function asyncHandler(
  context: CallContext,
  payload: Uint8Array,
): Promise<string> {
  const data = JSON.parse(new TextDecoder().decode(payload));

  // 提交到后台异步处理
  setImmediate(() => processLongRunningTask(data));

  // 立即返回
  return JSON.stringify({ status: "submitted" });
}
```

2. 启用连接复用

```typescript
config.enableKeepAlive = true;
config.keepAliveIntervalSeconds = 30;
```

---

## 更多资源

- [约定规范](conventions.md) - 命名约定和最佳实践
- [约定规范](./conventions.md) - 命名约定和最佳实践
- [API 参考](../api/) - 详细的 API 文档
- [问题反馈](https://github.com/cuihairu/croupier-sdk-js/issues)
