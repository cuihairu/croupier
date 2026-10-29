# 审批并发与执行领取语义

> ARCH-001 · P1 · 方案决策 · 审批并发与执行领取语义
>
> 范围：函数调用的审批门（`internal/api/approval/service.go`
> `Service.Approve`/`Reject` → `continueApprovedFunction`）及其持久层
> （`internal/platform/approvals/store.go` 的 `Store` 接口，含 `SQLStore`
> 与 `MemStore`）。工作流引擎（`internal/platform/approvals/workflow.go`，
> 带过期/升级的多步工作流）是独立特性，本文不覆盖。
>
> 状态：**已决策**（2026-10-29）。D1–D7 已定：CAS 原子转换、胜出者续跑领取、
> 结果未知语义、重复请求 409、转换前强制授权、兼容目标，以及 D7 持久派发意图/
> 领取令牌（fencing）/恢复对账协议。驱动 ARCH-002 实现；崩溃安全派发的游戏侧
> exactly-once 以 GAME-009/010 收据为界，本文不宣称生产就绪。
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
  成功走到这一步的调用方都可能再次派发函数；游戏侧副作用取决于派发结果与游戏实现。

静态风险（以下交错顺序可由代码推导；真实多实例故障注入为 NOT RUN）：

| #   | 场景                                                | 当前行为                                                                                                        | 危害                                                                                           |
| --- | --------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| 1   | 两个操作员并发批准同一 pending 请求                 | 双方都读到 `pending`，都写 `approved`，都触发续跑                                                               | **函数执行两次**——双重副作用（如双重充值/双重封禁）。                                          |
| 2   | 批准与拒绝竞争                                      | 双方都读到 `pending`；后写者胜；若批准的写最后落库，即使已有拒绝记录续跑照样执行                                | 终态不可判定；同一条审批在审计链上同时出现 `approved` 与 `rejected` 行；被拒绝的请求仍被执行。 |
| 3   | 重复批准（同一操作员双击/重试）                     | 第二次 `Approve` 覆盖 `Approver`/`ReviewedAt` 并再次触发续跑                                                    | **函数执行两次**；审批人归属与时间戳被污染。                                                   |
| 4   | 批准成功但续跑失败/服务在批准落库与结果更新之间崩溃 | `state="approved"` 但 `ResultKind`/`TaskID`/`Result` 为空；`Reason` 可能携带 `approved but continuation failed` | 「已批准但结果未知」——尚无经过验证的自动恢复协议，需查询或对账。                               |

## 2. 决策

### D1 — pending→终态的原子转换（CAS）

`Approve` 与 `Reject` 必须是**单条语句的条件更新**：

```sql
UPDATE approval SET state = :new, approver = :operator, reviewed_at = :now, reason = :reason
WHERE id = :id AND state = 'pending'
```

- `SQLStore` 用 `db.Model(&ApprovalModel{}).Where("id = ? AND state = 'pending'", id).Updates(...)`
  实现并检查 `RowsAffected`。各受支持数据库行为仍需集成验证，
  且不需要 `SELECT ... FOR UPDATE`（单语句原子性已足够——不存在需要加锁的
  读后写窗口）。
- `MemStore` 在既有 `mu` 锁内实现同一规则：`a.State != "pending"` 时拒绝
  转换。
- `RowsAffected == 0`（或 MemStore 状态守卫失败）时，存储重读记录（不存在仍返回 not-found，数据库错误保留为错误），并连同
  类型化哨兵错误 `ErrAlreadyReviewed` 一起返回（携带当前
  state/approver/reviewedAt）。调用方（service）将其映射为 HTTP 409，
  **不**触发续跑。
- CAS 失败时存储不得改动记录；失败的 CAS 是只读操作。

**决策：** 第一条把 `pending → 终态` 的评审胜出。落选请求（批准对批准、
批准对拒绝、重复批准/拒绝）得到稳定的冲突；不触发续跑、不写审计、不发通知。

### D2 — 续跑仅由状态转换的胜出者领取

`Service.Approve` 必须把「CAS 成功（`RowsAffected == 1`）」视为运行
`continueApprovedFunction` 的胜出条件（仍须通过强制授权）。`ErrAlreadyReviewed` 必须在任何续跑/
审计/通知副作用之前短路。

续跑 invoke（带 `metadata["approvalBypass"]="approved"`、
`runtime_api=approval.continue` 的 `FunctionInvoke`）保持现有形态，但
**只有胜出者调用它**。这只能防止落选评审再次派发；CAS 本身不保证崩溃后的持久派发或游戏副作用恰好一次。

