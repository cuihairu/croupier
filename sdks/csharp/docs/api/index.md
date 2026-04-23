# API 参考

本文档提供 Croupier C# SDK 的完整 API 参考。

## 包结构

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;
using Croupier.Sdk.Logging;
using Microsoft.Extensions.DependencyInjection;
```

## 核心类型

### IFunctionHandler

函数处理器接口。

```csharp
public interface IFunctionHandler
{
    /// <summary>
    /// 异步处理函数调用
    /// </summary>
    /// <param name="context">函数上下文</param>
    /// <param name="payload">请求负载（JSON 字符串）</param>
    /// <returns>响应负载（JSON 字符串）</returns>
    Task<string> HandleAsync(FunctionContext context, string payload);
}
```

**实现示例:**

```csharp
public class BanPlayerHandler : IFunctionHandler
{
    public async Task<string> HandleAsync(FunctionContext context, string payload)
    {
        var request = JsonSerializer.Deserialize<BanRequest>(payload);

        // 业务逻辑
        await BanPlayerInDatabaseAsync(request.PlayerId, request.Reason);

        return JsonSerializer.Serialize(new { status = "success" });
    }
}
```

---

### FunctionHandlerDelegate

异步处理器委托。

```csharp
public delegate Task<string> FunctionHandlerDelegate(
    FunctionContext context,
    string payload);
```

**Lambda 使用示例:**

```csharp
client.RegisterFunction(descriptor, async (context, payload) =>
{
    var data = JsonSerializer.Deserialize<JsonElement>(payload);
    // 业务逻辑
    return "{\"status\":\"success\"}";
});
```

---

### SyncFunctionHandlerDelegate

同步处理器委托。

```csharp
public delegate string SyncFunctionHandlerDelegate(
    FunctionContext context,
    string payload);
```

**使用示例:**

```csharp
client.RegisterFunction(descriptor, (context, payload) =>
{
    // 同步业务逻辑
    return "{\"status\":\"success\"}";
});
```

---

### FunctionContext

函数调用上下文。

```csharp
public class FunctionContext
{
    public string FunctionId { get; init; }      // 函数 ID
    public string CallId { get; init; }          // 调用 ID
    public string GameId { get; init; }          // 游戏 ID
    public string Env { get; init; }             // 环境
    public string? UserId { get; init; }         // 用户 ID
    public long Timestamp { get; init; }         // 时间戳
    public string? IdempotencyKey { get; init; } // 幂等键
    public string? CallerServiceId { get; init; } // 调用方服务 ID
}
```

---

### ClientConfig

客户端配置类。

```csharp
public class ClientConfig
{
    // 连接配置
    public string AgentAddr { get; set; } = "127.0.0.1:19090";  // Agent 地址
    public string LocalAddr { get; set; } = "0.0.0.0:0";        // 本地监听地址
    public int TimeoutSeconds { get; set; } = 30;               // 连接超时（秒）
    public bool Insecure { get; set; }                          // 是否跳过 TLS

    // 多租户隔离
    public string GameId { get; set; } = "default-game";        // 游戏 ID
    public string Env { get; set; } = "dev";                    // 环境
    public string ServiceId { get; set; } = "csharp-service";   // 服务 ID
    public string ServiceVersion { get; set; } = "1.0.0";       // 服务版本

    // TLS 配置
    public string? CertFile { get; set; }                       // 证书文件路径
    public string? KeyFile { get; set; }                        // 私钥文件路径
    public string? CaFile { get; set; }                         // CA 证书路径
    public string? ServerName { get; set; }                     // TLS 服务器名称

    // 重连配置
    public bool AutoReconnect { get; set; } = true;             // 自动重连
    public int ReconnectIntervalSeconds { get; set; } = 5;      // 重连间隔（秒）
    public int ReconnectMaxAttempts { get; set; } = 0;          // 最大重连次数

