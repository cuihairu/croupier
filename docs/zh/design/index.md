---
title: 设计简档
---

# 设计简档

本目录收录已实施或分批实施的设计简档，用来说明组件边界、业务流程和治理机制。

| 领域         | 文档                                                                                                                                                                                                     |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Agent 运行时 | [agent-core](./agent-core-design)、[Agent 清单](./agents-inventory)、[Supervisor](./agent-supervisor-design)。                                                                                           |
| 采集与 CDC   | [capture-agent](./capture-agent-design)、[CDC 过滤策略](./cdc-filter-policy-design)、[通用 CDC 核心库与回放](./cdc-library-design)。                                                                     |
| 运营流程     | [批量操作补偿](./batch-operations-compensation-design)、[高风险操作契约](./high-risk-operation-contract-design)、[玩家身份目标](./player-identity-target-model-design)、[事件报告](./incident-reports)。 |
| 控制台与页面 | [Agent 面板](./agent-dashboard-design)、[默认游戏菜单](./default-game-menus)、[菜单管理](./menu-management)。                                                                                            |
| 扩展能力     | [插件机制](./plugin-mechanism)、[游戏数据源](./game-data-sources)、[云资源申请](./cloud-resource-requests)。                                                                                             |

每份简档都会标注实现状态和已知边界。设计与实现不一致时，先修正设计再调整对外契约。
