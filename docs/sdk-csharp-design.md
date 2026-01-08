# Croupier C# SDK 设计文档

## 概述

Croupier C# SDK 是为 .NET 平台设计的 SDK，支持：
- **.NET 6/8+** - 现代服务端应用
- **Unity 2021+** - 游戏客户端（可选）

## 架构设计

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              .NET / Unity 应用                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│   ┌──────────────────────────────────────────────────────────────────────────┐  │
│   │                         C# SDK Layer                                     │  │
│   │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐                 │  │
│   │  │ CroupierClient│  │   Invoker     │  │  Extensions   │                 │  │
│   │  │   (Client)    │  │   (Invoker)   │  │  (DI/Logging) │                 │  │
│   │  └───────────────┘  └───────────────┘  └───────────────┘                 │  │
│   └──────────────────────────────────────────────────────────────────────────┘  │
│                                      │                                          │
│                                      │ Grpc.Net.Client                          │
│                                      ▼                                          │
│   ┌──────────────────────────────────────────────────────────────────────────┐  │
│   │                        Google.Protobuf                                   │  │
│   │                       Grpc.Core                                           │  │
│   └──────────────────────────────────────────────────────────────────────────┘  │
│                                      │                                          │
│                                      │ gRPC / HTTP/2                             │
│                                      ▼                                          │
│                    ┌─────────────┐                                              │
│                    │ Croupier    │                                              │
│                    │   Agent     │                                              │
│                    └─────────────┘                                              │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 目录结构

```
croupier-sdk-csharp/
├── src/
│   ├── Croupier.Sdk/                      # 主 SDK 项目
│   │   ├── CroupierClient.cs             # 客户端实现
│   │   ├── CroupierInvoker.cs            # 调用器实现
│   │   ├── IFunctionHandler.cs           # 函数处理器接口
│   │   ├── Models/
│   │   │   ├── FunctionDescriptor.cs    # 函数描述符
│   │   │   ├── ClientConfig.cs          # 客户端配置
│   │   │   ├── InvokeOptions.cs         # 调用选项
│   │   │   └── JobEvent.cs              # 任务事件
│   │   ├── Configuration/               # 配置管理
│   │   │   └── ClientConfigProvider.cs
│   │   ├── Logging/                     # 日志系统
│   │   │   └── ICroupierLogger.cs
│   │   └── Extensions/                  # 扩展方法
│   │       └── ServiceCollectionExtensions.cs
│   │
│   ├── Croupier.Sdk.Unity/               # Unity 集成（可选）
│   │   ├── Runtime/
│   │   │   └── CroupierRuntime.cs       # Unity 运行时
│   │   ├── Components/
│   │   │   └── CroupierBehaviour.cs     # MonoBehaviour
│   │   └── Editor/
│   │       └── CroupierEditor.cs
│   │
│   └── Croupier.Sdk.Extensions/          # 扩展包
│       ├── DependencyInjection/         # DI 集成
│       ├── Hosting/                     # 通用主机集成
│       └── Logging/
│           └── Serilog/                 # Serilog 集成
│
├── tests/                               # 测试
│   ├── Croupier.Sdk.Tests/
│   └── Croupier.Sdk.Unity.Tests/
│
├── examples/                            # 示例
│   ├── SimpleService/
│   ├── UnityDemo/
│   └── WebApi/
│
├── protos/                              # Protobuf 定义
│   └── croupier/
│
├── Directory.Build.props                # 构建配置
├── Croupier.Sdk.sln                     # 解决方案
└── README.md
```

## 核心代码

### 1. 客户端配置

```csharp
// src/Croupier.Sdk/Models/ClientConfig.cs

namespace Croupier.Sdk.Models;

/// <summary>
/// 客户端配置
/// </summary>
public class ClientConfig
{
    /// <summary>
    /// Agent 服务器地址
    /// </summary>
    public string AgentAddr { get; set; } = "127.0.0.1:19090";

    /// <summary>
    /// 服务标识符
    /// </summary>
    public string ServiceId { get; set; } = "csharp-service";

    /// <summary>
    /// 服务版本
    /// </summary>
    public string ServiceVersion { get; set; } = "1.0.0";

    /// <summary>
    /// 游戏 ID
    /// </summary>
    public string GameId { get; set; } = "default-game";

    /// <summary>
    /// 环境
    /// </summary>
    public string Env { get; set; } = "dev";

    /// <summary>
    /// 本地监听地址
    /// </summary>
    public string LocalAddr { get; set; } = "0.0.0.0:0";

    /// <summary>
    /// 是否使用不安全连接（跳过 TLS 验证）
    /// </summary>
    public bool Insecure { get; set; }

    /// <summary>
    /// TLS 证书文件路径
    /// </summary>
    public string? CertFile { get; set; }

    /// <summary>
    /// TLS 私钥文件路径
    /// </summary>
    public string? KeyFile { get; set; }

    /// <summary>
    /// TLS CA 证书文件路径
    /// </summary>
    public string? CaFile { get; set; }

    /// <summary>
    /// 服务器名称（用于 SNI）
    /// </summary>
    public string? ServerName { get; set; }

    /// <summary>
    /// 超时时间（秒）
    /// </summary>
    public int TimeoutSeconds { get; set; } = 30;

    /// <summary>
    /// 心跳间隔（秒）
    /// </summary>
    public int HeartbeatInterval { get; set; } = 30;

    /// <summary>
    /// 自动重连
    /// </summary>
    public bool AutoReconnect { get; set; } = true;

    /// <summary>
    /// 重连间隔（秒）
    /// </summary>
    public int ReconnectIntervalSeconds { get; set; } = 5;

    /// <summary>
    /// 最大重连次数（0 表示无限重试）
    /// </summary>
    public int ReconnectMaxAttempts { get; set; } = 0;

    /// <summary>
    /// 验证配置
    /// </summary>
    public void Validate()
    {
        if (string.IsNullOrWhiteSpace(AgentAddr))
            throw new InvalidOperationException("AgentAddr is required");

        if (string.IsNullOrWhiteSpace(ServiceId))
            throw new InvalidOperationException("ServiceId is required");

        if (string.IsNullOrWhiteSpace(GameId))
            throw new InvalidOperationException("GameId is required");

        if (!Insecure && string.IsNullOrWhiteSpace(CertFile))
            throw new InvalidOperationException("CertFile is required when secure connection is enabled");
    }
}
```

