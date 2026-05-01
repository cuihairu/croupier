---
home: true
title: 首页
heroImage: /logo.png
heroText: Croupier
tagline: 分布式游戏运营控制面与 Agent 协同平台
actions:
  - text: 快速开始 →
    link: /guide/quick-start
    type: primary
  - text: 架构概述
    link: /architecture/
    type: secondary
features:
  - title: 控制面与 Agent 协同
    details: Server 负责权限、审批、审计、配置和函数路由，Agent 负责接入游戏服务与节点能力。
  - title: 函数注册驱动
    details: OpenAPI、JSON Schema、函数 UI 配置与扩展安装绑定共同驱动管理界面与调用流程。
  - title: 双层政策架构
    details: YAML 默认政策与数据库覆盖策略结合，支持低/中/高/危险四级风险控制。
  - title: 完整的审计链
    details: 所有操作记录审计日志，高风险操作需要双人审批，支持哈希链防篡改。
footer: Apache-2.0 License | Copyright © 2024-present Croupier
---

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier.git
cd croupier

# 下载依赖并构建
go mod download && make build

# 启动服务
./bin/croupier-server --config configs/server.yaml
./bin/croupier-agent --config configs/agent.yaml
```

## 文档导航

| 模块 | 说明 |
|------|------|
| [指南](/guide/) | 快速开始、安装配置、核心概念、运维指南 |
| [架构](/architecture/) | 系统分层、传输协议、扩展系统设计 |
| [API 参考](/api/) | REST API 接口文档 |
| [开发](/development/) | 仓库结构、开发约定、发布流程 |
| [SDK](/sdks/) | 多语言 SDK 文档与能力矩阵 |

## 核心特性

- **统一控制面**：集中管理权限、审批、审计和函数路由
- **分布式 Agent**：支持多游戏服务接入，负载均衡和故障转移
- **函数注册**：OpenAPI 驱动的函数注册与 UI 自动生成
- **政策系统**：基于风险等级的自动审批和审计策略
- **可观测性**：完整的审计链和监控指标
