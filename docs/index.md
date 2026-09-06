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
  - text: SDK 指南
    link: /sdks/
    type: secondary
  - text: API 参考
    link: /api/
    type: secondary
features:
  - title: 控制面与 Agent 协同
    details: Server 负责权限、审批、审计、配置和函数路由，Agent 通过统一 session 链路接入游戏服务与节点能力。
  - title: 函数注册驱动
    details: OpenAPI/JSON Schema 作为函数契约输入，Server 识别资源能力并生成可直接发布、可局部编辑的默认业务页面。
  - title: 双层政策架构
    details: YAML 默认政策与数据库覆盖策略结合，支持低/中/高/危险四级风险控制。
  - title: 完整的审计链
    details: 所有操作记录审计日志，高风险操作需要双人审批，支持哈希链防篡改。
  - title: 数据分析链路
    details: ingest、worker、Redis Streams 与 ClickHouse 组成独立分析链路，文档单独归入 Analytics 入口。
footer: Apache-2.0 License | Copyright © 2024-present Croupier
---

## 什么是 Croupier

Croupier 是面向游戏运营与控制场景的 Server / Agent / SDK 平台，默认服务于单一游戏公司内部的多个游戏与多个环境。当前架构已经收敛到“统一 session 传输”方向：

- `Agent <-> Server`：默认采用 `TCP session`，默认启用 `TLS`
- `SDK <-> Agent`：默认采用 `TCP session`，默认不启用 `TLS`，按需开启
- 两条链路共享同一套 session 传输基座，只在首条握手消息和业务语义上区分子协议

## 系统架构

入口按受众分为**两个独立的面**，不要合并成一个「负载均衡层」：

- **Web 控制台面**（面向运营/管理员，公网或办公网）：L7 `dashboard nginx`，
  按请求分流 HTTP API / SSE 到 Server 的 `:18780`
- **Agent 面**（内网，面向游戏 VPC 出站连接）：L4 `haproxy`（`:19090`），
  按 TCP 长连接打散到 Server 的 control listener；连接语义是「断线重连即漂移」，
  与 HTTP 的按请求分流完全不同

```mermaid
graph TB
  subgraph "展示层"
    UI[Dashboard<br/>React + Ant Design Pro + ProComponents]
  end

  subgraph "入口 · Web 控制台面（L7，按请求分流）"
    NGINX[dashboard nginx<br/>HTTP API / SSE → :18780]
  end

  subgraph "入口 · Agent 面（L4，内网，按连接打散）"
    HAPROXY[haproxy :19090<br/>TCP 长连接 / 主动健康检查]
  end

  subgraph "控制层（可多实例 HA）"
    ServerA["Server A<br/>:18780 HTTP · :19090 TCP<br/>Registry / Dispatch / RBAC / Audit"]
    ServerB["Server B<br/>:18780 HTTP · :19090 TCP<br/>Registry / Dispatch / RBAC / Audit"]
    ServerA <-.->|实例互联<br/>转发 + fencing| ServerB
  end

  subgraph "共享状态"
    Shared[(共享目录<br/>成员表 / 注册表 / DB / Redis)]
  end

  subgraph "代理层（游戏 VPC，只出不进）"
    Agent1[Agent 1<br/>Session Client + Local Gateway]
    Agent2[Agent 2<br/>Session Client + Local Gateway]
  end

  subgraph "业务层"
    GS1[Game Server A<br/>SDK / Third-party App]
    GS2[Game Server B<br/>SDK / Third-party App]
    GS3[Game Server C<br/>SDK / Third-party App]
  end

  UI -->|HTTPS · REST / SSE| NGINX
  NGINX --> ServerA
  NGINX --> ServerB
  Agent1 -->|TCP Session + TLS| HAPROXY
  Agent2 -->|TCP Session + TLS| HAPROXY
  HAPROXY --> ServerA
  HAPROXY --> ServerB
  ServerA --- Shared
  ServerB --- Shared
  GS1 -->|TCP Session| Agent1
  GS2 -->|TCP Session| Agent2
  GS3 -->|TCP Session| Agent1
```

