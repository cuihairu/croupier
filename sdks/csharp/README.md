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
  <a href="https://dotnet.microsoft.com/download/dotnet/10.0">
    <img src="https://img.shields.io/badge/.NET-10.0%2B-purple.svg" alt=".NET Version">
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

Croupier C# SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 .NET 客户端实现。支持 .NET 10+，提供两类能力：

- **Provider 端（`CroupierClient`）**：注册函数、被平台调用（核心能力）
- **Invoker 端（`CroupierInvoker`）**：作为调用方通过 Server HTTP API 发起同步 / 异步调用（独立能力）

Provider 通过 **单条 TCP session**（`sdk-agent subprotocol`）与 Agent 通信，不监听本地端口；Invoker 则独立连接 Server HTTP API，不复用 Provider session。

## 正式文档

- 功能矩阵（跨语言一致性的单一事实来源）：[`sdks/SDK_FEATURE_MATRIX.md`](../SDK_FEATURE_MATRIX.md)
- 线协议约定：[`docs/architecture/sdk-wire-protocol.md`](../../docs/architecture/sdk-wire-protocol.md)
- 统一文档站入口：`/docs/sdks/csharp/`
- 仓库内路径：`docs/sdks/csharp`

## 主项目

| 项目           | 描述                                  | 链接                                                           |
| -------------- | ------------------------------------- | -------------------------------------------------------------- |
| **Croupier**   | 游戏后端平台主项目（包含 Proto 定义） | [cuihairu/croupier](https://github.com/cuihairu/croupier)      |
| **Proto 文件** | 协议定义（Protobuf）                  | [proto/](https://github.com/cuihairu/croupier/tree/main/proto) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言   | 目录                                                                       | CI                                                                                                                                                                    | Docs                          |
| ------ | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| Go     | [sdks/go/](https://github.com/cuihairu/croupier/tree/main/sdks/go)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml)         | [README](../go/README.md)     |
| C++    | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp)       | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml)       | [README](../cpp/README.md)    |
| Java   | [sdks/java/](https://github.com/cuihairu/croupier/tree/main/sdks/java)     | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml)     | [README](../java/README.md)   |
| JS/TS  | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml)         | [README](../js/README.md)     |
| Python | [sdks/python/](https://github.com/cuihairu/croupier/tree/main/sdks/python) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](../python/README.md) |

## 支持平台

| 平台        | 架构                       | 状态    |
| ----------- | -------------------------- | ------- |
| **Windows** | x64                        | ✅ 支持 |
| **Linux**   | x64, ARM64                 | ✅ 支持 |
| **macOS**   | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

按 [功能矩阵](../SDK_FEATURE_MATRIX.md) 分层：

**L1 Core Provider（`CroupierClient`，必备）**

- 单条 TCP session 客户端（`sdk-agent subprotocol`），不监听本地端口
- 自动心跳与重连（`AutoReconnect` / `ReconnectIntervalSeconds` / `ReconnectMaxAttempts`）
- 异步 / 同步 handler，签名 `Func<string, string, Task<string>>`
- 多游戏多环境作用域（`GameId` / `Env` / `ServiceId`）

**L2 Provider 扩展（可选）**

- TLS（`CertFile` / `KeyFile` / `CaFile` / `ServerName`）
- 文件传输（`EnableFileTransfer=true`，受 `MaxFileSize` 约束）
- 灵活配置：环境变量、JSON 文件、内存配置
- 日志抽象（`ICroupierLogger`，可接 `ILogger`）

**L3 Invoker（`CroupierInvoker`，独立调用方）**

- 通过 Server HTTP API 进行同步调用、异步任务、状态查询与事件轮询
- 独立 `InvokerConfig`，不与 Provider 共享配置、DI 注册或生命周期

**L4 语言/引擎扩展（仅 C# 提供）**

- 依赖注入：`Extensions/ServiceCollectionExtensions.AddCroupier(...)`
- Unity 集成：`Unity/CroupierUnityBehaviour`

## 快速开始

### 系统要求

- **.NET 10.0 SDK** 或更高版本

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

### Provider 端：注册函数并服务（核心流程）

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;

var config = new ClientConfig
{
    AgentAddr = "127.0.0.1:19091",
    ServiceId = "my-service",
    GameId = "my-game",
    Env = "production"
};

var client = new CroupierClient(config);

var descriptor = new FunctionDescriptor
{
    Id = "player.get",
    Version = "1.0.0",
    Resource = "player",
    Operation = "get",
    Risk = "safe",
    Summary = "获取玩家信息",
    Description = "根据玩家 ID 获取玩家详细信息"
};

client.RegisterFunction(descriptor, async (context, payload) =>
{
    var response = new
    {
        status = "success",
        player = new { id = "123", name = "TestPlayer" }
    };

    return System.Text.Json.JsonSerializer.Serialize(response);
});

await client.ConnectAsync();
await client.ServeAsync(); // 阻塞直到停止
```

### Invoker 端：调用远程函数（独立能力）

`CroupierInvoker` 是独立的调用方能力，不在 Provider 主流程中。它需要 Server REST API 地址与调用权限的 Bearer Token：

```csharp
var invoker = new CroupierInvoker(new InvokerConfig
{
    ServerBaseUrl = "https://croupier.example/api/v1",
    AuthToken = Environment.GetEnvironmentVariable("CROUPIER_TOKEN"),
    GameId = "my-game",
    Env = "production"
});

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

若使用依赖注入，Provider 与 Invoker 必须显式、分开注册：

```csharp
services.AddCroupier(provider => provider.AgentAddr = "127.0.0.1:19091");
services.AddCroupierInvoker(invoker => invoker.ServerBaseUrl = "https://croupier.example/api/v1");
```

### 完整游戏后台 Demo

`examples/GameDemo/` 包含19个业务动作函数（player/order/leaderboard/inventory/mail），与 Go SDK demo 功能对齐：

```bash
dotnet run --project examples/GameDemo
```

### 启动服务

```csharp
await client.ServeAsync();
```

> Provider 端 `ConnectAsync()` + `ServeAsync()` 已包含主流程；下方"依赖注入"与"Unity 集成"属于 **L4 语言扩展**，不阻塞核心使用。

### 依赖注入（L4 语言扩展）

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

| 变量名                     | 说明                        | 默认值          |
| -------------------------- | --------------------------- | --------------- |
| `CROUPIER_AGENT_ADDR`      | Agent 本地 SDK gateway 地址 | 127.0.0.1:19091 |
| `CROUPIER_SERVICE_ID`      | 服务 ID                     | csharp-service  |
| `CROUPIER_GAME_ID`         | 游戏 ID                     | default-game    |
| `CROUPIER_ENV`             | 环境                        | dev             |
| `CROUPIER_INSECURE`        | 跳过 TLS 验证               | false           |
| `CROUPIER_CERT_FILE`       | 客户端证书路径              | -               |
| `CROUPIER_KEY_FILE`        | 客户端私钥路径              | -               |
| `CROUPIER_CA_FILE`         | CA 证书路径                 | -               |
| `CROUPIER_TIMEOUT_SECONDS` | 连接超时（秒）              | 30              |
| `CROUPIER_AUTO_RECONNECT`  | 自动重连                    | true            |

### 从环境变量加载配置

```csharp
using Croupier.Sdk.Configuration;

var configProvider = new EnvironmentConfigProvider();
var config = configProvider.GetConfig();
var client = new CroupierClient(config);
```

## 许可证

Apache License 2.0 - 详见 [LICENSE](LICENSE) 文件
