---
title: Agent 健康监控与恢复——外部方案重合度审计与 server 下放迁移评审（评审定稿）
---

# Agent 健康监控与恢复评审（定稿）

## 状态

- 状态: **评审定稿（未动实现码；R2 起为代码批）**。来源: 用户 2026-10-14 五条——外部设计建议做可嵌入 Health Monitoring & Recovery 模块；本仓早有监控外部第三方服务存活的需求（server 侧实现），要拆到 agent 统一为基础能力；补充问题（第三方监控是否需要新增 agent 部署）；硬约束（探测实现收口 core/healthprobe 一份 + 实现层/实例层分标 + 熔断器归属判定）；三点拍板（归属/通道/动表时机）；外部评审文档核验（docs/review/chatgpt-2026-10-14-probe-architecture.md）；协议适配骨架设计探讨单（§5.5）。
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

### 5.5 协议适配骨架设计（设计探讨单 2026-10-14，接 §5.4）

§5.4 立场与来历不变。本节把「协议探测器」拆成三层并给骨架契约与状态语义。

#### 5.5.1 三层切分与接口契约

- **层 1 探测内核（已有，不动）**：`core/healthprobe` 的 `Checker` 契约（`healthprobe.go:31-34`）、阈值/状态机/窗口/时间线（`probe.go:51-97`）。协议探测**只许实现 Checker 契约**——失败计数、防抖、窗口、上行全部复用内核；骨架不得自持计数器/状态机（「不另起一套」的硬口径）。
- **层 2 协议适配骨架（新增，共用）**：声明式多步交互运行时，职责=连接生命周期、步骤推进、阶段标记、期望匹配、结果语义、传输层重试。进 `core/healthprobe`（仅 stdlib：net/bytes/regexp/time——与 HTTPChecker/TCPChecker 同级，满足「实现一份」硬约束）；游戏协议知识一律不进此层。
- **层 3 游戏协议适配器（每游戏一份）**：包格式/握手序列/加密/登录。L2 形态=纯数据（steps 配置，零码）；L3 形态=Go 代码，唯一要求=实现 Checker 契约后直接进内核装配（Probe+阈值+窗口+Timeline+OnWindow）。骨架对 L3 只提供可选辅助（定长/行分帧读取），不新增顶层概念。

接口契约（Go 签名；配置落 `probe.targets[].steps`，类型名沿用仓库 spec/descriptor 既有词——PageSpec、FunctionDescriptor 先例）：

```go
// 步骤期望：字节级匹配，三形态覆盖静态私有协议握手
type StepExpect struct {
    Contains []byte // 响应含该字节段（默认）
    Prefix   []byte // 响应以此开头
    Regexp   string // 正则（全缓冲）
}

// 单步：发什么、等什么、等多久。Name=阶段标记（进 Result.Details）
type StepSpec struct {
    Name    string        // 阶段标记：handshake / login / ping ...
    Send    []byte        // 静态载荷；留空=纯等待（只收不发）
    Expect  StepExpect    // 留空=任意响应即通过
    Timeout time.Duration // 单步超时；<=0 用默认 5s
}

// 骨架本体：实现 healthprobe.Checker——一次 Check=新建连接、走完步骤、关连接
type ProtocolChecker struct {
    Address string     // host:port（L2 v1 仅 TCP；HTTP 归 HTTPChecker）
    Steps   []StepSpec
    // Dialer/Reader 可选注入（测试用；生产 nil）
}
```

骨架内部规则（围绕「内核是唯一失败计数权威」）：

