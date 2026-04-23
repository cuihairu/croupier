# 安装

## NuGet 包

Croupier C# SDK 通过 NuGet 分发：

```bash
dotnet add package Croupier.Sdk
```

## 从源码构建

### 前置要求

- .NET 8.0 SDK
- Git

### 克隆仓库

```bash
git clone https://github.com/cuihairu/croupier-sdk-csharp.git
cd croupier-sdk-csharp
```

### 构建

```bash
dotnet build
```

### 运行示例

```bash
cd examples/SimpleService
dotnet run
```

## 验证安装

创建测试程序验证安装：

```csharp
using Croupier.Sdk;
using Croupier.Sdk.Models;

var config = new ClientConfig();
Console.WriteLine($"Croupier C# SDK loaded");
Console.WriteLine($"Default Agent: {config.AgentAddr}");
```

编译并运行，应该看到 SDK 信息输出。
