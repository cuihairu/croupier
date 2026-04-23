# 主线程调度器

主线程调度器（MainThreadDispatcher）用于控制回调执行时机，实现批量处理和限流。

## 使用场景

JavaScript 是单线程的，但调度器在以下场景仍然有用：

- **控制执行时机** - 在主循环中批量处理回调，而非立即执行
- **防止阻塞** - 限流处理，避免大量回调堆积导致事件循环阻塞
- **统一 API** - 与其他语言 SDK 保持一致的接口

## 基本用法

```typescript
import { MainThreadDispatcher, getDispatcher } from "croupier-js-sdk/threading";

// 初始化
const dispatcher = getDispatcher();
dispatcher.initialize();

// 入队回调（默认立即执行）
dispatcher.enqueue(() => processResponse(data));

// 延迟执行（加入队列，等待 processQueue 处理）
dispatcher.enqueueDeferred(() => processResponse(data));

// 主循环中处理队列
function mainLoop() {
  dispatcher.processQueue();
  // ... 业务逻辑
  setImmediate(mainLoop);
}
```

## API 参考

### `MainThreadDispatcher.getInstance()`

获取单例实例。

```typescript
const dispatcher = MainThreadDispatcher.getInstance();
```

### `getDispatcher()`

便捷函数，获取单例实例。

```typescript
import { getDispatcher } from "croupier-js-sdk/threading";

const dispatcher = getDispatcher();
```

### `initialize()`

初始化调度器。应在启动时调用一次。

```typescript
dispatcher.initialize();
```

### `isInitialized()`

检查调度器是否已初始化。

```typescript
if (dispatcher.isInitialized()) {
  // 已初始化
}
```

### `enqueue(callback, executeImmediatelyIfInitialized?)`

将回调加入队列。

**参数:**

- `callback` - 要执行的回调函数
- `executeImmediatelyIfInitialized` - 如果为 true（默认），已初始化时立即执行

```typescript
// 立即执行（默认）
dispatcher.enqueue(() => console.log("立即执行"));

// 加入队列
dispatcher.enqueue(() => console.log("延迟执行"), false);
```

### `enqueueDeferred(callback)`

将回调加入队列，永不立即执行。

```typescript
dispatcher.enqueueDeferred(() => console.log("延迟执行"));
```

### `enqueueWithData<T>(callback, data)`

将带参数的回调加入队列。

```typescript
dispatcher.enqueueWithData<string>((msg) => {
  console.log(msg);
}, "Hello");
```

### `processQueue()`

处理队列中的回调，返回处理的数量。

```typescript
const processed = dispatcher.processQueue();
```

### `processQueueWithLimit(maxCount)`

限量处理队列中的回调。

```typescript
const processed = dispatcher.processQueueWithLimit(100);
```

### `getPendingCount()`

获取队列中待处理的回调数量。

```typescript
const count = dispatcher.getPendingCount();
```

### `isMainThread()`

在 JavaScript 中，这返回 `isInitialized()` 的结果（API 兼容性）。

```typescript
if (dispatcher.isMainThread()) {
  // 已初始化
}
```

### `setMaxProcessPerFrame(max)`

设置每次 `processQueue()` 最多处理的回调数量。

```typescript
dispatcher.setMaxProcessPerFrame(500);
```

### `clear()`

清空队列中所有待处理的回调。

```typescript
dispatcher.clear();
```

## 便捷函数

```typescript
import { enqueue, processQueue } from "croupier-js-sdk/threading";

// 入队回调
enqueue(() => console.log("Hello"));

// 处理队列
const count = processQueue();
```

## 服务器集成示例

### 基础 Node.js 服务器

```typescript
import { getDispatcher } from "croupier-js-sdk/threading";

const dispatcher = getDispatcher();
dispatcher.initialize();

let running = true;

process.on("SIGINT", () => {
  running = false;
});

// 主循环
function mainLoop() {
  if (!running) return;

  dispatcher.processQueue();
  // ... 业务逻辑

  setImmediate(mainLoop);
}

mainLoop();
```

### 定时器模式

```typescript
import { getDispatcher } from "croupier-js-sdk/threading";

const dispatcher = getDispatcher();
dispatcher.initialize();

// 每帧处理
setInterval(() => {
  dispatcher.processQueue();
}, 16); // ~60fps
```

### 与 gRPC 回调集成

```typescript
// gRPC 回调中
function onResponse(response: Response) {
  const dispatcher = getDispatcher();

  // 使用 enqueueDeferred 确保在主循环中处理
  dispatcher.enqueueDeferred(() => {
    handleResponse(response);
  });
}
```

### 批量处理模式

```typescript
import { getDispatcher } from "croupier-js-sdk/threading";

const dispatcher = getDispatcher();
dispatcher.initialize();
dispatcher.setMaxProcessPerFrame(100); // 每帧最多处理 100 个

// 大量回调入队
for (let i = 0; i < 1000; i++) {
  dispatcher.enqueueDeferred(() => processItem(i));
}

// 分批处理，避免阻塞
function processInBatches() {
  const processed = dispatcher.processQueue();
  if (dispatcher.getPendingCount() > 0) {
    setImmediate(processInBatches);
  }
}

processInBatches();
```

## 注意事项

- JavaScript 是单线程的，`enqueue()` 默认立即执行
- 使用 `enqueueDeferred()` 或 `enqueue(callback, false)` 来延迟执行
- 回调执行时的异常会被捕获并记录到 console.error，不会中断队列处理
