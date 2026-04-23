# 依赖注入

Croupier C# SDK 内置对 Microsoft.Extensions.DependencyInjection 的支持。

## ASP.NET Core 集成

### 基本使用

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Extensions;
using Croupier.Sdk.Models;

var builder = WebApplication.CreateBuilder(args);

// 添加 Croupier SDK
builder.Services.AddCroupier(options =>
{
    options.AgentAddr = "127.0.0.1:19090";
    options.GameId = "my-game";
    options.Env = "production";
});

var app = builder.Build();

// 从 DI 获取客户端
using (var scope = app.Services.CreateScope())
{
    var client = scope.ServiceProvider.GetRequiredService<CroupierClient>();
    await client.ConnectAsync();
}
```

### 使用 IOptions\<T\>

```csharp
// 配置
builder.Services.Configure<ClientConfig>(
    builder.Configuration.GetSection("Croupier"));

// 添加 SDK
builder.Services.AddCroupier();
```

### 注册函数

```csharp
// 创建函数处理器类
public class PlayerHandlers
{
    private readonly ILogger<PlayerHandlers> _logger;

    public PlayerHandlers(ILogger<PlayerHandlers> logger)
    {
        _logger = logger;
    }

    public async Task<string> GetPlayer(FunctionContext context, string payload)
    {
        _logger.LogInformation("GetPlayer called: {CallId}", context.CallId);
        // 处理逻辑...
        return "{ \"id\": \"123\", \"name\": \"Player\" }";
    }
}

// 在 Startup 或 Program.cs 中注册
builder.Services.AddCroupier(options =>
{
    options.AgentAddr = "127.0.0.1:19090";
});

// 获取客户端并注册函数
var client = app.Services.GetRequiredService<CroupierClient>();
var handlers = app.Services.GetRequiredService<PlayerHandlers>();

client.RegisterFunction(new FunctionDescriptor
{
    Id = "player.get",
    Version = "1.0.0",
    Category = "player",
    Risk = "low"
}, handlers.GetPlayer);
```

### 后台服务

```csharp
public class CroupierHostedService : BackgroundService
{
    private readonly CroupierClient _client;
    private readonly ILogger<CroupierHostedService> _logger;

    public CroupierHostedService(
        CroupierClient client,
        ILogger<CroupierHostedService> logger)
    {
        _client = client;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        await _client.ConnectAsync(stoppingToken);
        _logger.LogInformation("Croupier client connected");

        await _client.ServeAsync(stoppingToken);
    }

    public override async Task StopAsync(CancellationToken cancellationToken)
    {
        _client.Stop();
        await base.StopAsync(cancellationToken);
    }
}

// 注册后台服务
builder.Services.AddHostedService<CroupierHostedService>();
```
