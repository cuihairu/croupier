---
title: Agent Supervisor 进程监管设计简档——字段/接口/分期（#67 立项）
---

# Agent Supervisor 进程监管设计简档

## 状态

- 状态: **Delivered（2026-10-10 S1/S2/S3 全部交付；S3 交付记录见文末）**。S1 落地对应 §3.3 采样、§3.5 上报捎带、§4 面板只读部分、§5 快照端点；S2 落地对应 §3.4 退避自动拉起与熔断（`BACKOFF/BROKEN` 态启用、`restartBackoffInitial/Max`/`restartBreakerLimit` 配置、成功判定=存活超退避封顶）、§3.6 事件日志双通道（agent 本地轮转文件全量 + metrics 上报捎带 server 内存环 500 条/agent、seq 增量去重）、面板操作列（start/stop/restart）与事件日志 Tab、事件拉取与日志下载端点（`GET /api/v1/ops/agents/:agentId/supervisor/events`、`GET /api/v1/ops/agents/:agentId/supervisor/logs`）；S3 落地轻档 `snapshotProfile` spawn 注入 + 中档 core_pattern 只读提示 + detect_down 产物登记。原 Proposed 简档（2026-10-08，OPEN-ISSUES #67 立项）全文保留如下，与实现漂移处以实现为准
- **S3 拍板记录（2026-10-10 用户授权代拍）**：①轻档 `snapshotProfile` 字段（`none|go|node|python|jvm` 显式声明）同意，随 S3 落地；②中档 core_pattern 宿主机操作=**agent 只读提示**——扫描宿主机现状+面板/日志显示建议命令供运维复制，agent 不直接写宿主配置（权限/安全边界）；③保留期/配额/下载权限仍待拍板（调研 §8.4）。
- S1 实现增量（简档未预见的两点）：快照覆盖「配置面 ∪ 实例面」（配置了但从未启动的进程以 STOPPED 出现在监管视图，ListProcesses 维持实例面口径不变）；`monitorProcess` 加实例归属守卫（修复 RestartProcess 后旧 monitor 把 RUNNING 翻成 FAILED 的预存瑕疵，S1 面板首次把 state 暴露给用户故必须修）
- 范围: agent 对托管进程的监管五要素——存活/时长/重启次数、资源采样与超限标记、崩溃自动拉起与熔断、面板列与进程详情、上报捎带；崩溃时内存快照的采集方案见 [崩溃快照与跨语言抓取调研](../research/crash-capture-survey-2026-10.md)（独立调研，分级方案与四项可拍板项已获用户批复「按调研分级方案」执行，S3 落地轻/中档）
- 关联: 现状代码 `internal/app/agent/ops_server.go`（managedProcess 雏形）· `internal/app/agent/ops_config.go` · `proto/croupier/ops/v1/ops.proto` · `internal/api/ops/`（server 代理）
- 关联更新（2026-10-09）：被监管对象一侧的运行时契约（信号纪律/退出码语义/心跳打点）已并入 [agent-core 简档](agent-core-design.md) §3「被监管对象契约」——监管协议仍以本简档 + ops.proto 为唯一来源，core 系 agent 出厂即合格被监管对象；agent 定名见 [Agent 清单](agents-inventory.md)（sidecar-agent / devops-agent / capture-agent，监管面（supervisor）归 sidecar-agent）

## 1. 现状盘点（代码事实）

agent 侧已有半套监管骨架，本设计是**演进**而非新建：

| 能力         | 现状                                                                                                                                                                 |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 托管进程注册 | `OpsConfig.ManagedProcesses map[name]ManagedProcessConfig`（command/args/workingDir/env/gracefulTimeout）——**配置显式声明**，agent 只管自己启动的进程                |
| 手动启停重启 | `OpsServer.StartProcess/StopProcess/RestartProcess`（RPC 触发，`Enabled+AllowRestart` 双开关门禁），`restarts` 计数与 `lastStart` 已存在                             |
| 退出检测     | `monitorProcess` goroutine 持有 `cmd.Wait()` 唯一属主权（waitDone 收尸协议）——**自动拉起的挂点就在这**                                                               |
| 状态模型     | `ProcessState` 枚举：UNSPECIFIED/RUNNING/STOPPED/FAILED/STARTING/STOPPING（ops.proto:162）——缺「退避中」「熔断」两态                                                 |
| 资源采样     | `MetricsCollector`（gopsutil v4 已是依赖）：agent 级 CPU/Memory/Disk/Network，`MetricsReport` 按interval 上报；**per-process 采样未接线**（`processes` 字段 7 空置） |
| server 代理  | `/ops/agents/:agentId/processes` 列表 + restart/stop/start 已通（routes.go:549-552）；metrics 内存历史 `MetricsStore.GetHistory` 先例                                |
| 面板         | Ops/Nodes 页有 NodeDetailDrawer/CronJobsDrawer，**无进程视图**                                                                                                       |
| 崩溃快照     | 无                                                                                                                                                                   |

## 2. 需求 → 缺口对照

