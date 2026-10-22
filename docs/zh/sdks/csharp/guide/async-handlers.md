# 异步 Handler

C# SDK 优先使用 `async/await` 风格的异步 handler。

## 建议

- 所有外部 I/O 使用异步 API
- handler 内不做同步阻塞
- 超时与取消 token 一路透传到下游调用
