# API 参考

本文档提供 Croupier JS/TS SDK 的完整 API 参考。

## 包结构

```typescript
import {
  // 核心类
  BasicClient,
  createClient,

  // 类型定义
  CroupierClient,
  FunctionDescriptor,
  FunctionHandler,
  FileTransferConfig,
  FileUploadRequest,
} from "croupier-sdk";
```

## 核心类型

### FunctionHandler

函数处理器类型定义。

```typescript
type FunctionHandler = (
  context: string,
  payload: string,
) => Promise<string> | string;
```

**参数:**

- `context`: 调用上下文（JSON 字符串）
- `payload`: 函数参数（JSON 字符串）

**返回值:**

- `Promise<string> | string`: 函数执行结果（JSON 字符串），支持同步或异步

**使用示例:**

```typescript
const handler: FunctionHandler = async (context, payload) => {
  const ctx = JSON.parse(context);
  const data = JSON.parse(payload);
  const playerId = data.player_id;

  // 业务逻辑
  await banPlayer(playerId);

  return JSON.stringify({ status: "success" });
};
```

---

### FunctionDescriptor

函数描述符接口。

```typescript
interface FunctionDescriptor {
  // 必填字段
  id: string; // 函数 ID，格式: [namespace.]entity.operation
  version: string; // 语义化版本号

  // 可选字段
  name?: string; // 显示名称
  description?: string; // 描述
  input_schema?: Record<string, any>; // 输入 JSON Schema
  output_schema?: Record<string, any>; // 输出 JSON Schema
}
```

**使用示例:**

```typescript
const descriptor: FunctionDescriptor = {
  id: "player.ban",
  version: "1.0.0",
  name: "封禁玩家",
  description: "封禁指定玩家账号",
  input_schema: {
    type: "object",
    required: ["player_id"],
    properties: {
      player_id: { type: "string" },
      reason: { type: "string" },
    },
  },
};
```

---

### ClientConfig

客户端配置接口。

```typescript
interface ClientConfig {
  // 连接配置
  agentAddr?: string; // Agent 地址，默认 "tcp://127.0.0.1:19090"
  controlAddr?: string; // 控制面地址，用于上传 provider manifest
  timeout?: number; // 连接超时（毫秒），默认 30000
  insecure?: boolean; // 是否跳过 TLS，默认 true
  serverName?: string; // TLS Server Name

  // 服务标识
  agentId?: string; // Agent 标识符
  serviceId?: string; // 服务标识符
  serviceVersion?: string; // 服务版本，默认 "1.0.0"
  gameId?: string; // 游戏标识符
  env?: string; // 环境，默认 "development"

  // 心跳配置
  heartbeatIntervalSeconds?: number; // 心跳间隔（秒），默认 60

  // Provider 元数据
  providerLang?: string; // Provider 语言，默认 "node"
  providerSdk?: string; // SDK 标识，默认 "croupier-js-sdk"

  // 认证与元数据
  authToken?: string; // Bearer Token
  headers?: Record<string, string>; // 默认 metadata / headers

  // 重连与重试
  autoReconnect?: boolean; // 是否自动重连，默认 true
  reconnect?: ReconnectConfig;
  retry?: RetryConfig;

  // 文件传输
  enableFileTransfer?: boolean; // 是否启用文件传输
  maxFileSize?: number; // 最大文件大小，默认 10 MiB

  // 日志
  disableLogging?: boolean;
  debugLogging?: boolean;
  logLevel?: "DEBUG" | "INFO" | "WARN" | "ERROR" | "OFF";
}
```

**使用示例:**

```typescript
const config: ClientConfig = {
  agentAddr: "tcp://127.0.0.1:19090",
  controlAddr: "tcp://127.0.0.1:19100",
  insecure: true,
  serviceId: "player-service",
  serviceVersion: "1.0.0",
  gameId: "my-game",
  env: "staging",
  heartbeatIntervalSeconds: 60,
  authToken: process.env.CROUPIER_AUTH_TOKEN,
  headers: {
    "X-Client": "player-service",
  },
};
```

---

### FileUploadRequest

文件上传请求接口。

