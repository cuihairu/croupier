---
title: Agent Supervisor 进程监管设计简档——字段/接口/分期（#67 立项）
---

# Agent Supervisor 进程监管设计简档

## 状态

- 状态: Partial（2026-10-09 S1/S2 已交付；S3 未动）。S1 落地对应 §3.3 采样、§3.5 上报捎带、§4 面板只读部分、§5 快照端点；S2 落地对应 §3.4 退避自动拉起与熔断（`BACKOFF/BROKEN` 态启用、`restartBackoffInitial/Max`/`restartBreakerLimit` 配置、成功判定=存活超退避封顶）、§3.6 事件日志双通道（agent 本地轮转文件全量 + metrics 上报捎带 server 内存环 500 条/agent、seq 增量去重）、面板操作列（start/stop/restart）与事件日志 Tab、事件拉取与日志下载端点（`GET /api/v1/ops/agents/:agentId/supervisor/events`、`GET /api/v1/ops/agents/:agentId/supervisor/logs`）。原 Proposed 简档（2026-10-08，OPEN-ISSUES #67 立项）全文保留如下，与实现漂移处以实现为准
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

事件类型闭集：`detect_down` / `auto_restart` / `restart_failed` / `breaker_tripped` / `resource_over_limit` / `manual_start` / `manual_stop`。

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

| 期  | 内容                                                                                                                                                                   | 验收                                                                          |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| S1  | 只读监控：采样接线（§3.3）+ MetricsReport 捎带（§3.5）+ server 快照存储与 API + 面板列与详情抽屉只读部分（§4）——**2026-10-09 已交付**                                  | 面板可见真实 RSS/CPU/状态；proto 生成与既有 wire 兼容（旧 server 收新报不炸） |
| S2  | 自动拉起与熔断（§3.4）+ 事件日志双通道与下载（§3.6）+ 详情页操作按钮                                                                                                   | 崩溃→退避拉起→熔断全链有测试（注入假进程）；事件上下文完整；下载端点限流生效  |
| S3  | 崩溃内存快照（依 crash-capture 调研拍板结论，分级方案落地）——**拍板已清（2026-10-10）**：`snapshotProfile` 字段准入随本批；core_pattern=agent 只读提示（不写宿主配置） | 快照产物落盘与面板下载链通；生产默认关、可按进程开启                          |

每期独立 commit，message 写清改动；门禁按交付完成定义（tsc/go test/guard + 涉及渲染走发布链）。
