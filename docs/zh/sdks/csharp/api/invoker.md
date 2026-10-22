# Invoker API 参考

`CroupierInvoker` 是独立调用方，只与 Server HTTP API 通信；不复用 `CroupierClient` 的 Provider TCP session 及其配置。认证、`gameId/env` 作用域、审计与 task 持久化均由 Server 完成。

## 典型能力

- `InvokeAsync`
- `BatchInvokeAsync`
- `StartTaskAsync`
- `CancelTaskAsync`
- `GetTaskStatusAsync`
