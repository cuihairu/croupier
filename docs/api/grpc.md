---
title: gRPC API
icon: network-wired
order: 2
category:
  - API 参考
tag:
  - gRPC
  - API
  - Protobuf
---

# gRPC API

Croupier 使用 gRPC 作为服务间通信协议，提供高性能、类型安全的 API。

## 目录

[[toc]]

## 服务列表

| 服务 | 端口 | 用途 |
|------|------|------|
| `ControlService` | 8443 | Agent 注册与连接管理 |
| `FunctionService` | 8443 | 函数调用与作业管理 |
| `RegistryService` | 8443 | 函数注册与发现 |

## 连接配置

### mTLS 配置

```go
creds, err := credentials.NewClientTLSFromFile(
    "ca.crt",
    "server.example.com",
)
if err != nil {
    log.Fatal(err)
}

conn, err := grpc.Dial("server.example.com:8443",
    grpc.WithTransportCredentials(creds),
)
```

### 客户端配置

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "time"
)

// 创建连接
conn, err := grpc.Dial("server:8443",
    // mTLS 凭证
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
    // 保持连接
    grpc.WithKeepaliveParams(keepalive.ClientParameters{
        Time:                10 * time.Second,
        Timeout:             time.Second,
        PermitWithoutStream: true,
    }),
    // 消息大小限制
    grpc.WithDefaultCallOptions(
        grpc.MaxCallRecvMsgSize(10 * 1024 * 1024),
        grpc.MaxCallSendMsgSize(10 * 1024 * 1024),
    ),
)
```

## ControlService

Agent 注册与连接管理。

### RegisterAgent

注册新 Agent 到 Server。

```protobuf
rpc RegisterAgent(RegisterAgentRequest) returns (RegisterAgentResponse);
```

**请求**：
```protobuf
message RegisterAgentRequest {
  string agent_id = 1;       // Agent 唯一标识
  string game_id = 2;        // 游戏 ID
  string env = 3;            // 环境
  string version = 4;        // Agent 版本
  repeated string functions = 5;  // 支持的函数列表
  map<string, string> labels = 6;  // 标签
}
```

**响应**：
```protobuf
message RegisterAgentResponse {
  bool success = 1;
  string agent_id = 2;
  int64 heartbeat_interval = 3;  // 心跳间隔（秒）
}
```

### Heartbeat

Agent 心跳保活。

```protobuf
rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
```

**请求**：
```protobuf
message HeartbeatRequest {
  string agent_id = 1;
  int64 timestamp = 2;
  map<string, string> status = 3;  // Agent 状态
}
```

**响应**：
```protobuf
message HeartbeatResponse {
  bool success = 1;
}
```

### GetAssignments

获取分配给 Agent 的配置。

```protobuf
rpc GetAssignments(GetAssignmentsRequest) returns (GetAssignmentsResponse);
```

**响应**：
```protobuf
message GetAssignmentsResponse {
  repeated FunctionAssignment assignments = 1;
  int64 version = 2;  // 配置版本
}

message FunctionAssignment {
  string function_id = 1;
  bool enabled = 2;
  map<string, string> config = 3;
}
```

## FunctionService

函数调用与作业管理。

### InvokeFunction

同步调用函数。

```protobuf
rpc InvokeFunction(InvokeFunctionRequest) returns (InvokeFunctionResponse);
```

**请求**：
```protobuf
message InvokeFunctionRequest {
  string game_id = 1;
  string env = 2;
  string function_id = 3;
  google.protobuf.Struct payload = 4;
  InvokeOptions options = 5;
}

message InvokeOptions {
  string idempotency_key = 1;  // 幂等键
  int32 timeout = 2;           // 超时（秒）
  string routing_mode = 3;     // 路由模式
  string target_agent = 4;     // 目标 Agent
}
```

**响应**：
```protobuf
message InvokeFunctionResponse {
  bool success = 1;
  google.protobuf.Struct result = 2;
  string error = 3;
  string error_code = 4;
}
```

### InvokeJob

异步调用函数（创建作业）。

```protobuf
rpc InvokeJob(InvokeJobRequest) returns (InvokeJobResponse);
```

**响应**：
```protobuf
message InvokeJobResponse {
  bool success = 1;
  string job_id = 2;
  string status = 3;  // pending, running, completed, failed
}
```

### StreamJobEvents

流式获取作业事件。

```protobuf
rpc StreamJobEvents(StreamJobEventsRequest) returns (stream JobEvent);
```

**事件**：
```protobuf
message JobEvent {
  string job_id = 1;
  EventType type = 2;
  string message = 3;
  double progress = 4;  // 0.0 - 1.0
  int64 timestamp = 5;
  google.protobuf.Struct data = 6;  // 附加数据
}

enum EventType {
  START = 0;
  PROGRESS = 1;
  LOG = 2;
  DONE = 3;
  ERROR = 4;
}
```

### CancelJob

取消正在执行的作业。

```protobuf
rpc CancelJob(CancelJobRequest) returns (CancelJobResponse);
```

**请求**：
```protobuf
message CancelJobRequest {
  string job_id = 1;
  string reason = 2;
}
```

### RegisterFunction

注册函数到 Server。

```protobuf
rpc RegisterFunction(RegisterFunctionRequest) returns (RegisterFunctionResponse);
```

**请求**：
```protobuf
message RegisterFunctionRequest {
  string game_id = 1;
  string env = 2;
  FunctionDescriptor descriptor = 3;
}

