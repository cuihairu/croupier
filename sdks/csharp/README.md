<p align="center">
  <h1 align="center">Croupier C# SDK</h1>
  <p align="center">
    <strong>Croupier 游戏函数注册与执行系统的官方 .NET SDK</strong>
  </p>
</p>

<p align="center">
  <a href="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml">
    <img src="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg" alt="CI">
  </a>
  <a href="https://codecov.io/gh/cuihairu/croupier">
    <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=csharp-sdk" alt="Coverage">
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://dotnet.microsoft.com/download/dotnet/9.0">
    <img src="https://img.shields.io/badge/.NET-9.0%2B-purple.svg" alt=".NET Version">
  </a>
</p>

<p align="center">
  <a href="#支持平台">
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg" alt="Platform">
  </a>
  <a href="https://github.com/cuihairu/croupier">
    <img src="https://img.shields.io/badge/Main%20Project-Croupier-green.svg" alt="Main Project">
  </a>
</p>

---

## 目录

- [简介](#简介)
- [主项目](#主项目)
- [其他语言 SDK](#其他语言-sdk)
- [支持平台](#支持平台)
- [核心特性](#核心特性)
- [快速开始](#快速开始)
- [使用示例](#使用示例)
- [配置](#配置)
- [许可证](#许可证)

---

## 简介

Croupier C# SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 .NET 客户端实现。支持 .NET 8+，提供简洁的异步 API 用于服务器端服务连接 Agent、注册函数和调用远程函数。

## 正式文档

- 统一文档站入口：`/docs/sdks/csharp/`
- 仓库内路径：`docs/sdks/csharp`

## 主项目

| 项目 | 描述 | 链接 |
|------|------|------|
| **Croupier** | 游戏后端平台主项目（包含 Proto 定义） | [cuihairu/croupier](https://github.com/cuihairu/croupier) |
| **Proto 文件** | 协议定义（Protobuf） | [proto/](https://github.com/cuihairu/croupier/tree/main/proto) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言 | 目录 | CI | Docs |
| --- | --- | --- | --- |
| Go | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml) | [README](sdks/go/README.md) |
| C++ | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml) | [README](sdks/cpp/README.md) |
| Java | [sdks/java/](https://github.com/cuihairu/croupier/tree/main/sdks/java) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml) | [README](sdks/java/README.md) |
| JS/TS | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml) | [README](sdks/js/README.md) |
| Python | [sdks/python/](https://github.com/cuihairu/croupier/tree/main/sdks/python) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](sdks/python/README.md) |

## 支持平台

| 平台 | 架构 | 状态 |
|------|------|------|
| **Windows** | x64 | ✅ 支持 |
| **Linux** | x64, ARM64 | ✅ 支持 |
| **macOS** | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

- **gRPC 通信** - 基于 Grpc.Net.Client 的高效双向通信
- **多租户支持** - 内置 game_id/env 隔离机制
- **函数注册** - 使用描述符和处理器注册游戏函数
- **异步/同步** - 支持 async/await 和同步处理器
- **依赖注入** - 集成 Microsoft.Extensions.DependencyInjection
- **日志抽象** - 支持 ILogger 和自定义日志实现
- **灵活配置** - 环境变量、JSON 文件、内存配置支持

## 快速开始

### 系统要求

- **.NET 9.0 SDK** 或更高版本

### 安装

从 NuGet 安装：

```bash
dotnet add package Croupier.Sdk
```

或从源码构建：

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier/sdks/csharp
dotnet build
```

## 使用示例

### 创建客户端并连接

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;

var config = new ClientConfig
{
    AgentAddr = "127.0.0.1:19090",
    ServiceId = "my-service",
    GameId = "my-game",
    Env = "production"
};

var client = new CroupierClient(config);
await client.ConnectAsync();
```

### 注册函数

```csharp
var descriptor = new FunctionDescriptor
{
    Id = "player.get",
    Version = "1.0.0",
    Category = "player",
    Risk = "low",
    DisplayName = "获取玩家信息",
    Description = "根据玩家 ID 获取玩家详细信息"
};

client.RegisterFunction(descriptor, async (context, payload) =>
{
    // 处理函数调用
    var response = new
    {
        status = "success",
        player = new { id = "123", name = "TestPlayer" }
    };

    return System.Text.Json.JsonSerializer.Serialize(response);
});
```

### 调用远程函数

```csharp
var invoker = new CroupierInvoker("127.0.0.1:19090", "my-game", "production");

var result = await invoker.InvokeAsync("player.ban", "{\"player_id\":\"123\"}");

if (result.Success)
{
    Console.WriteLine($"Result: {result.Data}");
}
else
{
    Console.WriteLine($"Error: {result.Error}");
}
```

### 启动服务

```csharp
await client.ServeAsync();
```

### 依赖注入

```csharp
using Croupier.Sdk.Extensions;
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();

services.AddCroupier(options =>
{
    options.AgentAddr = "127.0.0.1:19090";
    options.GameId = "my-game";
    options.Env = "production";
});

var serviceProvider = services.BuildServiceProvider();
var client = serviceProvider.GetRequiredService<CroupierClient>();
```

## 配置

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `CROUPIER_AGENT_ADDR` | Agent 地址 | 127.0.0.1:19090 |
| `CROUPIER_SERVICE_ID` | 服务 ID | csharp-service |
| `CROUPIER_GAME_ID` | 游戏 ID | default-game |
| `CROUPIER_ENV` | 环境 | dev |
| `CROUPIER_INSECURE` | 跳过 TLS 验证 | false |
| `CROUPIER_CERT_FILE` | 客户端证书路径 | - |
| `CROUPIER_KEY_FILE` | 客户端私钥路径 | - |
| `CROUPIER_CA_FILE` | CA 证书路径 | - |
| `CROUPIER_TIMEOUT_SECONDS` | 连接超时（秒） | 30 |
| `CROUPIER_AUTO_RECONNECT` | 自动重连 | true |

### 从环境变量加载配置

```csharp
using Croupier.Sdk.Configuration;

var configProvider = new EnvironmentConfigProvider();
var config = configProvider.GetConfig();
var client = new CroupierClient(config);
```

## 许可证

Apache License 2.0 - 详见 [LICENSE](LICENSE) 文件
