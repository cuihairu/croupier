---
title: API 概览
icon: code
order: 1
---

# API 概览

本目录记录面向 Dashboard、外部管理系统和兼容调用方的 HTTP API。内部 Agent/SDK 链路不以 REST 文档为准，应参考 [架构总览](/architecture/) 与 [SDK Wire Protocol](/zh/architecture/sdk-wire-protocol)。

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
- 每个业务域只保留一个 canonical 页面，例如函数域使用 [函数 API](./function.md)，任务域使用 [任务 API](/archive/legacy-en/api/task)。
- 兼容历史调用的页面必须在标题或正文标明“兼容”，例如 [函数调用兼容 API](/archive/legacy-en/api/function_call)。
- `ops.md` 是运维域当前主入口，`ops_core.md` 和 `ops-simple.md` 保留为拆分/兼容参考，不能新增独立语义。
- Analytics 的 HTTP API 页面保留在本目录，分析系统设计和指标说明保留在 [Analytics 文档](/analytics/)。

## 主要接口分类

| 分类       | Canonical 文档                                                                                                                                                                                                                                                                                                                                                                  |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 认证与基础 | [认证 API](./auth.md)、[REST 契约](/archive/legacy-en/api/rest)、[Schema API](/archive/legacy-en/api/schema)、[元数据 API](/archive/legacy-en/api/meta)                                                                                                                                                                                                                         |
| 核心业务   | [游戏 API](/archive/legacy-en/api/game)、[玩家 API](/archive/legacy-en/api/player)、[函数 API](./function.md)、[函数元数据注册](/archive/legacy-en/api/meta)、[任务 API](/archive/legacy-en/api/task)、[消息 API](/archive/legacy-en/api/message)、[配置 API](/sdks/cpp/api/config)                                                                                             |
| 审批与审计 | [审批 API](/archive/legacy-en/api/approval)、[审计 API](/archive/legacy-en/api/audit)                                                                                                                                                                                                                                                                                           |
| 页面产品域 | [页面与控制台 API](/archive/legacy-en/api/page)（Proposal / Pages / Versioning / Console）、[控制台菜单 API](/archive/legacy-en/api/menu)、[Resource Catalog API](./resource.md)、[OpenAPI 注册](/archive/legacy-en/guide/integrations/openapi-registration)                                                                                                                    |
| 运维与平台 | [运维 API](/archive/legacy-en/api/ops)（含系统维护/性能参数/日志维护/第三方探针）、[Agent API](/archive/legacy-en/api/agent)、[节点 API](/archive/legacy-en/api/node)、[注册表 API](/archive/legacy-en/api/registry)、[平台 API](/archive/legacy-en/api/platform)、[Provider API](/archive/legacy-en/api/provider)、[CI/CD 接入 API](/archive/legacy-en/api/cicd)               |
| 数据分析   | [数据分析 API](/zh/analytics)、[分析概览 API](/archive/legacy-en/api/analytics_overview)、[行为分析 API](/archive/legacy-en/api/analytics_behavior)、[留存分析 API](/archive/legacy-en/api/analytics_retention)、[支付分析 API](/archive/legacy-en/api/analytics_payments)                                                                                                      |
| 控制台域   | [管理员 API](/archive/legacy-en/api/admin)、[Profile API](/archive/legacy-en/api/profile)                                                                                                                                                                                                                                                                                       |
| 运营支持   | [分配 API](/archive/legacy-en/api/assignment)、[工单 API](/archive/legacy-en/api/ticket)、[公告 API](/archive/legacy-en/api/announcement)、[反馈 API](/archive/legacy-en/api/feedback)、[支持 API](/archive/legacy-en/api/support)、[FAQ](/archive/legacy-en/api/faq)                                                                                                           |
| 站点配置   | [网站配置 API](/archive/legacy-en/api/site-settings)（L3 配置中心：品牌/通知/安全/出站/登录方式）                                                                                                                                                                                                                                                                               |
| 系统能力   | [存储 API](/archive/legacy-en/api/storage)、[备份 API](/archive/legacy-en/api/backup)、[监控 API](/operations/monitoring)、[组件模板 API](/archive/legacy-en/api/component-templates)、[定时调度 API](/archive/legacy-en/api/schedule)、[证书 API](/archive/legacy-en/api/certificate)、[限流 API](/archive/legacy-en/api/rate_limit)、[告警 API](/archive/legacy-en/api/alert) |

## 已实现、暂无独立 API 页

以下端点族已在 `internal/handler/routes.go` 注册生效，但尚未建立独立 canonical 页
（接口形态以路由注册与对应 handler 为准；补页前此处如实登记，不留「文档比代码少」的暗坑）：

| 端点族                 | 路由（/api/v1 前缀）                       | 相关文档/代码                                                                           |
| ---------------------- | ------------------------------------------ | --------------------------------------------------------------------------------------- |
| 执行日志               | `/execution-logs`                          | `internal/api/executionlog/`                                                            |
| 配置浏览器             | `/config-explorer`                         | `internal/api/configexplorer/`、[配置工作流分析](/research/config-workflows-analysis)   |
| 资源语义目录           | `/resource-catalog`                        | `internal/api/resourcecatalog/`                                                         |
| 数据库监控             | `/dbmon`                                   | `internal/api/dbmon/`、[库监控设计](/research/db-monitoring-design)                     |
| 术语表                 | `/terms`                                   | `internal/api/terms/`                                                                   |
| 研发域四族（dev 开关） | `/bugs` `/tools` `/releases` `/hotpatches` | `internal/api/{bug,tool,release,hotpatch}/`                                             |
| 扩展域                 | `/extensions` `/platforms` `/agents`       | [扩展域 API 契约基线](/archive/legacy-en/architecture/extensions-api-contract-baseline) |

## 兼容页

以下页面存在是为了兼容历史调用或拆分过渡，不应作为新功能设计入口：

- [函数调用兼容 API](/archive/legacy-en/api/function_call)
- [运维核心 API](/archive/legacy-en/api/ops_core)
- [运维简化 API](/archive/legacy-en/api/ops-simple)
