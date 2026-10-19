---
title: Agent 清单——四类 agent 定名/职责边界/公共模块/面板分页/scope 归属
---

# Agent 清单（Agent Inventory）

## 状态

- **最新需求覆盖（2026-10-17）**：用户明确补充第四类外部游戏与第三方服务探测 Agent，沿用 `synthetics-agent` 定名，从路线图占位提升为明确待实现需求；负责拉起并复用开发者实现的多语言探测程序，指定调研 HashiCorp go-plugin，详见 §6 与 [选型调研](../research/synthetics-agent-go-plugin.md)，下列“三驻留”记录为历史已实施范围，当前分类为四类、已存在代码入口仍为前三类。
- 状态: **以代码现实为准（2026-10-11 核）**：公共模块 `healthprobe`/`execlog` **已建成**（agent-core K1–K4 落地，见 [agent-core 简档](agent-core-design.md) §5.1），上行控制连接 `agents/controlconn`（core 系 agent 共用，2026-10-11 自 capture 包上收共享）；`capture-agent` **C2 已交付**（C1 骨架 2026-10-10：注册/心跳/事件流/位点；闸层三闸 reconcile/cep/lineage + 告警上报链 + 闸规则下行拉取 + 面板编辑面 2026-10-11，`cmd/capture-agent` + `agents/capture/`；下行链闭环）；`devops-agent` **骨架已实施**（2026-10-11 拍板批简档：`cmd/devops-agent` + `agents/devops/`——注册/心跳/execlog 审计 + 会话建立链 Broker/白名单/存档元数据，协议通道 guacd/SSH 与下行会话接线另批）；`sidecar-agent` = 现有主体已部分切 core，**源码目录 `cmd/sidecar-agent`（2026-10-11 改名方案 A 落地：目录定名，产物二进制名保持 `croupier-agent`，部署契约零影响）**。
- **当前定名（2026-10-17）**：四类 Agent = **`sidecar-agent`** / **`devops-agent`** / **`capture-agent`** / **`synthetics-agent`**，前三类有代码入口，第四类需求明确待实现；公共模块 = **`healthprobe`**（健康探针）、**`execlog`**（执行审计）；`connector-agent` 仍为路线图占位，旧候选（gameserver-agent / cicd-agent / cdc-agent / ci-agent）弃用。
- **角色口径重定（用户令，2026-10-11，覆盖 2026-10-09 旧口径）**：`sidecar-agent` = **外网侧、只做业务相关功能**（监控游戏进程、注册函数、汇报健康信息含主机与定时任务）；`devops-agent` = **内网堡垒机式远程接入代理**（可操作、更高权限、必须记录；旧「CI/CD 只观测」口径整节废弃）；`capture-agent` = **CDC**（防私改库）。
- 本页是**唯一权威定名与职责边界清单**；每个 agent 的实现文档开头有同口径「作用」节；与 [agent-core 能力清单表](agent-core-design.md)（§2）互链。
- 本页目录：§1 全景 → §2–§4 已有三类 Agent → §5 公共模块 → §6 synthetics 与其余路线图 → §7 scope 归属规则 → §8 四类后台分页（详设另页）。
- **六件套索引（顺序令，2026-10-09：文档先行，过目前不动码）**：①《Agent 清单》=本页；②能力清单+目录树=[agent-core 简档](agent-core-design.md) §2/§4；③healthprobe+execlog 设计=[agent-core 简档](agent-core-design.md) §6；④dash 页面清单=[Agent 面板设计](agent-dashboard-design.md)；⑤scope 规则=本页 §7（页面侧联动=[Agent 面板设计](agent-dashboard-design.md) §5）；⑥herald 对接=[herald 对接简档](/archive/legacy-en/design/agent-herald-integration)。

## 1. 全景

| 名称               | 层             | 状态                                                              | 一句话                                                 | 实现文档                                                    |
| ------------------ | -------------- | ----------------------------------------------------------------- | ------------------------------------------------------ | ----------------------------------------------------------- |
| `sidecar-agent`    | 驻留 agent     | 已有主体（源码目录 `cmd/sidecar-agent`，产物名 `croupier-agent`） | 游戏服务器旁（外网侧）的 GM 基础设施代理               | [agent-core 简档](agent-core-design.md) §1                  |
| `devops-agent`     | 驻留 agent     | 骨架已实施（2026-10-11；协议通道/下行接线另批）                   | 内网堡垒机式远程接入代理（SSH/RDP/VNC，可操作+全留痕） | [agent-core 简档](agent-core-design.md) §7                  |
| `capture-agent`    | 驻留 agent     | C2 已交付（C1 骨架 + 三闸 + 告警链 + 下行链闭环，2026-10-11）     | 防私改数据库：CDC 变更捕获与三道闸                     | [capture-agent 简档](capture-agent-design.md)               |
| `synthetics-agent` | 独立探测 agent | 需求明确，待实现（2026-10-17）                                    | 从游戏外部探测游戏及第三方服务，托管多语言探测插件     | [go-plugin 调研](../research/synthetics-agent-go-plugin.md) |
| `healthprobe`      | 公共模块       | **已建成**（接线待消费方）                                        | Liveness/Readiness/Heartbeat 三语义探针                | [agent-core 简档](agent-core-design.md) §6                  |
| `execlog`          | 公共模块       | **已建成**（接线待消费方）                                        | 统一执行审计（audit-first）                            | [agent-core 简档](agent-core-design.md) §6                  |
| `connector-agent`  | 路线图占位     | 只登记不实现                                                      | 第三方服务接入                                         | [agent-core 简档](agent-core-design.md) §8                  |

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