```typescript
interface FileUploadRequest {
  filePath: string; // 文件路径
  content: Buffer | string; // 文件内容
  metadata?: Record<string, any>; // 元数据
  mimeType?: string; // 可选 MIME 类型覆盖
  onProgress?: (progress: FileUploadProgress) => void; // 预检进度回调
}
```

```typescript
interface FileUploadProgress {
  filePath: string;
  loaded: number;
  total?: number;
  percent?: number;
}
```

```typescript
interface FileUploadBatchItemResult {
  ok: boolean;
  filePath: string;
  result?: FileUploadResult;
  error?: string;
}
```

---

## CroupierClient 接口

客户端接口定义。

```typescript
interface CroupierClient {
  // 连接到 Agent
  connect(): Promise<void>;

  // 断开连接
  disconnect(): Promise<void>;

  // 注册函数
  registerFunction(
    descriptor: FunctionDescriptor,
    handler: FunctionHandler,
  ): void;

  // 同步调用
  invoke(
    functionId: string,
    payload: string,
    optionsOrMetadata?: InvokeOptions | Record<string, string>,
  ): Promise<string>;

  // 异步任务
  startTask(
    functionId: string,
    payload: string,
    optionsOrMetadata?: InvokeOptions | Record<string, string>,
  ): string;
  streamTask(taskId: string): AsyncIterable<TaskEvent>;
  cancelTask(taskId: string): boolean;

  // 运行模式
  serve(): Promise<void>;
  serveAsync(): Promise<void>;
}
```

---

## BasicClient 类

主客户端类，实现 CroupierClient 接口。

### 构造函数

```typescript
constructor(config?: ClientConfig)
```

**参数:**

- `config`: 客户端配置，可选，使用默认配置

### 公共方法

#### connect

连接到 Agent。

```typescript
async connect(): Promise<void>
```

**异常:**

- `Error`: 未注册任何函数时抛出
- `Error`: 连接失败时抛出

---

#### disconnect

断开连接。

```typescript
async disconnect(): Promise<void>
```

---

#### registerFunction

注册函数处理器。

```typescript
async registerFunction(
  descriptor: FunctionDescriptor,
  handler: FunctionHandler
): Promise<void>
```

**参数:**

- `descriptor`: 函数描述符
- `handler`: 函数处理器

**异常:**

- `Error`: 已连接时注册抛出
- `Error`: 描述符缺少 id 或 version 时抛出

---

#### uploadFile

上传文件并执行客户端侧预检。

```typescript
async uploadFile(request: FileUploadRequest): Promise<FileUploadResult>
```

当前实现会完成以下检查：

- 文件传输功能是否启用
- 文件路径是否有效
- 文件大小是否超过 `maxFileSize`
- 文件扩展名是否命中 `allowedExtensions`
- MIME 类型是否命中 `allowedMimeTypes`
- 生成 `sha256` 校验值和标准化元数据
- 触发一次 `onProgress` 完成事件

返回值中的 `status` 当前固定为 `validated`，表示客户端预检已完成。

---

#### uploadFileStream

从 Node.js 可读流上传文件并执行客户端侧预检。

```typescript
async uploadFileStream(
  request: FileUploadStreamRequest
): Promise<FileUploadResult>
```

支持对流式内容执行大小限制和超时控制，超时时间由 `uploadTimeout` 控制。
如果 `metadata.size` 提供了总字节数，进度回调会额外填充 `total` 和 `percent`。

---

#### uploadFiles

批量上传文件并执行客户端侧预检。

```typescript
async uploadFiles(
  requests: FileUploadRequest[]
): Promise<FileUploadBatchResult>
```

当前实现支持：

- 基于 `parallelUploads` 的并发预检
- 保持输入顺序返回 `items`
- 返回批量统计信息 `total/succeeded/failed`
- 对单个失败项返回 `items[n].error`，不会中断整个批次

---

## 工厂函数

### createClient

创建客户端实例的工厂函数。

```typescript
function createClient(config?: FileTransferConfig): CroupierClient;
```

**使用示例:**

```typescript
const client = createClient({
  agentAddr: "localhost:19090",
  insecure: true,
});
```

