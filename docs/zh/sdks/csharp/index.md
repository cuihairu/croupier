---
title: C# SDK
---

# C# SDK

连接 Croupier Agent、注册函数并在服务端处理调用的官方 .NET SDK。

## 代码位置

- `sdks/csharp`

## 特性

- 支持 .NET 10+
- Async API 优先
- 支持依赖注入、配置系统与日志抽象

## 安装

```bash
dotnet add package Croupier.Sdk
```

## 快速开始

```csharp
using Croupier.Sdk;

var client = new CroupierClient(new ClientConfig {
    AgentAddr = "127.0.0.1:19091",
    ServiceId = "my-service",
    GameId = "my-game"
});
```

## 继续阅读

- [指南](/zh/sdks/csharp/guide/quick-start)
- [API 参考](/zh/sdks/csharp/api/)
- [仓库 README](https://github.com/cuihairu/croupier/tree/main/sdks/csharp)