### 2. 函数描述符

```csharp
// src/Croupier.Sdk/Models/FunctionDescriptor.cs

namespace Croupier.Sdk.Models;

/// <summary>
/// 函数描述符
/// </summary>
public class FunctionDescriptor
{
    /// <summary>
    /// 函数唯一标识符
    /// </summary>
    public string Id { get; set; } = string.Empty;

    /// <summary>
    /// 函数版本
    /// </summary>
    public string Version { get; set; } = "1.0.0";

    /// <summary>
    /// 函数分类
    /// </summary>
    public string Category { get; set; } = string.Empty;

    /// <summary>
    /// 风险等级 (low, medium, high)
    /// </summary>
    public string Risk { get; set; } = "low";

    /// <summary>
    /// 实体类型
    /// </summary>
    public string? Entity { get; set; }

    /// <summary>
    /// 操作类型 (create, read, update, delete)
    /// </summary>
    public string? Operation { get; set; }

    /// <summary>
    /// 是否启用
    /// </summary>
    public bool Enabled { get; set; } = true;

    /// <summary>
    /// 函数显示名称
    /// </summary>
    public string? DisplayName { get; set; }

    /// <summary>
    /// 函数描述
    /// </summary>
    public string? Description { get; set; }

    /// <summary>
    /// 输入参数 JSON Schema
    /// </summary>
    public string? InputSchema { get; set; }

    /// <summary>
    /// 输出参数 JSON Schema
    /// </summary>
    public string? OutputSchema { get; set; }

    /// <summary>
    /// 自定义标签
    /// </summary>
    public Dictionary<string, string> Tags { get; set; } = new();

    /// <summary>
    /// 验证描述符
    /// </summary>
    public void Validate()
    {
        if (string.IsNullOrWhiteSpace(Id))
            throw new InvalidOperationException("Function Id is required");

        if (string.IsNullOrWhiteSpace(Version))
            throw new InvalidOperationException("Function Version is required");

        if (string.IsNullOrWhiteSpace(Category))
            throw new InvalidOperationException("Function Category is required");

        var validRisks = new[] { "low", "medium", "high" };
        if (!validRisks.Contains(Risk?.ToLower()))
            throw new ArgumentException($"Risk must be one of: {string.Join(", ", validRisks)}");

        var validOperations = new[] { "create", "read", "update", "delete" };
        if (!string.IsNullOrEmpty(Operation) && !validOperations.Contains(Operation?.ToLower()))
            throw new ArgumentException($"Operation must be one of: {string.Join(", ", validOperations)}");
    }
}
```

### 3. 函数处理器接口

```csharp
// src/Croupier.Sdk/IFunctionHandler.cs

namespace Croupier.Sdk;

/// <summary>
/// 函数处理器接口
/// </summary>
public interface IFunctionHandler
{
    /// <summary>
    /// 处理函数调用
    /// </summary>
    /// <param name="context">调用上下文</param>
    /// <param name="payload">输入负载</param>
    /// <param name="cancellationToken">取消令牌</param>
    /// <returns>函数执行结果</returns>
    Task<string> HandleAsync(FunctionContext context, string payload, CancellationToken cancellationToken = default);
}

/// <summary>
/// 函数上下文
/// </summary>
public class FunctionContext
{
    /// <summary>
    /// 调用 ID
    /// </summary>
    public string CallId { get; set; } = string.Empty;

    /// <summary>
    /// 函数 ID
    /// </summary>
    public string FunctionId { get; set; } = string.Empty;

    /// <summary>
    /// 游戏 ID
    /// </summary>
    public string GameId { get; set; } = string.Empty;

    /// <summary>
    /// 环境
    /// </summary>
    public string Env { get; set; } = string.Empty;

    /// <summary>
    /// 调用者信息
    /// </summary>
    public CallerInfo? Caller { get; set; }

    /// <summary>
    /// 调用时间戳
    /// </summary>
    public DateTime Timestamp { get; set; } = DateTime.UtcNow;

    /// <summary>
    /// 自定义属性
    /// </summary>
    public Dictionary<string, string> Properties { get; set; } = new();
}

/// <summary>
/// 调用者信息
/// </summary>
public class CallerInfo
{
    public string UserId { get; set; } = string.Empty;
    public string ServiceId { get; set; } = string.Empty;
    public string? IpAddress { get; set; }
    public string? UserAgent { get; set; }
}

/// <summary>
/// 同步函数处理器委托
/// </summary>
public delegate string FunctionHandlerSync(FunctionContext context, string payload);

/// <summary>
/// 异步函数处理器委托
/// </summary>
public delegate Task<string> FunctionHandlerAsync(FunctionContext context, string payload, CancellationToken cancellationToken);
```

