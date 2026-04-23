# API 参考

本文档提供 Croupier Go SDK 的完整 API 参考。

## 包结构

```go
import (
    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)
```

## 核心类型

### Handler

函数处理器类型定义。

```go
type Handler func(ctx context.Context, payload string) (string, error)
```

**参数:**
- `ctx`: 上下文，包含调用元数据
- `payload`: 函数参数（JSON 字符串）

**返回值:**
- `string`: 函数执行结果（JSON 字符串）
- `error`: 错误信息，成功时返回 nil

---

### ClientConfig

客户端配置结构。

```go
type ClientConfig struct {
    // 连接配置
    AgentAddr      string // Agent gRPC 地址，默认 "localhost:19090"
    TimeoutSeconds int    // 连接超时（秒），默认 30
    Insecure       bool   // 使用不安全的 gRPC 连接

    // 多租户隔离
    GameID         string // 游戏标识符（必填）
    Env            string // 环境：dev/staging/prod
    ServiceID      string // 服务标识符
    ServiceVersion string // 服务版本
    AgentID        string // Agent 标识符

    // TLS 配置（非 insecure 模式时必填）
    CAFile   string // CA 证书文件路径
    CertFile string // 客户端证书文件路径
    KeyFile  string // 私钥文件路径

    // 重连配置
    AutoReconnect          bool // 是否自动重连，默认 true
    ReconnectIntervalSecs  int  // 重连间隔（秒），默认 5
    ReconnectMaxAttempts   int  // 最大重连次数，0 表示无限

    // 心跳配置
    HeartbeatIntervalSecs int // 心跳间隔（秒），默认 10

    // 重试配置
    MaxRetries     int // 最大重试次数，默认 3
    RetryBackoffMs int // 重试退避时间（毫秒），默认 1000
}
```

**环境变量覆盖:**

| 环境变量 | 配置字段 | 说明 |
|----------|----------|------|
| `CROUPIER_AGENT_ADDR` | AgentAddr | Agent 地址 |
| `CROUPIER_GAME_ID` | GameID | 游戏 ID |
| `CROUPIER_ENV` | Env | 环境 |
| `CROUPIER_SERVICE_ID` | ServiceID | 服务 ID |
| `CROUPIER_INSECURE` | Insecure | 是否跳过 TLS |
| `CROUPIER_CA_FILE` | CAFile | CA 证书路径 |
| `CROUPIER_CERT_FILE` | CertFile | 客户端证书路径 |
| `CROUPIER_KEY_FILE` | KeyFile | 私钥路径 |

---

### FunctionDescriptor

函数描述符，用于向 Agent 描述函数。

```go
type FunctionDescriptor struct {
    // 必填字段
    ID      string // 函数 ID，格式: [namespace.]entity.operation
    Version string // 语义化版本号，如 "1.0.0"

    // 推荐字段
    Category  string // 业务分类，如 "player", "item"
    Risk      string // 风险等级: "low"|"medium"|"high"
    Entity    string // 关联实体类型
    Operation string // 操作类型: "create"|"read"|"update"|"delete"
    Enabled   bool   // 是否启用，默认 true

    // 可选字段
    DisplayName  string   // 显示名称
    Summary      string   // 简短描述
    Description  string   // 详细描述
    Tags         []string // 标签列表
    InputSchema  string   // 输入参数 JSON Schema
    OutputSchema string   // 输出结果 JSON Schema
    TimeoutMs    int      // 函数超时（毫秒）
}
```

**函数 ID 命名规范:**

```go
// ✅ 正确
"player.get"           // entity.operation
"player.ban"           // entity.operation
"game.player.ban"      // namespace.entity.operation
"inventory.item.add"   // namespace.entity.operation

// ❌ 错误
"PlayerGet"            // 不要使用驼峰
"player-get"           // 不要使用连字符
"get_player"           // 实体应在操作前
```

---

### VirtualObjectDescriptor

虚拟对象描述符，将相关的 CRUD 操作组合在一起。

```go
type VirtualObjectDescriptor struct {
    // 必填字段
    ID      string // 对象 ID
    Version string // 版本号

    // 推荐字段
    Name        string // 显示名称
    Description string // 描述

    // 操作映射
    Operations map[string]string // 操作名 -> 函数 ID

    // 可选字段
    Schema       json.RawMessage            // 实体 JSON Schema
    Metadata     map[string]interface{}     // 元数据
    Relationships map[string]Relationship   // 实体关系
}
```

**标准操作映射:**

