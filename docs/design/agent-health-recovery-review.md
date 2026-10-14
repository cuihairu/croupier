---
title: Agent 健康监控与恢复——外部方案重合度审计与 server 下放迁移评审（评审定稿）
---

# Agent 健康监控与恢复评审（定稿）

## 状态

- 状态: **评审定稿（未动实现码；R2 起为代码批）**。来源: 用户 2026-10-14 五条——外部设计建议做可嵌入 Health Monitoring & Recovery 模块；本仓早有监控外部第三方服务存活的需求（server 侧实现），要拆到 agent 统一为基础能力；补充问题（第三方监控是否需要新增 agent 部署）；硬约束（探测实现收口 core/healthprobe 一份 + 实现层/实例层分标 + 熔断器归属判定）；三点拍板（归属/通道/动表时机）；外部评审文档核验（docs/review/chatgpt-2026-10-14-probe-architecture.md）。
- 评审基准: `core/healthprobe`（三 agent 共用探针包）+ sidecar supervisor（进程恢复环）+ server 侧 dbmon / serverstatus / dispatch 熔断器。**外部设计的 Checker/RecoveryHandler/Target/ProbeTarget 命名一律不引入**，重合概念按本仓既有命名与语义归位。

## 1. 重合度审计（外部设计 → 本仓现状，逐条）

| 外部设计概念                                     | 本仓现状                                                                                                                                                                                                                                | 结论                                                      |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `Checker` 接口                                   | `core/healthprobe/healthprobe.go:32-34`（`Check(ctx) error` 单值契约）+ `CheckerFunc`（:37-40）                                                                                                                                         | **已有，且更严格**：单值返回天然禁止 Checker 携带副作用面 |
| 三语义探针（状态机）                             | `healthprobe.go:21-29`（`Semantics` liveness/readiness/heartbeat 闭集）；两态机在 `probe.go:51-71`                                                                                                                                      | **已有**：`Probe` 是唯一状态机持有者                      |
| 防抖（debounce）                                 | `probe.go:76-87` `WithThresholds`（默认 fail=3/recover=1，对标 k8s failureThreshold/successThreshold）；推进逻辑 `probe_run.go:11-71`（连续失败达 failThr 开窗、连续成功达 okThr 闭窗，任一成功清零失败连击）                           | **已有**：阈值+连击清零即防抖，无缺口                     |
| 失败阈值与故障窗口                               | `probe.go:20-29`（`Window`：StartTS/EndTS=0 未恢复/持续时长/LastReason/scope）；历史环 `probe.go:99-109` + `probe_run.go:118-123`（默认 100 截断）                                                                                      | **已有**                                                  |
| 窗口事件出口                                     | `probe.go:33-39`（`WindowEvent`/`WindowListener`，注释预留 herald 出口 `probe.unavailable/probe.recovered`）；`OnWindow` `probe.go:95-97`                                                                                               | **已有**                                                  |
| 平台进程适配层                                   | `process_unix.go:13-30`（信号 0 探活）+ `process_windows.go:9-27`（FindProcess 退化，准确性核查见 §5.3）                                                                                                                                | **已有**（Windows 底层语义退化，见 §5.3）                 |
| 内置探测方式                                     | HTTP `healthprobe.go:43-71`（2xx/3xx 通过）、TCP `healthprobe.go:74-98`（拨号即关）                                                                                                                                                     | **已有**（icmp/dns 无——见缺口）                           |
| 心跳语义                                         | `heartbeat.go:14-53` `HeartbeatFile`（mtime 即最近活体时刻）+ `core/supervisable/supervisable.go:75-97`（beatLoop）                                                                                                                     | **已有**                                                  |
| 窗口持久化                                       | `timeline.go:13-88`（`FileTimeline` JSON-lines 真值通道 + `LoadTimeline` 读回）                                                                                                                                                         | **已有**                                                  |
| `RecoveryHandler`（恢复动作钩子）                | **进程域已有**：sidecar supervisor 恢复环 `internal/app/agent/ops_server.go:451-590`（detect_down → 熔断检查 → 指数退避 → auto_restart → restart_failed 回环）；**外部目标域没有**（agent 对外部第三方服务无恢复动作，也不应有——见 §8） | 进程域已有，外部目标域明确**不做**自动恢复                |
| 恢复闭环（动作后再探、判定恢复成功 vs 持续失败） | `ops_server.go:573-615`：`armSuccessTimer` 新实例存活超过退避封顶才清连续失败计数（防启动即崩刷穿退避序列，设计 §3.4）                                                                                                                  | **已有**（进程域）                                        |
| 恢复权限边界                                     | `ops_server.go:443-448` `autoRestartAllowed`：进程级 AutoRestart 开关 AND ops 双闸（Enabled+AllowRestart）                                                                                                                              | **已有**（进程域）                                        |
| 并发恢复防护                                     | 每进程单监视 goroutine + 所有权守卫（`p.cmd != watched` 每次状态迁移前重查，`ops_server.go:463-476/486-491`）；退避 sleep 在锁外可被停机信号打断                                                                                        | **已有**（进程域；全局并发上限无——见缺口）                |
| 熔断器                                           | `internal/platform/dispatch/agent_health_tracker.go:13-35`（Closed/Open/HalfOpen）+ 状态迁移 `:232-304` + 打分/衰减 `:38-105`                                                                                                           | **已有但非探测**：请求结果驱动的路由熔断——归属裁决见 §7   |
| Monitor Registry（目标注册）                     | 无动态注册面：capture 探针目标是 config 推导的单源（`agents/capture/runner/runner.go:633-657`）；devops 无探针                                                                                                                          | **真缺口**（见 §2）                                       |
| 自动重启外部服务/回调类恢复                      | 无                                                                                                                                                                                                                                      | **缺口按边界处理**：外部目标只告警不自动恢复（§8）        |

