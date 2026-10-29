---
layout: home
title: 首页
hero:
  name: Croupier
  image:
    src: /logo.png
  tagline: 分布式游戏运营控制面与 Agent 协同平台
  actions:
    - text: 快速开始 →
      link: /guide/quick-start
      theme: primary
    - text: SDK 指南
      link: /sdks/
      theme: secondary
    - text: API 参考
      link: /api/
      theme: secondary
features:
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><rect x="3" y="4.5" width="18" height="13" rx="2"/><path d="M7 9h5"/><path d="M7 13h7"/><circle cx="17" cy="9" r="1.6"/><circle cx="17" cy="13" r="1.6"/><path d="M9 17.5v2"/><path d="M15 17.5v2"/></svg>'
    title: 控制面与 Agent 协同
    details: Server 负责权限、审批、审计、配置和函数路由，Agent 通过统一 session 链路接入游戏服务与节点能力。
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M9.5 3.5h5v3h-5z"/><rect x="4.5" y="6.5" width="15" height="14" rx="2"/><path d="M8.5 11.5h7"/><path d="M8.5 15.5h4.5"/><path d="M15.5 15.5h1.5"/></svg>'
    title: 函数注册驱动
    details: OpenAPI/JSON Schema 作为函数契约输入，Server 识别资源能力并生成可直接发布、可局部编辑的默认业务页面。
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M12 3 5 6v5.2c0 4.4 2.9 7.4 7 8.8 4.1-1.4 7-4.4 7-8.8V6l-7-3z"/><path d="M9 10.5h6"/><path d="M9 14h6"/></svg>'
    title: 双层政策架构
    details: YAML 默认政策与数据库覆盖策略结合，支持低/中/高/危险四级风险控制。
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M9.5 14.5H7a2.5 2.5 0 0 1 0-5h2.5"/><path d="M14.5 9.5H17a2.5 2.5 0 0 1 0 5h-2.5"/><path d="M8.5 12h7"/><circle cx="12" cy="12" r="9" stroke-dasharray="2.5 3.5"/></svg>'
    title: 审批与审计
    details: 按执行路径与政策记录审计，支持审批和哈希链校验；持久化及完整性保证以当前架构说明为准。
  - icon: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" width="24" height="24"><path d="M4 20V11"/><path d="M9.5 20V5"/><path d="M15 20v-6"/><path d="M20.5 20V8"/><path d="m3.5 8.5 5-4 5 4.5 6-5.5"/></svg>'
    title: 数据分析链路
    details: ingest、worker、Redis Streams 与 ClickHouse 组成独立分析链路，文档单独归入 Analytics 入口。
footer: Apache-2.0 License | Copyright © 2024-present Croupier
---

## 什么是 Croupier

Croupier 是面向单公司多游戏、多环境的运营控制面：Server 负责治理、契约、页面与路由；sidecar 接入游戏 SDK，capture、synthetics 和 devops 按角色扩展采集、探测与运维能力。分析事件通过独立管道处理。

<HomeCarousel />

## 当前系统架构

Web 经 HTTP/SSE 访问 Server；Agent 主动建立上行 TCP session，SDK 连接 sidecar 本地网关。多实例部署通过 owner 转发访问实际持有连接的 Server。图中区分了进程边界、存储和可选部署组件。

<ArchitectureDiagram lang="zh" />

[阅读当前架构、状态归属与已知边界 →](/zh/architecture/)

> TLS 按配置启用，开发模板未开启。当前集群互联依赖可信网络隔离；审批的生产装配仍是内存存储。多实例路由能力不代表所有业务状态都已实现高可用。

## 核心能力

### 函数管理

- **OpenAPI 驱动注册**：通过 OpenAPI 规范快速注册函数
- **JSON Schema 表单展示**：根据函数契约生成字段与校验，JSON Schema form adapter 统一渲染调用、查询、创建、编辑与动作表单
- **Resource/PageSpec 编排**：CRUD Resource、独立操作、分页表格、报表和任务页由强类型 PageSpec 组合多个函数
- **统一调用链路**：控制面通过 Agent session 路由调用，本地接入通过 Agent gateway 完成

### 权限与安全

权限判定基于 Casbin 的 RBAC/ABAC 混合模型，角色之外还能按属性放行或拦截。政策分两层：YAML 给默认值，数据库覆盖策略在线调整。操作按低/中/高/危险四级定风险，高危操作自动进审批流程，其中双人审批规则要求多人确认才放行。

### 作用域模型

- **单公司多游戏**：不引入 SaaS 多租户抽象，标准业务边界是 `game_id`
- **多环境治理**：`env` 表达 `dev/test/staging/prod` 等逻辑环境
- **归属与部署分离**：`scope` 表达业务归属，`target` 表达运行位置

### 可观测性

审计按执行路径与政策记录，哈希链校验参与摘要的字段；它不等于不可篡改存储，也不覆盖每一条操作。执行留痕、遥测与审计各有用途，敏感字段按对应脱敏规则处理。

### 运维工具

- **工单系统**：玩家问题工单流转
- **反馈管理**：收集玩家反馈
- **公告系统**：游戏公告发布
- **数据分析**：玩家行为、留存、支付等数据分析

## 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier.git
cd croupier

# 下载依赖
go mod download

# 构建服务
make build
```

### 启动服务

```bash
# 启动 Server
./bin/croupier-server --config configs/server.yaml

# 启动 Agent
./bin/croupier-agent --config configs/agent.yaml
```

### 验证安装

```bash
# 检查 Server 健康状态
curl http://localhost:18780/healthz

# 查看 API 元信息（版本/能力声明）
curl http://localhost:18780/api/v1/
```

## 文档导航

按使用路径组织：

| 路径         | 入口                                      | 说明                                                             |
| ------------ | ----------------------------------------- | ---------------------------------------------------------------- |
| **快速开始** | [指南](/guide/) · 快速开始                | 安装、配置、Server + Agent 启动、最小闭环                        |
| **页面产品** | 函数管理 · Page Studio · Resource Catalog | 注册函数 → 自动生成页面 → Proposal Inbox 发布 → Console 受控执行 |
| **SDK**      | [SDK 概览](/sdks/)                        | Provider / Invoker / 配置 / 能力矩阵，6 种语言                   |
| **API**      | [API 参考](/api/)                         | REST 契约、鉴权、game/env scope、各资源 API                      |
| **运维**     | 监控 · [数据分析](/analytics/)            | 部署、监控、备份、分析、故障排除                                 |
| **开发**     | 开发 · [架构](/architecture/)             | 架构、代码规范、扩展策略、发布规则                               |

历史设计与迁移文档归入 [架构 - 提案与迁移](/architecture/)（侧栏已折叠），不作为接入主路径。

## 技术栈

- 后端：Go 1.27，HTTP 框架 Gin，ORM 为 GORM（MySQL / PostgreSQL / SQLite / SQL Server）
- 权限判定：Casbin（RBAC/ABAC）
- 内部链路：自建 TCP session，长度前缀分帧 + protobuf 信封
- 控制台：Umi Max + React + Ant Design Pro / ProComponents
- 文档站点：VitePress

## 路线图

- [x] 基础函数注册与调用
- [x] RBAC 权限控制
- [x] 审批流程
- [x] 审计日志
- [x] 双层政策架构
- [x] Dashboard vNext：函数注册自动生成页面（Proposal Inbox / Page Studio / Resource Catalog / Console 动态菜单）
- [ ] 插件市场
- [ ] 更多 SDK 语言支持

## 开源协议

Apache-2.0 License
