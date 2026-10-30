---
title: Async task retry and terminal-state recovery protocol
---

# Async task retry and terminal-state recovery protocol (ARCH-014)

> Implementation recheck, 2026-10-30: In-flight Start deduplication and bounded terminal retries exist, but run-state and event persistence are not atomic; retries after an event write failure can be ACKed without the missing event. ARCH-015 is reopened. The watchdog is a worktree implementation; independent-process acceptance is NOT RUN.

**Status:** decided (2026-10-30). Implementation is tracked by ARCH-015 (TaskRunner duplicate-Start constraint and event replay / terminal convergence) and ARCH-018 (execution-interruption drill).

This brief defines the protocol semantics for asynchronous agent tasks that the current code only implements by accident. It does not change the wire contract: `task_runs` / `task_events` are reused as-is; no new table is required.

## Problem

Three gaps around in-flight agent tasks leave the task_runs row in a state that contradicts the game's real outcome, or let two executions of one task ID interleave:

1. **Duplicate Start is unchecked.** `TaskRunner.Start` (agents/sidecar/task_runner.go) stores a `taskID → cancel` map entry unconditionally. A second Start for an ID that is already in-flight overwrites the entry, and the first finisher's deferred `delete` removes the second execution's tracking. Worse, `Cancel` deletes the entry eagerly while the executor is still winding down, so a subsequent same-ID Start inserts a new entry that the old goroutine's cleanup then deletes (the "old cleanup must not delete a new same-ID task" hazard). The result is un-cancellable, uncounted tasks and, in the worst case, two concurrent executions of one task ID.
2. **Events are fire-and-forget.** `run` reports started/completed/failed/cancelled through `ReportTaskEvent` and ignores the returned error (`_ = r.report(...)`). A transport failure silently drops the terminal event, leaving the row at `running` forever.
3. **Agent session loss strands the row.** When an agent session is removed (internal/server/agent_session.go), nothing reaps that agent's non-terminal task_runs; a row can sit in `running` / `cancel_requested` indefinitely, with the game-side effect either applied or not.

The shared trap is treating _page execution idempotency_ as _game side-effect deduplication_. A page idempotency record proves the page ran once on the control side; it says nothing about whether the game's own function call completed. Task recovery must therefore surface "result unknown" as a distinct, operator-resolvable state and must not silently re-run.

## Decisions

### D1 — Same taskId duplicate Start: idempotent no-op while in-flight

- A Start for a task ID that is **already in-flight at this agent** is acknowledged (the existing task ID is returned) without launching a second execution and without reporting a second `started` event. No interleaved event stream is ever produced for one task ID.
- Task tracking becomes **generation-tagged**: the map stores `taskID → {generation, cancel}`. `run`'s deferred cleanup deletes the entry **only if the stored generation still matches its own**, so an old goroutine winding down can never delete a newer same-ID task's entry.
- `Cancel` **stops deleting the entry eagerly**; the entry lives until the executor actually returns (cleanup is the single deletion point, generation-checked). A Start arriving during the cancel wind-down sees the ID as still in-flight and is a no-op — correct, because a second execution while the first has not exited would double-apply side effects.
- Rationale: the Server is the sole legitimate source of task IDs (dispatch persists a `task_runs` row — ARCH-026 — then sends StartTask). A duplicate Start is either transport re-delivery or an operator-initiated re-dispatch; in both cases running the old execution once and acknowledging the duplicate is the side-effect-safe answer. Re-running a task is expressed with a **new task ID** (operator re-execute creates a new row), never a same-ID restart.
- Cross-restart duplicates are explicitly out of agent-memory scope: a restart clears the map, so a re-delivered Start after restart will execute. This is a honest limitation surfaced to the operator, not a claim of game-side dedup.

### D2 — Event delivery: the Server response is the ACK; terminal events are retried