---

## 内部类型

### TaskEvent（来自 Proto 定义）

任务事件类型。

```typescript
interface TaskEvent {
  type: string; // 事件类型: "started"|"progress"|"completed"|"error"|"cancelled"
  message: string; // 事件消息
  progress: number; // 进度 0-100
  payload: Uint8Array; // 结果负载
}
```

---

## 完整示例

### Provider 示例

```typescript
import {
  createClient,
  FunctionDescriptor,
  FunctionHandler,
} from "croupier-sdk";

async function main() {
  // 创建客户端
  const client = createClient({
    agentAddr: "localhost:19090",
    insecure: true,
    serviceId: "player-service",
    serviceVersion: "1.0.0",
  });

  // 定义函数描述符
  const banDescriptor: FunctionDescriptor = {
    id: "player.ban",
    version: "1.0.0",
    name: "封禁玩家",
    description: "封禁指定玩家账号",
    input_schema: {
      type: "object",
      required: ["player_id"],
      properties: {
        player_id: { type: "string" },
        reason: { type: "string" },
      },
    },
  };

  // 定义处理器
  const banHandler: FunctionHandler = async (context, payload) => {
    const data = JSON.parse(payload);
    const playerId = data.player_id;
    const reason = data.reason || "未指定";

    console.log(`封禁玩家: ${playerId}, 原因: ${reason}`);

    // 业务逻辑
    await performBan(playerId, reason);

    return JSON.stringify({ status: "success" });
  };

  // 注册函数
  await client.registerFunction(banDescriptor, banHandler);

  // 注册更多函数
  await client.registerFunction(
    { id: "player.get", version: "1.0.0" },
    async (ctx, payload) => {
      const { player_id } = JSON.parse(payload);
      const player = await getPlayer(player_id);
      return JSON.stringify(player);
    },
  );

  // 连接到 Agent
  await client.connect();
  console.log("服务已启动");

  // 优雅退出
  process.on("SIGINT", async () => {
    console.log("正在关闭...");
    await client.disconnect();
    process.exit(0);
  });

  // 保持进程运行
  await new Promise(() => {});
}

async function performBan(playerId: string, reason: string) {
  // 实际业务逻辑
}

async function getPlayer(playerId: string) {
  // 实际业务逻辑
  return { id: playerId, name: "Player" };
}

main().catch(console.error);
```

### Express 集成示例

```typescript
import express from "express";
import { createClient, CroupierClient } from "croupier-sdk";

const app = express();
let croupierClient: CroupierClient;

// 初始化 Croupier 客户端
async function initCroupier() {
  croupierClient = createClient({
    agentAddr: process.env.CROUPIER_AGENT_ADDR || "localhost:19090",
    insecure: process.env.NODE_ENV !== "production",
    serviceId: "express-service",
    serviceVersion: "1.0.0",
  });

  // 注册 HTTP 触发的函数
  await croupierClient.registerFunction(
    { id: "api.health", version: "1.0.0" },
    () => JSON.stringify({ status: "healthy", timestamp: Date.now() }),
  );

  await croupierClient.registerFunction(
    { id: "user.create", version: "1.0.0" },
    async (ctx, payload) => {
      const user = JSON.parse(payload);
      // 创建用户逻辑
      return JSON.stringify({ id: "new-user-id", ...user });
    },
  );

  await croupierClient.connect();
  console.log("Croupier 客户端已连接");
}

// Express 路由
app.get("/health", (req, res) => {
  res.json({ status: "ok" });
});

// 启动服务器
const PORT = process.env.PORT || 3000;

initCroupier()
  .then(() => {
    app.listen(PORT, () => {
      console.log(`服务器运行在端口 ${PORT}`);
    });
  })
  .catch((err) => {
    console.error("初始化失败:", err);
    process.exit(1);
  });

// 优雅退出
process.on("SIGTERM", async () => {
  await croupierClient?.disconnect();
  process.exit(0);
});
```

### NestJS 集成示例