## 2. 真实缺口（全部）

1. **探针目标注册面**：没有「一个 agent 声明 N 个外部目标（http/tcp/db）各自探」的配置与装配。现状每 agent 至多一个硬编码目标（capture 的数据源，`runner.go:644-657`；devops 零探针——healthprobe 包注释里「devops 探 CI 端点」是规划用法未落地）。
2. **窗口事件上行通道**：`OnWindow` 目前只打本地日志（capture：`runner.go:648-653` warn 日志）；`core/report` 的 `Conn` 最小面只有 `SendTaskEvent/SendMetricEvent`（`report.go:17-23`）。（已拍板 2026-10-14：扩 `SendAlertEvent` 新 wire 消息，见 §11。）
3. **server 侧窗口消费**：收到 unavailable/recovered 后进 Alert 生命周期的链路不存在。dbmon 已有同语义先例可复用：`internal/api/dbmon/service.go:156-203`（firing/resolved 状态机 + alertID 去重）。
4. **DB 类 Checker**：dbmon 的探针本体（`internal/api/dbmon/probe.go`，只读 SQL + 5s 硬超时 + 云 RDS 不暴露指标报 degraded 不猜）是 server 侧第二套实现；收敛方案见 §6。
5. **icmp/dns Checker**：无（blackbox_exporter 的分类里仅这两类缺；可用 stdlib 补，零依赖）。
6. **全局并发恢复上限**：进程域是每 target 串行（单监视 goroutine），无跨 target 并发上限。**建议不做**——恢复动作只存在于进程域，agent 间无共享恢复池需求；真出现再加。
7. **恢复动作注册**（重启服务/回调/告警三分支）：进程域=重启（已有）；外部目标域=只告警；「回调」类动作无需求，不预留死接口（对齐 `report.go` 注释「v1 不预置死接口」口径）。

## 3. server → agent 迁移方案

### 3.1 先归位：三类「健康」在 server 侧的真实身份

