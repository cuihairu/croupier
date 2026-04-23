# 主线程调度器

主线程调度器（MainThreadDispatcher）用于确保 gRPC 回调在指定线程执行，避免并发问题。

## 使用场景

- **gRPC 回调线程安全** - 网络回调可能在后台线程执行，通过调度器统一到主线程处理
- **控制执行时机** - 在主循环中批量处理回调，避免回调分散执行
- **防止阻塞** - 限流处理，避免大量回调堆积导致阻塞

## 基本用法

```csharp
using Croupier.Sdk.Threading;

// 初始化（在主线程调用一次）
MainThreadDispatcher.Instance.Initialize();

// 从任意线程入队回调
Task.Run(() => {
    MainThreadDispatcher.Instance.Enqueue(() => ProcessResponse(data));
});

// 主循环中处理队列
while (running)
{
    MainThreadDispatcher.Instance.ProcessQueue();
    // ... 业务逻辑
}
```

## API 参考

### `MainThreadDispatcher.Instance`

获取单例实例。

```csharp
var dispatcher = MainThreadDispatcher.Instance;
```

### `Initialize()`

初始化调度器，记录当前线程为主线程。必须在主线程调用一次。

```csharp
MainThreadDispatcher.Instance.Initialize();
```

### `IsInitialized`

检查调度器是否已初始化。

```csharp
if (MainThreadDispatcher.Instance.IsInitialized)
{
    // 已初始化
}
```

### `Enqueue(Action callback)`

将回调加入队列。如果当前在主线程且已初始化，立即执行。

```csharp
MainThreadDispatcher.Instance.Enqueue(() => {
    Console.WriteLine("在主线程执行");
});
```

### `Enqueue<T>(Action<T> callback, T data)`

将带参数的回调加入队列。

```csharp
MainThreadDispatcher.Instance.Enqueue<string>(msg => {
    Console.WriteLine(msg);
}, "Hello");
```

### `ProcessQueue(int maxCount = int.MaxValue)`

处理队列中的回调，返回处理的数量。

```csharp
int processed = MainThreadDispatcher.Instance.ProcessQueue();
// 或限量处理
int processed = MainThreadDispatcher.Instance.ProcessQueue(100);
```

### `PendingCount`

获取队列中待处理的回调数量。

```csharp
int count = MainThreadDispatcher.Instance.PendingCount;
```

### `IsMainThread`

检查当前是否在主线程。

```csharp
if (MainThreadDispatcher.Instance.IsMainThread)
{
    // 在主线程
}
```

### `SetMaxProcessPerFrame(int max)`

设置每次 `ProcessQueue()` 最多处理的回调数量。

```csharp
MainThreadDispatcher.Instance.SetMaxProcessPerFrame(500);
```

### `Clear()`

清空队列中所有待处理的回调。

```csharp
MainThreadDispatcher.Instance.Clear();
```

## 服务器集成示例

### 基础服务器

```csharp
using Croupier.Sdk.Threading;

class Program
{
    private static volatile bool _running = true;

    static void Main(string[] args)
    {
        MainThreadDispatcher.Instance.Initialize();

        // 信号处理
        Console.CancelKeyPress += (s, e) => {
            e.Cancel = true;
            _running = false;
        };

        // 主循环
        while (_running)
        {
            MainThreadDispatcher.Instance.ProcessQueue();
            Thread.Sleep(16); // ~60fps
        }
    }
}
```

### 与 ASP.NET Core 集成

```csharp
using Croupier.Sdk.Threading;

public class DispatcherHostedService : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        MainThreadDispatcher.Instance.Initialize();

        while (!stoppingToken.IsCancellationRequested)
        {
            MainThreadDispatcher.Instance.ProcessQueue();
            await Task.Delay(16, stoppingToken); // ~60fps
        }
    }
}

// 在 Program.cs 中注册
builder.Services.AddHostedService<DispatcherHostedService>();
```

### 与 gRPC 服务集成

```csharp
// gRPC 回调中
public void OnResponse(Response response)
{
    MainThreadDispatcher.Instance.Enqueue(() => {
        // 在主线程处理响应
        HandleResponse(response);
    });
}
```

## 扩展方法

SDK 提供了 CroupierClient 的扩展方法：

```csharp
using Croupier.Sdk.Extensions;

// 在主线程执行
client.InvokeOnMainThread(() => {
    // 业务逻辑
});

// 带数据执行
client.InvokeOnMainThread<MyData>(data => {
    // 处理数据
}, myData);
```

## 线程安全

- `Enqueue()` 是线程安全的，可从任意线程调用
- `ProcessQueue()` 应只在主线程调用
- 回调执行时的异常会被捕获并记录，不会中断队列处理
- 使用 `ConcurrentQueue<Action>` 实现无锁队列