message FunctionDescriptor {
  string id = 1;
  string name = 2;
  string category = 3;
  google.protobuf.Struct params_schema = 4;
  google.protobuf.Struct result_schema = 5;
  AuthConfig auth = 6;
  Semantics semantics = 7;
  UIConfig ui = 8;
}
```

## RegistryService

函数注册与发现。

### GetRegistrations

获取函数注册信息。

```protobuf
rpc GetRegistrations(GetRegistrationsRequest) returns (GetRegistrationsResponse);
```

**请求**：
```protobuf
message GetRegistrationsRequest {
  string game_id = 1;
  string env = 2;
  string function_id = 3;  // 可选，查询特定函数
}
```

**响应**：
```protobuf
message GetRegistrationsResponse {
  repeated FunctionRegistration registrations = 1;
}

message FunctionRegistration {
  string function_id = 1;
  string game_id = 2;
  string env = 3;
  repeated AgentInfo agents = 4;  // 提供该函数的 Agent
}

message AgentInfo {
  string agent_id = 1;
  string addr = 2;
  int64 last_heartbeat = 3;
  map<string, string> labels = 4;
}
```

### WatchRegistrations

监听注册变化。

```protobuf
rpc WatchRegistrations(WatchRegistrationsRequest) returns (stream RegistrationEvent);
```

**事件**：
```protobuf
message RegistrationEvent {
  EventType type = 1;  // ADDED, UPDATED, REMOVED
  FunctionRegistration registration = 2;
}
```

## 错误处理

### gRPC 状态码

| 状态码 | 说明 | HTTP 映射 |
|--------|------|----------|
| `OK` | 成功 | 200 |
| `INVALID_ARGUMENT` | 参数错误 | 400 |
| `UNAUTHENTICATED` | 未认证 | 401 |
| `PERMISSION_DENIED` | 权限不足 | 403 |
| `NOT_FOUND` | 资源不存在 | 404 |
| `ALREADY_EXISTS` | 资源已存在 | 409 |
| `RESOURCE_EXHAUSTED` | 限流 | 429 |
| `INTERNAL` | 内部错误 | 500 |
| `UNAVAILABLE` | 服务不可用 | 503 |

### 错误详情

```protobuf
message ErrorInfo {
  string code = 1;       // 错误码
  string message = 2;    // 错误消息
  map<string, string> details = 3;  // 错误详情
}
```

## 客户端示例

### Go 客户端

```go
package main

import (
    "context"
    "log"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

func main() {
    // 创建连接
    conn, err := grpc.Dial("server:8443",
        grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // 创建客户端
    client := NewFunctionServiceClient(conn)

    // 调用函数
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    resp, err := client.InvokeFunction(ctx, &InvokeFunctionRequest{
        GameId:     "my-game",
        Env:        "prod",
        FunctionId: "player.ban",
        Payload:    structpb.NewStructValue(payload),
        Options: &InvokeOptions{
            IdempotencyKey: "unique-key-123",
        },
    })

    if err != nil {
        log.Printf("调用失败: %v", err)
        return
    }

    log.Printf("调用成功: %+v", resp.Result)
}
```

### C++ 客户端

```cpp
#include <croupier/sdk/client.h>
#include <grpc/grpc.h>
#include <grpc++/channel.h>

int main() {
    // 创建通道
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

    // 创建客户端
    croupier::FunctionServiceClient client(channel);

    // 调用函数
    grpc::ClientContext context;
    context.set_deadline(
        std::chrono::system_clock::now() + std::chrono::seconds(30)
    );

    croupier::InvokeFunctionRequest request;
    request.set_game_id("my-game");
    request.set_env("prod");
    request.set_function_id("player.ban");
    request.mutable_payload()->CopyFrom(payload);

    auto response = client.InvokeFunction(&context, request);

    if (response.ok()) {
        std::cout << "调用成功" << std::endl;
    } else {
        std::cout << "调用失败: " << response.error_message() << std::endl;
    }

    return 0;
}
```

## 拦截器

### 日志拦截器

```go
func loggingInterceptor(
    ctx context.Context,
    method string,
    req, reply interface{},
    cc *grpc.ClientConn,
    invoker grpc.UnaryInvoker,
    opts ...grpc.CallOption,
) error {
    start := time.Now()
    err := invoker(ctx, method, req, reply, cc, opts...)
    log.Printf("调用 %s 耗时 %v", method, time.Since(start))
    return err
}
```

### 重试拦截器

```go
func retryInterceptor(maxRetries int) grpc.UnaryClientInterceptor {
    return func(
        ctx context.Context,
        method string,
        req, reply interface{},
        cc *grpc.ClientConn,
        invoker grpc.UnaryInvoker,
        opts ...grpc.CallOption,
    ) error {
        var lastErr error
        for i := 0; i < maxRetries; i++ {
            lastErr = invoker(ctx, method, req, reply, cc, opts...)
            if lastErr == nil {
                return nil
            }
            time.Sleep(time.Second * time.Duration(i+1))
        }
        return lastErr
    }
}
```

## 相关文档

- [REST API](./rest.md)
- [Proto 选项](./proto-options.md)
- [API 概览](./README.md)
