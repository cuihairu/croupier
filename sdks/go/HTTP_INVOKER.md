# Go L3 Invoker

`NewInvoker` 是 Go SDK 面向调用方的 L3 入口。它只调用 Croupier Server 的 HTTP API，不会直连
Provider TCP；因此鉴权、`game/env` 作用域、审计、路由和任务持久化均由 Server 统一执行。

Provider 注册函数仍使用 SDK 与 Agent 之间的 TCP session。这是独立于 L3 调用方链路的职责，不能把
Provider TCP Invoker 当作生产调用方 API 使用。

## 创建 Invoker

```go
invoker := croupier.NewInvoker(&croupier.InvokerConfig{
    // 可传 Server 根地址、完整 API 地址或 host:port。
    Address:   "https://server.example/api/v1",
    AuthToken: "server-access-token", // 发送为 Authorization: Bearer ...
    GameID:    "game-a",              // 默认 X-Game-ID
    Env:       "production",          // 默认 X-Env
})
defer invoker.Close()

if err := invoker.Connect(context.Background()); err != nil {
    log.Fatal(err)
}
```

默认地址是 `http://127.0.0.1:18780/api/v1`。根地址和 `host:port` 会自动补全 `/api/v1`。

`Connect` 仅标记 Invoker 就绪：HTTP 是请求式传输，不建立或绕过 Server 的持久 Provider 会话。

## 同步调用

```go
result, err := invoker.Invoke(ctx, "player.ban", `{"playerId":"p-1"}`, croupier.InvokeOptions{
    IdempotencyKey: "ban-p-1-20260817",
    Headers: map[string]string{
        "X-Game-ID": "game-a",
        "X-Env":     "production",
    },
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(result) // Server 返回的 result 原始 JSON
```

调用会发送 `POST /api/v1/functions/:id/invoke`，请求体为 `{"params": ...}`。可使用 `SetSchema`
配置可选的本地 JSON Schema 预检；Server 的合同和权限校验始终是最终依据。

## 异步任务

```go
taskID, err := invoker.StartTask(ctx, "report.generate", `{"range":"daily"}`, options)
if err != nil {
    log.Fatal(err)
}

status, err := invoker.GetTaskStatus(ctx, taskID)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("%s: %s (%d%%)\n", status.TaskID, status.Status, status.Progress)

events, err := invoker.StreamTask(ctx, taskID)
if err != nil {
    log.Fatal(err)
}
for event := range events {
    fmt.Printf("%s %s\n", event.EventType, event.Payload)
}

if err := invoker.CancelTask(ctx, taskID); err != nil {
    log.Fatal(err)
}
```

任务合同如下：

| SDK 方法        | Server HTTP 合同                                            |
| --------------- | ----------------------------------------------------------- |
| `StartTask`     | `POST /api/v1/tasks`，由 Server 返回 `taskId`               |
| `GetTaskStatus` | `GET /api/v1/tasks/:id`                                     |
| `StreamTask`    | 轮询 `GET /api/v1/tasks/:id/events?after_seq=N` 直至 `done` |
| `CancelTask`    | `POST /api/v1/tasks/:id/cancel`                             |

`StartTask` 绝不会构造本地任务 ID。`GetTaskStatus` 的 `Result` 是 Server 返回的原始 JSON，以保持业务结果的形状。

## 请求头、超时与重试

- `AuthToken` 自动映射为 `Authorization: Bearer <token>`；`InvokeOptions.Headers["Authorization"]` 可显式覆盖。
- `GameID`、`Env` 设定默认的 `X-Game-ID`、`X-Env`；`InvokeOptions.Headers` 可逐请求覆盖。
- `IdempotencyKey` 映射为 `Idempotency-Key` 请求头。
- `TimeoutSeconds` 为客户端默认请求超时；`InvokeOptions.Timeout` 可覆盖单次调用。
- `Retry` 只重试网络错误、429 和 5xx；不要对无幂等键的写操作依赖重试。

## 验证

Mock 合同测试不依赖运行中的 Server：

```bash
cd sdks/go
go test ./pkg/croupier -count=1
```

真实 Server 验收需要有授权的测试环境，并设置 `CROUPIER_SERVER_URL`、`CROUPIER_SERVER_TOKEN`、
`CROUPIER_GAME_ID` 与 `CROUPIER_ENV`。这不是本地 Mock 测试的替代品。
