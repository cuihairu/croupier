# HTTP Invoker 使用指南

## 背景

Croupier server 已从 **gRPC** 迁移到 **TCP** (Nanomsg v3) 协议。原有的 `NewInvoker` 使用 gRPC 协议无法连接到新的 server。

## 解决方案

我们提供了 **HTTP REST API Invoker** (`NewHTTPInvoker`) 作为替代方案。

## 使用方法

### 1. 导入包

```go
import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
```

### 2. 创建 HTTP Invoker

```go
invokerConfig := &croupier.InvokerConfig{
    Address:        "localhost:18780", // HTTP REST API 端口
    TimeoutSeconds: 30,
    Insecure:       true,
}

invoker := croupier.NewHTTPInvoker(invokerConfig)
defer invoker.Close()
```

### 3. 连接到 Server

```go
ctx := context.Background()
if err := invoker.Connect(ctx); err != nil {
    log.Fatalf("Failed to connect: %v", err)
}
```

### 4. 调用函数

```go
payload := map[string]interface{}{
    "player_id": "player_123",
    "reason":    "违规操作",
}
payloadJSON, _ := json.Marshal(payload)

options := croupier.InvokeOptions{
    IdempotencyKey: "unique-key-123",
    Headers: map[string]string{
        "X-Game-ID": "your-game-id",
        "X-Env":     "production",
    },
}

result, err := invoker.Invoke(ctx, "player.ban", string(payloadJSON), options)
if err != nil {
    log.Fatalf("Invoke failed: %v", err)
}

fmt.Printf("Result: %s\n", result)
```

## 完整示例

参见 `examples/http_invoker_example/main.go`。

## 功能支持情况

| 功能 | HTTP Invoker | gRPC Invoker |
|------|--------------|--------------|
| ✅ 同步调用 (Invoke) | 支持 | 支持 |
| ⚠️  异步任务 (StartTask) | 部分支持 | 完全支持 |
| ❌ 任务流式传输 (StreamTask) | 不支持 | 支持 |
| ❌ 取消任务 (CancelTask) | 不支持 | 支持 |
| ✅ Schema 验证 (SetSchema) | 支持 | 支持 |

**注意：** 如需完整功能（异步作业、流式传输等），请继续使用 gRPC invoker 并连接到支持 gRPC 的 server。

## 端口映射

| 用途 | 协议 | 端口 |
|------|------|------|
| HTTP REST API | HTTP | 18780 |
| TCP ControlService | TCP (SP) | 19090 |
| 旧 gRPC 服务 | gRPC | 18443 (已废弃) |

## 迁移指南

如果您之前使用 `NewInvoker`，只需做以下更改：

1. **更改创建函数：**
   ```go
   // 旧代码 (gRPC)
   invoker := croupier.NewInvoker(config)

   // 新代码 (HTTP)
   invoker := croupier.NewHTTPInvoker(config)
   ```

2. **更新地址：**
   ```go
   config := &croupier.InvokerConfig{
       Address: "localhost:18780", // HTTP 而非 gRPC
       // ... 其他配置
   }
   ```

3. **处理异步功能：**
   - 如果使用了 `StreamTask` 或 `CancelTask`，需要添加错误处理
   - 或者继续使用 gRPC invoker（需要 server 支持）

## 常见问题

**Q: 为什么不直接更新 Go SDK 使用 TCP？**
A: TCP 的 Go 绑定尚不成熟。HTTP REST API 是更稳定的跨语言解决方案。

**Q: HTTP invoker 性能如何？**
A: 对于大多数用例，HTTP REST API 性能足够。如需更高性能，可考虑使用 gRPC invoker。

**Q: 什么时候会完全支持 HTTP 异步作业？**
A: 我们计划在未来的版本中为 HTTP REST API 添加 SSE (Server-Sent Events) 支持。
