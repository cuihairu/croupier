# 审批并发与执行领取语义

> ARCH-001 · P1 · 方案决策 · 审批并发与执行领取语义
>
> 范围：函数调用的审批门（`internal/api/approval/service.go`
> `Service.Approve`/`Reject` → `continueApprovedFunction`）及其持久层
> （`internal/platform/approvals/store.go` 的 `Store` 接口，含 `SQLStore`
> 与 `MemStore`）。工作流引擎（`internal/platform/approvals/workflow.go`，
> 带过期/升级的多步工作流）是独立特性，本文不覆盖。
>
> 状态：**已决策**（2026-10-29）。驱动 ARCH-002 实现。
> 前置关系：ARCH-002（接入审批 SQL 存储）。

## 1. 问题

调用审批门没有并发保护。SQL 与内存两种存储把 `pending → approved|rejected`
的转换实现为无条件的读后写，且服务端对**每一次** `Approve` 调用都触发续跑，
而不是只让真正完成状态转换的那一次触发。

证据：

- `internal/platform/approvals/sql_store.go` 的 `Approve`：先 `First(id)`
  再无条件 `Save(state="approved")`。`Reject` 相同。没有
  `WHERE state='pending'`、没有行锁、没有受影响行数检查。
- `internal/platform/approvals/store.go` 的 `MemStore.Approve`：在锁内
  无条件 `a.State = "approved"` 覆盖——同样没有状态守卫。
- `internal/api/approval/service.go` 的 `Approve`：
  `ApprovalsStore.Approve(...)` 返回后无条件调用
  `continueApprovedFunction(ctx, record)`。续跑 invoke 不带幂等键，因此每个
  走到这一步的调用方都会执行一次函数。

后果（全部真实可达）：

| #   | 场景                                                | 当前行为                                                                                                        | 危害                                                                                           |
| --- | --------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| 1   | 两个操作员并发批准同一 pending 请求                 | 双方都读到 `pending`，都写 `approved`，都触发续跑                                                               | **函数执行两次**——双重副作用（如双重充值/双重封禁）。                                          |
| 2   | 批准与拒绝竞争                                      | 双方都读到 `pending`；后写者胜；若批准的写最后落库，即使已有拒绝记录续跑照样执行                                | 终态不可判定；同一条审批在审计链上同时出现 `approved` 与 `rejected` 行；被拒绝的请求仍被执行。 |
| 3   | 重复批准（同一操作员双击/重试）                     | 第二次 `Approve` 覆盖 `Approver`/`ReviewedAt` 并再次触发续跑                                                    | **函数执行两次**；审批人归属与时间戳被污染。                                                   |
| 4   | 批准成功但续跑失败/服务在批准落库与结果更新之间崩溃 | `state="approved"` 但 `ResultKind`/`TaskID`/`Result` 为空；`Reason` 可能携带 `approved but continuation failed` | 「已批准但结果未知」——无恢复路径：操作员无法判断函数是否已执行。                               |

## 2. 决策

### D1 — pending→终态的原子转换（CAS）

`Approve` 与 `Reject` 必须是**单条语句的条件更新**：

```sql
UPDATE approval SET state = :new, approver = :operator, reviewed_at = :now, reason = :reason
WHERE id = :id AND state = 'pending'
```

- `SQLStore` 用 `db.Model(&ApprovalModel{}).Where("id = ? AND state = 'pending'", id).Update(...)`
  实现并检查 `RowsAffected`。该写法在 sqlite/mysql/postgres 上均可移植，
  且不需要 `SELECT ... FOR UPDATE`（单语句原子性已足够——不存在需要加锁的
  读后写窗口）。
- `MemStore` 在既有 `mu` 锁内实现同一规则：`a.State != "pending"` 时拒绝
  转换。
- `RowsAffected == 0`（或 MemStore 状态守卫失败）时，存储重读记录并连同
  类型化哨兵错误 `ErrAlreadyReviewed` 一起返回（携带当前
  state/approver/reviewedAt）。调用方（service）将其映射为 HTTP 409，
  **不**触发续跑。
- CAS 失败时存储不得改动记录；失败的 CAS 是只读操作。

**决策：** 第一条把 `pending → 终态` 的评审胜出。落选请求（批准对批准、
批准对拒绝、重复批准/拒绝）得到稳定的冲突；不触发续跑、不写审计、不发通知。

### D2 — 续跑仅由状态转换的胜出者领取

`Service.Approve` 必须把「CAS 成功（`RowsAffected == 1`）」视为运行
`continueApprovedFunction` 的唯一授权。`ErrAlreadyReviewed` 必须在任何续跑/
审计/通知副作用之前短路。

续跑 invoke（带 `metadata["approvalBypass"]="approved"`、
`runtime_api=approval.continue` 的 `FunctionInvoke`）保持现有形态，但
**只有胜出者调用它**。这就是双重执行的修复：落选的评审不得触及执行入口。

### D3 — 执行结果未知的语义

胜出者的 `continueApprovedFunction` 报错时：

- `state` 保持 `approved`（决定是终局——已通过 `EventApprovalApproved`
  落入审计链）。
- `Reason` 置为 `approved but continuation failed: <err>`（保留现有行为）。
- 记录的 `ResultKind`/`TaskID`/`Result` 保持为空，控制台渲染
  「已批准，结果未知」而不是伪造结果。
- 恢复**不**通过重新批准（state 已非 `pending`，再批准会被 CAS 拒为 409）。
  恢复路径是：
  - task 模式：操作员按 `TaskID` 查询任务的真实结果（task 模式续跑返回
    `TaskID`，审批行会持久化它）；
  - sync 模式：操作员重新提交原函数调用（若风控门仍要求审批则产生新审批）
    ——审批决定与执行结果保持解耦。

