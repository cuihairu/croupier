---
title: 快速开始
icon: lightbulb
order: 1
category:
  - 入门指南
tag:
  - 快速开始
  - 安装
---

# 快速开始

本指南将帮助你快速搭建 Croupier 开发环境并运行第一个示例。

## 环境要求

在开始之前，请确保你的开发环境满足以下要求：

- **Go**: 1.25 或更高版本
- **Node.js**: 22 或更高版本
- **pnpm**: 10.22.0 或更高版本
- **Docker**: 可选，用于容器化部署
- **CMake**: 3.20+（仅 C++ SDK 开发需要）
- **buf**: Protocol Buffers 工具链

### 检查环境

```bash
# 检查 Go 版本
go version

# 检查 Node.js 版本
node -v

# 检查 pnpm 版本
pnpm -v

# 检查 buf 版本
buf --version
```

## 安装依赖

### 1. 克隆仓库

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier

# 初始化并更新子模块
git submodule update --init --recursive
```

### 2. 安装 Go 依赖

```bash
go mod download
```

### 3. 安装工具链

```bash
# 安装 buf（如果未安装）
go install github.com/bufbuild/buf/cmd/buf@latest

# 验证安装
buf --version
```

## 构建项目

### 完整构建

```bash
# 清理构建
make clean

# 完整构建（生成协议 + 编译）
make dev
```

### 单独构建组件

```bash
# 仅生成协议代码
make proto

# 构建所有二进制文件
make build

# 构建特定组件
make build-server
make build-agent
make build-worker
make build-ingest
```

构建产物将输出到 `/bin` 目录：

```
bin/
├── croupier-server
├── croupier-agent
├── analytics-worker
└── ingest
```

## 运行服务

### 1. 生成 TLS 证书（开发环境）

```bash
./scripts/dev-certs.sh
```

这将生成自签名证书到 `data/dev-certs/` 目录。

### 2. 配置服务

复制示例配置并根据需要修改：

```bash
# Server 配置
cp services/server/etc/server.yaml configs/server.yaml

# Agent 配置
cp configs/agent.example.yaml configs/agent.yaml
```

### 3. 启动 Server

```bash
./bin/croupier-server --config configs/server.yaml
```

Server 默认监听：
- **gRPC**: `8443` (mTLS)
- **HTTP**: `18780` (REST API)

### 4. 启动 Agent

```bash
./bin/croupier-agent --config configs/agent.yaml
```

Agent 将连接到 Server 并注册游戏服务器函数。

### 5. 启动 Dashboard（可选）

```bash
cd dashboard
pnpm install
pnpm dev
```

访问 http://localhost:8000 查看管理界面。

## 验证安装

### 健康检查

```bash
# 检查 Server HTTP 端点
curl http://localhost:18780/healthz

# 预期输出
# {"status":"ok"}
```

### 查看日志

```bash
# Server 日志
tail -f logs/server.log

# Agent 日志
tail -f logs/agent.log
```

## 下一步

安装完成后，你可以：

- 阅读核心概念了解系统设计
- 查看配置指南了解详细配置选项
- 探索示例代码学习最佳实践
- 选择合适的 SDK 开始集成

## 常见问题

### 端口被占用

如果默认端口被占用，可以通过配置文件修改：

```yaml
# server.yaml
Control:
  Addr: ":19090"  # Control 端口
Port: 18780       # HTTP 端口
```

### TLS 证书错误

开发环境使用自签名证书，需要在客户端跳过证书验证：

```bash
curl -k https://localhost:8443/healthz
```

### 子模块为空

如果子模块目录为空，重新初始化：

```bash
git submodule deinit --all
git submodule update --init --recursive
```