| server 侧件                                                                                                                             | 身份                                                                                                                                                                                                       | 迁移动向                                                                                                                                                                                     |
| --------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `dispatch.HealthTracker` + 熔断器（`internal/platform/dispatch/agent_health_tracker.go`，接线于 `internal/svc/service_context.go:525`） | **路由面**：server 挑哪个 agent 转发函数调用的打分/熔断，由请求成败驱动                                                                                                                                    | **不下放、不改**（归属裁决 §7）。`internal/config/config.go:677-687` 的 `healthCheck` 配置节专属此面，**不改名不挪位**，与 agent 探针配置（`probe` 节，见 §9）是两个口径，边界写死           |
| `dbmon`（`internal/api/dbmon/`）                                                                                                        | **外部第三方服务存活监控**——用户说的「早就有、在 server 侧」的主体：DBSource 注册表 + 只读 SQL 探针 + Alert 生命周期。当前探针是**按需**（`handler.go:70-71` POST /dbmon/probe，`routes.go:1421`），无周期 | 探测执行**下放 agent**（网络位置真值：agent 贴着游戏库，server 隔着网关）；探测**实现**收敛进 core/healthprobe（§6，硬约束：不留第二套）；server 保留 sources 注册表、告警聚合、手动诊断探针 |
| `serverstatus`（`internal/platform/serverstatus/`，provider 闭集 noop/atlas/http `registry.go:10-12`）                                  | **维护状态源**（显示面 + 告警 gate 裁决 fail-open，`gate.go:14-31`），不是存活探针                                                                                                                         | **不动**。它是状态查询不是周期探测，职责不同                                                                                                                                                 |

### 3.2 迁移顺序（每步可回退）

- **M1 加法：agent 探针装配面**——per-agent 配置 `probe.targets[]`（type: http/tcp/process/db）+ Checker 工厂 + 多 `Probe` 装配 + `OnWindow` 汇聚。capture 现有单源探针改为走同一装配（行为等价）。可回退：删配置即回退，无删除性改动。
- **M2 上行**：`WindowEvent` → server（通道已拍板：`report.Conn` 扩 `SendAlertEvent` 新 wire 消息，§11）。server 收口：Alert 生命周期复用 dbmon firing/resolved 语义（alertID=`probe:<agentID>:<target>`，视角标注防跨视角去重塌缩，见 §4.1）；本地 `FileTimeline` 保持真值通道不动。可回退：server 不消费时 agent 照跑（本地窗口+日志语义与今天等价）。
- **M3 dbmon 下放收尾**：DB 源周期探针按归属 agent 下发（归属列已拍板加、R4 动表，编号迁移三处同步）；server `POST /dbmon/probe` 保留为手动诊断。可回退：agent 探针停用后手动诊断面完整如初。
- **M4 面板**：探针卡/故障窗口 timeline 展示——**与 Agent 面板三页（agent-dashboard-design §2 探针卡）合流交付**，不另开页面。

### 3.3 迁移后 server 侧还剩什么

dispatch 熔断与打分（路由面）、dbmon sources 注册表 + 手动诊断探针（经同一内核装配，§4.3）、Alert 聚合与生命周期（`model.Alert`）、serverstatus 状态源、面板展示。server 不再执行任何周期性外部存活探测。

## 4. 补充裁决：部署形态与单一分工口径（2026-10-14）

### 4.1 第三方监控不需要为监控新增 agent 部署

对用户五条初判的核验（两条需修正精确度，结论不变）：

1. **属实（含精确化）**：Readiness 语义注释（`healthprobe.go:3-4`）确以外部依赖为对象；但「devops 探 CI 端点」是注释里的规划用法，devops runner 当前零探针；已落地的是 capture 探库连通（`runner.go:644-657`，TCP 层）。
2. **属实（含边界）**：三类 agent 各就其位，下放=复用既有部署。边界如实写明：**视角覆盖=agent 覆盖面**——没有 agent 的网段就没有该网段视角。若确需某网段的纯探测存在，devops agent 已有 sessions 关闭的 registration-only 模式（`agents/devops/runner/runner.go:69`）可作最小载体——但那是业务纳管决策，不是监控功能的部署前提。
3. **属实**：Liveness 同机限制是机制性的（`process_unix.go:13-30` 信号 0 / Windows FindProcess 均同机语义），且属 sidecar 既有职责。
4. **属实且必须落实**：探测点决定视角。解法：视角随事件标注——上行信封天然携带 agentID（注册连接身份），server 侧 alertID 采用 `probe:<agentID>:<target>`，同一第三方多视角并存是特性不是矛盾（server 手动诊断视角与 agent 周期视角各自成立）。本地 `FileTimeline` 的视角=写文件的本机（隐含）。
5. **属实（一处精确化）**：dbmon 当前是**按需**探测（`routes.go:1421` POST /dbmon/probe，无周期调度）——迁移不是撤走既有周期监控（今天本来就没有周期面），而是把周期能力建在视角真值处；SQL 层探测由 §6 收敛后 agent 获得。

