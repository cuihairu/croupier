---
title: 分层
---

# 分层

Croupier 把控制面职责和游戏进程接入分开。每一层都有明确归属和契约。

## 组件分层

| 层               | 归属     | 职责                                                             |
| ---------------- | -------- | ---------------------------------------------------------------- |
| 控制台与 API     | Server   | 认证、RBAC、审计、提案、发布、监控和运营侧执行。                 |
| 注册表与策略     | Server   | FunctionContract、资源语义、快照、审批和执行策略。               |
| Agent session    | Agent    | 维护一条到 Server 的上行会话，处理心跳、重连、drain 和请求复用。 |
| Provider gateway | Agent    | 接入 SDK 与游戏进程的本地会话，并把请求路由到健康实例。          |
| 嵌入式 SDK       | 游戏进程 | 注册函数、调用本地处理器，并上报进程与 provider 元数据。         |

## 技术边界

```text
运营界面
  -> Server HTTP/API
    -> Server 注册表与策略
      -> Agent/Server session
        -> Agent 本地网关
          -> SDK provider session
            -> 游戏处理器
```

- Server 不会反向拨号 Agent；上行会话由 Agent 主动建立。
- Agent 不会反向拨号 SDK provider；本地会话由 provider 主动建立。
- 请求复用既有 session，不在每次调用时新开链路。
- protobuf 承载平台信封，JSON 承载业务 payload。

## 数据与页面分层

| 边界       | 模型                                          |
| ---------- | --------------------------------------------- |
| 可执行单元 | `FunctionContract`                            |
| 业务能力   | `ResourceCapability` 与 `CapabilitySemantics` |
| 可编辑页面 | `PageProposal` 与 `PageSpec`                  |
| 运行时页面 | `PublishedPageSpec`                           |
| 导航       | `ConsoleMenuSpec`                             |

这样做的目的是避免一个函数描述符同时变成隐式页面、菜单、权限或部署边界。

## 位置边界

`gameId + env` 表达业务范围，不表达数据库、主机或集群。物理位置由 target、agent、node 表达，因此逻辑环境可以独立于基础设施完成迁移或高可用调整。

## 相关页面

- [架构总览](/zh/architecture/)