    // 心跳配置
    public int HeartbeatIntervalSeconds { get; set; } = 30;     // 心跳间隔（秒）

    // 消息配置
    public int MaxConcurrentMessages { get; set; } = 100;       // 最大并发消息数
    public int MaxMessageSize { get; set; } = 4 * 1024 * 1024;  // 消息最大大小（4MB）
}
```

**使用示例:**

```csharp
var config = new ClientConfig
{
    AgentAddr = "localhost:19090",
    GameId = "my-game",
    Env = "production",
    ServiceId = "player-service",
    Insecure = false,
    CertFile = "/etc/tls/client.crt",
    KeyFile = "/etc/tls/client.key",
    CaFile = "/etc/tls/ca.crt",
    AutoReconnect = true,
};
```

---

### FunctionDescriptor

函数描述符。

```csharp
public class FunctionDescriptor
{
    // 必填字段
    public string Id { get; set; }                   // 函数 ID
    public string Version { get; set; } = "1.0.0";   // 版本号

    // 推荐字段
    public string Category { get; set; }             // 业务分类
    public string Risk { get; set; } = "medium";     // 风险等级
    public string? Entity { get; set; }              // 关联实体
    public string? Operation { get; set; }           // 操作类型
    public bool Enabled { get; set; } = true;        // 是否启用

    // 可选字段
    public string? DisplayName { get; set; }         // 显示名称
    public string? Description { get; set; }         // 描述
    public string? InputSchema { get; set; }         // 输入 JSON Schema
    public string? OutputSchema { get; set; }        // 输出 JSON Schema
    public Dictionary<string, string>? Tags { get; set; } // 标签

    // 方法
    public bool IsValid();                           // 验证描述符
    public string GetFullName();                     // 获取完整名称
}
```

**使用示例:**

```csharp
var descriptor = new FunctionDescriptor
{
    Id = "player.ban",
    Version = "1.0.0",
    Category = "player",
    Risk = "high",
    Entity = "player",
    Operation = "update",
    Enabled = true,
    DisplayName = "封禁玩家",
    Description = "封禁指定玩家账号",
    InputSchema = "{\"type\":\"object\",\"properties\":{\"player_id\":{\"type\":\"string\"}}}"
};
```

---

## CroupierClient 类

主客户端类，管理与 Croupier Agent 的连接和函数注册。

### 构造函数

```csharp
// 使用自定义配置和日志
public CroupierClient(ClientConfig? config = null, ICroupierLogger? logger = null)

// 使用 Microsoft.Extensions.Logging
public CroupierClient(ClientConfig config, ILogger logger)
```

### 属性

```csharp
public ClientConfig Config { get; }        // 客户端配置
public bool IsConnected { get; }           // 是否已连接
public string? LocalAddress { get; }       // 本地监听地址
```

### 公共方法

#### ConnectAsync

连接到 Agent。

```csharp
public Task ConnectAsync(CancellationToken cancellationToken = default)
```

**异常:** `InvalidOperationException` - 连接失败时抛出

---

#### Disconnect

断开连接。

```csharp
public void Disconnect()
```

---

#### RegisterFunction

注册函数处理器（三种重载）。

```csharp
// 使用接口
public void RegisterFunction(FunctionDescriptor descriptor, IFunctionHandler handler)

// 使用异步委托
public void RegisterFunction(FunctionDescriptor descriptor, FunctionHandlerDelegate handler)

// 使用同步委托
public void RegisterFunction(FunctionDescriptor descriptor, SyncFunctionHandlerDelegate handler)
```

**参数:**
- `descriptor`: 函数描述符
- `handler`: 函数处理器

**异常:** `ArgumentException` - 描述符无效时抛出

---

#### UnregisterFunction

取消注册函数。

```csharp
public bool UnregisterFunction(string functionId)
```

**返回值:** 是否成功取消注册

---

#### ServeAsync

开始服务（阻塞调用）。

```csharp
public Task ServeAsync(CancellationToken cancellationToken = default)
```

---

#### InvokeAsync

调用远程函数。

```csharp
public Task<string> InvokeAsync(
    string functionId,
    string payload,
    InvokeOptions? options = null,
    CancellationToken cancellationToken = default)
