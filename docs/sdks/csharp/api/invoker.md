# Invoker API 参考

`CroupierInvoker` 是独立调用方，只访问 Server HTTP API；它不复用 `CroupierClient` 的 Provider TCP session 或配置。鉴权、`gameId/env` 作用域、审计与任务持久化均由 Server 执行。

## 常见能力

- `InvokeAsync`
- `BatchInvokeAsync`
- `StartTaskAsync`
- `CancelTaskAsync`
- `GetTaskStatusAsync`
