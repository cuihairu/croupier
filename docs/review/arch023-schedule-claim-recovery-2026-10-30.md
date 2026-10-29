# ARCH-023 · 定时触发先领取再派发（实现与崩溃恢复记录）

日期：2026-10-30。源码基线：本分支 HEAD。实现：`internal/tasks/scheduler/manager.go`、
`internal/model/task_schedule_model.go`、`internal/model/task_schedule.go`（无 schema 变更）。
真实多进程数据库 HA 演练 NOT RUN（确定性替身 + sqlite 单测，见诚实边界）。

## 缺陷与修复

**VERIFIED 缺陷（2026-10-29 再审）**：`manager.go` 旧顺序为 查 HasRunLog → StartTask →
CreateRunLog 且忽略冲突。两个 Manager 并发触发同一槽位都通过 HasRunLog 判重、双双派发、
仅 1 条唯一记录——唯一索引只保证日志写一条，不能撤销已发出的第二次执行。

**修复（claim-first，`6fffb0ea`）**：把 `CreateRunLog` 的 `(schedule_id, slot)` 唯一索引当作
**领取令牌**置于派发之前。唯一冲突（`claimed==false`）即本槽已被领取，不派发；领取写入
返回基础设施错误时宁可不派发（记录失败不得执行），下轮重试同槽。

## 领取租约与崩溃恢复（本次新增）

领取先行仍留缺口：领取方在派发完成/回写前崩溃时，领取行永远停在 `status='claimed'`。
同槽重触发被唯一索引挡住 → 旧代码只推进不派发，触发静默丢失直到下一个 cron 周期，
且历史行永远显示 claimed。补上租约语义：

- **领取租约**：默认 60s（= 2× 默认扫描周期 30s），`SetClaimLease` 可覆盖。
- **孤儿判定**：领取行 `status='claimed'` 且 `created_at` 早于 `now-claimLease` → 领取方已崩溃。
- **原子重领（CAS）**：`ReclaimStaleRunLog` 单条 UPDATE——仅当行仍 `status='claimed'` 且
  `created_at < olderThan` 时重置为一次新领取（`created_at=now`，清 `task_run_id/message`），
  `RowsAffected>0` 为命中。两个并发恢复者只有一个命中（另一个的 WHERE 因 `created_at` 已被
  刷新而失配），不会同槽双恢复。
- **恢复派发**：重领成功即落入正常派发路径（崩溃恢复派发），派发结果照常回填领取行。

## 中断窗口覆盖

| 窗口   | 中断点             | 恢复行为                | 语义                                                                                                                       | 证据（测试）                                                                    |
| ------ | ------------------ | ----------------------- | -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| 窗口一 | 领取后/派发前      | 孤儿领取被重领 → 重派发 | 触发不静默丢失（at-least-once 触发投递）                                                                                   | `TestManager_StaleClaimRecoversTrigger`                                         |
| 窗口二 | 派发后/回写前      | 同窗口一：重派发        | 上一次派发可能已创建 TaskRun；领取行无 taskID 可查，无法区分两窗口——按重派发处理，业务副作用去重由 GAME-010 的游戏幂等承担 | `TestManager_Window2RedispatchIsAtLeastOnce`                                    |
| —      | 活领取（租约未过） | 不重领、不派发、只推进  | 不误伤持有中的实例                                                                                                         | `TestManager_FreshClaimNotRecovered`                                            |
| —      | 已终态行           | 永不重领                | 正常完成的槽位不被二次派发                                                                                                 | `TestManager_TerminalClaimNeverRecovered`、`TestTaskScheduleReclaimStaleRunLog` |
| —      | 并发恢复者         | 恰好一个重领并派发      | CAS 原子性                                                                                                                 | `TestManager_ConcurrentReclaimExactlyOneRedispatch`                             |

## 多实例同槽仅一派发

- 并发触发：`TestManager_ConcurrentTriggerExactlyOneDispatch`（4 实例竞争同一槽 → 派发 1 次）。
- 并发恢复：`TestManager_ConcurrentReclaimExactlyOneRedispatch`（4 实例竞争重领孤儿 → 派发 1 次）。

## 诚实边界（不夸口）

- **窗口二无法区分**：领取行在派发前不写 taskID，恢复时无从判断上一次派发是否已创建
  TaskRun。按 at-least-once 重派发（宁可重复、不可丢失），重复副作用由游戏幂等去重
  （GAME-010）。这是设计决策，不是静默丢任务。
- **租约低估的双派发**：一个仍在执行的慢派发超过租约窗口时可能被另一实例误判为孤儿并
  重领 → 双派发。默认租约 60s 相对最坏派发时长（dial 5s × failover 3 次 ≈ 15s）有裕量；
  属租约机制的固有边界，游戏侧去重兜底。
- **真实多进程数据库 HA 未演练**：CAS 重领与唯一索引在 sqlite 单测 + 确定性替身验证；
  多实例共享 postgres 的并发语义属运维 HA 演练范围（GAME-010 验收时一并做）。
- **触发不重放错过的历史槽**：宕机期间错过的 cron 周期不逐个补发（设计上只触发当前
  到期一次并推进到下一周期），与领取恢复独立。
