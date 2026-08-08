<p align="center">
  <h1 align="center">Croupier Go SDK</h1>
  <p align="center">
    <strong>高性能 Go SDK，用于 Croupier 游戏函数注册与执行系统</strong>
  </p>
</p>

<p align="center">
  <a href="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml">
    <img src="https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-go.yml/badge.svg" alt="CI Build">
  </a>
  <a href="https://codecov.io/gh/cuihairu/croupier">
    <img src="https://codecov.io/gh/cuihairu/croupier/branch/main/graph/badge.svg?flag=go-sdk" alt="Coverage">
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">
  </a>
  <a href="https://go.dev/">
    <img src="https://img.shields.io/badge/Go-1.26.5+-00ADD8.svg" alt="Go Version">
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

Croupier Go SDK 是 [Croupier](https://github.com/cuihairu/croupier) 游戏后端平台的官方 Go 客户端实现。SDK 作为 **Provider 端被调用方**，通过 **单条 TCP session**（`sdk-agent subprotocol`）接入 Agent，提供函数注册、心跳、自动重连、TLS 与可选的远程调用（Invoker）能力，并内置单公司多游戏多环境作用域支持。

## 正式文档

- 功能矩阵（跨语言一致性的单一事实来源）：[`sdks/SDK_FEATURE_MATRIX.md`](../SDK_FEATURE_MATRIX.md)
- 线协议约定：[`docs/architecture/sdk-wire-protocol.md`](../../docs/architecture/sdk-wire-protocol.md)
- 统一文档站入口：`/docs/sdks/go/`
- 仓库内路径：`docs/sdks/go`

## 主项目

| 项目           | 描述                               | 链接                                                           |
| -------------- | ---------------------------------- | -------------------------------------------------------------- |
| **Croupier**   | 游戏后端平台主项目（包含所有 SDK） | [cuihairu/croupier](https://github.com/cuihairu/croupier)      |
| **Proto 文件** | Protobuf 协议定义                  | [proto/](https://github.com/cuihairu/croupier/tree/main/proto) |

## 其他语言 SDK

所有 SDK 现已整合到主 monorepo 的 `sdks/` 目录下：

| 语言   | 目录                                                                       | CI                                                                                                                                                                    | Docs                          |
| ------ | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| C++    | [sdks/cpp/](https://github.com/cuihairu/croupier/tree/main/sdks/cpp)       | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-cpp.yml)       | [README](../cpp/README.md)    |
| Java   | [sdks/java/](https://github.com/cuihairu/croupier/tree/main/sdks/java)     | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-java.yml)     | [README](../java/README.md)   |
| JS/TS  | [sdks/js/](https://github.com/cuihairu/croupier/tree/main/sdks/js)         | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-js.yml)         | [README](../js/README.md)     |
| Python | [sdks/python/](https://github.com/cuihairu/croupier/tree/main/sdks/python) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-python.yml) | [README](../python/README.md) |
| C#     | [sdks/csharp/](https://github.com/cuihairu/croupier/tree/main/sdks/csharp) | [![CI](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml/badge.svg)](https://github.com/cuihairu/croupier/actions/workflows/ci-sdk-csharp.yml) | [README](../csharp/README.md) |

## 支持平台

| 平台        | 架构                       | 状态    |
| ----------- | -------------------------- | ------- |
| **Windows** | x64                        | ✅ 支持 |
| **Linux**   | x64, ARM64                 | ✅ 支持 |
| **macOS**   | x64, ARM64 (Apple Silicon) | ✅ 支持 |

## 核心特性

按 [功能矩阵](../SDK_FEATURE_MATRIX.md) 分层：

**L1 Core Provider（必备）**

- 📡 **TCP session 客户端** - 单条 `sdk-agent subprotocol` 长连接，不监听本地端口
- 🤝 **握手与心跳** - `ProviderConnectRequest`/`ProviderConnectResponse` 协商，可配置心跳间隔
- 🔁 **自动重连** - 指数退避 + jitter，可关闭或限制重试次数
- 📝 **函数注册** - 描述符 + handler 注册，handler 签名 `func(ctx, []byte) ([]byte, error)`
- 🏢 **多游戏多环境作用域** - 内置 `game_id` / `env` 业务隔离维度

**L2 Provider 扩展（可选）**

- 🔐 **TLS** - `ca_file` / `cert_file` / `key_file` / `server_name`
- 📋 **控制面 manifest 上传** - 配置 `control_addr` 后自动推送
- 🔍 **JSON Schema 校验** - 描述符 `input_schema` / `output_schema`
- 📦 **文件传输** - `enable_file_transfer=true` 启用，受白名单与上限约束

**L3 Invoker（独立调用方）**

- 🚀 **同步调用 / 异步作业** - `pkg/croupier/invoker.go`，独立配置入口

## 快速开始

### 系统要求

- **Go 1.26.5+**

> 运行时只需 Go：SDK 通过单条 TCP session（`sdk-agent subprotocol`）接入 Agent，不依赖任何 RPC 框架。
> 仅在修改主仓库 `proto/` 后重新生成 Protobuf 消息类型时才需要 protoc 工具链（见下文「Protobuf 代码生成」）。

### 安装

```bash
go get github.com/cuihairu/croupier/sdks/go
```

### 构建

```bash
go build ./...
```

SDK 始终通过单条 TCP session 与 Agent 通信：`ProviderConnectRequest` 握手 → 周期性 `ProviderHeartbeatRequest` → 接收 `InvokeRequest` 并回填 `InvokeResponse`。Protobuf 仅作为 TCP 帧的消息编解码格式，不是独立的传输层；SDK 不存在「mock / 真实传输」两种模式。

### Protobuf 代码生成（可选）

仅当主仓库 `proto/croupier/sdk/v1` 发生变更、需要刷新消息类型时执行：

```bash
make proto   # 从主仓库 proto/ 生成 Protobuf Go 代码到 pkg/pb/
```

详细步骤见 [PROTO_GENERATION.md](PROTO_GENERATION.md)。

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
        Resource:  "player",
        Risk:      "high",
        Operation: "ban",
        Permission: "player:ban",
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
CROUPIER_ENV=development
```

## 使用示例

### 函数描述符

跨语言统一的 `ProviderFunctionDescriptor` 字段（对应 `proto/croupier/sdk/v1/provider.proto`）：

```go
type FunctionDescriptor struct {
    ID          string // 函数 ID，如 "player.ban"
    Version     string // 语义化版本，如 "1.2.0"
    Resource    string // 业务资源或能力域，如 "player"
    Operation   string // 业务动作 key，如 "ban"、"send"、"list"
    Risk        string // "safe"|"warning"|"high"|"danger"
    Permission  string // 可选权限标识，如 "player:ban"
    Enabled     bool   // 是否启用
}
```

### 本地函数描述符

`sdk-agent subprotocol` 上承载的函数描述符（对应 `proto/croupier/sdk/v1/provider.proto` 的 `ProviderFunctionDescriptor`）：

```go
type ProviderFunctionDescriptor struct {
    ID      string // 函数 ID
    Version string // 函数版本
    // 扩展字段：tags / summary / description / operation_id / deprecated /
    // input_schema / output_schema / resource / operation / risk / enabled / permission
}
```

## 架构设计

### 数据流

```
Game Server → Go SDK (Provider) → Agent → Croupier Server → Web UI
                                       ↑
                          单条 TCP session（sdk-agent subprotocol）
```

SDK 是 `sdk-agent subprotocol` 上的 Provider 端：

1. 拨号到 Agent，发送 `ProviderConnectRequest`，接收 `ProviderConnectResponse(session_id)`
2. 周期性发送 `ProviderHeartbeatRequest`
3. 接收 `InvokeRequest`，调用 handler 后回填 `InvokeResponse`
4. 收到 `ProviderDrainRequest` 时停止接收新请求、完成在途、回 `ProviderDrainResponse`

### 构建与运行

SDK 始终使用 TCP session（`sdk-agent subprotocol`）作为唯一链路，本地、CI 与生产共用同一套传输实现，不存在「mock / 真实传输」分支。

```bash
# 本地开发 / CI / 生产
go build ./...
go run examples/basic/main.go
go run examples/game_demo/main.go
```

CI 流水线会在构建前运行 `make proto`，从主仓库 `proto/` 重新生成 Protobuf 消息类型，再执行上面的构建与测试。

## API 参考

### ClientConfig

```go
type ClientConfig struct {
    // 连接配置
    AgentAddr      string // Agent TCP session 地址（sdk-agent subprotocol）
    LocalListen    string // 兼容字段，新版本不再监听本地端口
    TimeoutSeconds int    // 连接超时
    Insecure       bool   // 是否跳过 TLS

    // 多游戏多环境作用域
    GameID         string // 游戏标识符
    Env            string // 逻辑环境（dev/staging/prod）
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
- session 通信错误
- 上下文取消时优雅关闭

## 开发指南

### 项目结构

```
sdks/go/
├── pkg/croupier/      # SDK 核心包
├── examples/          # 示例程序
├── scripts/           # 构建脚本
└── go.mod             # Go 模块定义 (module github.com/cuihairu/croupier/sdks/go)
```

### 构建命令

```bash
# 构建（TCP session 传输，本地与 CI 一致）
make build

# 运行测试
make test

# 重新生成 Protobuf 消息类型（仅 proto 变更时需要）
make proto
```

## 贡献指南

1. 确保所有类型与 proto 定义对齐
2. 为新功能添加测试
3. 更新 API 变更的文档
4. 在本地和 CI 中验证构建与测试

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源协议。

---

<p align="center">
  <a href="https://github.com/cuihairu/croupier">🏠 主项目</a> •
  <a href="https://github.com/cuihairu/croupier/issues">🐛 问题反馈</a> •
  <a href="https://github.com/cuihairu/croupier/discussions">💬 讨论区</a>
</p>