1. 每次 Check 新建连接、不跨 Check 复用会话；无状态，避免「available 由陈旧会话维持」。
2. 步骤失败即 Check 返回 error，error 串携带阶段标记（`protocol failed at step 2 (login): ...`）——分类落 `Window.LastReason`（`probe.go:26` 既有字段）。
3. 阶段标记出口=**R2 顺手在 Result 增加 `Details map[string]string`**（与 §6 的 LatencyMS 同批）：`phase=login`、`step=2/5`；`Window.LastReason` 供告警文案。
4. 传输层重试仅限「建连阶段瞬时错误」（dial reset 且未发出任何字节）单次重试，计数进 Details（`reconnect=1`）；握手半途步骤**不重试**（非幂等，登录半状态）。防抖与失败计数由内核阈值承担。
5. 期望匹配只做字节级（Contains/Prefix/Regexp）；加密、压缩、挑战-响应表达不了 → 升级 L3 代码适配，骨架不塞加密原语。

#### 5.5.2 四条接入路径（每游戏一份）

| 路径                             | 可行性                                                                                                                                                        | 成本                                                                                                | 适用                                   |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | -------------------------------------- |
| ① 游戏自带测试/压测 bot          | 中——存在性取决于游戏方，平台无法先行；有退出码/输出即可判活                                                                                                   | 每游戏一份胶水（包成 Checker；子进程判活有 supervisor 先例：`ops_server.go` 的 os/exec+退出码语义） | 有成熟自动化测试资产的游戏             |
| ② 游戏方客户端 SDK               | 低-中——客户端 SDK 多为 C#/C++/Lua，Go 不可直接链接；只能子进程包装或等其暴露 HTTP/CLI 面                                                                      | 高：跨语言进程管理 + 协议版本随客户端升级同步                                                       | 仅当 SDK 有独立可执行面时才顺          |
| ③ 声明式探测定义（**默认路径**） | 高——平台可先行（骨架是与游戏无关的平台能力）；HTTP 通用协议=其退化特例（L1 HTTPChecker 已覆盖）；握手固定的私有协议（发握手包等定长回包，网关类常见）表达充分 | 平台一次建骨架；每游戏只写 steps 配置零码                                                           | 握手序列静态或可模板化的协议——默认入口 |
| ④ 逆向客户端协议（不推荐）       | 可行但协议一改即断、维护差、合规风险（用户 2026-10-14 已列）                                                                                                  | 持续反维护                                                                                          | 最后手段，需书面授权才做               |

**本仓游戏实况（关键事实）**：本仓管理的 demo 全部是「游戏→agent」SDK 注册面（`sdks/go/examples/{basic,comprehensive,demo}` 经 `CROUPIER_AGENT_ADDR=127.0.0.1:19091` 注册函数）与 croupier 控制面示例（`examples/`）；**没有任何一款带玩家侧网络协议的游戏**，`bot/simulator/fake-client` 零命中。结论：现状下 ①②④ 都无对象；③ 作为默认路径先行（L1+L2 覆盖通用协议与静态私有握手），真实游戏接入时按「有无 bot（①）、握手是否固定（③）」逐游戏选路，②④ 不默认。

#### 5.5.3 「协议未接入」一等状态

两条正交轨道——**内核两态窗口不动**，协议接入度是标注层（先例：dbmon firing/resolved 是 Alert 生命周期标注，不是探测状态机）：

| 目标状态                        | 定义                                                      | 是否产生 Check/窗口            | 面板/告警语义                                                                                                                                         |
| ------------------------------- | --------------------------------------------------------- | ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| not-integrated 未接入           | 目标无协议探测定义（配置事实）                            | **不产生**——绝不判 unavailable | 探针卡灰态「未接入协议探测」；无 Alert                                                                                                                |
| reachable-unverified 可达未验证 | L1 端口/HTTP 探到、协议层未接入                           | L1 的 Check 正常进窗口         | 窗口/告警反映「端口/HTTP 可达性」；卡标注「可达≠可用」（§5.4 三层次）；不得报成「服务挂了」                                                           |
| protocol-failed 协议验证失败    | Check 执行了、握手/登录阶段失败（区别于 dial 阶段连不上） | 进窗口（真故障信号）           | 告警 reason 带阶段标记（LastReason=`protocol failed at step ...`），与「连接不上」分类呈现——连上但协议被拒大概率是服务端逻辑/账号池问题，不是网络挂了 |
| verified 已验证                 | 协议 Check 曾成功                                         | 进窗口（available）            | 卡上「已验证」+ 最近验证时间（At）                                                                                                                    |

