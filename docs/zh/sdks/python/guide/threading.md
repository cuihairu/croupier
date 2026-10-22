# 主线程 Dispatcher

主线程 dispatcher 保证回调在受控线程上执行，降低并发相关的状态问题。

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

## 入站并发与背压

入站请求（Agent→SDK 的 Invoke 等）由**有界 worker pool** 处理：
read loop 只做入队，从不执行业务逻辑（串行处理会让一个慢 handler 拖住整条连接）。

- 默认 worker 数 ≈ CPU 核数；pending 队列容量 ≈ `workers × 4`（约可吸收 4 轮满负荷突发）
- 业务队列打满时，立即返回**忙错误帧**（`inbound queue full, retry on
another instance`），Agent 随即 failover 到其他实例——SDK 内不累积内存
- 调参位于传输层配置（Go：`InboundWorkers`/`InboundQLen`；
  其他语言见各自的配置章节），六语言语义一致

控制消息（heartbeat/registration/drain）的**双车道隔离**目前仅 Go 具备：控制请求走
专用队列、永不拒绝，因此业务流量激增时 session 仍能保活（通过
`protocol.IsControlRequest()` 分类）。其他语言在 heartbeat 与业务流量之间共享单车道，待迁移到 Go 基线。