- **The Server's `handleTaskEvent` response is the ACK.** The Server persists (`UpdateRunIfStatusNotIn` + `AppendEvent`) before responding; the terminal-guard drop (a late event after a terminal status) also responds success with no record — an intentional no-op that the agent must **not** retry. This split already exists; the design pins it as the contract.
- **Terminal events (completed/failed/cancelled) are delivered with retry-until-ACK**, bounded (bounded attempts + bounded memory), with backoff. The task's `run` goroutine blocks on the terminal event send, so the task is not considered finished upstream until the Server has durably recorded (or intentionally dropped) the terminal event.
- **Non-terminal events (started/progress/log) are best-effort** under transport pressure: progress may coalesce, logs may be dropped. Losing them does not affect convergence — the terminal event still lands (or D3's reaper bounds the row).
- Within a task ID, events are reported sequentially by the single `run` goroutine; the Server's terminal guard keeps `task_events` authoritative (no event after terminal).
- **Explicit non-goal:** no durable agent-side event journal and no cross-restart replay in this iteration. Cross-restart recovery is D3's unknown-state convergence, not replay-to-terminal.

### D3 — Agent session loss / silent run → `unknown`; no auto-retry

- Add a task status **`unknown`** (a new string value on the existing `task_runs.status`; no schema change). It means: _the execution may have completed, failed, or never started; the game-side outcome is not established._
- **Trigger:** when an agent session is removed (deregister / heartbeat timeout / TCP close), the Server transitions that agent's non-terminal task_runs rows to `unknown` (bounded batch, message "agent session lost; execution state unknown").
- **Soft-terminal:** `unknown` stops dispatch-side active-set tracking and SSE waiters, but a **late terminal event from the same agent's reconnected session may still converge the row** to its true terminal state (the agent may have survived a transport blip; the late event is strictly more evidence). This is distinct from `TerminalStatuses` (succeeded/failed/cancelled/timed_out), where late events are dropped.
- **No automatic re-dispatch of `unknown` tasks.** Re-running may double-apply the game side effect; re-execute is an operator decision (new task ID). `unknown` is surfaced in the UI as an explicit state, not a failure.
- **Watchdog backstop:** independent of session loss, a reaper transitions tasks that have produced no event past a deadline (`started_at`/`updated_at` beyond the invoke budget) to `timed_out`. `timed_out` is terminal and means _no further evidence_, not _failed at the game_. (Shipped in ARCH-018; the semantics are fixed here.)

## Boundary: page idempotency is not game side-effect dedup

The page-execution idempotency record dedupes **page execution on the control side**. It is **not** evidence that the game function ran exactly once. Task recovery therefore:

- never cites the page idempotency record as justification to auto-rerun a task;
- surfaces `unknown` / `timed_out` as operator-resolvable states where the game outcome must be established via game-side evidence (a task-status query, a game-side receipt, or a manual check) before any re-run;
- requires a **new task ID** for any re-run, so each re-run is a distinct `task_runs` row and a distinct event stream.

## Implementation mapping (for ARCH-015 / ARCH-018)

| Protocol point                                        | Code seam                                                                                                                                                                       |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1 generation-tagged tracking, no eager cancel delete | `agents/sidecar/task_runner.go` (`Start`, `Cancel`, `run` cleanup, map type)                                                                                                    |
| D2 terminal-event retry-until-ACK                     | `agents/sidecar/task_runner.go` (`run` terminal report); agent reporter (`internal/app/agent/upstream.go`)                                                                      |
| D3 session-loss → `unknown` reaping                   | server agent-session removal path; task status predicates (`internal/tasks/types.go`)                                                                                           |
| D3 watchdog → `timed_out`                             | server ControlService background loop (`runTaskWatchdog`); `TaskRunModel.ReapStale`; `tasks.Store.ReapStale`; budget `taskLog.reaperTimeout` (default 5m) — shipped in ARCH-018 |
| Boundary: re-run = new task ID                        | dispatch / task create path (already the case; keep it)                                                                                                                         |

## Known boundaries

- Cross-restart duplicate execution is not prevented (D1); the agent has no durable local journal (D2). Both are honest limitations surfaced to the operator, not hidden.
- `unknown` is a new status value; UI/type surfaces and the task query render it (implemented in ARCH-015: TaskRunner D1/D2, server session-loss reaping D3, web `unknown` status).
- The D3 watchdog `timed_out` reaper **shipped in ARCH-018**; no-event-past-deadline rows converge to `timed_out` automatically (budget `taskLog.reaperTimeout`, default 5m).
- The protocol does not establish game-side exactly-once semantics; that requires game-provided receipts / idempotency keys in the payload (GAME-010 adjacent, out of scope here).

## Implementation status (ARCH-018, 2026-10-30)

- **Watchdog `timed_out` reaper shipped.** `ControlService.runTaskWatchdog` runs on a background ticker (`budget/2` clamped to 1–30s) and calls `TaskRunModel.ReapStale` via `tasks.Store.ReapStale`: `UPDATE task_runs SET status='timed_out', message='no event past watchdog budget; execution state unknown', finished_at=now WHERE status IN (queued,dispatching,running,cancel_requested) AND updated_at < now - budget`. Budget is `taskLog.reaperTimeout` (duration string, default 5m); `timed_out` is terminal and never re-selected.
- **Execution-interrupt fault drill** (`cmd/server/fault_drill_r27_test.go`, real `runServer` boot + `e2e-agent-probe` subprocess) records per interrupt: 断链 (probe killed) → running rows converge `unknown` + new requests 503; drain → new requests 503 while the in-flight task completes, undrain restores dispatch; watchdog (`-hang` probe: silent session, no disconnect reap) → `timed_out` within budget + `finished_at` set, new requests still accepted; SIGTERM → graceful shutdown returns nil, in-flight row `unknown`, HTTP listener released.
- **Boundary honesty:** the probe's task/side-effect counts are a **test fixture, not game-recovery evidence**. The drill asserts platform convergence (state-machine terminal/unknown/rejection semantics and resource release) only — it does not assert game-side exactly-once (GAME-010 remains NOT RUN).
