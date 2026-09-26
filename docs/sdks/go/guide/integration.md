# Croupier Go SDK 集成指南

## 安装

```bash
go get github.com/cuihairu/croupier/sdks/go
```

## 最小示例

```go
package main

import (
    "context"
    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    client := croupier.NewClient(&croupier.ClientConfig{
        AgentAddr: "localhost:19091",
        GameID:    "demo-game",
        Env:       "development",
        Insecure:  true,
    })

    _ = client.RegisterFunction(croupier.FunctionDescriptor{
        ID: "hello.world", Version: "0.1.0", Enabled: true,
    }, func(ctx context.Context, payload []byte) ([]byte, error) {
        // 契约签名：FunctionHandler = func(ctx, payload []byte) ([]byte, error)
        return []byte(`{"message":"Hello from Go!"}`), nil
    })
}
```

## 实例元数据（provider 自描述）

注册时可携带**用户自定义多 KV 实例元数据**（如 `serverId`、`pod`），随 `ProviderConnectRequest.metadata` 上报，用于控制台「SDK 版本分布」页展示实例信息与按元数据搜索（REST `GET /api/v1/providers/sdk-stats?metaKey=&metaValue=`）：

```go
client := croupier.NewClient(&croupier.ClientConfig{
    // ...
    InstanceMetadata: map[string]string{
        "serverId": "s1",
        "pod":      "game-7c4d",
    },
})
```

- 保留键 `sdkLanguage` / `sdkVersion` / `sdkName` / `protocol_version` / `gameId` / `env` 由平台固定字段生成；用户元数据撞键时 agent 丢弃该键并在注册响应 `warnings` 中告警。
- 元数据仅用于观测（展示/搜索/诊断），不参与路由与负载均衡。
- wire 语义与存储设计结论见 `docs/architecture/sdk-wire-protocol.md`「实例元数据」。
