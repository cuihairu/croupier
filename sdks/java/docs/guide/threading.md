# 主线程调度器

主线程调度器（MainThreadDispatcher）用于确保 gRPC 回调在指定线程执行，避免并发问题。

## 使用场景

- **gRPC 回调线程安全** - 网络回调可能在后台线程执行，通过调度器统一到主线程处理
- **控制执行时机** - 在主循环中批量处理回调，避免回调分散执行
- **防止阻塞** - 限流处理，避免大量回调堆积导致阻塞

## 基本用法

```java
import io.github.cuihairu.croupier.sdk.threading.MainThreadDispatcher;

public class Server {
    public static void main(String[] args) {
        // 初始化（在主线程调用一次）
        MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
        dispatcher.initialize();

        // 从任意线程入队回调
        new Thread(() -> {
            dispatcher.enqueue(() -> processResponse(data));
        }).start();

        // 主循环中处理队列
        while (running) {
            dispatcher.processQueue();
            // ... 业务逻辑
        }
    }
}
```

## API 参考

### `MainThreadDispatcher.getInstance()`

获取单例实例。

```java
MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
```

### `initialize()`

初始化调度器，记录当前线程为主线程。必须在主线程调用一次。

```java
dispatcher.initialize();
```

### `isInitialized()`

检查调度器是否已初始化。

```java
if (dispatcher.isInitialized()) {
    // 已初始化
}
```

### `enqueue(Runnable callback)`

将回调加入队列。如果当前在主线程且已初始化，立即执行。

```java
dispatcher.enqueue(() -> {
    System.out.println("在主线程执行");
});
```

### `enqueue(Consumer<T> callback, T data)`

将带参数的回调加入队列。

```java
dispatcher.enqueue(msg -> {
    System.out.println(msg);
}, "Hello");
```

### `processQueue()`

处理队列中的回调，返回处理的数量。

```java
int processed = dispatcher.processQueue();
```

### `processQueue(int maxCount)`

限量处理队列中的回调。

```java
int processed = dispatcher.processQueue(100);
```

### `getPendingCount()`

获取队列中待处理的回调数量。

```java
int count = dispatcher.getPendingCount();
```

### `isMainThread()`

检查当前是否在主线程。

```java
if (dispatcher.isMainThread()) {
    // 在主线程
}
```

### `setMaxProcessPerFrame(int max)`

设置每次 `processQueue()` 最多处理的回调数量。

```java
dispatcher.setMaxProcessPerFrame(500);
```

### `clear()`

清空队列中所有待处理的回调。

```java
dispatcher.clear();
```

## 服务器集成示例

### 基础服务器

```java
import io.github.cuihairu.croupier.sdk.threading.MainThreadDispatcher;
import java.util.concurrent.atomic.AtomicBoolean;

public class Server {
    private static final AtomicBoolean running = new AtomicBoolean(true);

    public static void main(String[] args) {
        MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
        dispatcher.initialize();

        // 信号处理
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            running.set(false);
        }));

        // 主循环
        while (running.get()) {
            dispatcher.processQueue();
            try {
                Thread.sleep(16); // ~60fps
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                break;
            }
        }
    }
}
```

### 与 Spring Boot 集成

```java
import io.github.cuihairu.croupier.sdk.threading.MainThreadDispatcher;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import javax.annotation.PostConstruct;

@Component
public class DispatcherProcessor {

    @PostConstruct
    public void init() {
        MainThreadDispatcher.getInstance().initialize();
    }

    @Scheduled(fixedRate = 16) // ~60fps
    public void processQueue() {
        MainThreadDispatcher.getInstance().processQueue();
    }
}
```

### 与 gRPC 服务集成

```java
// gRPC 回调中
public void onResponse(Response response) {
    MainThreadDispatcher.getInstance().enqueue(() -> {
        // 在主线程处理响应
        handleResponse(response);
    });
}
```

## 线程安全

- `enqueue()` 是线程安全的，可从任意线程调用
- `processQueue()` 应只在主线程调用
- 回调执行时的异常会被捕获并记录，不会中断队列处理
- 使用 `ConcurrentLinkedQueue` 实现无锁队列
