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
        // Contract signature: FunctionHandler = func(ctx, payload []byte) ([]byte, error)
        return []byte(`{"message":"Hello from Go!"}`), nil
    })
}
```

## 实例元数据（Provider 自描述）

注册时可以附加**用户自定义的多 KV 实例元数据**（如 `serverId`、`pod`）。它通过 `ProviderConnectRequest.metadata` 上报，控制台的 “SDK Version Distribution” 页面用它展示实例信息，并支持基于元数据的检索（REST `GET /api/v1/providers/sdk-stats?metaKey=&metaValue=`）：

```go
client := croupier.NewClient(&croupier.ClientConfig{
    // ...
    InstanceMetadata: map[string]string{
        "serverId": "s1",
        "pod":      "game-7c4d",
    },
})
```

- 保留键 `sdkLanguage` / `sdkVersion` / `sdkName` / `protocol_version` / `gameId` / `env` 由平台固定字段生成；当用户元数据与保留键冲突时，agent 会丢弃该键，并在注册响应的 `warnings` 中给出告警。
- 元数据仅用于可观测性（展示/检索/诊断）；不参与路由与负载均衡。
- demo（`sdks/go/examples/demo`）读取 `CROUPIER_INSTANCE_METADATA` 环境变量，接受 JSON 对象或 `k=v,k=v` 简写；空值表示不附加元数据：
  ```bash
  CROUPIER_INSTANCE_METADATA='serverId=s1,pod=game-7c4d'
  # Equivalent: CROUPIER_INSTANCE_METADATA='{"serverId":"s1","pod":"game-7c4d"}'
  ```
- wire 语义与存储设计决策见 `docs/architecture/sdk-wire-protocol.md` 的 “Instance Metadata” 一节。