| 用户需求                                                                                     | 现状缺口                                                       | 设计落点  |
| -------------------------------------------------------------------------------------------- | -------------------------------------------------------------- | --------- |
| ①存活检测/运行时长/重启次数                                                                  | pid/state/lastStart/restarts 已有；uptime 需派生；心跳时间戳缺 | §3.1/§3.2 |
| ②RSS/CPU% 采样、超阈值标异常                                                                 | gopsutil process 采样未接线；无阈值字段                        | §3.3      |
| ③挂→自动拉起（可配开关）或标红、连续失败熔断                                                 | 退出后无动作；无退避；无熔断                                   | §3.4      |
| ④agent 列表 supervisor 列（状态灯/内存/CPU）+详情                                            | server 有 API 面板无视图；详情页/事件日志缺                    | §4        |
| ⑤采样上报心跳捎带（/proc 或 go-psutil 类）                                                   | MetricsReport.processes 字段空置；gopsutil 已在依赖树          | §3.5      |
| 补充:全程事件日志（时间/事件/pid/退出码/次数，面板可查+文件可下载，带上下文与 OOM 疑似标记） | 全缺                                                           | §3.6      |

## 3. 设计

### 3.1 监管对象与配置增量（`OpsConfig.ManagedProcesses[x]`）

新增字段（yaml/json tag 按 lowerCamelCase 配置契约；OpsConfig 既有 snake_case 旧键不回溯，新键一律 canonical，examples 用新键）：

| 字段                    | 类型/默认       | 语义                                                                                                   |
| ----------------------- | --------------- | ------------------------------------------------------------------------------------------------------ |
| `autoRestart`           | bool / false    | 崩溃自动拉起开关——**默认关**，与 `AllowRestart` 安全姿态一致；开启还要求 `AllowRestart=true`（双门禁） |
| `restartBackoffInitial` | duration / 1s   | 退避序列起点（1s→2s→4s…）                                                                              |
| `restartBackoffMax`     | duration / 60s  | 退避封顶                                                                                               |
| `restartBreakerLimit`   | int / 5         | 连续失败熔断阈值；0=不熔断（不推荐）                                                                   |
| `memThresholdBytes`     | int64 / 0=不检  | RSS 超限标记阈值                                                                                       |
| `cpuThresholdPercent`   | double / 0=不检 | CPU% 超限标记阈值（按采样周期均值）                                                                    |

### 3.2 状态模型增量（`ProcessState` 枚举，proto 追加）

- `PROCESS_STATE_BACKOFF = 6`——已退出，退避等待自动拉起（附下次拉起时间）。
- `PROCESS_STATE_BROKEN = 7`——熔断：连续失败达 `restartBreakerLimit`，停止拉起，标红待人工。
- 存活判定沿用 monitorProcess 的 Wait 语义（进程句柄级，非轮询）；心跳时间戳 = 最近一次采样成功时间（§3.3），面板「最后上报」列即由此派生。
- 运行时长 = now − lastStart（派生值，不新增字段）；重启次数复用 `restarts`。

### 3.3 资源采样（gopsutil process，跨平台）

- 每 `MetricsInterval`（默认 30s）对 `state=RUNNING` 的托管进程采样：`process.Process.MemoryInfo().RSS`、`process.CPUPercent()`（gopsutil v4，Windows/Linux/macOS 同构，不直写 /proc）。
- 超阈值 → 实例标记 `flags[]`（`mem_over_limit` / `cpu_over_limit`）并落事件（§3.6），**不自动杀进程**（处置是人工决策，标记只负责暴露）。

### 3.4 自动拉起与熔断（monitorProcess 扩展）

```text
Wait 返回
  ├─ 主动 stop（stopCh 关闭/StopProcess RPC）→ 状态 STOPPED，链路结束
  └─ 非主动退出（崩溃）
       ├─ autoRestart=false → state=FAILED + 事件(exit_code/signal) → 等人工
       └─ autoRestart=true
            ├─ 连续失败 < limit → state=BACKOFF，退避 sleep 后重启
            │    └─ 重启成功 → backoff 归零、连续失败清零、事件(旧 pid/新 pid)
            └─ 连续失败 ≥ limit → state=BROKEN + 熔断事件(带原因) → 停止拉起
```

- 「成功」判定 = 新进程存活超过 `restartBackoffMax`（未被立即打回）——防止启动即崩的进程把退避序列刷穿。
- 手动 `StartProcess` 对 BROKEN 实例可用（人工修复后的复位路径），成功即清熔断计数。
- 退避 sleep 必须可被 `stopCh` 打断（停止监管时不留悬挂 goroutine）。

### 3.5 上报捎带（wire 增量，proto 追加不改旧字段）

`MetricsReport` 新增字段 9：

