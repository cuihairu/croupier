# 游戏后台可靠性修复复审 · 2026-10-30

**CHANGES REQUIRED。** 本轮完成最新修复的调用链/失败路径复审、诊断和台账对齐，不表示修复完成或批准高危游戏写入。基线 `1ec01eea`，覆盖 `c7ab7f94` 后的相关合并及开始审核时的工作区。旧报告是历史快照，见 [10 月 29 日审核](./code-todo-alignment-2026-10-29.md)。

## 范围与证据边界

源码、CLAUDE.md、差异、设计、台账和相关测试 FOUND；真实游戏契约/持久收据、生产配置、外部账号、独立故障及容量环境 ABSENT，相关验收 NOT RUN / NOT ASSESSED — NO DATA。Engine validation: NOT ASSESSED — no engine configured；无特定 story/ADR 输入，按仓库现有协议审核，不伪造 ADR 合规批准。

复核审批 CAS/生产装配；Invoke/tasks/schedule 治理；调度领取；任务持久化/终态；Provider drain；Server 停机；Router；审计链；分析接入、ACK 与聚合；Web 批量执行边界。不是全仓逐行安全审计、跨语言 SDK 全矩阵或生产认证。没有修改业务程序。工作区 watchdog、探针和文档修改被保留，局部单测不等于独立进程验收。

[证据目录](./evidence/reliability-2026-10-30/README.md) 保存诊断源、重放工具、结果和哈希。Go overlay 不改业务源码或既有测试；诊断 PASS 表示断言“缺陷已复现”成立，不是产品验收 PASS。

## 发现与验收回退

### R30-01 · P0 · 统一治理没有复用显式函数 ACL

**已修复（ARCH-024，2026-10-31）。** 原缺口：`internal/platform/functioninvoke/govern.go:govern` 加载 PermissionIDs 后丢弃结果，从未调用 `internal/logic/utils/function_helpers.go:CheckInvokePermission`——显式函数级 ACL 规则（`FunctionModel.ListPermissions` → `FunctionActionAllowed`，如「仅 auditor 可 invoke player.cleanup」）在普通/手动/自动入口从未生效，与 `/tasks Start` 的显式 ACL 检查漂移。`internal/api/task/service.go:Start` 额外检查函数 ACL；`internal/api/schedule/service.go:TriggerNow`、`cmd/server/root.go` 自动治理钩子只调用 Govern。Invoke 也复用该链，不能仅因函数同名便宣布授权一致。

修复：govern 链在主体加载后统一调用 `CheckInvokePermission(gctx, svcCtx, roleNames, permIDs, functionID, gameID, env)`（HasAdminRole 旁路；nil FunctionModel fail-closed 拒绝）——普通、任务、手动、自动入口同一条链，ACL 拒绝 → 零派发、先于审批创建（不制造审批噪音）。诊断覆盖：`internal/platform/functioninvoke/govern_test.go` 的 `TestGovern_ExplicitFunctionAclDenied`/`TestGovern_ExplicitFunctionAclAllowed`（operator 主体 + 仅 auditor 显式规则 → 403 零派发；operator 显式规则 → 放行）。

### R30-02 · P0 · 风险读取故障降低审批要求

**已修复（ARCH-025，2026-10-31）。** 原缺口：`internal/platform/functioninvoke/policy.go:resolvePolicyRisk` 忽略 OpenAPI/FunctionContract 读取错误，回退 RiskMedium。GetPolicy 查询错误已有 fail-closed 修复，但风险输入本身仍可错误降档。

修复：`resolvePolicyRisk` 改为返回 `(policy.RiskLevel, error)`，区分「未声明风险」与「来源故障」——契约行不存在（ErrRecordNotFound）= 未声明，合法回落 medium；其他查询错误 = 来源故障，向上传播，由治理链 fail-closed 拒绝（与 GetPolicy 查询错误的 fail-closed 同构），绝不静默降档。`EnforceFunctionPolicy` 传播该错误；approval step-up 门槛（`approveStepUpRisk`）在来源故障时 fail-closed 视作高危（要求二次验证），不因读取失败放宽。证据：`internal/platform/functioninvoke/govern_test.go` `TestGovern_ContractRiskReadErrorDenies`（function_contracts 表删除 → Govern 拒绝、零审批）、`internal/api/function/coverage_v9_test.go` `TestResolvePolicyRisk_ContractSourceFailure_V9`、`internal/api/approval/stepup_otp_test.go` `TestApproveStepUpRisk_SourceFailureFailsClosed`。

