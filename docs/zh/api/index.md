---
title: API 概览
icon: code
order: 1
---

# API 概览

本目录记录面向 Dashboard、外部管理系统和兼容调用方的 HTTP API。内部 Agent/SDK 链路不以 REST 文档为准，应参考 [架构总览](/architecture/) 与 [SDK Wire Protocol](/architecture/sdk-wire-protocol)。

## API 类型

| 类型             | 说明                                         | 协议                          |
| ---------------- | -------------------------------------------- | ----------------------------- |
| REST API         | 面向 Dashboard 与外部管理调用                | HTTP / HTTPS                  |
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

- **路由清单以 `internal/handler/routes.go` 为准**（生效注册）；文档与代码不一致时修文档。
- 每个业务域只保留一个 canonical 页面，例如函数域使用 [函数 API](./function.md)，任务域使用 [任务 API](./task.md)。
- 兼容历史调用的页面必须在标题或正文标明“兼容”，例如 [函数调用兼容 API](./function_call.md)。
- `ops.md` 是运维域当前主入口，`ops_core.md` 和 `ops-simple.md` 保留为拆分/兼容参考，不能新增独立语义。
- Analytics 的 HTTP API 页面保留在本目录，分析系统设计和指标说明保留在 [Analytics 文档](/analytics/)。

## 主要接口分类

| 分类       | Canonical 文档                                                                                                                                                                                                                             |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 认证与基础 | [认证 API](./auth.md)、[REST 契约](./rest.md)、[Schema API](./schema.md)、[元数据 API](./meta.md)                                                                                                                                          |
| 核心业务   | [游戏 API](./game.md)、[玩家 API](./player.md)、[函数 API](./function.md)、[函数元数据注册](./meta.md)、[任务 API](./task.md)、[消息 API](./message.md)、[配置 API](./config.md)                                                           |
| 审批与审计 | [审批 API](./approval.md)、[审计 API](./audit.md)                                                                                                                                                                                          |
| 页面产品域 | [页面与控制台 API](./page.md)（Proposal / Pages / Versioning / Console）、[控制台菜单 API](./menu.md)、[Resource Catalog API](./resource.md)、[OpenAPI 注册](../guide/integrations/openapi-registration.md)                                |
| 运维与平台 | [运维 API](./ops.md)（含系统维护/性能参数/日志维护/第三方探针）、[Agent API](./agent.md)、[节点 API](./node.md)、[注册表 API](./registry.md)、[平台 API](./platform.md)、[Provider API](./provider.md)、[CI/CD 接入 API](./cicd.md)        |
| 数据分析   | [数据分析 API](./analytics.md)、[分析概览 API](./analytics_overview.md)、[行为分析 API](./analytics_behavior.md)、[留存分析 API](./analytics_retention.md)、[支付分析 API](./analytics_payments.md)                                        |
| 控制台域   | [管理员 API](./admin.md)、[Profile API](./profile.md)                                                                                                                                                                                      |
| 运营支持   | [分配 API](./assignment.md)、[工单 API](./ticket.md)、[公告 API](./announcement.md)、[反馈 API](./feedback.md)、[支持 API](./support.md)、[FAQ](./faq.md)                                                                                  |
| 站点配置   | [网站配置 API](./site-settings.md)（L3 配置中心：品牌/通知/安全/出站/登录方式）                                                                                                                                                            |
| 系统能力   | [存储 API](./storage.md)、[备份 API](./backup.md)、[监控 API](./monitoring.md)、[组件模板 API](./component-templates.md)、[定时调度 API](./schedule.md)、[证书 API](./certificate.md)、[限流 API](./rate_limit.md)、[告警 API](./alert.md) |

## 已实现、暂无独立 API 页

以下端点族已在 `internal/handler/routes.go` 注册生效，但尚未建立独立 canonical 页
（接口形态以路由注册与对应 handler 为准；补页前此处如实登记，不留「文档比代码少」的暗坑）：

| 端点族                 | 路由（/api/v1 前缀）                       | 相关文档/代码                                                                              |
| ---------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------ |
| 执行日志               | `/execution-logs`                          | `internal/api/executionlog/`                                                               |
| 配置浏览器             | `/config-explorer`                         | `internal/api/configexplorer/`、[配置工作流分析](../research/config-workflows-analysis.md) |
| 资源语义目录           | `/resource-catalog`                        | `internal/api/resourcecatalog/`                                                            |
| 数据库监控             | `/dbmon`                                   | `internal/api/dbmon/`、[库监控设计](../research/db-monitoring-design.md)                   |
| 术语表                 | `/terms`                                   | `internal/api/terms/`                                                                      |
| 研发域四族（dev 开关） | `/bugs` `/tools` `/releases` `/hotpatches` | `internal/api/{bug,tool,release,hotpatch}/`                                                |
| 扩展域                 | `/extensions` `/platforms` `/agents`       | [扩展域 API 契约基线](../architecture/extensions-api-contract-baseline.md)                 |

## 兼容页

以下页面存在是为了兼容历史调用或拆分过渡，不应作为新功能设计入口：

- [函数调用兼容 API](./function_call.md)
- [运维核心 API](./ops_core.md)
- [运维简化 API](./ops-simple.md)