```proto
message SupervisedProcessSnapshot {
  string name = 1;            // ManagedProcesses 逻辑名
  int32 pid = 2;              // 0 = 未运行
  ProcessState state = 3;     // 含 BACKOFF/BROKEN 新态
  int64 uptime_seconds = 4;
  int32 restart_count = 5;
  int64 rss_bytes = 6;
  double cpu_percent = 7;
  repeated string flags = 8;  // mem_over_limit / cpu_over_limit / oom_suspect / breaker_tripped
  int64 last_event_unix = 9;  // 最近事件时间（驱动面板轮询增量）
}
repeated SupervisedProcessSnapshot supervised_processes = 9;
```

- 复用既有 `StartReporting` 通道与 server 端 `MetricsStore` 模式：server 收报后更新各 agent 的 supervisor 快照（内存态，与 metrics history 同生命周期策略）；Agent 离线时快照保留最后一次 + 面板标注「数据过期」。
- `ManagedProcess`（ListProcesses 响应）同步追加 uptime/flags/next_restart_at 派生字段，读取端一次拿全。

### 3.6 事件日志（双通道）

事件类型闭集：`detect_down` / `auto_restart` / `restart_failed` / `breaker_tripped` / `resource_over_limit` / `manual_start` / `manual_stop` / `schedule_fire`（§10.6） / `schedule_paused` / `schedule_resumed`（§10.2 暂停与恢复） / `update_applied` / `update_rolled_back`（§10.4）。

| 通道           | 载体与语义                                                                                                                                      |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| agent 本地文件 | supervisor 专属轮转日志（复用 lumberjack 形态：目录/ maxSize/ maxBackups/ maxAge 可配，键 `ops.supervisorLog.*`）——**全量真值**，agent 离线不丢 |
| 上报捎带       | 每次 MetricsReport 附最近事件（自 `last_event_unix` 增量），server 内存环保留最近 N=500 条/agent——面板列表用                                    |
| 下载           | `GET /ops/agents/:agentId/supervisor/logs` 经既有 ops 代理链回源 agent 拉文件（带大小上限），面板「下载日志」按钮直用                           |

事件结构：`{ts, agent_id, process, event, old_pid, new_pid, exit_code, signal, restart_count, message, context{last_heartbeat, last_error, oom_suspect, last_rss_bytes}}`。

- `oom_suspect` 判定（best-effort，如实标注启发式）：exit 为 SIGKILL 类 + 最近一次 RSS 采样占系统可用内存比例超 90%——不承诺精确，仅排查线索。
- `breaker_tripped` 事件必须带 `message`（连续失败原因链摘要），面板红标直接展示。

## 4. 面板（web）

- **agent 列表 supervisor 列**：状态灯聚合（全绿=全部 RUNNING；黄=有 BACKOFF/超限 flag；红=有 FAILED/BROKEN）+ 每进程 RSS/CPU 微缩文本；数据来自列表接口的 supervisor 快照聚合，零额外请求。
- **进程详情抽屉**（NodeDetailDrawer 内新 tab 或独立 SupervisorDrawer，与 CronJobsDrawer 同构）：
  - 进程卡：state 灯/pid/uptime/restarts/RSS/CPU/flags 徽标/BROKEN 红标+原因；
  - 事件日志列表：时间倒序、事件类型筛选、`oom_suspect` 等上下文字段展开；
  - 操作：手动 start/stop/restart（走既有代理端点，按 `ops:operate` 权限显隐）+「下载日志」。
- 轮询沿 Ops 页现状节奏（30s 或页面现有刷新间隔），不引入推送。

## 5. server API 增量（经既有 ops 代理链，权限沿用 ops 族）

| 端点                                                       | 语义                                            |
| ---------------------------------------------------------- | ----------------------------------------------- |
| `GET /ops/agents/:agentId/supervisor/events?since=&limit=` | 从内存环取事件（默认 100，最大 500）            |
| `GET /ops/agents/:agentId/supervisor/logs`                 | 代理回源 agent 拉轮转日志文件（流式，大小上限） |

- 列表/操作既有端点不变（ListProcesses 响应结构增量字段向后兼容）。
- 响应契约沿用平台规范（成功直返业务 JSON，错误 `{error,message,details}`）。

## 6. 安全与边界

- 默认全关：`autoRestart` 默认 false，自动拉起同时要求 `Enabled+AllowRestart`（任何一路关闭 → 只检测标记不拉起）。
- 熔断是防疯转的硬闸：退避封顶 + 连续失败上限双保险，无「无限重启」路径。
- 采样开销：gopsutil per-process 每 30s 一次 × 进程数，量级与现有 agent 级采样相同，不构成新负载点。
- 快照与事件不落 server DB（内存环 + agent 本地文件），server 重启丢最近事件环——诚实边界；若需要持久化再评估 DB 表（届时按迁移契约走编号迁移）。
- 崩溃内存快照**不在本期**：方案依赖 [crash-capture 调研](../research/crash-capture-survey-2026-10.md) 的分级建议与用户拍板（体积/敏感数据/生产开关），拍板后并入 P3。

## 7. proto / 配置键变更清单

