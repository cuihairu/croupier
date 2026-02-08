# Agent Server 端口冲突修复报告

## 问题描述

### 错误现象

Agent 启动后显示 "synced with upstream server via NNG"，但同时 Server 日志显示：

```
ERROR failed to handle request msgID=RegisterRequest error=unknown message type: 0x010101
ERROR failed to handle request msgID=HeartbeatRequest error=unknown message type: 0x010103
```

### 根本原因

**代码错误 + 端口冲突**：Agent 启动代码错误地使用了 `ServerControl.Port`（连接到 Server 的端口）作为本地监听端口，导致端口冲突。

#### 1. 代码错误（主要问题）

在 `services/agent/cmd/root.go:184` 中：

```go
// ❌ 错误代码（修复前）
nngPort := c.ServerControl.Port  // ServerControl.Port 是连接到 Server 的端口，不是本地监听端口！
if nngPort == 0 {
    nngPort = 19091
}
nngAddr := fmt.Sprintf("%s:%d", nngHost, nngPort)
```

**问题**：代码使用了 `ServerControl.Port`（默认 19090），而 `ServerControl` 是用来**连接到 Server** 的配置，不是 Agent 本地监听的配置。

**正确配置**：
- `ServerControl.Port: 19090` - Server 的 NNG ControlService 端口
- `LocalNNG.Addr: ":19091"` - Agent 本地监听地址

#### 2. 配置文件问题（次要问题）

在 `services/agent/etc/agent.yaml` 中，`LocalNNG.Addr` 的默认值也是 `:19090`，进一步加剧了冲突：

```yaml
ServerControl:
  Port: 19090                    # Server 端口（正确）

LocalNNG:
  Addr: ":19090"                 # ❌ 与 Server 端口冲突！
```

#### 导致的问题

1. **Agent 的 Client** 尝试连接到 `localhost:19090`（期望连接到 Server）
2. **实际上连接到了 Agent 自己的 LocalServer**（也监听在 `19090`）
3. **AgentServer** 不支持 ControlService 的消息类型：
   - `MsgRegisterRequest` (0x010101)
   - `MsgHeartbeatRequest` (0x010103)
4. 因此报错："unknown message type: 0x010101"

### 代码分析

#### AgentServer 支持的消息类型

`internal/nng/agent_server.go` 的 `handleRequest` 函数：

```go
func (s *AgentServer) handleRequest(ctx context.Context, msgID uint32, data []byte) ([]byte, error) {
    switch msgID {
    // InvokerService
    case protocol.MsgInvokeRequest:
        return s.handleInvoke(ctx, data)
    case protocol.MsgStartJobRequest:
        return s.handleStartJob(ctx, data)
    case protocol.MsgCancelJobRequest:
        return s.handleCancelJob(ctx, data)

    // OpsService
    case protocol.MsgGetSystemInfoRequest:
        return s.handleGetSystemInfo(ctx, data)
    // ... 其他 OpsService 消息

    // LocalControlService
    case protocol.MsgRegisterLocalRequest:
        return s.handleRegisterLocal(ctx, data)
    case protocol.MsgHeartbeatLocalRequest:
        return s.handleHeartbeatLocal(ctx, data)
    case protocol.MsgListLocalRequest:
        return s.handleListLocal(ctx, data)

    default:
        return nil, fmt.Errorf("unknown message type: 0x%06X", msgID)
    }
}
```

**AgentServer 不支持**：
- `MsgRegisterRequest` (0x010101) - ControlService
- `MsgHeartbeatRequest` (0x010103) - ControlService

这些消息类型只在 `internal/nng/server.go`（Server 的 NNG ControlService）中处理。

## 解决方案

### 1. 修复代码（主要修复）

修改 `services/agent/cmd/root.go` 第 179-188 行：

```go
// ✅ 修复后的代码
slog.Info("loading agent config", "config_file", cfgFile, "config_dir", configDir)

// NNG local service address (for SDK→Agent communication)
// Use LocalNNG.Addr instead of ServerControl.Port to avoid port conflicts
nngAddrStr := strings.TrimSpace(c.LocalNNG.Addr)
if nngAddrStr == "" {
    nngAddrStr = ":19091" // Default NNG Agent port
}
// Remove leading colon if present for display
nngDisplayAddr := nngAddrStr
if strings.HasPrefix(nngAddrStr, ":") {
    nngDisplayAddr = "0.0.0.0" + nngAddrStr
}
nngAddr := nngDisplayAddr
```

### 2. 修复配置文件（配合代码修复）

修改 `services/agent/etc/agent.yaml`：

```yaml
# Agent 配置（修复后）
Agent:
  LocalAddr: "127.0.0.1:19091"   # ✅ 修改为 19091 避免冲突
  HTTPAddr: "127.0.0.1:19092"    # ✅ 相应调整

LocalNNG:
  Addr: ":19091"                 # ✅ 修改为 19091 避免冲突
```

### 端口分配

| 组件 | 端口 | 用途 |
|------|------|------|
| **Server (ControlService)** | `19090` | Server 的 NNG ControlService 端口（标准） |
| **Agent LocalServer** | `19091` | Agent 的本地 NNG 服务（SDK 连接用） |
| **Agent HTTP 管理** | `19092` | Agent 的 HTTP 管理接口 |

## 验证

### 修改前

