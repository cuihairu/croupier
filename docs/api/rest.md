---
title: REST API
icon: server
order: 3
category:
  - API 参考
tag:
  - REST
  - HTTP
  - API
---

# REST API

Croupier 提供 HTTP REST API，供 Dashboard 和外部系统调用。

## 目录

[[toc]]

## 基础信息

### Base URL

```
开发环境: http://localhost:8080
生产环境: https://croupier.example.com
```

### 认证

使用 Bearer Token 认证：

```http
Authorization: Bearer {token}
```

### 通用请求头

```http
Content-Type: application/json
X-Game-ID: {game_id}
X-Env: {env}
```

### 通用响应格式

**成功响应**：
```json
{
  "success": true,
  "data": {...}
}
```

**错误响应**：
```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述",
    "details": {...}
  }
}
```

## 函数调用

### 调用函数（同步）

```http
POST /api/invoke
```

**请求体**：
```json
{
  "function_id": "player.ban",
  "payload": {
    "player_id": "player_123",
    "duration": 24,
    "reason": "作弊"
  },
  "options": {
    "idempotency_key": "unique-key-123"
  }
}
```

**响应**：
```json
{
  "success": true,
  "result": {
    "ban_id": "ban_456",
    "expires_at": "2024-12-02T10:30:00Z"
  }
}
```

### 调用函数（异步）

```http
POST /api/jobs
```

**请求体**：
```json
{
  "function_id": "data.export",
  "payload": {...}
}
```

**响应**：
```json
{
  "success": true,
  "job_id": "job_abc123",
  "status": "pending"
}
```

### 获取作业状态

```http
GET /api/jobs/{job_id}
```

**响应**：
```json
{
  "job_id": "job_abc123",
  "status": "running",
  "progress": 0.5,
  "started_at": "2024-12-01T10:00:00Z",
  "result": null
}
```

### 流式获取作业事件

```http
GET /api/jobs/{job_id}/events
```

返回 Server-Sent Events (SSE) 流：

```
data: {"type":"PROGRESS","progress":0.1,"message":"处理中..."}

data: {"type":"PROGRESS","progress":0.5,"message":"处理中..."}

data: {"type":"DONE","progress":1.0,"result":{...}}
```

### 取消作业

```http
DELETE /api/jobs/{job_id}
```

**请求体**：
```json
{
  "reason": "用户取消"
}
```

## 函数管理

### 获取函数列表

```http
GET /api/functions?game_id={game_id}&env={env}
```

**查询参数**：
| 参数 | 类型 | 说明 |
|------|------|------|
| `game_id` | string | 游戏 ID |
| `env` | string | 环境 |
| `category` | string | 函数分类 |
| `page` | int | 页码 |
| `size` | int | 每页数量 |

**响应**：
```json
{
  "functions": [
    {
      "id": "player.ban",
      "name": "封禁玩家",
      "category": "player",
      "risk_level": "high"
    }
  ],
  "total": 100,
  "page": 1,
  "size": 20
}
```

### 获取函数详情

```http
GET /api/functions/{function_id}?game_id={game_id}&env={env}
```

**响应**：
```json
{
  "id": "player.ban",
  "name": "封禁玩家",
  "description": "封禁指定玩家账号",
  "params_schema": {...},
  "result_schema": {...},
  "auth": {...},
  "ui": {...}
}
```

### 注册函数

```http
POST /api/functions
```

**请求体**：
```json
{
  "game_id": "my-game",
  "env": "prod",
  "descriptor": {...}
}
```

### 注销函数

```http
DELETE /api/functions/{function_id}?game_id={game_id}&env={env}
```

## Agent 管理

### 获取 Agent 列表

```http
GET /api/agents?game_id={game_id}&env={env}
```

**响应**：
```json
{
  "agents": [
    {
      "agent_id": "agent_1",
      "game_id": "my-game",
      "env": "prod",
      "status": "online",
      "last_heartbeat": "2024-12-01T10:00:00Z",
      "functions": ["player.ban", "player.kick"]
    }
  ]
}
```

### 获取 Agent 详情

```http
GET /api/agents/{agent_id}
```

### 获取 Agent 分配配置

```http
GET /api/agents/{agent_id}/assignments
```

## 审批流程

### 创建审批请求

```http
POST /api/approvals
```

**请求体**：
```json
{
  "function_id": "player.ban",
  "game_id": "my-game",
  "env": "prod",
  "payload": {
    "player_id": "player_123",
    "duration": 24
  },
  "reason": "玩家使用外挂"
}
```

**响应**：
```json
{
  "approval_id": "approval_123",
  "status": "pending",
  "required_approvals": 2,
  "current_approvals": 0
}
```

### 获取审批列表

```http
GET /api/approvals?state=pending&game_id={game_id}
```