### 4. 客户端实现

```csharp
// src/Croupier.Sdk/CroupierClient.cs

using Croupier.Sdk.Models;
using Grpc.Core;
using Grpc.Net.Client;
using Microsoft.Extensions.Logging;

namespace Croupier.Sdk;

/// <summary>
/// Croupier 客户端
/// </summary>
public class CroupierClient : IAsyncDisposable
{
    private readonly ClientConfig _config;
    private readonly ILogger<CroupierClient> _logger;
    private readonly Dictionary<string, RegisteredFunction> _functions = new();
    private readonly CancellationTokenSource _shutdownCts = new();

    private GrpcChannel? _channel;
    private AgentService.AgentServiceClient? _agentClient;
    private ServerService.ServerServiceServer? _server;
    private Task? _serveTask;
    private bool _disposed;

    /// <summary>
    /// 客户端配置
    /// </summary>
    public ClientConfig Config => _config;

    /// <summary>
    /// 是否已连接
    /// </summary>
    public bool IsConnected { get; private set; }

    /// <summary>
    /// 本地监听地址
    /// </summary>
    public string? LocalAddress { get; private set; }

    /// <summary>
    /// 构造函数
    /// </summary>
    public CroupierClient(ClientConfig config, ILogger<CroupierClient>? logger = null)
    {
        _config = config ?? throw new ArgumentNullException(nameof(config));
        _config.Validate();
        _logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger<CroupierClient>.Instance;
    }

    /// <summary>
    /// 注册函数
    /// </summary>
    public void RegisterFunction(FunctionDescriptor descriptor, IFunctionHandler handler)
    {
        ThrowIfDisposed();

        descriptor.Validate();

        if (_functions.ContainsKey(descriptor.Id))
            throw new InvalidOperationException($"Function '{descriptor.Id}' is already registered");

        var registeredFunc = new RegisteredFunction
        {
            Descriptor = descriptor,
            Handler = handler
        };

        _functions[descriptor.Id] = registeredFunc;

        _logger.LogInformation("Function registered: {FunctionId} v{Version}", descriptor.Id, descriptor.Version);
    }

    /// <summary>
    /// 注册函数（委托形式）
    /// </summary>
    public void RegisterFunction(FunctionDescriptor descriptor, FunctionHandlerAsync handler)
    {
        RegisterFunction(descriptor, new DelegateFunctionHandler(handler));
    }

    /// <summary>
    /// 注册函数（同步委托形式）
    /// </summary>
    public void RegisterFunction(FunctionDescriptor descriptor, FunctionHandlerSync handler)
    {
        RegisterFunction(descriptor, new SyncFunctionHandlerWrapper(handler));
    }

    /// <summary>
    /// 连接到 Agent
    /// </summary>
    public async Task ConnectAsync(CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (IsConnected)
            return;

        _logger.LogInformation("Connecting to Agent at {AgentAddr}...", _config.AgentAddr);

        try
        {
            // 创建 gRPC 通道
            _channel = CreateGrpcChannel();

            _agentClient = new AgentService.AgentServiceClient(_channel);

            // 启动本地 gRPC 服务器
            _server = CreateServer();

            await _server.StartAsync(cancellationToken);

            LocalAddress = _server.Ports.FirstOrDefault().ToString();

            _logger.LogInformation("Local gRPC server started on port {Port}", LocalAddress);

            // 注册到 Agent
            await RegisterToAgentAsync(cancellationToken);

            IsConnected = true;

            _logger.LogInformation("Connected to Agent successfully");
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to connect to Agent");
            await CleanupAsync();
            throw;
        }
    }

    /// <summary>
    /// 开始服务（接收函数调用）
    /// </summary>
    public Task ServeAsync(CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (!IsConnected)
            throw new InvalidOperationException("Not connected. Call ConnectAsync first.");

        if (_serveTask != null)
            throw new InvalidOperationException("Already serving");

        _logger.LogInformation("Starting to serve...");

        // 使用 Combine 链接外部取消和内部取消
        var linkedCts = CancellationTokenSource.CreateLinkedTokenSource(
            cancellationToken,
            _shutdownCts.Token);

        _serveTask = Task.Run(async () =>
        {
            try
            {
                // 发送心跳
                await StartHeartbeatAsync(linkedCts.Token);

                // 等待服务器关闭
                await _server!.ShutdownAsync(linkedCts.Token);
            }
            catch (OperationCanceledException)
            {
                // 正常关闭
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Serve loop error");
            }
        }, linkedCts.Token);

        return _serveTask;
    }

    /// <summary>
    /// 停止服务
    /// </summary>
    public async Task StopAsync()
    {
        if (_serveTask == null)
            return;

        _logger.LogInformation("Stopping...");

        _shutdownCts.Cancel();

        try
        {
            await _serveTask;
        }
        catch (OperationCanceledException)
        {
            // 忽略取消异常
        }

        _serveTask = null;
        _logger.LogInformation("Stopped");
    }

    /// <summary>
    /// 断开连接
    /// </summary>
    public async Task DisconnectAsync()
    {
        await StopAsync();
        await CleanupAsync();
        IsConnected = false;
        _logger.LogInformation("Disconnected");
    }

    /// <summary>
    /// 创建 gRPC 通道
    /// </summary>
    private GrpcChannel CreateGrpcChannel()
    {
        var credentials = _config.Insecure
            ? ChannelCredentials.Insecure
            : ChannelCredentials.CreateSsl(
                LoadCert(_config.CertFile),
                LoadCert(_config.KeyFile),
                LoadCert(_config.CaFile));

        var options = new GrpcChannelOptions
        {
            MaxReceiveMessageSize = 4 * 1024 * 1024, // 4MB
            MaxSendMessageSize = 4 * 1024 * 1024,
            Credentials = credentials,
            // 添加拦截器用于日志
            Interceptors = new[] { new LoggingInterceptor(_logger) }
        };

        // 解析地址
        var uri = new Uri($"dns://{_config.AgentAddr}");

        return GrpcChannel.ForAddress(uri, options);
    }

    /// <summary>
    /// 创建本地服务器
    /// </summary>
    private ServerService.ServerServiceServer CreateServer()
    {
        var server = new ServerService.ServerServiceServer
        {
            Ports = { { Server.PickUnused, ServerCredentials.Insecure } },
            Services = { ServerService.BindService(new ServerServiceImpl(_functions, _logger)) }
        };

        return server;
    }

    /// <summary>
    /// 注册到 Agent
    /// </summary>
    private async Task RegisterToAgentAsync(CancellationToken cancellationToken)
    {
        var request = new RegisterRequest
        {
            ServiceId = _config.ServiceId,
            Version = _config.ServiceVersion,
            GameId = _config.GameId,
            Env = _config.Env,
            LocalAddr = $"127.0.0.1:{LocalAddress}"
        };

        foreach (var func in _functions.Values)
        {
            request.Functions.Add(new FunctionInfo
            {
                Id = func.Descriptor.Id,
                Version = func.Descriptor.Version,
                Category = func.Descriptor.Category,
                Risk = func.Descriptor.Risk,
                Entity = func.Descriptor.Entity ?? "",
                Operation = func.Descriptor.Operation ?? "",
                Enabled = func.Descriptor.Enabled
            });
        }

        var response = await _agentClient!.RegisterAsync(request,
            deadline: DateTime.UtcNow.AddSeconds(_config.TimeoutSeconds),
            cancellationToken: cancellationToken);

        if (!response.Success)
        {
            throw new InvalidOperationException($"Registration failed: {response.ErrorMessage}");
        }

        _logger.LogInformation("Registered {Count} functions to Agent", _functions.Count);
    }

    /// <summary>
    /// 启动心跳
    /// </summary>
    private async Task StartHeartbeatAsync(CancellationToken cancellationToken)
    {
        while (!cancellationToken.IsCancellationRequested)
        {
            try
            {
                await Task.Delay(_config.HeartbeatInterval * 1000, cancellationToken);

                var request = new HeartbeatRequest
                {
                    ServiceId = _config.ServiceId,
                    Timestamp = Timestamp.FromDateTime(DateTime.UtcNow)
                };

                await _agentClient!.HeartbeatAsync(request, cancellationToken: cancellationToken);
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Heartbeat failed");

                if (_config.AutoReconnect)
                {
                    _logger.LogInformation("Attempting to reconnect...");
                    await CleanupAsync();
                    await ConnectAsync(cancellationToken);
                }
                else
                {
                    throw;
                }
            }
        }
    }

    /// <summary>
    /// 清理资源
    /// </summary>
    private async Task CleanupAsync()
    {
        if (_server != null)
        {
            await _server.ShutdownAsync();
            _server = null;
        }

        _channel?.Dispose();
        _channel = null;
        _agentClient = null;
    }

    /// <summary>
    /// 加载证书
    /// </summary>
    private byte[]? LoadCert(string? path)
    {
        if (string.IsNullOrEmpty(path))
            return null;

        if (!File.Exists(path))
            throw new FileNotFoundException($"Certificate file not found: {path}");

        return File.ReadAllBytes(path);
    }

    private void ThrowIfDisposed()
    {
        if (_disposed)
            throw new ObjectDisposedException(nameof(CroupierClient));
    }

    /// <summary>
    /// 释放资源
    /// </summary>
    public async ValueTask DisposeAsync()
    {
        if (_disposed)
            return;

        _disposed = true;

        await DisconnectAsync();
        _shutdownCts.Dispose();

        GC.SuppressFinalize(this);
    }

    /// <summary>
    /// 委托函数处理器包装器
    /// </summary>
    private class DelegateFunctionHandler : IFunctionHandler
    {
        private readonly FunctionHandlerAsync _handler;

        public DelegateFunctionHandler(FunctionHandlerAsync handler)
        {
            _handler = handler;
        }

        public Task<string> HandleAsync(FunctionContext context, string payload, CancellationToken cancellationToken)
        {
            return _handler(context, payload, cancellationToken);
        }
    }

    /// <summary>
    /// 同步函数处理器包装器
    /// </summary>
    private class SyncFunctionHandlerWrapper : IFunctionHandler
    {
        private readonly FunctionHandlerSync _handler;

        public SyncFunctionHandlerWrapper(FunctionHandlerSync handler)
        {
            _handler = handler;
        }

        public Task<string> HandleAsync(FunctionContext context, string payload, CancellationToken cancellationToken)
        {
            return Task.FromResult(_handler(context, payload));
        }
    }
}

/// <summary>
/// 已注册的函数
/// </summary>
internal class RegisteredFunction
{
    public FunctionDescriptor Descriptor { get; set; } = null!;
    public IFunctionHandler Handler { get; set; } = null!;
}
```