- `proto/croupier/ops/v1/ops.proto`：`ProcessState` 加 6/7 两值；新增 `SupervisedProcessSnapshot`；`MetricsReport` 加 field 9；`ManagedProcess` 加 uptime/flags/next_restart_at（编号追加）。生成走 `make proto`（本机 protoc 34.x）。
- `OpsConfig`：§3.1 六键 + `ops.supervisorLog.*` 轮转键，tag 全部 lowerCamelCase；`configs/` 示例同步。
- 零 DB 表变更（事件不落库）；零迁移。

## 8. 分期实现计划

| 期  | 内容                                                                                                                                                                                                                                                                                               | 验收                                                                          |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| S1  | 只读监控：采样接线（§3.3）+ MetricsReport 捎带（§3.5）+ server 快照存储与 API + 面板列与详情抽屉只读部分（§4）——**2026-10-09 已交付**                                                                                                                                                              | 面板可见真实 RSS/CPU/状态；proto 生成与既有 wire 兼容（旧 server 收新报不炸） |
| S2  | 自动拉起与熔断（§3.4）+ 事件日志双通道与下载（§3.6）+ 详情页操作按钮                                                                                                                                                                                                                               | 崩溃→退避拉起→熔断全链有测试（注入假进程）；事件上下文完整；下载端点限流生效  |
| S3  | 崩溃内存快照（依 crash-capture 调研拍板结论，分级方案落地）——**2026-10-10 已交付**：`snapshotProfile` 闭集（none/go/node/python/jvm）+ spawn 注入 + `snapshotDir`（默认 `logs/snapshots`，0700）+ detect_down 产物登记 + core_pattern 只读提示事件 `snapshot_hint`；保留期/配额/下载权限未拍板未做 | 生产默认关、可按进程开启；快照产物落盘与面板事件可见；下载链随 §8.4 拍板      |

每期独立 commit，message 写清改动；门禁按交付完成定义（tsc/go test/guard + 涉及渲染走发布链）。

## 9. S3 交付记录（2026-10-10）

- **proto 增量**（wire 兼容，全旧字段不动）：`SupervisorEvent` +`snapshot_dir`/`snapshot_files`（detect_down 携带崩溃产物 best-effort）；`SupervisedProcessSnapshot` +`snapshot_profile`；事件闭集注释 +`snapshot_hint`。make proto 四路再生成（protoc 34.1/25.1）。
- **agent**（`internal/app/agent/`）：`ManagedProcessConfig.SnapshotProfile`（闭集校验进 `OpsConfig.Validate`，非法值加载即报错）+ `OpsConfig.SnapshotDir`（默认 `logs/snapshots`）；`ops_supervisor_snapshot.go` 新模块——`snapshotEnv`（go/python/jvm env 注入，档位优先于既有 env）、node 档 argv 前置（`NODE_OPTIONS` 不允许这两个 flag）、`ensureProcessSnapshotDir`（0700，失败不阻断启动）、`readCorePattern`（/proc 只读，包级变量可测）、`snapshotHintNeeded`/`snapshotHintEvent`（有进程开轻档且 core_pattern 非管道模式时发一次宿主机级提示，`sync.Once` 防刷屏）、`collectSnapshotArtifacts`（快照目录全量 + node 报告 WorkingDir 内 mtime 倒序 ≤5）；`startProcess` 接线注入；`superviseExit` detect_down 填产物。
- **server**（`internal/api/ops/`）：快照 DTO 透传 `snapshotProfile`/`snapshotDir`/`snapshotFiles`；顺手修 S2 预存缺陷——事件 DTO 补 `tsUnix`（web 读 `tsUnix` 而 server 只发格式化 `ts`，线上事件时间恒显示 "-"）。
- **web**：抽屉 flags 列加 `snapshot:<profile>` 标记；事件详情翼渲染快照目录与产物；`snapshot_hint` 事件落「快照提示」标签（紫）并进过滤闭集；zh/en locale。
- **竞态修复**（覆盖批次 -race 实测暴露的两处预存窗口，S3 新增写路径使其可复现故必修）：`StartProcess` 响应在锁外重读 `p.pid` → 改用锁内捕获的 `newPID`；`superviseExit` 入口锁外读 `p.stopCh` 与 `RestartProcess` 换 stopCh 竞态 → 判定收进 p.mu。
- **已知边界**：Go 档的 core 落盘位置取决于宿主机 core_pattern（中档前提）；node 报告落 WorkingDir（cwd）；产物下载/保留期/配额未拍板未做（调研 §8.4）；`JAVA_TOOL_OPTIONS` 有 "Picked up" stderr 副作用（注释明示）。
- **测试**：agent 包 S3 用例 10 个（闭集/env/argv/目录 0700/分类/提示闭集/产物登记/detect_down 集成/once/profile 采样透传）+ 全包 -race 绿；api/ops 映射用例 2 个；web Drawer 15 用例（含 snapshot 标记与产物渲染两例）；guard PASSED。

## 10. 生命周期统一模型（2026-10-14 三份外部评审反哺；未动码，采纳判定见各评审文件）

