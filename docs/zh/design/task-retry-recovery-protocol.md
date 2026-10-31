---
title: 异步任务重试与终态恢复协议
---

# 异步任务重试与终态恢复协议（ARCH-014）

> 2026-10-31 实现复核：在途重复 Start（D1）、终态有限重试（D2）与会话丢失/`unknown` 收割（D3）均已实现。2026-10-30 重新打开 ARCH-015 的「行状态与事件持久化不原子」缺口已闭合：Server 以单一事务（`RecordTaskEvent`）持久化 guard 状态更新与事件追加，事件写失败整体回滚状态更新，来源恢复后重试收敛该行，而非缺失事件仍被 ACK。watchdog（`timed_out` 收割器）ARCH-018 已随附；独立进程验收仍 NOT RUN。

**状态：** 已决策（2026-10-30）。实现由 ARCH-015（TaskRunner 重复 Start 约束与事件重放/终态收敛）与 ARCH-018（执行中断演练）承接。

本简档定义异步 Agent 任务的协议语义——现状代码仅「碰巧」如此，并非有意实现。它不改动 wire 契约：复用 `task_runs` / `task_events`，不新增表。

## 问题

围绕在途 Agent 任务的三处缺口会让 task_runs 行的状态与游戏真实结果矛盾，或让同一任务 ID 的两个执行交错：

1. **重复 Start 无约束。** `TaskRunner.Start`（agents/sidecar/task_runner.go）无条件写 `taskID → cancel` 映射项。对已在途 ID 的第二次 Start 覆盖旧项，先结束协程的 deferred `delete` 会删掉第二次执行的跟踪。更糟的是 `Cancel` 在执行器尚未退出时即急删映射项，其后同 ID 的 Start 插入新项，又被旧协程清理删除（即「旧清理不得删除同 ID 新任务」风险）。后果：任务不可取消、计数失真，最坏同 ID 两个执行并发。
2. **事件 fire-and-forget。** `run` 经 `ReportTaskEvent` 上报 started/completed/failed/cancelled 并忽略返回错误（`_ = r.report(...)`）。传输失败静默丢失终态事件，行永远停在 `running`。
3. **Agent 会话丢失让行搁浅。** Agent 会话被移除（internal/server/agent_session.go）时无人清理该 agent 的非终态 task_runs；行可无限停在 `running` / `cancel_requested`，而游戏侧效果可能已发生或未发生。

共同陷阱是**把页面执行幂等当成游戏副作用去重**。页面幂等记录只证明控制侧页面执行过一次，不证明游戏自身函数调用完成。任务恢复因此必须把「结果未知」作为独立、可运维解析的状态浮出，且不得静默重跑。

## 决策

### D1 — 同 taskId 重复 Start：在途时幂等 no-op

- 对本 agent **已在途** 的任务 ID 的 Start：确认（返回既有任务 ID），不启动第二次执行，不重复上报 `started`。同一任务 ID 永不产生交错事件流。
- 任务跟踪改为**代际标记**：映射存 `taskID → {generation, cancel}`。`run` 的 deferred 清理**仅在存储代际仍等于自身**时删项，因此旧协程收尾永不删除同 ID 新任务的项。
- `Cancel` **不再急删映射项**；项存活到执行器真正返回（清理是唯一删除点，且带代际校验）。取消收尾期到达的 Start 视为在途 no-op——正确，因为旧执行未退出时的第二次执行会重复施加副作用。
- 理由：Server 是任务 ID 的唯一合法来源（派发先持久化 task_runs——ARCH-026——再发 StartTask）。重复 Start 要么是传输重投、要么是运维强制重派；两种情况「旧执行跑一次、确认重复」是副作用安全答案。重跑任务用**新任务 ID** 表达（运维重执行创建新行），绝不同 ID 重启。
- 跨重启重复不在 agent 内存作用域内：重启清空映射，重启后重投的 Start 会执行。这是如实浮出的限制，不是游戏侧去重的声明。

