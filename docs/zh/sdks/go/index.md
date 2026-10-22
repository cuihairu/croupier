---
title: Go SDK
---

# Go SDK

用于连接 Croupier Agent、注册函数并处理调用的 Go 高性能 SDK。

## 代码位置

- `sdks/go`

## 特性

- 与 monorepo 中的 `proto/**` 同步演进
- 支持函数描述符、session 管理与错误处理
- 适合高并发服务端接入场景

```go
metadata, _ := function.NewMetadataBuilder().
    SetID("player.ban").
    SetInputSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`).
    SetFieldWidget("id", "Select").
    SetFieldHint("id", "x-options-source", map[string]interface{}{
        "functionId": "player.list",
        "labelPath":  "/items/*/name",
        "valuePath":  "/items/*/id",
    }).
    Build()
```

## 安装

```bash
go get github.com/cuihairu/croupier/sdks/go
```

## 快速开始

```go
package main

import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"

func main() {
	cfg := &croupier.ClientConfig{
        AgentAddr: "127.0.0.1:19091",
		GameID:    "my-game",
		Env:       "development",
	}
	_ = croupier.NewClient(cfg)
}
```

## 延伸阅读

- [指南](/zh/sdks/go/guide/)
- [API 参考](/zh/sdks/go/api/)
- [示例](/zh/sdks/go/examples/)
