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

## 入站并发与背压

入站请求（Agent→SDK 的 Invoke 等）由**有界 worker 池**处理：
读循环只投递不执行业务（串行处理会让一个慢 handler 卡住整条连接）。

- 默认 worker 数 ≈ CPU 核数；待处理队列容量 ≈ `workers × 4`（突发吸收 4 轮满载）
- 业务队列打满时**立即回 busy 错误帧**（`inbound queue full, retry on
another instance`），Agent 侧 failover 换实例重试——SDK 内存不积累
- 调优入口在 transport 层配置（Go：`InboundWorkers`/`InboundQLen`；
  其他语言见各自配置章节），语义六语言一致

控制消息（心跳/注册/drain）的**双车道隔离**目前仅 Go 落地：控制请求走
专用队列永不 reject，业务洪峰时会话仍存活（`protocol.IsControlRequest()`
分类）。其余语言心跳与业务共用车道，待按 Go 基准迁移。