### D2 — 事件投递：Server 响应即 ACK；终态事件带重试

- **Server 的 `handleTaskEvent` 响应即 ACK。** Server 响应前经 `RecordTaskEvent` 持久化——guard 状态更新与事件追加跑在**单一事务**内，事件写失败整体回滚状态更新，任何部分 `task_runs`/`task_events` 状态都不会被 ACK（R30-03 / ARCH-015）。终态守卫丢弃（终态后的迟到事件）同样成功返回且不落记录——是刻意的 no-op，agent **不得**重试。此拆分已存在，本设计将其钉为契约。
- **终态事件（completed/failed/cancelled）带重试直至 ACK**，有界（有界次数 + 有界内存），带退避。任务 `run` 协程阻塞在终态事件发送上，因此上游视角下任务直到 Server 持久化（或刻意丢弃）终态事件才算结束。
- **非终态事件（started/progress/log）尽力而为**：传输压力下 progress 可合并、log 可丢弃。丢失不影响收敛——终态事件仍会落地（或 D3 收割器给行定界）。
- 同一任务 ID 内事件由单个 `run` 协程顺序上报；Server 终态守卫保证 task_events 权威（终态后无事件）。
- **明确非目标：** 本迭代不做 agent 侧持久事件日志与跨重启重放。跨重启恢复是 D3 的未知态收敛，不是重放到终态。

### D3 — Agent 会话丢失 / 静默运行 → `unknown`；不自动重试

- 新增任务状态 **`unknown`**（既有 `task_runs.status` 的新字符串值；无 schema 变更）。含义：_执行可能已完成、失败或从未开始；游戏侧结果未确立。_
- **触发：** Agent 会话被移除（注销 / 心跳超时 / TCP 关闭）时，Server 将该 agent 的非终态 task_runs 批量转为 `unknown`（有界批量，消息「agent 会话丢失；执行状态未知」）。
- **软终态：** `unknown` 停止派发侧活动集跟踪与 SSE 等待者，但**同 agent 重连会话的迟到终态事件仍可将行收敛到真实终态**（agent 可能扛过传输抖动；迟到事件是更多证据）。这与 `TerminalStatuses`（succeeded/failed/cancelled/timed_out，迟到事件丢弃）相区分。
- **不自动重派 `unknown` 任务。** 重跑可能重复施加游戏副作用；重执行是运维决策（新任务 ID）。`unknown` 在 UI 中作为显式状态浮出，而非失败。
- **看门狗兜底：** 与会话丢失无关，收割器把超过截止时间（`started_at`/`updated_at` 超出调用预算）仍无事件的任务转为 `timed_out`。`timed_out` 是终态且含义为*无进一步证据*，非*游戏失败*。（ARCH-018 已随附；此处钉住语义。）

## 边界：页面幂等 ≠ 游戏副作用去重

页面执行幂等记录去重的是**控制侧页面执行**。它**不是**游戏函数恰好执行一次的凭据。任务恢复因此：

- 绝不引用页面幂等记录作为自动重跑任务的依据；
- 把 `unknown` / `timed_out` 作为运维可解析状态浮出——重跑前须经游戏侧证据（任务状态查询、游戏侧回执或人工核查）确立游戏结果；
- 任何重跑用**新任务 ID**，使每次重跑都是独立 task_runs 行与独立事件流。

## 实现映射（供 ARCH-015 / ARCH-018）

| 协议点                         | 代码接缝                                                                                                                                                                                                          |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1 代际标记跟踪、不急删 cancel | `agents/sidecar/task_runner.go`（`Start`、`Cancel`、`run` 清理、映射类型）                                                                                                                                        |
| D2 终态事件重试至 ACK          | `agents/sidecar/task_runner.go`（`run` 终态上报）；agent 上报器（`internal/app/agent/upstream.go`）；server `TaskStore.RecordTaskEvent` + `TaskRunModel.RecordTaskEventTransactional`（单一事务持久化，ARCH-015） |
| D3 会话丢失 → `unknown` 收割   | server agent 会话移除路径；任务状态谓词（`internal/tasks/types.go`）                                                                                                                                              |
| D3 看门狗 → `timed_out`        | server ControlService 后台循环（`runTaskWatchdog`）；`TaskRunModel.ReapStale`；`tasks.Store.ReapStale`；预算 `taskLog.reaperTimeout`（默认 5m）——ARCH-018 已随附                                                  |
| 边界：重跑 = 新任务 ID         | 派发 / 任务创建路径（已是现状；保持）                                                                                                                                                                             |

