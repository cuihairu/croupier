# 错误处理

## InvokeResult

所有函数调用返回 `InvokeResult`：

```csharp
public class InvokeResult
{
    public bool Success { get; init; }
    public string? Data { get; init; }
    public string? Error { get; init; }
    public string? ErrorCode { get; init; }
    public long DurationMs { get; init; }
}
```

## 处理调用错误

```csharp
var result = await invoker.InvokeAsync("player.get", payload);

if (result.Success)
{
    Console.WriteLine($"Success: {result.Data}");
}
else
{
    Console.WriteLine($"Error: {result.Error}");
    Console.WriteLine($"Code: {result.ErrorCode}");
    Console.WriteLine($"Duration: {result.DurationMs}ms");
}
```

## 函数处理器异常

函数处理器中的异常会被自动捕获：

```csharp
client.RegisterFunction(descriptor, async (context, payload) =>
{
    // 抛出异常
    if (string.IsNullOrEmpty(payload))
    {
        throw new ArgumentException("Payload is required");
    }

    // 返回结果
    return "{\"status\":\"ok\"}";
});
```

调用方会收到错误响应：

```json
{
  "error": "Payload is required",
  "error_detail": "System.ArgumentException"
}
```

## 连接错误处理

```csharp
try
{
    await client.ConnectAsync();
}
catch (Exception ex) when (ex is TimeoutException or ConnectionException)
{
    _logger.LogError(ex, "Failed to connect to Agent");
    // 重试逻辑
    await Task.Delay(5000);
    await client.ConnectAsync();
}
```

## 重试策略

```csharp
public class RetryInvoker
{
    private readonly CroupierInvoker _invoker;

    public async Task<InvokeResult> InvokeWithRetryAsync(
        string functionId,
        string payload,
        int maxRetries = 3)
    {
        for (int i = 0; i < maxRetries; i++)
        {
            var result = await _invoker.InvokeAsync(functionId, payload);

            if (result.Success || result.ErrorCode != "UNAVAILABLE")
            {
                return result;
            }

            // 指数退避
            await Task.Delay(TimeSpan.FromSeconds(Math.Pow(2, i)));
        }

        return InvokeResult.Failed("Max retries exceeded");
    }
}
```
