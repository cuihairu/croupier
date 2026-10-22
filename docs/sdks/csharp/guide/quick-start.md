# Quick Start

## Requirements

- .NET 10.0 SDK or later
- A reachable Croupier Agent

## Minimal Example

```csharp
using Croupier.Sdk;

var client = new CroupierClient(new ClientConfig {
    AgentAddr = "127.0.0.1:19091",
    ServiceId = "my-service",
    GameId = "my-game",
    Env = "development"
});
```
