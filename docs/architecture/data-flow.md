---
title: 数据流
icon: route
order: 4
category:
  - 系统架构
tag:
  - 架构
  - 数据流
  - 调用流程
---

# 数据流

本文档详细说明 Croupier 系统中的调用流、数据流转和事件处理。

## 目录

[[toc]]

## 标准调用流程

### 端到端流程

```mermaid
sequenceDiagram
    participant User as 用户
    participant UI as Dashboard
    participant Server as Croupier Server
    participant Agent as Croupier Agent
    participant Game as Game Server

    User->>UI: 点击"封禁玩家"
    UI->>Server: POST /api/invoke
    Server->>Server: 验证 JWT Token
    Server->>Server: RBAC/ABAC 权限检查
    Server->>Server: 审批检查
    Server->>Server: 选择目标 Agent
    Server->>Agent: gRPC InvokeFunction
    Agent->>Game: 本地 gRPC 调用
    Game->>Agent: 返回结果
    Agent->>Server: 返回结果
    Server->>Server: 记录审计日志
    Server->>UI: 返回结果
    UI->>User: 显示成功提示
```

### 详细步骤

| 步骤 | 操作 | 说明 |
|------|------|------|
| 1 | 用户操作 | 在 Dashboard 点击操作按钮 |
| 2 | HTTP 请求 | 发送 POST /api/invoke |
| 3 | 身份验证 | 验证 JWT Token |
| 4 | 权限检查 | RBAC/ABAC 验证 |
| 5 | 审批检查 | 检查是否需要审批 |
| 6 | 路由选择 | 选择目标 Agent |
| 7 | gRPC 调用 | 调用 Agent 的 InvokeFunction |
| 8 | 业务执行 | Game Server 执行业务逻辑 |
| 9 | 结果返回 | 逐层返回结果 |
| 10 | 审计记录 | 记录操作审计日志 |

## 同步调用流

### 请求格式

```json
POST /api/invoke
{
  "function_id": "player.ban",
  "game_id": "my-game",
  "env": "prod",
  "payload": {
    "player_id": "player_123",
    "duration": 24,
    "reason": "作弊"
  },
  "options": {
    "idempotency_key": "unique-key-123",
    "timeout": "30s"
  }
}
```

### 响应格式

**成功响应**：
```json
{
  "success": true,
  "result": {
    "ban_id": "ban_456",
    "expires_at": "2024-12-02T10:30:00Z"
  }
}
```

**需要审批**：
```json
{
  "success": false,
  "pending_approval": true,
  "approval_id": "approval_789",
  "message": "操作需要双人审批"
}
```

**权限拒绝**：
```json
{
  "success": false,
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "没有权限执行该操作",
    "required_permission": "player.ban"
  }
}
```

## 异步调用流（作业）

### 异步调用流程

```mermaid
sequenceDiagram
    participant UI as Dashboard
    participant Server as Croupier Server
    participant Agent as Croupier Agent
    participant Game as Game Server

    UI->>Server: POST /api/jobs
    Server->>Server: 创建作业记录
    Server-->>UI: 返回 job_id
    UI->>Server: SSE /api/jobs/{id}/events

    Server->>Agent: InvokeJob
    Agent->>Game: 异步执行

    loop 作业事件流
        Game-->>Agent: 进度更新
        Agent-->>Server: JobEvent(PROGRESS)
        Server-->>UI: SSE 推送事件
    end

    Game-->>Agent: 执行完成
    Agent-->>Server: JobEvent(DONE)
    Server-->>UI: SSE 推送完成
```

### 作业事件类型

```protobuf
enum EventType {
  START    = 0;  // 作业开始
  PROGRESS = 1;  // 进度更新
  LOG      = 2;  // 日志输出
  DONE     = 3;  // 作业完成
  ERROR    = 4;  // 作业错误
}

message JobEvent {
  string job_id = 1;
  EventType type = 2;
  string message = 3;
  double progress = 4;  // 0.0 - 1.0
  int64 timestamp = 5;
}
```

### 作业管理 API

```bash
# 创建异步作业
POST /api/jobs
{
  "function_id": "data.export",
  "payload": {...}
}
# 返回: {"job_id": "job_123"}

# 获取作业状态
GET /api/jobs/job_123
# 返回: {"status": "running", "progress": 0.5}

# 流式获取事件
GET /api/jobs/job_123/events
# SSE 流式事件

# 取消作业
DELETE /api/jobs/job_123
```

## 审批流程

### 审批数据流

```mermaid
stateDiagram-v2
    [*] --> 提交: 用户提交调用
    提交 --> 检查: Server 检查配置
    检查 --> 待审批: 需要双人审批
    检查 --> 直接执行: 无需审批

    待审批 --> 已通过: 第二人审批
    待审批 --> 已拒绝: 拒绝
    待审批 --> 已取消: 超时

    已通过 --> 执行: 开始执行
    直接执行 --> 执行: 开始执行

    执行 --> 完成: 执行成功
    执行 --> 失败: 执行失败

    完成 --> [*]
    失败 --> [*]
    已拒绝 --> [*]
    已取消 --> [*]
```

### 审批请求格式

