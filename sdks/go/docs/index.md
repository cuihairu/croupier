---
home: true
title: Croupier Go SDK
titleTemplate: false
heroImage: /logo.png
heroText: Croupier Go SDK
tagline: 高性能 Go SDK，用于 Croupier 游戏函数注册与执行系统
actions:
  - text: 快速开始
    link: /guide/quick-start.html
    type: primary
  - text: 安装指南
    link: /guide/installation.html
    type: secondary
features:
  - title: 📡 Proto 对齐
    details: 所有类型与官方 Croupier proto 定义保持一致
  - title: 🔧 双构建系统
    details: 本地开发使用 Mock 实现，CI/生产使用真实 gRPC
  - title: 🏢 多租户支持
    details: 内置 game_id/env 隔离机制
  - title: 📝 函数注册
    details: 使用描述符和处理器注册游戏函数
  - title: 🚀 gRPC 通信
    details: 与 Agent 的高效双向通信
  - title: 🛡️ 错误处理
    details: 完善的错误处理和连接管理

footer: Apache License 2.0 | Copyright © 2024 Croupier
---

## 📋 简介

Croupier Go SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 Go 客户端实现。

## 🚀 快速开始

### 安装

```bash
go get github.com/cuihairu/croupier/sdks/go
```

### 基础使用

```go
package main

import (
    "context"
    "log"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    config := &croupier.ClientConfig{
        AgentAddr: "localhost:19090",
        GameID:    "my-game",
        Env:       "development",
        Insecure:  true,
    }

    client := croupier.NewClient(config)

    desc := croupier.FunctionDescriptor{
        ID:      "player.ban",
        Version: "0.1.0",
    }

    handler := func(ctx context.Context, payload string) (string, error) {
        return `{"status":"success"}`, nil
    }

    client.RegisterFunction(desc, handler)
    client.Serve(context.Background())
}
```

## 🔗 相关链接

- [主项目](https://github.com/cuihairu/croupier)
- [C++ SDK](https://github.com/cuihairu/croupier-sdk-cpp)
- [Java SDK](https://github.com/cuihairu/croupier-sdk-java)
- [JavaScript SDK](https://github.com/cuihairu/croupier-sdk-js)
- [Python SDK](https://github.com/cuihairu/croupier-sdk-python)
