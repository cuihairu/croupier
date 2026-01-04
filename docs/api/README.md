---
title: API 概览
icon: code
order: 1
category:
  - API 参考
tag:
  - API
  - 接口
---

# API 概览

Croupier 提供两套 API：**gRPC API**（服务间通信）和 **REST API**（Web 调用）。

## API 类型

| 类型 | 协议 | 端口 | 用途 |
|------|------|------|------|
| gRPC | HTTP/2 + mTLS | 8443 | Server-Agent、Server-SDK 通信 |
| REST | HTTP/HTTPS | 8080 | Dashboard、外部调用 |

## gRPC API

### 基础配置

```protobuf
// 连接选项
{
  "address": "server.example.com:8443",
  "tls": {
    "ca_cert": "path/to/ca.crt",
    "client_cert": "path/to/client.crt",
    "client_key": "path/to/client.key",
    "server_name": "server.example.com"
  }
}
```

### 服务列表

#### Control Service (控制服务)

Agent 注册与连接管理。

```protobuf
service ControlService {
  // 注册 Agent
  rpc RegisterAgent(RegisterAgentRequest) returns (RegisterAgentResponse);

  // 心跳保活
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);

  // 获取分配配置
  rpc GetAssignments(GetAssignmentsRequest) returns (GetAssignmentsResponse);

  // 导出函数包
  rpc ExportPack(ExportPackRequest) returns (stream ExportPackChunk);
}
```

#### Function Service (函数服务)

函数调用与作业管理。

```protobuf
service FunctionService {
  // 调用函数（同步）
  rpc InvokeFunction(InvokeFunctionRequest) returns (InvokeFunctionResponse);

  // 调用函数（异步作业）
  rpc InvokeJob(InvokeJobRequest) returns (InvokeJobResponse);

  // 流式获取作业事件
  rpc StreamJobEvents(StreamJobEventsRequest) returns (stream JobEvent);

  // 取消作业
  rpc CancelJob(CancelJobRequest) returns (CancelJobResponse);

  // 注册函数
  rpc RegisterFunction(RegisterFunctionRequest) returns (RegisterFunctionResponse);

  // 批量注册函数
  rpc RegisterFunctions(RegisterFunctionsRequest) returns (RegisterFunctionsResponse);

  // 注销函数
  rpc UnregisterFunction(UnregisterFunctionRequest) returns (UnregisterFunctionResponse);

  // 获取函数列表
  rpc ListFunctions(ListFunctionsRequest) returns (ListFunctionsResponse);

  // 获取函数详情
  rpc GetFunction(GetFunctionRequest) returns (GetFunctionResponse);
}
```

#### Registry Service (注册中心)

函数注册与发现。

```protobuf
service RegistryService {
  // 获取函数注册信息
  rpc GetRegistrations(GetRegistrationsRequest) returns (GetRegistrationsResponse);

  // 监听注册变化
  rpc WatchRegistrations(WatchRegistrationsRequest) returns (stream RegistrationEvent);
}
```

### 调用示例

#### Go

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

conn, err := grpc.Dial("server:8443",
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
)

client := NewFunctionServiceClient(conn)

resp, err := client.InvokeFunction(ctx, &InvokeFunctionRequest{
    GameId:    "my-game",
    Env:       "prod",
    FunctionId: "player.ban",
    Payload:   structpb.NewStructValue(payload),
    Options: &InvokeOptions{
        IdempotencyKey: "unique-key-123",
        Timeout:        durationpb.New(30 * time.Second),
    },
})
```

#### C++

```cpp
#include <croupier/sdk/client.h>
#include <grpc/grpc.h>

auto tls_credentials = grpc::SslCredentials(
    grpc::SslCredentialsOptions{
        .pem_root_certs = ReadFile("ca.crt"),
        .pem_cert_chain = ReadFile("client.crt"),
        .pem_private_key = ReadFile("client.key"),
    }
);

auto channel = grpc::CreateChannel(
    "server:8443",
    tls_credentials
);

croupier::FunctionServiceClient client(channel);

auto response = client.InvokeFunction(
    croupier::InvokeFunctionRequest{}
        .set_game_id("my-game")
        .set_env("prod")
        .set_function_id("player.ban")
        .set_payload(payload)
);
```

## REST API

### 基础信息

- **Base URL**: `http://server:8080` 或 `https://server:8080`
- **Content-Type**: `application/json`
- **认证**: Bearer Token

### 通用请求头

```http
Authorization: Bearer {token}
X-Game-ID: {game_id}
X-Env: {env}
Content-Type: application/json
```

### 端点列表

#### 函数调用

```http
# 调用函数
POST /api/invoke
{
  "function_id": "player.ban",
  "payload": { ... },
  "idempotency_key": "optional-key"
}

# 调用函数（异步）
POST /api/jobs
{
  "function_id": "player.ban",
  "payload": { ... }
}

# 获取作业状态
GET /api/jobs/{job_id}

# 取消作业
DELETE /api/jobs/{job_id}

# 流式获取作业事件
GET /api/jobs/{job_id}/events
```

#### 函数管理

```http
# 获取函数列表
GET /api/functions?game_id={game_id}&env={env}

# 获取函数详情
GET /api/functions/{function_id}?game_id={game_id}&env={env}

# 注册函数
POST /api/functions
{
  "game_id": "my-game",
  "env": "prod",
  "descriptor": { ... }
}

# 注销函数
DELETE /api/functions/{function_id}?game_id={game_id}&env={env}
```

#### Agent 管理

```http
# 获取 Agent 列表
GET /api/agents?game_id={game_id}&env={env}

# 获取 Agent 详情
GET /api/agents/{agent_id}

# 获取 Agent 分配配置
GET /api/agents/{agent_id}/assignments
```

#### 审批流程

```http
# 创建审批请求
POST /api/approvals
{
  "function_id": "player.ban",
  "payload": { ... },
  "reason": "违规玩家处理"
}

# 获取审批列表
GET /api/approvals?status=pending

# 审批通过
POST /api/approvals/{approval_id}/approve

# 审批拒绝
POST /api/approvals/{approval_id}/reject
```

#### 审计日志

```http
# 查询审计日志
POST /api/audit/query
{
  "game_id": "my-game",
  "env": "prod",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-02T00:00:00Z",
  "filters": { ... }
}

# 获取审计详情
GET /api/audit/{audit_id}
```

### 错误响应

```json
{
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

### 错误码

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

## 相关文档

- [gRPC API 详情](./grpc.md)
- [REST API 详情](./rest.md)
- [Proto 选项指南](./proto-options.md)