这里有意把「决定」与「执行结果」分成两个阶段：下游运行失败不回滚审批决定。

### D4 — 重复请求规则

对已处于终态的记录再次发起 `Approve`/`Reject`（同人或他人、双击或重试）
**不是**重新执行的触发器：

- `ErrAlreadyReviewed` → HTTP 409 `approval_already_reviewed`，`details`
  携带当前 `state`/`approver`/`reviewedAt`。调用方看到既有决定并据此调和
  （如渲染「已由 X 批准」）。
- 幂等**不是**「第二次请求静默成功」——第一个决定生效，重复请求以冲突拒绝，
  客户端不会把它误当新批准。理由：重复请求静默成功会掩盖续跑已派发的事实；
  冲突让双重执行风险在 UI 层显式可见。

### D5 — 前置检查留在预读上；CAS 是最终门

scope 校验、自审批拒绝、step-up OTP 强制都在 CAS 之前通过 `Get` 预读记录
（现有行为）。它们是建议性的，保持不变。CAS 是**唯一**决定「谁赢了」的门。
不新增前置检查；顺序（预读 → CAS → 仅胜出者续跑）即不变量。

### D6 — 保持不变的部分

- `Create`/`List`/`Get`/`Update` 的形态与 `Approval` 字段
  （`State`、`Approver`、`ReviewedAt`、`Reason`、`ResultKind`、`TaskID`、
  `Result`）。
- 续跑 payload/metadata（approvalBypass、approvalId、approvalActor、
  runtime_api）。
- 审计事件（`EventApprovalApproved`/`EventApprovalRejected`）、扩展 upsert、
  通知事件——但只在胜出路径上触发（落选者一个都不触发）。

## 3. 评审表（ARCH-001 交付物）

| 用例                                               | 前置                                      | 胜出者行为                                                                          | 落选者/重复者行为                                                                          | 终态                                 | 执行次数                                                           |
| -------------------------------------------------- | ----------------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------------------------------------ | ------------------------------------------------------------------ |
| 批准对批准（两操作员并发）                         | 两请求都读到 `pending`                    | 首个 CAS 胜；审计 `approved`，续跑执行，`ResultKind`/`TaskID` 落库                  | 第二个 CAS `RowsAffected==0` → 409 `approval_already_reviewed`；**不续跑、不审计、不通知** | `approved`                           | **恰好 1**                                                         |
| 批准对拒绝（并发）                                 | 一批准 + 一拒绝落在 `pending` 上          | 首个 CAS 胜；其决定 + 续跑（若批准胜出）                                            | 落选者 → 409 `approval_already_reviewed`；无副作用                                         | `approved` 或 `rejected`（先到先得） | 批准胜出为 1，否则 0                                               |
| 重复批准（同人双击/重试）                          | 记录已 `approved`                         | 首次请求的续跑已派发                                                                | 第二次 → 409；返回既有 `approver`/`reviewedAt`                                             | `approved`                           | 合计 1                                                             |
| 重复拒绝                                           | 记录已 `rejected`                         | —                                                                                   | 第二次 → 409；返回既有 reason/approver                                                     | `rejected`                           | 0                                                                  |
| 批准成功但续跑失败（agent 掉线/契约过期/任务错误） | CAS 胜出，`continueApprovedFunction` 报错 | `state=approved`，`Reason=approved but continuation failed: ...`，`ResultKind` 为空 | —                                                                                          | `approved`（决定终局）               | 0（续跑失败）；task 模式可按 `TaskID` 恢复，sync 模式重新提交      |
| 批准成功、服务在结果更新前崩溃                     | CAS 胜出，`ResultKind`/`TaskID` 未写      | 重启后记录为 `approved` 且结果字段为空；控制台渲染「已批准，结果未知」              | —                                                                                          | `approved`                           | 0..1（设计上不可判定——决定终局；执行结果经任务查询或重新提交恢复） |

## 4. ARCH-002 实现要点

1. 在 `internal/platform/approvals/store.go` 增加
   `ErrAlreadyReviewed = errors.New("approval already reviewed")`（导出，
   附提取冲突记录的助手，如 `AsAlreadyReviewed(err) (*Approval, bool)`）。
2. `SQLStore.Approve`/`Reject` 改为条件 UPDATE + `RowsAffected` 检查
   （0 行时重读）。
3. `MemStore.Approve`/`Reject` 在既有锁内加 `a.State == "pending"` 守卫。
4. `Service.Approve`：`ErrAlreadyReviewed` 时返回
   `errorx.NewConflict("approval_already_reviewed", ...)`，`details` 携带
   当前 state/approver/reviewedAt；**跳过**续跑、审计、扩展 upsert、事件与
   通知。仅 `RowsAffected==1` 路径进入 `continueApprovedFunction`。
5. `Service.Reject`：同样的冲突映射；落选拒绝无副作用。
6. 测试：并发测试对同一记录并发发 N 次 `Approve`，断言恰好一次续跑
   （如用假件统计 `continueApprovedFunction` 调用次数或 `FunctionInvoke`
   间谍），另覆盖批准对拒绝、重复批准断言 409 与恰好一次执行。
   `MemStore` 与 `SQLStore` 都需要 CAS 单测。
7. 多人/法定人数审批（若某工作流要求 N 个批准后才 `approved`）**不属于**
   本门——调用门的 `Store` 是单批准语义。未来工作流若需要法定人数，那是
   `workflow_store.go` 的新关注点，不是 `Store.Approve` 的变更。