### 5. 调用器实现

```csharp
// src/Croupier.Sdk/CroupierInvoker.cs

using Croupier.Sdk.Models;
using Grpc.Net.Client;

namespace Croupier.Sdk;

/// <summary>
/// Croupier 调用器
/// </summary>
public class CroupierInvoker : IAsyncDisposable
{
    private readonly ClientConfig _config;
    private readonly ILogger<CroupierInvoker> _logger;
    private GrpcChannel? _channel;
    private AgentService.AgentServiceClient? _client;
    private bool _disposed;

    /// <summary>
    /// 构造函数
    /// </summary>
    public CroupierInvoker(ClientConfig config, ILogger<CroupierInvoker>? logger = null)
    {
        _config = config ?? throw new ArgumentNullException(nameof(config));
        _config.Validate();
        _logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger<CroupierInvoker>.Instance;
    }

    /// <summary>
    /// 同步调用函数
    /// </summary>
    public async Task<string> InvokeAsync(
        string functionId,
        string payload,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        EnsureConnected();

        options ??= new InvokeOptions();

        var request = new InvokeRequest
        {
            FunctionId = functionId,
            Payload = payload,
            GameId = options.GameId ?? _config.GameId,
            Env = options.Env ?? _config.Env,
        };

        if (!string.IsNullOrEmpty(options.IdempotencyKey))
            request.IdempotencyKey = options.IdempotencyKey;

        try
        {
            var call = _client!.InvokeAsync(request,
                deadline: DateTime.UtcNow.AddSeconds(options.Timeout),
                cancellationToken: cancellationToken);

            var response = await call;

            if (response.Success)
            {
                _logger.LogDebug("Invoke succeeded: {FunctionId}", functionId);
                return response.Result ?? string.Empty;
            }
            else
            {
                throw new CroupierException($"Invoke failed: {response.ErrorMessage}");
            }
        }
        catch (RpcException ex)
        {
            _logger.LogError(ex, "RPC error calling {FunctionId}", functionId);
            throw new CroupierException($"RPC error: {ex.Status}", ex);
        }
    }

    /// <summary>
    /// 同步调用函数（带类型化响应）
    /// </summary>
    public async Task<T> InvokeAsync<T>(
        string functionId,
        object payload,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        var payloadJson = System.Text.Json.JsonSerializer.Serialize(payload);
        var responseJson = await InvokeAsync(functionId, payloadJson, options, cancellationToken);
        return System.Text.Json.JsonSerializer.Deserialize<T>(responseJson)!;
    }

    /// <summary>
    /// 启动异步任务
    /// </summary>
    public async Task<string> StartJobAsync(
        string functionId,
        string payload,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        EnsureConnected();

        options ??= new InvokeOptions();

        var request = new StartJobRequest
        {
            FunctionId = functionId,
            Payload = payload,
            GameId = options.GameId ?? _config.GameId,
            Env = options.Env ?? _config.Env,
        };

        if (!string.IsNullOrEmpty(options.IdempotencyKey))
            request.IdempotencyKey = options.IdempotencyKey;

        try
        {
            var response = await _client!.StartJobAsync(request,
                deadline: DateTime.UtcNow.AddSeconds(options.Timeout),
                cancellationToken: cancellationToken);

            if (!string.IsNullOrEmpty(response.JobId))
            {
                _logger.LogDebug("Job started: {JobId} for {FunctionId}", response.JobId, functionId);
                return response.JobId;
            }
            else
            {
                throw new CroupierException($"Start job failed: {response.ErrorMessage}");
            }
        }
        catch (RpcException ex)
        {
            _logger.LogError(ex, "RPC error starting job {FunctionId}", functionId);
            throw new CroupierException($"RPC error: {ex.Status}", ex);
        }
    }

    /// <summary>
    /// 流式读取任务事件
    /// </summary>
    public async IAsyncEnumerable<JobEvent> StreamJobAsync(
        string jobId,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        EnsureConnected();

        var request = new StreamJobRequest { JobId = jobId };

        var call = _client!.StreamJobAsync(request, cancellationToken: cancellationToken);

        await foreach (var response in call.ResponseStream.ReadAllAsync(cancellationToken))
        {
            yield return new JobEvent
            {
                EventType = response.EventType,
                JobId = response.JobId,
                Timestamp = response.Timestamp.ToDateTime(),
                Data = response.Data.ToString()
            };
        }
    }

    /// <summary>
    /// 取消任务
    /// </summary>
    public async Task<bool> CancelJobAsync(
        string jobId,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        EnsureConnected();

        var request = new CancelJobRequest { JobId = jobId };

        try
        {
            var response = await _client!.CancelJobAsync(request, cancellationToken: cancellationToken);
            return response.Cancelled;
        }
        catch (RpcException)
        {
            return false;
        }
    }

    /// <summary>
    /// 确保已连接
    /// </summary>
    private void EnsureConnected()
    {
        if (_client == null)
        {
            var channel = CreateGrpcChannel();
            _channel = channel;
            _client = new AgentService.AgentServiceClient(channel);
        }
    }

    /// <summary>
    /// 创建 gRPC 通道
    /// </summary>
    private GrpcChannel CreateGrpcChannel()
    {
        var credentials = _config.Insecure
            ? ChannelCredentials.Insecure
            : ChannelCredentials.CreateSsl(
                LoadCert(_config.CertFile),
                LoadCert(_config.KeyFile),
                LoadCert(_config.CaFile));

        var options = new GrpcChannelOptions
        {
            MaxReceiveMessageSize = 4 * 1024 * 1024,
            MaxSendMessageSize = 4 * 1024 * 1024,
            Credentials = credentials
        };

        var uri = new Uri($"dns://{_config.AgentAddr}");
        return GrpcChannel.ForAddress(uri, options);
    }

    private byte[]? LoadCert(string? path)
    {
        if (string.IsNullOrEmpty(path))
            return null;

        return File.ReadAllBytes(path);
    }

    private void ThrowIfDisposed()
    {
        if (_disposed)
            throw new ObjectDisposedException(nameof(CroupierInvoker));
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed)
            return;

        _disposed = true;

        _channel?.Dispose();
        _channel = null;
        _client = null;

        GC.SuppressFinalize(this);
    }
}
```

