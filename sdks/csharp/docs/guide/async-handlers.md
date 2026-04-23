# 异步处理器

## 异步函数处理器

SDK 支持使用 `async/await` 模式的异步处理器：

```csharp
client.RegisterFunction(new FunctionDescriptor {
    Id = "player.update",
    Version = "1.0.0",
    Category = "player",
    Risk = "medium"
}, async (context, payload) =>
{
    // 模拟异步操作
    await Task.Delay(100);

    // 调用数据库
    var player = await _db.GetPlayerAsync(payload);

    return System.Text.Json.JsonSerializer.Serialize(player);
});
```

## 异步流处理

```csharp
client.RegisterFunction(new FunctionDescriptor {
    Id = "player.list",
    Version = "1.0.0",
    Category = "player",
    Risk = "low"
}, async (context, payload) =>
{
    var request = JsonSerializer.Deserialize<ListRequest>(payload);
    var players = new List<Player>();

    await foreach (var player in _db.GetPlayersAsync(request.Limit))
    {
        players.Add(player);
    }

    return JsonSerializer.Serialize(players);
});
```

## 取消令牌

```csharp
client.RegisterFunction(descriptor, async (context, payload) =>
{
    using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(30));

    var result = await _longRunningService
        .ExecuteAsync(payload, cts.Token);

    return JsonSerializer.Serialize(result);
});
```
