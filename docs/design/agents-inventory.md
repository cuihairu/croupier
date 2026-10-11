---
title: Agent 清单——三驻留 agent 定名/职责边界/公共模块/面板分页/scope 归属（立项待批）
---

# Agent 清单（Agent Inventory）

## 状态

- 状态: **以代码现实为准（2026-10-11 核）**：公共模块 `healthprobe`/`execlog` **已建成**（agent-core K1–K4 落地，见 [agent-core 简档](agent-core-design.md) §5.1）；`capture-agent` **部分实施**（C1 骨架 2026-10-10：注册/心跳/事件流/位点，闸层 C2 待批，`cmd/capture-agent` + `agents/capture/` 均在）；`devops-agent` **未立项**（无任何目录，简档已按 2026-10-11 运维口径重写，待批）；`sidecar-agent` = 现有主体已部分切 core，**现实目录/二进制仍为 `cmd/agent`**（定名未落，改名方案见 todo.md 待确认）。
- **定名（用户令，2026-10-09）**：三驻留 agent = **`sidecar-agent`** / **`devops-agent`** / **`capture-agent`**；公共模块 = **`healthprobe`**（健康探针）、**`execlog`**（执行审计）；路线图登记 `connector-agent` / `synthetics-agent`（**只登记不实现**）。旧候选（gameserver-agent / cicd-agent / cdc-agent / ci-agent）一律弃用。
- **角色口径重定（用户令，2026-10-11，覆盖 2026-10-09 旧口径）**：`sidecar-agent` = **外网侧、只做业务相关功能**（监控游戏进程、注册函数、汇报健康信息含主机与定时任务）；`devops-agent` = **内网堡垒机式远程接入代理**（可操作、更高权限、必须记录；旧「CI/CD 只观测」口径整节废弃）；`capture-agent` = **CDC**（防私改库）。
- 本页是**唯一权威定名与职责边界清单**；每个 agent 的实现文档开头有同口径「作用」节；与 [agent-core 能力清单表](agent-core-design.md)（§2）互链。
- 本页目录：§1 全景 → §2–§4 三驻留 agent（逐个：职责/部署/监控对象/输入输出/边界/探针/审计/scope）→ §5 公共模块（healthprobe/execlog/herald 出口）→ §6 路线图占位 → §7 scope 归属规则 → §8 面板分页（详设另页）。
- **六件套索引（顺序令，2026-10-09：文档先行，过目前不动码）**：①《Agent 清单》=本页；②能力清单+目录树=[agent-core 简档](agent-core-design.md) §2/§4；③healthprobe+execlog 设计=[agent-core 简档](agent-core-design.md) §6；④dash 页面清单=[Agent 面板设计](agent-dashboard-design.md)；⑤scope 规则=本页 §7（页面侧联动=[Agent 面板设计](agent-dashboard-design.md) §5）；⑥herald 对接=[herald 对接简档](agent-herald-integration.md)。

## 1. 全景

| 名称               | 层         | 状态                                                                | 一句话                                                 | 实现文档                                      |
| ------------------ | ---------- | ------------------------------------------------------------------- | ------------------------------------------------------ | --------------------------------------------- |
| `sidecar-agent`    | 驻留 agent | 已有主体（现实目录 `cmd/agent`，定名待落）                          | 游戏服务器旁（外网侧）的 GM 基础设施代理               | [agent-core 简档](agent-core-design.md) §1    |
| `devops-agent`     | 驻留 agent | 未立项（简档按运维口径重写待批）                                    | 内网堡垒机式远程接入代理（SSH/RDP/VNC，可操作+全留痕） | [agent-core 简档](agent-core-design.md) §7    |
| `capture-agent`    | 驻留 agent | 部分实施（C1 骨架 2026-10-10：注册/心跳/事件流/位点；闸层 C2 待批） | 防私改数据库：CDC 变更捕获与三道闸                     | [capture-agent 简档](capture-agent-design.md) |
| `healthprobe`      | 公共模块   | **已建成**（接线待消费方）                                          | Liveness/Readiness/Heartbeat 三语义探针                | [agent-core 简档](agent-core-design.md) §6    |
| `execlog`          | 公共模块   | **已建成**（接线待消费方）                                          | 统一执行审计（audit-first）                            | [agent-core 简档](agent-core-design.md) §6    |
| `connector-agent`  | 路线图占位 | 只登记不实现                                                        | 第三方服务接入                                         | [agent-core 简档](agent-core-design.md) §8    |
| `synthetics-agent` | 路线图占位 | 只登记不实现                                                        | 外部合成探测                                           | [agent-core 简档](agent-core-design.md) §8    |

## 2. sidecar-agent（游戏服务器）