来源：[agent-lifecycle 评审](../review/chatgpt-2026-10-14-agent-lifecycle.md)、[probe-adapter 评审](../review/chatgpt-2026-10-14-probe-adapter.md)（均 2026-10-14，外部建议经本仓核验采纳）；与本简档 S1-S3 已交付部分的关系=**S1-S3 是本节的运行状态面与进程恢复环既有实现**，以下为统一模型的增量面。

### 10.1 统一受管对象：ManagedProcess（不引入外部新词）

外部评审提出的 `ManagedProgram` 与本仓 `OpsConfig.ManagedProcesses[x]`（§3.1）是同一概念——**沿用既有词 ManagedProcess**。统一模型=一个受管对象同时表示**游戏辅助程序**与**外部探测适配器**（新增 `kind: program | probe-adapter`），两类受管对象不再各建一套生命周期。

**已实现**（2026-10-15）：`ManagedProcessConfig` 增 `kind` 闭集字段（`program` / `probe-adapter`，缺省=program，存量配置零漂移）+ 适配器清单字段（`adapterId` / `adapterVersion` / `adapterProtocol` / `adapterCapabilities`，协议缺省 `croupier-probe-jsonl-v1`）；`Validate()` 拦未知 kind（对齐 snapshotProfile 闭集先例）与 `kind=probe-adapter` 缺身份；`kind` 与 `adapterId` 随 supervisor 快照上行走面板（`SupervisedProcessSnapshot.kind` / `adapter_id`，proto 追加 12/13，wire 兼容）。kind=probe-adapter 走与 program 完全同一套生命周期（开机自启/崩溃恢复/熔断/快照档/期望状态），无特判分支。

### 10.2 生命周期能力表（已采纳，未实现）

| 能力                     | 归属           | 说明                                                                                   |
| ------------------------ | -------------- | -------------------------------------------------------------------------------------- |
| 安装、启动、停止、重启   | 是             | 统一进程管理（S1/S2 已有监控+拉起）                                                    |
| 开机自启与异常退出恢复   | 是             | **区分程序崩溃与主动停止**（否则误启）；主动停止跨重启保留已落地（§10.3 管理状态首块） |
| 定时启动、停止、重启     | 是             | 统一调度器（§10.6）                                                                    |
| 更新二进制文件           | 是             | §10.4 五要求                                                                           |
| 更新配置文件             | 是             | 版本化、校验、原子发布（configsync 先例）                                              |
| 修改探测频率             | 是             | 属期望配置（与采样策略同源，简档 §13.3）                                               |
| 暂停与恢复               | 是             | **指明暂停对象：调度 / 程序 / 探测**；v1 已交付**调度暂停**（§10.6 下）                |
| 健康探测                 | 是             | 复用 `core/healthprobe`（本仓既有）                                                    |
| 异常恢复策略             | 是             | 重启、退避、熔断、人工介入（S2 已有）                                                  |
| 查看日志、版本、运行状态 | 是             | 统一上报 Server（S1 已有）                                                             |
| 任意远程 Shell 执行      | **不默认开放** | §10.7 硬边界                                                                           |

**暂停与恢复 v1 已实现（2026-10-15，暂停对象=调度）**：`POST /api/v1/ops/agents/:agentId/processes/:name/schedule/pause`（或 `.../resume`，两条路由同一处理器、`suspend` 由路由固定不走请求体，防路径与 body 语义分叉）→ wire `MsgSchedulePauseRequest/Response`（0x040125/26，追加号）→ agent `SetSchedulePause`。暂停的是**声明的 cron 触发**，进程运行态不动（不 stop、不影响恢复环、不改进程期望状态）。落盘 `logs/supervisor/schedule_pause_state.json`（`map[process]paused`，与期望状态 `desired_state.json` 同目录同原子写模式；**跨 agent 重启保留**——维护窗口意图不因 agent 重启自动解除），文件缺失/损坏 fail-open 空映射（缺记录=未暂停，宁可不拦误触发，不因状态文件坏了拒绝装配）。触发侧：`scheduleTick` 跳过暂停进程（`continue` 在槽去重之前，暂停期间**不写 `schedFired`**——恢复后下一匹配槽照常触发，暂停期间的槽不补跑，crond 语义）。门禁：与手动 start/stop/restart **同双闸**（`ops.enabled` + `allowRestart` 均须显式开启，无独立配置键）——它是远程运维动作；与 §10.6「定时触发本身不受 `allowRestart` 门禁」不矛盾（门禁的是远程开关，不是本地配置意图）。无调度条目的进程暂停为幂等空操作（意图落盘，快照不渲染字段）。快照上行 `SupervisedProcessSnapshot.schedule_suspended`（proto 追加 17），事件分型 `schedule_paused` / `schedule_resumed`（§3.6 闭集追加）。**边界诚实**：暂停**程序**与暂停**探测**两个对象未做——程序暂停语义上≈「手动停 + 记住要恢复」（本仓手动停已有期望状态落盘与恢复环抑制，增量价值弱）；探测暂停同理≈「维护窗口内抑制健康告警」，随告警策略面（§8/§13）一并做更合理，届时若对象语义清晰再单列 v2，不猜实现。