> **口径重定（用户令，2026-10-11，覆盖旧「CI/CD 只观测」口径）**：devops-agent 不是 CI 状态监控，而是**内网堡垒机式远程接入代理**——运维/开发在 croupier 后台直接进入内网机器操作（打包机进程挂了，后台直接进入处理，不再打开本地远程软件）。参考实现 cockpit（仓 `~/workspaces/cockpit`，已实现同类能力）。**骨架已实施（2026-10-11 拍板批简档）**：`cmd/devops-agent` + `agents/devops/`（注册/心跳/execlog 审计 + 会话建立链 Broker/白名单/存档元数据）；协议通道（guacd/SSH）与面板→agent 下行会话接线另批，见 [agent-core 简档 §7](agent-core-design.md)。

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

**探测体系三层与统一装配（2026-10-14 评审定稿，权威出处 [健康评审](/archive/legacy-en/design/agent-health-recovery-review)）**：内核=本包（一份实现）；协议探测器=http/tcp/process/db+游戏协议骨架；执行位置按视角（每 agent 均可配 `probe.targets`，devops 为主使用方——用户 2026-10-14 拍板）。**实现层 ≠ 实例层**：共用 core 代码 ≠ 单点发起。上行不对称口径（正常聚合/异常明细）与采样策略（server 下发、默认 30s）见同简档 §13。当前现状：capture 已探库连通（TCP 层），devops 零探针待 M1。

### 5.2 execlog（统一执行审计）

**每个 agent 的每次执行都留审计记录**（用户令，2026-10-09）。字段/audit-first 放行闸/保存期导出配置位见 agent-core §6.2。面板侧：各 agent 页「执行记录」列表 + 筛选（执行人/结果/时间段）+ 详情下钻（入参/结果全文），全按 scope 过滤。

### 5.3 herald（告警投递出口）

capture 告警 / healthprobe 故障窗口 / devops 异常（白名单主机不可达、审计拒绝等）经 **herald**（通知编排与投递平台）投递——croupier 当接入方（courier 式接法），**渠道/收件人由 herald 侧配置**，croupier 只报事件不配收件人；告警页仍是真值，herald 不可达不阻塞告警链。接法见 [herald 对接简档](/archive/legacy-en/design/agent-herald-integration)。**M2 已落地（2026-10-10）**：`internal/platform/outlet` 出口适配器 + `herald:` 配置段（缺省关），首事件源 supervisor 熔断，实现与简档差异见简档 §8。

## 6. 外部探测 Agent 与其余路线图

### 6.1 synthetics-agent（第四类，需求明确，待实现）

| 维度       | 职责与边界                                                                                                                                                               |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 部署位置   | 独立于游戏进程，在代表玩家或第三方服务消费方的网络位置运行，可按网段/地域部署多个探测点                                                                                  |
| 游戏探测   | 从外部进行握手、登录及按游戏定义的业务可用性检查，端口可达与协议/业务验证分别标注                                                                                        |
| 第三方探活 | 主动检查 HTTP/TCP/数据库及其他服务，复杂协议交给开发者插件；外部系统事件汇入仍属于 connector 路线                                                                        |
| 目标类别   | 显式字段 `targetCategory=game/third_party` 区分游戏与第三方服务，贯穿配置、调用、结果、告警及页面筛选/统计；与协议、插件能力和 Agent 类型分别建模，两类均归属 gameId/env |
| 插件宿主   | 拉起并复用任意兼容语言实现的常驻程序，负责握手、版本、超时取消、并发限制、崩溃恢复与关闭，HashiCorp go-plugin 为用户指定调研路线                                         |
| 输入输出   | 平台下发目标、能力、频率、截止时间与凭证引用；回传状态、阶段、时延、错误、探测点与故障窗口，按 game/env 隔离                                                             |
| 公共底座   | 复用 TCP/TLS session、注册心跳、配置与 core/healthprobe 内核；本地插件 IPC 单独分层，生命周期只有一个管理者                                                              |
| 权限边界   | 只探测并上报，不获得游戏进程管理或堡垒机权限；插件崩溃与目标不可用分别记录                                                                                               |
| 当前状态   | 尚无独立入口与外部插件调用运行时；现有 kind=probe-adapter 元数据和内置协议探测不视为该能力已完成                                                                         |

2026-10-17 最新需求覆盖此前“不新增 agent 类型”的部署限制；既有 Agent 仍可复用公共探测能力，独立外部探测载体单列为第四类，代码复用不等于部署实例合并；用户进一步澄清无 gRPC 约束针对游戏 SDK 依赖负担，本地 go-plugin gRPC 允许用于独立探测宿主及插件，游戏 SDK 依赖闭包必须保持轻量，更新后的 [传输决策](/archive/legacy-en/architecture/transport-no-grpc) 与 [选型调研](../research/synthetics-agent-go-plugin.md) 为当前口径。