### 6. 调用选项和模型

```csharp
// src/Croupier.Sdk/Models/InvokeOptions.cs

namespace Croupier.Sdk.Models;

/// <summary>
/// 调用选项
/// </summary>
public class InvokeOptions
{
    /// <summary>
    /// 游戏 ID（覆盖默认值）
    /// </summary>
    public string? GameId { get; set; }

    /// <summary>
    /// 环境（覆盖默认值）
    /// </summary>
    public string? Env { get; set; }

    /// <summary>
    /// 超时时间（秒）
    /// </summary>
    public int Timeout { get; set; } = 30;

    /// <summary>
    /// 幂等键（用于防重）
    /// </summary>
    public string? IdempotencyKey { get; set; }
}

/// <summary>
/// 任务事件
/// </summary>
public class JobEvent
{
    /// <summary>
    /// 事件类型 (queued, running, completed, failed, cancelled)
    /// </summary>
    public string EventType { get; set; } = string.Empty;

    /// <summary>
    /// 任务 ID
    /// </summary>
    public string JobId { get; set; } = string.Empty;

    /// <summary>
    /// 事件时间戳
    /// </summary>
    public DateTime Timestamp { get; set; }

    /// <summary>
    /// 事件数据
    /// </summary>
    public string Data { get; set; } = string.Empty;
}

/// <summary>
/// Croupier 异常
/// </summary>
public class CroupierException : Exception
{
    public CroupierException(string message) : base(message) { }
    public CroupierException(string message, Exception innerException) : base(message, innerException) { }
}
```

