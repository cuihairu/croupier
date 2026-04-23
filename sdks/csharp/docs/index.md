---
home: true
title: Croupier C# SDK
titleTemplate: false
heroImage: /logo.png
heroText: Croupier SDK for .NET 8+
tagline: 官方 C# SDK，用于连接 Croupier 游戏后端平台
actions:
  - text: 快速开始
    link: /guide/quick-start
    type: primary
  - text: GitHub 仓库
    link: https://github.com/cuihairu/croupier-sdk-csharp
    type: secondary

features:
  - title: 📡 gRPC 通信
    details: 基于 Grpc.Net.Client 的高效双向通信，支持流式调用和双向通信。
  - title: 🏢 多租户支持
    details: 内置 game_id/env 隔离机制，支持多游戏、多环境部署。
  - title: 📝 函数注册
    details: 简洁的描述符和处理器注册 API，支持异步/同步函数。
  - title: 🔄 异步/同步
    details: 完整支持 async/await 模式，同时也支持同步处理器。
  - title: 🛠️ 依赖注入
    details: 集成 Microsoft.Extensions.DependencyInjection，便于集成到现有项目。
  - title: 📊 日志抽象
    details: 支持 ILogger 和自定义日志实现，灵活的日志记录。
  - title: ⚙️ 灵活配置
    details: 环境变量、JSON 文件、内存配置等多种配置方式。
  - title: 🎮 Unity 支持
    details: 支持 Unity 2021.3+，可直接在游戏客户端中使用。

footer: MIT Licensed | Copyright © 2025 Croupier Project
---

## 安装

```bash
dotnet add package Croupier.Sdk
```

## 快速示例

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;

// 创建客户端
var client = new CroupierClient(new ClientConfig {
    AgentAddr = "127.0.0.1:19090",
    ServiceId = "my-service",
    GameId = "my-game"
});

// 注册函数
client.RegisterFunction(new FunctionDescriptor {
    Id = "player.get",
    Version = "1.0.0",
    Category = "player",
    Risk = "low"
}, async (context, payload) => {
    // 处理调用
    return "{\"status\":\"ok\"}";
});

// 连接并启动服务
await client.ConnectAsync();
await client.ServeAsync();
```

## 文档

- [指南](/guide/) - 详细的使用指南
- [API 参考](/api/) - 完整的 API 文档
