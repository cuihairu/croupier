---
home: true
title: 使用指南
heroImage: /logo.png
heroText: Croupier Guide
tagline: 上手与核心概念——部署、配置与运维见「运维」栏目
actions:
  - text: 快速开始 →
    link: /guide/quick-start.html
    type: primary
  - text: 配置管理
    link: /operations/
    type: secondary
---

## 维护说明

本节只保留和当前仓库结构一致的使用文档：

- `quick-start.md`: 本地启动最短路径
- `installation.md`: 依赖安装

部署、配置全解、负载均衡、监控/安全/故障排除、备份恢复等运维内容已独立为 [运维栏目](/operations/)。

以下旧内容已经移除：

- 基于 `services/*` 的入口说明
- go-zero 迁移过程文档
- 已失效的旧 FAQ、tutorial、运行手册

## 推荐阅读顺序

1. [快速开始](./quick-start.md)
2. [安装指南](./installation.md)
3. [界面预览（本节仅有中文版）](/zh/guide/interface-preview)——真实运行栈的主要界面截图
4. [配置管理](../operations/config-server.md)
5. [部署指南（本节仅有中文版）](/zh/operations/deploy-docker)
6. 核心概念：[系统概述（本节仅有中文版）](/zh/guide/concepts/overview) → [函数管理（本节仅有中文版）](/zh/guide/concepts/function-management) → [Page Studio（本节仅有中文版）](/zh/guide/concepts/function-registration-ui) → [权限控制（本节仅有中文版）](/zh/guide/concepts/permissions)
7. 集成指南：[OpenAPI 注册](/archive/legacy-en/guide/integrations/openapi-registration)、[第三方平台](/archive/legacy-en/guide/integrations/third-party-platforms)
