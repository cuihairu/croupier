# 模型 API 参考

## FunctionDescriptor

函数描述符，用于注册函数时定义函数的元数据。

```csharp
public class FunctionDescriptor
{
    // 必需属性
    public string Id { get; set; }           // 函数唯一标识符
    public string Version { get; set; }      // 函数版本
    public string Category { get; set; }     // 函数分类
    public string Risk { get; set; }         // 风险级别：low/medium/high/critical

    // 可选属性
    public string? Entity { get; set; }      // 操作实体类型
    public string? Operation { get; set; }   // 操作类型
    public bool Enabled { get; set; }        // 是否启用
    public string? DisplayName { get; set; } // 显示名称
    public string? Description { get; set; } // 函数描述
    public string? InputSchema { get; set; } // 输入参数 JSON Schema
    public string? OutputSchema { get; set;} // 输出结果 JSON Schema
    public Dictionary<string, string>? Tags { get; set; } // 标签

    // 方法
    public bool IsValid()                    // 验证描述符是否有效
    public string GetFullName()              // 获取完整标识符 "category.id"
}
```

**示例：**
```csharp
var descriptor = new FunctionDescriptor {
    // 必需
    Id = "player.ban",
    Version = "1.0.0",
    Category = "moderation",
    Risk = "high",

    // 可选
    Entity = "player",
    Operation = "ban",
    DisplayName = "封禁玩家",
    Description = "永久封禁指定玩家账号",
    Enabled = true,
    Tags = new Dictionary<string, string> {
        ["permission"] = "player.ban",
        ["audit"] = "true"
    }
};

// 验证
if (!descriptor.IsValid()) {
    throw new ArgumentException("Invalid descriptor");
}

// 获取完整名称
Console.WriteLine(descriptor.GetFullName()); // "moderation.player.ban"
```

## ClientConfig

客户端配置类。

```csharp
public class ClientConfig
{
    // 连接配置
    public string AgentAddr { get; set; }              // Agent 地址
    public string ServiceId { get; set; }              // 服务标识符
    public string ServiceVersion { get; set; }         // 服务版本
    public string GameId { get; set; }                 // 游戏 ID
    public string Env { get; set; }                    // 环境
    public string LocalAddr { get; set; }              // 本地监听地址

    // TLS 配置
    public bool Insecure { get; set; }                 // 跳过 TLS 验证
    public string? CertFile { get; set; }             // 客户端证书路径
    public string? KeyFile { get; set; }              // 客户端私钥路径
    public string? CaFile { get; set; }               // CA 证书路径
    public string? ServerName { get; set; }           // SNI 服务器名称

    // 超时配置
    public int TimeoutSeconds { get; set; }            // 连接超时（秒）
    public int HeartbeatIntervalSeconds { get; set; }  // 心跳间隔（秒）

    // 重连配置
    public bool AutoReconnect { get; set; }           // 自动重连
    public int ReconnectIntervalSeconds { get; set; }  // 重连间隔（秒）
    public int ReconnectMaxAttempts { get; set; }     // 最大重试次数（0=无限）

    // 性能配置
    public int MaxConcurrentMessages { get; set; }    // 最大并发消息数
    public int MaxMessageSize { get; set; }           // 最大消息大小（字节）
}
```

**默认值：**
```csharp
new ClientConfig {
    AgentAddr = "127.0.0.1:19090",
    ServiceId = "csharp-service",
    ServiceVersion = "1.0.0",
    GameId = "default-game",
    Env = "dev",
    LocalAddr = "0.0.0.0:0",
    TimeoutSeconds = 30,
    HeartbeatIntervalSeconds = 30,
    AutoReconnect = true,
    ReconnectIntervalSeconds = 5,
    ReconnectMaxAttempts = 0,
    MaxConcurrentMessages = 100,
    MaxMessageSize = 4 * 1024 * 1024
}
```

## InvokeOptions

函数调用选项。

```csharp
public class InvokeOptions
{
    public string? GameId { get; set; }              // 覆盖默认 GameId
    public string? Env { get; set; }                 // 覆盖默认 Env
    public int TimeoutSeconds { get; set; }          // 调用超时（秒）
    public string? IdempotencyKey { get; set; }      // 幂等性键
    public string? RequestId { get; set; }           // 请求 ID（追踪）
    public string? UserId { get; set; }              // 用户 ID（审计）
    public Dictionary<string, string>? Metadata { get; set; } // 自定义元数据
}
```

## InvokeResult

函数调用结果。

```csharp
public class InvokeResult
{
    public bool Success { get; init; }               // 是否成功
    public string? Data { get; init; }               // 响应数据
    public string? Error { get; init; }              // 错误信息
    public string? ErrorCode { get; init; }          // 错误码
    public long DurationMs { get; init; }            // 执行时长（毫秒）

    // 静态工厂方法
    public static InvokeResult Succeeded(string data, long durationMs = 0)
    public static InvokeResult Failed(string error, string? errorCode = null, long durationMs = 0)
}
```

## FunctionContext

函数调用上下文。

```csharp
public class FunctionContext
{
    public required string FunctionId { get; init; }   // 函数 ID
    public required string CallId { get; init; }       // 调用 ID
    public required string GameId { get; init; }       // 游戏 ID
    public required string Env { get; init; }          // 环境
    public required long Timestamp { get; init; }      // 时间戳
    public string? UserId { get; init; }              // 用户 ID
    public string? IdempotencyKey { get; init; }      // 幂等性键
    public string? CallerServiceId { get; init; }     // 调用方服务 ID
}
```

## JobStatus

异步任务状态。

```csharp
public class JobStatus
{
    public required string JobId { get; init; }        // 任务 ID
    public required string Status { get; init; }       // 状态：pending/running/completed/failed/canceled
    public double Progress { get; init; }             // 进度 0.0-1.0
    public string? Error { get; init; }              // 错误信息（失败时）
    public string? Result { get; init; }             // 结果数据（完成时）
    public DateTime? StartTime { get; init; }         // 开始时间
    public DateTime? EndTime { get; init; }           // 结束时间
}
```

## BatchInvokeRequest

批量调用请求。

```csharp
public class BatchInvokeRequest
{
    public required string FunctionId { get; init; }  // 函数 ID
    public required string Payload { get; init; }     // 请求负载
    public string? IdempotencyKey { get; init; }      // 幂等性键
}
```
