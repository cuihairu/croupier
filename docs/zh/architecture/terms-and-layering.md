---
title: 术语与分层
---

# 术语与分层

这里区分业务范围、运行连接、可执行能力和运营呈现，避免同一个词在不同层被赋予不同含义。

| 术语           | 含义                                                   | 常见误用                      |
| -------------- | ------------------------------------------------------ | ----------------------------- |
| `gameId`       | 游戏归属边界。                                         | 当作数据库、主机或发布物。    |
| `env`          | `dev`、`prod` 这类逻辑环境。                           | 当作物理部署位置。            |
| Provider       | 连接到 Agent 的游戏进程或 SDK 服务。                   | 把每个本地进程都当作 Agent。  |
| Agent          | 持有 Server session 和本地 provider session 的运行时。 | 当作被动回调服务器。          |
| Function       | 最小可寻址业务操作。                                   | 当作运营页面或菜单项。        |
| Resource       | `player`、`mail` 这类聚合能力。                        | 自动等价于某张数据库表。      |
| PageSpec       | 强类型运营页面 DSL。                                   | 存放 React 组件树或静态文案。 |
| Published page | 控制台可用的不可变运行页。                             | 把草稿混入运营导航。          |

## Session 词汇

- **Shared session runtime** 提供 framing、多路复用、心跳、重连、drain 和背压。
- **Subprotocol** 标识某个边界上的消息契约，例如 `sdk-agent` 或 `agent-server`。
- **Provider session** 是 SDK 或本地服务注册函数时建立的本地连接。
- **Agent session** 是 Agent 到 Server 的上行控制面连接。

## 契约词汇

- `FunctionContract` 描述调用方式、schema、风险和身份。
- `CapabilitySemantics` 描述 collection、identity、CRUD、action、task 或 report 语义。
- `PageProposal` 由可验证的能力语义生成。
- `PageSpec` 负责编辑、校验和发布。
- `PublishedPageSpec` 冻结执行与呈现事实。

## 位置词汇

部署位置使用 target、agent 和 node 字段表达。基础设施变化时，环境语义保持稳定；一个逻辑环境可以在迁移或高可用切换过程中映射到不同集群。

## 相关页面

- [架构总览](/zh/architecture/)