硬规则：**只有执行了的 Check 驱动窗口**；not-integrated / reachable-unverified 一律不得产生 unavailable 判定——否则上线即淹没真告警（本设计要防的事故）。

#### 5.5.4 分阶段与验收口径

- **L1 端口+HTTP（现在可做，=R2 装配面）**：`probe.targets` 配 http/tcp 目标 → 窗口/时间线/SendAlertEvent 上行全链绿；探针卡「可达未验证」标注生效（目标声明了协议期望但无协议探测时显示）。
- **L2 声明式私有协议（下一批）**：骨架 `ProtocolChecker` 进 core/healthprobe 零外部依赖；测试内 mock listener 演示私有握手（发握手包→期望前缀→发登录→期望成功码），**配置零码接入**；阶段标记进 Result.Details；not-integrated 配置缺失时不产生窗口事件、卡灰态正确；protocol-failed 与 dial 失败在 LastReason 可区分。
- **L3 深度协议适配（P2，按游戏重要度逐个接）**：适配器=实现 Checker 契约的每游戏包；加密/挑战-响应等声明式表达不了的才上；玩家代表性网络选点方案随立项给（§5.4 排期不变：P2 按需触发，触发即玩家视角）。

#### 5.5.5 命名裁定

- 骨架=`ProtocolChecker`（沿 HTTPChecker/TCPChecker/ProcessChecker/DBChecker 的 {域}Checker 先例）；steps 配置沿用仓库 spec/descriptor 既有词（PageSpec/FunctionDescriptor 先例），不造「剧本/脚本」顶层词。
- 状态集合=标注层概念，沿用「Window 之外的生命周期标注」先例（dbmon firing/resolved）。
- 继续不引入 ProbeTarget/RecoveryHandler/Monitor 等外部词。

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
- **R2 agent 装配面：已完成**（代码批，87 条新用例 + 全量门禁绿）：`probe.targets` 配置契约（TargetConfig 小驼峰、每目标视角 semantics 声明，缺省 readiness）+ Checker 工厂（http/tcp/**db 全进 core**，process 不进工厂）+ 多 Probe 装配（Manager 每目标 goroutine）+ `SendAlertEvent` 上行接线（report.Conn → tcp/mux 双实现 → wire `MsgAlertEvent=0x04011E`）+ capture 迁移统一装配 + `Result` 增 Target/LatencyMS/Details。
  - **验收「OnWindow 消费者禁止触发进程重启」已生效**：capture 迁移后窗口消费者只打 warn 日志；sidecar ProbeManager 窗口消费者只做上行（硬约束注释落在 `internal/app/agent/probe_manager.go` 文件头，不引用 supervisor/ops 装配）。
  - **已知边界**：(a) server 侧 R2 只做「接得住/留得下/查得到」——`internal/server/probe_events.go` 诊断环（500 条/agent、30min 保留、按窗口记日志），**不写 Alert 表**，窗口→Alert 生命周期属 R3；(b) capture/devops 无 metrics 采样器，统计面（ProbeStatsSampler 折叠进 `MetricsReport.probe_stats`）仅 sidecar 生效，capture 窗口事件暂只落本地日志；(c) devops 的 `probe.targets` 配置段待后续批接入（sidecar 已接线，capture 走数据源派生单目标）。
- **R3 server 消费面**（M2 server 侧 + M4）：窗口→Alert 生命周期（alertID=`probe:<agentID>:<target>`）+ server 侧内核装配（手动诊断/自身依赖）+ 探针卡面板（与 Agent 面板三页合流）。
  - **窗口→Alert 生命周期：已交付**（R3 首子批，`internal/server/probe_lifecycle.go` + `AlertModel.UpdateLifecycle`）：probe.window 开/合转 firing/resolved，stable alertID 同行转换（`UpsertByAlertID` 去重不更新语义不适用，走 FindBy+UpdateLifecycle 单语句刷 status+message+details，details 须 `datatypes.JSONMap` 走 Valuer）；recovered 无在案 firing 也落 resolved 行（事实优先，server 重启错过开窗不丢闭窗）；firing 中重放幂等；resolved→firing 复活刷最新窗口；details 随窗口永久落最近 5 次失败明细（诊断环按 `at >= startTs` 过滤防历史窗口串台，明细为诊断环 30min 保留的快照拷贝）；scope 打戳取事件载荷（同 capture 口径，空 = 平台级）。**已定边界**：窗口告警不触发 notify/出口分发（分发面=AlertRule evaluator 规则路径；窗口告警先落 ops 告警中心，需要通道分发时在 transitionFiring 单点加）；agentID 取自载荷不核会话（与 capture 告警同口径）。
  - **统计消费（evaluator 指标路径）：已交付**（R3 次子批，`internal/platform/alertrule/evaluator.go`）：`probe.<target>.<successRate|failed|checked|latencyP95Ms>` 四字段进 ExtractMetric/ExtractMetricFromName（target 可含 `:`/`.`，LastIndex 切分同 disk 口径）；successRate 为 0-1 比率且 **checked=0 视为指标缺失**（不把「没测」当「全成功」）；latencyP95Ms 为 5 桶直方图桶粒度近似（累计占比首达 95% 的桶边界，全溢出取 5000 下界，口径锚 `core/report.LatencyBucketBoundaries`）；规则校验（`internal/api/alert/rules.go` 调 ExtractMetricFromName）自动生效。
  - **probe.policy 采样策略管理面与下发：已交付**（R3 三子批，kernel + server 管理面 + wire 下行 + agent 热更三段）：
    - 内核 `core/healthprobe/policy.go`（`ParseProbePolicy` 为唯一校验真值，发布预检与 agent apply 共用）：三粒度 base interval / targets 按目标 / windows 时段（本地时区钟点 `HH:MM`，start 含 end 不含，不跨午夜）/ events 特殊时点（RFC3339 必带显式偏移 + duration）；叠加优先级 event > window > target > base；`RunDynamic`/`StartDynamic` 间隔函数读侧现取（探针每轮探测后向 `intervalFn` 现取，热更下轮即跟随，无需重建探针，不重启 agent）。
    - server 管理面 `internal/api/ops/probe_policy.go`（五端点 GET/PUT/validate/rollback/versions，ConfigVersion key=`probe.policy`、namespace=ops、乐观锁 409、回滚=历史内容再过当前校验再发一版）+ wire 下行 `internal/server/control_handler.go`（`MsgProbePolicyRequest=0x04011F`/`Response=0x040120`，按 agent 会话 game/env scope 读 `FindLatestByScope`）。
    - agent 消费 `internal/app/agent/probe_policy.go`（configsync Puller 30s 轮询）+ `probe_manager.go`（`SetPolicy` 热换快照 + `IntervalFor` 每轮现取 + 开窗目标 `unavailable` 提频 C#5：interval/2、下限 5s、绝不放慢、autoBoost 默认开）。
    - **已定边界**：未下发（空版本）保持本地 `probe.targets` 声明间隔、不清空已生效策略；拉取失败/策略解析失败保持 last-good 且版本游标不推进（下轮重试）；策略已下发即以策略为唯一真值（基础未声明=30s 显式默认，不做「本地值兜底」隐藏层）；server 管理面 import `core/healthprobe`（发布预检与 agent apply 同真值，capture gates 先例；core 零内部依赖无环，R4 收敛时再归位）。
  - **probe_stats 持久化：已交付**（R3 四子批，迁移 0044 + `internal/platform/registry/metrics.go`）：`agent_metrics_history.probe_json` 列（同表 cpu/memory/disks JSON 先例，零新表零新查询面）；`persistToDB` 落探测统计快照（无探针面上报留 NULL 不落 "[]"），`getFromDB` 重建 `report.ProbeStats`——DB 历史回读与内存同口径，evaluator/ops 历史查询消费面白得。**已定边界**：存量行不回填（历史窗口无统计快照）；迁移决策=列持久化而非内存环（表、历史查询路径、Prune 生命周期全部既有，内存环要新环+新查询 API 才是新增面）——已代决(巡检 R2)。
  - **server 侧内核装配（手动诊断/自身依赖）：已交付**（R3 五子批，§4.2/§4.3）：server 配置 `probe:` 段（目标契约与 agent probe.targets 同一份，`internal/config.ServerProbeConfig`）→ `internal/svc/server_probe.go` 装配（core/healthprobe 同一实现，server 由此引入 import，core 零内部依赖无环；组包复用 core/report.ProbeUplink——窗口/明细与 agent 上行完全同构）→ `ControlService.HandleServerProbeEvent` 入站（诊断环 + 窗口→Alert 生命周期同一条链，视角标注=server，alertID=`probe:server:<target>`，与 agent 视角并存不串台）；手动诊断 `POST /api/v1/ops/probe/diagnose`（按名即时探一次，与周期探测共用同一探针实例与状态机；探测面未配置/目标未配置 404 区分）。**已定边界**：server 本机探针无 probe.policy 下发语义（采样策略只作用于 agent 侧采样面，server 间隔走本地配置）；server 探测统计不进 MetricsReport 批量面（server 无 agent 上行链路；ops 汇总页消费 agent 视角，server 视角走告警行）；配置非法启动失败（与 agent 同口径）。
  - **ops 在线汇总（§13.2 状态矩阵）server 面：已交付**（R3 六子批，`internal/api/ops/probe_summary.go` + `GET /api/v1/ops/probe/summary`）：一行 = 目标 × 执行位置（探活结果 available/unavailable 两态 + 开窗时长 + 窗口聚合计数 + 最近失败原因 + 数据新鲜度锚点），**不出单一合成分**（权重无真实数据支撑，B#3）；两视角并存不串台——agent 视角取各 agent 最近一份 MetricsReport 的 `probe_stats` 快照（30s 批量面新鲜度，`entry.Received` 回传锚点），server 视角取本机探针面当前状态（`ServerProbeManager.Statuses()`，读侧**不探测**——呈现现状不注入新事实，与手动诊断端点分工明确）；scope 走 `X-Game-ID/X-Env` 头（`GameScopeFromContext`，带 scope 显示同 scope 行 + 平台级行、空 game/env 行，告警列表 4862 同口径；server 探自身依赖天然平台级）；在线趋势列为**「未接入在线数据」灰态**（`onlineDataIntegrated=false` 显式声明，SDK metric 入口未排期——复用 §5.5.3 not-integrated 一等状态，不造第二套，字段先行以便接入后只改值不改契约）。**已定边界**：窗口未开但从未探测过的 server 目标按「可用」呈现（无窗口=无故障证据，事实缺失不谎报故障，与 probe 内核两态一致）。
  - **ops 在线汇总页前端卡片：已交付**（R3 六子批 b 段，`web/src/pages/Ops/Nodes/ProbeSummarySection.tsx` + `web/src/services/api/opsProbe.ts`）：状态矩阵挂**节点维护页**（`/ops/nodes`，不另起面板——§13.2 B#1「复用 ops 模块与既有 game/env 维度」，也避开尚待用户批准的 Agent 面板三页）；列=执行位置（含 scope 行，平台级显式标注）/ 目标 / 视角 / 探活两态 / 窗口时长 / 检查·失败 / 成功率 / 最近失败原因 / 数据时间；**异常行前置排序**（先看有问题的）；`checked=0` 渲染「-」而非 100%（没测过不是全成功，与 evaluator 指标缺失口径同源）；在线趋势「未接入」灰态常驻 Alert 并说明来源（游戏自身上报、SDK metric 入口未接入），不静默消失；scope 走顶栏游戏/环境（`useScopeReload` 切 scope 重拉，与 #38 族一致）；加载失败走行内 Alert 并清空旧数据，不留陈旧结论。
  - **待做**：无（R3 全部子批收口；后续随 §13.2 B#2 的 SDK metric 入口接入把 `onlineDataIntegrated` 翻真，并按真实数据评估骤降检测三档——权重在有数据前不拍）。
- **R4 dbmon 收敛收尾**（M3）：DBSource 归属列（已拍板动表，编号迁移三处同步）+ 周期探针按归属下发 + dbmon 切 DBChecker **删自有 SQL 电池** + server 手动诊断保留。
- **不排期**：外部目标自动恢复、恢复动作注册、全局并发恢复上限（§8 边界）；游戏客户端模拟探测单独 P2（§5.4，协议适配骨架设计已出：§5.5——L1=R2 顺路、L2 声明式为默认路径、L3 按游戏逐个接）。

## 13. 上行统计、在线健康度与采样策略（设计单 2026-10-14 三条，仍不写码）

### 13.1 探测结果回流与不对称上报（A）

核心口径：**正常态与异常态的上报量不对称**——正常态聚合省上行，异常态明细保现场；「有问题时更需要回流」= 失败时不能为省流量只报一个 bool。

| 维度       | 正常态（聚合/采样）                                                                                                                                                                          | 异常态（明细）                                                                                                                                   |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| 上什么     | 每 target 每窗口：检查次数、失败次数、成功率、时延分布（桶边界配置，默认 [50ms,200ms,1s,5s]——覆盖内网毫秒级到拨号超时级的常见量级）、开窗中的窗口持续时长、探测点分布（多 agent 视角各一行） | 逐次失败明细：目标标识、执行位置（agentID）、探测类型、语义、时间戳、失败阶段（phase）、错误原文（截断默认 2KB 防日志炸弹）、最近 N=5 次结果序列 |
| 什么时候   | 批量窗口（默认 30s，随 agent metrics 周期）                                                                                                                                                  | 即时（不等批量窗口）                                                                                                                             |
| 走哪条通道 | MetricsReport 批量面（既有）                                                                                                                                                                 | SendAlertEvent 事件面（新，已拍板）                                                                                                              |

**通道结论（A#2，与用户倾向一致）**：统计上报与告警事件**不是同一消息**——统计走批量通道、告警与失败明细走事件通道，两者都带目标标识+执行位置+时间戳，按目标聚合。依据：§11 已决 2 的 QoS 区分（告警要即时可靠、指标是周期批量）；统计=metrics 语义天然同面。

复用点（几乎零新通道）：

- agent 侧 metrics 收集器已有 **Sampler 折叠先例**：supervisor 采样器折进每份 MetricsReport（`internal/app/agent/upstream.go:622,645` `WithSupervisorSampler`）→ 探测统计=新增一个 probe 采样器同型折叠，周期默认 30s（`upstream.go:632` 既有默认，零新默认值）。
- server 侧已有 MetricsStore 接收/持久化（`internal/platform/registry/metrics.go:139,198`）与**告警规则消费先例**（`internal/platform/alertrule/evaluator.go:42-56` AlertRule 直接评估 MetricsReport）→ 「成功率低于 X」「时延 P95 超阈值」类探测告警零新告警链路。
- SendAlertEvent 承载两类 kind：`probe.window`（窗口开/合，server 转 Alert 生命周期——R3，首子批已交付见 §12）与 `probe.detail`（失败明细，诊断面不触发告警）。明细 server 侧进内存 ring（容量 500/agent，同 supervisor 事件环先例），开窗时最近 N 次序列随 Alert details 永久落库；长期留存=agent 本地 FileTimeline 真值（既有双通道语义，零新表）。

上行量控制（A#3，参照 supervisor 事件环先例）：批量窗口内成功样本只计数不逐条；每 target 每窗口明细条数上限默认 10，超出计 `droppedDetails` 进聚合一栏（失败总数永远如实——上限只裁明细不裁事实）；agent 侧 outbox 环形缓冲 + 丢弃计数。

### 13.2 在线数据汇总与在线健康度（B）

**复用 ops 模块与既有 game/env 维度，不另起面板/维度表**（B#1，file:line）：

- ops 模块挂载面：`internal/api/ops/`（alerts.go:32-33 基础设施告警列表先例；metrics_history.go:13 agent 指标历史查询先例）——在线统计页挂本模块。
- game/env 维度：`internal/model/alert_model.go:30-37`（Alert GameID/Env scope 列，空=平台级）+ 迁移 0043（`internal/svc/migrations.go:1411`）+ `svc.GameScopeFromContext` 过滤——在线汇总按同维度切（game × env），零新维度表。

**在线数据口径（B#2）**：在线人数/在线状态/峰值的来源=**游戏服务器自身**（业务真值——agent 侧探活推不出在线人数）。本仓现状：SDK 面**无** metric 上报入口（`sdks/SDK_FEATURE_MATRIX.md` 零 metric 条目）→ **通道复用、入口新增（最小）**：SDK 增一条 metric 上报 API（游戏→agent 本地连接→折叠进 MetricsReport，同 13.1 采样器路径），游戏侧每周期报当前在线数即可；峰值=server 从指标序列按窗口取 max（默认 24h 展示口径），不需游戏侧计算。CDC capture 可作补充口径（游戏库在线表行数）但标「派生口径，非业务真值」，不作主源。游戏未接 metric 入口时 ops 面显示「未接入在线数据」灰态——复用 §5.5.3 not-integrated 一等状态语义，不造第二套「未接入」。

**在线健康度（B#3，不拍脑袋）**：v1 呈现为**状态矩阵+证据**，不出单一合成分（权重无真实数据支撑，出了就是拍脑袋）：

| 构成     | 取值                                | 口径与阈值来源                                                                                                                                                                               |
| -------- | ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 探活结果 | available / unavailable（内核两态） | Window 状态（探测是事实不是渐变分数）；不造健康分                                                                                                                                            |
| 在线趋势 | normal / drop / crash 三档          | 骤降检测=对比滚动基线（同时刻前 N 天同窗口均值，N 默认 7 天——消除日周期）；骤降阈值默认 50%/5min，**走配置可调**（分钟级腰斩是掉线事故典型形态，缓慢流失不告警）；阈值与窗口全配置化零硬编码 |
| 异常证据 | 最近失败明细 / 骤降区间明细         | 13.1 明细面直接挂卡（A/B 共用）                                                                                                                                                              |

骤降告警直接落**既有 AlertRule evaluator**（在线指标进 MetricsReport 后自动可评估，`alertrule/evaluator.go:42` 同先例）。若未来确需单一展示分，沿 dispatch 打分先例（`agent_health_tracker.go:38-105` 历史成功率指数衰减口径）用真实数据定权重，延后。

**与 A 的关系（B#4）**：**共用同一条上行通道（MetricsReport 批量面），报文种类不同**（探测统计 vs 在线指标）；告警类事件统一走 SendAlertEvent / evaluator。依据：通道按 QoS 分（批量 vs 即时），不按数据域分——两者都是周期批量统计语义。

### 13.3 采样频率策略（C）

**频率是 server 下发的策略，agent 不本地写死**（C#1，与 §5.2 约束 3「配置与执行分离」同一原则，唯一来源）。三种粒度（C#2）：

```yaml
probe:
  policy: # server 管理面维护，configsync 下发；agent 本地仅缓存只读副本
    interval: 30s # ① 基础采样频率（全局默认；未设置即此值）
    targets: # 按目标覆盖
      - target: "game-db-primary"
        interval: 10s
    windows: # ② 按时间段的频率（叠加覆盖；时区显式，默认服务器配置时区）
      - start: "19:00"
        end: "23:00"
        interval: 10s
    events: # ③ 特殊时间点（一次性/日历式；RFC3339 带偏移，无时区歧义）
      - at: "2026-10-20T09:00:00+08:00" # 活动开服前
        duration: 30m
        interval: 5s
```

**默认值（显式，含理由）**（C#4）：基础间隔 **30s**——与 configsync 轮询默认 30s（`configsync.go:14-15`）及 metrics 上报周期默认 30s（`upstream.go:632`）同档，一条连接上三类周期统一量级；配合内核默认 fail=3 → 最坏 90s 开窗（可用性监控常见可接受检测延迟；k8s 默认 10s×3=30s，本仓跨网段探测更重取 30s）。按目标/时段/特殊点未设置=继承基础间隔，无隐藏层级。

**落点与生效（C#3）**：server 管理面=ConfigVersion 资产（`internal/model/config_version.go:13-14`「versioned configuration values for arbitrary keys」——`probe.policy` 一个 key，乐观锁/发布/回滚/appliedRevision 全白得，**零新表**）；下发=configsync Puller（30s 轮询，`configsync.go:57-61`）；**生效=热更**（apply 回调重建 ticker，不重启 agent），生效延迟=下个轮询周期，最坏 30s；断连 configsync 时按最后已知策略跑（降级语义诚实标注）。

**异常态兜底（C#5，叠加关系）**：时段降频只作用于**成功样本**的采集与上报节奏；目标处于开窗（unavailable）状态时异常态覆盖时段策略——(a) 失败明细仍逐次即时上报，不受任何降频影响；(b) 窗口事件即时；(c) 自动提频 interval/2、下限 5s、默认开（可关）——理由：recover=1 意味着下一次成功即闭窗，提频减半恢复检测延迟；下限 5s 防内网打爆。时段降频**不会**导致异常漏报：失败路径无周期约束。

### 13.4 分期归属与命名

- R2 起步批含：基础 interval 消费（默认 30s）+ probe 采样器折叠进 MetricsReport（统计面）+ SendAlertEvent 的 probe.window/probe.detail 两 kind。（R2 批已交付，见 §12；采样策略三粒度与 server 统计消费属 R3。）
- R3 含：server 统计消费（MetricsStore/evaluator 复用）+ ops 在线汇总页（复用 ops 模块与 game/env 维度）+ `probe.policy` ConfigVersion 管理面与 windows/events 两粒度。（`probe.policy` 管理面/下发/agent 热更已交付，见 §12。）
- SDK metric 入口（B 的游戏侧上报）为跨面独立小批，随第一个真实游戏接入排期；未接入前 ops 面灰态（§5.5.3 语义）。
- 命名：`interval`（与 configsync/metrics 既有字段同名同义）、`windows`/`events`（通用词）、`probe.window`/`probe.detail`（沿 herald 出口 `probe.unavailable/probe.recovered` 注释先例 `probe.go:31-32`）；「在线健康度」=状态矩阵，不引入「健康分」新概念（dispatch 侧 score 是既有词，如未来引入合成分沿用之）。

## 已知边界

- 本评审只做审计与方案，未动任何实现码；§2 缺口 5/6/7 明确不做或推迟。
- devops agent 当前零探针，M1 落地后其探针卡才有数据——与 Agent 面板三页「页面不渲染空页」口径一致，交付顺序上 M4 合流。
- Windows `ProcessChecker` 为退化实现（存在性检查，`process_windows.go:9-15` 注明），保持现状，见 §5.3 裁决。
- server 引入 core/healthprobe import 是首条 server→core 依赖边（无环；core 零内部依赖），R4 收敛时生效。