| 维度               | 内容                                                                                                                                                           |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 职责一句话         | 游戏服务器旁（**外网侧**）的 GM 基础设施代理，**只做业务相关功能**：函数注册调用（Function Registry）+ 进程监管（Supervision）+ 自探活 + 主机/定时任务信息汇报 |
| 部署位置（贴谁跑） | **游戏服务器本机/同网段（外网侧）**（Sidecar 形态，随游戏服部署）；连 croupier server 上行注册                                                                 |
| 监控对象           | 同机游戏进程（`managedProcesses`，S1/S2 快照/拉起/熔断）、本机资源（gopsutil 采集）、本实例注册的函数、主机信息与定时任务（健康汇报口径）                      |
| 输入               | server 下发：函数调用、进程操作（start/stop/restart）、配置同步（configsync）、规则/白名单                                                                     |
| 输出               | 注册/心跳/指标捎带（MetricsReport）、supervisor 快照与事件、执行审计（execlog）、主机与定时任务健康信息                                                        |
| **禁止项**         | **不得具备堡垒机类远程接入能力**（SSH/RDP/VNC 代登入内网机器属 devops-agent 专属）；sidecar 只读+监管自己的游戏进程，不开交互式终端、不代理文件传输            |
| 与其他 agent 边界  | 不做远程接入/堡垒机（devops 专属）、**不读业务库变更日志**（capture 专属）、不做第三方接入/外网拨测（路线图占位专属）                                          |
| 健康探针           | Liveness（游戏进程存活，与 supervisor 采样同源）+ Heartbeat（心跳打点）                                                                                        |
| 执行审计           | 函数调用/进程操作/配置热更每次执行落 execlog（operator=GM 账号或 system）                                                                                      |
| scope 归属         | 注册携带 `game_id`+`env`；进程/函数/审计数据全按 scope 过滤                                                                                                    |

## 3. devops-agent（内网堡垒机代理）

> **口径重定（用户令，2026-10-11，覆盖旧「CI/CD 只观测」口径）**：devops-agent 不是 CI 状态监控，而是**内网堡垒机式远程接入代理**——运维/开发在 croupier 后台直接进入内网机器操作（打包机进程挂了，后台直接进入处理，不再打开本地远程软件）。参考实现 cockpit（仓 `~/workspaces/cockpit`，已实现同类能力）。**尚未立项**：本节与 [agent-core 简档 §7](agent-core-design.md) 为简档（待批），批准后建 `cmd/devops-agent` 骨架。

| 维度                   | 内容                                                                                                                                                                         |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 职责一句话             | **内网堡垒机式远程接入代理**：SSH/RDP/VNC 进白名单内网主机，可操作；**更高权限 + 必须记录**（每次进入留痕：谁、何时、连哪台、做了什么）                                      |
| 部署位置（贴谁跑）     | **内网**（能直连白名单主机的 SSH/RDP/VNC 端口；对 croupier server 仍走上行注册，无需公网入站）                                                                               |
| 使用者                 | **运维 + 开发**（后台直接连接管理；权限门禁控制谁能连、连哪些主机）                                                                                                          |
| 监控/服务对象          | 白名单内网主机（打包机/构建机/运维节点）的 SSH（终端）、RDP/VNC（远程桌面）；服务探活（如 VNC 5900 端口 running）决定连接入口可用性                                          |
| 输入                   | 面板下发：连接请求（协议/目标主机/会话）、白名单主机与端口（configsync，game-scoped）、权限变更                                                                              |
| 输出                   | 会话流（终端/桌面画面回传面板）、会话审计（execlog 会话级）、探活状态上报、异常告警（herald 出口）                                                                           |
| **记录（堡垒机核心）** | **每次进入的会话都留痕**：谁、什么时间、连哪台、执行了什么——execlog 扩展到**会话级**（连接起止、命令流或会话录像的存档口径、可回放粒度）；**审计先行：无审计通道不得开连接** |
| 与其他 agent 边界      | **能操作但仅限白名单主机/端口**；**不碰游戏进程**（sidecar 专属）、**不读业务库变更日志**（capture 专属）                                                                    |
| 健康探针               | Readiness（白名单主机端口可达/guacd 通道可用）+ Heartbeat                                                                                                                    |
| 执行审计               | 会话开/关即落 execlog（audit-first：审计记录未落库不得放行连接）；会话内容按存档口径留档可回放                                                                               |
| scope 归属             | 注册携带 `game_id`+`env`；主机白名单/会话/审计全按 scope 过滤，跨环境不串                                                                                                    |

## 4. capture-agent（防私改数据库）