### R30-03 · P1 · 终态部分持久化后，重试 ACK 掩盖缺失事件

**已修复（ARCH-015，2026-10-31）。** 原缺口：`internal/server/control_handler.go:handleTaskEvent` 先 UpdateRunIfStatusNotIn 更新 task_runs，再 AppendEvent 写 task_events，无共同事务。第二步失败后，重试命中终态 !updated，直接成功返回而不补事件——诊断第一次 append 失败、第二次 ACK，append 总次数仍为 1，有限重试未解决部分写入。

修复：`handleTaskEvent` 改调 `tasks.Store.RecordTaskEvent` → `TaskRunModel.RecordTaskEventTransactional`，在单一数据库事务内执行 guard 状态更新 + 事件 seq 计算 + 事件追加，任一失败整体回滚——行不可能在事件行缺失时变成终态，来源恢复后重投的终态事件收敛该行（guard 仍拦截真正迟到的终态后事件）。证据：`internal/server/control_handler_test.go` `TestControlService_HandleTaskEvent_RealStore_NoPartialWrite`（事件表缺失 → 报错且行保持 queued、无部分写入；表恢复 → 重试补写 seq=1 并收敛 failed）、`internal/model/task_model_test.go` `TestTaskRunModel_RecordTaskEventTransactional`/`_RollsBackOnAppendFailure`（seq 顺序、终态 guard 拦截、append 失败回滚状态）。

### R30-04 · P1 · 跨分钟恢复遗漏旧 claimed

**已修复（ARCH-023，2026-10-31）。** 原缺口：`internal/tasks/scheduler/manager.go:triggerSchedule` 领取 Slot(now)，resolveStaleClaim 仅在当前槽唯一冲突时读取该槽——t0 崩溃后的旧槽 claimed 永不被扫描。

修复：tick 每次周期统一调用 `reapStaleClaims`（`ListStaleClaims(created_at < now-claimLease, status=claimed)`）跨槽列出全部过期领取，解析为 `indeterminate`（run-log 不再永久停在 claimed）；绝不重派发（重发=对派发后中断赌二次执行，业务副作用去重归 GAME-010）。同槽 `resolveStaleClaim` 同样改为 `indeterminate`——R30-04 要求派发后/回写前的不确定状态不得写成确定 `failed` 或 `claim lease expired before dispatch`。证据：`internal/tasks/scheduler/arch023_test.go`（跨槽恢复、未到期计划独立扫描）、`internal/model/task_schedule_model.go:ListStaleClaims`。

### R30-05 · P1 · 分析聚合存在生产字段竞态

**CONFIRMED，DATA-014 新增。** `internal/analytics/worker/worker.go:touchRevenue` 写 rv.revenue，flush 读取它，聚合 map 也由消费、pending 回收和刷新并发访问。batchMu/lastMu 只保护批次/checkpoint。

独立 race 诊断用现有 worker 和 Send 失败替身，命中 touchRevenue:625 写与 flush:721 读。另一次常规 race 命中测试跨用例修改 runFlushInterval 与未退出后台读取：这是不同问题，不能当作收入竞态证据，DATA-015 单独处理 Run/main 的有界退出等待与夹具隔离。

明细批次确已改为 Send 后 ACK，重入队/死信 XADD 错误不 ACK。旧文档“Append 后即 ACK”已过时；但真实 Redis/ClickHouse 杀进程与重复数据核对仍属 ARCH-012，ARCH-011 原验收中的这部分未完成。

### R30-06 · P1 · Open 阶段未被首次开库预算中断

