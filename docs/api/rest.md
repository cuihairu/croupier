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
开发环境: http://localhost:18780
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

`X-Game-ID` / `X-Env` 是 scope 的唯一传递方式：业务路由按这两个 header 解析 game/env 并路由到对应游戏数据库，payload 与 URL 中不可覆盖 scope。

### 通用响应格式

成功响应直接返回业务 payload，不使用 envelope：

```json
{ "id": 1, "name": "admin" }
```

列表返回业务对象本身（如 `{ "items": [...], "total": 10 }`）。`201` 创建成功、`204` 无 body。

错误响应使用统一错误对象，`error` 为稳定的 snake_case 错误码：

```json
{
  "error": "validation_failed",
  "message": "请求参数无效",
  "details": { "gameId": "不能为空" }
}
```

显式例外：SSE 端点返回 `text/event-stream`；健康检查返回最小 payload；文件下载遵循 content-type 要求。

## 函数调用

### 调用函数（同步，管理/调试路径）

```http
POST /api/v1/functions/{function_id}/invoke
```

**请求体**：

```json
{
  "payload": {
    "player_id": "player_123",
    "duration": 24,
    "reason": "作弊"
  },
  "idempotencyKey": "unique-key-123"
}
```

**响应**（直接返回函数结果）：

```json
{
  "ban_id": "ban_456",
  "expires_at": "2026-12-02T10:30:00Z"
}
```

运营页面的受控执行主路径是 `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute`（见 [页面与控制台 API](./page.md)），浏览器不得直接调用函数目录。

### 启动任务

```http
POST /api/v1/tasks
```

**请求体**：

```json
{
  "functionId": "data.export",
  "payload": {}
}
```

**响应**：

```json
{
  "taskId": "task_abc123",
  "status": "queued"
}
```

### 获取任务详情 / 事件 / 取消

```http
GET  /api/v1/tasks/{task_id}
GET  /api/v1/tasks/{task_id}/events
POST /api/v1/tasks/{task_id}/cancel
POST /api/v1/tasks/cancel
```

## 函数管理

### 获取函数描述符列表

```http
GET /api/v1/functions/descriptors?type={type}&gameId={gameId}
```

**查询参数**：
| 参数 | 类型 | 说明 |
|------|------|------|
| `type` | string | 可选，按类型过滤 |
| `gameId` | string | 可选，按游戏过滤 |

### 获取/删除函数

```http
GET    /api/v1/functions/{function_id}
DELETE /api/v1/functions/{function_id}
```

函数注册由 SDK / OpenAPI provider 经 Agent session 完成，或通过 `POST /api/v1/metadata/functions` 登记元数据；注册字段以 [OpenAPI / SDK Descriptor v2](../architecture/openapi-sdk-descriptor-v2.md) 为准。OpenAPI 文档导入见 [OpenAPI 注册](../guide/integrations/openapi-registration.md)。

## Agent 管理

### 获取 Agent 列表

```http
GET /api/v1/ops/agents
```

需要 Bearer 认证与 `X-Game-ID` / `X-Env` scope 头。运维命令（system-info、processes、exec 等）见[运维 API](./ops.md)。

## 审批流程

审批请求由执行链路按治理策略自动创建（如 `approval.required=true` 的 binding execute），不提供手工创建端点。

```http
GET  /api/v1/approvals          # 审批列表
GET  /api/v1/approvals/{id}     # 审批详情
POST /api/v1/approvals/{id}/approve
POST /api/v1/approvals/{id}/reject
```

## 审计日志

```http
GET  /api/v1/audit     # 查询审计（支持过滤与分页参数）
POST /api/v1/audit     # 同上，复杂过滤走 body
```

页面执行的审计记录包含 scope/page/binding/function/版本/结果与 trace 关联。

## 认证与当前用户

```http
POST /api/v1/auth/login     # { "username", "password", "totp"? } -> { "token" }
POST /api/v1/auth/logout
POST /api/v1/auth/check     # 权限检查
GET  /api/v1/profile        # 当前用户信息
GET  /api/v1/profile/permissions
GET  /api/v1/profile/games
PATCH /api/v1/profile/scope # 切换当前 scope
```

## 健康检查

```http
GET /healthz
GET /api/v1/monitoring/healthz
```

**响应**：

```json
{ "status": "ok" }
```

## 错误码

`error` 字段为稳定 snake_case 错误码，前端逻辑应按 `error` 分支、UI 文案取 `message`：

| error                 | HTTP 状态 | 说明                           |
| --------------------- | --------- | ------------------------------ |
| `bad_request`         | 400       | 参数错误                       |
| `unauthorized`        | 401       | 未认证 / token 无效            |
| `forbidden`           | 403       | 已认证但无权限                 |
| `not_found`           | 404       | 资源不存在                     |
| `conflict`            | 409       | 通用冲突                       |
| `binding_stale`       | 409       | 页面绑定合同已变化，执行被阻断 |
| `validation_failed`   | 422       | 语义校验失败                   |
| `internal_error`      | 500       | 内部错误                       |
| `not_implemented`     | 501       | 未实现                         |
| `service_unavailable` | 503       | 服务不可用                     |

业务错误可定义更细的稳定码（如 `binding_stale`）覆盖同状态的通用码。

### 错误响应示例

```json
{
  "error": "forbidden",
  "message": "没有权限执行该操作",
  "details": {
    "requiredPermission": "player:ban"
  }
}
```

## SDK 示例

### cURL

```bash
curl -X POST http://localhost:18780/api/v1/functions/player.ban/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Game-ID: my-game" \
  -H "X-Env: prod" \
  -d '{
    "payload": {
      "player_id": "player_123",
      "duration": 24
    }
  }'
```

### JavaScript

```javascript
const response = await fetch(
  "http://localhost:18780/api/v1/functions/player.ban/invoke",
  {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      "X-Game-ID": gameId,
      "X-Env": env,
    },
    body: JSON.stringify({
      payload: { player_id: "player_123", duration: 24 },
    }),
  },
);

const result = await response.json();
```

### Python

```python
import requests

response = requests.post(
    'http://localhost:18780/api/v1/functions/player.ban/invoke',
    headers={
        'Content-Type': 'application/json',
        'Authorization': f'Bearer {token}',
        'X-Game-ID': game_id,
        'X-Env': env,
    },
    json={'payload': {'player_id': 'player_123', 'duration': 24}},
)

result = response.json()
```

## 相关文档

- [API 概览](./)
- [页面与控制台 API](./page.md)