**结论：不需要为监控第三方而新增 agent 部署。**发起点=已存在消费方位置上的既有 agent；新增部署只发生在「新网段需要新视角」时，属业务纳管决策。

### 4.2 单一权威分工表（合并外部评审 7 行表与本仓口径）

判断标准：**谁的视角有意义就由谁探**（探测结果要回答什么问题为第一标准，网络可达性决定执行位置）。本表为唯一权威口径，外部评审文档的分工表不再单独引用。

| 探测对象                              | 执行位置（发起点）                             | 依据                                           | 本仓现状锚点                                 |
| ------------------------------------- | ---------------------------------------------- | ---------------------------------------------- | -------------------------------------------- |
| 游戏进程存活（Liveness）              | 游戏所在节点 sidecar                           | 同机才能信号 0 探活                            | `process_unix.go:13-30`；supervisor 既有     |
| 游戏端口 / 内网服务可达               | 能访问该内网的 agent（sidecar/capture/devops） | server 可能进不了游戏内网                      | M1 `probe.targets`                           |
| 数据库源（游戏库）                    | capture（贴库）或所在网段 agent                | 消费方网络视角才是运维语义                     | `runner.go:644-657` TCP 层已有；SQL 层 §6/R4 |
| server 自身依赖（server 库/内部 API） | server                                         | server 是消费方，server 视角即真值             | dbmon 按需诊断保留（经同一内核装配）         |
| 只有 server 能访问的第三方平台 API    | server                                         | agent 无法代表 server 的访问路径               | R3 后 server 装配 `probe.targets`            |
| 只有游戏内网能访问的第三方服务        | 该内网 agent                                   | server 的结论不具代表性                        | M1                                           |
| 模拟真实客户端登录/协议交互（玩家侧） | 代表性网络位置的探测实例                       | 必须验协议链路非端口；玩家视角必选（用户立场） | **无存量实现**，存量延后需求补落档，见 §5.4  |

### 4.3 实现层与实例层分标（代码一份 ≠ 实例一份）

- **实现层（一份）**：`core/healthprobe`——Checker 契约、状态机、窗口、时间线、Checker 工厂（http/tcp/process/db 全部进 core，db 见 §6）。三 agent 与 server 共同 import（server 由此**首次**引入 core/healthprobe import——core 零内部依赖，无环）。
- **实例/发起点层（多处）**：agent 装配（M1 `probe.targets`）与 server 装配（手动诊断、server 自身依赖）各自实例化，按 §4.2 分工。**共用一处实现 ≠ 只由一处发起 ≠ 探测相同目标。**
- **视角标注方式**：窗口/事件携带 target+语义+时间（`Window` 既有字段）；视角（agentID）由上行信封携带；server 自发探测的视角标注=server。

## 5. 外部评审核验结论（三层结构与五条约束）

### 5.1 三层结构 vs 现状

| 层                    | 现状                                                                                                                                                         | 判定     |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------- |
| 层 1 探测内核         | `core/healthprobe`：接口/调度/超时/阈值/状态机/事件全有（`probe.go:51-71`、`probe_run.go:74-89`、`WithThresholds`）                                          | **已有** |
| 层 2 协议探测器       | http/tcp/process 已有（`healthprobe.go:43-98`、`process_*.go`）；**db 在 server 侧第二套实现**（dbmon，收敛方案 §6）；**游戏协议适配器缺**（独立演进，§5.4） | 部分已有 |
| 层 3 执行位置（装配） | per-agent/server 装配面未建（M1/R3）；视角分界已定（§4.2）                                                                                                   | **新层** |

dispatch 熔断器**不属于任何一层**（调用方策略，§7）。

### 5.2 五条设计约束逐条判定