**CONFIRMED，ARCH-009 重新打开。** `internal/db/router/router.go:openGameDB` 在预算 context 内同步调用无 context 参数的 cfg.Open，之后才 PingContext。阻塞替身下 OpenBudget=5ms，40ms 后仍未返回，释放 Open 后才报超时。

取消等待者、ensure/migrate context、singleflight 与统计有真实价值，但不能据此保证整段开库有界。须约束实际驱动初始化/建连，不能用遗留 goroutine 冒充资源释放。

### R30-07 · P1 · 外层停机截止未覆盖最终 defer

**已修复（2026-10-31，ARCH-017）。** `cmd/server/root.go` 既 defer StopScheduler，又在停机 goroutine 同步 StopScheduler；30s 外层到期只使函数返回，仍要执行 defer，而 `internal/tasks/scheduler/manager.go:Stop` 等待 stoppedCh，卡死的 tick 曾使最终返回无界。修复分两层：

1. **lifecycle context**（aa02cb9d）：Start 创建 `lifecycleCtx`/`lifecycleCancel`，Stop 先 cancel 再等 stoppedCh；tick 用 lifecycle ctx（nil→Background 回退）。对观察 ctx 取消的卡死调用（如 DB 查询）立即中止。局限：调用不观察 ctx 取消时（硬卡死）无效。
2. **硬停止上限 + 真实入口钻机**（ARCH-017 补）：`Manager.Stop` 对两条调用路径（优雅停机同步 + defer 兜底）统一 `waitStopped(stopTimeout)`，默认 5s（`SetStopTimeout` 可覆盖），超时记日志后返回，**不把「资源已释放」当超时**——循环退出仍由 stoppedCh 保证，阻塞 tick 解卡后自行退出无泄漏。`SetTickHook` 测试缝 + `StartScheduler` 不再自动 Start（root.go 注入治理/钻机钩子后显式 Start）。真实停机入口钻机 `TestDrill_BlockedScheduler_ShutdownBoundedAndCloseRuns`（SIGINT → 30s 内返回、HTTP/control listener 拒绝新连接）通过；`TestManagerStopBoundedWhenTickStuck`/`TestManagerStopSecondCallerBoundedWhenTickStuck` 覆盖「不观察 ctx 的硬卡死」两路径。

活跃 Agent 连接停机回归通过，证明旧 listener 顺序问题得到修复；阻塞调度、总截止、defer 与关池顺序经真实停机入口核对（卡死 tick 有界返回，后续步骤继续执行，非单纯超时日志）。

### R30-08 · P1 · strict 审计错误未到达关键调用方

**VERIFIED — 源码，ARCH-008 重新打开。** `internal/audit/audit.go:Log` strict 会返回错误；`internal/platform/functioninvoke/approval.go:AuditFunctionInvoke/AuditApprovalCreated` 只打印后继续，`internal/api/approval/service.go:recordApprovalAudit` 忽略错误。审计设计 D4 的调用方可见错误/5xx 尚未落地。

SHA-256、全字段规范化、v1 兼容和迁移单测通过，不应重做。需补调用方失败策略与缺口呈现；副作用后的审计失败不能暗示“业务没执行”或引导盲重试。真实审计库故障 NOT RUN。

### R30-09 · P1 · drain 遗漏已接收但尚未进入 Provider RPC 的任务

**已修复（2026-10-31，ARCH-016，3d5312fc）。** 真实 TaskRunner→LocalHandler 诊断中，StartTask 已 ACK、executor 已进入但暂未调用 Provider；Inflight=0 使 DrainProviderSession 摘掉 agentlocal 注册后立即 ACK，释放 executor 后 executeTask→handleInvoke→pickInstance 报未注册。修复：任务续接绕过被清空的 agentlocal，经 session store 固定已接收任务的 Provider 归属；drain 等待 TaskRunner.Count()==0 而非仅 Inflight。已接收但未进入 Provider RPC 的任务不再漏接。

线性化故障验证仍由 ARCH-016 单测/实现覆盖（`count accepted tasks and route continuations through drain`），真实 drain 断连与 Provider 归属的端到端钻机留待 ARCH-018（执行中断与停机演练）故障注入覆盖。

