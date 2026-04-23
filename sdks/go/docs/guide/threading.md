# 主线程调度器

主线程调度器（MainThreadDispatcher）用于确保 gRPC 回调在指定线程执行，避免并发问题。

## 使用场景

- **gRPC 回调线程安全** - 网络回调可能在后台线程执行，通过调度器统一到主线程处理
- **控制执行时机** - 在主循环中批量处理回调，避免回调分散执行
- **防止阻塞** - 限流处理，避免大量回调堆积导致阻塞

## 基本用法

```go
package main

import (
    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    // 初始化（在主 goroutine 调用一次）
    dispatcher := croupier.GetDispatcher()
    dispatcher.Initialize()

    // 从任意 goroutine 入队回调
    go func() {
        dispatcher.Enqueue(func() {
            processResponse(data)
        })
    }()

    // 主循环中处理队列
    running := true
    for running {
        dispatcher.ProcessQueue()
        // ... 业务逻辑
    }
}
```

## API 参考

### `GetDispatcher()`

获取单例实例。

```go
dispatcher := croupier.GetDispatcher()
```

### `Initialize()`

初始化调度器，记录当前 goroutine 为主 goroutine。必须在主 goroutine 调用一次。

```go
dispatcher.Initialize()
```

### `Enqueue(callback func())`

将回调加入队列。如果当前在主 goroutine 且已初始化，立即执行。

```go
dispatcher.Enqueue(func() {
    fmt.Println("在主 goroutine 执行")
})
```

### `EnqueueWithData[T any](d *MainThreadDispatcher, callback func(T), data T)`

将带参数的回调加入队列。

```go
croupier.EnqueueWithData(dispatcher, func(msg string) {
    fmt.Println(msg)
}, "Hello")
```

### `ProcessQueue() int`

处理队列中的回调，返回处理的数量。

```go
processed := dispatcher.ProcessQueue()
```

### `ProcessQueueWithLimit(maxCount int) int`

限量处理队列中的回调。

```go
processed := dispatcher.ProcessQueueWithLimit(100)
```

### `GetPendingCount() int`

获取队列中待处理的回调数量。

```go
count := dispatcher.GetPendingCount()
```

### `IsMainGoroutine() bool`

检查当前是否在主 goroutine。

```go
if dispatcher.IsMainGoroutine() {
    // 在主 goroutine
}
```

### `SetMaxProcessPerFrame(max int)`

设置每次 `ProcessQueue()` 最多处理的回调数量。

```go
dispatcher.SetMaxProcessPerFrame(500)
```

### `Clear()`

清空队列中所有待处理的回调。

```go
dispatcher.Clear()
```

## 服务器集成示例

### 基础服务器

```go
package main

import (
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    dispatcher := croupier.GetDispatcher()
    dispatcher.Initialize()

    // 信号处理
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    running := true
    go func() {
        <-sigCh
        running = false
    }()

    // 主循环
    ticker := time.NewTicker(16 * time.Millisecond) // ~60fps
    defer ticker.Stop()

    for running {
        select {
        case <-ticker.C:
            dispatcher.ProcessQueue()
            // ... 业务逻辑
        }
    }
}
```

### 与 gRPC 服务集成

```go
// gRPC 回调中
func (s *Server) OnResponse(ctx context.Context, resp *Response) {
    dispatcher := croupier.GetDispatcher()
    dispatcher.Enqueue(func() {
        // 在主 goroutine 处理响应
        s.handleResponse(resp)
    })
}
```

## 线程安全

- `Enqueue()` 是 goroutine 安全的，可从任意 goroutine 调用
- `ProcessQueue()` 应只在主 goroutine 调用
- 回调执行时的 panic 会被 recover，不会中断队列处理
- 使用 `sync.Mutex` 保护队列操作
