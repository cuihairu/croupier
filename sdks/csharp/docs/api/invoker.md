# CroupierInvoker API

## 构造函数

### CroupierInvoker(string agentAddr, string? gameId = null, string? env = null, bool insecure = false, ICroupierLogger? logger = null)

创建一个新的 Croupier 调用器实例。

**参数：**
- `agentAddr` - Agent 服务器地址
- `gameId` - 游戏 ID
- `env` - 环境
- `insecure` - 是否跳过 TLS 验证
- `logger` - 日志记录器

**示例：**
```csharp
// 最简单的用法
var invoker = new CroupierInvoker();

// 完整配置
var invoker = new CroupierInvoker(
    agentAddr: "192.168.1.100:19090",
    gameId: "my-game",
    env: "production",
    insecure: false
);
```

## 属性

| 属性 | 类型 | 描述 |
|------|------|------|
| `AgentAddr` | `string` | Agent 服务器地址 |
| `GameId` | `string` | 游戏 ID |
| `Env` | `string` | 环境 |

## 方法

### InvokeAsync

调用远程函数。

```csharp
public Task<InvokeResult> InvokeAsync(
    string functionId,
    string payload,
    InvokeOptions? options = null,
    CancellationToken cancellationToken = default
)
```

**参数：**
- `functionId` - 函数 ID（格式：category.entity.operation）
- `payload` - 请求负载（JSON 字符串）
- `options` - 调用选项
- `cancellationToken` - 取消令牌

**返回值：** `Task<InvokeResult>`

**示例：**
```csharp
var result = await invoker.InvokeAsync(
    "player.get",
    "{\"player_id\":\"123\"}"
);

if (result.Success) {
    Console.WriteLine(result.Data);
} else {
    Console.WriteLine($"Error: {result.Error}");
}
```

### BatchInvokeAsync

批量调用多个函数。

```csharp
public Task<List<InvokeResult>> BatchInvokeAsync(
    List<BatchInvokeRequest> requests,
    InvokeOptions? options = null,
    CancellationToken cancellationToken = default
)
```

**示例：**
```csharp
var requests = new List<BatchInvokeRequest>
{
    new() { FunctionId = "player.get", Payload = "{\"player_id\":\"1\"}" },
    new() { FunctionId = "player.get", Payload = "{\"player_id\":\"2\"}" },
    new() { FunctionId = "player.get", Payload = "{\"player_id\":\"3\"}" }
};

var results = await invoker.BatchInvokeAsync(requests);

foreach (var (result, index) in results.Select((r, i) => (r, i)))
{
    Console.WriteLine($"Request {index + 1}: {(result.Success ? "OK" : "FAILED")}");
}
```

### StartJobAsync

启动异步任务（不需要等待响应的调用）。

```csharp
public Task<string> StartJobAsync(
    string functionId,
    string payload,
    InvokeOptions? options = null
)
```

**返回值：** 任务 ID

**示例：**
```csharp
var jobId = await invoker.StartJobAsync(
    "player.export_data",
    "{\"player_id\":\"123\"}"
);

Console.WriteLine($"Job started: {jobId}");
```

### CancelJobAsync

取消正在运行的任务。

```csharp
public Task<bool> CancelJobAsync(
    string jobId,
    CancellationToken cancellationToken = default
)
```

**示例：**
```csharp
var cancelled = await invoker.CancelJobAsync(jobId);
Console.WriteLine($"Job cancelled: {cancelled}");
```

### GetJobStatusAsync

获取任务状态。

```csharp
public Task<JobStatus?> GetJobStatusAsync(
    string jobId,
    CancellationToken cancellationToken = default
)
```

**返回值：** `JobStatus?`

**示例：**
```csharp
var status = await invoker.GetJobStatusAsync(jobId);
if (status != null)
{
    Console.WriteLine($"Status: {status.Status}, Progress: {status.Progress}");
}
```

### Dispose

释放资源。

```csharp
public void Dispose()
```