## 可保留的交付及边界

| 任务           | 已核对的范围                                              | 不据此承诺                              |
| -------------- | --------------------------------------------------------- | --------------------------------------- |
| ARCH-001       | D1–D7 方案及 CAS/409 局部测试                             | ARCH-002 的生产 SQL 装配、持久意图/恢复 |
| ARCH-004       | cluster/monitoring/cmd-server 本地存活/就绪与降级         | ARCH-005 双实例故障摘流                 |
| ARCH-006       | 自定义绑定、碰撞、并发和 Forget 用例/生产 resolver        | ARCH-009 的完整开库预算                 |
| ARCH-007       | 版本摘要与写失败策略决策                                  | ARCH-008 全调用链 strict                |
| ARCH-013       | ingest 不支持组合配置门禁                                 | 外部 Kafka 消费者已接通                 |
| ARCH-014       | 在途重复 Start、有限重试、unknown 协议                    | 持久终态/跨重启去重                     |
| ARCH-026       | 生产注入 writer；落库失败阻止 StartTask                   | 整体任务/真实 DB 恢复已验收             |
| ARCH-027       | descriptor 读取错误与无 schema 区分                       | 风险查询无故障降档                      |
| REVIEW-001–004 | 精确余额/未选不变、空白阻断、冲突响应、确认取消及正向入参 | UI-001–003 独立发布态验收               |
| REVIEW-005     | 当前函数已改名 summarizeQuantityTotal                     | 安全整数/最终乘积边界仍缺失             |

## 台账与检查

回流的旧父子任务、阻塞登记和重复审核片段原样保存在 [整理前快照](./todo-import-2026-10-30.md)。有效 C2/E2/D2/D5 验收复用 UI-001/002/003；ARCH-017 的 DONE/READY 双状态已消除。九项完成状态回退，既有实现进度仍保留，新增 DATA-014/015 分开验收。根 TODO 共 78 个稳定任务，12 项仅在注明范围内完成。检查器只验证结构/引用/依赖，不判断业务正确性。

首轮 14 个有效 Go 包通过，但另一包路径误写为 internal/platform/audit，该命令总退出非零；随后按正确 internal/audit 路径补跑，连同 cluster/monitoring/policy/agent/tasks 六包通过，共 20 个有效定向包。Web 六套 124 用例、tsc、dashboard_vnext_guard 通过。race **不全绿**，已有测试后台读取竞争和独立生产聚合字段竞争分别留证。诊断不是真实故障演练；没有全程序编译、生产部署/压测或真实游戏收据验收。

文档构建、SVG XML、浏览器检查和 DAG 的最终结果记录在证据摘要。外部产品沿用此前注明日期/版本的官方核查，本轮不扩展平台研究/选型。

## 最终交付检查

- 20 个有效 Go 定向包通过；首次命令中的一个错误包路径如上保留，不把总退出非零写成全绿。
- Web 6 套 / 124 用例、TypeScript、PageSpec guard 通过。
- 最终归档源重放的 6 个缺陷确认诊断通过；独立聚合 race 预期失败并指向生产字段。常规 worker race 的跨用例变量竞争也保留，没有用过滤掉失败来宣称 race 全绿。
- 文档通过现有 VitePress 1.6.4 CLI 构建。pnpm 脚本先被存储索引权限阻止，提权后又因依赖目录自动准备/no-TTY 退出；采用现有 CLI，没有重装或改动依赖/锁文件。
- SVG XML、站内引用、78 个唯一编号和无环依赖通过；12 项完成均按表内有限范围审阅。
- Chromium 实际渲染 18 张 SVG、12 个架构页面和 8 个分析页面，通过文字重叠/越界/总览连线穿字、资源引用及手机页面溢出检查；人工查看更新的停机时序图。
- 整理前 TODO 快照与基线原文逐字相同；最终业务源码哈希与审核记录一致，本轮未修改业务源码或现有测试。并行开发的 watchdog/探针等修改保留供其作者继续。
