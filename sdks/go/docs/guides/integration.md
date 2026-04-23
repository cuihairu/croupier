# Croupier Go SDK 集成指南

本指南提供完整的 Croupier Go SDK 集成步骤，帮助开发者快速接入游戏后端平台。

## 目录

- [快速开始](#快速开始)
- [安装](#安装)
- [核心概念](#核心概念)
- [完整接口参考](#完整接口参考)
- [配置说明](#配置说明)
- [生产部署](#生产部署)
- [故障排查](#故障排查)

---

## 快速开始

### 安装 SDK

```bash
# 获取 SDK
go get github.com/cuihairu/croupier/sdks/go
```

### 最小集成示例

```go
package main

import (
    "context"
    "encoding/json"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// 定义函数处理器
func myHandler(ctx context.Context, payload []byte) string {
    var data map[string]interface{}
    json.Unmarshal(payload, &data)
    // 处理业务逻辑
    result := map[string]interface{}{
        "status": "success",
    }
    bytes, _ := json.Marshal(result)
    return string(bytes)
}

func main() {
    // 创建配置
    config := &croupier.ClientConfig{
        AgentAddr:  "127.0.0.1:19090",
        ServiceID:  "my-service",
    }

    // 创建客户端
    client := croupier.NewClient(config)

    // 注册函数
    descriptor := &croupier.FunctionDescriptor{
        ID:      "game.action",
        Version: "1.0.0",
        Category: "gameplay",
        Risk:     "low",
    }
    client.RegisterFunction(descriptor, myHandler)

    // 连接并启动服务
    client.Connect()
    client.Serve()  // 阻塞运行
}
```

---

## 安装

### 系统要求

| 平台 | 架构 | 状态 |
|------|------|------|
| **Linux** | x64, ARM64 | ✅ 支持 |
| **macOS** | x64, ARM64 (Apple Silicon) | ✅ 支持 |
| **Windows** | x64 | ✅ 支持 |

**构建工具：**
- Go 1.20 或更高版本
- Protocol Buffers 编译器 (protoc) - 可选，仅 gRPC 模式需要

### 从源码安装

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier-sdk-go.git
cd croupier-sdk-go

# 下载依赖
go mod download

# 构建
go build ./...
```

### 验证安装

```bash
go run examples/basic/main.go
```

---

## 核心概念

### 客户端 (Client)

客户端负责注册和管理游戏函数，接收来自 Agent 的调用请求。

```go
import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"

client := croupier.NewClient(config)
```

### 函数描述符 (FunctionDescriptor)

描述函数的元数据：

```go
descriptor := &croupier.FunctionDescriptor{
    ID:        "player.ban",        // 函数唯一标识
    Version:   "1.0.0",             // 版本号
    Category:  "moderation",        // 业务分类
    Risk:      "high",              // 风险等级: low, medium, high
    Entity:    "player",            // 关联实体
    Operation: "update",            // 操作类型: create, read, update, delete
    Enabled:   true,                // 是否启用
}
```

### 函数处理器 (Handler)

函数处理器是处理具体业务逻辑的函数：

```go
// Handler 类型定义
type Handler func(ctx context.Context, payload []byte) string

func handler(ctx context.Context, payload []byte) string {
    /**
     * Args:
     *   ctx: 调用上下文，包含调用者信息
     *   payload: 请求负载，JSON 格式的 []byte
     *
     * Returns:
     *   string: JSON 格式的响应字符串
     */

    var data map[string]interface{}
    json.Unmarshal(payload, &data)

    // 处理业务逻辑
    result := map[string]interface{}{
        "status": "success",
    }
    bytes, _ := json.Marshal(result)
    return string(bytes)
}
```

---

## 完整接口参考

### CroupierClient

#### 初始化

```go
import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"

config := &croupier.ClientConfig{
    // 连接配置
    AgentAddr:  "127.0.0.1:19090",
    ControlAddr: "127.0.0.1:18080",
    LocalAddr:   "127.0.0.1:0",

    // 身份配置
    ServiceID:      "my-service",
    ServiceVersion: "1.0.0",
    GameID:         "my-game",
    Env:            "production",

    // TLS 配置
    Insecure:   false,
    CertFile:   "/path/to/cert.pem",
    KeyFile:    "/path/to/key.pem",
    CAFile:     "/path/to/ca.pem",
    ServerName: "agent.croupier.io",

    // 超时配置
    Timeout: 30 * time.Second,
}

client := croupier.NewClient(config)
```

#### 方法

| 方法 | 说明 | 返回值 |
|------|------|--------|
| `RegisterFunction(descriptor, handler)` | 注册函数 | `error` |
| `UnregisterFunction(functionID)` | 取消注册函数 | `error` |
| `Connect()` | 连接到 Agent | `error` |
| `Disconnect()` | 断开连接 | `error` |
| `Serve()` | 启动服务循环（阻塞） | `error` |
| `IsConnected()` | 检查连接状态 | `bool` |

### Invoker

用于主动调用远程函数（可选）：

```go
import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"

invokerConfig := &croupier.InvokerConfig{
    Address:       "localhost:8080",
    Insecure:      true,
    TimeoutSeconds: 30,
}

invoker := croupier.NewInvoker(invokerConfig)

// 连接
invoker.Connect()

// 调用函数
result := invoker.Invoke("player.get", `{"player_id":"123"}`)

// 启动异步任务
taskID := invoker.StartTask("item.create", `{"type":"sword"}`)

// 流式获取任务事件
eventCh := invoker.StreamTask(taskID)
for event := range eventCh {
    fmt.Printf("事件: %s, 数据: %s\n", event.EventType, event.Payload)
}

// 取消任务
invoker.CancelTask(taskID)

// 关闭
invoker.Close()
```

---

## 配置说明

### ClientConfig 完整参数

```go
config := &croupier.ClientConfig{
    // === 连接配置 ===
    AgentAddr:  "127.0.0.1:19090",  // Agent 地址
    ControlAddr: "127.0.0.1:18080", // Control 平台地址（可选）

    // === 身份配置 ===
    ServiceID:      "my-service",  // 服务标识（必填）
    ServiceVersion: "1.0.0",       // 服务版本
    GameID:         "my-game",     // 游戏标识
    Env:            "production",  // 环境: development, staging, production

    // === TLS 配置 ===
    Insecure:   false,                   // 是否禁用 TLS
    CertFile:   "/path/to/cert.pem",     // 客户端证书
    KeyFile:    "/path/to/key.pem",      // 客户端私钥
    CAFile:     "/path/to/ca.pem",       // CA 证书
    ServerName: "agent.croupier.io",     // SNI 名称

    // === 超时配置 ===
    Timeout: 30 * time.Second,

    // === 重连配置 ===
    AutoReconnect:         true,
    ReconnectInterval:     5 * time.Second,
    ReconnectMaxAttempts:  0,  // 0 = 无限重试
}
```

### 环境变量

可通过环境变量覆盖配置：

```bash
export CROUPIER_AGENT_ADDR="127.0.0.1:19090"
export CROUPIER_SERVICE_ID="my-service"
export CROUPIER_INSECURE="false"
```

---

## 生产部署

### Docker 部署

创建 `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建
RUN CGO_ENABLED=0 go build -o game-service ./cmd/server

# 运行时镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/game-service .

# 暴露健康检查端口
EXPOSE 8080

# 运行服务
CMD ["./game-service"]
```

创建 `docker-compose.yml`:

```yaml
version: '3.8'

services:
  game-service:
    build: .
    environment:
      - CROUPIER_AGENT_ADDR=agent:19090
      - CROUPIER_ENV=production
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Kubernetes 部署

创建 `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: croupier-game-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: game-service
  template:
    metadata:
      labels:
        app: game-service
    spec:
      containers:
      - name: game-service
        image: your-registry/croupier-game-service:latest
        env:
        - name: CROUPIER_AGENT_ADDR
          value: "croupier-agent:19090"
        - name: CROUPIER_ENV
          value: "production"
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: croupier-game-service
spec:
  selector:
    app: game-service
  ports:
  - port: 80
    targetPort: 8080
```

---

## 故障排查

### 连接失败

**问题**: 无法连接到 Agent

```go
// 检查配置
fmt.Printf("Agent 地址: %s\n", config.AgentAddr)

// 检查网络连通性
import "net"
conn, err := net.Dial("tcp", config.AgentAddr)
if err != nil {
    fmt.Printf("连接测试失败: %v\n", err)
} else {
    conn.Close()
    fmt.Println("连接测试成功")
}
```

**解决方案**:
1. 确认 Agent 服务正在运行
2. 检查防火墙规则
3. 验证地址格式

### 函数未注册

**问题**: 函数注册失败

```go
// 检查描述符
descriptor := &croupier.FunctionDescriptor{
    ID:      "player.ban",
    Version: "1.0.0",
}

// 验证必填字段
if descriptor.ID == "" {
    fmt.Println("函数 ID 不能为空")
}
if descriptor.Version == "" {
    fmt.Println("版本号不能为空")
}

// 注册前检查
if !client.IsConnected() {
    fmt.Println("客户端未连接")
}
```

### 性能问题

**优化建议**:

1. 使用 goroutine 处理耗时操作

```go
func asyncHandler(ctx context.Context, payload []byte) string {
    var data map[string]interface{}
    json.Unmarshal(payload, &data)

    // 提交到 goroutine 异步处理
    go processLongRunningTask(data)

    // 立即返回
    return `{"status":"submitted"}`
}
```

2. 启用连接复用

```go
config.EnableKeepAlive = true
config.KeepAliveInterval = 30 * time.Second
```

---

## 更多资源

- [约定规范](conventions.md) - 命名约定和最佳实践
- [约定规范](./conventions.md) - 命名约定和最佳实践
- [示例代码](../examples/) - 完整的示例程序
- [API 参考](../api/) - 详细的 API 文档
- [问题反馈](https://github.com/cuihairu/croupier-sdk-go/issues)