| 维度               | 内容                                                                                                                                                                  |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 职责一句话         | **防私改数据库 = CDC**（Change Data Capture，用户令 2026-10-11 明确「capture-agent 就是 cdc」）：读游戏库变更日志，变更发生即告警——谁、怎么改、改了什么、有无流水背书 |
| 部署位置（贴谁跑） | **游戏网络内、贴近游戏业务库**（与 sidecar-agent 同机或邻近）                                                                                                         |
| 监控对象           | 道具相关表的行级变更（insert/update/delete）                                                                                                                          |
| 输入               | binlog / WAL 逻辑复制 + 面板下发规则（白名单/阈值，game-scoped）                                                                                                      |
| 输出               | 告警事件（带规则命中详情）+ 位点/命中统计上报 + 执行审计（execlog）                                                                                                   |
| 与其他 agent 边界  | 只管数据变更合规：不做库指标健康（db-monitoring 姊妹件）、不碰函数通道（sidecar 专属）、不碰 CI/CD（devops 专属）；**只告警不处置**                                   |
| 健康探针           | Readiness（库连通/复制流健康）+ Heartbeat                                                                                                                             |
| 执行审计           | 位点操作/白名单配置变更落 execlog；**白名单等命令类操作必须 audit-first 放行**                                                                                        |
| scope 归属         | 注册携带 `game_id`+`env`；变更事件/告警/审计按 scope 过滤，跨环境不串                                                                                                 |

## 5. 公共模块（三 agent 共用，不各写各的）

### 5.1 healthprobe（健康探针，首批交付物）

三语义对标 k8s probe 术语（定稿见 agent-core §6.1）：**Liveness**（存活）/ **Readiness**（依赖就绪）/ **Heartbeat**（周期心跳）。三 agent 消费映射：

| agent         | Liveness     | Readiness                         | Heartbeat |
| ------------- | ------------ | --------------------------------- | --------- |
| sidecar-agent | 游戏进程存活 | server 上行链路可达               | 通用打点  |
| devops-agent  | 自身存活     | 白名单主机端口可达/guacd 通道可用 | 通用打点  |
| capture-agent | 自身存活     | 库连通/复制流健康                 | 通用打点  |

探针结果直接上面板：每页实例列表带状态灯 + liveness/readiness 结论 + 心跳时长（last heartbeat age）。

**按 system 角色建模（故障窗口，用户令 2026-10-09）**：探针不只看当下活死——每个受监控系统（target）记录故障窗口（开始/恢复/持续时长），历史可查，探活数据带 timeline；排查时直接回答「什么时间段服务不可用」。模型与留存见 agent-core §6.1，面板展示见 [Agent 面板设计](agent-dashboard-design.md) §2。

### 5.2 execlog（统一执行审计）

**每个 agent 的每次执行都留审计记录**（用户令，2026-10-09）。字段/audit-first 放行闸/保存期导出配置位见 agent-core §6.2。面板侧：各 agent 页「执行记录」列表 + 筛选（执行人/结果/时间段）+ 详情下钻（入参/结果全文），全按 scope 过滤。

### 5.3 herald（告警投递出口）

capture 告警 / healthprobe 故障窗口 / devops 异常（白名单主机不可达、审计拒绝等）经 **herald**（通知编排与投递平台）投递——croupier 当接入方（courier 式接法），**渠道/收件人由 herald 侧配置**，croupier 只报事件不配收件人；告警页仍是真值，herald 不可达不阻塞告警链。接法见 [herald 对接简档](agent-herald-integration.md)。**M2 已落地（2026-10-10）**：`internal/platform/outlet` 出口适配器 + `herald:` 配置段（缺省关），首事件源 supervisor 熔断，实现与简档差异见简档 §8。

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
- **每页四块**：实例列表+状态灯（探针结论/心跳时长/故障窗口 timeline）→ 职责实时数据（sidecar：进程与注册函数；devops：主机白名单与在线会话/探活状态；capture：变更事件与命中告警）→ 日志入口 → 「执行记录」（execlog，筛选执行人/结果/时间段；devops 页含会话级记录与回放入口）。
- **实例详情下钻**沿用现有详情页模式（Ops/Nodes 抽屉先例）。
- **与本页互链**：各 agent 页头部链接《Agent 清单》对应小节。

## 9. 已知边界与待批点

- sidecar-agent 主体已存在（`internal/agent` + `internal/app/agent`，**现实目录 `cmd/agent` 即 sidecar-agent，定名待落**）；改名方案（含存量 agent 升级兼容）**待用户确认**，确认前禁止直接 rename，见 todo.md。
- devops-agent **未立项**：简档按 2026-10-11 运维口径（内网堡垒机代理，可操作+全留痕）重写，**待批**；批后建 `cmd/devops-agent` 骨架（注册/心跳/execlog，与 capture C1 同深度）。会话录像存档口径/回放粒度、guacd 依赖与本地终端兜底取舍随简档报批。
- capture-agent 闸层 C2 待批（按 [capture-agent 简档](capture-agent-design.md) 推进）。
- 旧 outlet kind 闭集含 `devops.build_failed`（按旧口径实施）：devops 事件语义随简档定稿改判（改名或复用），简档批准前代码不动（零代码单）。
- execlog server 侧保存期默认值与导出格式随实施报批；agent 本地文件保留期有限（轮转），真值以 server 审计存储为准。
- 无标签注册「不进面板」的展示口径（彻底不可见 vs 列表置灰）随面板实施报批，默认彻底不可见。
