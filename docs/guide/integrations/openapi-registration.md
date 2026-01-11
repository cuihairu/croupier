---
title: OpenAPI 函数注册
icon: code
order: 5
category:
  - 集成指南
tag:
  - OpenAPI
  - 函数注册
  - Agent
---

# OpenAPI 函数注册

## 概述

Croupier Agent 支持通过 OpenAPI 3.0.3 规范格式注册函数。游戏服务器可以通过 `LocalControlService.RegisterLocal` API 向 Agent 注册函数，函数描述符完全兼容 OpenAPI 3.0.3 Operation Object 字段。

## 协议定义

```protobuf
// 基于 OpenAPI 3.0.3 Operation Object 字段
message LocalFunctionDescriptor {
  string id = 1;                          // 唯一函数标识符
  string version = 2;                     // 函数版本

  // OpenAPI 3.0.3 Operation Object 字段
  repeated string tags = 3;               // 标签（用于分组操作）
  string summary = 4;                     // 简短摘要
  string description = 5;                 // 详细描述（支持 Markdown）
  string operation_id = 6;                // 唯一操作 ID
  bool deprecated = 7;                    // 废弃状态
}

message RegisterLocalRequest {
  string service_id = 1;                  // 服务标识（如 "game-server"）
  string version = 2;                     // 服务版本
  string rpc_addr = 3;                    // RPC 地址（可选，用于回调）
  repeated LocalFunctionDescriptor functions = 4;  // 函数列表
}

service LocalControlService {
  rpc RegisterLocal(RegisterLocalRequest) returns (RegisterLocalResponse);
}
```

## OpenAPI 3.0.3 字段映射

| Croupier 字段 | OpenAPI 3.0.3 字段 | 说明 |
|---------------|-------------------|------|
| `id` | 自定义 | 函数唯一标识，格式如 `game.player.ban` |
| `version` | 自定义 | 函数版本号 |
| `tags` | `tags` | 用于分组操作的标签数组 |
| `summary` | `summary` | 简短摘要字符串 |
| `description` | `description` | 详细描述，支持 Markdown 语法 |
| `operation_id` | `operationId` | 唯一操作 ID |
| `deprecated` | `deprecated` | 标记该函数是否已废弃 |

## 注册流程

```
┌─────────────┐                              ┌─────────────┐
│Game Server  │                              │  Croupier   │
│             │                              │    Agent    │
└──────┬──────┘                              └──────┬──────┘
       │                                            │
       │ 1. RegisterLocal(functions[])            │
       │──────────────────────────────────────────►│
       │                                            │
       │ 2. session_id                             │
       │◄───────────────────────────────────────────│
       │                                            │
       │ 3. Heartbeat(session_id)  [定期]           │
       │──────────────────────────────────────────►│
       │                                            │
       │                    ┌────────────────────────┤
       │                    │  Functions Registered: │
       │                    │  - game.player.ban      │
       │                    │  - game.player.kick     │
       │                    │  - game.item.give      │
       │                    └────────────────────────┘
```

## 使用示例

### Go SDK

```go
import (
    localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
)

func RegisterFunctions(conn *grpc.ClientConn) error {
    client := localv1.NewLocalControlServiceClient(conn)

    resp, err := client.RegisterLocal(context.Background(), &localv1.RegisterLocalRequest{
        ServiceId: "game-server",
        Version:   "1.0.0",
        RpcAddr:   "localhost:9090",
        Functions: []*localv1.LocalFunctionDescriptor{
            {
                Id:          "game.player.ban",
                Version:     "1.0.0",
                Tags:        []string{"player", "moderation"},
                Summary:     "封禁玩家",
                Description: "封禁指定玩家账号，禁止登录\n\n**参数**:\n- `player_id`: 玩家 ID\n- `reason`: 封禁原因\n- `duration`: 封禁时长（秒）",
                OperationId: "banPlayer",
                Deprecated:  false,
            },
            {
                Id:          "game.player.kick",
                Version:     "1.0.0",
                Tags:        []string{"player"},
                Summary:     "踢出玩家",
                Description: "将玩家从游戏中踢出",
                OperationId: "kickPlayer",
            },
        },
    })

    if err != nil {
        return err
    }

    fmt.Printf("Registered with session: %s\n", resp.SessionId)
    return nil
}
```

### C++ SDK

```cpp
#include <croupier/agent/local/v1/local.pb.h>
#include <grpc/grpc.h>

void RegisterFunctions(std::shared_ptr<grpc::Channel> channel) {
    auto stub = croupier::agent::local::v1::LocalControlService::NewStub(channel);

    croupier::agent::local::v1::RegisterLocalRequest request;
    request.set_service_id("game-server");
    request.set_version("1.0.0");

    // 添加函数描述
    auto* func = request.add_functions();
    func->set_id("game.player.ban");
    func->set_version("1.0.0");
    func->add_tags("player");
    func->add_tags("moderation");
    func->set_summary("封禁玩家");
    func->set_description("封禁指定玩家账号");
    func->set_operation_id("banPlayer");
    func->set_deprecated(false);

    grpc::ClientContext ctx;
    croupier::agent::local::v1::RegisterLocalResponse response;
    grpc::Status status = stub->RegisterLocal(&ctx, request, &response);

    if (status.ok()) {
        std::cout << "Session: " << response.session_id() << std::endl;
    }
}
```

### HTTP JSON 格式

如果你有 OpenAPI 3.0.3 JSON 文档，可以转换并注册：

```json
{
  "service_id": "game-server",
  "version": "1.0.0",
  "functions": [
    {
      "id": "game.player.ban",
      "version": "1.0.0",
      "tags": ["player", "moderation"],
      "summary": "封禁玩家",
      "description": "封禁指定玩家账号，禁止登录",
      "operation_id": "banPlayer",
      "deprecated": false
    }
  ]
}
```

## 心跳保活

注册成功后，需要定期发送心跳保持连接：

```protobuf
message HeartbeatRequest {
  string service_id = 1;
  string session_id = 2;
}
```

**建议心跳间隔**: 30 秒

## 函数调用

注册后的函数可以通过 Server 端调用：

```bash
# 通过 gRPC 调用
grpcurl -plaintext localhost:8443 \
  croupier.function.v1.FunctionService/InvokeFunction \
  -d '{
    "game_id": "my-game",
    "env": "prod",
    "function_id": "game.player.ban",
    "payload": {
      "player_id": "12345",
      "reason": "作弊",
      "duration": 86400
    }
  }'
```

## 完整示例

参考以下完整示例：

- [Go SDK 示例](https://github.com/cuihairu/croupier-sdk-go)
- [C++ SDK 示例](https://github.com/cuihairu/croupier-sdk-cpp)
- [游戏服务器示例](../examples/)

## 最佳实践

1. **使用有意义的 ID**: 格式建议为 `{category}.{entity}.{action}`，如 `game.player.ban`
2. **填写完整元数据**: `summary` 和 `description` 会在 Dashboard 中显示
3. **使用标签分组**: 通过 `tags` 字段将相关函数分组
4. **设置废弃标记**: 废弃的函数设置 `deprecated: true`
5. **保持心跳**: 定期发送心跳避免注册过期
