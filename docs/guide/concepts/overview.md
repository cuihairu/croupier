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
| Schema 驱动 UI | OpenAPI / JSON Schema 作为契约输入，运行时 UI 统一使用 Formily Schema |
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
| **Page Studio** | 页面装配层 | ResourceSpec / OperationSpec / PageSpec 草稿 | 生成默认页面、编辑 Formily Page Schema、预览、发布、回滚 |
| **运行控制台** | 执行层 | PublishedPageSpec / ConsoleMenuSpec | 面向最终用户/运营执行业务操作 |

三者串成一条主流程：

```
函数注册 → 函数目录（确认能力） → Page Studio（装配页面） → 运行控制台（执行）
```

- 函数目录只负责单个函数的元数据、Schema、实例和单函数调用，不负责最终业务页面。
- Page Studio 把多个函数组合成 Entity Page、Operation Page、Task Page 或 Report Page，管理草稿、发布版本和回滚。
- 不是所有函数都进入 Entity Page。只有明确围绕同一资源生命周期展开的函数才进入 Entity Page。
- 默认页面由 Server 根据 FunctionSpec / ResourceSpec / OperationSpec 生成建议，用户确认后再编辑和发布。
- 运行控制台只展示已发布 PageSpec，动态菜单不依赖静态国际化文件。

详见 [Page Studio](./object-workspace.md)、[函数管理](./function-management.md) 和 [Dashboard Resource/Page 模型](../../architecture/dashboard-page-model.md)。

## Dashboard 核心概念

当前 Dashboard 只有一套目标模型：

```text
FunctionSpec -> ResourceSpec + OperationSpec -> PageSpec -> PublishedPageSpec -> ConsoleMenuSpec
```

### FunctionSpec

`FunctionSpec` 是函数注册后的归一化能力描述。它说明“有什么函数、怎么调用、输入输出是什么、风险是什么”。

它不等于页面，也不决定左侧菜单。

### ResourceSpec

`ResourceSpec` 是 Dashboard 组织页面用的资源或能力域，例如 `player`、`mail`、`inventory`、`analytics`。

它不是数据库表，也不是传统 CRUD Entity。一个 Resource 可以承载 CRUD 操作，也可以承载游戏运营里的封禁、发奖、发邮件、补单等动作。

### OperationSpec

`OperationSpec` 描述函数在资源中的业务动作和候选诊断，不保存最终页面位置。

必须区分：

| 字段 | 含义 | 示例 |
| --- | --- | --- |
| `operation` | 业务动作 key | `ban`、`grant`、`send`、`list` |
| `PageCandidate.kind` | Server 生成的候选页面形态 | `entity`、`operation`、`task`、`report` |
| `PageFunctionBinding.usage` | PageSpec 中页面实际消费函数的方式 | `query`、`detail`、`action`、`task`、`report` |

`operation` 只能表示业务动作 key。SDK/OpenAPI 注册不提供 `operationKind`、`placement`、菜单或动态显示字段；页面类型、按钮位置和多语言标题必须由 PageSpec 表达。

### PageSpec

`PageSpec` 是业务页面编排，必须是 Formily JSON Schema。它负责查询区、分页、表格、详情、弹窗、批量操作、任务状态和报表图表。

Page 可以组合多个函数。分页查询属于 Page 的状态和字段映射，不属于单函数输入表单。

### Page Studio

Page Studio 管理 PageSpec 草稿。它可以展示 Server 生成的默认页面建议，也允许用户编辑、预览、校验、发布和回滚。

Page Studio 不是运行控制台。未发布草稿不能出现在运行控制台左侧菜单里。

### 运行控制台

运行控制台只消费已发布页面：

```text
PublishedPageSpec[] -> ConsoleMenuSpec
```

动态分类、页面标题和菜单多语言来自 PublishedPageSpec 的强类型字段。前端静态 locale 文件只用于固定系统菜单。

## 不再推荐的理解

以下旧概念不应再当作新设计依据：

- `历史 REQ/REP` 作为主链路模型
- `Server -> Agent` 直接回拨
- `rpc_addr` 作为长期运行时入口
- SDK 开本地监听端口给 `Agent`
- Resource API 直接修改业务对象数据
- 自定义 `layout` 枚举作为运行时 UI 协议
- 动态菜单分类写入静态国际化文件
