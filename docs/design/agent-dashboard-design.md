---
title: Agent 面板设计——每类 agent 单独一页：导航/页面构成/下钻/scope 联动（立项待批）
---

# Agent 面板设计（Agent Pages）

## 状态

- 状态: **Proposed（简档待用户过目，未动码）**，六件套之一（[Agent 清单](agents-inventory.md) 顶部有索引）。
- 面板令（用户令，2026-10-09）：**面板中每种 agent 单独一页**——侧栏按 agent 分导航；每页=实例列表+状态灯+健康探针结果+职责实时数据+日志入口；详情下钻沿用现有详情页模式；与《Agent 清单》互链。
- scope 令（用户令，2026-10-09）：所有 agent 页面与数据按 `game_id`+`env` 过滤——复用顶栏全局 game/env scope，切换即整页联动；注册无标签=不进面板；跨环境不串。scope 归属权威规则见 [Agent 清单](agents-inventory.md) §7。

## 1. 导航与页面清单

侧栏新增「Agents」组（Ops 域内），按 agent 分导航：

| 页面                               | 内容主体                       | 状态                   |
| ---------------------------------- | ------------------------------ | ---------------------- |
| sidecar-agent                      | 游戏侧实例：进程与注册函数     | v1 交付                |
| devops-agent                       | 内网实例：流水线/构建状态      | 随 agent 交付          |
| capture-agent                      | 数据面实例：变更事件与命中告警 | 随 agent 交付          |
| connector-agent / synthetics-agent | ——                             | **路线图占位，不出页** |

- 页面在对应 agent 落地前**不渲染空页**（devops/capture 页随 K 批交付渐进上线）；sidecar-agent 页 v1 即有（实例=现存全部注册 agent 中 type=sidecar 的）。
- 与既有 Ops/Nodes 页的边界见 §6。

## 2. 每页构成（四块，三页同构）

1. **实例列表 + 状态灯**：该 type 的全部实例（按 scope 过滤）；每行=实例 id / 所在主机 / 状态灯（聚合 healthprobe 结论：liveness / readiness / 心跳时长 last-heartbeat-age）/ 最近上报时间。
2. **健康探针卡（healthprobe）**：每个实例的 Liveness/Readiness/Heartbeat 结论 + **故障窗口 timeline**（历史不可用时间段列表：开始/恢复/持续时长，未恢复标「进行中」——直接回答「什么时间段服务不可用」）。
3. **职责实时数据**（每页专属，见 §3）。
4. **日志与执行记录**：日志入口（复用 supervisor 日志下载链）+ 「执行记录」列表（execlog：筛选 执行人/结果/时间段，详情下钻看入参/结果全文）。

每页头部链接《Agent 清单》对应小节（职责边界/scope 规则速查）。

## 3. 职责实时数据（每页专属）

| 页面          | 实时数据                                                                                                                                                 |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| sidecar-agent | 进程（supervisor 快照：state/pid/uptime/restarts/RSS/CPU/flags + start/stop/restart 操作）与**注册函数**（本实例注册的 function 列表：id/描述/审批要求） |
| devops-agent  | 流水线/构建状态（v1 = GitHub Actions runs 流：repo/workflow/branch/结论/时长/runner，命中规则的事件高亮）                                                |
| capture-agent | 变更事件流（表/操作/delta 摘要/账号，可按表过滤）与**命中告警**（规则/严重度/命中详情入口）                                                              |

## 4. 实例详情下钻

沿用现有详情页模式（Ops/Nodes 的 SupervisorDrawer 先例：列表行 → 详情抽屉 → Tabs 分区）：

- Tab 1 概览：实例元数据（id/主机/版本/scope 标签/注册时间）+ 探针卡；
- Tab 2 职责数据：§3 内容按实例过滤；
- Tab 3 执行记录：execlog 按实例过滤；
- Tab 4 日志：supervisor 日志查看/下载。

## 5. scope 联动（页面侧）

- 三页复用**顶栏全局 game/env scope**（与既有 scoped 组同口径），切换即整页数据联动（列表/探针/实时数据/执行记录/日志全量刷新）。
- 全部列表/详情 API 请求带当前 scope 参数；服务端按 scope 过滤后才返回——前端不做跨 scope 兜底。
- 切换 scope 后实例列表可能为空（该环境无此 type 实例）：显示空态与「该 scope 无实例」文案，不显示其他环境数据。

## 6. 与既有 Ops/Nodes 页的边界

- **Ops/Nodes 保留**：节点运维面（drain/undrain/restart、注册中心视角的节点列表）职责不变；其 supervisor 列与 SupervisorDrawer（S1/S2 已交付）维持原位。
- **Agents 三页新增**：按 agent type 组织的健康+职责数据面；sidecar-agent 页的进程 Tab 与 Nodes 页 supervisor 视图**同源同数据**（同一 server API），入口并存互链——是否在 sidecar 页稳定后把 Nodes 页 supervisor 视图归并过来，作为面板整理项另行报批（v1 不动既有页，避免一次性大迁移）。

## 7. server API 需求（增量，随批次交付）

| 端点（拟）                                                          | 语义                                                                  |
| ------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `GET /api/v1/ops/agents?type=&gameId=&env=`                         | 按 type+scope 列实例与探针聚合状态（注册标签补齐后天然可查）          |
| `GET /api/v1/ops/agents/:id/probe/timeline?since=`                  | 故障窗口历史（healthprobe 时间线）                                    |
| `GET /api/v1/ops/agents/:id/executions?operator=&result=&from=&to=` | 执行记录列表（execlog）+ 详情端点                                     |
| 职责数据端点                                                        | sidecar 复用既有 processes/registry；devops/capture 随 agent 批次新增 |

- 响应契约沿用平台规范（成功直返业务 JSON，错误 `{error,message,details}`）；下载端点走 Content-Disposition 例外。

## 8. 已知边界（诚实清单）

- devops/capture 页依赖对应 agent 落地（K 批 + C/D 批），页面先出骨架还是随 agent 一起交付，随实施批次报批。
- 注册无标签的实例「不进面板」：v1 口径=彻底不可见（注册事件落 server 日志），置灰展示方案不预留。
- 故障窗口历史在 server 侧为内存环起步（与 S2 事件环同生命周期策略），持久化需求出现时按迁移契约走编号迁移再升级。
