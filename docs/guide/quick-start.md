---
title: 快速开始
icon: lightbulb
order: 1
category:
  - 入门指南
tag:
  - 快速开始
---

# 快速开始

本指南只覆盖当前仍然有效的启动方式：

- `Server <-> Agent` 使用单条内部 session 链路
- `SDK / GameServer / 第三方本地应用 <-> Agent` 使用 Agent 本地 gateway
- 不再以历史 `gRPC` 或 `历史 REQ/REP` 文档作为新的接入依据

## 环境要求

- Go `1.26.x`
- Node.js `22+`
- pnpm `10.22+`
- Docker（可选）
- `buf`

## 安装与构建

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
go mod download
make proto
make build
```

构建产物默认输出到 `bin/`。

## 准备配置

本地开发推荐直接使用仓库内示例配置：

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.local.yaml configs/agent.runtime.yaml
```

关键端口含义：

- `18780`: Server REST API
- `19090`: Server session/control 入口，供 Agent 主动连接
- `19091`: Agent 本地 gateway，供 SDK / GameServer / 第三方本地程序接入

说明：

- `19090` 是内部控制与会话链路端口，不应再理解为 `gRPC` 或 `旧传输` 专用端口
- `19091` 是本地接入边界，不是 `Server -> Agent` 回拨入口

## 启动服务

### 1. 启动 Server

```bash
./bin/croupier-server --config configs/server.local.yaml
```

### 2. 启动 Agent

```bash
./bin/croupier-agent --config configs/agent.runtime.yaml
```

### 3. 启动 Dashboard（可选）

```bash
cd web
pnpm install
pnpm dev
```

## 验证

### 检查 Server 健康状态

```bash
curl http://localhost:18780/healthz
```

### 检查 Agent 是否已连上 Server

```bash
curl http://localhost:18780/api/v1/agents
```

### 检查 Dashboard

```bash
curl http://localhost:8000
```

### 检查本地接入边界

如果你准备接入 SDK，请确认 SDK 目标地址指向 Agent 本地 gateway，例如：

```text
127.0.0.1:19091
```

而不是 Server 的 `19090` 端口。

## 下一步

- [配置管理](./configuration.md)
- [部署指南](./deployment.md)
- [核心概念](./concepts/overview.md)
- [SDK 文档](../sdks/)
- [SDK-Agent 传输设计](../architecture/sdk-agent-transport-redesign.md)