### D3 — 执行结果未知的语义

报错或超时不能证明游戏操作没有执行。审批决定保持终局，保留所有已知
operation/task 标识与收据；未查证前执行状态保持未知，不能抹去已有结果或伪造失败。

- 仅当 `TaskID` 已持久化时查询原任务。当前服务在续跑返回后才更新它，
  崩溃可能导致审批中没有 TaskID，不能声称任务模式总能恢复。
- 同步操作须用原操作身份查询游戏侧持久收据；没有该契约时保持未知并人工对账。
- 新审批或新调用不是安全重试。只有验证过持久幂等契约后，才可沿用原操作身份
  重试，不能换新 ID 规避未知结果（GAME-009/010）。
- ARCH-001 还须决定持久执行意图/领取及恢复协议：派发前持久化原操作身份与
  参数指纹，明确所有权/fencing、恢复和结果对账。可能需要存储迁移；ARCH-002
  不能仅凭 CAS 宣称实现崩溃安全派发。

现状依据：`Service.Approve` 忽略结果 `Update` 的失败，续跑报错时写 `Reason`；
`recordApprovalAudit` 是尽力写入，因此 approved 也不能证明审计已落库。
控制台是否准确展示未知结果仍待核实。

### D4 — 重复请求规则

对已处于终态的记录再次发起 `Approve`/`Reject`（同人或他人、双击或重试）
**不是**重新执行的触发器：

- `ErrAlreadyReviewed` → HTTP 409 `approval_already_reviewed`，`details`
  携带当前 `state`/`approver`/`reviewedAt`。调用方看到既有决定并据此调和
  （如渲染「已由 X 批准」）。
- 幂等**不是**「第二次请求静默成功」——第一个决定生效，重复请求以冲突拒绝，
  客户端不会把它误当新批准。理由：重复请求静默成功会掩盖续跑已派发的事实；
  冲突让双重执行风险在 UI 层显式可见。

### D5 — 转换前的强制授权

scope、登录身份、自审批拒绝与 step-up OTP 检查均为强制约束。CAS 只决定谁赢，
不能替代授权。若 scope 或已批准参数可在预读与转换之间变化，须绑定已授权的
revision/参数指纹，或在事务中重新校验，不能沿用过期授权。

### D6 — 兼容目标与未决存储方案

保持续跑 metadata 的兼容目标，以及仅胜出者写成功决定审计/通知的语义。
落选请求不得产生成功决定事件，但允许记录被拒绝尝试的安全日志。兼容目标
不等于保证无迁移：持久领取字段、事务边界与恢复协议仍属 ARCH-001，
GAME-009/010 提供游戏侧操作身份/收据边界。

### D7 — 持久执行领取与恢复协议（ARCH-001 收口决策）

CAS 只保证「一次批准决定」；它不保证崩溃后恰好派发一次。为使「已批准但
结果未知」可对账，胜出者必须先把**派发意图**持久化，再调用续跑：

- **意图先行**：CAS 胜出后、调用 `continueApprovedFunction` 前，在同一
  记录上写 `claim_status = claimed` + `claim_token`（单调递增的领取令牌）
  - `claimed_at` + `dispatch_payload_fingerprint`（参数指纹）。只对
    `RowsAffected==1` 的意图写入者开放续跑入口，其余（已领取/已回写/未知）
    一律 409，不重复派发。
- **所有权/fencing**：`claim_token` 必须随续跑 invoke 的 metadata 传给游戏
  侧（如 `approvalClaimToken`），游戏适配器以 `(operationId, claimToken)`
  做持久去重；令牌不再前进则旧派发失效。没有游戏侧去重契约时，平台只保留
  意图与未知状态，不宣称恰好一次（GAME-009/010 边界）。
- **恢复/对账**：崩溃后按 `claim_status` 分派——`claimed` 未回写
  `result_kind/task_id` 的，查询原任务/收据收敛；`dispatched` 无收据的
  保持未知并人工对账；不能仅凭 CAS 推导副作用次数。
- **与现有原语复用**：不新建平行表。审批单已有 `IdempotencyKey`/`Result`/
  `TaskID`，页面绑定执行已有 `page_execute_idempotencies`（running/completed
  占位 + 超窗回收），任务执行已有 `task_runs`。ARCH-001 在审批单上补充
  领取字段与条件更新，复用既有占位/对账口径；不引入第三套去重系统。

