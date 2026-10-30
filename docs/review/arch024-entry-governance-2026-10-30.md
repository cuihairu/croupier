# ARCH-024 · 任务及调度入口统一执行治理（记录）

日期：2026-10-30。源码基线：本分支 HEAD。实现：
`internal/platform/functioninvoke/governance.go`（新增）、`invoke.go`、
`internal/api/task/service.go`、`internal/api/schedule/service.go`、
`internal/tasks/scheduler/manager.go`、`cmd/server/root.go`、
`internal/api/approval/service.go`。

## 缺陷（VERIFIED 诊断，2026-10-29 再审）

1. **task.Service.Start 只做部分 gate**：/tasks Start 路径只做函数权限检查，
   政策审批/输入契约与 invoke 路径不共享，同一高危函数在两个入口行为不一致。
2. **schedule 与 svc.dispatcherAdapter 直连 Dispatcher**：schedule Create/TriggerNow
   与 cron 自动触发直接派发，完全不经函数治理——手动调度/定时触发可绕过审批、
   禁用、分配与输入契约。
3. **不可据统一 transport 宣称统一治理**：transport 统一不构成执行治理统一；
   治理语义须在调用链入口显式收敛。

## 修复：可注入统一治理闸门（Governance.Gate）

`functioninvoke.NewGovernance(svcCtx)` 提供共享闸门，四个入口（普通 invoke、
/tasks Start、schedule Create/TriggerNow、cron 自动触发）全部经同一链：

```
validate ID → EnsureFunctionEnabled（E2 禁用）→ EnsureFunctionAssigned（BUG-032 分配）
→ CheckInvokePermission（人类主体函数权限）→ EnforceFunctionPolicy（ARCH-025 fail-closed）
→ ValidateInvokeInput（输入契约）→ RequireApproval = policy.RequireApproval && !hasSnapshot
```

`functioninvoke` import `svc` 引入环，governance 注入点在 `cmd/server/root.go`
（`svcCtx.StartScheduler(functioninvoke.NewGovernance(svcCtx))`）与各入口
`NewGovernance(s.svcCtx)` 就地构造。

### 自动化主体与批准快照显式定义

- **自动化主体（cron）**：`Subject{AutomatedID: "schedule:<id>"}`，Admin=nil，
  无独立权限 ID；角色由调用方经调度 creator 回载（`LoadCurrentAdmin` 注入
  `username=schedule.Actor`），供策略 `AllowedRoles` 判定。creator 被删除/撤权
  → 角色为空 → 策略拒绝 → 跳过派发（fail-closed）。
- **批准快照**：调度 metadata 写入 `approvalBypass=approved`（+ `approvalId`、
  `approvalActor`）作为站立授权，满足 `RequireApproval`（不二次审批）；但
  禁用/分配/函数权限/政策本身仍 live 回读——撤权后不继续裸派发。
- **函数级政策与 scope 授权正交**：scope 由入口中间件/显式
  `RequireGameEnvScope` 承担，闸门只管函数级治理，不把模块级 JWT/scope 误作
  函数政策。

### 入口行为

| 入口            | 政策要求审批时                                                        | 不要求审批时           |
| --------------- | --------------------------------------------------------------------- | ---------------------- |
| /tasks Start    | 创建审批请求（approvalTarget=task），不派发                           | 派发                   |
| schedule Create | 调度创建为 paused + 审批请求（approvalTarget=schedule）；批准续跑激活 | 直接 active + 派发周期 |
| TriggerNow      | 创建审批请求（approvalTarget=schedule），不派发                       | 派发                   |
| cron 自动触发   | 跳过派发（RunLog=skipped，推进下次触发）                              | 派发                   |

schedule Create 的审批创建失败会删除已建调度（不留半截 paused）；批准续跑经
`approval/service.go` 的 `activateApprovedSchedule` 激活并写入批准快照、重算
`next_triggered_at`。

## 边界与副作用

- **未注入治理维持派发**：`scheduler.Manager` 未 `SetGovernance` 时（旧测试/未
  接线）维持派发；生产由 cmd/server 注入。
- **cron 治理跳过不计数失败**：`skipped` 不递增失败计数、推进下次触发时间，
  否则每周期重复判定同一槽位。
- **审批创建失败回滚调度**：schedule Create 在审批请求失败时删除调度，避免
  paused 半截状态残留。
- **自动化主体无独立权限 ID**：cron 不查函数权限（无人类操作员），其授权由
  creator 角色经策略 `AllowedRoles` 判定；函数权限（function:invoke）仅约束
  人类入口。
- **线上 postgres HA 演练**：多实例并发治理/触发一致性随 GAME-010 与
  ARCH-023 的真实多进程演练范围，本记录不重复验证。

## 验证

- 新增 22 用例：`governance_test.go`（11）、`manager_governance_test.go`（5）、
  `task/approval_gate_test.go`（2）、`schedule/approval_gate_test.go`（3）、
  `approval/schedule_activation_test.go`（1）。
- `go test ./internal/...` 全绿（164 包）、`go build ./...`、`go vet` 通过、
  gofmt 干净、docs build 通过。