```go
vo := VirtualObjectDescriptor{
    ID:      "player",
    Version: "1.0.0",
    Operations: map[string]string{
        "create": "player.create",
        "read":   "player.get",
        "update": "player.update",
        "delete": "player.delete",
        "ban":    "player.ban",
        "unban":  "player.unban",
    },
}
```

---

### ComponentDescriptor

组件描述符，用于将相关函数分组。

```go
type ComponentDescriptor struct {
    ID          string   // 组件 ID
    Version     string   // 版本号
    Name        string   // 显示名称
    Description string   // 描述
    Functions   []string // 包含的函数 ID 列表
    Enabled     bool     // 是否启用
}
```

---

## Client 接口

```go
type Client interface {
    // 注册函数
    RegisterFunction(desc FunctionDescriptor, handler Handler) error

    // 注册虚拟对象
    RegisterVirtualObject(desc VirtualObjectDescriptor, handlers map[string]Handler) error

    // 注册组件
    RegisterComponent(comp ComponentDescriptor) error

    // 连接到 Agent
    Connect(ctx context.Context) error

    // 启动服务（阻塞）
    Serve(ctx context.Context) error

    // 停止服务
    Stop() error

    // 关闭连接
    Close() error

    // 检查连接状态
    IsConnected() bool

    // 设置连接状态回调
    OnConnectionStateChange(callback func(connected bool))

    // 设置错误回调
    OnError(callback func(err error))
}
```

---

## Invoker 接口

调用端接口，用于调用已注册的函数。

```go
type Invoker interface {
    // 同步调用函数
    Invoke(ctx context.Context, functionID string, payload string, opts ...InvokeOption) (string, error)

    // 启动异步任务
    StartTask(ctx context.Context, functionID string, payload string, opts ...InvokeOption) (string, error)

    // 流式获取任务事件
    StreamTask(ctx context.Context, taskID string) (<-chan TaskEvent, error)

    // 取消任务
    CancelTask(ctx context.Context, taskID string) error

    // 获取任务结果
    GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
}
```

### InvokeOption

调用选项。

```go
type InvokeOption func(*invokeOptions)

// 设置超时
func WithTimeout(d time.Duration) InvokeOption

// 设置幂等键
func WithIdempotencyKey(key string) InvokeOption

// 设置元数据
func WithMetadata(metadata map[string]string) InvokeOption

// 设置重试次数
func WithRetryCount(count int) InvokeOption
```

**使用示例:**

```go
result, err := invoker.Invoke(ctx, "player.ban",
    `{"player_id": "12345", "reason": "违规"}`,
    WithTimeout(10*time.Second),
    WithIdempotencyKey("ban-12345-20260117"),
    WithMetadata(map[string]string{"operator": "admin"}),
)
```

---

### TaskEvent

任务事件。

```go
type TaskEvent struct {
    Type     string // 事件类型: "progress"|"log"|"done"|"error"
    Message  string // 事件消息
    Progress int    // 进度 0-100（仅 progress 类型）
    Payload  []byte // 最终结果（仅 done 类型）
}
```

### TaskResult

任务结果。

```go
type TaskResult struct {
    TaskID  string     // 任务 ID
    Status  TaskStatus // 任务状态
    Payload []byte     // 结果数据
    Error   string     // 错误信息
}

type TaskStatus int

const (
    TaskStatusPending   TaskStatus = 1
    TaskStatusRunning   TaskStatus = 2
    TaskStatusCompleted TaskStatus = 3
    TaskStatusFailed    TaskStatus = 4
    TaskStatusCancelled TaskStatus = 5
)
```

---

## 创建客户端

### NewClient

创建新的客户端实例。

```go
func NewClient(config *ClientConfig) Client
```

**示例:**

```go
config := &croupier.ClientConfig{
    AgentAddr: "localhost:19090",
    GameID:    "my-game",
    Env:       "development",
    ServiceID: "player-service",
    Insecure:  true,
}

client := croupier.NewClient(config)
```

### NewInvoker

创建新的调用端实例。

```go
func NewInvoker(config *ClientConfig) Invoker
```

**示例:**

```go
config := &croupier.ClientConfig{
    AgentAddr: "localhost:19090",
    GameID:    "my-game",
    Env:       "development",
    Insecure:  true,
}

invoker := croupier.NewInvoker(config)
```

---

## 配置加载

### LoadConfig

从文件加载配置。

```go
func LoadConfig(path string) (*ClientConfig, error)
```