### 7. 依赖注入集成

```csharp
// src/Croupier.Sdk.Extensions/ServiceCollectionExtensions.cs

using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Logging;
using Croupier.Sdk.Models;

namespace Croupier.Sdk.Extensions;

/// <summary>
/// 服务集合扩展
/// </summary>
public static class ServiceCollectionExtensions
{
    /// <summary>
    /// 添加 Croupier 客户端
    /// </summary>
    public static IServiceCollection AddCroupierClient(
        this IServiceCollection services,
        IConfiguration configuration,
        Action<ClientConfig>? configure = null)
    {
        services.Configure<ClientConfig>(configuration.GetSection("Croupier"));

        if (configure != null)
        {
            services.PostConfigure<ClientConfig>(config =>
            {
                configure(config);
                config.Validate();
            });
        }

        services.AddSingleton<ICroupierClient>(sp =>
        {
            var config = sp.GetRequiredService<IOptions<ClientConfig>>().Value;
            var logger = sp.GetRequiredService<ILogger<CroupierClient>>();
            return new CroupierClient(config, logger);
        });

        services.AddSingleton<ICroupierInvoker>(sp =>
        {
            var config = sp.GetRequiredService<IOptions<ClientConfig>>().Value;
            var logger = sp.GetRequiredService<ILogger<CroupierInvoker>>();
            return new CroupierInvoker(config, logger);
        });

        return services;
    }

    /// <summary>
    /// 添加 Croupier 客户端（简化版本）
    /// </summary>
    public static IServiceCollection AddCroupierClient(
        this IServiceCollection services,
        Action<ClientConfig> configure)
    {
        services.Configure<ClientConfig>(config =>
        {
            configure(config);
            config.Validate();
        });

        return services.AddCroupierClientInternal();
    }

    private static IServiceCollection AddCroupierClientInternal(this IServiceCollection services)
    {
        services.AddSingleton<ICroupierClient>(sp =>
        {
            var config = sp.GetRequiredService<IOptions<ClientConfig>>().Value;
            var logger = sp.GetRequiredService<ILogger<CroupierClient>>();
            return new CroupierClient(config, logger);
        });

        services.AddSingleton<ICroupierInvoker>(sp =>
        {
            var config = sp.GetRequiredService<IOptions<ClientConfig>>().Value;
            var logger = sp.GetRequiredService<ILogger<CroupierInvoker>>();
            return new CroupierInvoker(config, logger);
        });

        return services;
    }

    /// <summary>
    /// 添加托管服务（自动连接和启动）
    /// </summary>
    public static IServiceCollection AddCroupierHostedService(this IServiceCollection services)
    {
        services.AddSingleton<IHostedService, CroupierHostedService>();
        return services;
    }
}

/// <summary>
/// Croupier 托管服务
/// </summary>
internal class CroupierHostedService : IHostedService, IAsyncDisposable
{
    private readonly ICroupierClient _client;
    private readonly ILogger<CroupierHostedService> _logger;
    private Task? _serveTask;

    public CroupierHostedService(ICroupierClient client, ILogger<CroupierHostedService> logger)
    {
        _client = client;
        _logger = logger;
    }

    public async Task StartAsync(CancellationToken cancellationToken)
    {
        _logger.LogInformation("Starting Croupier hosted service...");

        await _client.ConnectAsync(cancellationToken);
        _serveTask = _client.ServeAsync(cancellationToken);

        _logger.LogInformation("Croupier hosted service started");
    }

    public async Task StopAsync(CancellationToken cancellationToken)
    {
        _logger.LogInformation("Stopping Croupier hosted service...");

        await _client.DisconnectAsync();

        if (_serveTask != null)
        {
            await _serveTask;
        }

        _logger.LogInformation("Croupier hosted service stopped");
    }

    public async ValueTask DisposeAsync()
    {
        await _client.DisposeAsync();
    }
}
```