1. **共用引擎不共用执行位置**——采纳，§4.3 即此义；R2 起每目标声明期望视角与执行位置（配置字段）。
2. **探测与恢复分离（外部服务探测失败不默认触发本地进程重启）**——**结构隔离已存在**：supervisor 恢复环唯一触发源=被管进程退出事件（`ops_server.go:426-439` monitorProcess `cmd.Wait()` → superviseExit），探针窗口不接入重启决策（capture readiness `OnWindow` 只打日志，`runner.go:648-653`）；恢复另有权限双闸（`ops_server.go:443-448`）。写进 R2 验收项：OnWindow 消费者禁止触发进程重启。
3. **配置与执行分离**——部分已有：agent config 本地 + `core/configsync` 版本化下发通道（`configsync.go:1-10`）；server 管配置/版本/授权=R3/R4 下发面；上报带目标标识/执行位置/时间/探测类型——`WindowEvent` 已含 Target/Semantics/时间戳，视角补 agentID（§4.1）。
4. **游戏模拟探测独立演进**——采纳，且与 core 零外部依赖约束同向：游戏协议适配独立包、实现 Checker 契约、不进 core；排期 §5.4。
5. **避免重复探测与重复告警**——采纳：多视角并存但各自标注（§4.1）；server 汇总不把单 agent 失败判全局——alertID 含 agentID 天然不跨视角塌缩，面板聚合层按 target 聚合视角集合。

### 5.3 Windows 进程探测准确性（外部点名核查）

核验 `process_windows.go:9-27`：**「只统一接口、底层判定仍不准」部分成立**——接口统一（Checker），但 Windows `os.FindProcess` 恒成功（仅返回句柄包装），PID 复用窗口内会**误报存活**（误报方向是漏警不是误杀；`:10-13` 注释自认）。修法=`x/sys/windows` OpenProcess，但那是外部模块，与 core 零外部依赖冲突。**裁决：保持退化实现+诚实注释**（当前无 Windows agent 部署面证据），列已知边界；Windows 部署需求出现时按「core 唯一例外依赖」单独拍板（§11 开放项）。

### 5.4 游戏客户端模拟探测（存量延后需求，本次补落档）

- **来历**（用户 2026-10-14 澄清）：设计曾存在、当时拍板**延后实现**（复杂度+客户端分散）、**从未落入文档**——非全新需求，是「已决策延后、未落文档的存量需求」，本次补落档。
- **立场（权威口径）**：要做就站在**玩家客户端视角**探测；服务端互探已否决（「意义就小了」），不做「先用服务端互探凑合」的中间方案。
- **可行性核验**：本仓**无**可复用的游戏协议客户端/协议栈——`sdks/` 六语言 SDK 是「游戏服务器→agent」的注册/调用面（`sdks/SDK_FEATURE_MATRIX.md`），`examples/cmd/e2e-agent-probe` 是 croupier 自有 wire 的 mock agent（`main.go:1-17`），均非玩家协议。
- **工程量**：框架层小（复用 Checker/Probe/Window 生命周期，一个 R 批量级）；协议层=**每游戏一份适配器**（游戏私有协议无法通用化，成本随游戏数线性）；部署层=玩家代表性网络选点（当初延后根因「客户端分散在各处」）。
- **排期**：**P2**，按需触发（某游戏提出玩家侧拨测需求时立项；触发即玩家视角）。已登记 todo.md。

## 6. dbmon → healthprobe 收敛方案（硬约束：探测实现只留一份）

原则：探测实现收口 `core/healthprobe` 一份；dbmon 保留的是「注册表 + 诊断 API + 阈值告警」三个非探测职责，不是第二套探测。

| dbmon 现件                                           | 去向                                                                                                                                                                          |
| ---------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SQL 连接 + 查询电池（`probe.go:78+`，5s 硬超时只读） | → core/healthprobe `DBChecker`：`database/sql` 是 **stdlib，core 零外部依赖不破**；driver 由装配侧 blank import 注册（server 已注册，agent 侧照做）                           |
| `ProbeResult.OK/Error`                               | → Checker 结果语义（error 即不可用，携带原因串）                                                                                                                              |
| `ProbeResult.LatencyMS`                              | → **搬进 `Result`**（普适字段，R2 顺手加）                                                                                                                                    |
| degraded 原因串（云 RDS 不暴露≠猜）                  | → `Result` 新增可选 `Details map[string]string`（诚实语义随 Checker 走）                                                                                                      |
| LockWaits/Deadlocks/QueryCount 等指标富化            | **非探测语义**（阈值告警=gate 语义）：留 dbmon 诊断端点，但数据来源改为复用 DBChecker 同一次执行的产出（电池结果经 Details/诊断结构返回），**不再独立连库**——一份实现两处消费 |
| firing/resolved 告警生命周期（`service.go:156-203`） | 留 server（R3 与 probe 窗口共用同一 Alert 生命周期）                                                                                                                          |

