# 快速开始

## 系统要求

- **.NET 8.0 SDK** 或更高版本
- **Croupier Agent** 运行中

## 安装

从 NuGet 安装：

```bash
dotnet add package Croupier.Sdk
```

或从源码构建：

```bash
git clone https://github.com/cuihairu/croupier-sdk-csharp.git
cd croupier-sdk-csharp
dotnet build
```

## 创建第一个应用

### 1. 创建控制台应用

```bash
dotnet new console -n MyCroupierService
cd MyCroupierService
dotnet add package Croupier.Sdk
```

### 2. 编写代码

编辑 `Program.cs`：

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;

var config = new ClientConfig
{
    AgentAddr = "127.0.0.1:19090",
    ServiceId = "my-service",
    GameId = "my-game",
    Env = "dev"
};

var client = new CroupierClient(config);

// 注册函数
client.RegisterFunction(new FunctionDescriptor {
    Id = "hello.world",
    Version = "1.0.0",
    Category = "example",
    Risk = "low"
}, async (context, payload) =>
{
    return System.Text.Json.JsonSerializer.Serialize(new {
        message = "Hello from C# SDK!",
        timestamp = DateTime.UtcNow
    });
});

// 连接并启动
await client.ConnectAsync();
Console.WriteLine("Service started. Press Ctrl+C to exit.");

await client.ServeAsync();
```

### 3. 运行

```bash
dotnet run
```

## 下一步

- [配置选项](./configuration.md) - 了解更多配置选项
- [依赖注入](./dependency-injection.md) - 在 ASP.NET Core 中使用
- [API 参考](../api/) - 完整的 API 文档
