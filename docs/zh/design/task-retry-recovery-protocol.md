# 异步任务重试与终态恢复协议

> ARCH-014 · P1 · 方案决策 · 异步任务重试与终态恢复协议
>
> 范围：sidecar 在途任务生命周期（`agents/sidecar/task_runner.go`
> `TaskRunner`）、agent 上行事件传输（`internal/app/agent/upstream.go`
> `ReportTaskEvent`）与服务端任务持久化（`internal/server/control_handler.go`
> `handleTaskEvent`、`internal/tasks` `UpdateRunIfStatusNotIn`/`AppendEvent`、
> `internal/model/task.go` `task_runs`/`task_events`）。定时任务触发路径
> （`internal/tasks/scheduler`）与派发前落库失败（ARCH-026）不在本协议范围。
>
> 状态：**DECIDED**（2026-10-30）。D1–D5 已定：同 taskId 重复 Start 为幂等
> no-op（不启动第二次执行）、事件投递为落库后 ACK 且有界重试、Agent 重启
> 产出显式 **unknown** 结果态（绝不静默、非幂等函数绝不自动重试）、终态单调
> （迟到事件不可回退）、平台幂等显式不等同游戏副作用去重。驱动 ARCH-015
> （收口 TaskRunner 重复 Start 与事件重放/终态收敛）、ARCH-018（执行中断与
> 停机演练）、ARCH-026。本文档不建立生产就绪。
> 前置：ARCH-015、ARCH-018。

## 1. 问题

异步任务路径在故障后目前无法回答三个问题，且有一个概念混淆未被点明。证据在
当前代码中。

### 1.1 同 taskId 重复 Start 破坏在途表

`TaskRunner.Start`（`agents/sidecar/task_runner.go:67`）无条件写
`r.tasks[taskID] = cancel`。同一 `taskId` 两次并发 `Start` 因此会：

- **覆盖**：第二次注册替换第一次任务的 `cancel` 函数，第一次任务不可取消。
- **完成即删**：每个 goroutine 的 `defer`（`task_runner.go:108`）执行
  `delete(r.tasks, taskID)`。_第一个_ goroutine 结束时删掉的是现在属于
  *第二个*任务的表项，使第二个任务不可取消并泄漏其槽位（它自己的完成
  `delete` 变成 no-op）。
- **竞写同一行**：两个任务为同一 `task_runs` 行上报事件，一个任务的终态事件
  会经 `UpdateRunIfStatusNotIn`（`control_handler.go:969`）挡住另一个任务的
  `started`。

没有任何任务身份守卫：无法区分合法新任务与同一 `taskId` 的重复派发。

### 1.2 事件投递是 fire-and-forget

`TaskRunner.run` 上报事件时丢弃错误（`_ = r.report(...)`，
`task_runner.go:114/130/140`）。发送失败（agent→server 传输断开）静默丢事件。
最坏情况：任务**已完成**，`completed` 事件发送失败，`task_runs` 行永远停在
`running`——已完成任务带永久未知结果且无恢复。

### 1.3 Agent 重启令运行中任务卡死

重启后内存 `tasks` 表丢失。留在 `running` 的 `task_runs` 行永远收不到终态
事件，卡死且无法知道游戏副作用是否执行。无任何对账。

### 1.4 页面幂等不是游戏副作用去重

UI/API 的 `idempotency-key` 只在平台层去重任务*记录*创建。它不说明游戏函数
副作用是否执行过一次——游戏已应用效果但平台尚未记录完成时崩溃，与「从未
执行」在没有游戏侧键的情况下不可区分。

## 2. 决策

### D1 · 同 taskId 重复 Start 为幂等 no-op

- **任务身份**：`taskId` 是平台级任务身份。服务端生成它并在**派发前**创建
  `task_runs` 行（`task_id` 唯一索引，`internal/model/task.go:10`）。一个
  `taskId` ⇒ 一行 `task_runs` ⇒ 至多一次预期执行。
- **重复 Start**（对已在运行的 `taskId` 再次 `Start`）：**不启动第二个
  goroutine、不覆盖 `cancel` 注册。** 调用返回既有注册（幂等 no-op）并记
  warning。重复是派发/重试产物（或重注册竞争），不是新任务。
- **所有权守卫**（供 ARCH-015）：每个在途表项恰好由一个 goroutine 拥有。
  结束的 goroutine 只删除*自己的*表项，绝不删除同名新任务的——用每 taskId
  代数/引用校验实现，使旧 goroutine 清理不能删掉同名新任务的槽位（修复
  §1.1 的完成即删）。

### D2 · 事件投递为落库后 ACK 且有界重试

- **持久化点**：服务端仅在事件落库（`handleTaskEvent` → 状态更新 +
  `AppendEvent` 到 `task_events`）后才 ACK。sidecar 视服务端 ACK 成功为已投递。
- **发送失败**：sidecar 不得静默丢弃（§1.2）。有界退避重试；窗口内无法投递则
  转恢复路径（D3）并上报失败。已完成任务的完成事件绝不处于「未投递且被忽略」。
- **重放/排序**：事件带每 taskId 单调 `seq`（`task_events` `(task_id, seq)`
  索引）。已投递事件 `seq` ≤ 最后持久化 `seq` 则幂等跳过；按 `seq` 顺序应用。

### D3 · Agent 重启产出显式 "unknown" 结果，绝不静默

- Agent（重）启动后在途表丢失；平台无法知道 `running` 任务的游戏副作用是否
  执行。**平台不静默重试、也不把行留在卡死态。**
- **恢复**：agent 重注册时，服务端把仍 `running` 的 `task_runs` 对账为一级
  **`unknown`** 态（独立、终态邻近的状态，意为「平台无法判定游戏副作用是否
  执行」）并追加恢复事件。这是暴露给人工对账，不是隐藏。