### 10.3 四类状态（已采纳；管理状态首块已实现，余两类随能力面落地）

**运行状态 / 管理状态 / 调度状态 / 更新状态** 四类必须分清——混同导致「更新中」被当成「已停止」误判。与本简档 §3.2 `ProcessState` 的关系：`ProcessState` 是**运行状态面**的既有枚举；管理/调度/更新三类是期望状态协调面的新增枚举（Reconciler 归属已核验=既有机制补全不新增组件，见 data-flow §6.2）。

**管理状态首块已实现**（受管程序生命周期第一块）：运维手动 stop/start 的**期望状态**落盘 supervisor 日志目录 `desired_state.json`（`internal/app/agent/ops_desired_state.go`）——缺记录视为期望 running（对齐既有开机自启语义，行为零漂移）；文件缺失/损坏 fail-open 空映射（宁可不拦误启，不因状态文件坏了拒绝起进程）；原子写（tmp+rename）。开机自启门（`Start()`）：AutoRestart=true 且期望无记录 → 照旧拉起；运维主动停过的进程（期望 stopped）跨 agent 重启保持停——§10.2「区分程序崩溃与主动停止（否则误启）」就此落地。手动 start/restart 落期望 running。写盘失败仅告警不回滚操作（期望是持久性提示，操作本身已成功）。

### 10.4 更新二进制：五要求 + 指纹硬要求（已采纳；v1 已实现 2026-10-15，边界见下）

五要求：可信来源分发；内容哈希与签名校验；运行中进程与文件占用处理（先停后换或原子目录切换）；数据库/配置兼容性检查；**新版本启动失败回滚**——只做原子替换一个二进制不等于更新安全。

**受管范围（用户 2026-10-14 追问）**：更新能力覆盖**游戏进程本体**（不只辅助程序与探测适配器）——`ManagedProcess` 本就是通用受管进程模型，游戏进程只是又一类 kind。但游戏进程更新有额外前置：玩家在线时换包是事故，必须满足 (a) 维护窗口或在线人数低于阈值（在线数据来自健康评审 §13.2 在线指标），(b) 暂停**调度与新连接导入**（§10.2「暂停要指明对象」——更新期间探针继续跑，否则失败不可见），(c) 指纹留痕+失败回滚。流量导入闸的具体形态依赖游戏侧配合，**未核实，随首个真实游戏接入定**。

**v1 落地面**（`internal/app/agent/ops_update.go`）：HTTP `POST /ops/agents/:agentId/processes/:name/update`（stagedPath + expectedSha256 必填，64 位小写 hex fail-closed）→ wire `MsgUpdateProcessRequest/Response`（0x040123/24）→ agent `UpdateProcess`。流程：哈希钉扎（staged sha256 ≠ 期望即拒绝，不碰运行态）→ 静默核心先停后换（coreStop/coreStart 不发 manual_* 中间事件）→ 旧二进制 rename 备份 `<target>.croupier-bak` → 安装（rename 优先，EXDEV 走同目录临时文件保原子）→ 拉起 → **稳定性窗口**（缺省 5s）：窗口内退出（waitDone 关闭）即回滚旧版并留 `update_rolled_back` 事件；窗口后退出属运行态走既有恢复环。备份**成功后保留**为 last-known-good（同目录滚动覆盖，人工回滚最后手段）。更新动作自己的启动=显式运行意图（desired=true，与手动 restart 一致）。

**指纹记录（硬要求，零新表）**：事件面 `update_applied` / `update_rolled_back`（message 带旧→新指纹）；持久查询主入口=server 侧 execlog 审计（source=`process_update`，每次尝试含客户端不可用都留痕，入参/响应载荷全文含指纹与 rolledBack）；快照 `SupervisedProcessSnapshot.fingerprint`（proto 追加 16）随 metrics 上行——运行实例指纹=spawn 时缓存的二进制内容 sha256（文件换指纹不变，直至重启换实例），面板下钻与告警 timeline 按 (target, 时间) 对齐。

**诚实边界（v1 未做）**：① 签名校验未实现——只做内容哈希（签名随密钥轮换会变，哈希是稳定可查锚点；来源可信性由分发通道保证）；② 数据库/配置兼容性检查未实现——属游戏侧知识，由调用方把关（配置版本经 ConfigVersion 关联）；③ 游戏进程维护窗口 / 在线人数阈值 / 流量导入闸未核实——随首个真实游戏接入定（见下）；④ 首次安装（无旧版可回滚）即失败时**无事件**（事件闭集仅 applied/rolled_back 两型），留痕在 execlog 审计。

**指纹是硬要求（用户 2026-10-14：「每次更新都需要记录指纹，方便查询 log」）**：

