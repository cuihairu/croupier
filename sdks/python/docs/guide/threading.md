# 主线程调度器

主线程调度器（MainThreadDispatcher）用于确保 gRPC 回调在指定线程执行，避免并发问题。

## 使用场景

- **gRPC 回调线程安全** - 网络回调可能在后台线程执行，通过调度器统一到主线程处理
- **控制执行时机** - 在主循环中批量处理回调，避免回调分散执行
- **防止阻塞** - 限流处理，避免大量回调堆积导致阻塞

## 基本用法

```python
from croupier.dispatcher import MainThreadDispatcher

# 初始化（在主线程调用一次）
dispatcher = MainThreadDispatcher.get_instance()
dispatcher.initialize()

# 从任意线程入队回调
def on_grpc_response(data):
    dispatcher.enqueue(lambda: process_response(data))

# 主循环中处理队列
while running:
    processed = dispatcher.process_queue()
    # ... 业务逻辑
```

## API 参考

### `MainThreadDispatcher.get_instance()`

获取单例实例。

### `initialize()`

初始化调度器，记录当前线程为主线程。必须在主线程调用一次。

### `enqueue(callback)`

将回调加入队列。如果当前在主线程且已初始化，立即执行。

**参数:**
- `callback` - 无参数的可调用对象

### `enqueue_with_data(callback, data)`

将带参数的回调加入队列。

**参数:**
- `callback` - 接受一个参数的可调用对象
- `data` - 传递给回调的数据

### `process_queue(max_count=None)`

处理队列中的回调。

**参数:**
- `max_count` - 最多处理的回调数量，默认使用 `max_process_per_frame` 设置

**返回:**
- 实际处理的回调数量

### `get_pending_count()`

获取队列中待处理的回调数量。

### `is_main_thread()`

检查当前是否在主线程。

### `set_max_process_per_frame(max_count)`

设置每次 `process_queue()` 最多处理的回调数量。

**参数:**
- `max_count` - 最大数量，0 表示使用默认值 (1000)

### `clear()`

清空队列中所有待处理的回调。

## 便捷函数

```python
from croupier.dispatcher import get_dispatcher, enqueue, process

# 获取实例
dispatcher = get_dispatcher()

# 入队回调
enqueue(lambda: print("Hello"))

# 处理队列
count = process()
```

## 服务器集成示例

### 基础服务器

```python
import signal
from croupier.dispatcher import MainThreadDispatcher

running = True
dispatcher = MainThreadDispatcher.get_instance()
dispatcher.initialize()

def signal_handler(sig, frame):
    global running
    running = False

signal.signal(signal.SIGINT, signal_handler)

# 主循环
while running:
    dispatcher.process_queue()
    time.sleep(0.016)  # ~60fps
```

### 与 asyncio 集成

```python
import asyncio
from croupier.dispatcher import MainThreadDispatcher

dispatcher = MainThreadDispatcher.get_instance()
dispatcher.initialize()

async def main_loop():
    while True:
        dispatcher.process_queue()
        await asyncio.sleep(0.016)

asyncio.run(main_loop())
```

## 线程安全

- `enqueue()` 和 `enqueue_with_data()` 是线程安全的，可从任意线程调用
- `process_queue()` 应只在主线程调用
- 回调执行时的异常会被捕获并记录，不会中断队列处理
