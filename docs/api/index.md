---
title: API 概览
icon: code
order: 1
---

# API 概览

本目录记录面向 Dashboard、外部管理系统和兼容调用方的 HTTP API。内部 Agent/SDK 链路不以 REST 文档为准，应参考 [架构总览](/architecture/) 与 [SDK Wire Protocol](/architecture/sdk-wire-protocol)。

## API 类型

| 类型 | 说明 | 协议 |
| --- | --- | --- |
| REST API | 面向 Dashboard 与外部管理调用 | HTTP / HTTPS |
| Session Wire API | SDK 与 Agent、Agent 与 Server 之间的内部协议 | TCP session，可按链路启用 TLS |

## REST API

REST API 用于：

- Dashboard 管理界面
- 外部系统集成
- 查询与配置操作

**基础路径：** `/api/v1/`

**认证方式：** JWT Bearer Token

## Canonical 文档规则

API 文档当前处于收敛期。新增或修改接口时按以下规则维护：

- 每个业务域只保留一个 canonical 页面，例如函数域使用 [函数 API](./function.md)，任务域使用 [任务 API](./task.md)。
- 兼容历史调用的页面必须在标题或正文标明“兼容”，例如 [函数调用兼容 API](./function_call.md)。
- `ops.md` 是运维域当前主入口，`ops_core.md` 和 `ops-simple.md` 保留为拆分/兼容参考，不能新增独立语义。
- Analytics 的 HTTP API 页面保留在本目录，分析系统设计和指标说明保留在 [Analytics 文档](/analytics/)。

## 主要接口分类

| 分类 | Canonical 文档 |
| --- | --- |
| 认证与基础 | [认证 API](./auth.md)、[REST API](./rest.md)、[Schema API](./schema.md)、[元数据 API](./meta.md) |
| 核心业务 | [游戏 API](./game.md)、[玩家 API](./player.md)、[函数 API](./function.md)、[任务 API](./task.md)、[消息 API](./message.md)、[配置 API](./config.md) |
| 审批与审计 | [审批 API](./approval.md)、[审计 API](./audit.md) |
| 运维与平台 | [运维 API](./ops.md)、[Agent API](./agent.md)、[节点 API](./node.md)、[注册表 API](./registry.md)、[平台 API](./platform.md)、[Provider API](./provider.md) |
| 数据分析 | [数据分析 API](./analytics.md)、[分析概览 API](./analytics_overview.md)、[行为分析 API](./analytics_behavior.md)、[留存分析 API](./analytics_retention.md)、[支付分析 API](./analytics_payments.md) |
| 控制台域 | [管理员 API](./admin.md)、[Workspace API](./workspace.md)、[实体 API](./entity.md)、[Profile API](./profile.md) |
| 运营支持 | [分配 API](./assignment.md)、[工单 API](./ticket.md)、[反馈 API](./feedback.md)、[支持 API](./support.md)、[FAQ](./faq.md) |
| 系统能力 | [存储 API](./storage.md)、[备份 API](./backup.md)、[迁移 API](./migrate.md)、[监控 API](./monitoring.md)、[证书 API](./certificate.md)、[限流 API](./rate_limit.md)、[告警 API](./alert.md) |

## 兼容页

以下页面存在是为了兼容历史调用或拆分过渡，不应作为新功能设计入口：

- [函数调用兼容 API](./function_call.md)
- [运维核心 API](./ops_core.md)
- [运维简化 API](./ops-simple.md)
