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

本指南只覆盖当前仓库仍然有效的启动方式：`cmd/*` 二进制入口加 `configs/*` 配置文件。

## 环境要求

- Go 1.26.x
- Node.js 22+
- pnpm 10.22+
- Docker（可选）
- buf

## 安装与构建

```bash
git clone https://github.com/cuihairu/croupier.git
cd croupier
go mod download
make proto
make build
```

构建产物会输出到 `bin/`：

```text
bin/
├── croupier-server
├── croupier-agent
├── analytics-worker
├── ingest
└── ...
```

## 运行服务

### 1. 准备本地配置

```bash
cp configs/server.yaml configs/server.local.yaml
cp configs/agent.yaml configs/agent.local.yaml
```

### 2. 启动 Server

```bash
./bin/croupier-server --config configs/server.local.yaml
```

默认端口：

- HTTP: `18780`
- NNG Control: `19090`

### 3. 启动 Agent

```bash
./bin/croupier-agent --config configs/agent.local.yaml
```

### 4. 启动 Dashboard（可选）

```bash
cd dashboard
pnpm install
pnpm dev
```

## 验证

```bash
curl http://localhost:18780/api/v1/monitoring/health
```

## 下一步

- [配置管理](./configuration.md)
- [部署指南](./deployment.md)
- [开发文档](../development/README.md)