### 8. Unity 集成（可选）

```csharp
// src/Croupier.Sdk.Unity/Runtime/CroupierRuntime.cs

using UnityEngine;
using System.Threading.Tasks;

namespace Croupier.Unity
{
    /// <summary>
    /// Croupier Unity 运行时
    /// </summary>
    public class CroupierRuntime : MonoBehaviour
    {
        [Header("Configuration")]
        [SerializeField] private string agentAddr = "127.0.0.1:19090";
        [SerializeField] private string serviceId = "unity-client";
        [SerializeField] private string gameId = "unity-game";
        [SerializeField] private bool insecure = true;

        private CroupierClient? _client;
        private readonly CancellationTokenSource _cts = new();

        /// <summary>
        /// 是否已连接
        /// </summary>
        public bool IsConnected => _client != null && _client.IsConnected;

        /// <summary>
        /// Unity Awake
        /// </summary>
        private void Awake()
        {
            DontDestroyOnLoad(this.gameObject);
        }

        /// <summary>
        /// Unity Start
        /// </summary>
        private void Start()
        {
            _ = InitializeAsync();
        }

        /// <summary>
        /// Unity OnDestroy
        /// </summary>
        private void OnDestroy()
        {
            _cts.Cancel();
            _client?.DisposeAsync();
        }

        /// <summary>
        /// 初始化客户端
        /// </summary>
        private async Task InitializeAsync()
        {
            var config = new ClientConfig
            {
                AgentAddr = agentAddr,
                ServiceId = serviceId,
                GameId = gameId,
                Env = "production",
                Insecure = insecure
            };

            _client = new CroupierClient(config);

            try
            {
                await _client.ConnectAsync(_cts.Token);
                Debug.Log("[Croupier] Connected to Agent");

                // 启动服务
                _ = _client.ServeAsync(_cts.Token);
            }
            catch (Exception ex)
            {
                Debug.LogError($"[Croupier] Connection failed: {ex.Message}");
            }
        }

        /// <summary>
        /// 注册函数
        /// </summary>
        public void RegisterFunction(FunctionDescriptor descriptor, FunctionHandlerAsync handler)
        {
            _client?.RegisterFunction(descriptor, handler);
        }

        /// <summary>
        /// 调用远程函数
        /// </summary>
        public async Task<string> InvokeAsync(string functionId, string payload)
        {
            if (_client == null)
                throw new InvalidOperationException("Client not initialized");

            var invoker = new CroupierInvoker(_client.Config);
            return await invoker.InvokeAsync(functionId, payload);
        }
    }
}
```

