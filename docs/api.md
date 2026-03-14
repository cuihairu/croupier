# Croupier API 文档

本文档描述 Croupier 系统暴露的 gRPC 和 HTTP REST API。

> **注意**: 权威 API 定义在 `proto/` 目录中。本文档提供概览和使用指南。

## 目录

- [核心服务](#核心服务)
  - [ControlService](#controlservice) - Agent 注册与管理
  - [FunctionService](#functionservice) - 函数调用
  - [LocalControlService](#localcontrolservice) - 本地控制
- [HTTP REST API](#http-rest-api)
- [数据模型](#数据模型)
- [错误码](#错误码)

---

## 核心服务

### ControlService

**包**: `croupier.server.v1`

**功能**: Agent 注册、心跳、能力声明和函数目录查询

#### 方法

| 方法 | 请求 | 响应 | 描述 |
|------|------|------|------|
| `Register` | `RegisterRequest` | `RegisterResponse` | Agent 注册到 Server |
| `Heartbeat` | `HeartbeatRequest` | `HeartbeatResponse` | Agent 心跳保活 |
| `RegisterCapabilities` | `RegisterCapabilitiesRequest` | `RegisterCapabilitiesResponse` | 注册 Provider 能力清单 |
| `ListFunctionsSummary` | `google.protobuf.Empty` | `ListFunctionsSummaryResponse` | 获取函数目录摘要 |

#### 消息定义

```protobuf
// Agent 注册请求
message RegisterRequest {
  string agent_id = 1;              // Agent 唯一 ID
  string version = 2;               // Agent 版本
  repeated FunctionDescriptor functions = 3;  // 函数列表
  string rpc_addr = 4;              // Agent 可达 gRPC 地址
  string game_id = 5;               // 游戏 ID (多租户必需)
  string env = 6;                   // 环境 (prod/stage/test)
  repeated AgentProcess processes = 7;  // 注册的进程
}

// Agent 注册响应
message RegisterResponse {
  string session_id = 1;            // 会话 ID
  int64 expire_at = 2;              // 过期时间 (Unix 秒)
}

// 心跳请求
message HeartbeatRequest {
  string agent_id = 1;
  string session_id = 2;
}

// Provider 能力注册
message RegisterCapabilitiesRequest {
  ProviderMeta provider = 1;        // Provider 元信息
  bytes manifest_json_gz = 2;       // Gzip 压缩的 manifest.json
}

// 函数描述符（包含 UI/RBAC 元数据）
message FunctionDescriptor {
  string id = 1;                   // 函数 ID，如 "player.ban"
  string version = 2;              // SemVer，如 "1.2.0"
  string category = 3;             // 分组类别
  string risk = 4;                 // 风险级别: low/medium/high
  string entity = 5;               // 实体类型，如 "player"
  string operation = 6;            // 操作类型: create/read/update/delete
  bool enabled = 7;                // 是否启用

  // UI/i18n/权限元数据
  croupier.common.v1.I18nText display_name = 20;
  croupier.common.v1.I18nText summary = 21;
  repeated string tags = 22;
  croupier.common.v1.Menu menu = 23;
  croupier.common.v1.PermissionSpec permissions = 24;
}
```

#### 使用示例 (Go)

```go
import serverv1 "github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"

// Agent 注册
resp, err := controlClient.Register(ctx, &serverv1.RegisterRequest{
    AgentId: "agent-001",
    Version: "1.0.0",
    GameId:  "mygame",
    Env:     "prod",
    Functions: []*serverv1.FunctionDescriptor{
        {
            Id:      "player.ban",
            Version: "1.0.0",
            Enabled: true,
        },
    },
})
```

---

### FunctionService

**包**: `croupier.function.v1`

**功能**: 函数调用、异步作业、流式事件

#### 方法

| 方法 | 请求 | 响应 | 描述 |
|------|------|------|------|
| `Invoke` | `InvokeRequest` | `InvokeResponse` | 同步函数调用 |
| `StartJob` | `InvokeRequest` | `StartJobResponse` | 启动异步作业 |
| `StreamJob` | `JobStreamRequest` | `stream JobEvent` | 订阅作业事件流 |
| `CancelJob` | `CancelJobRequest` | `StartJobResponse` | 取消作业 |

#### 消息定义

```protobuf
// 函数调用请求
message InvokeRequest {
  string function_id = 1;           // 函数 ID
  string idempotency_key = 2;       // 幂等键（可选）
  bytes payload = 3;                // 请求载荷 (JSON/Proto)
  map<string, string> metadata = 4; // 元数据
}

// 函数调用响应
message InvokeResponse {
  bytes payload = 1;                // 响应载荷
}

// 作业事件
message JobEvent {
  string type = 1;                  // 事件类型: progress/log/done/error
  string message = 2;               // 消息内容
  int32 progress = 3;              // 进度 0-100 (type=progress 时)
  bytes payload = 4;               // 最终结果 (type=done 时)
}
```

#### 使用示例 (Go)

```go
// 同步调用
resp, err := functionClient.Invoke(ctx, &functionv1.InvokeRequest{
    FunctionId:     "player.ban",
    IdempotencyKey: "req-12345",
    Payload:       jsonPayload,
})

// 启动异步作业
jobResp, err := functionClient.StartJob(ctx, &functionv1.InvokeRequest{
    FunctionId: "player.mass_ban",
    Payload:    payload,
})

// 订阅作业事件流
stream, err := functionClient.StreamJob(ctx, &functionv1.JobStreamRequest{
    JobId: jobResp.JobId,
})
for {
    event, err := stream.Recv()
    if err == io.EOF {
        break
    }
    // 处理 event...
}
```

---

### LocalControlService

**包**: `croupier.agent.local.v1`

**功能**: Agent 本地控制面（SDK 到 Agent 的本地注册）

#### 方法

| 方法 | 描述 |
|------|------|
| `RegisterService` | SDK 注册本地服务到 Agent |
| `UnregisterService` | SDK 注销服务 |

---

## HTTP REST API

Server 同时暴露 HTTP REST API (端口 8080)，用于 Dashboard 和 Web 客户端。

### 基础路径

```
http://server:8080/api/v1
```

### 模块与端点（来自 `services/server/modules/*.api`）

#### admin

| 方法 | 路径 |
|------|------|
| GET | /api/v1/admin/ |
| POST | /api/v1/admin/ |
| GET | /api/v1/admin/:id |
| PUT | /api/v1/admin/:id |
| DELETE | /api/v1/admin/:id |
| POST | /api/v1/admin/:id/password-reset |
| GET | /api/v1/roles/ |
| POST | /api/v1/roles/ |
| GET | /api/v1/roles/:id |
| PUT | /api/v1/roles/:id |
| DELETE | /api/v1/roles/:id |
| GET | /api/v1/permissions/ |
| GET | /api/v1/permissions/:id |
| GET | /api/v1/admin/:id/games |
| PUT | /api/v1/admin/:id/games |

#### agent

| 方法 | 路径 |
|------|------|
| GET | /api/v1/agent/analytics-filters |
| POST | /api/v1/agent/meta |

#### alert

| 方法 | 路径 |
|------|------|
| GET | /api/v1/alerts/ |
| POST | /api/v1/alerts/:id/silence |
| GET | /api/v1/alerts/silences |
| DELETE | /api/v1/alerts/silences/:id |

#### analytics

| 方法 | 路径 |
|------|------|
| GET | /api/v1/analytics/overview |
| GET | /api/v1/analytics/realtime |
| GET | /api/v1/analytics/realtime/series |
| GET | /api/v1/analytics/behavior |
| GET | /api/v1/analytics/behavior/events |
| GET | /api/v1/analytics/behavior/funnel |
| GET | /api/v1/analytics/behavior/paths |
| GET | /api/v1/analytics/behavior/adoption |
| GET | /api/v1/analytics/behavior/adoption/breakdown |
| GET | /api/v1/analytics/payments |
| GET | /api/v1/analytics/payments/summary |
| GET | /api/v1/analytics/payments/transactions |
| GET | /api/v1/analytics/payments/product-trend |
| GET | /api/v1/analytics/levels |
| GET | /api/v1/analytics/levels/episodes |
| GET | /api/v1/analytics/levels/maps |
| GET | /api/v1/analytics/retention |
| POST | /api/v1/analytics/ingest |
| POST | /api/v1/analytics/payments/ingest |
| GET | /api/v1/analytics/filters |
| PUT | /api/v1/analytics/filters |

#### approval

| 方法 | 路径 |
|------|------|
| POST | /api/v1/approvals/:id/approve |
| GET | /api/v1/approvals/:id |
| POST | /api/v1/approvals/:id/reject |
| GET | /api/v1/approvals/ |

#### assignment

| 方法 | 路径 |
|------|------|
| GET | /api/v1/assignments/ |
| PUT | /api/v1/assignments/ |

#### audit

| 方法 | 路径 |
|------|------|
| GET | /api/v1/audit |

#### auth

| 方法 | 路径 |
|------|------|
| POST | /api/v1/auth/login |
| POST | /api/v1/auth/logout |
| POST | /api/v1/auth/check |
| POST | /api/v1/auth/check/batch |

#### backup

| 方法 | 路径 |
|------|------|
| GET | /api/v1/backups/ |
| POST | /api/v1/backups/ |
| DELETE | /api/v1/backups/:id |
| GET | /api/v1/backups/:id/download |

#### certificate

| 方法 | 路径 |
|------|------|
| GET | /api/v1/certificates/ |
| POST | /api/v1/certificates/ |
| GET | /api/v1/certificates/:id |
| POST | /api/v1/certificates/:id/check |
| DELETE | /api/v1/certificates/:id |
| GET | /api/v1/certificates/stats |
| POST | /api/v1/certificates/alerts |
| GET | /api/v1/certificates/alerts |
| POST | /api/v1/certificates/check-all |
| GET | /api/v1/certificates/domain-info |
| GET | /api/v1/certificates/expiring |

#### component

| 方法 | 路径 |
|------|------|
| GET | /api/v1/components/ |
| POST | /api/v1/components/install |
| GET | /api/v1/components/:id |
| POST | /api/v1/components/:id/enable |
| POST | /api/v1/components/:id/disable |
| DELETE | /api/v1/components/:id |
| PATCH | /api/v1/components/:id |

#### config

| 方法 | 路径 |
|------|------|
| POST | /api/v1/configs/ |
| GET | /api/v1/configs/version |
| GET | /api/v1/configs/versions |

#### entity

| 方法 | 路径 |
|------|------|
| GET | /api/v1/entities/ |
| POST | /api/v1/entities/ |
| GET | /api/v1/entities/:id |
| PUT | /api/v1/entities/:id |
| DELETE | /api/v1/entities/:id |
| GET | /api/v1/entities/:id/preview |
| POST | /api/v1/entities/validate |

#### faq

| 方法 | 路径 |
|------|------|
| GET | /api/v1/faqs/ |
| POST | /api/v1/faqs/ |
| PUT | /api/v1/faqs/:id |
| DELETE | /api/v1/faqs/:id |
| GET | /api/v1/faqs/categories |

#### feedback

| 方法 | 路径 |
|------|------|
| GET | /api/v1/feedback/ |
| POST | /api/v1/feedback/ |
| PUT | /api/v1/feedback/:id |
| DELETE | /api/v1/feedback/:id |
| GET | /api/v1/feedback/stats |

#### function

| 方法 | 路径 |
|------|------|
| GET | /api/v1/functions/ |
| GET | /api/v1/functions/:id |
| POST | /api/v1/functions/:id/enable |
| POST | /api/v1/functions/:id/disable |
| POST | /api/v1/functions/:id/copy |
| DELETE | /api/v1/functions/:id |
| POST | /api/v1/functions/:id/invoke |
| POST | /api/v1/functions/:id/publish |
| GET | /api/v1/functions/:id/instances |
| GET | /api/v1/functions/instances |
| GET | /api/v1/functions/:id/permissions |
| PUT | /api/v1/functions/:id/permissions |
| GET | /api/v1/functions/:id/ui |
| PUT | /api/v1/functions/:id/ui |
| GET | /api/v1/functions/descriptors |
| GET | /api/v1/functions/pending |
| POST | /api/v1/functions/batch-update |
| POST | /api/v1/functions/batch-copy |
| POST | /api/v1/functions/batch-delete |
| POST | /api/v1/functions/_openapi-batch |

#### function-call

| 方法 | 路径 |
|------|------|
| GET | /api/v1/function-calls |
| GET | /api/v1/function-calls/:id |
| GET | /api/v1/function-calls/stats |
| POST | /api/v1/function-calls/:id/rerun |
| POST | /api/v1/function-calls/:id/cancel |

#### game

| 方法 | 路径 |
|------|------|
| GET | /api/v1/games |
| POST | /api/v1/games |
| GET | /api/v1/games/ |
| POST | /api/v1/games/ |
| GET | /api/v1/games/:id |
| PUT | /api/v1/games/:id |
| DELETE | /api/v1/games/:id |
| GET | /api/v1/games/:id/envs |
| POST | /api/v1/games/:id/envs |
| PUT | /api/v1/games/:id/envs/:envId |
| DELETE | /api/v1/games/:id/envs/:envId |

#### job

| 方法 | 路径 |
|------|------|
| GET | /api/v1/jobs/ |
| POST | /api/v1/jobs/ |
| POST | /api/v1/jobs/:id/cancel |
| GET | /api/v1/jobs/:id/result |
| GET | /api/v1/jobs/:jobId/stream |

#### message

| 方法 | 路径 |
|------|------|
| GET | /api/v1/messages/ |
| POST | /api/v1/messages/ |
| GET | /api/v1/messages/:id |
| POST | /api/v1/messages/:id/read |
| GET | /api/v1/messages/unread-count |
| GET | /api/v1/messages/stream |

#### meta

| 方法 | 路径 |
|------|------|
| GET | /api/v1/ |

#### migrate

| 方法 | 路径 |
|------|------|
| GET | /api/v1/migrate/status |
| GET | /api/v1/migrate/history |
| POST | /api/v1/migrate/up |
| POST | /api/v1/migrate/down |

#### monitoring

| 方法 | 路径 |
|------|------|
| GET | /api/v1/healthz |
| GET | /api/v1/status |
| GET | /api/v1/metrics |

#### node

| 方法 | 路径 |
|------|------|
| GET | /api/v1/nodes/ |
| GET | /api/v1/nodes/:id/meta |
| PUT | /api/v1/nodes/:id/meta |
| POST | /api/v1/nodes/:id/drain |
| POST | /api/v1/nodes/:id/undrain |
| POST | /api/v1/nodes/:id/restart |
| GET | /api/v1/nodes/commands |

#### openapi

| 方法 | 路径 |
|------|------|
| GET | /api/v1/functions/:id/openapi |
| POST | /api/v1/functions/_import |
| GET | /api/v1/entities/:id/functions |

#### ops

| 方法 | 路径 |
|------|------|
| PUT | /api/v1/ops/agent-meta |
| GET | /api/v1/ops/alerts |
| POST | /api/v1/ops/alerts/silence |
| POST | /api/v1/ops/backups |
| DELETE | /api/v1/ops/backups/:id |
| GET | /api/v1/ops/backups/:id/download |
| GET | /api/v1/ops/backups |
| GET | /api/v1/ops/config |
| GET | /api/v1/ops/functions |
| GET | /api/v1/ops/health |
| POST | /api/v1/ops/health/run |
| PUT | /api/v1/ops/health |
| GET | /api/v1/ops/maintenance |
| PUT | /api/v1/ops/maintenance |
| GET | /api/v1/ops/metrics |
| GET | /api/v1/ops/mq |
| GET | /api/v1/ops/nodes/commands |
| POST | /api/v1/ops/nodes/:nodeId/drain |
| GET | /api/v1/ops/nodes/:nodeId/meta |
| POST | /api/v1/ops/nodes/:nodeId/restart |
| GET | /api/v1/ops/nodes |
| POST | /api/v1/ops/nodes/:nodeId/undrain |
| GET | /api/v1/ops/notifications |
| PUT | /api/v1/ops/notifications |
| GET | /api/v1/ops/services |
| DELETE | /api/v1/ops/silences/:id |
| GET | /api/v1/ops/silences |
| GET | /api/v1/ops/agents |
| GET | /api/v1/ops/agents/metrics |
| GET | /api/v1/ops/agents/:agentId/system-info |
| GET | /api/v1/ops/agents/:agentId/processes |
| POST | /api/v1/ops/agents/:agentId/processes/:name/restart |
| POST | /api/v1/ops/agents/:agentId/processes/:name/stop |
| POST | /api/v1/ops/agents/:agentId/processes/:name/start |
| POST | /api/v1/ops/agents/:agentId/exec |

#### pack

| 方法 | 路径 |
|------|------|
| GET | /api/v1/packs/export |
| POST | /api/v1/packs/import |
| GET | /api/v1/packs/ |
| POST | /api/v1/packs/reload |
| GET | /api/v1/packs/plugin |

#### platform

| 方法 | 路径 |
|------|------|
| POST | /api/v1/platforms/call |
| GET | /api/v1/platforms/ |
| GET | /api/v1/platforms/:platform/methods |
| POST | /api/v1/platforms/reload |

#### player

| 方法 | 路径 |
|------|------|
| GET | /api/v1/players/ |
| POST | /api/v1/players/ |
| GET | /api/v1/players/:id |
| PUT | /api/v1/players/:id |
| DELETE | /api/v1/players/:id |
| POST | /api/v1/players/:id/balance |

#### profile

| 方法 | 路径 |
|------|------|
| GET | /api/v1/profile/ |
| PUT | /api/v1/profile/ |
| PUT | /api/v1/profile/password |
| GET | /api/v1/profile/permissions |
| GET | /api/v1/profile/games |

#### provider

| 方法 | 路径 |
|------|------|
| GET | /api/v1/providers/ |
| GET | /api/v1/providers/capabilities |
| GET | /api/v1/providers/descriptors |
| GET | /api/v1/providers/:id |
| GET | /api/v1/providers/:id/entities |
| DELETE | /api/v1/providers/:id |
| POST | /api/v1/providers/:id/reload |

#### rate_limit

| 方法 | 路径 |
|------|------|
| GET | /api/v1/rate-limits/ |
| GET | /api/v1/rate-limits/:id |
| PUT | /api/v1/rate-limits/ |
| DELETE | /api/v1/rate-limits/:id |
| POST | /api/v1/rate-limits/preview |

#### registry

| 方法 | 路径 |
|------|------|
| GET | /api/v1/registry/ |
| GET | /api/v1/registry/services |

#### routes

| 方法 | 路径 |
|------|------|
| GET | /api/v1/routes |

#### schema

| 方法 | 路径 |
|------|------|
| GET | /api/v1/schemas/ |
| POST | /api/v1/schemas/ |
| GET | /api/v1/schemas/:id |
| PUT | /api/v1/schemas/:id |
| DELETE | /api/v1/schemas/:id |
| POST | /api/v1/schemas/:id/validate |
| POST | /api/v1/schemas/raw-validate |
| GET | /api/v1/schemas/:id/ui-config |
| PUT | /api/v1/schemas/:id/ui-config |

#### storage

| 方法 | 路径 |
|------|------|
| GET | /api/v1/storage/signed-url |
| GET | /api/v1/storage/objects |
| POST | /api/v1/storage/objects |
| DELETE | /api/v1/storage/objects |
| POST | /api/v1/storage/objects/batch-delete |
| POST | /api/v1/storage/directories |
| POST | /api/v1/storage/directories/rename |

#### support

| 方法 | 路径 |
|------|------|
| GET | /api/v1/support/tickets |
| POST | /api/v1/support/tickets |
| GET | /api/v1/support/tickets/:id |
| PUT | /api/v1/support/tickets/:id |
| DELETE | /api/v1/support/tickets/:id |
| POST | /api/v1/support/tickets/:id/transition |
| GET | /api/v1/support/tickets/:ticketId/comments |
| POST | /api/v1/support/tickets/:ticketId/comments |
| GET | /api/v1/support/faq |
| POST | /api/v1/support/faq |
| PUT | /api/v1/support/faq/:id |
| DELETE | /api/v1/support/faq/:id |
| GET | /api/v1/support/feedback |
| POST | /api/v1/support/feedback |
| PUT | /api/v1/support/feedback/:id |
| DELETE | /api/v1/support/feedback/:id |

#### ticket

| 方法 | 路径 |
|------|------|
| GET | /api/v1/tickets/ |
| POST | /api/v1/tickets/ |
| GET | /api/v1/tickets/:id |
| PUT | /api/v1/tickets/:id |
| DELETE | /api/v1/tickets/:id |
| POST | /api/v1/tickets/:id/transition |
| GET | /api/v1/tickets/:ticketId/comments |
| POST | /api/v1/tickets/:ticketId/comments |



## 数据模型

### I18nText (国际化文本)

```protobuf
message I18nText {
  string en = 1;  // 英文
  string zh = 2; // 中文
}
```

### Menu (菜单元数据)

```protobuf
message Menu {
  string section = 1;   // 菜单分区
  string group = 2;     // 菜单分组
  string path = 3;      // 路由路径
  int32 order = 4;      // 排序
  string icon = 5;      // 图标
  string badge = 6;     // 徽章
  bool hidden = 7;      // 是否隐藏
}
```

### PermissionSpec (权限规范)

```protobuf
message PermissionSpec {
  repeated string verbs = 1;           // 权限动词: read/invoke/write
  repeated string scopes = 2;          // 权限范围
  repeated RoleBinding defaults = 3;   // 默认角色绑定
  map<string, string> i18n_zh = 10;    // 中文国际化
}
```

---

## 错误码

| gRPC 状态 | HTTP 状态 | 描述 |
|-----------|-----------|------|
| `OK` | 200 | 成功 |
| `INVALID_ARGUMENT` | 400 | 请求参数无效 |
| `UNAUTHENTICATED` | 401 | 未认证 |
| `PERMISSION_DENIED` | 403 | 权限不足 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `ALREADY_EXISTS` | 409 | 资源已存在 |
| `RESOURCE_EXHAUSTED` | 429 | 请求过于频繁 |
| `INTERNAL` | 500 | 内部错误 |
| `UNAVAILABLE` | 503 | 服务不可用 |

---

## 认证与安全

### mTLS

所有服务间通信 (Server ↔ Agent) 强制使用 mTLS。

配置示例:
```yaml
tls:
  ca_file:   /etc/croupier/ca.crt
  cert_file: /etc/croupier/server.crt
  key_file:  /etc/croupier/server.key
```

### JWT Token

HTTP REST API 使用 JWT 认证。

```bash
# 登录获取 token
curl -X POST http://server:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "..."}'

# 使用 token 调用 API
curl http://server:8080/api/v1/functions/descriptors \
  -H "Authorization: Bearer <token>"
```

## 兼容性说明

- `/api/v1/games` 与 `/api/v1/games/` 现均可访问，供 Dashboard 兼容调用。
- `/api/v1/function-calls*` 当前是基于 `jobs` 的兼容视图，优先用于消除 refactor 后的前端 404。
- `/api/v1/function-calls/:id/rerun` 当前返回“暂不支持从调用历史重跑”，不会伪造重跑结果。
- `/api/v1/auth/check` 与 `/api/v1/auth/check/batch` 使用当前登录管理员在数据库中的角色与权限进行校验。

---

## 参考资料

- **Proto 定义**: `proto/` 目录
- **服务实现**: `services/server/`, `services/agent/`
- **HTTP 路由**: `services/server/internal/handler/routes.go`
- **客户端 SDK**: `sdks/go/`, `sdks/cpp/`, `sdks/java/`