```json
POST /api/approvals
{
  "function_id": "player.ban",
  "game_id": "my-game",
  "env": "prod",
  "payload": {...},
  "reason": "玩家使用外挂",
  "requested_by": "user_123"
}
```

### 审批操作

```bash
# 审批通过
POST /api/approvals/{id}/approve
{
  "approved_by": "user_456"
}

# 审批拒绝
POST /api/approvals/{id}/reject
{
  "rejected_by": "user_456",
  "reason": "证据不足"
}
```

## 隧道模式（经 Edge）

### 隧道调用流程

```mermaid
sequenceDiagram
    participant UI as Dashboard
    participant Server as Server (内网)
    participant Edge as Edge (DMZ)
    participant Agent as Agent (游戏内网)
    participant Game as Game Server

    UI->>Server: HTTPS POST /api/invoke
    Server->>Server: 路由决策 (需要经 Edge)
    Server->>Edge: gRPC ForwardInvoke
    Edge->>Agent: 隧道传输 InvokeFunction
    Agent->>Game: 本地调用
    Game-->>Agent: 结果
    Agent-->>Edge: 隧道传输结果
    Edge-->>Server: 返回结果
    Server-->>UI: 返回结果
```

### 隧道复用

```
单一隧道连接复用多个请求：

Server <---> Edge
    |
    +-- Tunnel 1 --> Agent 1 --> Game Server A
    |                       +--> Game Server B
    |
    +-- Tunnel 2 --> Agent 2 --> Game Server C
```

## 广播模式

### 广播调用流程

```mermaid
graph LR
    A[Server] -->|Invoke| B1[Agent 1]
    A -->|Invoke| B2[Agent 2]
    A -->|Invoke| B3[Agent 3]

    B1 --> G1[Game 1]
    B2 --> G2[Game 2]
    B3 --> G3[Game 3]

    A --> R[聚合结果]
```

### 广播调用示例

```bash
POST /api/invoke
{
  "function_id": "config.reload",
  "routing": {
    "mode": "broadcast"
  }
}
```

## 审计数据流

### 审计记录流程

```mermaid
sequenceDiagram
    participant User as 用户
    participant Server as Server
    participant Audit as 审计服务
    participant Storage as 存储

    User->>Server: 调用函数
    Server->>Audit: 记录审计事件 (START)
    Audit->>Storage: 持久化

    Server->>Server: 执行业务逻辑
    Server->>Audit: 记录审计事件 (COMPLETE)
    Audit->>Storage: 持久化

    Audit->>Audit: 计算哈希链
```

### 审计事件结构

```json
{
  "audit_id": "audit_20241201_001",
  "timestamp": "2024-12-01T10:30:00Z",
  "user_id": "user_123",
  "username": "admin",
  "action": "function.invoke",
  "game_id": "my-game",
  "env": "prod",
  "function_id": "player.ban",
  "payload_preview": {
    "player_id": "***",
    "duration": 24
  },
  "result": "success",
  "ip": "192.168.1.100",
  "ip_region": "中国 上海",
  "hash": "sha256(...)",
  "prev_hash": "sha256(...)"
}
```

## 实时数据流

### WebSocket / SSE

```mermaid
sequenceDiagram
    participant Client as 前端
    participant Server as Server

    Client->>Server: SSE GET /api/events
    Server-->>Client: 建立连接

    loop 事件推送
        Server->>Server: 检测到事件
        Server-->>Client: data: {...}
    end

    Client->>Server: 关闭连接
```

### 事件类型

| 事件类型 | 说明 | 示例 |
|----------|------|------|
| `function.called` | 函数被调用 | `{function_id, user, result}` |
| `job.progress` | 作业进度更新 | `{job_id, progress, message}` |
| `approval.pending` | 待审批请求 | `{approval_id, function_id}` |
| `agent.connected` | Agent 上线 | `{agent_id, game_id}` |
| `agent.disconnected` | Agent 下线 | `{agent_id, reason}` |

## 错误处理流

### 错误传播

```mermaid
graph LR
    A[Game Server Error] --> B[Agent]
    B --> C[Server]
    C --> D[Dashboard]

    style A fill:#ffcccc
    style B fill:#ffcccc
    style C fill:#ffcccc
    style D fill:#ffcccc
```

### 错误响应格式

```json
{
  "success": false,
  "error": {
    "code": "PLAYER_NOT_FOUND",
    "message": "玩家不存在",
    "details": {
      "player_id": "player_123"
    },
    "trace_id": "trace_abc123",
    "timestamp": "2024-12-01T10:30:00Z"
  }
}
```

## 数据格式转换

### JSON ↔ Protobuf

```javascript
// HTTP JSON 请求
{
  "function_id": "player.ban",
  "payload": {"player_id": "123", "duration": 24}
}

// 转换为 gRPC Protobuf
InvokeFunctionRequest {
  function_id: "player.ban"
  payload {
    fields {
      key: "player_id"
      value { string_value: "123" }
    }
    fields {
      key: "duration"
      value { number_value: 24 }
    }
  }
}
```

## 相关文档

- [分层设计](./layers.md)
- [组件说明](./components.md)
- [API 参考](../api/README.md)
