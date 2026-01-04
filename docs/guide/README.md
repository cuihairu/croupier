---
home: true
title: 首页
heroImage: /logo.png
heroText: Croupier
tagline: 分布式游戏管理系统
actions:
  - text: 快速开始 →
    link: /guide/quick-start.html
    type: primary
  - text: 项目介绍
    link: /guide/concepts/overview.html
    type: secondary
features:
  - title: 🔐 零信任安全
    details: gRPC+mTLS、细粒度 RBAC/ABAC、操作审批与审计日志，确保游戏运营安全。
  - title: 🎮 函数注册控制
    details: 游戏服务器通过 Agent 注册函数，控制面统一调用、可视化进度与日志。
  - title: 📊 Schema 驱动 UI
    details: X-Render + JSON Schema 自动生成表单、风控提示、参数校验。
  - title: 🔄 可观测性解耦
    details: 控制面与遥测面分离，支持实时事件处理与多维度监控。
  - title: 📦 多语言 SDK
    details: Go / C++ / Java / JS / Python 全覆盖，保持 Nightly 构建。
  - title: 🚀 协议优先开发
    details: 所有 API 通过 Protocol Buffers 定义，使用 Buf 工具链管理。
footer: Apache-2.0 License | Copyright © 2024-present Croupier
---

## 快速开始

::: tip 环境要求
- Go 1.25+
- Node.js 22+
- Docker (可选)
- CMake 3.20+ (C++ SDK)
:::

### 1. 克隆仓库

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
git submodule update --init --recursive
```

### 2. 安装依赖并构建

```bash
# 安装依赖
go mod download

# 生成协议代码
make proto

# 构建所有组件
make build
```

### 3. 运行服务

```bash
# 启动 Server
./bin/croupier-server --config configs/server.example.yaml

# 启动 Agent
./bin/croupier-agent --config configs/agent.example.yaml
```

## 项目结构

```
croupier/
├── cmd/                 # 程序入口
├── proto/               # Protobuf 定义
├── internal/            # 内部实现
│   ├── server/          # 服务端核心逻辑
│   ├── agent/           # 代理实现
│   └── auth/            # 认证授权
├── sdks/                # 多语言 SDK
│   ├── go/
│   ├── cpp/
│   ├── java/
│   ├── js/
│   └── python/
├── dashboard/           # Web 管理界面
├── configs/             # 配置文件
├── examples/            # 示例代码
└── docs/                # 项目文档
```

## 系统架构

```mermaid
graph TB
  subgraph "客户端"
    Client[游戏客户端]
  end

  subgraph "管理控制层"
    UI[Web 管理界面]
    Server[Croupier Server]
  end

  subgraph "分布式代理层"
    A1[Agent 1]
    A2[Agent 2]
  end

  subgraph "游戏服务层"
    GS1[Game Server A]
    GS2[Game Server B]
  end

  UI -->|HTTP REST| Server
  Server -->|gRPC mTLS| A1
  Server -->|gRPC mTLS| A2
  A1 --> GS1
  A2 --> GS2
  Client --> GS1
  Client --> GS2
```

## 核心组件

| 组件 | 说明 | 仓库 |
|------|------|------|
| Server | 中央控制平面（HTTP + gRPC） | [cuihairu/croupier](https://github.com/cuihairu/croupier) |
| Agent | 分布式代理进程 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |
| Edge | DMZ/边缘代理 | [cuihairu/croupier](https://github.com/cuihairu/croupier) |
| Dashboard | Web 管理界面 | [cuihairu/croupier-dashboard](https://github.com/cuihairu/croupier-dashboard) |

## 相关链接

- [GitHub 仓库](https://github.com/cuihairu/croupier)
- [问题跟踪](https://github.com/cuihairu/croupier/issues)
- [更新日志](https://github.com/cuihairu/croupier/releases)
