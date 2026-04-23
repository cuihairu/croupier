# Croupier JS/TS SDK 约定规范

本文档详细说明使用 Croupier JavaScript/TypeScript SDK 时需要遵守的约定和规范。

## 目录

- [命名约定](#命名约定)
- [函数注册约定](#函数注册约定)
- [虚拟对象设计约定](#虚拟对象设计约定)
- [错误处理约定](#错误处理约定)
- [版本管理约定](#版本管理约定)
- [安全约定](#安全约定)
- [避让规则](#避让规则)

---

## 命名约定

### 函数 ID 命名

函数 ID 采用 `entity.operation` 格式，描述函数所属实体和执行的操作。

#### 格式规则

```
[namespace.]entity.operation
```

| 部分               | 说明                   | 示例                                       |
| ------------------ | ---------------------- | ------------------------------------------ |
| `namespace` (可选) | 命名空间，用于模块分组 | `game`, `inventory`, `chat`                |
| `entity`           | 实体名称，小写         | `player`, `item`, `guild`                  |
| `operation`        | 操作名称，小写动词     | `get`, `create`, `update`, `delete`, `ban` |

#### 命名示例

```typescript
// ✅ 正确的函数 ID
"player.get"; // 获取玩家信息
"player.ban"; // 封禁玩家
"player.updateProfile"; // 更新玩家资料
"item.create"; // 创建道具
"item.delete"; // 删除道具
"wallet.transfer"; // 钱包转账
"game.player.ban"; // 带命名空间
"inventory.item.add"; // 带命名空间

// ❌ 错误的函数 ID
"PlayerGet"; // 不要使用驼峰命名
"player-get"; // 不要使用连字符
"player_get"; // 不要使用下划线
"get_player"; // 实体应该在前
"player"; // 缺少操作
""; // 不能为空
```

### 实体命名

实体代表业务领域的对象，使用**小写单数名词**。

```typescript
// ✅ 推荐的实体名称
"player"; // 玩家
"item"; // 道具
"guild"; // 公会
"wallet"; // 钱包
"match"; // 比赛
"chat"; // 聊天

// ❌ 避免使用
"players"; // 不要使用复数
"Player"; // 不要使用大写
"playerData"; // 不要使用驼峰
```

### 操作命名

操作使用**小写动词**，表示对实体执行的动作。

#### CRUD 标准操作

| 操作     | 说明         | 适用场景                   |
| -------- | ------------ | -------------------------- |
| `create` | 创建新实体   | 新建玩家、创建道具         |
| `get`    | 获取实体信息 | 查询玩家、查询道具         |
| `update` | 更新实体信息 | 修改玩家属性、更新道具状态 |
| `delete` | 删除实体     | 删除玩家、删除道具         |
| `list`   | 列出实体集合 | 查询玩家列表、查询道具列表 |

#### 业务操作

| 操作       | 说明      | 示例                                           |
| ---------- | --------- | ---------------------------------------------- |
| `ban`      | 封禁/禁用 | `player.ban`                                   |
| `unban`    | 解封/启用 | `player.unban`                                 |
| `transfer` | 转移      | `wallet.transfer`, `item.transfer`             |
| `add`      | 添加      | `inventory.item.add`, `guild.member.add`       |
| `remove`   | 移除      | `inventory.item.remove`, `guild.member.remove` |
| `start`    | 启动      | `match.start`                                  |
| `end`      | 结束      | `match.end`                                    |
| `join`     | 加入      | `match.join`                                   |
| `leave`    | 离开      | `match.leave`                                  |

---

## 函数注册约定

### 注册时机

所有函数必须在调用 `connect()` **之前**完成注册。

```typescript
const client = new CroupierClient(config);

// ✅ 正确：先注册，后连接
client.registerFunction(descriptor1, handler1);
client.registerFunction(descriptor2, handler2);
await client.connect(); // 连接时会将所有注册的函数上传到 Agent

// ❌ 错误：连接后不能再注册
await client.connect();
client.registerFunction(descriptor3, handler3); // 无效，会失败
```

### 函数唯一性

同一个服务实例内，函数 ID 必须唯一。

```typescript
// ❌ 错误：重复注册相同 ID
const desc1: FunctionDescriptor = {
  id: "player.get",
  version: "1.0.0",
};

const desc2: FunctionDescriptor = {
  id: "player.get", // 与 desc1 相同
  version: "1.0.0",
};

client.registerFunction(desc1, handler1);
client.registerFunction(desc2, handler2); // 会覆盖或报错
```

### FunctionDescriptor 必填字段

```typescript
const descriptor: FunctionDescriptor = {
  // ✅ 必填字段
  id: "player.get", // 函数 ID，不能为空
  version: "1.0.0", // 版本号，不能为空

  // ⭐ 推荐字段
  category: "player", // 业务分类
  risk: "low", // 风险等级: low, medium, high
  entity: "player", // 关联实体
  operation: "read", // 操作类型: create, read, update, delete
  enabled: true, // 是否启用
};
```

### 风险等级 (Risk)

风险等级用于标识函数操作的影响范围和重要性。

| 等级     | 说明               | 示例                                             |
| -------- | ------------------ | ------------------------------------------------ |
| `low`    | 只读操作，无副作用 | `player.get`, `item.list`                        |
| `medium` | 有副作用但可逆     | `player.update`, `item.add`                      |
| `high`   | 有重大影响或不可逆 | `player.delete`, `player.ban`, `wallet.transfer` |

```typescript
const descriptor: FunctionDescriptor = {
  id: "player.ban",
  version: "1.0.0",
  risk: "high", // 封禁是高风险操作
};
```

---

## 虚拟对象设计约定

### 虚拟对象结构

虚拟对象将相关的 CRUD 操作组合在一起，形成完整的实体管理单元。

```typescript
const player: VirtualObjectDescriptor = {
  // 基础信息
  id: "player",
  version: "1.0.0",
  name: "游戏玩家",
  description: "管理玩家信息",

  // Schema 定义（可选）
  schema: {
    type: "object",
    properties: {
      id: { type: "string" },
      name: { type: "string" },
      level: { type: "integer" },
    },
  },

  // 操作映射
  operations: {
    create: "player.create",
    read: "player.get",
    update: "player.update",
    delete: "player.delete",
    ban: "player.ban",
    unban: "player.unban",
  },
};
```

### 标准操作映射

虚拟对象应尽可能支持标准 CRUD 操作：

| 操作     | 函数 ID           | 说明         |
| -------- | ----------------- | ------------ |
| `create` | `{entity}.create` | 创建新实体   |
| `read`   | `{entity}.get`    | 读取实体信息 |
| `update` | `{entity}.update` | 更新实体信息 |
| `delete` | `{entity}.delete` | 删除实体     |

---

## 错误处理约定

### 函数处理器错误处理

函数处理器应该捕获所有异常并返回标准错误响应。

```typescript
async function safeHandler(
  context: CallContext,
  payload: Uint8Array,
): Promise<string> {
  try {
    // 1. 解析输入
    const data = JSON.parse(new TextDecoder().decode(payload));

    // 2. 验证参数
    if (!data.player_id || typeof data.player_id !== "string") {
      return errorResponse("INVALID_PARAM", "player_id is required");
    }

    // 3. 执行业务逻辑
    const result = await processPlayer(data.player_id);

    // 4. 返回成功响应
    return successResponse(result);
  } catch (error) {
    if (error instanceof SyntaxError) {
      return errorResponse("INVALID_JSON", error.message);
    }
    return errorResponse("INTERNAL_ERROR", error.message);
  }
}

function errorResponse(code: string, message: string): string {
  return JSON.stringify({
    status: "error",
    code,
    message,
  });
}

function successResponse(data: unknown): string {
  return JSON.stringify({
    status: "success",
    data,
  });
}
```

### 标准错误响应格式

所有错误响应应遵循统一格式：

```json
{
  "status": "error",
  "code": "ERROR_CODE",
  "message": "Human readable error message",
  "details": {}
}
```

### 常见错误码

| 错误码                | 说明          | HTTP 等价 |
| --------------------- | ------------- | --------- |
| `INVALID_PARAM`       | 参数错误      | 400       |
| `INVALID_JSON`        | JSON 格式错误 | 400       |
| `UNAUTHORIZED`        | 未授权        | 401       |
| `FORBIDDEN`           | 无权限        | 403       |
| `NOT_FOUND`           | 资源不存在    | 404       |
| `ALREADY_EXISTS`      | 资源已存在    | 409       |
| `INTERNAL_ERROR`      | 内部错误      | 500       |
| `SERVICE_UNAVAILABLE` | 服务不可用    | 503       |

---

## 版本管理约定

### 语义化版本

SDK 遵循语义化版本规范 (Semver)：`MAJOR.MINOR.PATCH`

| 部分    | 说明                         | 示例          |
| ------- | ---------------------------- | ------------- |
| `MAJOR` | 主版本号，不兼容的 API 变更  | 1.0.0 → 2.0.0 |
| `MINOR` | 次版本号，向后兼容的功能新增 | 1.0.0 → 1.1.0 |
| `PATCH` | 修订号，向后兼容的问题修正   | 1.0.0 → 1.0.1 |

### 版本兼容性

```typescript
// ✅ 兼容变更：增加 PATCH
// 1.0.0 → 1.0.1
descriptor.version = "1.0.1"; // 修复 bug

// ✅ 兼容变更：增加 MINOR
// 1.0.0 → 1.1.0
descriptor.version = "1.1.0"; // 新增可选参数

// ❌ 不兼容变更：增加 MAJOR
// 1.0.0 → 2.0.0
descriptor.version = "2.0.0"; // 修改参数结构
```

---

## 安全约定

### 输入验证

所有函数处理器必须验证输入参数：

```typescript
async function secureHandler(
  context: CallContext,
  payload: Uint8Array,
): Promise<string> {
  // 1. 检查 payload 是否为空
  if (payload.length === 0) {
    return errorResponse("EMPTY_PAYLOAD", "payload is empty");
  }

  // 2. 验证 JSON 格式
  let data: unknown;
  try {
    data = JSON.parse(new TextDecoder().decode(payload));
  } catch (error) {
    return errorResponse("INVALID_JSON", "Invalid JSON format");
  }

  // 3. 验证是否为对象
  if (typeof data !== "object" || data === null) {
    return errorResponse("INVALID_PAYLOAD", "Payload must be an object");
  }

  const payloadObj = data as Record<string, unknown>;

  // 4. 验证必需字段
  if (!payloadObj.player_id) {
    return errorResponse("MISSING_PLAYER_ID", "player_id is required");
  }

  // 5. 类型检查
  if (typeof payloadObj.player_id !== "string") {
    return errorResponse("INVALID_PLAYER_ID_TYPE", "player_id must be string");
  }

  // 6. 值范围检查
  const playerId = payloadObj.player_id as string;
  if (playerId.length === 0 || playerId.length > 64) {
    return errorResponse(
      "INVALID_PLAYER_ID_VALUE",
      "player_id length must be 1-64",
    );
  }

  // 7. 业务逻辑
  return processRequest(payloadObj);
}
```

### TLS 配置

生产环境必须启用 TLS：

```typescript
const config: ClientConfig = {
  agentAddr: "agent.croupier.io:443",
  insecure: false, // 生产环境必须为 false
  certFile: "/etc/tls/client.crt",
  keyFile: "/etc/tls/client.key",
  caFile: "/etc/tls/ca.crt",
  serverName: "agent.croupier.io",
};
```

---

## 避让规则

### 函数注册避让

当多个服务实例注册相同函数时，Agent 会根据以下规则路由请求：

1. **版本优先级**：优先路由到高版本的函数

   ```
   player.get v2.0.0 > player.get v1.1.0 > player.get v1.0.0
   ```

2. **负载均衡**：同版本函数之间进行负载分配

3. **健康检查**：不健康的实例不会被路由

```typescript
// 设置合适的版本号
descriptor.version = "2.0.0"; // 新版本会优先处理请求
```

### 服务优先级

通过 `risk` 字段影响路由优先级：

```typescript
// 高风险函数会被更谨慎地路由
const highRiskDesc: FunctionDescriptor = {
  id: "player.ban",
  risk: "high",
};

// 低风险函数可以更自由地负载均衡
const lowRiskDesc: FunctionDescriptor = {
  id: "player.get",
  risk: "low",
};
```

### 优雅降级

当服务不可用时，返回降级响应而非错误：

```typescript
async function gracefulDegradeHandler(
  context: CallContext,
  payload: Uint8Array,
): Promise<string> {
  try {
    return await processRequest(payload);
  } catch (error) {
    // 返回降级响应
    return JSON.stringify({
      status: "degraded",
      message: "Service temporarily unavailable",
      cached: true,
    });
  }
}
```

---

## 完整示例

### 符合约定的完整实现

```typescript
import {
  CroupierClient,
  FunctionDescriptor,
  CallContext,
  ClientConfig,
} from "@croupier/sdk";

// 玩家管理处理器 - 符合所有约定
class PlayerHandlers {
  // 获取玩家信息 (low risk, read operation)
  static async get(context: CallContext, payload: Uint8Array): Promise<string> {
    try {
      const data = JSON.parse(new TextDecoder().decode(payload));

      // 参数验证
      if (!data.player_id || typeof data.player_id !== "string") {
        return PlayerHandlers.error("MISSING_PARAM", "player_id is required");
      }

      // 业务逻辑
      const result = {
        id: data.player_id,
        name: "Player One",
        level: 50,
      };

      return PlayerHandlers.success(result);
    } catch (error) {
      if (error instanceof SyntaxError) {
        return PlayerHandlers.error("INVALID_JSON", error.message);
      }
      return PlayerHandlers.error("INTERNAL_ERROR", error.message);
    }
  }

  // 更新玩家信息 (medium risk, update operation)
  static async update(
    context: CallContext,
    payload: Uint8Array,
  ): Promise<string> {
    try {
      const data = JSON.parse(new TextDecoder().decode(payload));

      if (!data.player_id) {
        return PlayerHandlers.error("MISSING_PARAM", "player_id is required");
      }

      // 业务逻辑
      return PlayerHandlers.success({
        updated_fields: ["level", "exp"],
      });
    } catch (error) {
      return PlayerHandlers.error("INTERNAL_ERROR", error.message);
    }
  }

  // 封禁玩家 (high risk, sensitive operation)
  static async ban(context: CallContext, payload: Uint8Array): Promise<string> {
    try {
      const data = JSON.parse(new TextDecoder().decode(payload));

      // 验证操作者权限（从 context 获取）
      // 高风险操作需要额外验证

      return PlayerHandlers.success({
        action: "ban",
        player_id: data.player_id,
      });
    } catch (error) {
      return PlayerHandlers.error("INTERNAL_ERROR", error.message);
    }
  }

  private static error(code: string, message: string): string {
    return JSON.stringify({
      status: "error",
      code,
      message,
    });
  }

  private static success(data: unknown): string {
    return JSON.stringify({
      status: "success",
      data,
    });
  }
}

async function main() {
  // 配置
  const config: ClientConfig = {
    gameId: "my-game",
    env: "production",
    serviceId: "player-service",
    agentAddr: "agent.croupier.io:443",
    insecure: false, // 生产环境启用 TLS
  };

  const client = new CroupierClient(config);

  // 注册函数 - 遵循命名约定
  const getDesc: FunctionDescriptor = {
    id: "player.get",
    version: "1.0.0",
    category: "player",
    risk: "low",
    entity: "player",
    operation: "read",
  };

  const updateDesc: FunctionDescriptor = {
    id: "player.update",
    version: "1.0.0",
    category: "player",
    risk: "medium",
    entity: "player",
    operation: "update",
  };

  const banDesc: FunctionDescriptor = {
    id: "player.ban",
    version: "1.0.0",
    category: "player",
    risk: "high",
    entity: "player",
    operation: "update",
  };

  // 在连接前注册所有函数
  client.registerFunction(getDesc, PlayerHandlers.get);
  client.registerFunction(updateDesc, PlayerHandlers.update);
  client.registerFunction(banDesc, PlayerHandlers.ban);

  // 连接并启动服务
  await client.connect();
  console.log("服务已启动");

  // 保持运行
  await client.serve();
}

main().catch(console.error);
```