## 已知边界

- 跨重启重复执行未防（D1）；agent 无持久本地日志（D2）。两者都是如实浮出的限制，不隐藏。
- `unknown` 是新状态值；UI/类型面与任务查询已渲染之（ARCH-015 实现：TaskRunner D1/D2、server 会话丢失收割 D3、web `unknown` 状态）。
- D3 看门狗 `timed_out` 收割器 **ARCH-018 已随附**；无事件超截止行自动收敛 `timed_out`（预算 `taskLog.reaperTimeout`，默认 5m）。
- 协议不确立游戏侧 exactly-once 语义；需游戏侧回执/载荷内幂等键（GAME-010 相邻，此处越界）。

## 实现状态（ARCH-015，2026-10-31）

- **行状态与事件持久化原子化已随附（R30-03）。** `handleTaskEvent` 不再走 `UpdateRunIfStatusNotIn` + `AppendEvent` 两段式；改为调用 `tasks.Store.RecordTaskEvent`，委托 `TaskRunModel.RecordTaskEventTransactional`：单一数据库事务内先执行带 guard 的 `task_runs` 更新，再计算 `task_events` 下一 seq 并追加事件。任一失败整体回滚——行不可能在事件行缺失时变成终态，来源恢复后重投的终态事件可收敛该行（真正迟到的终态后事件仍被 guard 拦截）。证据：`internal/server/control_handler_test.go` `TestControlService_HandleTaskEvent_RealStore_NoPartialWrite`（事件表被删 → 报错且行保持 `queued`；表恢复 → 重试补写 seq=1 并收敛 `failed`）、`internal/model/task_model_test.go` `TestTaskRunModel_RecordTaskEventTransactional` / `_RollsBackOnAppendFailure`。
- 边界不变：guard 丢弃契约（终态后迟到事件 → ACK 且不落记录）保留；跨重启重复执行与游戏侧 exactly-once 仍越界（GAME-010）。

## 实现状态（ARCH-018，2026-10-30）

- **看门狗 `timed_out` 收割器已随附。** `ControlService.runTaskWatchdog` 跑在后台 ticker（`budget/2` 钳位 1–30s），经 `tasks.Store.ReapStale` 调 `TaskRunModel.ReapStale`：`UPDATE task_runs SET status='timed_out', message='no event past watchdog budget; execution state unknown', finished_at=now WHERE status IN (queued,dispatching,running,cancel_requested) AND updated_at < now - budget`。预算为 `taskLog.reaperTimeout`（duration 字符串，默认 5m）；`timed_out` 是终态、不再被选。
- **执行中断故障钻机**（`cmd/server/fault_drill_r27_test.go`，真实 `runServer` boot + `e2e-agent-probe` 子进程）逐项记录：断链（杀探针）→ running 收敛 `unknown` + 新请求 503；drain → 新请求 503 且在途任务完成，undrain 恢复派发；看门狗（`-hang` 探针：静默会话、无断连收割）→ 预算内 `timed_out` + `finished_at` 置位，新请求仍接受；SIGTERM → 优雅停机返回 nil、在途行 `unknown`、HTTP 监听释放。
- **边界诚实：** 探针的任务/副作用计数是**测试 fixture，非游戏恢复凭据**。钻机只断言平台收敛（状态机终态/未知态/拒绝语义与资源释放），不断言游戏侧 exactly-once（GAME-010 仍 NOT RUN）。
