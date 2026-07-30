# 主线程调度器

主线程调度器用于确保回调在受控线程执行，减少并发状态问题。

## 基本用法

```python
from croupier.dispatcher import MainThreadDispatcher

dispatcher = MainThreadDispatcher.get_instance()
dispatcher.initialize()

def on_response(data):
    dispatcher.enqueue(lambda: process_response(data))

while running:
    dispatcher.process_queue()
```

## 常用接口

- `initialize()`
- `enqueue(callback)`
- `enqueue_with_data(callback, data)`
- `process_queue(max_count=None)`
- `get_pending_count()`
- `is_main_thread()`
- `set_max_process_per_frame(max_count)`
- `clear()`
