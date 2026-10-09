---
title: Agent 清单——三驻留 agent 定名/职责边界/公共模块/面板分页/scope 归属（立项待批）
---

# Agent 清单（Agent Inventory）

## 状态

- 状态: **Proposed（定名已由用户拍板，实现未动）**，与 [agent-core 简档](agent-core-design.md)、[capture-agent 简档](capture-agent-design.md) 同批报审。
- **定名（用户令，2026-10-09）**：三驻留 agent = **`sidecar-agent`** / **`devops-agent`** / **`capture-agent`**；公共模块 = **`healthprobe`**（健康探针）、**`execlog`**（执行审计）；路线图登记 `connector-agent` / `synthetics-agent`（**只登记不实现**）。旧候选（gameserver-agent / cicd-agent / cdc-agent / ci-agent）一律弃用。
- 本页是**唯一权威定名与职责边界清单**；每个 agent 的实现文档开头有同口径「作用」节；与 [agent-core 能力清单表](agent-core-design.md)（§2）互链。
- 本页目录：§1 全景 → §2–§4 三驻留 agent（逐个：职责/部署/监控对象/输入输出/边界/探针/审计/scope）→ §5 公共模块（healthprobe/execlog/herald 出口）→ §6 路线图占位 → §7 scope 归属规则 → §8 面板分页（详设另页）。
- **六件套索引（顺序令，2026-10-09：文档先行，过目前不动码）**：①《Agent 清单》=本页；②能力清单+目录树=[agent-core 简档](agent-core-design.md) §2/§4；③healthprobe+execlog 设计=[agent-core 简档](agent-core-design.md) §6；④dash 页面清单=[Agent 面板设计](agent-dashboard-design.md)；⑤scope 规则=本页 §7（页面侧联动=[Agent 面板设计](agent-dashboard-design.md) §5）；⑥herald 对接=[herald 对接简档](agent-herald-integration.md)。

## 1. 全景

| 名称               | 层         | 状态               | 一句话                                  | 实现文档                                      |
| ------------------ | ---------- | ------------------ | --------------------------------------- | --------------------------------------------- |
| `sidecar-agent`    | 驻留 agent | 已有主体（定名中） | 游戏服务器旁的 GM 基础设施代理          | [agent-core 简档](agent-core-design.md) §1    |
| `devops-agent`     | 驻留 agent | 新建（简档待批）   | 内网 CI/CD/打包/部署链路状态观测        | [agent-core 简档](agent-core-design.md) §7    |
| `capture-agent`    | 驻留 agent | 新建（简档待批）   | 防私改数据库：变更捕获与三道闸          | [capture-agent 简档](capture-agent-design.md) |
| `healthprobe`      | 公共模块   | 首批交付物（待批） | Liveness/Readiness/Heartbeat 三语义探针 | [agent-core 简档](agent-core-design.md) §6    |
| `execlog`          | 公共模块   | 新建（待批）       | 统一执行审计（audit-first）             | [agent-core 简档](agent-core-design.md) §6    |
| `connector-agent`  | 路线图占位 | 只登记不实现       | 第三方服务接入                          | [agent-core 简档](agent-core-design.md) §8    |
| `synthetics-agent` | 路线图占位 | 只登记不实现       | 外部合成探测                            | [agent-core 简档](agent-core-design.md) §8    |

## 2. sidecar-agent（游戏服务器）

| 维度               | 内容                                                                                                               |
| ------------------ | ------------------------------------------------------------------------------------------------------------------ |
| 职责一句话         | 游戏服务器旁的 GM 基础设施代理：函数注册调用（Function Registry）+ 进程监管（Supervision）+ 自探活                 |
| 部署位置（贴谁跑） | **游戏服务器本机/同网段**（Sidecar 形态，随游戏服部署）；连 croupier server 上行注册                               |
| 监控对象           | 同机游戏进程（`managedProcesses`，S1/S2 快照/拉起/熔断）、本机资源（gopsutil 采集）、本实例注册的函数              |
| 输入               | server 下发：函数调用、进程操作（start/stop/restart）、配置同步（configsync）、规则/白名单                         |
| 输出               | 注册/心跳/指标捎带（MetricsReport）、supervisor 快照与事件、执行审计（execlog）                                    |
| 与其他 agent 边界  | **不探 CI 端点**（devops 专属）、**不读业务库变更日志**（capture 专属）、不做第三方接入/外网拨测（路线图占位专属） |
| 健康探针           | Liveness（游戏进程存活，与 supervisor 采样同源）+ Heartbeat（心跳打点）                                            |
| 执行审计           | 函数调用/进程操作/配置热更每次执行落 execlog（operator=GM 账号或 system）                                          |
| scope 归属         | 注册携带 `game_id`+`env`；进程/函数/审计数据全按 scope 过滤                                                        |