```typescript
// croupier.module.ts
import { Module, OnModuleInit, OnModuleDestroy } from "@nestjs/common";
import { createClient, CroupierClient } from "croupier-sdk";

@Module({
  providers: [
    {
      provide: "CROUPIER_CLIENT",
      useFactory: () => {
        return createClient({
          agentAddr: process.env.CROUPIER_AGENT_ADDR || "localhost:19090",
          insecure: process.env.NODE_ENV !== "production",
        });
      },
    },
  ],
  exports: ["CROUPIER_CLIENT"],
})
export class CroupierModule implements OnModuleInit, OnModuleDestroy {
  constructor(
    @Inject("CROUPIER_CLIENT") private readonly client: CroupierClient,
  ) {}

  async onModuleInit() {
    await this.client.registerFunction(
      { id: "service.status", version: "1.0.0" },
      () => JSON.stringify({ status: "running" }),
    );
    await this.client.connect();
  }

  async onModuleDestroy() {
    await this.client.disconnect();
  }
}

// player.service.ts
import { Injectable, Inject } from "@nestjs/common";
import { CroupierClient } from "croupier-sdk";

@Injectable()
export class PlayerService {
  constructor(
    @Inject("CROUPIER_CLIENT") private readonly croupier: CroupierClient,
  ) {}

  async registerHandlers() {
    await this.croupier.registerFunction(
      { id: "player.get", version: "1.0.0" },
      async (ctx, payload) => {
        const { id } = JSON.parse(payload);
        const player = await this.findPlayer(id);
        return JSON.stringify(player);
      },
    );
  }

  private async findPlayer(id: string) {
    // 实际数据库查询
    return { id, name: "Player" };
  }
}
```

### 同步处理器示例

```typescript
import { createClient } from "croupier-sdk";

const client = createClient({
  agentAddr: "localhost:19090",
  insecure: true,
});

// 同步处理器（不使用 async）
await client.registerFunction(
  { id: "math.add", version: "1.0.0" },
  (ctx, payload) => {
    const { a, b } = JSON.parse(payload);
    return JSON.stringify({ result: a + b });
  },
);

// 异步处理器
await client.registerFunction(
  { id: "data.fetch", version: "1.0.0" },
  async (ctx, payload) => {
    const { url } = JSON.parse(payload);
    const response = await fetch(url);
    const data = await response.json();
    return JSON.stringify(data);
  },
);

await client.connect();
```

### 错误处理示例

```typescript
import { createClient } from "croupier-sdk";

const client = createClient({
  agentAddr: "localhost:19090",
  insecure: true,
});

// 带错误处理的处理器
await client.registerFunction(
  { id: "user.validate", version: "1.0.0" },
  async (ctx, payload) => {
    try {
      const { email, password } = JSON.parse(payload);

      if (!email) {
        return JSON.stringify({
          status: "error",
          code: "INVALID_PARAM",
          message: "Email is required",
        });
      }

      if (password.length < 8) {
        return JSON.stringify({
          status: "error",
          code: "INVALID_PARAM",
          message: "Password must be at least 8 characters",
        });
      }

      const user = await validateUser(email, password);
      return JSON.stringify({ status: "success", data: user });
    } catch (error) {
      console.error("Validation error:", error);
      return JSON.stringify({
        status: "error",
        code: "INTERNAL_ERROR",
        message: error instanceof Error ? error.message : "Unknown error",
      });
    }
  },
);

// 连接错误处理
try {
  await client.connect();
} catch (error) {
  console.error("连接失败:", error);
  // 重试逻辑或退出
  process.exit(1);
}

// 优雅退出
async function shutdown() {
  try {
    await client.disconnect();
    console.log("已断开连接");
  } catch (error) {
    console.error("断开连接失败:", error);
  }
  process.exit(0);
}

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
```

### TypeScript 类型安全示例

