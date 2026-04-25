# 主线程调度器

主线程调度器（`MainThreadDispatcher`）用于控制回调执行时机，实现批量处理和限流。

## 使用场景

- 控制执行时机，在主循环中批量处理回调
- 防止大量回调堆积导致事件循环阻塞
- 与其他语言 SDK 保持一致的抽象接口

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
