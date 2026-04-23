# 主线程调度器

主线程调度器（MainThreadDispatcher）用于确保 gRPC 回调在指定线程执行，避免并发问题。

## 使用场景

- **gRPC 回调线程安全** - 网络回调可能在后台线程执行，通过调度器统一到主线程处理
- **控制执行时机** - 在主循环中批量处理回调，避免回调分散执行
- **防止阻塞** - 限流处理，避免大量回调堆积导致阻塞

## 基本用法

```cpp
#include <croupier/sdk/threading/main_thread_dispatcher.h>

int main() {
    // 初始化（在主线程调用一次）
    auto& dispatcher = croupier::sdk::MainThreadDispatcher::GetInstance();
    dispatcher.Initialize();

    // 从任意线程入队回调
    std::thread([&dispatcher]() {
        dispatcher.Enqueue([]() {
            ProcessResponse(data);
        });
    }).detach();

    // 主循环中处理队列
    while (running) {
        dispatcher.ProcessQueue();
        // ... 业务逻辑
    }
}
```

## API 参考

### `MainThreadDispatcher::GetInstance()`

获取单例实例。

```cpp
auto& dispatcher = croupier::sdk::MainThreadDispatcher::GetInstance();
```

### `Initialize()`

初始化调度器，记录当前线程为主线程。必须在主线程调用一次。

```cpp
dispatcher.Initialize();
```

### `IsInitialized()`

检查调度器是否已初始化。

```cpp
if (dispatcher.IsInitialized()) {
    // 已初始化
}
```

### `Enqueue(std::function<void()> callback)`

将回调加入队列。如果当前在主线程且已初始化，立即执行。

```cpp
dispatcher.Enqueue([]() {
    std::cout << "在主线程执行" << std::endl;
});
```

### `EnqueueWithData<T>(std::function<void(T)> callback, T data)`

将带参数的回调加入队列。

```cpp
dispatcher.EnqueueWithData<std::string>([](const std::string& msg) {
    std::cout << msg << std::endl;
}, "Hello");
```

### `ProcessQueue(int max_count = INT_MAX)`

处理队列中的回调，返回处理的数量。

```cpp
int processed = dispatcher.ProcessQueue();
// 或限量处理
int processed = dispatcher.ProcessQueue(100);
```

### `GetPendingCount()`

获取队列中待处理的回调数量。

```cpp
int count = dispatcher.GetPendingCount();
```

### `IsMainThread()`

检查当前是否在主线程。

```cpp
if (dispatcher.IsMainThread()) {
    // 在主线程
}
```

### `SetMaxProcessPerFrame(int max)`

设置每次 `ProcessQueue()` 最多处理的回调数量。

```cpp
dispatcher.SetMaxProcessPerFrame(500);
```

### `Clear()`

清空队列中所有待处理的回调。

```cpp
dispatcher.Clear();
```

## 服务器集成示例

### 基础服务器

```cpp
#include <croupier/sdk/threading/main_thread_dispatcher.h>
#include <atomic>
#include <chrono>
#include <csignal>
#include <thread>

std::atomic<bool> running{true};

void SignalHandler(int) {
    running = false;
}

int main() {
    auto& dispatcher = croupier::sdk::MainThreadDispatcher::GetInstance();
    dispatcher.Initialize();

    // 信号处理
    std::signal(SIGINT, SignalHandler);
    std::signal(SIGTERM, SignalHandler);

    // 主循环
    while (running) {
        dispatcher.ProcessQueue();
        std::this_thread::sleep_for(std::chrono::milliseconds(16)); // ~60fps
    }
}
```

### 与 gRPC 服务集成

```cpp
// gRPC 回调中
void OnResponse(const Response& response) {
    auto& dispatcher = croupier::sdk::MainThreadDispatcher::GetInstance();
    dispatcher.Enqueue([response]() {
        // 在主线程处理响应
        HandleResponse(response);
    });
}
```

### 带数据的回调

```cpp
struct ResponseData {
    int id;
    std::string message;
};

void OnResponse(const Response& response) {
    auto& dispatcher = croupier::sdk::MainThreadDispatcher::GetInstance();

    ResponseData data{response.id(), response.message()};
    dispatcher.EnqueueWithData<ResponseData>([](const ResponseData& d) {
        HandleResponse(d);
    }, data);
}
```

## 头文件

该实现为 header-only，只需包含头文件即可使用：

```cpp
#include <croupier/sdk/threading/main_thread_dispatcher.h>
```

## 线程安全

- `Enqueue()` 是线程安全的，可从任意线程调用
- `ProcessQueue()` 应只在主线程调用
- 回调执行时的异常会被捕获，不会中断队列处理
- 使用 `std::mutex` 保护队列操作
- 使用 `std::atomic` 保护初始化状态