落点：R2 建 DBChecker 进 core；R4 dbmon 切换消费 DBChecker、删自有 SQL 电池（届时「第二套探测实现」清零）。

## 7. 熔断器归属裁决（探测 vs 调用方策略）

**结论：同意用户倾向——熔断是调用方策略，留 server dispatch，不跟探测走。**

依据：

1. **输入源不同**：`RecordSuccess/RecordFailure` 全部调用点在 dispatch 调用路径（`internal/platform/dispatch/dispatcher.go:433/441/451/561/563/701/712/1519`），消费的是**函数调用成败**，不是任何探测结果。
2. **消费方不同**：其存在目的是路由选择（`IsAvailable`，消费于 `load_balancer.go:37`）；agent 不做路由决策，熔断搬去 agent 无消费方。
3. **语义错位风险**：若熔断跟着探测走，会出现「agent 上报某第三方目标不可达 → server 熔断该 agent」的错配——目标是第三方，不是 agent 本身。探测事实（窗口）与调用熔断（策略）解耦后各有真值：窗口=目标可达性事实；熔断=路由面历史成功率策略。
4. 唯一合理的远期关联：agent 心跳/会话窗口作为路由可用性的**弱参考输入**——留观察，不接线。

## 8. 「不让 Checker 自己决定恢复」与并发风险

- **结构性保证已存在**：`Checker` 契约单值无副作用（`healthprobe.go:32-34`）；`Probe` 是唯一状态机；恢复动作进程域在 supervisor 退出监视环（死亡事件驱动，不是 probe 窗口驱动）——探测环与恢复环天然分离，两个环之间唯一的既有耦合点是 `supervisable` 的退出码语义（0=不拉起/非 0=进退避），不是 Checker。
- **现状并发风险评估**：capture `OnWindow` 只打日志（`runner.go:648-653`）；devops 无探针；sidecar supervisor 每进程单监视 goroutine + 所有权守卫（`ops_server.go:463-476`）——**今天不存在并发恢复风险**。风险只在未来把恢复 hook 挂上 `OnWindow` 时出现。
- **结论**：外部目标域**不引入自动恢复**。agent 无权重启第三方服务；「恢复」= 告警上行 + 人工（devops agent 的堡垒机会话就是人工恢复的执行面）。因此并发恢复上限、恢复动作注册在 M1-M4 内**不实现**；若未来进程域之外出现真恢复需求，沿用 supervisor 先例（target 级单 worker + 世代守卫 + 权限双闸）扩一层即可，接口不必预埋。

## 9. 持久化 / 观测 / 配置统一

- **目标注册持久化**：agent 侧 config 文件（与 capture `options`、devops `allowlist` 同构）；中心下发走 `core/configsync` 版本化轮询通道（`configsync.go:1-10`，包注释明示「capture 白名单/阈值、devops 规则配置复用同一通道」）——不新增表。DBSource 归属列已拍板（R4 动表，编号迁移三处同步）。
- **观测**：窗口本地真值=既有 `FileTimeline`；上行后 server 侧落 `model.Alert`（firing/resolved 语义复用 dbmon 先例）；进程域已有 supervisor 事件面先例（事件环 500 + lumberjack 旋转文件，`ops_supervisor_events.go:35-48`）。恢复/故障事件不新造 metrics 面。
- **配置统一**：`internal/config/config.go:677-687` `healthCheck`（score/circuit 参数）**专属 dispatch 路由面，保持原名原位**；agent 探针配置用独立 `probe` 节（lowerCamelCase，入 agent config 契约）。禁止合并两套——合并会让路由打分参数误入探针语义。

## 10. 外部依赖判定（逐个）

