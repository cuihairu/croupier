<p align="center">
  <h1 align="center">Croupier Go SDK</h1>
  <p align="center">
    <strong>高性能 Go SDK，用于 Croupier 游戏函数注册与执行系统</strong>
  </p>
</p>

<p align="center">
  <a href="https://github.com/cuihairu/croupier-sdk-go/actions/workflows/nightly.yml">
    <img src="https://github.com/cuihairu/croupier-sdk-go/actions/workflows/nightly.yml/badge.svg" alt="Nightly Build">
  </a>
  <a href="https://codecov.io/gh/cuihairu/croupier-sdk-go">
    <img src="https://codecov.io/gh/cuihairu/croupier-sdk-go/branch/main/graph/badge.svg" alt="Coverage">
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://go.dev/">
    <img src="https://img.shields.io/badge/Go-1.25+-00ADD8.svg" alt="Go Version">
  </a>
</p>

<p align="center">
  <a href="#支持平台">
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg" alt="Platform">
  </a>
  <a href="https://github.com/cuihairu/croupier">
    <img src="https://img.shields.io/badge/Main%20Project-Croupier-green.svg" alt="Main Project">
  </a>
</p>

---

## 📋 目录

- [简介](#简介)
- [主项目](#主项目)
- [其他语言 SDK](#其他语言-sdk)
- [支持平台](#支持平台)
- [核心特性](#核心特性)
- [快速开始](#快速开始)
- [使用示例](#使用示例)
- [架构设计](#架构设计)
- [API 参考](#api-参考)
- [开发指南](#开发指南)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

---

## 简介

Croupier Go SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 Go 客户端实现。它提供了与官方 Croupier proto 定义对齐的数据结构、双构建系统（本地开发 Mock 和 CI/生产环境真实 gRPC）以及多租户支持。

## 主项目

| 项目 | 描述 | 链接 |
|------|------|------|
| **Croupier** | 游戏后端平台主项目（包含 Proto 定义） | [cuihairu/croupier](https://github.com/cuihairu/croupier) |
| **Proto 文件** | 协议定义（Protobuf/gRPC） | [proto/](https://github.com/cuihairu/croupier/tree/main/proto) |

## 其他语言 SDK

| 语言 | 仓库 | Nightly | Release | Docs | Coverage |
| --- | --- | --- | --- | --- | --- |
| C++ | [croupier-sdk-cpp](https://github.com/cuihairu/croupier-sdk-cpp) | [![nightly](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-cpp/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-cpp)](https://github.com/cuihairu/croupier-sdk-cpp/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-cpp/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-cpp/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-cpp) |
| Java | [croupier-sdk-java](https://github.com/cuihairu/croupier-sdk-java) | [![nightly](https://github.com/cuihairu/croupier-sdk-java/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-java/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-java)](https://github.com/cuihairu/croupier-sdk-java/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-java/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-java/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-java) |
| JS/TS | [croupier-sdk-js](https://github.com/cuihairu/croupier-sdk-js) | [![nightly](https://github.com/cuihairu/croupier-sdk-js/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-js/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-js)](https://github.com/cuihairu/croupier-sdk-js/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-js/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-js/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-js) |
| Python | [croupier-sdk-python](https://github.com/cuihairu/croupier-sdk-python) | [![nightly](https://github.com/cuihairu/croupier-sdk-python/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-python/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-python)](https://github.com/cuihairu/croupier-sdk-python/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-python/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-python/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-python) |
| C# | [croupier-sdk-csharp](https://github.com/cuihairu/croupier-sdk-csharp) | [![nightly](https://github.com/cuihairu/croupier-sdk-csharp/actions/workflows/nightly.yml/badge.svg)](https://github.com/cuihairu/croupier-sdk-csharp/actions/workflows/nightly.yml) | [![release](https://img.shields.io/github/v/release/cuihairu/croupier-sdk-csharp)](https://github.com/cuihairu/croupier-sdk-csharp/releases) | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-csharp/) | [![codecov](https://codecov.io/gh/cuihairu/croupier-sdk-csharp/branch/main/graph/badge.svg)](https://codecov.io/gh/cuihairu/croupier-sdk-csharp) |
| Lua | [croupier-sdk-cpp](https://github.com/cuihairu/croupier-sdk-cpp) | - | - | [![docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://cuihairu.github.io/croupier-sdk-cpp/) | - |

## 支持平台

| 平台 | 架构 | 状态 |
|------|------|------|
| **Windows** | x64 | ✅ 支持 |
| **Linux** | x64, ARM64 | ✅ 支持 |
| **macOS** | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

- 📡 **Proto 对齐** - 所有类型与官方 Croupier proto 定义保持一致
- 🔧 **双构建系统** - 本地开发使用 Mock 实现，CI/生产使用真实 gRPC
- 🏢 **多租户支持** - 内置 game_id/env 隔离机制
- 📝 **函数注册** - 使用描述符和处理器注册游戏函数
- 🚀 **gRPC 通信** - 与 Agent 的高效双向通信
- 🛡️ **错误处理** - 完善的错误处理和连接管理

## 快速开始

### 系统要求

- **Go 1.25**
- **Protocol Buffers 编译器** (protoc)
- **Go protoc 插件**:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

### 安装

```bash
go get github.com/cuihairu/croupier/sdks/go
```

### Build Options

The SDK supports two build modes:

**1. Mock Mode (Default - No Dependencies)**
```bash
go build ./...
```

**2. Real gRPC Mode (Requires Proto Generation)**
```bash
# Generate gRPC code first
./generate_proto.sh

# Or use Makefile
make build-with-grpc
```

For detailed proto generation instructions, see [PROTO_GENERATION.md](PROTO_GENERATION.md).

### 基础使用

```go
package main

import (
    "context"
    "log"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
    // 创建客户端配置
    config := &croupier.ClientConfig{
        AgentAddr:      "localhost:19090",
        GameID:         "my-game",
        Env:            "development",
        ServiceID:      "my-service",
        ServiceVersion: "1.0.0",
        Insecure:       true, // 开发环境
    }

    // 创建客户端
    client := croupier.NewClient(config)

    // 注册函数
    desc := croupier.FunctionDescriptor{
        ID:        "player.ban",
        Version:   "1.0.0",
        Category:  "moderation",
        Risk:      "high",
        Entity:    "player",
        Operation: "update",
        Enabled:   true,
    }

    handler := func(ctx context.Context, payload string) (string, error) {
        // 处理函数调用
        return `{"status":"success"}`, nil
    }

    if err := client.RegisterFunction(desc, handler); err != nil {
        log.Fatal(err)
    }

    // 启动服务
    ctx := context.Background()
    if err := client.Serve(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### 常驻游戏后台 Demo

如果你要快速拉起一个可长期在线的示例 Provider，而不是只验证最小注册链路，优先使用：

```bash
go run ./examples/game_demo
```

`game_demo` 会常驻注册一组典型游戏后台函数，覆盖：

- 玩家：`player.create` `player.get` `player.update` `player.delete` `player.list`
- 订单：`order.create` `order.get` `order.update` `order.delete` `order.list`
- 排行榜：`leaderboard.list` `leaderboard.upsert` `leaderboard.reset`
- 背包：`inventory.list` `inventory.grant` `inventory.consume`
- 邮件：`mail.send` `mail.list` `mail.claim`

默认环境变量：

```bash
CROUPIER_AGENT_ADDR=127.0.0.1:19091
CROUPIER_GAME_ID=demo-game
CROUPIER_SERVICE_ID=game-demo-service
CROUPIER_LOCAL_LISTEN=127.0.0.1:19103
CROUPIER_ENV=development
```

## 使用示例

### 函数描述符

与 `control.proto` 对齐：

```go
type FunctionDescriptor struct {
    ID        string // 函数 ID，如 "player.ban"
    Version   string // 语义化版本，如 "1.2.0"
    Category  string // 分组类别
    Risk      string // "low"|"medium"|"high"
    Entity    string // 实体类型，如 "player"
    Operation string // "create"|"read"|"update"|"delete"
    Enabled   bool   // 是否启用
}
```

### 本地函数描述符

与 `agent/local/v1/local.proto` 对齐：

```go
type LocalFunctionDescriptor struct {
    ID      string // 函数 ID
    Version string // 函数版本
}
```

## 架构设计

### 数据流

```
Game Server → Go SDK → Agent → Croupier Server
```

SDK 实现两层注册系统：
1. **SDK → Agent**: 使用 `LocalControlService`（来自 `local.proto`）
2. **Agent → Server**: 使用 `ControlService`（来自 `control.proto`）

### 构建模式

**本地开发（Mock gRPC）：**
```bash
go build ./...
go run examples/basic/main.go
go run examples/game_demo/main.go
```

**CI/生产（真实 gRPC）：**
```bash
export CROUPIER_CI_BUILD=ON
go run scripts/generate_proto.go
go build -tags croupier_real_grpc ./...
```

CI 系统自动：
1. 从主仓库下载 proto 文件
2. 使用 protoc 生成 gRPC Go 代码
3. 使用真实 gRPC 实现构建
4. 运行测试和示例

## API 参考

### ClientConfig

```go
type ClientConfig struct {
    // 连接配置
    AgentAddr      string // Agent gRPC 地址
    LocalListen    string // 本地服务器地址
    TimeoutSeconds int    // 连接超时
    Insecure       bool   // 使用不安全的 gRPC

    // 多租户隔离
    GameID         string // 游戏标识符
    Env            string // 环境（dev/staging/prod）
    ServiceID      string // 服务标识符
    ServiceVersion string // 服务版本
    AgentID        string // Agent 标识符

    // TLS（非 insecure 模式）
    CAFile   string // CA 证书
    CertFile string // 客户端证书
    KeyFile  string // 私钥
}
```

### 错误处理

SDK 提供完善的错误处理：

- 连接失败自动重试
- 函数注册验证
- gRPC 通信错误
- 上下文取消时优雅关闭

## 开发指南

### 项目结构

```
croupier-sdk-go/
├── pkg/croupier/      # SDK 核心包
├── examples/          # 示例程序
├── scripts/           # 构建脚本
└── go.mod             # Go 模块定义
```

### 构建命令

```bash
# 本地开发（mock）
make build

# CI 构建（真实 gRPC）
make ci-build

# 运行测试
make test

# 手动生成 proto 代码
go run scripts/generate_proto.go
```

## 贡献指南

1. 确保所有类型与 proto 定义对齐
2. 为新功能添加测试
3. 更新 API 变更的文档
4. 测试本地和 CI 两种构建模式

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源协议。

---

<p align="center">
  <a href="https://github.com/cuihairu/croupier">🏠 主项目</a> •
  <a href="https://github.com/cuihairu/croupier-sdk-go/issues">🐛 问题反馈</a> •
  <a href="https://github.com/cuihairu/croupier/discussions">💬 讨论区</a>
</p>
