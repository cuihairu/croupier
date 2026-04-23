# 使用示例

本章节提供 Croupier Go SDK 的使用示例。

## 基础示例

### main.go

```go
package main

import (
    "context"
    "encoding/json"
    "log"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    config := &croupier.ClientConfig{
        AgentAddr: "localhost:19090",
        GameID:    "demo-game",
        Env:       "development",
        Insecure:  true,
    }

    client := croupier.NewClient(config)

    // 注册函数
    desc := croupier.FunctionDescriptor{
        ID:      "hello.world",
        Version: "0.1.0",
        Enabled: true,
    }

    handler := func(ctx context.Context, payload string) (string, error) {
        return `{"message":"Hello from Go!"}`, nil
    }

    if err := client.RegisterFunction(desc, handler); err != nil {
        log.Fatal(err)
    }

    // 启动服务
    log.Println("Starting server...")
    if err := client.Serve(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### 运行

```bash
go run main.go
```

## 综合示例

详见 [examples/comprehensive](https://github.com/cuihairu/croupier-sdk-go/tree/main/examples/comprehensive)。

## 游戏后台 Demo 示例

详见 [examples/game_demo](https://github.com/cuihairu/croupier-sdk-go/tree/main/examples/game_demo)。

这个示例适合部署在 Croupier 自托管 Demo 环境中，特点是：

- 常驻运行，不会像 `comprehensive` 那样演示完成后主动退出
- 覆盖玩家 CRUD、订单 CRUD、排行榜、背包、邮件等典型游戏运营场景
- 内置内存态示例数据，方便 Dashboard 直接看到已注册函数并立刻调用

## 更多示例

- [基础示例](https://github.com/cuihairu/croupier-sdk-go/tree/main/examples/basic)
- [游戏后台 Demo 示例](https://github.com/cuihairu/croupier-sdk-go/tree/main/examples/game_demo)
- [综合示例](https://github.com/cuihairu/croupier-sdk-go/tree/main/examples/comprehensive)
