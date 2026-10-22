# 主线程 Dispatcher

主线程 dispatcher（`MainThreadDispatcher`）控制回调的执行时机，支持批处理与限流。

## 使用场景

- 在主循环中控制执行时机、批量处理回调
- 防止大量回调积压阻塞 event loop
- 与其他语言 SDK 保持一致的抽象

## 基本用法

```typescript
import { getDispatcher } from "croupier-js-sdk/threading";

const dispatcher = getDispatcher();
dispatcher.initialize();
dispatcher.enqueue(() => processResponse(data));

function mainLoop() {
  dispatcher.processQueue();
  setImmediate(mainLoop);
}
```

## 常用接口

- `initialize()`
- `enqueue(callback)`
- `enqueueDeferred(callback)`
- `processQueue()`
- `processQueueWithLimit(maxCount)`
- `getPendingCount()`
- `setMaxProcessPerFrame(max)`
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