```typescript
import {
  createClient,
  FunctionDescriptor,
  FunctionHandler,
} from "croupier-sdk";

// 定义请求和响应类型
interface BanRequest {
  player_id: string;
  reason?: string;
  duration_hours?: number;
}

interface BanResponse {
  status: "success" | "error";
  message?: string;
  banned_until?: string;
}

interface PlayerInfo {
  id: string;
  name: string;
  level: number;
  created_at: string;
}

// 类型安全的处理器工厂
function createTypedHandler<TReq, TRes>(
  handler: (ctx: Record<string, string>, req: TReq) => Promise<TRes> | TRes,
): FunctionHandler {
  return async (context: string, payload: string) => {
    const ctx = JSON.parse(context) as Record<string, string>;
    const req = JSON.parse(payload) as TReq;
    const res = await handler(ctx, req);
    return JSON.stringify(res);
  };
}

// 使用类型安全的处理器
const client = createClient({ agentAddr: "localhost:19090", insecure: true });

await client.registerFunction(
  { id: "player.ban", version: "1.0.0" },
  createTypedHandler<BanRequest, BanResponse>(async (ctx, req) => {
    // req 具有完整的类型提示
    const { player_id, reason, duration_hours = 24 } = req;

    await banPlayer(player_id, reason, duration_hours);

    return {
      status: "success",
      banned_until: new Date(
        Date.now() + duration_hours * 3600000,
      ).toISOString(),
    };
  }),
);

await client.registerFunction(
  { id: "player.get", version: "1.0.0" },
  createTypedHandler<{ player_id: string }, PlayerInfo>(async (ctx, req) => {
    const player = await getPlayer(req.player_id);
    return player;
  }),
);

await client.connect();

// 辅助函数
async function banPlayer(id: string, reason?: string, hours?: number) {
  // 实现
}

async function getPlayer(id: string): Promise<PlayerInfo> {
  return {
    id,
    name: "Player",
    level: 1,
    created_at: new Date().toISOString(),
  };
}
```

---

## 环境变量

| 环境变量              | 说明         | 默认值            |
| --------------------- | ------------ | ----------------- |
| `CROUPIER_AGENT_ADDR` | Agent 地址   | `127.0.0.1:19090` |
| `CROUPIER_INSECURE`   | 是否跳过 TLS | `true`            |
| `NODE_ENV`            | 环境模式     | -                 |

---

## 错误处理

### 常见错误

| 错误消息                                           | 原因             | 解决方案                  |
| -------------------------------------------------- | ---------------- | ------------------------- |
| `Register at least one function before connecting` | 未注册函数就连接 | 先调用 `registerFunction` |
| `Cannot register new functions while connected`    | 连接后注册函数   | 先调用 `disconnect`       |
| `Function descriptor must include id and version`  | 描述符不完整     | 提供 id 和 version        |
| `Function not registered`                          | 调用未注册的函数 | 确保函数已注册            |

### 标准错误响应格式

```typescript
interface ErrorResponse {
  status: "error";
  code: string;
  message: string;
  details?: Record<string, any>;
}
```

**标准错误码:**

- `INVALID_PARAM`: 参数错误
- `INVALID_JSON`: JSON 格式错误
- `UNAUTHORIZED`: 未授权
- `FORBIDDEN`: 无权限
- `NOT_FOUND`: 资源不存在
- `INTERNAL_ERROR`: 内部错误

---

## 最佳实践

### 1. 函数 ID 命名

```typescript
// ✅ 正确
"player.get";
"player.ban";
"game.player.ban";

// ❌ 错误
"PlayerGet";
"player-get";
"getPlayer";
```

### 2. 错误处理

```typescript
// ✅ 总是返回标准格式的错误响应
const handler: FunctionHandler = async (ctx, payload) => {
  try {
    // 业务逻辑
    return JSON.stringify({ status: "success", data: result });
  } catch (error) {
    return JSON.stringify({
      status: "error",
      code: "INTERNAL_ERROR",
      message: error.message,
    });
  }
};
```

### 3. 资源清理

```typescript
// ✅ 使用优雅退出
const client = createClient(config);
await client.connect();

const shutdown = async () => {
  await client.disconnect();
  process.exit(0);
};

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
```

### 4. 配置管理

```typescript
// ✅ 使用环境变量
const client = createClient({
  agentAddr: process.env.CROUPIER_AGENT_ADDR || "localhost:19090",
  insecure: process.env.NODE_ENV !== "production",
  serviceId: process.env.SERVICE_ID || "my-service",
});
```