## 3. devops-agent（内网 CI/CD/打包/部署）

| 维度               | 内容                                                                                                               |
| ------------------ | ------------------------------------------------------------------------------------------------------------------ |
| 职责一句话         | 内网侧观测 CI/CD/打包/部署链路状态（v1 = GitHub Actions / 自建 runner 构建状态），命中异常上报告警页               |
| 部署位置（贴谁跑） | **内网、靠近 runner 的主机**（能访问 GitHub API / CI 内网端点，无需公网入站）                                      |
| 监控对象           | 流水线（Pipeline）构建状态：失败/超时/持续不绿/停滞；打包与部署节点状态为扩展位                                    |
| 输入               | CI 数据源（REST 轮询）+ 面板下发规则（仓库/工作流白名单、阈值，game-scoped 配置）                                  |
| 输出               | 告警事件（现有告警页）+ 构建状态快照上报 + 执行审计（execlog）                                                     |
| 与其他 agent 边界  | **只观测不写操作**：不触发构建、不改 CI 配置、不执行部署；不碰游戏进程（sidecar 专属）、不读库日志（capture 专属） |
| 健康探针           | Readiness（CI 端点/runner 可达性）+ Heartbeat                                                                      |
| 执行审计           | 轮询/规则评估摘要定期落 execlog；**未来若开命令类操作（如部署动作）必须 audit-first 放行**                         |
| scope 归属         | 注册携带 `game_id`+`env`；流水线数据/审计按 scope 过滤                                                             |

## 4. capture-agent（防私改数据库）

| 维度               | 内容                                                                                                                                |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------- |
| 职责一句话         | **防私改数据库**：读游戏库变更日志（Change Data Capture），变更发生即告警——谁、怎么改、改了什么、有无流水背书                       |
| 部署位置（贴谁跑） | **游戏网络内、贴近游戏业务库**（与 sidecar-agent 同机或邻近）                                                                       |
| 监控对象           | 道具相关表的行级变更（insert/update/delete）                                                                                        |
| 输入               | binlog / WAL 逻辑复制 + 面板下发规则（白名单/阈值，game-scoped）                                                                    |
| 输出               | 告警事件（带规则命中详情）+ 位点/命中统计上报 + 执行审计（execlog）                                                                 |
| 与其他 agent 边界  | 只管数据变更合规：不做库指标健康（db-monitoring 姊妹件）、不碰函数通道（sidecar 专属）、不碰 CI/CD（devops 专属）；**只告警不处置** |
| 健康探针           | Readiness（库连通/复制流健康）+ Heartbeat                                                                                           |
| 执行审计           | 位点操作/白名单配置变更落 execlog；**白名单等命令类操作必须 audit-first 放行**                                                      |
| scope 归属         | 注册携带 `game_id`+`env`；变更事件/告警/审计按 scope 过滤，跨环境不串                                                               |

## 5. 公共模块（三 agent 共用，不各写各的）

### 5.1 healthprobe（健康探针，首批交付物）

三语义对标 k8s probe 术语（定稿见 agent-core §6.1）：**Liveness**（存活）/ **Readiness**（依赖就绪）/ **Heartbeat**（周期心跳）。三 agent 消费映射：

| agent         | Liveness     | Readiness           | Heartbeat |
| ------------- | ------------ | ------------------- | --------- |
| sidecar-agent | 游戏进程存活 | server 上行链路可达 | 通用打点  |
| devops-agent  | 自身存活     | CI 端点可达         | 通用打点  |
| capture-agent | 自身存活     | 库连通/复制流健康   | 通用打点  |

