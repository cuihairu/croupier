# Croupier SDK 行为规范

本文档定义了所有 Croupier SDK 必须遵守的行为规范，确保跨语言 SDK 的一致性和可预测性。

## 目录

- [核心接口规范](#核心接口规范)
- [配置规范](#配置规范)
- [错误处理规范](#错误处理规范)
- [生命周期管理规范](#生命周期管理规范)
- [网络行为规范](#网络行为规范)
- [安全规范](#安全规范)
- [日志规范](#日志规范)
- [测试规范](#测试规范)

---

## 核心接口规范

### 1. Client 接口

所有 SDK **必须**实现 `Client` 接口，用于向 Agent 注册函数并接收调用。

#### 必需方法

| 方法名 | 签名 | 行为规范 |
|--------|------|----------|
| `registerFunction` | `(descriptor, handler) -> void` | 在连接前注册函数，连接后禁止注册 |
| `connect` | `() -> Promise/void` | 建立与 Agent 的连接，启动本地 gRPC 服务器 |
| `serve` | `() -> Promise/void` | 阻塞当前线程/协程，直到调用 `stop()` |
| `stop` | `() -> Promise/void` | 优雅停止服务，清理资源 |
| `close` | `() -> Promise/void` | 完全关闭客户端，释放所有资源 |
| `getLocalAddress` | `() -> string` | 返回本地 gRPC 服务器地址 |

#### 行为约束

1. **函数注册时机**
   - 函数只能在 `connect()` **之前**注册
   - `connect()` 后调用 `registerFunction()` **必须**抛出错误
   - 空函数列表（0个函数）调用 `connect()` **必须**抛出错误

2. **连接状态管理**
   - `isConnected()` 必须准确反映连接状态
   - 重复调用 `connect()` **应该**幂等（不应报错，应直接返回）

3. **线程/并发安全**
   - `registerFunction()` 必须线程安全
   - `connect()`/`stop()` 必须与函数调用并发安全

### 2. Invoker 接口

所有 SDK **必须**实现 `Invoker` 接口，用于调用远程注册的函数。

#### 必需方法

| 方法名 | 签名 | 行为规范 |
|--------|------|----------|
| `connect` | `() -> Promise/void` | 建立与服务器的连接 |
| `invoke` | `(functionId, payload, options) -> string` | 同步调用函数，返回结果 |
| `startJob` | `(functionId, payload, options) -> jobId` | 启动异步任务，返回任务ID |
| `streamJob` | `(jobId) -> Stream<JobEvent>` | 流式获取任务事件 |
| `cancelJob` | `(jobId) -> void` | 取消正在运行的任务 |
| `close` | `() -> Promise/void` | 关闭连接，释放资源 |

#### 行为约束

1. **自动连接**
   - `invoke()` 和 `startJob()` 在未连接时**应该**自动连接

2. **幂等性**
   - 使用相同 `idempotencyKey` 的请求**必须**返回相同结果
   - 幂等性键的有效期由服务器决定

3. **流式事件**
   - `streamJob()` 必须在任务完成时结束流
   - 必须支持事件类型：`started`, `progress`, `completed`, `error`, `cancelled`

---

## 配置规范

### ClientConfig 标准字段

所有 SDK 的 `ClientConfig` **必须**支持以下字段：

| 字段名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `agentAddr` | string | 否 | `"127.0.0.1:19090"` | Agent gRPC 地址 |
| `controlAddr` | string | 否 | `""` | 控制面地址（可选） |
| `serviceId` | string | 否 | 自动生成 | 唯一服务标识符 |
| `serviceVersion` | string | 否 | `"1.0.0"` | 服务版本 |
| `gameId` | string | 否 | `""` | 游戏标识符（多租户） |
| `env` | string | 否 | `"development"` | 环境标识 |
| `localListen` | string | 否 | `"127.0.0.1:0"` | 本地监听地址 |
| `timeout` | number | 否 | `30000` | 连接超时（毫秒） |
| `insecure` | boolean | 否 | `true` | 是否使用不安全连接（开发用） |
| `heartbeatIntervalSeconds` | number | 否 | `60` | 心跳间隔（秒） |

### TLS 配置标准字段

| 字段名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `caFile` | string | 否 | 系统证书 | CA 证书文件路径 |
| `certFile` | string | 否 | 无 | 客户端证书文件路径 |
| `keyFile` | string | 否 | 无 | 客户端私钥文件路径 |
| `serverName` | string | 否 | 从地址提取 | TLS 服务器名称验证 |
| `insecureSkipVerify` | boolean | 否 | `false` | 跳过证书验证（不推荐） |

### 文件传输配置标准字段

| 字段名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `enableFileTransfer` | boolean | 否 | `false` | **是否启用文件传输功能（默认关闭）** |
| `maxFileSize` | number | 否 | `10485760` (10MB) | 单个文件最大大小（字节） |
| `allowedExtensions` | string[] | 否 | `[]` | 允许的文件扩展名（白名单） |
| `allowedMimeTypes` | string[] | 否 | `[]` | 允许的 MIME 类型（白名单） |
| `uploadTimeout` | number | 否 | `300000` (5分钟) | 文件上传超时（毫秒） |

**重要安全要求：**
- 文件传输**必须默认关闭**
- 启用时**必须**配置文件大小限制
- **必须**使用扩展名或 MIME 类型白名单（拒绝所有其他类型）
- 上传的文件**必须**在服务端进行二次验证

---

## 错误处理规范

### 错误类型标准

所有 SDK **必须**区分以下错误类型：

| 错误类型 | HTTP/gRPC 码 | 触发条件 |
|----------|--------------|----------|
| `InvalidArgument` | 400/InvalidArgument | 参数无效或缺失 |
| `NotFound` | 404/NotFound | 函数或任务不存在 |
| `AlreadyExists` | 409/AlreadyExists | 重复注册 |
| `Unauthenticated` | 401/Unauthenticated | 认证失败 |
| `PermissionDenied` | 403/PermissionDenied | 权限不足 |
| `Internal` | 500/Internal | 内部错误 |
| `Unavailable` | 503/Unavailable | 服务不可用 |

### 错误处理行为

1. **错误传播**
   - Handler 内部错误**必须**包装为 `Internal` 错误
   - 网络错误**必须**传播给调用者

2. **错误信息**
   - 错误消息**必须**包含足够的调试信息
   - 不应泄露敏感信息（密码、密钥等）

---

## 生命周期管理规范

### 启动流程

```
1. 创建 Client 实例
2. 调用 registerFunction() 注册所有函数
3. 调用 connect() 建立连接
4. 调用 serve() 进入服务循环
```

### 关闭流程

```
1. 调用 stop() 停止接受新请求
2. 等待进行中的请求完成（超时时间可配置）
3. 关闭本地 gRPC 服务器
4. 停止心跳
5. 关闭与 Agent 的连接
```

### 状态转换

```
      ┌─────────┐
      │  New   │
      └────┬────┘
           │ registerFunction()
           ▼
      ┌─────────┐
      │Registered│
      └────┬────┘
           │ connect()
           ▼
      ┌─────────┐  stop()
   ┌──▶│Connected│◀─────┐
   │   └─────────┘      │
   │                    │
   │                    │
   │   ┌─────────┐      │
   └───│  Closed │◀─────┘
       └─────────┘
```

---

## 网络行为规范

### 心跳机制

1. **默认行为**
   - 心跳间隔默认 60 秒
   - 心跳失败**应该**记录警告但不中断服务
   - 心跳失败超过阈值（如 3 次）**应该**触发重连

2. **心跳负载**
   - 必须包含 `serviceId` 和 `sessionId`
   - 必须使用与注册相同的标识符

### 重连机制

1. **自动重连**
   - 网络错误后**应该**自动重连
   - 重连间隔**应该**使用指数退避策略
   - 最大重试次数**应该**可配置（默认无限制）

2. **重连状态**
   - 重连期间，`isConnected()` 返回 `false`
   - 重连成功后，**必须**重新注册函数

### 重试机制（指数退避）

**所有 SDK 应该实现可配置的重试机制**

1. **重试配置**
   - `enabled`: 是否启用重试（默认 true）
   - `maxAttempts`: 最大重试次数（默认 3）
   - `initialDelayMs`: 初始重试延迟（默认 100ms）
   - `maxDelayMs`: 最大重试延迟（默认 5000ms）
   - `backoffMultiplier`: 指数退避倍数（默认 2.0）
   - `jitterFactor`: 抖动因子，避免雷群效应（默认 0.1）

2. **重试行为**
   - 仅对可重试的错误码进行重试（如网络错误、超时）
   - 使用指数退避计算延迟：`delay = min(initialDelay * multiplier^attempt, maxDelay)`
   - 添加随机抖动：`finalDelay = delay * (1 ± jitterFactor)`
   - 达到最大重试次数后停止并返回最后错误

3. **可重试错误示例**
   - gRPC: `UNAVAILABLE` (14), `INTERNAL` (13), `UNKNOWN` (2)
   - HTTP: 503 Service Unavailable, 502 Bad Gateway

### 超时处理

| 操作 | 默认超时 | 行为 |
|------|----------|------|
| 连接 | 30 秒 | 抛出超时错误 |
| 调用 | 30 秒 | 抛出超时错误 |
| 心跳 | 10 秒 | 记录警告 |
| 关闭 | 5 秒 | 强制关闭 |

---

## 安全规范

### TLS/mTLS

1. **生产环境**
   - 生产环境**必须**使用 TLS
   - **禁止**使用 `insecure = true`

2. **证书验证**
   - 默认**必须**验证服务器证书
   - 支持自定义 CA 证书

3. **mTLS**
   - 支持双向认证
   - 客户端证书和私钥**必须**安全存储

### 敏感信息处理

1. **日志脱敏**
   - 日志中**禁止**输出密码、密钥
   - Token **必须**部分遮蔽（如 `abc...xyz`，显示前 3 和后 3 个字符）
   - API Key **必须**部分遮蔽
   - 敏感头信息（如 `Authorization`）**必须**脱敏

2. **脱敏格式要求**
   - 推荐格式：`前3位...后3位`，如 `eyJ0...iOiJ`
   - 如果值长度 ≤ 6 位，全部替换为 `*`：`******`
   - 日志输出示例：`Using auth token: eyJ0...iOiJ`

3. **SDK 实现要求**
   - **必须**提供 `MaskSensitive(value)` 工具函数
   - **必须**在所有日志输出中自动应用脱敏
   - **应该**提供 `MaskJsonSensitive()` 用于 JSON 负载脱敏

4. **错误信息**
   - 错误响应**禁止**包含内部路径或堆栈
   - **禁止**在错误消息中泄露敏感配置

### 文件传输安全

1. **默认关闭原则**
   - 文件传输功能**必须默认关闭**（`enableFileTransfer = false`）
   - **必须**通过显式配置才能启用
   - SDK 启动时如果检测到文件传输被启用，**应该**记录警告日志

2. **权限验证**
   - **必须**在调用任何文件传输方法前检查 `enableFileTransfer` 标志
   - 如果标志为 `false`，**必须**拒绝操作并返回 `PermissionDenied` 错误
   - 服务端**必须**二次验证客户端的文件传输权限

3. **文件类型白名单**
   - **必须**使用扩展名白名单（`.png`, `.jpg`, `.pdf` 等）
   - **必须**使用 MIME 类型白名单（`image/png`, `application/pdf` 等）
   - **必须**拒绝不在白名单中的文件类型
   - 白名单为空时**必须**拒绝所有文件上传

4. **文件大小限制**
   - **必须**限制单个文件大小（默认 10MB）
   - **必须**在传输前验证文件大小
   - 超过限制**必须**拒绝上传并返回错误

5. **安全扫描要求**
   - **应该**在上传后进行病毒扫描
   - **应该**验证文件内容类型与扩展名匹配
   - **禁止**执行上传的文件

---

## 日志规范

### 日志级别

| 级别 | 用途 | 示例 |
|------|------|------|
| `DEBUG` | 详细调试信息 | 函数调用详情 |
| `INFO` | 正常操作信息 | 函数注册成功 |
| `WARN` | 警告信息 | 心跳失败 |
| `ERROR` | 错误信息 | 连接失败 |

### 日志格式

推荐格式：
```
[timestamp] [level] [component] message
```

示例：
```
2024-01-06T10:30:00Z [INFO] [croupier] Registered function: player.ban
2024-01-06T10:30:05Z [ERROR] [croupier] Connection failed: dial tcp 127.0.0.1:19090: connection refused
```

### 可配置性

- **必须**支持禁用日志（`disableLogging`）
- **必须**支持调试模式（`debugLogging`）
- **应该**支持自定义日志输出

---

## 测试规范

### 必需测试

每个 SDK **必须**包含以下测试：

1. **单元测试**
   - 函数注册测试
   - 连接管理测试
   - 错误处理测试

2. **集成测试**
   - 与真实 Agent 的连接测试
   - 函数调用端到端测试
   - 异步任务测试

### 测试覆盖率

- 核心路径覆盖率 **应≥ 80%**
- 错误处理路径覆盖率 **应≥ 60%**

---

## 兼容性规范

### 版本兼容性

1. **SDK 版本**
   - 遵循语义化版本（SemVer）
   - 主版本变更可能包含破坏性更新

2. **协议兼容性**
   - SDK **必须**与同版本或更高版本的 Agent 兼容
   - 跨小版本（如 v1.x）**应该**兼容

### 平台支持

| SDK | 支持平台 | 最低版本 |
|-----|----------|----------|
| Go | Linux, macOS, Windows | Go 1.21 |
| JS | Node.js | Node.js 20 |
| Python | Linux, macOS, Windows | Python 3.10 |
| Java | Linux, macOS, Windows | Java 17 |
| C++ | Linux, macOS, Windows | C++17 |

---

## 示例代码模板

### 基础使用（所有 SDK 应遵循）

```pseudo
// 1. 创建配置
config = ClientConfig{
    agentAddr = "127.0.0.1:19090",
    serviceId = "my-service",
    serviceVersion = "1.0.0"
}

// 2. 创建客户端
client = createClient(config)

// 3. 注册函数
client.registerFunction(
    FunctionDescriptor{id = "func1", version = "1.0.0"},
    handler
)

// 4. 连接并服务
client.connect()
client.serve()  // 阻塞直到停止
```

### 调用函数（所有 SDK 应遵循）

```pseudo
// 1. 创建 Invoker
invoker = createInvoker(InvokerConfig{address = "127.0.0.1:8080"})

// 2. 调用函数
result = invoker.invoke(
    "func1",
    JSON.stringify({data: "value"}),
    InvokeOptions{idempotencyKey = "key-123"}
)

// 3. 异步任务
jobId = invoker.startJob("func1", payload)

// 4. 流式事件
for event in invoker.streamJob(jobId):
    print(f"Event: {event.type}")
    if event.done:
        break
```
