# Async Task Retry & Terminal-State Recovery Protocol

> ARCH-014 · P1 · 方案决策 · 异步任务重试与终态恢复协议
>
> Scope: the sidecar in-flight task lifecycle (`agents/sidecar/task_runner.go`
> `TaskRunner`), the agent upstream event transport
> (`internal/app/agent/upstream.go` `ReportTaskEvent`), and the server-side task
> persistence (`internal/server/control_handler.go` `handleTaskEvent`,
> `internal/tasks` `UpdateRunIfStatusNotIn`/`AppendEvent`, `internal/model/task.go`
> `task_runs`/`task_events`). The scheduled-task trigger path
> (`internal/tasks/scheduler`) and the pre-dispatch persist failure (ARCH-026)
> are out of scope here.
>
> Status: **DECIDED** (2026-10-30). D1–D5 settled: same-taskId duplicate Start is
> an idempotent no-op (not a second execution), event delivery is ACK-after-persist
> with bounded retry, agent restart yields an explicit **unknown** result state
> (never silent, never auto-retried for non-idempotent functions), terminal state
> is monotonic (late events cannot regress it), and platform idempotency is
> explicitly not game side-effect deduplication. Drives ARCH-015 (收口 TaskRunner
> 重复 Start 与事件重放/终态收敛), ARCH-018 (执行中断与停机演练), ARCH-026.
> This document does not establish production readiness.
> Prerequisite for: ARCH-015, ARCH-018.

## 1. Problem

The async task path currently cannot answer three questions after a fault, and
one conceptual confusion is unstated. Evidence is in the current code.

### 1.1 Same taskId repeated Start corrupts the in-flight map

`TaskRunner.Start` (`agents/sidecar/task_runner.go:67`) writes
`r.tasks[taskID] = cancel` unconditionally. Two concurrent `Start` calls with the
same `taskId` therefore:

- **Overwrite**: the second registration replaces the first task's `cancel`
  function in the map, so the first task becomes uncancellable.
- **Delete-on-finish**: each goroutine's `defer` (`task_runner.go:108`) runs
  `delete(r.tasks, taskID)`. When the _first_ goroutine finishes it deletes the
  map entry that now belongs to the _second_ task, making the second task
  uncancellable and leaking its slot (its own finish `delete` is then a no-op).
- **Races the same row**: both tasks report events for the same `task_runs`
  row, so a terminal event from one can block the other's `started` via
  `UpdateRunIfStatusNotIn` (`control_handler.go:969`).

There is no task-identity guard: nothing distinguishes a legitimate new task
from a duplicate dispatch of the same `taskId`.

### 1.2 Event delivery is fire-and-forget

`TaskRunner.run` reports events with the error discarded
(`_ = r.report(...)`, `task_runner.go:114/130/140`). A send failure (agent→server
transport down) silently drops the event. The worst case: the task **completed**,
the `completed` event failed to send, and the `task_runs` row stays `running`
forever — a completed task with a permanently unknown result and no recovery.

### 1.3 Agent restart leaves running tasks stuck

The in-memory `tasks` map is lost on restart. A `task_runs` row left in `running`
never receives a terminal event, so it is stuck with no way to know whether the
game side effect executed. Nothing reconciles it.

### 1.4 Page idempotency is not game side-effect deduplication

The UI/API `idempotency-key` dedups task-_record_ creation at the platform
layer. It says nothing about whether the game function's side effect ran once —
a crash after the game applied the effect but before the platform recorded
completion is indistinguishable from "never ran" without a game-side key.

## 2. Decisions

### D1 · Same taskId duplicate Start is an idempotent no-op

- **Task identity**: `taskId` is the platform-level task identity. The server
  generates it and creates the `task_runs` row (unique index on `task_id`,
  `internal/model/task.go:10`) **before** dispatch. One `taskId` ⇒ one
  `task_runs` row ⇒ at most one intended execution.
- **Duplicate Start** (a second `Start` for a `taskId` already running): **do not
  start a second goroutine and do not overwrite the `cancel` registration.** The
  call returns the existing registration (idempotent no-op) and logs a warning.
  A duplicate is a dispatch/retry artifact (or a re-registration race), not a new
  task.