| 外部点名                  | 判定                 | 依据                                                                                                                                                                                                                                                  |
| ------------------------- | -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| go-subprocess             | **不引**             | 本仓 `os/exec` + supervisor 恢复环（退避/熔断/成功判定/所有权守卫）经 S1/S2/S3 实战修复，且与 `supervisable` 信号纪律/退出码契约耦合；换库等于推倒该契约                                                                                              |
| kahi                      | **不引**             | 未检索到主流 Go 健康库以此命名；所描述能力与 `core/healthprobe` 重合                                                                                                                                                                                  |
| health-go                 | **不引**             | 该类库是「暴露 /healthz HTTP 端点」框架——方向相反：我们探别人，不是被探；core 系 agent 出站-only 无 HTTP listener                                                                                                                                     |
| blackbox_exporter         | **不引，只借鉴思路** | 独立服务非 Go 库，且 AGPL-3.0；引入等于搭一整套 Prometheus 采集面，违背「agent 基础能力统一」。其探针分类 http/tcp/icmp/dns 中 http/tcp 已有（`healthprobe.go:43-98`），icmp/dns 需要时以 stdlib（`net.Resolver`、unix raw socket build tag）补进本包 |
| 「core 包零外部依赖」约束 | **守住**             | DBChecker 用 `database/sql`（stdlib）即可承载，driver 由装配侧 blank import 注册——db 探测器进 core 不破零依赖；游戏协议适配独立包；`x/sys/windows` 是唯一潜在例外（§5.3，届时单独拍板）                                                               |

## 11. 决策记录与开放项

**已决（用户 2026-10-14 拍板）**：

1. **probe.targets 归属**：每 agent 均可配，devops 为主使用方。依据：能力同源——「代码一份」的必然结果（若只允许 devops 承载，共用实现就被绑死到单一 agent 类型，视角分工也不成立）。
2. **上行通道**：扩 `report.Conn` 的 `SendAlertEvent`（新 wire 消息），不复用 `MetricsReport` 扩展位。依据：`report.go` 注释已预留「告警通道落位时在本包扩展 SendAlert」；告警要投递可靠性与独立演进，指标是周期批量，复用扩展位会把告警塞进指标的 QoS。
3. **DBSource 归属列**：要加，**排 R4 才动表**（M3 下放必须能把 DB 源指到具体 agent，归属跑不掉；但 R2/R3 不依赖它，先做会白等一次编号迁移）。R2/R3 按不依赖归属列推进，**不因这条停**。

**开放项（风险，非拍板项）**：

4. 窗口事件风暴：target 多 + 抖动窗口时上行量——supervisor 事件环 500 + 去重先例可循；M2 设计时按 target 配 dedup 窗口。
5. 双真值时钟漂移：窗口 StartTS 来自 agent 本地钟，server 记接收时间——告警详情双列并存，不做时钟对齐。
6. core 引入 `x/sys/windows`（唯一例外依赖）：仅 Windows agent 部署需求出现时拍板（§5.3）。

## 12. 分期

- **R1 评审定稿：已完成**（含 2026-10-14 补充裁决、硬约束收敛方案、熔断归属、三点拍板、外部评审三层五约束并入）。
- **R2 agent 装配面**（M1+M2 agent 侧）：`probe.targets` 配置契约（含每目标视角声明）+ Checker 工厂（http/tcp/process/**db 全进 core**）+ 多 Probe 装配 + `SendAlertEvent` 上行接线 + capture 迁移到统一装配 + `Result` 增 LatencyMS/Details。代码批，全套门禁（含新用例）；验收含「OnWindow 消费者禁止触发进程重启」。
- **R3 server 消费面**（M2 server 侧 + M4）：窗口→Alert 生命周期（alertID=`probe:<agentID>:<target>`）+ server 侧内核装配（手动诊断/自身依赖）+ 探针卡面板（与 Agent 面板三页合流）。
- **R4 dbmon 收敛收尾**（M3）：DBSource 归属列（已拍板动表，编号迁移三处同步）+ 周期探针按归属下发 + dbmon 切 DBChecker **删自有 SQL 电池** + server 手动诊断保留。
- **不排期**：外部目标自动恢复、恢复动作注册、全局并发恢复上限（§8 边界）；游戏客户端模拟探测单独 P2（§5.4）。

## 已知边界

- 本评审只做审计与方案，未动任何实现码；§2 缺口 5/6/7 明确不做或推迟。
- devops agent 当前零探针，M1 落地后其探针卡才有数据——与 Agent 面板三页「页面不渲染空页」口径一致，交付顺序上 M4 合流。
- Windows `ProcessChecker` 为退化实现（存在性检查，`process_windows.go:9-15` 注明），保持现状，见 §5.3 裁决。
- server 引入 core/healthprobe import 是首条 server→core 依赖边（无环；core 零内部依赖），R4 收敛时生效。
