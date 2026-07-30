---
title: 系统概览
icon: circle-info
order: 1
category:
  - 核心概念
tag:
  - 架构
  - 概览
---

# 系统概览

Croupier 是面向游戏运营与控制场景的 Server / Agent / SDK 平台。
当前架构已经明确收敛到“统一 session 传输”：

- `Agent <-> Server`：`TCP session + TLS`
- `SDK <-> Agent`：`TCP session`，默认不启用 `TLS`

在业务模型上，Croupier 默认服务于单一游戏公司内部的多个游戏与多个环境：

- 标准业务边界是 `game_id + env`
- `game_id` 表达游戏归属
- `env` 表达逻辑环境，例如 `dev`、`test`、`staging`、`prod`
- `env` 不直接等于数据库、节点或集群
- 部署位置应通过 `target`、`agent`、`node` 等单独概念表达

## 核心特点

| 特性 | 说明 |
| --- | --- |
| 统一 session 传输 | 双向请求、重连、heartbeat、drain、背压 |
| 函数注册驱动 | SDK / Agent 上报 function/provider/process 能力 |
| 契约驱动后台 | OpenAPI / JSON Schema 提供能力契约，平台生成可发布的 ProComponents 页面 |
| JSON payload | 用户业务数据默认 JSON |
| protobuf 信封 | 平台控制字段与消息路由统一 protobuf |

## 关键组件

### Server

- 保存 registry
- 做 RBAC / 审批 / 审计
- 负责路由 invoke/task/ops 请求
- 持有 `Agent session`

### Agent

- 主动连接 `Server`
- 本地接入 `GameServer / SDK / 第三方应用`
- 在本地和上游两侧维护 session

### SDK

- 作为嵌入式客户端主动连接 `Agent`
- 默认不监听本地端口
- 通过 provider session 暴露函数能力

## 关键术语

### shared session runtime

指共享的 session 运行时基座：

- `tcp/tls`
- framing
- request/response 复用
- reconnect
- heartbeat
- drain
- backpressure

### subprotocol

指运行在 shared session runtime 之上的具体子协议：

- `sdk-agent subprotocol`
- `agent-server subprotocol`

它不是“配置模板”，而是“不同边界上的应用层协议变体”。

## 界面分层：函数目录 vs Page Studio vs 运行控制台

Croupier 的管理界面在“函数”这一模块下分为三个层次，各自职责不同，不重复：

| 层次 | 定位 | 数据来源 | 典型操作 |
| --- | --- | --- | --- |
| **函数目录** | 能力供给层 | FunctionSpec / 函数注册目录 | 确认函数是否注册成功、Schema 是否正确、有没有可调用实例、单函数 invoke |
| **Resource Catalog** | 能力语义层 | FunctionContract / CapabilitySemantics | 审核资源、CRUD/任务/报表语义与生成诊断 |
| **Page Studio** | 页面装配层 | PageProposal / PageDraft | 接受默认页面、语义化编辑、预览、发布、合并、回滚 |
| **运行控制台** | 执行层 | PublishedPageSpec / ConsoleMenuSpec | 面向最终用户/运营执行业务操作 |

三者串成一条主流程：

```
函数注册 → 函数目录（确认能力） → Resource Catalog（确认语义） → Page Studio（装配页面） → 运行控制台（执行）
```

- 函数目录只负责单个函数的元数据、Schema、实例和单函数调用，不负责最终业务页面。
- Page Studio 把多个函数组合成 ResourcePage、OperationPage、TaskPage 或 ReportPage，管理草稿、发布版本和回滚。
- CRUD Resource 是默认高质量路径；非 CRUD 函数也有独立 Operation/Task/Report 页面，不会被强行塞入资源页。
- 默认页面由 Server 根据 FunctionContract / CapabilitySemantics 生成 Proposal，用户可以直接发布或局部编辑。
- 运行控制台只展示已发布 PageSpec，动态菜单不依赖静态国际化文件。

详见 [函数管理](./function-management.md)、[函数注册与默认界面](./function-registration-ui.md) 和 [Dashboard Resource/Page 模型](../../architecture/dashboard-page-model.md)。

## Dashboard 核心概念

当前 Dashboard 只有一套目标模型：

```text
FunctionContract -> ResourceCapability -> CapabilitySemantics -> PageProposal -> PageSpec -> PublishedPageSpec -> ConsoleMenuSpec
```

### FunctionContract

`FunctionContract` 是函数注册后的归一化能力描述。它说明“有什么函数、怎么调用、输入输出是什么、风险是什么”。

它不等于页面，也不决定左侧菜单。

### ResourceCapability

`ResourceCapability` 是 Dashboard 组织页面用的资源能力集合，例如 `player`、`mail`、`inventory`、`analytics`。

一个 Resource 可以承载 CRUD 操作，也可以承载游戏运营里的封禁、发奖、发邮件、补单等动作。

### CapabilitySemantics

`CapabilitySemantics` 描述 collection、identity、CRUD、action、task、report 等可验证业务用法，不保存最终页面位置、列或菜单。

SDK/OpenAPI 的 `operation` 仍只是业务动作 key。SDK/OpenAPI 可提供受控 `capability`，但不提供菜单、列、mapping、按钮位置或动态显示字段；页面类型、动作位置和多语言标题只能由 PageSpec 表达。

### PageSpec

`PageSpec` 是强类型业务页面编排，负责查询区、分页、表格、详情、弹窗、批量操作、任务状态和报表图表。它不包含 React 组件树，由 ProComponents renderer 负责显示。

Page 可以组合多个函数。分页查询属于 Page 的状态和字段映射，不属于单函数输入表单。

### Page Studio

Page Studio 管理 PageProposal 和 PageSpec 草稿。它可以展示 Server 生成的默认页面建议，允许直接发布，也允许用户语义化编辑、预览、校验、合并、发布和回滚。

Page Studio 不是运行控制台。未发布草稿不能出现在运行控制台左侧菜单里。

### 运行控制台

运行控制台只消费已发布页面：

```text
PublishedPageSpec[] -> ConsoleMenuSpec
```

动态分类、页面标题和菜单多语言来自 PublishedPageSpec 的强类型字段。前端静态 locale 文件只用于固定系统菜单。

## 当前边界

- Agent 与 SDK 通过 session 模型接入，Server 调用复用已建立链路。
- Resource API 管理能力语义，不直接修改业务对象数据。
- PageSpec 是运行控制台唯一页面协议，动态菜单分类来自 PublishedPageSpec。
- 静态国际化文件只负责固定系统菜单，不承载动态页面事实。