- **非幂等函数不自动重试**：自动重派可能重复执行一个可能已生效的游戏副作用。
  仅对游戏显式声明可安全重跑的函数允许自动重试（D5）。

### D4 · 终态单调；取消不回退终态

- 终态（`succeeded`/`failed`/`cancelled`/`timed_out`/`unknown`）不可再变。迟到
  或重复事件（重启后旧 goroutine、重放）不可回退——现有
  `UpdateRunIfStatusNotIn` 守卫扩展到含 `unknown`。
- **取消是请求，不是保证**：`cancel_requested` 不是终态。已执行副作用的任务
  在取消请求后仍可上报 `completed`（副作用在取消生效前已应用）；服务端接受
  该 `completed` 且不回退成 `cancelled`。「取消不回退终态」：取消绝不改写
  已终态的行。

### D5 · 平台幂等不等同游戏副作用去重

- 平台 `idempotency-key` 在 API/页面层去重 **task_runs 记录创建**（每键至多一
  行）。它**不是**游戏副作用执行一次的保证。
- 平台绝不把自己的幂等键注入游戏函数自身的去重机制，也绝不从它推导游戏语义。
  必须防重复应用的函数由游戏实现**自己的**幂等（游戏自定义唯一键/订单号）；
  平台透传调用方键但不执行游戏去重。
- 恢复因此必须保守：仅对游戏声明为幂等的函数自动重试；否则暴露 `unknown` 供
  人工解决（D3）。

## 3. 评审表

| #   | 场景（当前代码）                           | 采用 D1–D5 后                                                                  |
| --- | ------------------------------------------ | ------------------------------------------------------------------------------ |
| 1   | 同一 `taskId` 两次 `Start`                 | 第二次为幂等 no-op；无第二 goroutine；cancel 注册保留（D1）                    |
| 2   | 第一个 goroutine 结束而同名任务仍在运行    | 旧清理只删自己的表项；新任务保持可取消（D1）                                   |
| 3   | `completed` 事件发送失败                   | 有界重试；仍无法投递则转恢复路径（D3）；行不静默卡死（D2）                     |
| 4   | 任务中途 Agent 重启                        | `task_runs` `running` → 重注册时 `unknown`，追加恢复事件，暴露供人工复核（D3） |
| 5   | 终态事件后迟到/重复事件                    | 按 `seq` 幂等跳过；终态不回退（D2/D4）                                         |
| 6   | 对副作用已生效的任务发取消请求             | 接受 `completed`；不回退 `cancelled`；`cancel_requested` 非终态（D4）          |
| 7   | 同一 `idempotency-key` 提交两次（UI 双击） | 至多一行 `task_runs`；除非游戏幂等否则副作用可能双应用（D5）                   |

## 4. 边界诚实

- **unknown 是诚实，不是谎言**：agent 崩溃后平台确实无法判定游戏副作用是否
  执行。`unknown` 如实陈述；它不是隐藏重试或伪造结果。
- **每 agent，非跨 agent**：`taskId` 属于收到它的 agent；任务无跨 agent 对账，
  也无跨 agent 交接的 at-least-once/exactly-once 保证（那需要游戏级幂等，D5）。
- **服务端持久化是耐久下限**：服务端 DB 宕机时任务事件无法落库/ACK；该路径与
  派发前落库失败为 ARCH-026，不在本协议。
- **重试/重派有界且人工**：协议定义任务*如何*恢复（unknown + 暴露），不定义
  *自动*重派。非幂等函数的重派是产品决策（游戏声明幂等，D5）。
- 不声称生产就绪：协议是 ARCH-015 实现、ARCH-018 故障演练的契约；演练跑
  fixture 而非真实游戏（真实游戏验证为 GAME-010）。

## 5. 实现状态（ARCH-015）

ARCH-015 已实现 D1、D2（sidecar）与 D4（服务端）：

- **D1**：`TaskRunner.Start` 对已在运行的 `taskId` 为幂等 no-op（不启第二
  goroutine、不覆盖 cancel 注册）；在途表项携带每 `taskId` 代数
  （`taskSlot.gen`），结束的 goroutine 只删除自己的表项，旧协程清理不能删除
  同名新任务的槽位。
- **D2**：`TaskRunner` 以有界指数退避投递生命周期事件
  （`SetEventRetryPolicy`）；窗口耗尽以 warning 暴露，绝不静默丢弃。ACK 仍在
  服务端落库之后。
- **D4**：`StatusUnknown` 为 `internal/tasks/types.go` 的终态
  （`IsTerminalStatus`/`TerminalStatuses`），`unknown` 行由此获得与其它终态
  相同的 `UpdateRunIfStatusNotIn` 迟到事件守卫。`cancel_requested` 保持非终
  态：待取消行上的 `completed` 事件不被拦（「取消不回退终态」）。

**D3 的触发已落地（ARCH-018，2026-10-30）**，无需 wire 改动：agent 进程启动
随机生成一次 `bootId`，经既有 `RegisterRequest.labels` map 上报
（`internal/common/labels.BootID`）。server 在重注册时比较上次会话与本次的
`bootId`：同值 = 断线重连（不对账存活任务）；不同值且两侧非空 = 进程重启
（把该 agent 在途 `dispatching`/`running` 的 `task_runs` 对账为 `unknown` 并
追加 `recovery` 事件）；任一侧为空 = 未声明重启身份（不对账，诚实边界——
绝不误标重连 agent 的存活任务）。

故障演练证据：`docs/review/arch018-fault-drill-2026-10-30.md`（断链+重启 →
`unknown` 走真实 TCP 帧、重连保持 `running`、终态守卫、drain/SIGTERM 结果、
仅 fixture 的边界）。