> 两个入口面互不共享：Web 面的 nginx 不碰 Agent 流量，Agent 面的 haproxy
> 不碰 HTTP。L4/L7 选型与 VRRP 消除 LB 单点的论证见
> [负载均衡](/operations/load-balancing)。

> Server 支持多实例高可用部署（`cluster.enabled`）：成员发现走共享存储（无
> seed、无静态 peers），请求落到非 owner 实例时经实例互联转发（一跳限制 +
> epoch fencing）。单实例部署默认关闭集群，零开销。详见
> [Server 多实例高可用设计](/architecture/server-ha-multi-instance.md)。

关键边界说明：

- `Server` 不再依赖反向直连 `Agent` 暴露的 `rpc_addr`
- `Agent` 本地监听只服务 `GameServer / SDK / 第三方应用`
- `Server -> Agent` 的 `Invoke / StartTask / CancelTask / Ops` 都应复用既有 `Agent-Server` session

## 核心能力

### 函数管理

- **OpenAPI 驱动注册**：通过 OpenAPI 规范快速注册函数
- **JSON Schema 表单展示**：根据函数契约生成字段与校验，JSON Schema form adapter 统一渲染调用、查询、创建、编辑与动作表单
- **Resource/PageSpec 编排**：CRUD Resource、独立操作、分页表格、报表和任务页由强类型 PageSpec 组合多个函数
- **统一调用链路**：控制面通过 Agent session 路由调用，本地接入通过 Agent gateway 完成

### 权限与安全

- **RBAC/ABAC 混合模型**：基于角色和属性的灵活权限控制
- **双层政策架构**：YAML 默认策略 + 数据库覆盖策略
- **四级风险控制**：低、中、高、危险四级，自动触发审批流程
- **双人审批规则**：高风险操作需要多人审批

### 作用域模型

- **单公司多游戏**：不引入 SaaS 多租户抽象，标准业务边界是 `game_id`
- **多环境治理**：`env` 表达 `dev/test/staging/prod` 等逻辑环境
- **归属与部署分离**：`scope` 表达业务归属，`target` 表达运行位置

### 可观测性

- **完整审计链**：所有操作记录审计日志
- **哈希防篡改**：审计记录通过哈希链关联，确保数据完整性
- **敏感字段脱敏**：自动脱敏密码、token 等敏感信息

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

# 查看 API 文档
curl http://localhost:18780/api/v1/
```

## 文档导航

按使用路径组织：

| 路径         | 入口                                                                                                                                          | 说明                                                             |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| **快速开始** | [指南](/guide/) · [快速开始](/guide/quick-start)                                                                                              | 安装、配置、Server + Agent 启动、最小闭环                        |
| **页面产品** | [函数管理](/guide/concepts/function-management) · [Page Studio](/guide/concepts/function-registration-ui) · [Resource Catalog](/api/resource) | 注册函数 → 自动生成页面 → Proposal Inbox 发布 → Console 受控执行 |
| **SDK**      | [SDK 概览](/sdks/)                                                                                                                            | Provider / Invoker / 配置 / 能力矩阵，6 种语言                   |
| **API**      | [API 参考](/api/)                                                                                                                             | REST 契约、鉴权、game/env scope、各资源 API                      |
| **运维**     | [监控](/operations/monitoring) · [数据分析](/analytics/)                                                                                      | 部署、监控、备份、分析、故障排除                                 |
| **开发**     | [开发](/development/) · [架构](/architecture/)                                                                                                | 架构、代码规范、扩展策略、发布规则                               |

历史设计与迁移文档归入 [架构 - 提案与迁移](/architecture/)（侧栏已折叠），不作为接入主路径。

## 技术栈

- **Go**：后端核心实现
- **TCP Session**：Agent/SDK 内部主链路
- **Protobuf**：接口定义与信封序列化
- **SQLite/PostgreSQL**：数据存储
- **React + Ant Design Pro + ProComponents**：管理界面与生成式页面运行时
- **VitePress**：文档站点

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