- **Ownership guard** (for ARCH-015): each in-flight entry is owned by exactly
  one goroutine. A finishing goroutine removes only its _own_ entry, never a
  replacement's — implement with a per-taskId generation/reference check so the
  old goroutine's cleanup cannot delete a same-ID newer task's slot (fixes
  §1.1's delete-on-finish).

### D2 · Event delivery is ACK-after-persist with bounded retry

- **Durability point**: the server ACKs a task event only after it is persisted
  (`handleTaskEvent` → status update + `AppendEvent` to `task_events`). The
  sidecar treats a successful server ACK as delivered.
- **Send failure**: the sidecar must not silently drop (§1.2). It retries with
  bounded backoff; if it cannot deliver within the window it transitions to the
  recovery path (D3) and reports the failure. A task that completed is never left
  with its completion undelivered and ignored.
- **Replay/ordering**: events carry a monotonic `seq` per `taskId`
  (`task_events` `(task_id, seq)` index). A delivered event with `seq` ≤ the
  last persisted `seq` is idempotent-skipped; events are applied in `seq` order.

### D3 · Agent restart yields an explicit "unknown" result, never silent

- On agent (re)start the in-flight map is lost; the platform cannot know whether
  a `running` task's game side effect executed. **The platform does not silently
  retry or leave the row stuck.**
- **Recovery**: on agent re-registration, the server reconciles `task_runs` still
  `running` for that agent into a first-class **`unknown`** state (a distinct,
  terminal-adjacent status meaning "the platform cannot determine whether the
  game side effect ran") and appends a recovery event. This is surfaced for
  manual reconciliation, not hidden.
- **No auto-retry** for non-idempotent functions: auto-redispatch could duplicate
  a game side effect that may already have executed. Auto-retry is allowed only
  for functions the game explicitly declares safe to re-run (D5).

### D4 · Terminal state is monotonic; cancel does not regress it

- A terminal status (`succeeded`/`failed`/`cancelled`/`timed_out`/`unknown`) is
  final. A late or duplicate event (old goroutine after restart, replay) must not
  regress it — the existing `UpdateRunIfStatusNotIn` guard extends to include
  `unknown`.
- **Cancel is a request, not a guarantee**: `cancel_requested` is not terminal. A
  task that already executed its side effect may still report `completed` after a
  cancel request (the game applied the effect before cancellation took effect);
  the server accepts that `completed` event and does not revert it to `cancelled`.
  "取消不回退终态": cancellation never rewrites an already-terminal row.

### D5 · Platform idempotency is not game side-effect deduplication

- The platform `idempotency-key` dedups **task_runs record creation** at the
  API/page layer (at most one row per key). It is **not** a guarantee the game
  side effect ran once.
- The platform never injects its idempotency key into the game function's own
  dedup mechanism, and never derives game semantics from it. A game function that
  must not double-apply implements its **own** idempotency (game-defined unique
  key / order id); the platform passes the caller's key through but does not
  enforce game dedup.
- Recovery must therefore be conservative: auto-retry only for functions the game
  declares idempotent; otherwise surface `unknown` for manual resolution (D3).

## 3. Review table

| #   | Scenario (current code)                                  | With D1–D5                                                                                                     |
| --- | -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| 1   | Two `Start` with the same `taskId`                       | Second is an idempotent no-op; no second goroutine; cancel registration preserved (D1)                         |
| 2   | First goroutine finishes while a same-ID task is running | Old cleanup removes only its own entry; new task stays cancellable (D1)                                        |
| 3   | `completed` event send fails                             | Bounded retry; if undelivered → recovery path (D3); row never silently stuck (D2)                              |
| 4   | Agent restarts mid-task                                  | `task_runs` `running` → `unknown` on re-registration, recovery event appended, surfaced for manual review (D3) |
| 5   | Late/duplicate event after a terminal event              | Idempotent-skip by `seq`; terminal status not regressed (D2/D4)                                                |
| 6   | Cancel request on a task whose side effect already ran   | `completed` accepted; not reverted to `cancelled`; cancel_requested is not terminal (D4)                       |
| 7   | Same `idempotency-key` submitted twice (UI double-click) | At most one `task_runs` row; game side effect may still double-apply unless game is idempotent (D5)            |

## 4. Honest limits (边界诚实)

- **Unknown is honest, not a lie**: after an agent crash the platform genuinely
  cannot tell whether the game side effect ran. `unknown` states this; it is not
  a hidden retry or a fabricated result.
- **Per-agent, not cross-agent**: a `taskId` belongs to the agent that received
  it; there is no cross-agent reconciliation for a task, and no
  at-least-once/exactly-once guarantee across an agent handover (that would need
  game-level idempotency, D5).
- **Server persistence is the durability floor**: if the server DB is down, a
  task event cannot be persisted/ACKed; that path (and the pre-dispatch persist
  failure) is ARCH-026, not settled here.
- **Retry/redispatch is bounded and manual**: the protocol defines _how_ a task
  recovers (unknown + surfaced), not _that_ it auto-redispatches. Redispatch of a
  non-idempotent function is a product decision (game declares idempotency, D5).
- No production readiness is claimed: the protocol is a contract for ARCH-015 to
  implement and ARCH-018 to fault-drill; the drill runs against fixtures, not a
  real game (real-game verification is GAME-010).