```bash
$ grep -n "19090" services/agent/etc/agent.yaml
19:  Addr: "localhost:19090"       # Server 地址
34:  LocalAddr: "127.0.0.1:19090"   # ❌ 冲突
45:  Port: 19090                    # Server 端口
55:  Addr: ":19090"                 # ❌ 冲突

# 代码错误地使用了 ServerControl.Port
$ grep -A5 "nngPort := c.ServerControl.Port" services/agent/cmd/root.go
nngPort := c.ServerControl.Port  # ❌ 错误！
if nngPort == 0 {
    nngPort = 19091
}
nngAddr := fmt.Sprintf("%s:%d", nngHost, nngPort)
```

### 修改后

```bash
$ grep -n "19090\|19091\|19092" services/agent/etc/agent.yaml
19:  Addr: "localhost:19090"       # Server 地址（不变）
34:  LocalAddr: "127.0.0.1:19091"   # ✅ 已修复
35:  HTTPAddr: "127.0.0.1:19092"    # ✅ 已修复
45:  Port: 19090                    # Server 端口（不变）
55:  Addr: ":19091"                 # ✅ 已修复

# 代码现在使用 LocalNNG.Addr
$ grep -A8 "nngAddrStr := strings.TrimSpace(c.LocalNNG.Addr)" services/agent/cmd/root.go
nngAddrStr := strings.TrimSpace(c.LocalNNG.Addr)  # ✅ 正确！
if nngAddrStr == "" {
    nngAddrStr = ":19091"
}
nngDisplayAddr := nngAddrStr
if strings.HasPrefix(nngAddrStr, ":") {
    nngDisplayAddr = "0.0.0.0" + nngAddrStr
}
nngAddr := nngDisplayAddr
```

## 影响范围

### 已修改的文件

- ✅ `services/agent/cmd/root.go` - **代码已修复**（使用 LocalNNG.Addr 而不是 ServerControl.Port）
- ✅ `services/agent/etc/agent.yaml` - **配置已修复**（LocalNNG.Addr 改为 :19091）

### 不需要修改的文件

- `internal/nng/server.go` - Server 实现（无需修改）
- `internal/nng/agent_server.go` - Agent Server 实现（无需修改）
- `internal/nng/client.go` - NNG Client 实现（无需修改）
- `pkg/protocol/message.go` - 协议定义（无需修改）
- `services/agent/internal/config/config.go` - 配置结构体（无需修改）

### 需要更新的文档

- `docs/AGENT_SESSION_PERSISTENCE.md` - 可能需要更新端口示例
- `README.md` - 可能需要更新端口说明

## 测试建议

### 1. 重新构建

```bash
# 必须重新构建，因为修改了代码
make build
```

### 2. 重启 Server 和 Agent

```bash
# 停止现有进程
pkill croupier-server
pkill croupier-agent

# 启动 Server
./bin/croupier-server -f services/server/etc/server.yaml

# 启动 Agent（使用新代码和新配置）
./bin/croupier-agent -f services/agent/etc/agent.yaml
```

### 3. 验证 Agent 连接到 Server

**Agent 日志应该显示**：
```
INFO agent nng core started listen=0.0.0.0:19091  # ✅ 注意端口是 19091
INFO NNG Agent server listening addr=tcp://0.0.0.0:19091 transport=tcp
INFO connecting to upstream server addr=localhost:19090 tls=true
INFO NNG Control client connected addr=tcp://localhost:19090
INFO synced with upstream server via NNG functions=19
INFO ✅ upstream connected and registered successfully
```

**Server 日志应该显示**：
```
INFO Agent registered via NNG agent_id=xxx game_id=tower_defense
```

**不应该再出现**：
```
❌ ERROR failed to handle request msgID=RegisterRequest error=unknown message type: 0x010101
❌ ERROR failed to handle request msgID=HeartbeatRequest error=unknown message type: 0x010103
```

### 4. 测试心跳

等待 30 秒，观察心跳日志：

**Agent 日志**：
```
（心跳成功时没有错误日志）
```

**Server 日志**：
```
（心跳正常时没有错误日志）
```

**不应该再出现**：
```
❌ ERROR failed to handle request msgID=HeartbeatRequest error=unknown message type: 0x010103
```

## 总结

### 问题

**代码错误**：Agent 启动代码错误地使用了 `ServerControl.Port`（连接到 Server 的端口）作为本地监听端口，导致 Agent 的 LocalServer 和 Server 监听在相同的端口 `19090`，造成端口冲突。

**端口冲突**：Agent 的 Client 尝试连接到 `localhost:19090`（期望连接到 Server），但实际连接到了 Agent 自己的 LocalServer，而 AgentServer 不支持 `RegisterRequest`/`HeartbeatRequest` 消息类型。

### 解决方案

1. **修复代码**：`services/agent/cmd/root.go` 使用 `LocalNNG.Addr` 而不是 `ServerControl.Port`
2. **修复配置**：`services/agent/etc/agent.yaml` 将 `LocalNNG.Addr` 从 `:19090` 改为 `:19091`

### 验证方法

1. **重新构建**：`make build`（必须，因为修改了代码）
2. **重启 Server 和 Agent**
3. **检查日志**：
   - ✅ Agent 日志显示 `agent nng core started listen=0.0.0.0:19091`
   - ✅ 不再出现 "unknown message type: 0x010101" 错误
   - ✅ Agent 成功连接到 Server 并注册

---

**修复时间**: 2026-02-08
**修复文件**:
- `services/agent/cmd/root.go` - **代码修复**（主要）
- `services/agent/etc/agent.yaml` - **配置修复**（配合）

**问题类型**: 代码错误（错误使用配置字段）+ 端口冲突
**影响范围**: Agent 无法连接到 Server 进行注册
