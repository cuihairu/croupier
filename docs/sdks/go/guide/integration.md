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