- **指纹算什么**：二进制内容的 **sha256 哈希**（hex）。签名校验结果单独记布尔（签名随密钥轮换会变，哈希才稳定可查）；配置不进指纹——配置已有版本化真值（ConfigVersion revision），作关联字段；版本字符串作展示关联。
- **记录字段**：目标标识（ManagedProcess id）、执行位置（agentID）、旧指纹 → 新指纹、时间戳、操作者/来源（server 下发=操作者用户 / 定时=调度器 / 回滚=recovery）、校验结果（hashMatch / signatureVerified / 兼容检查）、结果（success / failed / rolled_back，回滚记 rolledBackTo=旧指纹——**回滚路径同样留痕**）。
- **落在哪（复用两套既有面，零新表）**：更新是受管程序生命周期事件 → supervisor 事件闭集追加 `update_applied` / `update_rolled_back`（沿 detect_down 等事件先例，本地时间线+内存环，§3.6）；更新又是**受授权动作**（§10.7）→ 动作审计进 **execlog**（每个 agent 的每次执行都留审计记录，agents-inventory §5.2 既有口径），入参/结果全文含指纹——**持久查询主入口=execlog**。
- **查询口径**：按「目标 + 时间 + 指纹」检索 execlog；一条日志反查版本=execlog 详情自带指纹+版本串。与 metrics/告警衔接：`SupervisedProcessSnapshot`（§3.5）追加 fingerprint 字段随快照上行（proto 追加，wire 兼容先例 §7）——告警/探针 timeline 与更新记录按 (target, 时间) 对齐，即可定位「哪次更新后开始报警」；面板下钻两轴并排。

### 10.5 Agent 自身升级单独设计（已采纳；v0.1 草稿 2026-10-15，未实现——四个待拍板问题见下）

自升级失败=agent 失联，风险面与普通受管程序不同（没有上位监管者拉起它），不能套用 §10.4 流程——§10.4 的「先停后换 + 稳定性窗口回滚」假设换完后仍有角色观测新二进制并处置失败；自升级时连这个观测者都不存在。**关键不变量：自升级路径必须保留「不替换也能活」的退路——失败的最坏结果是 agent 按旧版重启，而不是砖化。**

**四个待拍板问题（实现前必须由用户拍板，本稿给建议）**：

1. **升级源**：(a) server 侧暂存中继（server 收包暂存 → 下发，与 §10.4 stagedPath 同形态，复用既有 wire 与审计面）；(b) agent 直取外部 URL（需 TLS+签名信任链）；(c) 配置版本钉扎（server 下发 targetVersion，agent 自取）。**建议 (a)**：升级面天然受授权面与审计面覆盖，不引入第二条信任链。
2. **触发模型**：(a) 手动 API（受授权双闸家族，与 allowUpdate 同族独立键控）；(b) ConfigVersion 配置热更（版本面扩字段下发）；(c) 复用 §10.6 cron 调度。**建议 (a) 起步**：自升级是最高风险动作，不应随配置默默发生；批量升级依赖 (a) 之上再叠批量编排。
3. **重启机制（核心难点）**：(a) 外部监管者重拉起（agent exit≠死，systemd/k8s 部署面保证）；(b) fork+exec 子进程接力（Unix 经典自更新：fork 子进程等待替换完成后 exec 新路径，父进程退出——无外部监管者也自洽）；(c) 双进程交替（影子进程复杂度高）。**建议 (b)**：部署面无监管者也能自洽；有监管者时 (b) 的父进程退出自然退化为 exit-restart，两种部署形态同一代码路径。
4. **失败姿态与回滚**：自升级失败=失联，回滚只能带外（监管者重启/人工登录）。设计约束：旧二进制**rename 备份不删**（§10.4 同款 last-known-good）；新版本起来后**存活过稳定性窗口**（对齐 §10.4 缺省 5s，自升级建议更长）才判定成功；存活期内挂掉=监管者按既有重启策略拉起（备份仍在 → 旧版回来）或人工介入。指纹留痕沿 §10.4 三处落面（事件闭集+execlog 审计 `source=agent_self_upgrade`+上报面）。

**指纹已先落（2026-10-15 交付，不依赖上述拍板）**：agent 二进制内容 sha256 随版本上报面上行——`SystemInfo.agent_binary_sha256`（proto 11，wire 兼容先例 §7），agent 侧进程生命周期缓存一次（运行实例指纹语义，同 §10.4：文件被换值不变，直至重启换实例）；server 侧 `GET /ops/agents/:agentId/system-info` 三级数据面（SystemInfoCache 最后已知 → 隧道 live RPC 回填 → registry 标签降级视图）全部携带 `agentBinarySha256`，缓存拷贝改 `proto.Clone` 根治手写字段清单漂移（新字段落地即踩中旧写法丢字段）。诚实边界：缓存是 server 内存态（server 重启即失）——**持久「最后已知指纹」锚点未做**，需 `agent_sessions` 表扩列（编号迁移，随自升级实现一并落）；失联期间锚点只有 execlog 按 (agentID, 时间) 检索 §10.4 更新记录。

**已知边界（草稿态）**：Windows 无 fork，(b) 形态需单独方案（子进程 spawn+等待）；多 agent 批量升级顺序（先备后主分批）；升级期间该 agent 全部受管程序的监管空窗（恢复环/探针随 agent 一起停，server 侧 metrics 停更告警应区分「升级中」与「失联」）。