探针结果直接上面板：每页实例列表带状态灯 + liveness/readiness 结论 + 心跳时长（last heartbeat age）。

**按 system 角色建模（故障窗口，用户令 2026-10-09）**：探针不只看当下活死——每个受监控系统（target）记录故障窗口（开始/恢复/持续时长），历史可查，探活数据带 timeline；排查时直接回答「什么时间段服务不可用」。模型与留存见 agent-core §6.1，面板展示见 [Agent 面板设计](agent-dashboard-design.md) §2。

### 5.2 execlog（统一执行审计）

**每个 agent 的每次执行都留审计记录**（用户令，2026-10-09）。字段/audit-first 放行闸/保存期导出配置位见 agent-core §6.2。面板侧：各 agent 页「执行记录」列表 + 筛选（执行人/结果/时间段）+ 详情下钻（入参/结果全文），全按 scope 过滤。

### 5.3 herald（告警投递出口）

capture 告警 / healthprobe 故障窗口 / devops 构建失败经 **herald**（通知编排与投递平台）投递——croupier 当接入方（courier 式接法），**渠道/收件人由 herald 侧配置**，croupier 只报事件不配收件人；告警页仍是真值，herald 不可达不阻塞告警链。接法见 [herald 对接简档](agent-herald-integration.md)。

## 6. 路线图占位（只登记不实现，面板不建页）

| Agent              | 定位                               | 对标（命名来源）             |
| ------------------ | ---------------------------------- | ---------------------------- |
| `connector-agent`  | 第三方服务接入（外部系统事件汇入） | Datadog Integrations / n8n   |
| `synthetics-agent` | 外部合成探测（外网拨测线上端点）   | Datadog Synthetics / Pingdom |

立项触发条件与同构形态见 agent-core §8；**面板侧栏不为占位 agent 出页**。

## 7. scope 归属规则（game_id + env，强制）

1. **注册携带标签**：三类 agent 上线注册必须携带 `game_id`+`env` 标签；**无标签 = 不进面板**（注册事件落 server 日志以便排查，不静默丢弃）。
2. **顶栏全局 scope**：三类 agent 页复用顶栏全局 game/env scope（与既有 scoped 组同口径），切换即整页数据联动。
3. **数据全按 scope 过滤**：实例列表、健康探针结果、职责实时数据（sidecar 进程与函数 / devops 流水线 / capture 变更事件与告警）、execlog 执行记录、日志入口——一律按 scope 过滤，**跨环境不串**。
4. agent 离线/标签变更：实例按最后已知 scope 归属展示并标注数据过期，不跨 scope 兜底。

## 8. 面板分页（每类 agent 单独一页）

**详设见 [Agent 面板设计](agent-dashboard-design.md)**（导航/页面构成/下钻/scope 联动/server API），要点：

- **侧栏按 agent 分导航**：sidecar-agent 页 / devops-agent 页 / capture-agent 页（路线图占位不出页）。
- **每页四块**：实例列表+状态灯（探针结论/心跳时长/故障窗口 timeline）→ 职责实时数据（sidecar：进程与注册函数；devops：流水线/构建状态；capture：变更事件与命中告警）→ 日志入口 → 「执行记录」（execlog，筛选执行人/结果/时间段）。
- **实例详情下钻**沿用现有详情页模式（Ops/Nodes 抽屉先例）。
- **与本页互链**：各 agent 页头部链接《Agent 清单》对应小节。

## 9. 已知边界与待批点

- sidecar-agent 主体已存在（internal/agent + internal/app/agent），定名与面板分页属增量，不改运行时行为。
- devops-agent v1 只观测无命令面，其「命令类执行 audit-first」约束在扩展位启用时生效；capture 白名单变更 v1 即命令类，按 audit-first 落地。
- execlog server 侧保存期默认值与导出格式随实施报批；agent 本地文件保留期有限（轮转），真值以 server 审计存储为准。
- 无标签注册「不进面板」的展示口径（彻底不可见 vs 列表置灰）随面板实施报批，默认彻底不可见。
