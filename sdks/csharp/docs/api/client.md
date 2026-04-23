# CroupierClient API

## 构造函数

### CroupierClient(ClientConfig? config = null, ICroupierLogger? logger = null)

创建一个新的 Croupier 客户端实例。

**参数：**
- `config` - 客户端配置，为 null 时使用默认配置
- `logger` - 日志记录器，为 null 时使用控制台日志

**示例：**
```csharp
// 使用默认配置
var client = new CroupierClient();

// 使用自定义配置
var config = new ClientConfig {
    AgentAddr = "192.168.1.100:19090",
    GameId = "production-game"
};
var client = new CroupierClient(config);

// 使用自定义日志
var client = new CroupierClient(config, new MyCustomLogger());
```

## 属性

### Config

获取客户端配置。

```csharp
public ClientConfig Config { get; }
```

### IsConnected

获取是否已连接到 Agent。

```csharp
public bool IsConnected { get; }
```

### LocalAddress

获取本地监听地址。

```csharp
public string? LocalAddress { get; }
```

## 方法

### ConnectAsync

连接到 Agent。

```csharp
public Task ConnectAsync(CancellationToken cancellationToken = default)
```

**异常：**
- `InvalidOperationException` - 已连接时抛出
- `TimeoutException` - 连接超时
- `ConnectionException` - 连接失败

### Disconnect

断开与 Agent 的连接。

```csharp
public void Disconnect()
```

### ServeAsync

启动服务，开始接收函数调用。

```csharp
public Task ServeAsync(CancellationToken cancellationToken = default)
```

**注意：** 这是一个阻塞方法，会持续运行直到取消。

### Stop

停止服务。

```csharp
public void Stop()
```

### RegisterFunction

注册函数处理器。

```csharp
// 使用接口
public void RegisterFunction(FunctionDescriptor descriptor, IFunctionHandler handler)

// 使用异步委托
public void RegisterFunction(FunctionDescriptor descriptor, FunctionHandlerDelegate handler)

// 使用同步委托
public void RegisterFunction(FunctionDescriptor descriptor, SyncFunctionHandlerDelegate handler)
```

### UnregisterFunction

取消注册函数。

```csharp
public bool UnregisterFunction(string functionId)
```

**返回值：** 成功返回 true，函数不存在返回 false

### Dispose

释放资源。

```csharp
public void Dispose()
```

## 完整示例

```csharp
using var client = new CroupierClient(new ClientConfig {
    AgentAddr = "127.0.0.1:19090",
    ServiceId = "my-service",
    GameId = "my-game"
});

// 注册函数
client.RegisterFunction(new FunctionDescriptor {
    Id = "echo",
    Version = "1.0.0",
    Category = "test",
    Risk = "low"
}, async (context, payload) => {
    return payload;
});

// 连接
await client.ConnectAsync();

// 启动服务（阻塞）
await client.ServeAsync();

// 自动释放（using 块结束时）
```
