# ARCH-025 · 策略查询与初始化故障禁止静默放行（fail-closed 记录）

日期：2026-10-30。源码基线：本分支 HEAD。实现：`internal/platform/functioninvoke/policy.go`、
`internal/platform/functioninvoke/invoke.go`。故障复现：确定性（缺失 function_policies 表 +
nil 管理器），非线上数据库故障注入。

## 缺陷（VERIFIED 诊断，2026-10-29 再审）

1. **GetPolicy 错误被吞为「无政策、无错误」**：`EnforceFunctionPolicy`（policy.go:78–85）在
   GetPolicy 返回错误（function_policies 表缺失 / SQL 失败 / db down）时返回 `nil, nil`。
   调用方 `Invoke` 只有 `functionPolicy != nil` 时才进入审批与审计分支——高危函数在
   无有效政策时绕过审批/审计直接派发。
2. **PolicyManager 初始化失败同样放行**：`service_context.go` 在 NewManager 失败时设
   `PolicyManager = nil`（malformed default-policies.yaml），`Invoke` 的 `if PolicyManager != nil`
   守卫直接跳过整个政策分支。

## 修复：fail-closed 拒绝路线（不采用 last-good）

| 故障面                                | 旧行为                                     | 新行为                                   | 证据（测试）                                                                                                                                                                                          |
| ------------------------------------- | ------------------------------------------ | ---------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GetPolicy 错误（表缺失/SQL/db down）  | `return nil, nil` → 绕过审批/审计          | 返回包装错误 → `Invoke` 拒绝执行         | `TestEnforceFunctionPolicy_StorageFailureFailsClosed`、`TestFunctionInvoke_PolicyStorageFailureRefusesHighRisk`、`TestEnforceFunctionPolicy_GetPolicyError_V9`（原 fail-open 断言翻转为 fail-closed） |
| PolicyManager 初始化失败（nil）       | `if PolicyManager != nil` 跳过整个政策分支 | 拒绝执行（`policy manager unavailable`） | `TestEnforceFunctionPolicy_NilManagerFailsClosed`、`TestFunctionInvoke_NilPolicyManagerRefuses`                                                                                                       |
| 已批准续行（approvalBypass=approved） | 豁免政策分支                               | 保持豁免（上游已把关，不二次拒绝）       | `TestFunctionInvoke_ApprovedContinuationExemptFromNilManager`                                                                                                                                         |
| 无覆盖策略（无行）                    | 回落风险级默认策略（非 nil）               | 不变（GetPolicy 既有语义，与故障面区分） | `TestEnforceFunctionPolicy_NoRoleRestriction` 等既有默认策略用例                                                                                                                                      |

**「无覆盖策略」与「存储/配置故障」的区分**：由 GetPolicy 既有语义承担——无行回落
`GetDefaultPolicy(riskLevel)`（永不 nil），SQL 故障返回错误。旧 `nil,nil` 只会出现在故障面；
现在故障面一律 fail-closed，`nil` 政策只伴随错误（调用方拒绝），不再有「政策为空且无错」
的放行形态。

**为何不用 last-good**：策略管理器是内存态（启动时读 `default-policies.yaml`，`loadConfig`）；
初始化失败时无既有 last-good 可回退。配置故障的诚实选项是拒绝并明确报错，而非静默放行。
如实记录：本修复不宣称存在 last-good 回退路径。

## 边界与副作用

- **拒绝范围**：故障面（表缺失/SQL 失败/db down/配置损坏）下所有需要政策决策的调用被拒绝，
  包括低/中风险——比「仅高风险拒绝」更严格，因为无法确定 RequireApproval/RequireAudit/
  AllowedRoles 时不存在「安全放行」的判定依据（中风险默认要求审计）。属安全保守姿态。
- **已批准续行/页面快照治理豁免**：两类调用上游已做门禁（审批续跑/页面级校验），
  不因策略存储故障二次拒绝，避免误伤已获批准的合法续行。
- **注册期默认策略创建不受影响**：`EnsureDefaultPolicy` 仍在注册时 best-effort 创建
  （已有 nil 守卫，仅记警告），非执行门禁。
- **线上数据库故障注入 NOT RUN**：诊断用缺失表确定性复现；生产 postgres 的查询故障
  注入属运维演练范围。

## 验证

- `go test ./internal/api/function/ ./internal/platform/functioninvoke/` 全绿（含 6 个 ARCH-025 用例）。
- `go build ./...`、`go vet` 通过、gofmt 干净、docs build 通过。
