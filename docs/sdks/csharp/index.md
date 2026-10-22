---
title: C# SDK
---

# C# SDK

The official .NET SDK for connecting to the Croupier Agent, registering functions, and handling invocations server-side.

## Code Location

- `sdks/csharp`

## Features

- .NET 10+ support
- Async-API first
- Dependency injection, configuration system, and logging abstractions supported

## Installation

```bash
dotnet add package Croupier.Sdk
```

## Quick Start

```csharp
using Croupier.Sdk;

var client = new CroupierClient(new ClientConfig {
    AgentAddr = "127.0.0.1:19091",
    ServiceId = "my-service",
    GameId = "my-game"
});
```

## Keep Reading

- [Guide](/sdks/csharp/guide/)
- [API Reference](/sdks/csharp/api/)
- [Repository README](https://github.com/cuihairu/croupier/tree/main/sdks/csharp)
