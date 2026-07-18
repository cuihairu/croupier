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

## 界面分层：函数目录 vs 对象工作台 vs 运行控制台

Croupier 的管理界面在“函数”这一模块下分为三个层次，各自职责不同，不重复：

| 层次 | 定位 | 数据来源 | 典型操作 |
| --- | --- | --- | --- |
| **函数目录** | 能力供给层 | `listDescriptors()` + `getFunctionSummary()` | 确认函数是否注册成功、Schema 是否正确、有没有可调用实例、单函数 invoke |
| **对象工作台** | 页面装配层 | `workspace_configs` + `listDescriptors()` | 以对象（objectKey）为维度，把多个函数组织成 Tabs、表单、仪表盘等页面 |
| **运行控制台** | 发布验证层 | 已发布 workspace configs | 面向最终用户/运营，验证页面表现、权限和运行结果 |

三者串成一条主流程：

```
函数目录（确认能力） → 对象工作台（装配页面） → 运行控制台（发布验证）
```

- 函数目录只负责单个函数的元数据、Schema、实例和单函数调用，不负责最终业务页面。
- 对象工作台把多个函数组合成面向业务对象（如 `player`、`claim`、`order`）的完整界面，管理草稿、发布版本和回滚。
- 首次访问对象工作台时自动创建默认配置（空 Tabs），不会 404。

详见 [对象工作台](./object-workspace.md) 和 [函数管理](./function-management.md)。

## 不再推荐的理解

以下旧概念不应再当作新设计依据：

- `历史 REQ/REP` 作为主链路模型
- `Server -> Agent` 直接回拨
- `rpc_addr` 作为长期运行时入口
- SDK 开本地监听端口给 `Agent`
