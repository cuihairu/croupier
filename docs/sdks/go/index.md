---
title: Go SDK
---

# Go SDK

高性能 Go SDK，用于连接 Croupier Agent、注册函数并处理调用。

## 代码位置

- `sdks/go`

## 特性

- 与 monorepo 中的 `proto/**` 保持演进一致
- 支持函数描述符、会话管理与错误处理
- 适合服务端高并发集成

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
		AgentAddr: "127.0.0.1:19090",
		GameID:    "my-game",
		Env:       "development",
	}
	_ = croupier.NewClient(cfg)
}
```

## 继续阅读

- [指南](/sdks/go/guide/)
- [API 参考](/sdks/go/api/)
- [示例](/sdks/go/examples/)
