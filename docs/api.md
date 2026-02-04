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

### 公共端点

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/functions/descriptors` | 获取所有函数描述符 |
| GET | `/functions/descriptors/:id` | 获取单个函数描述符 |
| GET | `/functions/instances` | 获取函数实例列表 |
| POST | `/functions/validate` | 验证函数调用请求 |
| GET | `/providers/capabilities` | 获取 Provider 能力 |
| GET | `/providers/descriptors` | 获取 Provider 描述符 |
| GET | `/packs` | 获取函数包列表 |
| POST | `/packs/reload` | 重新加载函数包 |
| GET | `/packs/export` | 导出函数包 |

### 请求头

```
X-Game-ID: mygame       # 游戏 ID (多租户必需)
X-Env: prod             # 环境 (可选)
Authorization: Bearer <token>  # 认证令牌
```

### 响应格式

成功响应:
```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

错误响应:
```json
{
  "code": 10001,
  "message": "function not found",
  "details": { ... }
}
```

---

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

---

## 参考资料

- **Proto 定义**: `proto/` 目录
- **服务实现**: `services/server/`, `services/agent/`
- **HTTP 路由**: `services/server/internal/handler/routes.go`
- **客户端 SDK**: `sdks/go/`, `sdks/cpp/`, `sdks/java/`