### 6.2 connector-agent（仍为路线图占位）

负责第三方系统状态/事件接入汇入，尚未实现；与主动发起请求的第三方服务探活分别建模；占位不建面板页，触发条件见 agent-core §8。

## 7. scope 归属规则（game_id + env，强制）

权威契约见 [游戏环境作用域 §14.7](../architecture/game-environment-scope.md#_14-7-四类-agent-的显式归属与后台页面-2026-10-17)，页面规则见 [四类后台设计](agent-dashboard-design.md) §5。

1. **显式身份**：四类配置及注册均需非空 `gameId/env`，proto/DB 使用 `game_id/env`；它们是固定身份字段，不能只写用户 labels 或从主机名推导，缺失启动/注册失败。
2. **单实例单归属**：每 Agent 只绑定一个游戏环境，跨 scope 同机部署多个独立实例及目录；插件、目标、主机、采集源、任务、配置、统计、审计和日志继承同 scope，禁止跨 scope 指派。
3. **明确标注**：四页标题、实例行、详情与操作确认始终显示 gameId/env，复用全局选择器；切换清理旧请求、缓存、抽屉与待提交操作，迟到响应不能串页。
4. **服务端隔离**：列表/聚合/详情/控制/下载/回放以及本地/集群/回退路径统一校验 ResolvedScope，遵守 scope 请求只取 context/header 的约定。
5. **离线历史**：按持久化最后已知 scope 展示并标数据时间与过期，禁止实时控制；缺归属的旧记录不进四页，保留受权诊断日志，不跨 scope 兜底。

## 8. 面板分页（每类 agent 单独一页）

**详设见 [Agent 面板设计](agent-dashboard-design.md)**（导航/页面构成/下钻/scope 联动/server API），要点：

- **侧栏四页**：游戏代理 `/ops/agents/sidecar`、运维代理 `/ops/agents/devops`、数据采集代理 `/ops/agents/capture`、外部探测代理 `/ops/agents/synthetics`；均为目标路由，随真实数据面交付，不把现有 Nodes/Capture 页当作四页已实现。
- **公共骨架与职责区**：身份/scope、连接与业务状态、实例列表、详情 Tabs、执行记录及日志；sidecar 展示进程/函数，devops 展示主机/真实会话，capture 展示采集位点/策略/命中告警，synthetics 展示探测点/目标/插件/故障窗口；原始 CDC 明细与 devops 内容回放须先具备真实数据面。
- **实例详情下钻**沿用现有详情页模式（Ops/Nodes 抽屉先例）。
- **与本页互链**：各 agent 页头部链接《Agent 清单》对应小节。
- **实现基线与验收**：当前缺口、公共 DTO/API、各页职责 Tabs、权限、空态/离线态、scope 切换与五批交付见 [后台页面设计](agent-dashboard-design.md)，四页均须完成真实后端链路后验收。

## 9. 已知边界与待批点

- sidecar-agent 主体已存在（`internal/agent` + `internal/app/agent`，**源码目录 `cmd/sidecar-agent` 即 sidecar-agent**）；2026-10-11 用户拍板改名方案 A 落地——仅源码目录改名，产物名 `croupier-agent` 保持，部署面（镜像 ENTRYPOINT/systemd/安装脚本/nightly 资产）零影响。
- devops-agent **骨架已实施（2026-10-11 拍板批简档）**：`cmd/devops-agent` + `agents/devops/`（config/session/runner）——注册/心跳/execlog audit-first + 会话建立链（白名单前置→审计落库→Open，顺序不可倒）+ 会话存档元数据 JSONL。**剩余边界**：协议通道（guacd/SSH 终端，noop Provider 占位）、面板→agent 下行会话接线、白名单 configsync 下发、会话录像存档格式/保留期/回放粒度——均另批报批。
- capture-agent 闸层 C2 已交付（reconcile/cep/lineage 三闸 + 告警上报链 + 规则下行拉取 + 面板编辑面，2026-10-11，下行链闭环；见 [capture-agent 简档](capture-agent-design.md) §8）。后续平台化演进（分层过滤策略/多数据库 Connector/风控一级分类）已立项待实施，见 [CDC 过滤策略体系](cdc-filter-policy-design.md)、[CDC Connector 架构](/archive/legacy-en/design/cdc-connector-architecture)、[风控面板设计](/archive/legacy-en/design/risk-control-panel-design)。
- 旧 outlet kind 闭集含 `devops.build_failed`（按旧口径实施）：devops 事件语义随简档定稿改判（改名或复用），简档批准前代码不动（零代码单）。
- execlog server 侧保存期默认值与导出格式随实施报批；agent 本地文件保留期有限（轮转），真值以 server 审计存储为准。
- 无标签注册「不进面板」的展示口径（彻底不可见 vs 列表置灰）随面板实施报批，默认彻底不可见。