```

---

#### Stop

停止服务。

```csharp
public void Stop()
```

---

#### Dispose

释放资源（实现 IDisposable）。

```csharp
public void Dispose()
```

---

## CroupierInvoker 类

调用端类，用于调用已注册的函数。

### 构造函数

```csharp
// 直接指定参数
public CroupierInvoker(
    string agentAddr = "127.0.0.1:19090",
    string? gameId = null,
    string? env = null,
    bool insecure = false,
    ICroupierLogger? logger = null)

// 从配置创建
public CroupierInvoker(ClientConfig config, ICroupierLogger? logger = null)

// 使用 Microsoft.Extensions.Logging
public CroupierInvoker(
    string agentAddr,
    string gameId,
    string env,
    ILogger logger)
```

### 属性

```csharp
public string AgentAddr { get; }  // Agent 地址
public string GameId { get; }     // 游戏 ID
public string Env { get; }        // 环境
```

### 公共方法

#### InvokeAsync

同步调用函数。

```csharp
public Task<InvokeResult> InvokeAsync(
    string functionId,
    string payload,
    InvokeOptions? options = null,
    CancellationToken cancellationToken = default)
```

**参数:**
- `functionId`: 函数 ID
- `payload`: 请求负载（JSON 字符串）
- `options`: 调用选项
- `cancellationToken`: 取消令牌

**返回值:** `InvokeResult` - 调用结果

---

#### BatchInvokeAsync

批量调用多个函数。

```csharp
public Task<List<InvokeResult>> BatchInvokeAsync(
    List<BatchInvokeRequest> requests,
    InvokeOptions? options = null,
    CancellationToken cancellationToken = default)
```

---

#### StartJobAsync

启动异步任务。

```csharp
public Task<string> StartJobAsync(
    string functionId,
    string payload,
    InvokeOptions? options = null)
```

**返回值:** 任务 ID

---

#### CancelJobAsync

取消任务。

```csharp
public Task<bool> CancelJobAsync(
    string jobId,
    CancellationToken cancellationToken = default)
```

---

#### GetJobStatusAsync

获取任务状态。

```csharp
public Task<JobStatus?> GetJobStatusAsync(
    string jobId,
    CancellationToken cancellationToken = default)
```

---

### InvokeOptions

调用选项。

```csharp
public class InvokeOptions
{
    public string? GameId { get; set; }                   // 游戏 ID
    public string? Env { get; set; }                      // 环境
    public int TimeoutSeconds { get; set; } = 30;         // 超时（秒）
    public string? IdempotencyKey { get; set; }           // 幂等键
    public string? RequestId { get; set; }                // 请求 ID
    public string? UserId { get; set; }                   // 用户 ID
    public Dictionary<string, string>? Metadata { get; set; } // 元数据
}
```

---

### InvokeResult

调用结果。

```csharp
public class InvokeResult
{
    public bool Success { get; init; }         // 是否成功
    public string? Data { get; init; }         // 响应数据
    public string? Error { get; init; }        // 错误信息
    public string? ErrorCode { get; init; }    // 错误码
    public long DurationMs { get; init; }      // 耗时（毫秒）

