---
title: Agent 健康监控与恢复——外部方案重合度审计与 server 下放迁移评审（待批）
---

# Agent 健康监控与恢复评审（待批）

## 状态

- 状态: **Proposed（评审定稿，未动码）**。来源: 用户 2026-10-14 两条——外部设计建议做可嵌入 Health Monitoring & Recovery 模块；本仓早有监控外部第三方服务存活的需求（server 侧实现），要拆到 agent 统一为基础能力。
- 评审基准: `core/healthprobe`（三 agent 共用探针包）+ sidecar supervisor（进程恢复环）+ server 侧 dbmon / serverstatus / dispatch 熔断器。**外部设计的 Checker/RecoveryHandler/Target 命名一律不引入**，重合概念按本仓既有命名与语义归位。

## 1. 重合度审计（外部设计 → 本仓现状，逐条）

| 外部设计概念                                     | 本仓现状                                                                                                                                                                                                                                | 结论                                                                                                                                                |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Checker` 接口                                   | `core/healthprobe/healthprobe.go:32-34`（`Check(ctx) error` 单值契约）+ `CheckerFunc`（:37-40）                                                                                                                                         | **已有，且更严格**：单值返回天然禁止 Checker 携带副作用面                                                                                           |
| 三语义探针（状态机）                             | `healthprobe.go:21-29`（`Semantics` liveness/readiness/heartbeat 闭集）；两态机在 `probe.go:51-71`                                                                                                                                      | **已有**：`Probe` 是唯一状态机持有者                                                                                                                |
| 防抖（debounce）                                 | `probe.go:76-87` `WithThresholds`（默认 fail=3/recover=1，对标 k8s failureThreshold/successThreshold）；推进逻辑 `probe_run.go:11-71`（连续失败达 failThr 开窗、连续成功达 okThr 闭窗，任一成功清零失败连击）                           | **已有**：阈值+连击清零即防抖，无缺口                                                                                                               |
| 失败阈值与故障窗口                               | `probe.go:20-29`（`Window`：StartTS/EndTS=0 未恢复/持续时长/LastReason/scope）；历史环 `probe.go:99-109` + `probe_run.go:118-123`（默认 100 截断）                                                                                      | **已有**                                                                                                                                            |
| 窗口事件出口                                     | `probe.go:33-39`（`WindowEvent`/`WindowListener`，注释预留 herald 出口 `probe.unavailable/probe.recovered`）；`OnWindow` `probe.go:95-97`                                                                                               | **已有**                                                                                                                                            |
| 平台进程适配层                                   | `process_unix.go:13-30`（信号 0 探活）+ `process_windows.go:9-27`（FindProcess 退化，注明精确判活按需再引 x/sys/windows）                                                                                                               | **已有**                                                                                                                                            |
| 内置探测方式                                     | HTTP `healthprobe.go:43-71`（2xx/3xx 通过）、TCP `healthprobe.go:74-98`（拨号即关）                                                                                                                                                     | **已有**（icmp/dns 无——见缺口）                                                                                                                     |
| 心跳语义                                         | `heartbeat.go:14-53` `HeartbeatFile`（mtime 即最近活体时刻）+ `core/supervisable/supervisable.go:75-97`（beatLoop）                                                                                                                     | **已有**                                                                                                                                            |
| 窗口持久化                                       | `timeline.go:13-88`（`FileTimeline` JSON-lines 真值通道 + `LoadTimeline` 读回）                                                                                                                                                         | **已有**                                                                                                                                            |
| `RecoveryHandler`（恢复动作钩子）                | **进程域已有**：sidecar supervisor 恢复环 `internal/app/agent/ops_server.go:451-590`（detect_down → 熔断检查 → 指数退避 → auto_restart → restart_failed 回环）；**外部目标域没有**（agent 对外部第三方服务无恢复动作，也不应有——见 §4） | 进程域已有，外部目标域明确**不做**自动恢复                                                                                                          |
| 恢复闭环（动作后再探、判定恢复成功 vs 持续失败） | `ops_server.go:573-615`：`armSuccessTimer` 新实例存活超过退避封顶才清连续失败计数（防启动即崩刷穿退避序列，设计 §3.4）                                                                                                                  | **已有**（进程域）                                                                                                                                  |
| 恢复权限边界                                     | `ops_server.go:443-448` `autoRestartAllowed`：进程级 AutoRestart 开关 AND ops 双闸（Enabled+AllowRestart）                                                                                                                              | **已有**（进程域）                                                                                                                                  |
| 并发恢复防护                                     | 每进程单监视 goroutine + 所有权守卫（`p.cmd != watched` 每次状态迁移前重查，`ops_server.go:463-476/486-491`）；退避 sleep 在锁外可被停机信号打断                                                                                        | **已有**（进程域；全局并发上限无——见缺口）                                                                                                          |
| 熔断器                                           | `internal/platform/dispatch/agent_health_tracker.go:13-35`（Closed/Open/HalfOpen）+ 状态迁移 `:232-304` + 打分/衰减 `:38-105`                                                                                                           | **已有，但语义不同**：这是**请求结果驱动**的路由熔断（`RecordSuccess/RecordFailure` :175-207），不是探测驱动。归 server 路由面，**不下放**（见 §3） |
| Monitor Registry（目标注册）                     | 无动态注册面：capture 探针目标是 config 推导的单源（`agents/capture/runner/runner.go:633-657`）；devops 无探针                                                                                                                          | **真缺口**（见 §2）                                                                                                                                 |
| 自动重启外部服务/回调类恢复                      | 无                                                                                                                                                                                                                                      | **缺口按边界处理**：外部目标只告警不自动恢复（§4）                                                                                                  |

## 2. 真实缺口（全部）

1. **探针目标注册面**：没有「一个 agent 声明 N 个外部目标（http/tcp/db）各自探」的配置与装配。现状每 agent 至多一个硬编码目标（capture 的数据源，`runner.go:644-657`；devops 零探针——healthprobe 包注释里「devops 探 CI 端点」是规划用法未落地）。
2. **窗口事件上行通道**：`OnWindow` 目前只打本地日志（capture：`runner.go:648-653` warn 日志）；`core/report` 的 `Conn` 最小面只有 `SendTaskEvent/SendMetricEvent`（`report.go:17-23`），窗口事件没有到 server 的上行线（proto/wire 扩展点，见 §7 未决）。
3. **server 侧窗口消费**：收到 unavailable/recovered 后进 Alert 生命周期的链路不存在。dbmon 已有同语义先例可复用：`internal/api/dbmon/service.go:156-203`（firing/resolved 状态机 + alertID 去重）。
4. **DB 类 Checker**：dbmon 的探针本体（`internal/api/dbmon/probe.go`，只读 SQL + 5s 硬超时 + 云 RDS 不暴露指标报 degraded 不猜）是 server 侧实现；agent 侧无 DB 探测能力。
5. **icmp/dns Checker**：无（blackbox_exporter 的分类里仅这两类缺；可用 stdlib 补，零依赖）。
6. **全局并发恢复上限**：进程域是每 target 串行（单监视 goroutine），无跨 target 并发上限。**建议不做**——恢复动作只存在于进程域，agent 间无共享恢复池需求；真出现再加。
7. **恢复动作注册**（重启服务/回调/告警三分支）：进程域=重启（已有）；外部目标域=只告警；「回调」类动作无需求，不预留死接口（对齐 `report.go` 注释「v1 不预置死接口」口径）。

## 3. server → agent 迁移方案

### 3.1 先归位：三类「健康」在 server 侧的真实身份

| server 侧件                                                                                                                             | 身份                                                                                                                                                                                     | 迁移动向                                                                                                                                                                                                         |
| --------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `dispatch.HealthTracker` + 熔断器（`internal/platform/dispatch/agent_health_tracker.go`，接线于 `internal/svc/service_context.go:525`） | **路由面**：server 挑哪个 agent 转发函数调用的打分/熔断，由请求成败驱动                                                                                                                  | **不下放、不改**。下放到 agent 是范畴错误——agent 不做路由决策。`internal/config/config.go:677-687` 的 `healthCheck` 配置节专属此面，**不改名不挪位**，与 agent 探针配置（`probe` 节，见 §5）是两个口径，边界写死 |
| `dbmon`（`internal/api/dbmon/`）                                                                                                        | **外部第三方服务存活监控**——用户说的「早就有、在 server 侧」的主体：DBSource 注册表 + 只读 SQL 探针 + Alert 生命周期。当前探针是**按需**（`handler.go:70-71` POST /dbmon/probe），无周期 | 探针执行**下放 agent**（网络位置真值：agent 贴着游戏库，server 隔着网关）；server 保留 sources 注册表、告警聚合、手动诊断探针                                                                                    |
| `serverstatus`（`internal/platform/serverstatus/`，provider 闭集 noop/atlas/http `registry.go:10-12`）                                  | **维护状态源**（显示面 + 告警 gate 裁决 fail-open，`gate.go:14-31`），不是存活探针                                                                                                       | **不动**。它是状态查询不是周期探测，职责不同                                                                                                                                                                     |

### 3.2 迁移顺序（每步可回退）

- **M1 加法：agent 探针装配面**——per-agent 配置 `probe.targets[]`（type: http/tcp/process/db）+ Checker 工厂 + 多 `Probe` 装配 + `OnWindow` 汇聚。capture 现有单源探针改为走同一装配（行为等价）。可回退：删配置即回退，无删除性改动。
- **M2 上行**：`WindowEvent` → server（通道选型见 §7 未决 2）。server 收口：Alert 生命周期复用 dbmon firing/resolved 语义（alertID=`probe:<target>`）；本地 `FileTimeline` 保持真值通道不动。可回退：server 不消费时 agent 照跑（本地窗口+日志语义与今天等价）。
- **M3 dbmon 下放收尾**：DB 源周期探针按归属 agent 下发（DBSource 归属改造若动表，按迁移契约走编号迁移三处同步）；server `POST /dbmon/probe` 保留为手动诊断。可回退：agent 探针停用后手动诊断面完整如初。
- **M4 面板**：探针卡/故障窗口 timeline 展示——**与 Agent 面板三页（agent-dashboard-design §2 探针卡）合流交付**，不另开页面。

### 3.3 迁移后 server 侧还剩什么

dispatch 熔断与打分（路由面）、dbmon sources 注册表 + 手动诊断探针、Alert 聚合与生命周期（`model.Alert`）、serverstatus 状态源、面板展示。server 不再执行任何周期性外部存活探测。

## 4. 「不让 Checker 自己决定恢复」与并发风险

- **结构性保证已存在**：`Checker` 契约单值无副作用（`healthprobe.go:32-34`）；`Probe` 是唯一状态机；恢复动作进程域在 supervisor 退出监视环（死亡事件驱动，不是 probe 窗口驱动）——探测环与恢复环天然分离，两个环之间唯一的既有耦合点是 `supervisable` 的退出码语义（0=不拉起/非 0=进退避），不是 Checker。
- **现状并发风险评估**：capture `OnWindow` 只打日志（`runner.go:648-653`）；devops 无探针；sidecar supervisor 每进程单监视 goroutine + 所有权守卫（`ops_server.go:463-476`）——**今天不存在并发恢复风险**。风险只在未来把恢复 hook 挂上 `OnWindow` 时出现。
- **结论**：外部目标域**不引入自动恢复**。agent 无权重启第三方服务；「恢复」= 告警上行 + 人工（devops agent 的堡垒机会话就是人工恢复的执行面）。因此并发恢复上限、恢复动作注册在 M1-M4 内**不实现**；若未来进程域之外出现真恢复需求，沿用 supervisor 先例（target 级单 worker + 世代守卫 + 权限双闸）扩一层即可，接口不必预埋。

## 5. 持久化 / 观测 / 配置统一

- **目标注册持久化**：agent 侧 config 文件（与 capture `options`、devops `allowlist` 同构）；中心下发走 `core/configsync` 版本化轮询通道（`configsync.go:1-10`，包注释明示「capture 白名单/阈值、devops 规则配置复用同一通道」）——不新增表。若 M3 需要 DBSource 归属 agent，才动表并走编号迁移。
- **观测**：窗口本地真值=既有 `FileTimeline`；上行后 server 侧落 `model.Alert`（firing/resolved 语义复用 dbmon 先例）；进程域已有 supervisor 事件面先例（事件环 500 + lumberjack 旋转文件，`ops_supervisor_events.go:35-48`）。恢复/故障事件不新造 metrics 面。
- **配置统一**：`internal/config/config.go:677-687` `healthCheck`（score/circuit 参数）**专属 dispatch 路由面，保持原名原位**；agent 探针配置用独立 `probe` 节（lowerCamelCase，入 agent config 契约）。禁止合并两套——合并会让路由打分参数误入探针语义。文档同步时写明边界。

## 6. 外部依赖判定（逐个）

| 外部点名                  | 判定                 | 依据                                                                                                                                                                                                                                                  |
| ------------------------- | -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| go-subprocess             | **不引**             | 本仓 `os/exec` + supervisor 恢复环（退避/熔断/成功判定/所有权守卫）经 S1/S2/S3 实战修复，且与 `supervisable` 信号纪律/退出码契约耦合；换库等于推倒该契约                                                                                              |
| kahi                      | **不引**             | 未检索到主流 Go 健康库以此命名；所描述能力与 `core/healthprobe` 重合                                                                                                                                                                                  |
| health-go                 | **不引**             | 该类库是「暴露 /healthz HTTP 端点」框架——方向相反：我们探别人，不是被探；core 系 agent 出站-only 无 HTTP listener                                                                                                                                     |
| blackbox_exporter         | **不引，只借鉴思路** | 独立服务非 Go 库，且 AGPL-3.0；引入等于搭一整套 Prometheus 采集面，违背「agent 基础能力统一」。其探针分类 http/tcp/icmp/dns 中 http/tcp 已有（`healthprobe.go:43-98`），icmp/dns 需要时以 stdlib（`net.Resolver`、unix raw socket build tag）补进本包 |
| 「core 包零外部依赖」约束 | **守住**             | DB 类 Checker 需要 database/sql + driver，放 agents 层新包（不进 core）；core 只收接口                                                                                                                                                                |

## 7. 风险与未决项（拍板点）

1. **probe.targets 归属哪个 agent type**：devops 是天然宿主（内网位置+堡垒机定位）；capture 已自带源探针；sidecar 管进程。建议：每 agent 均可配 `probe.targets`（能力同源），devops 为主使用方——待拍板。
2. **窗口事件上行通道**：`report.Conn` 扩 `SendAlertEvent`（wire 新消息）vs 复用 `MetricsReport` 扩展位——proto 变更面不同，待拍板（report.go 注释「告警通道落位时在本包扩展 SendAlert」已预留）。
3. **DBSource 归属改造**：是否给 `db_sources` 加 agent 归属列（动表 → 编号迁移三处同步）。
4. **窗口事件风暴**：target 多 + 抖动窗口时上行量——supervisor 事件环 500 + 去重先例可循；M2 设计时按 target 配 dedup 窗口。
5. **双真值时钟漂移**：窗口 StartTS 来自 agent 本地钟，server 记接收时间——告警详情双列并存，不做时钟对齐。

## 8. 推荐分期（拍板后）

- **R1 本评审定稿** → 用户拍板 §7 三点（归属、通道、动表与否）。
- **R2 agent 装配面**（M1+M2 的 agent 侧）：`probe.targets` 配置契约 + Checker 工厂（http/tcp/process 进 core；db 进 agents 层）+ 多 Probe 装配 + 上行接线 + capture 迁移到统一装配。代码批，全套门禁（含新用例）。
- **R3 server 消费面**（M2 server 侧 + M4）：窗口→Alert 生命周期 + 探针卡面板（与 Agent 面板三页合流）。
- **R4 dbmon 下放收尾**（M3）：周期探针 agent 化 + server 手动诊断保留 +（若拍板动表）编号迁移。
- **不排期**：外部目标自动恢复、恢复动作注册、全局并发恢复上限（§4 边界）。

## 已知边界

- 本评审只做审计与方案，未动任何代码；§2 缺口 5/6/7 明确不做或推迟。
- devops agent 当前零探针，M1 落地后其探针卡才有数据——与 Agent 面板三页「页面不渲染空页」口径一致，交付顺序上 M4 合流。
- Windows 平台 `ProcessChecker` 为退化实现（存在性检查，`process_windows.go:9-15` 注明），精确判活按需再引 x/sys/windows，不在本批。
