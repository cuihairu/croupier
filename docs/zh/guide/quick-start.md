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

本页只覆盖当前仍然有效的启动方式：

- `Server ↔ Agent` 走单条内部 session 链路。
- `SDK / GameServer / 第三方本地应用 ↔ Agent` 走 Agent 本地 gateway。
- 历史文档中的 `gRPC` 和旧 `REQ/REP` 模型不再作为新接入依据。

除默认 sidecar agent 外，仓库还包含 `devops-agent` 和 `capture-agent`，按需独立部署；快速开始不需要它们。

## 环境要求

| 依赖    | 版本                         |
| ------- | ---------------------------- |
| Go      | `1.27.2`                     |
| Node.js | `24+`                        |
| pnpm    | `11+`                        |
| buf     | 最新稳定版                   |
| Docker  | 可选，用于本地依赖或容器部署 |

## 获取与构建

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
go mod download
make proto
make build
```

构建产物输出到 `bin/`。

## 准备配置

本地开发可以先复制示例配置：

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.local.yaml configs/agent.runtime.yaml
```

| 端口    | 归属   | 用途                               |
| ------- | ------ | ---------------------------------- |
| `18780` | Server | REST API 和 Dashboard 后端。       |
| `19090` | Server | 控制与会话入口，Agent 主动连接。   |
| `19091` | Agent  | 本地 gateway，SDK 或游戏进程接入。 |

不要把 `19090` 当成 SDK 直连地址；SDK 只应连接 Agent 的本地边界 `19091`。

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

### 检查 Server 健康

```bash
curl http://localhost:18780/healthz
```

返回包含 `ok`、时间戳、运行时长和组件状态即说明核心服务已启动。

### 检查 Agent 注册

Agent 列表需要认证与业务范围头：

```bash
curl http://localhost:18780/api/v1/ops/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Game-ID: default" \
  -H "X-Env: prod"
```

如果列表为空，优先检查 Agent 配置里的 Server 地址、TLS 证书、`gameId/env` 与 Server 日志。

## 下一步

- 阅读[安装指南](./installation.md)了解源码、二进制和容器安装。
- 阅读[系统概述](./concepts/overview.md)理解统一 session 与契约模型。
- 进入[运维栏目](/zh/operations/)配置部署、监控、备份和故障处理。
