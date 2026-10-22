# 快速开始

## 环境要求

- .NET 10.0 SDK 或更高
- 一个可达的 Croupier Agent

## 最小示例

```csharp
using Croupier.Sdk;

var client = new CroupierClient(new ClientConfig {
    AgentAddr = "127.0.0.1:19091",
    ServiceId = "my-service",
    GameId = "my-game",
    Env = "development"
});
```
