---
title: API 概览
icon: code
order: 1
---

# API 概览

本目录记录面向 Dashboard、外部管理系统和兼容调用方的 HTTP API。内部 Agent/SDK 链路不以 REST 文档为准，应参考 [架构总览](/architecture/) 与 SDK Wire Protocol。

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
- 每个业务域只保留一个 canonical 页面，例如函数域使用 [函数 API](./function.md)，任务域使用 任务 API。
- `ops.md` 是运维域当前主入口，`ops_core.md` 和 `ops-simple.md` 保留为拆分/兼容参考，不能新增独立语义。
- Analytics 的 HTTP API 页面保留在本目录，分析系统设计和指标说明保留在 [Analytics 文档](/analytics/)。

## 主要接口分类

| 分类       | Canonical 文档                                                                                                                   |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 认证与基础 | [认证 API](./auth.md)、REST 契约、Schema API、元数据 API                                                                         |
| 核心业务   | 游戏 API、玩家 API、[函数 API](./function.md)、函数元数据注册、任务 API、消息 API、配置 API                                      |
| 审批与审计 | 审批 API、审计 API                                                                                                               |
| 页面产品域 | 页面与控制台 API（Proposal / Pages / Versioning / Console）、控制台菜单 API、[Resource Catalog API](./resource.md)、OpenAPI 注册 |
| 运维与平台 | 运维 API（含系统维护/性能参数/日志维护/第三方探针）、Agent API、节点 API、注册表 API、平台 API、Provider API、CI/CD 接入 API     |
| 数据分析   | [数据分析 API](/zh/analytics)、分析概览 API、行为分析 API、留存分析 API、支付分析 API                                            |
| 控制台域   | 管理员 API、Profile API                                                                                                          |
| 运营支持   | 分配 API、工单 API、公告 API、反馈 API、支持 API、FAQ                                                                            |
| 站点配置   | 网站配置 API（L3 配置中心：品牌/通知/安全/出站/登录方式）                                                                        |
| 系统能力   | 存储 API、备份 API、监控 API、组件模板 API、定时调度 API、证书 API、限流 API、告警 API                                           |

## 已实现、暂无独立 API 页

以下端点族已在 `internal/handler/routes.go` 注册生效，但尚未建立独立 canonical 页
（接口形态以路由注册与对应 handler 为准；补页前此处如实登记，不留「文档比代码少」的暗坑）：

| 端点族                 | 路由（/api/v1 前缀）                       | 相关文档/代码                                  |
| ---------------------- | ------------------------------------------ | ---------------------------------------------- |
| 执行日志               | `/execution-logs`                          | `internal/api/executionlog/`                   |
| 配置浏览器             | `/config-explorer`                         | `internal/api/configexplorer/`、配置工作流分析 |
| 资源语义目录           | `/resource-catalog`                        | `internal/api/resourcecatalog/`                |
| 数据库监控             | `/dbmon`                                   | `internal/api/dbmon/`、库监控设计              |
| 术语表                 | `/terms`                                   | `internal/api/terms/`                          |
| 研发域四族（dev 开关） | `/bugs` `/tools` `/releases` `/hotpatches` | `internal/api/{bug,tool,release,hotpatch}/`    |
| 扩展域                 | `/extensions` `/platforms` `/agents`       | 扩展域 API 契约基线                            |

## 兼容页

以下页面存在是为了兼容历史调用或拆分过渡，不应作为新功能设计入口：