### 9. 配置文件示例

```json
// appsettings.json
{
  "Croupier": {
    "AgentAddr": "127.0.0.1:19090",
    "ServiceId": "webapi-service",
    "ServiceVersion": "1.0.0",
    "GameId": "my-game",
    "Env": "production",
    "Insecure": false,
    "CertFile": "/path/to/cert.pem",
    "KeyFile": "/path/to/key.pem",
    "CaFile": "/path/to/ca.pem",
    "TimeoutSeconds": 30,
    "HeartbeatInterval": 30,
    "AutoReconnect": true
  }
}
```

### 10. 使用示例

```csharp
// SimpleService/Program.cs

using Croupier.Sdk;
using Croupier.Sdk.Extensions;
using Croupier.Sdk.Models;

// 创建客户端
var config = new ClientConfig
{
    AgentAddr = "127.0.0.1:19090",
    ServiceId = "game-service-1",
    GameId = "my-game",
    Env = "production"
};

using var client = new CroupierClient(config);

// 注册函数
client.RegisterFunction(new FunctionDescriptor
{
    Id = "player.get",
    Version = "1.0.0",
    Category = "player",
    Risk = "low",
    DisplayName = "获取玩家信息",
    Description = "根据玩家ID获取玩家详细信息"
}, async (context, payload, ct) =>
{
    // 解析输入
    var request = JsonSerializer.Deserialize<PlayerRequest>(payload);

    // 业务逻辑
    var player = await _playerService.GetAsync(request.PlayerId, ct);

    // 返回结果
    return JsonSerializer.Serialize(new PlayerResponse
    {
        Status = "success",
        Player = player
    });
});

// 注册函数（简单形式）
client.RegisterFunction(new FunctionDescriptor
{
    Id = "item.give",
    Version = "1.0.0",
    Category = "inventory",
    Risk = "medium"
}, async (context, payload, ct) =>
{
    // 处理逻辑...
    return await _inventoryService.GiveItemAsync(payload, ct);
});

// 连接并启动服务
await client.ConnectAsync();
await client.ServeAsync();

// 等待退出信号
Console.WriteLine("Press Ctrl+C to exit");
var tcs = new TaskCompletionSource();
Console.CancelKeyPress += (_, _) => tcs.SetResult();
await tcs.Task;
```

## 项目文件

```xml
<!-- src/Croupier.Sdk/Croupier.Sdk.csproj -->

<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <RootNamespace>Croupier.Sdk</RootNamespace>
    <Version>1.0.0</Version>
    <Authors>cuihairu</Authors>
    <Description>Croupier SDK for .NET</Description>
    <PackageProjectUrl>https://github.com/cuihairu/croupier-sdk-csharp</PackageProjectUrl>
    <RepositoryUrl>https://github.com/cuihairu/croupier-sdk-csharp</RepositoryUrl>
    <PackageLicenseExpression>MIT</PackageLicenseExpression>
    <GenerateDocumentationFile>true</GenerateDocumentationFile>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Grpc.Net.Client" Version="2.59.0" />
    <PackageReference Include="Grpc.Net.Client.Web" Version="2.59.0" />
    <PackageReference Include="Google.Protobuf" Version="3.25.2" />
    <PackageReference Include="Microsoft.Extensions.Logging" Version="8.0.0" />
    <PackageReference Include="Microsoft.Extensions.DependencyInjection" Version="8.0.0" />
    <PackageReference Include="Microsoft.Extensions.Options.ConfigurationExtensions" Version="8.0.0" />
    <PackageReference Include="Microsoft.Extensions.Hosting.Abstractions" Version="8.0.0" />
    <Protobuf Include="../protos/**/*.proto" GrpcServices="true" />
  </ItemGroup>

</Project>
```

## 编译和发布

```bash
# 构建
dotnet build Croupier.Sdk.sln -c Release

# 运行测试
dotnet test Croupier.Sdk.sln

# 打包
dotnet pack src/Croupier.Sdk/Croupier.Sdk.csproj -c Release -o ./nupkg

# 发布到 NuGet
dotnet nuget push ./nupkg/Croupier.Sdk.1.0.0.nupkg --source nuget.org
```