**支持的格式:**
- YAML (`.yaml`, `.yml`)
- JSON (`.json`)

### LoadConfigWithEnv

从文件加载配置并应用环境变量覆盖。

```go
func LoadConfigWithEnv(path string, prefix string) (*ClientConfig, error)
```

**示例:**

```go
// 加载 config.yaml，并用 CROUPIER_ 前缀的环境变量覆盖
config, err := croupier.LoadConfigWithEnv("config.yaml", "CROUPIER_")
```

---

## 错误处理

### 错误类型

```go
var (
    ErrNotConnected     = errors.New("not connected to agent")
    ErrAlreadyConnected = errors.New("already connected")
    ErrInvalidConfig    = errors.New("invalid configuration")
    ErrFunctionNotFound = errors.New("function not found")
    ErrTimeout          = errors.New("operation timeout")
    ErrCancelled        = errors.New("operation cancelled")
)
```

### 错误检查

```go
if errors.Is(err, croupier.ErrNotConnected) {
    // 处理未连接错误
}

if errors.Is(err, croupier.ErrTimeout) {
    // 处理超时错误
}
```

---

## 回调类型

### ConnectionCallback

连接状态变化回调。

```go
type ConnectionCallback func(connected bool)
```

### ErrorCallback

错误回调。

```go
type ErrorCallback func(err error)
```

**使用示例:**

```go
client.OnConnectionStateChange(func(connected bool) {
    if connected {
        log.Println("已连接到 Agent")
    } else {
        log.Println("与 Agent 断开连接")
    }
})

client.OnError(func(err error) {
    log.Printf("发生错误: %v", err)
})
```

---

## 完整示例

### Provider 示例

```go
package main

import (
    "context"
    "encoding/json"
    "log"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    // 配置
    config := &croupier.ClientConfig{
        AgentAddr: "localhost:19090",
        GameID:    "my-game",
        Env:       "production",
        ServiceID: "player-service",
        Insecure:  false,
        CAFile:    "/etc/tls/ca.crt",
        CertFile:  "/etc/tls/client.crt",
        KeyFile:   "/etc/tls/client.key",
    }

    // 创建客户端
    client := croupier.NewClient(config)

    // 设置回调
    client.OnConnectionStateChange(func(connected bool) {
        log.Printf("连接状态: %v", connected)
    })

    // 注册函数
    desc := croupier.FunctionDescriptor{
        ID:        "player.ban",
        Version:   "1.0.0",
        Category:  "player",
        Risk:      "high",
        Entity:    "player",
        Operation: "update",
        Enabled:   true,
    }

    handler := func(ctx context.Context, payload string) (string, error) {
        var req struct {
            PlayerID string `json:"player_id"`
            Reason   string `json:"reason"`
        }
        if err := json.Unmarshal([]byte(payload), &req); err != nil {
            return `{"status":"error","code":"INVALID_JSON"}`, nil
        }

        // 业务逻辑
        log.Printf("封禁玩家: %s, 原因: %s", req.PlayerID, req.Reason)

        return `{"status":"success"}`, nil
    }

    if err := client.RegisterFunction(desc, handler); err != nil {
        log.Fatal(err)
    }

    // 连接并启动服务
    ctx := context.Background()
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }

    log.Println("服务已启动")
    if err := client.Serve(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Invoker 示例

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    config := &croupier.ClientConfig{
        AgentAddr: "localhost:19090",
        GameID:    "my-game",
        Env:       "production",
        Insecure:  true,
    }

    invoker := croupier.NewInvoker(config)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 同步调用
    result, err := invoker.Invoke(ctx, "player.ban",
        `{"player_id": "12345", "reason": "违规"}`,
        croupier.WithTimeout(10*time.Second),
        croupier.WithIdempotencyKey("ban-12345-20260117"),
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("调用结果: %s", result)

    // 异步任务
    taskID, err := invoker.StartTask(ctx, "player.export",
        `{"format": "csv"}`,
    )
    if err != nil {
        log.Fatal(err)
    }

    // 监听任务事件
    events, err := invoker.StreamTask(ctx, taskID)
    if err != nil {
        log.Fatal(err)
    }

    for event := range events {
        switch event.Type {
        case "progress":
            log.Printf("进度: %d%%", event.Progress)
        case "log":
            log.Printf("日志: %s", event.Message)
        case "done":
            log.Printf("完成: %s", string(event.Payload))
        case "error":
            log.Printf("错误: %s", event.Message)
        }
    }
}
```