    // 静态工厂方法
    public static InvokeResult Succeeded(string data, long durationMs = 0)
    public static InvokeResult Failed(string error, string? errorCode = null, long durationMs = 0)
}
```

---

### BatchInvokeRequest

批量调用请求。

```csharp
public class BatchInvokeRequest
{
    public required string FunctionId { get; init; }     // 函数 ID
    public required string Payload { get; init; }        // 请求负载
    public string? IdempotencyKey { get; init; }         // 幂等键
}
```

---

### JobStatus

任务状态。

```csharp
public class JobStatus
{
    public required string JobId { get; init; }    // 任务 ID
    public required string Status { get; init; }   // 状态: pending|running|completed|failed|canceled
    public double Progress { get; init; }          // 进度 (0.0-1.0)
    public string? Error { get; init; }            // 错误信息
    public string? Result { get; init; }           // 结果数据
    public DateTime? StartTime { get; init; }      // 开始时间
    public DateTime? EndTime { get; init; }        // 结束时间
}
```

---

## 日志接口

### ICroupierLogger

```csharp
public interface ICroupierLogger
{
    void LogDebug(string category, string message);
    void LogInfo(string category, string message);
    void LogWarning(string category, string message);
    void LogError(string category, string message, Exception? exception = null);
}
```

**内置实现:**
- `ConsoleCroupierLogger`: 控制台输出
- `CroupierLogger`: 包装 `Microsoft.Extensions.Logging.ILogger`

---

## 依赖注入扩展

### ServiceCollectionExtensions

```csharp
public static class ServiceCollectionExtensions
{
    // 添加 CroupierClient 服务
    public static IServiceCollection AddCroupierClient(
        this IServiceCollection services,
        Action<ClientConfig> configure);

    // 添加 CroupierInvoker 服务
    public static IServiceCollection AddCroupierInvoker(
        this IServiceCollection services,
        Action<ClientConfig> configure);
}
```

**使用示例:**

```csharp
services.AddCroupierClient(config =>
{
    config.AgentAddr = "localhost:19090";
    config.GameId = "my-game";
    config.Insecure = true;
});

services.AddCroupierInvoker(config =>
{
    config.AgentAddr = "localhost:19090";
    config.GameId = "my-game";
});
```

---

## 完整示例

### Provider 示例

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;
using System.Text.Json;

class Program
{
    static async Task Main(string[] args)
    {
        // 配置
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            GameId = "my-game",
            Env = "production",
            ServiceId = "player-service",
            Insecure = true,
            AutoReconnect = true,
        };

        using var client = new CroupierClient(config);

        // 注册函数
        var descriptor = new FunctionDescriptor
        {
            Id = "player.ban",
            Version = "1.0.0",
            Category = "player",
            Risk = "high",
            Entity = "player",
            Operation = "update",
        };

        client.RegisterFunction(descriptor, async (context, payload) =>
        {
            var request = JsonSerializer.Deserialize<BanRequest>(payload);
            Console.WriteLine($"封禁玩家: {request?.PlayerId}, 原因: {request?.Reason}");

            // 业务逻辑
            await Task.Delay(100);

            return JsonSerializer.Serialize(new { status = "success" });
        });

        // 连接并启动
        using var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            cts.Cancel();
        };

        try
        {
            await client.ConnectAsync();
            Console.WriteLine("服务已启动");
            await client.ServeAsync(cts.Token);
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine("服务已停止");
        }
    }

    record BanRequest(string PlayerId, string Reason);
}
```

### Invoker 示例

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;
using System.Text.Json;

