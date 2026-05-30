# 快速开始

## 系统要求

- .NET 8.0 SDK 或更高版本
- 可访问的 Croupier Agent

## 最小示例

```csharp
using Croupier.Sdk;

var client = new CroupierClient(new ClientConfig {
    AgentAddr = "127.0.0.1:19090",
    ServiceId = "my-service",
    GameId = "my-game",
    Env = "dev"
});
```
