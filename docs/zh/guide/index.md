---
home: true
title: 使用指南
heroImage: /logo.png
heroText: Croupier 使用指南
tagline: 上手、核心概念与接入路径——部署、配置与运维见「运维」栏目
actions:
  - text: 快速开始 →
    link: /zh/guide/quick-start.html
    type: primary
  - text: 安装指南
    link: /zh/guide/installation.html
    type: secondary
---

## 本节范围

这里只保留与当前仓库结构一致的使用文档，不延续基于 `services/*`、go-zero 迁移或旧 FAQ 的历史入口。

| 内容                                          | 定位                                             |
| --------------------------------------------- | ------------------------------------------------ |
| [快速开始](./quick-start.md)                  | 本地拉起 Server、Agent 和 Dashboard 的最短路径。 |
| [安装指南](./installation.md)                 | 依赖、源码构建、二进制、Docker 与 SDK 安装方式。 |
| [界面预览](./interface-preview.md)            | 真实运行栈的主要界面截图。                       |
| [系统概述](./concepts/overview.md)            | 组件、传输、业务边界、Dashboard 核心模型。       |
| [函数管理](./concepts/function-management.md) | 函数注册、契约、调用与生命周期。                 |
| [权限控制](./concepts/permissions.md)         | 角色、资源动作与高风险操作治理。                 |

部署、配置全解、负载均衡、监控、安全、故障排除、备份恢复等内容在[运维栏目](/zh/operations/)维护。

## 推荐阅读顺序

1. 先跑通[快速开始](./quick-start.md)，确认三个关键端口的角色。
2. 阅读[安装指南](./installation.md)，区分开发环境与生产安装。
3. 看[系统概述](./concepts/overview.md)，理解 `gameId + env` 与统一 session 边界。
4. 进入[函数管理](./concepts/function-management.md)，理解函数、资源和页面的分层。
5. 需要 OpenAPI 或第三方平台接入时，再阅读对应集成指南。

## 适合谁读

- 游戏服务开发者：准备嵌入 SDK 或注册函数。
- 运营平台开发者：理解契约、页面与执行链路。
- 运维人员：需要确认本地验证方式，再进入运维栏目做部署和配置。