class Program
{
    static async Task Main(string[] args)
    {
        using var invoker = new CroupierInvoker(
            agentAddr: "localhost:19090",
            gameId: "my-game",
            env: "production",
            insecure: true
        );

        // 同步调用
        var options = new InvokeOptions
        {
            TimeoutSeconds = 10,
            IdempotencyKey = "ban-12345-20260117",
            Metadata = new Dictionary<string, string>
            {
                ["operator"] = "admin"
            }
        };

        var result = await invoker.InvokeAsync(
            "player.ban",
            JsonSerializer.Serialize(new { player_id = "12345", reason = "违规" }),
            options
        );

        if (result.Success)
        {
            Console.WriteLine($"调用成功: {result.Data}");
        }
        else
        {
            Console.WriteLine($"调用失败: {result.Error}");
        }

        // 批量调用
        var batchRequests = new List<BatchInvokeRequest>
        {
            new() { FunctionId = "player.get", Payload = "{\"player_id\":\"111\"}" },
            new() { FunctionId = "player.get", Payload = "{\"player_id\":\"222\"}" },
            new() { FunctionId = "player.get", Payload = "{\"player_id\":\"333\"}" },
        };

        var batchResults = await invoker.BatchInvokeAsync(batchRequests);
        foreach (var r in batchResults)
        {
            Console.WriteLine($"结果: {(r.Success ? r.Data : r.Error)}");
        }

        // 异步任务
        var jobId = await invoker.StartJobAsync(
            "player.export",
            JsonSerializer.Serialize(new { format = "csv" })
        );
        Console.WriteLine($"任务已启动: {jobId}");

        // 查询任务状态
        var status = await invoker.GetJobStatusAsync(jobId);
        Console.WriteLine($"任务状态: {status?.Status}, 进度: {status?.Progress:P0}");
    }
}
```

### ASP.NET Core 集成示例

```csharp
// Program.cs
using Croupier.Sdk;
using Croupier.Sdk.Models;

var builder = WebApplication.CreateBuilder(args);

// 添加 Croupier 服务
builder.Services.AddCroupierClient(config =>
{
    config.AgentAddr = builder.Configuration["Croupier:AgentAddr"] ?? "localhost:19090";
    config.GameId = builder.Configuration["Croupier:GameId"] ?? "my-game";
    config.Env = builder.Environmen.IsProduction() ? "prod" : "dev";
    config.Insecure = !builder.Environment.IsProduction();
});

builder.Services.AddCroupierInvoker(config =>
{
    config.AgentAddr = builder.Configuration["Croupier:AgentAddr"] ?? "localhost:19090";
    config.GameId = builder.Configuration["Croupier:GameId"] ?? "my-game";
});

var app = builder.Build();

// 启动 Croupier 客户端
var croupierClient = app.Services.GetRequiredService<CroupierClient>();
croupierClient.RegisterFunction(
    new FunctionDescriptor { Id = "player.get", Version = "1.0.0" },
    async (ctx, payload) => "{\"status\":\"ok\"}"
);

await croupierClient.ConnectAsync();
_ = croupierClient.ServeAsync();

app.Run();
```

### Unity 集成示例

```csharp
using UnityEngine;
using Croupier.Sdk;
using Croupier.Sdk.Models;

public class CroupierManager : MonoBehaviour
{
    private CroupierClient _client;
    private CroupierInvoker _invoker;

    async void Start()
    {
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            GameId = "unity-game",
            Insecure = true,
        };

        _client = new CroupierClient(config);
        _invoker = new CroupierInvoker(config);

        // 注册函数
        _client.RegisterFunction(
            new FunctionDescriptor { Id = "game.status", Version = "1.0.0" },
            (ctx, payload) => JsonUtility.ToJson(new { online = true })
        );

        await _client.ConnectAsync();
    }

    async void OnDestroy()
    {
        _client?.Dispose();
        _invoker?.Dispose();
    }

    public async Task<string> CallRemoteFunction(string functionId, string payload)
    {
        var result = await _invoker.InvokeAsync(functionId, payload);
        return result.Success ? result.Data : throw new Exception(result.Error);
    }
}
```

---

## 错误处理

### 异常类型

| 异常 | 场景 |
|------|------|
| `ArgumentException` | 参数无效 |
| `InvalidOperationException` | 操作无效（如未连接时调用） |
| `ObjectDisposedException` | 对象已释放 |
| `OperationCanceledException` | 操作被取消 |

### 错误处理示例

```csharp
using var client = new CroupierClient(config);

try
{
    await client.ConnectAsync();
    await client.ServeAsync();
}
catch (InvalidOperationException ex)
{
    Console.WriteLine($"连接失败: {ex.Message}");
}
catch (OperationCanceledException)
{
    Console.WriteLine("操作被取消");
}
finally
{
    client.Dispose();
}
```