存储迁移与事务边界：`claim_status`/`claim_token`/`claimed_at` 为审批单新增
列（ARCH-002 迁移），意图写入与结果回写在同一记录的条件更新内完成，不需要
跨表事务。实现验收须覆盖派发前崩溃、回包丢失、结果落库失败及强制授权失败，
仍为 NOT RUN。

## 3. 评审表（ARCH-001 交付物）

| 用例                                               | 前置                                      | 胜出者行为                                                                            | 落选者/重复者行为                                                                          | 终态                                 | 执行次数                                      |
| -------------------------------------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------------------------------------ | --------------------------------------------- |
| 批准对批准（两操作员并发）                         | 两请求都读到 `pending`                    | 首个 CAS 胜；审计 `approved`，续跑执行，`ResultKind`/`TaskID` 落库                    | 第二个 CAS `RowsAffected==0` → 409 `approval_already_reviewed`；**不续跑、不审计、不通知** | `approved`                           | 最多一次续跑尝试；游戏结果需收据确认          |
| 批准对拒绝（并发）                                 | 一批准 + 一拒绝落在 `pending` 上          | 首个 CAS 胜；其决定 + 续跑（若批准胜出）                                              | 落选者 → 409 `approval_already_reviewed`；无副作用                                         | `approved` 或 `rejected`（先到先得） | 批准胜出最多一次续跑尝试，否则 0              |
| 重复批准（同人双击/重试）                          | 记录已 `approved`                         | 首次请求的续跑已派发                                                                  | 第二次 → 409；返回既有 `approver`/`reviewedAt`                                             | `approved`                           | 重复请求不增加尝试                            |
| 重复拒绝                                           | 记录已 `rejected`                         | —                                                                                     | 第二次 → 409；返回既有 reason/approver                                                     | `rejected`                           | 0                                             |
| 批准成功但续跑失败（agent 掉线/契约过期/任务错误） | CAS 胜出，`continueApprovedFunction` 报错 | `state=approved`，`Reason=approved but continuation failed: ...`，`ResultKind` 为空   | —                                                                                          | `approved`（决定终局）               | 未知；查询原任务/收据，不盲目重试             |
| 批准成功、服务在结果更新前崩溃                     | CAS 胜出，`ResultKind`/`TaskID` 未写      | 重启后记录为 `approved` 且结果字段为空；控制台应展示「已批准，结果未知」（UI 待核实） | —                                                                                          | `approved`                           | 未知；需持久意图/收据对账，缺少 ID 时人工排查 |

## 4. ARCH-002 实现要点

1. 在 `internal/platform/approvals/store.go` 增加
   `ErrAlreadyReviewed = errors.New("approval already reviewed")`（导出，
   附提取冲突记录的助手，如 `AsAlreadyReviewed(err) (*Approval, bool)`）。
2. `SQLStore.Approve`/`Reject` 改为条件 UPDATE + `RowsAffected` 检查
   （0 行时重读）。
3. `MemStore.Approve`/`Reject` 在既有锁内加 `a.State == "pending"` 守卫。
4. `Service.Approve`：`ErrAlreadyReviewed` 时返回
   稳定码为 `approval_already_reviewed` 的 HTTP 409 错误（按实际错误助手契约构造），`details` 携带
   当前 state/approver/reviewedAt；**跳过**续跑、审计、扩展 upsert、事件与
   通知。仅 `RowsAffected==1` 路径进入 `continueApprovedFunction`。
5. `Service.Reject`：同样的冲突映射；落选拒绝无副作用。
6. 测试：并发测试对同一记录并发发 N 次 `Approve`，断言恰好一次续跑
   （如用假件统计 `continueApprovedFunction` 调用次数或 `FunctionInvoke`
   间谍），另覆盖批准对拒绝、重复批准断言 409 与无崩溃场景一次续跑尝试。崩溃/超时用例断言未知结果与收据对账，不以 CAS 推导游戏副作用恰好一次。
   `MemStore` 与 `SQLStore` 都需要 CAS 单测。
7. 多人/法定人数审批（若某工作流要求 N 个批准后才 `approved`）**不属于**
   本门——调用门的 `Store` 是单批准语义。未来工作流若需要法定人数，那是
   `workflow_store.go` 的新关注点，不是 `Store.Approve` 的变更。

实现验收还须覆盖派发前崩溃、回包丢失、结果落库失败及强制授权失败；以上为 NOT RUN。本文不修改运行时行为。