### 10.6 定时任务与健康恢复：统一调度模型，执行逻辑不合并（已采纳；调度状态已实现 2026-10-15）

调度入口统一（一份 schedule 模型），但**定时触发与恢复环是两条执行路径**——与本仓既有结构事实一致：恢复环仅由进程退出事件驱动（`ops_server.go:426-439`），探测窗口不馈入重启（健康评审 §5.2 约束 2）。

**调度状态已实现**（2026-10-15）：`ManagedProcessConfig.schedule[]`（`action` 闭集 start/stop/restart + `cron` 五字段）——cron 解析与 server 侧任务调度同一 `internal/tasks/scheduler.ParseCron`（一份 schedule 模型），Validate 配置加载即拦非法动作/表达式。到点触发走显式动作**同一漏斗**（`doStart/doStop/doRestart`：desired-state 落盘与 supervisor 事件随流），scheduled stop 与手动停同信号（stopCh close），恢复环不把定时停误判为崩溃拉起（用例 `TestScheduledStopRecordsDesiredStateAndBlocksAutoRestart` 固化）。30s 分钟槽扫描 + (进程,动作,槽) 去重幂等；**无补跑**（agent 停机错过的槽位不补触发，crond 语义）。事件分型 `schedule_fire`（与 manual_* 分型，四类状态在事件流可分辨）；快照追加 `next_schedule_at_unix=14` / `next_schedule_action=15`（调度状态事前可见，无调度零值零漂移）。边界：schedule 是 agent 本地配置意图（与 autoRestart/开机自启同信任根），**不受 allowRestart 门禁**（allowRestart 门的是远程操作面）；实例不存在时 stop 幂等跳过、start/restart 同 StartProcess 路径物化。触发侧唯一运行时闸门是**调度暂停**（§10.2 暂停与恢复 v1）：暂停进程在槽去重前被跳过（不写 `schedFired`），恢复后下一匹配槽照常触发、暂停期间不补跑。

### 10.7 远程 Shell 不默认开放（硬边界）

更新二进制/配置 ≠ 允许任意远程命令执行；程序操作建模为**明确、受授权的动作**（对齐 §6 「默认全关+双闸」既有口径），不开放通用命令执行入口。独立授权、白名单、审计三件套前置。

**边界已物化**（2026-10-15）：既有 `allowExec` 远程命令面（agent `OpsServer.ExecuteCommand`，server HTTP `POST /ops/agents/:agentId/exec` + 本地 TCP 双入口）三件套核对与收口——

| 前置         | 物化前                                             | 物化后                                                                                                                                                   |
| ------------ | -------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 双闸（既有） | `ops.enabled + allowExec` 缺省 false 同开          | 不变（既有口径）                                                                                                                                         |
| 白名单       | `execAllowedCommands` **空=放行一切**（fail-open） | **fail-closed**：空列表一律拒绝；`OpsConfig.Validate()` 拦配置面，`ExecuteCommand` 运行期兜底程序内直构路径                                              |
| 审计         | 全链零留痕（logic 层无 execlog 写入）              | 每次尝试（成功 / 业务失败 / 客户端不可用）写 `execution_logs`，`source=agent_exec`，操作人+命令+参数+退出码入审计载荷；审计面缺失 fail-open 不阻断主路径 |

**已定边界**：面板仍不暴露通用 Shell 入口——程序操作一律走托管进程的显式动作（start/stop/restart）；`Validate()` 未在 agent 启动路径接线（配置加载未调用，属存量），故运行期 fail-closed 是实际强制点，配置面校验是契约声明。

### 10.8 配置模型唯一权威口径（三评审归并，不许三套并存）

- **运行面**：`ManagedProcess`（本简档 §3.1 既有，扩 kind）——管「跑起来、管住它」（schedule/recovery/state）。
- **探测面**：`probe.targets` + `probe.policy`（健康评审 §4.2/§13.3）——管「探什么、什么频率、谁的视角」。
- **适配器清单**：probe-adapter 评审的 `ProbeAdapter` manifest **不是第三套独立配置模型**——是 `kind=probe-adapter` 的 ManagedProcess 的附带描述（executable/protocol/capabilities），被 `probe.targets` 引用；游戏地址/频率/授权/凭证引用留在探测面配置，**不硬编码进清单**（评审原文约束）。**落地方式（2026-10-15）**：清单字段内嵌在 `ManagedProcessConfig`（`adapterId`/`adapterVersion`/`adapterProtocol`/`adapterCapabilities`）而非独立文件——`executable` 复用既有 `command`/`args`，避免为同一进程维护两份入口声明；待 `probe.targets` 引用面（依赖 IPC 契约）落地时再评估是否独立成文件。
- 契约要点（`croupier-probe-jsonl-v1`，已采纳**未核实**——字段规范评审自述待定）：独立进程+版本化 IPC；**进程常驻复用，不得每次探测新建进程**（启动成本会主导探测周期）；外部适配器只探测不控制平台（能力归属表：控制权留 Agent/Server）；先做一个参考适配器+契约测试再谈分发/签名。