**查询参数**：
| 参数 | 类型 | 说明 |
|------|------|------|
| `state` | string | 状态：pending/approved/rejected |
| `game_id` | string | 游戏 ID |
| `function_id` | string | 函数 ID |
| `page` | int | 页码 |
| `size` | int | 每页数量 |

### 获取审批详情

```http
GET /api/approvals/{approval_id}
```

### 审批通过

```http
POST /api/approvals/{approval_id}/approve
```

**请求体**：
```json
{
  "comment": "确认封禁"
}
```

### 审批拒绝

```http
POST /api/approvals/{approval_id}/reject
```

**请求体**：
```json
{
  "reason": "证据不足"
}
```

## 审计日志

### 查询审计日志

```http
POST /api/audit/query
```

**请求体**：
```json
{
  "game_id": "my-game",
  "env": "prod",
  "start_time": "2024-12-01T00:00:00Z",
  "end_time": "2024-12-02T00:00:00Z",
  "filters": {
    "user_id": "user_123",
    "function_id": "player.ban",
    "action": "function.invoke"
  },
  "page": 1,
  "size": 50
}
```

**响应**：
```json
{
  "audits": [
    {
      "audit_id": "audit_001",
      "timestamp": "2024-12-01T10:30:00Z",
      "user": {
        "user_id": "user_123",
        "username": "admin"
      },
      "action": "function.invoke",
      "function_id": "player.ban",
      "payload_preview": {...},
      "result": "success",
      "ip": "192.168.1.100",
      "ip_region": "中国 上海"
    }
  ],
  "total": 1000,
  "page": 1,
  "size": 50
}
```

### 获取审计详情

```http
GET /api/audit/{audit_id}
```

## 用户管理

### 登录

```http
POST /api/auth/login
```

**请求体**：
```json
{
  "username": "admin",
  "password": "password",
  "totp": "123456"  // 可选，双因素认证
}
```

**响应**：
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "user_id": "user_123",
    "username": "admin",
    "roles": ["admin"]
  }
}
```

### 登出

```http
POST /api/auth/logout
```

### 获取当前用户信息

```http
GET /api/auth/me
```

**响应**：
```json
{
  "user_id": "user_123",
  "username": "admin",
  "email": "admin@example.com",
  "roles": ["admin"],
  "permissions": ["*.*"]
}
```

## 健康检查

### 健康检查

```http
GET /healthz
```

**响应**：
```json
{
  "status": "ok"
}
```

### 就绪检查

```http
GET /readyz
```

**响应**：
```json
{
  "status": "ready"
}
```

### 版本信息

```http
GET /version
```

**响应**：
```json
{
  "version": "1.0.0",
  "commit": "abc123",
  "build_time": "2024-12-01T10:00:00Z"
}
```

## 错误码

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| `OK` | 200 | 成功 |
| `INVALID_ARGUMENT` | 400 | 参数错误 |
| `UNAUTHENTICATED` | 401 | 未认证 |
| `PERMISSION_DENIED` | 403 | 权限不足 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `ALREADY_EXISTS` | 409 | 资源已存在 |
| `RESOURCE_EXHAUSTED` | 429 | 请求过于频繁 |
| `INTERNAL` | 500 | 内部错误 |
| `NOT_IMPLEMENTED` | 501 | 未实现 |
| `UNAVAILABLE` | 503 | 服务不可用 |

### 错误响应示例

```json
{
  "success": false,
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "没有权限执行该操作",
    "details": {
      "required_permission": "player.ban",
      "user_permissions": ["player.view"]
    }
  }
}
```

## 限流

### 速率限制

API 实施速率限制：

| 级别 | 限制 |
|------|------|
| 默认 | 100 req/min |
| 认证用户 | 1000 req/min |
| 管理员 | 无限制 |

### 限流响应

```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1701427200

{
  "success": false,
  "error": {
    "code": "RESOURCE_EXHAUSTED",
    "message": "请求过于频繁，请稍后再试"
  }
}
```

## SDK 示例

### cURL

```bash
curl -X POST http://localhost:8080/api/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "function_id": "player.ban",
    "payload": {
      "player_id": "player_123",
      "duration": 24
    }
  }'
```

### JavaScript

```javascript
const response = await fetch('http://localhost:8080/api/invoke', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`,
  },
  body: JSON.stringify({
    function_id: 'player.ban',
    payload: {
      player_id: 'player_123',
      duration: 24,
    },
  }),
});

const result = await response.json();
```

### Python

```python
import requests

response = requests.post(
    'http://localhost:8080/api/invoke',
    headers={
        'Content-Type': 'application/json',
        'Authorization': f'Bearer {token}',
    },
    json={
        'function_id': 'player.ban',
        'payload': {
            'player_id': 'player_123',
            'duration': 24,
        },
    },
)

result = response.json()
```

## 相关文档

- [gRPC API](./grpc.md)
- [Proto 选项](./proto-options.md)
- [API 概览](./README.md)
