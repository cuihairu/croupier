# TODO

更新：2026-10-30。以源码、失败路径及实际验收证据区分已实现、已验证和待核实；LOCAL VERIFIED 只覆盖注明的本地验收，不等于生产上线。优先级 P0 > P1 > P2。每个复选框对应一个可独立评审/验收的交付；依赖 `—` 不代表没有环境条件，NOT RUN 表示对应验收未执行。

最新结论见 [可靠性复审](docs/review/reliability-recheck-2026-10-30.md)。之前的完成声明与回流旧任务完整保存在 [本轮整理前快照](docs/review/todo-import-2026-10-30.md)；更早原文见 [10 月 29 日历史](docs/review/todo-history-2026-10-29.md)。旧 C2/E2/D2/D5 分别复用 UI-001/002/003，不重开一套父子任务；其已交付历史不删除，只退出当前执行清单。

## 架构与可靠性

- [x] **ARCH-001 · P1 · 方案决策 · 审批并发与执行领取语义**
  - 依赖：—。交付与验收：基于现有 SQLStore 定义 pending 原子转换、续跑领取及重复请求规则；评审表覆盖批准/拒绝竞争、重复批准、执行结果未知。
  - 状态／条件：LOCAL VERIFIED：已审 D1–D7 决策覆盖 CAS、胜出者、冲突、未知结果及持久领取边界；方案交付与 ARCH-002 实现分开。
  - 完成证据：2026-10-30 复审；`docs/design/approval-concurrency-design.md`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-002 · P1 · 实现 · 接入审批 SQL 存储**
  - 依赖：ARCH-001。交付与验收：复用 internal/platform/approvals/sql_store.go；A 创建、B 读取/批准、重启后可查，并发仅一决定且仅一执行领取；不能只改构造器。
  - 状态／条件：LOCAL VERIFIED：`ServiceContext` 已由 MemStore 切到 `NewSQLStore(db)`（`internal/svc/service_context.go:350`）；CAS `pending→approved/rejected`、`ClaimForDispatch` 持久领取、迁移 0050 补列（存量幂等 + 全新建表）；重启重开读回 claim、并发 6 路仅 1 领取且恰好 1 次续跑。
  - 完成证据：2026-10-30；`internal/platform/approvals/sql_store.go`、`internal/svc/service_context.go`、`internal/db/migrate/migrations.go`；`go test ./internal/platform/approvals/... ./internal/api/approval/... ./internal/svc/... ./internal/db/migrate/... ./internal/api/function/... ./internal/api/console/... ./internal/platform/functioninvoke/... ./internal/logic/function/... -count=1` 全绿；定向 `TestClaimForDispatchExactlyOnce/sql_claim_survives_store_reopen`、`TestApproveCov_ConcurrentApprovalRunsExactlyOnce`、`TestApprovalDispatchClaimMigration_*` 通过；commit `7e7d9557`。

- [x] **ARCH-003 · P1 · 实现 · 集群互联身份认证**
  - 依赖：—。交付与验收：增加专用 TLS 配置及身份绑定，拒绝无证书/错 CA/Agent 身份/未握手 forward；合法 invoke/start/cancel 保留 epoch fencing。
  - 状态／条件：LOCAL VERIFIED：`cluster.interconnectTls` 专用证书面（cert/key/ca 齐备且非 insecureSkipTls 时 mTLS）；`VerifyInterconnectPeer`（`internal/cluster/identity.go`）只接受双 EKU（ServerAuth+ClientAuth）server 角色证书、空 CommonName/Agent 证书（仅 ClientAuth）拒绝；`ServeHelloHandler` 标记连接会话、`ServeForwardHandler` 拒绝未握手 forward；合法 invoke/start/cancel 保留 epoch fencing（stale epoch 仍 NotOwner）。
  - 完成证据：2026-10-31；`internal/cluster/identity.go`、`internal/cluster/transport.go`、`internal/cluster/interconnect.go`、`internal/config/config.go`、`cmd/server/cluster.go`、`internal/devcert/devcert.go`；`go test -count=1 ./internal/... ./core/... ./agents/...` 全绿；定向 `TestInterconnect_MTLSFullChain`（无证书/错 CA/Agent 身份拒绝 + 合法 invoke/start/cancel + epoch fencing）、`TestServeForwardHandler_RejectsUnhandshaked`、`TestServeForwardHandler_AcceptsHandshaked`、`TestVerifyInterconnectPeer_*`、`TestStartCluster_InterconnectTLSWiring` 通过；commit `9547afb8`。

- [x] **ARCH-004 · P1 · 实现 · 集群就绪及失败降级语义**
  - 依赖：—。交付与验收：独立存活/就绪探针；配置要求集群时协调库、互联监听或租约不可用返回 ready=503，显式配置才允许 standalone 降级。
  - 状态／条件：LOCAL VERIFIED：本地存活/就绪及显式降级实现和相关包通过；双实例摘流故障仍由 ARCH-005 验收。
  - 完成证据：2026-10-30 复审；`cmd/server/cluster_readiness_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-005 · P1 · 故障验证 · 集群故障摘流与重连**
  - 依赖：ARCH-003, ARCH-004。交付与验收：双实例注入协调库断连、互联占端口、owner 退出、旧 epoch；逐例记录拒绝、摘流、恢复及副作用次数。
  - 状态／条件：LOCAL VERIFIED：真实 `startCluster` 装配双实例钻机，覆盖协调库断连、互联占端口、owner 退出、旧 epoch；逐例断言拒绝、摘流、恢复及副作用次数。
  - 完成证据：2026-11-01；`cmd/server/fault_drill_cluster_test.go`、`cmd/server/cluster.go`（`clusterInvokerOverride` 注入点）；定向运行 `go test ./cmd/server/ -run 'TestDrillCluster_' -count=1` 全绿（4/4 PASS，耗时 140.855s）；commit `bf784cc9`。

- [x] **ARCH-006 · P1 · 实现 · 物理数据库路由统一真值**
  - 依赖：—。交付与验收：复用授权后的 binding.database_name，消除前缀重算；验收旧名/自定义名/前缀变更/下划线碰撞/并发首次开库，已有绑定有显式迁移。
  - 状态／条件：LOCAL VERIFIED：自定义绑定、命名碰撞、并发首次打开及 Forget 用例通过；预算问题不算本项完成，见 ARCH-009。
  - 完成证据：2026-10-30 复审；`internal/db/router/router_binding_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-007 · P2 · 方案决策 · 版本化审计摘要与失败策略**
  - 依赖：—。交付与验收：列出 scope/Changes/Action 等规范化哈希字段、旧版本兼容和写失败策略；外部签名是否需要单列决策。
  - 状态／条件：LOCAL VERIFIED：保留版本化全字段摘要与写失败策略决策；D4 调用方落地尚缺，ARCH-008 已回退。
  - 完成证据：2026-10-30 复审；`docs/design/audit-chain-integrity-design.md`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-008 · P1 · 实现 · 落地审计完整性规则**
  - 依赖：ARCH-007。交付与验收：逐字段修改被检测、旧链可验证、多实例写入一致；审计库故障按已批准策略处理。
  - 状态／条件：LOCAL VERIFIED：版本化哈希/v1 兼容/Log strict 与逐字段篡改检测/多实例链冲突解析已有测试；本项补齐 D4 调用方契约——副作用已执行后审计写失败一律经 `ReportWriteGap` 呈现（WARN + rate-limited 告警），不再 `fmt.Printf`/`_ =` 静默吞掉，也不返回 5xx（5xx 会误导「业务未执行」并引诱盲重试）。真实审计库故障注入 NOT RUN，不据此宣称生产链路已验证。
  - 完成证据：2026-10-31；`internal/audit/audit.go`（ReportWriteGap）、`internal/platform/functioninvoke/approval.go`、`internal/api/approval/service.go`、`internal/api/approval/stepup_otp.go`、`internal/api/alert/inbound.go`、`internal/api/configexplorer/service.go`、`internal/api/console/service.go`、`internal/api/openapi/service.go`、`internal/api/ops/helpers.go`、`internal/api/page/service.go`、`internal/api/resourcecatalog/service.go`、`internal/service/contract_service.go`、`internal/audit/chain_version_test.go`（TestReportWriteGap_SurfacesGapAndRateLimits）、`internal/platform/functioninvoke/audit_gap_test.go`（TestAuditFunctionInvoke_WriteGapSurfaced、TestAuditApprovalCreated_WriteGapSurfaced）；`go test ./internal/... ./core/... ./agents/... -count=1` 全绿，`go build ./...` 过，dashboard vnext/e2e guard PASSED，dependency boundary PASSED，web tsc 过，docs build 过。

- [x] **ARCH-009 · P1 · 实现 · 数据库首次打开的取消与预算**
  - 依赖：—。交付与验收：让 resolver/open/migrate 消费 context；取消等待及时返回、同库 singleflight、不同库隔离，暴露池数/首开耗时；与 ARCH-006 路由命名修复可并行，合并时核对绑定行为
  - 状态／条件：LOCAL VERIFIED：取消等待/ensure/migrate context/singleflight/统计沿用既有实现；本项补齐 R30-06——同步无 context 的 `cfg.Open`（gorm.Open 驱动初始化）改为经 `openWithContext` 在预算 context 内运行，预算到期即返回 deadline 错误，驱动迟到返回的连接被 `closeQuietly` 关闭而非泄漏；随后 PingContext/migrate 仍按预算检查。真实数据库驱动初始化故障注入 NOT RUN，不据此宣称生产链路已验证。
  - 完成证据：2026-10-31；`internal/db/router/router.go`（openWithContext + GameDBCloseLateHook 测试接缝）、`internal/db/router/router_arch009_test.go`（TestGameDB_OpenBudgetBoundsBlockingOpen）；`go test ./internal/db/... ./internal/svc/... ./internal/api/page/... ./internal/api/console/... -count=1` 全绿，`go test -race ./internal/db/router/ -run 'TestGameDB_OpenBudgetBoundsBlockingOpen|TestGameDB_OpenBudgetExceeded|TestGameDB_LifecycleCancelAbortsInFlightOpen' -count=1` 过，`go build ./...` 过。

- [x] **ARCH-010 · P2 · 性能验证 · 多 game/env 连接池容量**
  - 依赖：ARCH-009。交付与验收：在给定 game/env 数、并发、数据库连接配额下记录池数/连接峰值/P95 首开耗时；给出回收阈值与容量结论。
  - 状态／条件：LOCAL VERIFIED：真实 atlas-cov-mysql（127.0.0.1:13307，max_connections=151）与 atlas-cov-pg（127.0.0.1:15432，max_connections=100）上，参数化（games×envs、并发、可选 P95/连接峰值目标）探针全绿：mysql 8 池并发 8 → pool=8/P95≈415ms/clientPeak=8/serverPeak=15/residual=0；pg 8 池并发 8 → pool=8/P95≈368ms/clientPeak=8/serverPeak=15/residual=0；pg 4×3=12 池并发 12（-race）→ pool=12/P95≈697ms/serverPeak=19/residual=0；mysql 72 池并发 72 → pool=72/P95≈462ms/clientPeak=72/serverPeak=83/residual=0。容量结论：`Forget` 回收后服务端连接回落基线（残差 0），database/sql 每池默认空闲 2 连接，mysql 151 配额下理论 poolCapacity≈72、pg 100 配额下≈47，本次 8/12/72 池均 fits=true。
  - 完成证据：2026-11-01；`internal/db/router/cap_probe_test.go`（TestArch010Probe：`ARCH010_DRIVER` 门禁 mysql/postgres，`ARCH010_GAMES/ENVS/CONCURRENCY` 参数化，服务端 `information_schema.processlist`/`pg_stat_activity` 峰值采样，`Forget` 回收残差，容量结论 = `(max_connections - 5% 预留) / idlePerPool`，可选 `ARCH010_TARGET_P95_MS`/`ARCH010_TARGET_MAX_CONNS` 断言）；`go test ./internal/db/router/ -count=1` 全绿（6.0s），`go test -race ./internal/db/router/ -run TestArch010Probe` 过，`go test ./internal/db/... ./internal/svc/... -count=1` 全绿，`go vet ./internal/db/router/` 过，`go build ./...` 过。

- [x] **ARCH-011 · P1 · 实现 · 分析 worker 持久化后确认**
  - 依赖：—。交付与验收：修正 worker.go 批量 Append 尚未 Send 即 XACK、重入队错误未核对的问题；杀进程及写库/入库故障下每条消息可核对，说明允许重复与去重策略。
  - 状态／条件：LOCAL VERIFIED：Send 成功后 ACK、失败保留 pending、重入队/死信 XADD 错误不 ACK（代码修复 5b25eb6d）；真实杀进程/写库故障逐条核对由 ARCH-012 钻机完成（Append后/Send前、Send后/ACK前、重入队失败三故障点，源消息/PEL/死信/ClickHouse 行对账无静默丢失）。去重策略：raw 表无服务端去重，消费端以 event_id/order_id 为去重键；Send后/ACK前崩溃重投会产生重复行（允许，消费端去重），非静默丢失。聚合字段竞态另由 DATA-014 修复。
  - 完成证据：2026-10-31；`internal/analytics/worker/worker.go`（deferred XACK：append 记录 ackRef、flush 后 ackSent；重入队/死信 XADD 错误不 ACK）、`internal/analytics/worker/arch011_test.go`（TestAppendEventRow_RecordsAckRef、TestFlushEventBatch_AcksRefsAfterSend、TestProcessMessage_RequeueFailureNoAck、TestProcessMessage_DeadLetterFailureNoAck、TestAdvanceCheckpoint_Monotonic）、`internal/analytics/worker/arch012_drill_test.go`（TestDrill_AppendThenCrash_BeforeSend_ReclaimNoLoss、TestDrill_SendThenCrash_BeforeAck_ReclaimNoLoss、TestDrill_RequeueFailure_MessageStaysPending）；`go test ./internal/analytics/... -count=1` 全绿，真实 Redis+ClickHouse 钻机全绿，`go build ./...` 过。

- [x] **ARCH-012 · P1 · 故障验证 · 分析管道崩溃恢复**
  - 依赖：ARCH-011。交付与验收：在 Append 后/Send 前、Send 后/ACK 前、重入队失败时中断；比对源消息、PEL、死信与 ClickHouse 行，证实无静默丢失。
  - 状态／条件：LOCAL VERIFIED：真实 Redis（redis-server / ARCH012_REDIS_ADDR）+ 真实 ClickHouse（ARCH012_CLICKHOUSE_DSN，需 analytics 库与 001_init.sql 建出的 events/payments 表）三故障点钻机全绿——(1) Append 后/Send 前崩溃：消息保持 PEL，重启 reclaim 重投 → 重新 append → flush → ClickHouse 行 + ACK，无丢失；(2) Send 后/ACK 前崩溃：行已落库、消息仍 PEL，重启 reclaim 重投 → 重复行（允许，消费端 event_id 去重）→ ACK，无丢失；(3) 重入队失败：消息不被 ACK、保持 PEL 供 reclaim 重试，无死信无丢行。缺可控环境时 t.Skip（边界：需 Redis 与 ClickHouse 可控环境）；CI 经 ci-analytics.yml 的 arch012-drill job（compose redis+clickhouse）强制执行。
  - 完成证据：2026-10-31；`internal/analytics/worker/arch012_drill_test.go`（TestDrill_AppendThenCrash_BeforeSend_ReclaimNoLoss、TestDrill_SendThenCrash_BeforeAck_ReclaimNoLoss、TestDrill_RequeueFailure_MessageStaysPending）、`.github/workflows/ci-analytics.yml`（arch012-drill job）；真实 Redis+ClickHouse 下 `go test ./internal/analytics/worker/ -run 'TestDrill_' -count=1` 全绿，`go build ./...` 过。

- [x] **ARCH-013 · P2 · 实现 · 阻止不支持的分析组合**
  - 依赖：—。交付与验收：配置/启动预检识别 Kafka 发布配内置 Redis-only worker；默认 Redis 链不受影响，不扩建 Kafka consumer。
  - 状态／条件：LOCAL VERIFIED：不支持的 Kafka/noop 组合配置门禁及相关包通过；不是外部消费者集成验收。
  - 完成证据：2026-10-30 复审；`cmd/ingest/cmd/root_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-014 · P1 · 方案决策 · 异步任务重试与终态恢复协议**
  - 依赖：—。交付与验收：基于 sidecar TaskRunner 定义同 taskId 重复 Start、事件 ACK/持久重放及 Agent 重启后的结果未知语义；不把页面幂等等同游戏副作用去重。
  - 状态／条件：LOCAL VERIFIED：已定义在途重复 Start、有限终态重试、unknown 及游戏收据边界；运行时缺口见 ARCH-015/018。
  - 完成证据：2026-10-30 复审；`docs/design/task-retry-recovery-protocol.md`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-015 · P1 · 实现 · 任务可靠事件与重复启动约束**
  - 依赖：ARCH-014。交付与验收：按已评审协议收口 TaskRunner 重复 Start 和事件重放/终态收敛，复用 task_runs/task_events；验证旧协程清理不删除同 ID 新任务，取消不回退终态；派发前落库失败由 ARCH-026 单独处理。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。在途重复 Start（D1 generation 去重）、终态有限重试（D2）与会话丢失/`unknown` 收割（D3）已实现；R30-03 部分写入缺口闭合——`handleTaskEvent` 以单一事务（`RecordTaskEvent`）持久化 guard 状态更新与事件追加，事件写失败整体回滚、来源恢复后重试收敛，不另建任务体系。详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。
  - 完成证据：2026-10-31；`internal/model/task_model.go` 新增 `TaskRunModel.RecordTaskEventTransactional`（单一事务：guard 更新 + seq 计算 + 事件追加，任一失败整体回滚）；`internal/tasks/store.go` 新增 `Store.RecordTaskEvent`；`internal/server/control_handler.go` `handleTaskEvent` 改调 `RecordTaskEvent`（`TaskStore` 接口收敛为该方法）。`go test ./internal/... -count=1` 全绿；新增 `TestControlService_HandleTaskEvent_RealStore_NoPartialWrite`（事件表缺失 → 报错且行保持 queued、无部分写入；表恢复 → 重试补写 seq=1 并收敛 failed）、`TestTaskRunModel_RecordTaskEventTransactional`/`_RollsBackOnAppendFailure`。边界诚实：task API `/tasks/cancel` 的 UpdateRunIfStatusNotIn+AppendEvent 仍非单事务，但 cancel_requested 非终态、重试可收敛（至多重复意图事件，非缺失），未纳入本次修复。

- [x] **ARCH-016 · P1 · 实现 · Provider drain 真正摘流**
  - 依赖：—。交付与验收：当前 handleDrain 仅记录并回空 ACK；实现停止新路由、等待在途及超时退出语义，验证 ACK 语义与执行结束条件。
  - 状态／条件：LOCAL VERIFIED：Draining/RPC 在途计数/路由门/宽限窗沿用既有实现；本项补齐 R30-09——(1) `handleInvoke` 对任务延续（taskId 存在且会话存储已装配）跳过读 agentlocal 的 `pickInstance`（drain 已摘除该表），改由 `callProvider` 按会话存储选路，draining 会话对延续放行、无命中明确失败，不再误报未注册；(2) `DrainProviderSession` 等待会话在途归零外，还等待 `TaskRunner.Count()==0`（覆盖「已 ACK 未进入 Provider RPC」窗口，该窗 Inflight=0 但任务仍在），宽限到点后不取消在途、留给 ARCH-015 D3 收敛。真实多实例原子准入竞争注入 NOT RUN。
  - 完成证据：2026-10-31；`internal/agent/local_handler.go`（handleInvoke 任务延续跳过 pickInstance、callProvider 无会话命中明确失败、DrainProviderSession 等待 TaskRunner.Count）、`internal/agent/provider_drain_test.go`（TestCallProvider_TaskContinuationAfterDrainNoAgentLocal、TestHandleInvoke_TaskContinuationSkipsPickInstanceAfterDrain、TestDrainProviderSession_WaitsForAcceptedTasks）；`go test ./internal/agent/... ./agents/sidecar/... ./internal/app/agent/... -count=1` 全绿，`go test -race ./internal/agent/ -run 'TestCallProvider_TaskContinuationAfterDrainNoAgentLocal|TestHandleInvoke_TaskContinuationSkipsPickInstanceAfterDrain|TestDrainProviderSession_WaitsForAcceptedTasks|TestCallProvider_InflightCountedAroundCall|TestDrainProviderSession_WaitsForInflightZero' -count=1` 过，`go build ./...` 过。

- [x] **ARCH-017 · P1 · 实现 · Server 停机收尾顺序**
  - 依赖：—。交付与验收：控制 listener.Close 等活跃会话时不阻塞后续 HTTP 停机/根取消；保证 ServiceContext.Close 冲刷留痕，设置有界清理并验证 socket/goroutine/池释放。
  - 状态／条件：LOCAL VERIFIED：停收→HTTP drain→rootCancel→等待→Close 正常路径回归沿用既有；本项补齐 R30-07——`scheduler.Manager` 新增生命周期 context（Start 创建、Stop 取消），`tick` 由 `context.Background()` 改用该 context；阻塞的 store/dispatch 调用被 Stop 取消，循环及时退出，`Stop` 有界返回，runServer 的 defer StopScheduler 不再在 30s 停机预算外挂住。补硬界兜底与真实停机入口故障注入：`Stop` 对两条调用路径统一 `waitStopped(stopTimeout)`（默认 5s，`SetStopTimeout` 覆盖），不观察 ctx 取消的硬卡死 tick 也超时返回（不把超时当资源已释放，循环退出仍由 stoppedCh 保证）；`SetTickHook` 测试缝；`StartScheduler` 不再自动 Start（root.go 注入治理/钻机钩子后显式 Start）。真实停机入口钻机验证阻塞调度、总截止、defer 与关池顺序（SIGINT→有界返回、HTTP/control listener 拒绝新连接）。
  - 完成证据：2026-10-31；`internal/tasks/scheduler/manager.go`（lifecycleCtx/lifecycleCancel、tick 用 lifecycle ctx；`stopTimeout`/`waitStopped`/`SetStopTimeout`/`SetTickHook`）、`internal/tasks/scheduler/manager_stop_test.go`（TestManagerStop_AbortsBlockedTick + blockingStore；TestManagerStopBoundedWhenTickStuck、TestManagerStopSecondCallerBoundedWhenTickStuck、TestManagerStopUnblocksBeforeTimeoutWhenTickHealthy）、`internal/svc/scheduler.go`（StartScheduler 不再自动 Start）、`cmd/server/root.go`（drillSchedulerHook 注入、显式 Start）、`cmd/server/fault_drill_r28_test.go`（TestDrill_BlockedScheduler_ShutdownBoundedAndCloseRuns：真实停机入口 SIGINT，卡死调度 tick 下 30s 内有界返回、HTTP/control listener 拒绝新连接）；`go test ./internal/tasks/... ./internal/svc/... ./cmd/server/... -count=1` 全绿（含钻机 3.48s），`go build ./...` 过。

- [x] **ARCH-018 · P1 · 故障验证 · 执行中断与停机演练**
  - 依赖：ARCH-015, ARCH-016, ARCH-017, ARCH-026。交付与验收：执行中注入断链、drain、SIGTERM；逐项记录新请求拒绝、终态/未知态、截止时间及资源释放；不把测试 fixture 的游戏副作用结果当真实游戏恢复凭据
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。D3 看门狗收割器随附（7da8d2d2：`runTaskWatchdog` 后台循环 + `ReapStale`，预算 `taskLog.reaperTimeout` 默认 5m）；执行中断钻机（1a2be73a：`cmd/server/fault_drill_r27_test.go`，真实 runServer boot + e2e-agent-probe 子进程）四场景全绿：断链→unknown+新请求 503；drain→在途完成+新请求 503+undrain 恢复派发；`-hang` 看门狗→预算内 timed_out+finishedAt 置位+新请求仍 200（agent 存活，非断连所致）；SIGTERM→优雅停机返回 nil+在途 unknown+HTTP 监听释放。已知边界：探针副作用计数是测试 fixture 非游戏恢复凭据，不断言游戏 exactly-once（GAME-010 仍 NOT RUN）；重连/迟到终态 soft-terminal 收敛未在钻机断言（ARCH-014 D3 设计域）。
  - 完成证据：2026-10-31；`internal/server/control_handler.go`（runTaskWatchdog）、`internal/model/task_model.go`（ReapStale）、`internal/tasks/store.go`（ReapStale 代理）、`internal/config/config.go`（reaperTimeout）、`cmd/server/fault_drill_r27_test.go`（四场景钻机）；`go test ./cmd/server/ -run 'TestDrill_' -count=1` 全绿；`go build ./...` 过；dashboard guard PASSED。

- [x] **ARCH-019 · P2 · 实现 · 执行领域依赖方向收敛**
  - 依赖：—。交付与验收：复用 assignment gate 下沉领域包，functioninvoke 不再 import internal/api；core/agents 的依赖边界检查可执行且原行为一致。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。闸门下沉（72fa504f）：`EnsureFunctionAssigned` 自 `internal/api/assignment/gate.go` 迁至 `internal/logic/assignment/gate.go`（复用既有 `LoadAllAssignments`，语义不变；用例随迁 `gate_test.go` + `gate_error_r52_test.go` 全绿）；`functioninvoke/govern.go` 改 import `internal/logic/assignment`，执行链不再 import `internal/api`（仓内仅余 `internal/handler/routes.go` 的 API handler 注册合法引用）。依赖边界检查可执行：`scripts/check_dependency_boundary.sh`（core/agents 生产代码禁 import `internal/` + functioninvoke 禁 import `internal/api`，含扫描路径存在断言防静默 no-op）+ 合成违规自测 `scripts/check_dependency_boundary_test.sh`，已接线 ci.yml。原行为一致：`go build ./...` 过，三包测试全绿，dashboard guard PASSED，docs build 过。
  - 完成证据：2026-10-31；`internal/logic/assignment/gate.go`、`internal/logic/assignment/gate_test.go`、`internal/logic/assignment/gate_error_r52_test.go`、`scripts/check_dependency_boundary.sh`、`scripts/check_dependency_boundary_test.sh`、`.github/workflows/ci.yml`；`go build ./...` 过；dashboard guard PASSED。

- [x] **ARCH-020 · P2 · 实现 · 架构文档防漂移检查入 CI**
  - 依赖：—。交付与验收：复用 scripts/docs/check-architecture.py、check-architecture.cjs 与生成器，核对 SVG、路径、首页资源和 TODO DAG；新链接失败即报告，避免直接开启全站 strict 阻断旧页面。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。check-architecture.py 已接线 docs-check.yml（`:60` 步；触发路径补 `todo.md`+`scripts/docs/**`）；干净树 PASS（78 TODO ID 无环、SVG XML、作用域链接/证据路径、README 图引用）；计数规则改为只数结构化任务项（审核发现 等非任务 checkbox 不再误报）；新链接失败即报告、作用域限定未开全站 strict。边界诚实：check-architecture.cjs 为手动 Playwright 布局工具，需已构建 docs，未入 CI。
  - 完成证据：2026-10-31；`scripts/docs/check-architecture.py`、`.github/workflows/docs-check.yml`、`docs/development/lifecycle.md`；`python3 scripts/docs/check-architecture.py` 干净树 PASS（78 TODO ID 无环）；docs build 过；dashboard guard PASSED。

- [x] **ARCH-021 · P1 · 实现 · 扩展同步身份与 scope 接入**
  - 依赖：—。交付与验收：复用现有 HTTP puller/版本 payload 与 protected 路由，为 Agent 建立受限机器身份及 scope 授权；无凭据/跨 scope 拒绝，合法 Agent 实际拉取生效且坏版本保持 last-good。证据：internal/app/agent/extension_sync_puller.go 未设认证头，internal/handler/routes.go 受 protected 保护，AgentSyncPayload 检查读取权限；不通过取消服务端授权绕过。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。受限机器身份（X-Croupier-Agent-Token）已接入：中间件将机器 token 限定在扩展同步端点（其它路径 403），AgentSyncPayload 校验 agentID scope（跨 scope 403、无凭据走 RBAC 被拒）；合法 Agent 拉取生效（PullOnce 携带头、版本 payload 应用），坏版本 last-good 由 configsync 承担；签发命令 `croupier-server token agent-sync`（ResolveSecret+SignMachine round-trip 已验证）。
  - 完成证据：2026-10-31；`internal/security/jwtutil/machine_token.go`、`internal/svc/service_context.go`、`internal/api/extension/service.go`、`internal/app/agent/extension_sync_puller.go`、`cmd/server/token.go`、`internal/svc/machine_auth_test.go`、`internal/api/extension/machine_scope_test.go`、`internal/app/agent/extension_sync_puller_test.go`；定向运行 go test（jwtutil/svc/extension/agent/cmd-server）全绿，go build ./... 过。

- [x] **ARCH-022 · P2 · 实现 · 注册与重连的显式请求预算**
  - 依赖：—。交付与验收：复用 core/register，为 DialAndRegister/SyncOnce/SyncAttempts 明确一致的注册超时边界及调用者 deadline 优先级；核对心跳 tick 重拨与首拨退避差异，验证无注册响应时按预算退出、Stop 可释放连接与 goroutine，不引入重复并行重连。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。注册预算已统一：registerCtx 对每次注册调用（DialAndRegister/SyncOnce/SyncAttempts）应用 requestTimeout（默认 10s），调用方 deadline 更早时以调用方为准；挂起 Register 无响应按预算退出（测试锁定 ~60ms 预算返回）；心跳 tick 重拨与首拨退避差异核清——首拨失败走 RunReconnectLoop 指数退避 5→60s，心跳恢复在 tick 内直接重拨（已被心跳间隔限频，不再叠加退避），属有意设计。Stop 经 lifecycleCancel+lifecycleWG.Wait 释放连接与 goroutine；dialInflight CAS 防重复并行重连。
  - 完成证据：2026-10-31；`core/register/registration.go`、`core/register/loops.go`、`core/register/register_budget_test.go`；定向运行 go test ./core/register/ ./internal/app/agent/ ./agents/capture/... -count=1 全绿，go build ./... 过。

- [x] **ARCH-023 · P0 · 实现 · 定时触发先领取再派发**
  - 依赖：—。交付与验收：复用 task_schedule_run_logs 唯一键，派发前原子领取 schedule+slot 并明确领取租约/崩溃恢复；多实例同槽仅一派发，记录失败不得执行；覆盖领取后/派发前和派发后/回写前中断，业务最终副作用去重仍由 GAME-010 承担
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。跨槽陈旧领取已由调度周期统一扫描（reapStaleClaims → ListStaleClaims(created_at<now-claimLease, status=claimed)）解析为 indeterminate——run-log 不再永久停在 claimed；租约过期的 claimed 不再记确定 failed（派发后/回写前中断与派发前中断不可区分，indeterminate 是唯一诚实终态）；绝不重派发。同槽 resolveStaleClaim 同步改 indeterminate。web 运行记录状态集扩至 claimed/indeterminate/approval_required 并新增 orange 标签（派发结果未知不得误读为确定失败）。
  - 完成证据：2026-10-31；`internal/tasks/scheduler/manager.go`、`internal/model/task_schedule.go`、`internal/model/task_schedule_model.go`、`internal/tasks/scheduler/arch023_test.go`、`internal/model/task_schedule_model_test.go`、`web/src/pages/Ops/Schedules/index.tsx`、`web/src/services/api/schedules.ts`；定向运行 go test ./internal/tasks/... ./internal/model/ ./internal/svc/ ./internal/server/ ./internal/api/schedule/ -count=1 全绿，web tsc 过，dashboard guard PASSED，docs build 过。

- [x] **ARCH-024 · P0 · 实现 · 任务及调度入口统一执行治理**
  - 依赖：—。交付与验收：让 /tasks Start、schedule Create/TriggerNow/自动触发复用可注入的执行治理服务；不把模块级 JWT/scope 误作函数政策。对同一高危函数，普通/页面/任务/手动调度/定时入口一致执行禁用/分配/函数权限/政策审批/输入契约；自动化主体和批准快照显式定义，撤权后不继续裸派发
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。Govern 链在主体加载后统一调用 CheckInvokePermission（显式函数 ACL FunctionActionAllowed + function:invoke 兜底，HasAdminRole 旁路，nil FunctionModel fail-closed 拒绝）——普通/任务/手动/自动入口同一条链，ACL 拒绝 → 零派发、先于审批创建（不制造审批噪音）；与 /tasks Start 的显式 ACL 检查不再漂移。详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。
  - 完成证据：2026-10-31；`internal/platform/functioninvoke/govern.go` 在主体加载后统一调用 `CheckInvokePermission`（显式函数 ACL `FunctionActionAllowed` + `function:invoke` 兜底，HasAdminRole 旁路；nil FunctionModel fail-closed 拒绝）——普通/任务/手动/自动入口同一条链，ACL 拒绝 → 零派发、先于审批创建（不制造审批噪音）。`go test ./internal/platform/functioninvoke/... ./internal/api/schedule/... ./internal/api/task/... ./internal/api/function/... ./internal/api/approval/... ./internal/api/console/... ./internal/logic/function/... -count=1` 全绿；新增 `TestGovern_ExplicitFunctionAclDenied`/`TestGovern_ExplicitFunctionAclAllowed`（operator 主体 + 显式 per-function 规则：仅 auditor 规则 → 403 零派发；operator 规则 → 放行）；入口测试 env 同步补 `FunctionModel` + 调用者权限（schedule/approval/console），使 TriggerNow/批准续跑/绑定执行能走到派发路径。

- [x] **ARCH-025 · P0 · 实现 · 策略查询和初始化故障禁止静默放行**
  - 依赖：—。交付与验收：区分无覆盖策略与策略存储/配置故障；高风险执行无法获得有效政策时拒绝或使用明确批准的 last-good，不返回 nil,nil 绕过审批/审计；覆盖 GetPolicy SQL 错误及 PolicyManager 初始化失败
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。GetPolicy SQL 错误 fail-closed；PolicyManager 初始化失败回退内置默认档（high/danger 仍需审批，治理保持强制，错误记日志）；契约风险读取故障与未声明（ErrRecordNotFound）已分开——来源故障向上传播，治理链 fail-closed 拒绝、不建审批，approval step-up 门槛视作高危要求二次验证，绝不静默降档。详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。
  - 完成证据：2026-10-31；`internal/platform/functioninvoke/policy.go` 的 `resolvePolicyRisk` 改为返回 `(policy.RiskLevel, error)` 并区分未声明与来源故障，`EnforceFunctionPolicy` 传播该错误；`internal/api/function/helpers.go` 的 `ResolvePolicyRisk` 同步签名；`internal/api/approval/stepup_otp.go` 的 `approveStepUpRisk` 在来源故障时 fail-closed 视作高危。`go test ./internal/... -count=1` 全绿（164 包）；新增 `TestGovern_ContractRiskReadErrorDenies`（function_contracts 表删除 → Govern 拒绝、零审批）、`TestResolvePolicyRisk_ContractSourceFailure_V9`、`TestApproveStepUpRisk_SourceFailureFailsClosed`。边界诚实：`RegistryStore.GetOpenAPI` 为内存查找，其唯一错误是未命中（未声明），保持回落 medium；PolicyManager 初始化失败采用既有「回退内置默认档」策略而非禁用治理。

- [x] **ARCH-026 · P1 · 实现 · 任务主记录持久化失败时禁止派发**
  - 依赖：—。交付与验收：Dispatcher 在 CreateRunWithMeta 失败时返回可定位错误并不得发送 StartTask；验证 DB 失败时执行器计数为 0、无孤立事件/不可取消任务；不与 Agent 事件重放重复实现
  - 状态／条件：LOCAL VERIFIED：生产 TaskRunWriter 已注入，落库失败返回且不发送 StartTask；dispatch 定向包通过。
  - 完成证据：2026-10-30 复审；`internal/platform/dispatch/dispatcher_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-027 · P1 · 实现 · 输入契约读取失败的明确拒绝语义**
  - 依赖：—。交付与验收：区分旧函数确无 schema 与 descriptor 查询失败；契约读取故障不得作为跳过验证依据；固定已批准快照/版本，覆盖 schema DB 失败、required 缺失和版本漂移；复用 ValidateInvokeInput
  - 状态／条件：LOCAL VERIFIED：descriptor 查询错误拒绝、无 schema 与读取失败区分；相关包通过，风险查询故障另见 ARCH-025。
  - 完成证据：2026-10-30 复审；`internal/platform/functioninvoke/govern_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

## 页面链验收

- [ ] **UI-001 · P1 · 故障验证 · 真实游戏页面发布全链（旧 C2）**
  - 依赖：GAME-002, GAME-010, ARCH-024。交付与验收：独立真实游戏环境走契约→模板→编辑保存→接受提案→发布→binding 执行；验证列表/详情/弹窗/刷新、筛选/契约变化/人工草稿隔离，记录落库快照与游戏调用凭据
  - 状态／条件：BLOCKED（2026-11-02 复核，HEAD 806b67c3 后复验）：仍无真实游戏环境（configs/games.json 仅 default/tower_defense/croupier-demo 演示占位，见 configs/games.json:10/26/41）；本机唯一真实游戏栈 Atlas（docker 容器 atlas-demo-mmorpg-atlas-1）为游戏基础设施控制面而非游戏后端，明确不拥有权威角色/战斗/经济/账号数据（/home/cui/workspaces/atlas/README.md:51/53/59），不能充当权威游戏环境；GAME-002、GAME-010 仍 BLOCKED（todo.md:166/198，需真实游戏权威玩家函数/游戏侧持久凭据与受控写环境），ARCH-024 已完成（todo.md:124）。demo 验收不能代替此项；本项复验仅确认机制侧门禁绿（web gameOpsPublishFixture/gameOpsCrudFixture 2 套件 7 用例 PASS、web tsc PASS、dashboard_vnext_guard PASSED、go test 定向包 PASS），不构成全链验收。缺真实游戏环境/GAME-002 完成。

- [x] **UI-002 · P1 · 故障验证 · 示例异常状态全链（旧 E2）**
  - 依赖：—。交付与验收：复用现有 Go gameops-demo 的 33 函数及内存状态，独立 Server/sidecar/demo 栈验证发布、审批、report.generate、失败/未知/取消/幂等重试；既有 fixture 只证机制，不证明进程重启后游戏侧持久去重
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：修复 gameops-demo 契约驼峰缺陷（`batch_ban/batch_recharge/batch_unban`），agent 注册从 3 函数 rebuild 失败变为 34 函数 `warnings=0`；`configs/games.json` 添加 `croupier-demo` scope 后搭起本仓自洽 Server(18881)+sidecar-agent(19096)+gameops-demo 栈，实测 report.generate 全链：成功 `started→completed/succeeded`、失败 `result.error="report_generate_failed"`、取消 `started→cancel_requested→cancelled`（5s delay 被 243ms 中断）。进程重启持久去重已补齐：demo `-state-file` 持久化 reportRunSeq，实测重启后 `report-demo-1→report-demo-2` 序列延续；单测 `TestGameOpsReportRunSeqPersistsAcrossRestart` 断言三次重启序列不重置。审批链说明：demo 唯一 task 函数 `report.generate` risk=warning，不触发 ApprovalRequired；审批门语义由 ARCH-024 政策层与 functionInvoke 契约验收覆盖，本项以审批路由未误触发（task 响应无 approvalId）为界。页面 UI 发布全链（契约→模板→提案→发布→binding）归 UI-001 真实环境验收。
  - 完成证据：2026-11-01；`sdks/go/examples/gameops-demo/store.go`（statePath 加载/持久化 reportRunSeq）、`executors.go`（递增后持久化）、`main.go`（`-state-file` flag）、`report_test.go`（`TestGameOpsReportRunSeqPersistsAcrossRestart`）；`configs/games.json`（croupier-demo/local-demo）；实测栈：`go build ./examples/gameops-demo` 后启动 server(18881)+agent(19096)+demo，`POST /api/v1/tasks` 依次触发成功/失败/取消，重启 demo 后 `report-demo-2` 延续；`go test ./examples/gameops-demo/ -count=1` PASS。

- [x] **UI-003 · P1 · 故障验证 · 批量确认与默认呈现实页验收（旧 D2/D5）**
  - 依赖：UI-002。交付与验收：复用 perTarget、字段语义和五模板；跨页选人、筛选变化、总量确认与实际提交一致，发布回读保留身份列/单位/时区/枚举；不重复规划已实现 UI。
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：代码侧 2026-10-31 收口（合并确认框、MAX_SAFE_INTEGER/乘积溢出 fail-closed、players 补偿空目标 fail-closed，`resourceBatchSelection.test.tsx` 覆盖跨页选人/筛选变更/清除/翻页不误报/目标预填/确认内容与提交一致）；本次补齐发布态 demo 栈实页验收，确认列表身份列/时区/金币单位、批量充值二次确认（每人 100 金 × 2 = 200 金，目标 `20001、20002`）与实际执行一致；修复批量 binding 的 canonical lowercase function ID、补 create/update/delete selector、补 selection `pick` transform 防止 `playerIds` 丢失。
  - 完成证据：2026-11-01；`sdks/go/examples/gameops-demo/contracts.go`（`player.batchban/batchunban/batchrecharge`）、`web/src/components/PageRenderer/templates/playerManage.tsx`（lowercase IDs、create/update/delete selectors、integer schemas、selection `pick` transform）、`gameOpsTemplateCapabilities.test.ts`（binding ID lowercase 断言）；发布版本 7 实测 snapshot 含 3 个 selection binding `transform={"type":"pick"}`；Playwright `/tmp/ui003.playwright.config.ts` 1 passed，执行返回 HTTP 200 `{"success":true,"count":2,"total":200}`；`go test ./examples/gameops-demo/... -count=1` PASS、`pnpm test -- gameOpsTemplateCapabilities playerManage gameOpsPublishFixture resourceBatchSelection compositeBatchTotal` 6 suites/69 tests PASS、`pnpm run tsc` PASS、`git diff --check` PASS。

## 真实游戏接入

- [ ] **GAME-001 · P1 · 资料调查 · 真实游戏权威数据与执行契约**
  - 依赖：—。交付与验收：向接入方整理玩家/角色/封禁/订单/邮件/活动权威读写入口、身份映射、凭据与测试目标；核对现有 ownership/high-risk 设计，交付缺口矩阵。
  - 状态／条件：BLOCKED（2026-11-02 复核）：需接入方真实游戏契约/样本/测试账号；本沙箱无真实游戏接入方。缺接入方契约/样本/测试账号。

- [ ] **GAME-002 · P1 · 实现 · 接入真实玩家工作台**
  - 依赖：GAME-001, GAME-008。交付与验收：复用现有玩家模板和发布链，列表/详情/操作以权威游戏回读及执行凭据验收，失败不显示成功。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需目标游戏环境（权威玩家函数）；缺真实游戏环境。

- [ ] **GAME-003 · P1 · 实现 · 接入真实邮件与补偿**
  - 依赖：GAME-001, GAME-008, GAME-010, GAME-012, ARCH-024。交付与验收：复用模板/审批/任务，按对象凭据核对发放、部分失败重试和重复操作不重发。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需目标游戏环境及受控写权限；缺真实游戏环境/受控写权限。

- [ ] **GAME-004 · P1 · 实现 · 接入真实封禁管理**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用封禁模板，身份/期限/原因批准快照一致，封禁解封后权威回读可核对。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需目标游戏环境及受控写权限；缺真实游戏环境/受控写权限。

- [ ] **GAME-005 · P1 · 实现 · 接入真实订单核查**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用订单模板，订单/发货权威状态一致，补发有凭据且重试不重复发货。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需测试订单与受控补发权限；缺真实测试订单/受控补发权限。

- [ ] **GAME-006 · P1 · 实现 · 接入真实活动配置**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用活动模板，发布/回读版本一致，失败保留旧版本并有审计。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需测试配置和发布权限；缺真实活动配置/发布权限。

- [ ] **GAME-007 · P1 · 方案决策 · 评审游戏权威归属与统一目标引用**
  - 依赖：GAME-001。交付与验收：复用已有 game-authority-ownership 和 player-identity-target-model 设计，决定本地 Player/Balance 的演示/副本定位以及账号/角色/区服/渠道/合服映射来源；交付批准的映射与迁移边界，禁止另建平台游戏经济真值
  - 状态／条件：BLOCKED（2026-11-01 复核）：既有设计仍待评审（用户拍板批准）；不能把文档已完成当业务已实现。缺评审批准。

- [ ] **GAME-008 · P1 · 实现 · 统一游戏目标引用与歧义阻断**
  - 依赖：GAME-007。交付与验收：实现已批准的 TargetRef 解析并供页面/审批/任务/审计复用；同角色 ID 多区服、账号多角色、合服旧引用明确解析，歧义拒绝，不以 gameId/env 或主机代替业务区服
  - 状态／条件：BLOCKED（2026-11-01 复核）：需 GAME-007 方案批准后实施 + 接入方身份映射样本；缺方案批准/身份映射样本。

- [ ] **GAME-009 · P0 · 方案决策 · 评审高风险操作凭据与未知结果协议**
  - 依赖：GAME-001。交付与验收：复用 high-risk-operation-contract-design，决定稳定 operationId、参数/目标指纹、批准快照、游戏持久去重与收据查询职责；明确响应丢失后先查凭据及重试/取消/补偿边界，不重建与 taskId/page idempotency 重复的系统
  - 状态／条件：BLOCKED（2026-11-01 复核）：设计存在但实现协议未批准（用户拍板）；禁止将平台去重当游戏 exactly-once。缺协议批准。

- [ ] **GAME-010 · P0 · 实现 · 游戏持久执行凭据与未知结果收敛**
  - 依赖：GAME-009, GAME-008。交付与验收：按批准协议贯通游戏适配器、执行结果和凭据查询；同 operationId 改参数拒绝、响应丢失/服务重启不重复发奖、未知结果可查询收敛；取消不谎称回滚，补偿引用原凭据
  - 状态／条件：BLOCKED（2026-11-01 复核）：需游戏方持久去重/查询实现与受控写环境；Go demo 的内存记录不能替代。缺游戏方实现/受控写环境。

- [ ] **GAME-011 · P1 · 方案决策 · 评审批量运营冻结与预算恢复规则**
  - 依赖：GAME-009。交付与验收：复用 batch-operations-compensation-design，评审名单快照、预演人数/预算、审批后变化处理、单位/倍率、速率并发、失败阈值、暂停恢复和逐对象凭据；冻结后的对象/金额变更重新审批
  - 状态／条件：BLOCKED（2026-11-01 复核）：批次方案待评审批准（用户拍板）；前端跨页选择和 perTarget 确认不等于服务端批次协议。缺方案批准。

- [ ] **GAME-012 · P1 · 实现 · 服务端批次对象账本与受控恢复**
  - 依赖：GAME-011, GAME-010。交付与验收：复用任务体系实施批准的快照/逐对象状态与预算约束；变更名单不可偷换批准范围，部分失败只重试可确认未执行对象；暂停恢复/阈值停止/最终核对可验，不以一次 onExecute 返回成功代替批次完成
  - 状态／条件：BLOCKED（2026-11-01 复核）：需批准的批次方案和游戏执行收据；缺方案批准/游戏执行收据。

## Agent 与隔离

## 运维分栏（OPS 系列，2026-11-01 用户令立项：游戏运维平台扩展；供给引擎不自研，集成现成项目）

- [x] **OPS-010 · P1 · 实现 · 节点资源指标与进程清单采集（OPS 系列前置，2026-11-01 盘点确认缺失）**
  - 依赖：—（OPS 系列前置项）。
  - 现状盘点：agent 采集对游戏数据足够深（CDC/函数观测/拨测），但无 CPU/内存/磁盘/网络指标、无进程与端口清单、心跳无资源读数——调度（OPS-001/003）与健康门禁（OPS-004）的数据前提不存在。
  - 交付与验收：agent 周期采集（默认 15s，可配）节点指标（CPU%、内存 used/total、磁盘各挂载点 used/total、网络收发速率）+ 进程清单（名称/pid/内存/监听端口）+ 端口列表；随心跳或独立批量上报；服务端入库（保留策略：原始 7 天、聚合 90 天）；面板出节点资源曲线。
  - 实现约束：跨平台无外部依赖（gopsutil 或等价）；采集关闭时可配；指标字段与 OBS-AGENT-001 的 OTel 语义对齐（避免两套字段名）。
  - 落位：实现为共享采集库，优先挂在 devops agent（运维角色，其会话执行记录/execlog/回放统计是现成的操作面数据，与资源指标互补成完整运维数据集）；capture/sidecar/synthetics 后续按需接入同一库。（2026-11-01 用户确认 devops agent 已有基础数据收集：会话记录+execlog+回放统计=审计与 runbook 输入；缺的资源指标由本项补齐）
  - 验收：Linux+Windows 真机各一台指标上报；数值与 top/任务管理器抽样对账误差 <10%；关闭采集后零上报流量。
  - 状态／条件：LOCAL VERIFIED（2026-11-01，边界见 [交付报告](docs/review/2026-11-01-ops-010-node-metrics.md) §5）：采集面（CPU/内存/磁盘/网络收发速率 + 进程清单 + 端口列表，gopsutil v4 无外部依赖）、入库（原始 7 天 + 迁移 0051 建 `agent_metrics_hourly` 小时聚合 90 天 + RollupHourly/PruneHourly）、history API `window=raw|hourly`、面板网络速率曲线 + 进程/端口清单表全部落地；`MetricsEnabled=false` 零上报由上报循环不启动保证；默认间隔 30s→15s；字段命名与 OBS-AGENT-001 OTel 语义对齐（usage_percent/bytes_sent_rate 单一命名）。**真机抽样对账（Linux+Windows 各一台、与 top/任务管理器 <10%）本环境不可运行，登记为部署后人工验收项**；Windows 采样未实测；每进程 CPU% 为 gopsutil 启动以来平均语义。
  - 完成证据：2026-11-01；`core/metrics/processes.go`、`core/metrics/disknet.go`、`core/metrics/processes_test.go`（TestCounterRate/CollectProcessAndPortFaces/NetworkRateNonNegative）、`internal/platform/registry/metrics.go`（三面持久化 + RollupHourly 幂等 + PruneHourly）、`internal/platform/registry/metrics_hourly_test.go`、`internal/platform/registry/metrics_enrichment_test.go`、迁移 0051 三处同步（`internal/svc/migrations.go` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=51 + `internal/db/migrate/migrate_test.go`）、`internal/svc/migrations_metrics_enrichment_test.go`（存量形态/缺表跳过/列集 round-trip）、`internal/api/ops/metrics_history.go` + `metrics_history_window_test.go`（raw 重建三面/hourly/越权 fail-closed）、`internal/app/agent/ops_config.go`、`proto/croupier/ops/v1/ops.proto`（PortMetrics/rates/listening_ports，四 SDK gencode 同步）、`web/src/pages/Ops/Nodes/NodeDetailDrawer.tsx`、[交付报告](docs/review/2026-11-01-ops-010-node-metrics.md)；`go build ./...` 过，`go test ./core/metrics/... ./internal/platform/registry/... ./internal/api/ops/... ./internal/svc/... ./internal/db/migrate/... ./internal/app/agent/... -count=1` 全绿，web tsc 0 错误，NodeDetailDrawer jest 29/29，dashboard guard PASSED，docs build 过；commit `42c3ca76`。

- [x] **OPS-001 · P1 · 实现 · 云资源供给引擎（集成 Terraform/OpenTofu，不自写云适配器）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-01，边界见 [交付报告](docs/review/2026-11-01-ops-001-provision-engine.md) §4）：`internal/ops/provision` 用 hashicorp terraform-exec 驱动 tofu/terraform；provider 模板（阿里云/腾讯云/AWS/Hetzner 起步）；一键扩容闭环=选厂商/地域/规格 → create → cloud-init 注入 croupier agent（bootstrap token+hub 地址）→ 自注册 → 按网络审批策略（个人直通/团队工单）→ 纳管；HTTP 面 `/ops/provision*` 五端点（提交/列表/详情/续行/销毁）落地，`provision.enabled=false` 时如实报 503 不假装已开启；迁移 0052 建 `provision_runs`。**真实 provider 集成（mock cloud + 真实 provider 各一）本环境不可运行（需云凭据 + tofu 二进制），登记为部署后人工验收项**——本环境以模板渲染真值面 + FakeRunner 全链 + `-race` 覆盖编排正确性；cloud-resource-requests.md 的目录/申请单三角色流（catalog/requests 表 + `cloud.request.*` 权限 + 执行回执）未随附（设计纸保持未实现状态，见其「Implementation status」）。
  - 理由：各云适配已有成熟项目（Terraform provider 全覆盖），自写=重复造轮子；croupier 只做策略与编排。
  - 交付与验收：`internal/ops/provision` 用 hashicorp terraform-exec 驱动 tofu/terraform；provider 模板（阿里云/腾讯云/AWS/Hetzner 起步）；一键扩容闭环=选厂商/地域/规格 → create → cloud-init 注入 croupier agent（bootstrap token+hub 地址）→ 自注册 → 按网络审批策略（个人直通/团队工单）→ 纳管；集成测试用 mock cloud + 真实 provider 各一。
  - 完成证据：2026-11-01；`internal/ops/provision/`（types/runner/template/orchestrator + orchestrator_test/template_test）、`internal/model/provision_model.go`、迁移 0052 三处同步（`internal/svc/migrations.go` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=52 + `internal/db/migrate/migrate_test.go`）、`internal/svc/migrations_provision_test.go`（建表/幂等/round-trip + 注册表含 0052）、`internal/svc/provision_wiring.go`（DBStore/ApprovalGate/AgentRegistry 适配）、`internal/svc/service_context.go`（enabled 装配）、`internal/api/ops/provision.go` + `provision_test.go`（503/422/404/409/202/200 映射）、`internal/handler/routes.go`（`/ops/provision*` 五路由）、[交付报告](docs/review/2026-11-01-ops-001-provision-engine.md)；`go build ./...` 过，`go test ./internal/ops/provision/... ./internal/api/ops/... ./internal/svc/... ./internal/model/... ./internal/db/migrate/... ./internal/handler/... -count=1` 全绿（provision 包含 `-race`），`go vet` 过，docs build 过。
- [x] **OPS-002 · P1 · 实现 · 缩容与排水（drain → destroy）**
  - 依赖：OPS-001。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见 [交付报告](docs/review/2026-11-02-ops-002-drain-destroy.md) §4）：销毁前置三闸落地——数据盘护栏（`provision_runs.destroy_protected` 持久化标记 + `PUT /ops/provision/:runId/protection`，迁移 0053 补列）拒绝自动销毁；排空门要求运行名下在线 agent 全部 drain（复用 `AgentSession.Draining` 路由门）；「无负载才允许销毁」保守版=目标机在途任务（task_runs 活跃状态集 `[queued, dispatching, running, cancel_requested]`）归零（`TaskRunModel.CountActiveByAgents` 新查询，经 `ServiceContext.GameScopeContext` 解析 game 库）；销毁走同一 Terraform 状态（`Orchestrator.Destroy` 复用 WorkDir/main.tf，防漂移）；失败 fail-closed 500 `provision_load_unverifiable` 不放行。**「账号负载迁移走调度编排」未做**（无账号分布数据源，诚实缺位——todo 原文「先行用无负载保守版」即认可此边界）。
  - 交付与验收：drain=把目标机账号负载迁移走（依赖调度编排，先行用「无负载才允许销毁」保守版）；数据盘护栏（持久化标记禁止自动销毁）；销毁走同一 Terraform 状态，防漂移。
  - 完成证据：2026-11-02；`internal/api/ops/provision.go`（`provisionDestroyGuards` 三闸 + `OpsProvisionDestroy` 守卫接线 + `OpsProvisionProtection` + `ProvisionProtection` handler + `provision_destroy_protected`/`provision_tasks_inflight`/`provision_agents_not_drained`/`provision_load_unverifiable` 哨兵）、`internal/model/provision_model.go`（`DestroyProtected` 列）、迁移 0053 三处同步（`internal/svc/migrations.go` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=53 + `internal/db/migrate/migrate_test.go`）、`internal/model/task_model.go`（`CountActiveByAgents`）、`internal/svc/provision_wiring.go`（护栏双向映射）、`internal/svc/service_context.go`（`GameScopeContext` 导出）、`internal/audit/audit.go`（`provision.destroy`/`provision.protect`/`provision.unprotect`）、`internal/handler/routes.go`（`PUT /ops/provision/:runId/protection`）、`provision_test.go` 六守卫用例 + `task_model_test.go` + `migrations_provision_test.go`（0053 存量形态复刻）、[交付报告](docs/review/2026-11-02-ops-002-drain-destroy.md)；`go build ./...` 过，`go test ./internal/api/ops/ ./internal/model/ ./internal/svc/ ./internal/ops/provision/ ./internal/handler/ ./internal/audit/ ./internal/db/migrate/ -count=1` 全绿（守卫用例含 `-race`），`go vet` 过，docs build 过。
- [x] **OPS-003 · P2 · 实现 · 审批双模式**
  - 依赖：OPS-001。
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：网络级 `provision.approval=auto|workflow` 双模式落地——auto=个人直通（缺省，行为与现状一致）；workflow=扩容/销毁均走申请单（创建→审批→执行，审计留痕），客户端传 direct 也被强制为 ticket 不得绕过审批；销毁入口先推进 `destroy_pending`（资源不立即销毁），审批通过后经 `POST /ops/provision/:runId/destroy` 执行 terraform destroy 原语，未批准/非 destroy_pending 返回 409 `provision_state_conflict`，批准后双续行被拒（防双跑）。
  - 交付与验收：网络级配置 `provision.approval=auto|workflow`；个人=直通，团队=扩容/销毁申请单（创建→审批→执行，审计留痕）；复用现有 scope 隔离与审计。
  - 完成证据：2026-11-01；`internal/config/config.go`（`ProvisionConfig.Approval` + `EffectiveApproval`，`ProvisionApprovalAuto`/`ProvisionApprovalWorkflow`）、`internal/api/ops/provision.go`（`requestDestroy` + `OpsProvisionDestroyResume` + `ProvisionDestroyResume` handler + `provision_state_conflict` 哨兵）、`internal/ops/provision/orchestrator.go`（`RequestDestroy` + `DestroyResume` + `ErrRunNotDestroyPending`）、`internal/svc/provision_wiring.go`（`CreateDestroyApproval` 审批单）、迁移 0054 三处同步（`internal/svc/migrations.go` `provisionRunDestroyApprovalMigration` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=54 + `internal/db/migrate/migrate_test.go`）、`internal/model/provision_model.go`（`DestroyApprovalID` 列）、`internal/ops/provision/types.go`（`StateDestroyPending` + `DestroyApprovalID`）、`internal/audit/audit.go`（`provision.destroy_request`）、`internal/handler/routes.go`（`POST /ops/provision/:runId/destroy`）、`configs/server.yaml`（`provision.approval` 注释）、测试 `internal/config/provision_approval_test.go`（`TestProvisionConfig_EffectiveApproval`）、`internal/api/ops/provision_test.go`（workflow 强制 ticket / 销毁走申请单 / 未批准 409 / 批准后执行）、`internal/ops/provision/orchestrator_test.go`（`RequestDestroy` 状态推进 / `DestroyResume` 批准后转 destroyed / 双续行拒绝）、`internal/svc/migrations_provision_test.go`（0054 补列幂等 + 注册表含 0054）；`go build ./...` 过，`go vet`（touched 包 + pre-commit `go vet ./...`）过，`go test ./internal/api/ops/ ./internal/config/ ./internal/model/ ./internal/svc/ ./internal/ops/provision/ ./internal/handler/ ./internal/audit/ ./internal/db/migrate/ -count=1` 全绿（`internal/svc` 446s / `internal/api/ops` 469s / `internal/model` 62s / `internal/db/migrate` 64s，超时 900s 下无假死），gofmt 干净；commit `16873069`。
- [x] **OPS-004 · P1 · 实现 · 部署发版（包仓库+灰度+回滚）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见下文）：服务端部署编排落地——包仓库（对象存储登记 + sha256 + size + 元数据）、发版单（选包 + 目标机集合 + 分批策略 1/10/50/100%）、健康门禁（探针失败自动暂停批次）、回滚（指回上一版本包重放）。HTTP 面 `/ops/deploy/packages`（POST 登记/GET 列表）+ `/ops/deployments`（POST 创建/GET 列表/GET :id）+ `/ops/deployments/:id/start` + `/ops/deployments/:id/rollback`；未装配（DB/对象存储缺）如实报 503 deploy_not_configured，不假装已开启。迁移 0055 建 `deploy_packages` + `deployments`。
  - 交付与验收：构建产物登记到内置包仓库（对象存储复用 rustfs 或文件服务）；发版单=选包+选目标机集合+分批策略（1/10/50/100%）；每批健康门禁（探针不过自动暂停）；回滚=指回上一版本重放。
  - 完成证据：`internal/ops/deploy/deploy.go`（编排：分批切分/健康门禁暂停/回滚/状态机）、`deploy_test.go`（分批 10%、健康门禁 c 失败暂停、apply 失败 failed、回滚 rolled_back、非法 percent/空目标拒绝）；`internal/model/deployment_model.go`（包/发版单/目标模型）+ `model_test.go`（CRUD + 表名）；`internal/api/ops/deployments.go`（HTTP + DTO）+ `deployments_test.go`（DTO 构建 / 未装配 503）；`internal/api/ops/deploy_agent.go` + `deploy_agent_test.go`（Healthy 探针失败/通过/无指标）；迁移 0055 `internal/svc/migrations.go` + `migrations_deploy_test.go`（建表幂等）+ `internal/db/migrate/migrate.go` MinimumRequiredVersion=55 + `migrate_test.go`；路由 `internal/handler/routes.go`。`go test ./internal/ops/deploy/ ./internal/model/ ./internal/api/ops/ ./internal/svc/ ./internal/db/migrate/ ./internal/handler/ -p 1 -count=1` 全绿（api/ops 445s、svc 225s、model 105s、db/migrate 65s）；`go build ./...` 过、`go vet`（touched 包）过、web tsc 0 错误、dashboard_vnext_guard PASSED、git diff --check 干净。
  - 边界诚实：生产投递（DeployAgent.StagePackage）复用对象存储签名 URL + agent 本地下载（经 ExecCommand），宿主机需有 `curl`/`wget` 且在 agent 命令白名单——本环境无真实多机部署/agent 白名单，未端到端实测，登记为部署后人工验收项；健康门禁当前按探针失败/无近期指标判定（资源过载阈值未接入，见 OPS-010 指标面）；分批/回滚编排语义经 Fake agent 单测验证，真实 agent 换包依赖既有 `UpdateProcess`（§10.4 已验收）。
- [x] **OPS-005 · P2 · 实现 · 日志采集与检索**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见下文）：agent 应用日志 tail 采集折入 metrics 批量面（`core/logs/tailer.go`，行级 seq 单调 + 截断 + 级别/时间戳解析 + 轮转/截断重置），服务端 `agent_log_entries` 落库（`LogStore` 按 (agent,file,seq) 去重，原始 7 天保留），检索 API `GET /ops/logs/search`（按机/文件/级别/时间窗，scope 隔离 fail-closed），节点详情面板应用日志视图（级别标签/文件/内容，最新在前分页）。
  - 交付与验收：agent 采集应用日志 tail 上报；服务端落库（按机/应用/级别/时间检索）；面板日志视图；采集开关与采样可配。
  - 实现要点：proto `MetricsReport.logs`（字段 13）+ `LogEntry` 消息（file/level/message/ts_unix_ms/seq）；`OpsConfig.LogFiles`（默认空=关）+ `LogMaxLineBytes`（默认 4096）+ `LogMaxLinesPerCycle`（默认 500）；`UpstreamClient.WithLogTailer` + `reportMetrics` 折入；迁移 0056 建 `agent_log_entries`（三列唯一索引 dedup，received_at 索引检索），`MinimumRequiredVersion=56`。
  - 边界诚实：tail 仅按配置显式路径（不 glob 展开）；message 为原始行（含级别前缀）；级别按行首 120 字符内正则匹配（未知=空）；时间戳仅解析行首 RFC3339（无=0 由服务端 received_at 兜底）；无 Windows 真机 tail 实测（跨平台路径同 gopsutil，轮转用 size 检测）；面板只检索最新 100 条（无历史分页加载 UI）。
  - 完成证据：2026-11-02；`core/logs/tailer.go` + `inode_unix.go` + `inode_windows.go` + `tailer_test.go`（新行/缺失文件重试/截断轮转/超长行/级别/时间戳）；`internal/platform/registry/logstore.go` + `logstore_test.go`（入库去重/按级按文件检索/分 agent 隔离/prune）；`internal/server/log_ingest_test.go`（handleMetricEvent 折入日志）；`internal/app/agent/log_tail_upstream_test.go`（reportMetrics 折入 logs）；`internal/api/ops/logs_search_test.go`（返回过滤/级别过滤）；`internal/svc/migrations_agentlog_test.go`（0056 建表幂等）；`internal/db/migrate/migrate_test.go`（合成 0056）；`web/src/services/api/ops.ts` + `NodeDetailDrawer.tsx` + `NodeDetailDrawer.test.tsx`（日志加载/空态）；`configs/agent.yaml`（logFiles 示例）。`go build ./...` 过、`go test ./core/logs/... ./internal/app/agent/... ./internal/platform/registry/... ./internal/server/... ./internal/api/ops/... ./internal/svc/... ./internal/db/migrate/... ./internal/model/... ./internal/handler/... -p 1 -count=1` 全绿、`cd sdks/go && go test ./... -count=1` 全绿、`cd web && pnpm run tsc` 0 错误、`pnpm test -- src/pages/Ops` 32 套件/373 测试通过（含 NodeDetailDrawer 31 例）；commit `a4b69e1a`。
- [x] **OPS-006 · P2 · 实现 · 配置下发与批操作**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见下文）：服务端批编排 `internal/ops/batchop`——按 scope/标签圈选机器（RegistrySelector 按 game/env + 标签 AND + 显式 agent 交集）、config push / restricted exec 两种载荷、dry-run 只预览不派发、逐机并发（8 路 cap）经 ops tunnel 执行、聚合报表（成功/失败/跳过）+ 审计（create/dry_run/execute 三类事件）。HTTP 面 `POST /ops/batch`（创建，默认 dry-run）+ `POST /ops/batch/:id/execute`（执行/预览）+ `GET /ops/batch` + `GET /ops/batch/:id`；迁移 0057 建 `batch_ops`。agent 侧新增 `PushFile`（`MsgPushFileRequest`，配置 `pushBaseDir` 白名单 + `maxPushFileBytes` 上限 + content sha256 校验 + 拒绝越界/覆盖需 force），命令执行复用既有 `ExecuteCommand`（白名单 fail-closed 已在 OPS-001 前验证）。
  - 交付与验收：按 scope/标签圈选机器集合下发配置文件或执行受限命令模板；dry-run 预览；结果聚合报表；全部进审计。
  - 实现要点：proto `PushFileRequest/Response`；`OpsConfig.PushBaseDir`/`MaxPushFileBytes`；`OpsClientWrapper.PushFile`；`batch_ops` 模型 + 迁移 0057 + `MinimumRequiredVersion=57`；审计事件 `batch_op.create/dry_run/execute`。
  - 边界诚实：batch 执行经 agent 活跃 TCP 会话（离线 agent 目标计入 failed）；命令模板复用 agent `ExecAllowedCommands` 白名单（未命中/空白名单 fail-closed）；config push 仅写 `pushBaseDir` 下（未配置=禁用）；dry-run 不落敏感载荷明文到审计（仅摘要）；本环境无真实多机 agent 栈，端到端派发经 Fake agent 单测验证，真实多机登记为部署后人工验收项；面板未做批量操作 UI（验收只要求 API 聚合报表与审计，面板归 OPS-011 大屏候选）。
  - 完成证据：2026-11-02；`internal/ops/batchop/batchop.go` + `selector.go` + `id.go` + `batchop_test.go`（创建校验/exec 聚合/失败统计/config push/dry-run 不派发）；`internal/app/agent/ops_pushfile.go` + `ops_pushfile_test.go`（写入/越界拒绝/未配置禁用/hash 不匹配/覆盖需 force）；`internal/agent/local_handler_test.go`（handlePushFile 未装配拒绝）；`internal/logic/ops/agent_ops_client.go`（PushFile）；`internal/api/ops/batchops.go` + `batchops_test.go`（创建校验/未知 kind）；`internal/svc/migrations_batchop_test.go`（0057 幂等）；`internal/db/migrate/migrate_test.go`（合成 0057）；`internal/model/batch_op_model.go`；`internal/handler/routes.go`；`configs/agent.yaml`（pushBaseDir 示例）。`go build ./...` 过、`go test ./internal/ops/batchop/... ./internal/app/agent/... ./internal/agent/... ./internal/logic/ops/... ./internal/api/ops/... ./internal/model/... ./internal/svc/... ./internal/db/migrate/... ./internal/handler/... ./pkg/protocol/... -p 1 -count=1` 全绿、`go vet` 定向包过、`cd sdks/go && go build ./...` 过、`python compileall` 过。
- [x] **OPS-007 · P2 · 实现 · 成本控制**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，边界见下文）：创建前规格×时长估算（内置价格目录，未知规格不计价不伪造价）；机器成本标签与面板（/ops/cost 页面：估算 + 成本总览 + 预算）；闲置回收评估（CPU 低于阈值持续 >= N 小时判定，默认只提醒不销毁，AutoDestroy 需显式 Enabled+AutoDestroy）；预算告警走通知渠道（超 80% 触发 notify 分发）。
  - 交付与验收：创建前规格×时长估算；机器成本标签与面板；闲置回收（无负载 N 小时提醒/自动销毁，可配）；预算告警走通知渠道。
  - 实现要点：`internal/ops/costcontrol`（`ComputeEstimate`/`BuildSummary`/`IdleEvaluator`/`BudgetEvaluator`）；迁移 0058 建 `cost_budgets`；HTTP `/ops/cost/{estimate,summary,idle,budget}`；web `/ops/cost` 面板。
  - 边界诚实：价格目录为内置参考价（生产价需接入真实账单/价格 API，估算精度登记为部署后对账）；闲置判定依赖 agent 资源指标（OPS-010），无近期样本视为未知不误判；自动销毁默认关闭（只提醒）；本环境无真实云/多机栈，端到端真实云成本对账与自动销毁登记为部署后人工验收项。
  - 完成证据：2026-11-03；`internal/ops/costcontrol/costcontrol.go` + `idle.go` + `costcontrol_test.go`（估算/汇总/闲置/预算/聚合）；`internal/model/cost_budget_model.go` + `cost_budget_model_test.go`；`internal/api/ops/costcontrol.go` + `costcontrol_test.go`（估算/预算/汇总 scope 隔离）；`internal/svc/migrations_costbudget_test.go`（0058 幂等）；`internal/handler/routes.go`（/ops/cost/*）；`web/src/services/api/ops.ts` + `web/src/pages/Ops/Cost/index.tsx` + `web/config/routes.ts`；`docs/api/index.md`（Ops - cost control 端点族）+ `docs/operations/cost-control.md` + `docs/operations/index.md` + `docs/.vitepress/config.mjs`（docs 导航）。`go build ./...` 过、`go test ./internal/ops/costcontrol/... ./internal/api/ops/... ./internal/model/... ./internal/svc/... ./internal/db/migrate/... -count=1` 全绿、`go vet` 定向包过、`cd sdks/go && go build ./...` 过、`cd web && npx tsc --noEmit` 0 错误、`npx jest config/routes.test.ts src/pages/routes-resolution.test.tsx src/utils/api.test.ts src/services/api` 全绿、`bash scripts/dashboard_vnext_guard.sh` PASSED、`cd docs && CI=true pnpm build` 过。
- [x] **OPS-011 · P2 · 实现 · 运维大屏（舰队健康总览，运维台独立页面）**
  - 依赖：OPS-010。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，交付完成）。前端独立大屏页 `web/src/pages/Ops/Dashboard/`（`index.tsx` 组件 + `health.ts` 纯逻辑）；五源 best-effort 轮询（listOpsNodes/fetchOpsAlerts/listOpsProvisionRuns/listSyntheticsResults + getAgentMetricsHistory，Promise.allSettled 单源失败不整体挂）；三色健康档（默认 60/85，HealthThresholds 可配）+ 防抖（连续 3 周期越界才变色，normal/offline/unknown 立即恢复）；离线灰态置底；聚合带（全网节点/在线率/平均 CPU·内存/今日扩缩容/活动告警 + 各档位计数）；告警滚动条（剔除已静默）；点击节点卡下钻（hourly 近 48 点资源曲线 + 进程/端口摘要）；`?agentId=` 深链预选下钻；断流 30s → 顶部「数据过期」横幅（不展示旧值为当前）。
  - 交付与验收：ops 前端独立大屏页——①节点卡片墙：每节点一枚卡片（名称/大区/角色/在线状态），三色健康态=正常（CPU+内存 <60%）/压力（60-85%）/过载（>85% 或探针失败），离线灰态置底；②聚合带：全网节点数、在线率、总负载、今日扩缩容次数、活动告警数；③账号负载视图：各节点承载账号数 vs 容量上限（依赖调度编排的账号分布数据）；④告警滚动条（实时事件流）；⑤点击节点卡下钻到该节点详情（资源曲线/进程/最近操作）。
  - 数据源：OPS-010 资源指标 + synthetics 探针 + 审计事件流；轮询或 SSE 推送（≤5s 延迟）。
  - 阈值：三色阈值可配（网络级）；状态判定防抖（连续 3 个周期越界才变色）。
  - 验收：模拟 20 节点（含 1 离线 2 过载）状态正确着色；断流 30s 后显示「数据过期」而非旧值；1080p 与 4K 电视墙两档布局。
  - 完成证据：commit `b6ee4114`（10 文件/1340 行）已推送 origin/main；验收对照：20 节点模拟（1 离线 2 过载 1 压力）计数与着色正确（`__tests__/index.test.tsx`：正常 16/压力 1/过载 2/离线 1，离线置底 DOM 末位）；断流 → 「数据过期」横幅（`isStale` + 横幅用例）；tsc 0 错误、`npx jest src/pages/Ops/Dashboard` 33/33、routes+ops service 47/47、`dashboard_vnext_guard.sh` PASSED、docs build 通过。
  - 边界诚实：账号负载视图为「待接入」占位（OPS-002 边界：当前无账号分布数据源，不伪造数字）；阈值当前取默认常量（网络级可配登记为后续工作）；大屏只读（探针失败为信号非强制执行）；1080p/4K 两档——页面用 `auto-fill minmax(240px,1fr)` 响应式网格，未设专用电视墙断点（登记为已知边界）。

- [ ] **OPS-008 · P3 · 实现 · 数据面备份恢复**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。
  - 交付与验收：游戏库定期备份策略（经 capture 所见的数据源登记）、恢复演练模式、备份清单与校验。
- [ ] **OPS-009 · P3 · 实现 · 运维 runbook 自动化**
  - 依赖：OPS-004, OPS-006。
  - 状态／条件：TODO（未开始）。
  - 交付与验收：常用操作固化为 runbook（参数化步骤序列，可引用 OPS-004/006 能力）；一键执行+审计。
- [ ] **OBS-AGENT-001 · P2 · 实现 · 四类 Agent 接入 OTel 打点（配置开关默认关）**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。
  - 交付与验收：agents/capture、devops、sidecar、synthetics 复用服务端 telemetry SDK（otel v1.46 已在 go.mod）；config 增 telemetry.enabled（默认 false）+ OTLP endpoint；开启后 span/metric 上报可被服务端面板看到；关闭时零开销路径（noop provider）；每 agent 至少覆盖：启停、连接状态、事件/吞吐计数、错误率；测试用内存 exporter 断言。(2026-11-01 用户提议「agent 应支持 otel 可选开启」,已代决立项;eBPF 火焰图评估后暂缓:宿主机级 profiler 非本仓职责,待真实 profiling 需求再议)

- [x] **AGENT-001 · P0 · 故障验证 · 四类 Agent 当前接口的 scope 隔离矩阵**
  - 依赖：—。交付与验收：按当前已支持的注册/配置/列表/统计/详情/动作/日志/离线读取逐项核对；复用非空 scope 和绑定测试，覆盖同 ID 冲突及跨 scope 指派；devops 未接通的会话/回放列 NOT IMPLEMENTED，不用假数据当验收
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-01）**：四类 Agent（capture/devops/sidecar/synthetics）× 注册/配置/列表/统计/详情/动作/日志/离线读取逐接口矩阵已落 `docs/review/2026-11-01-agent-scope-isolation-matrix.md`；每行标注 scope 来源 + fail-closed 与否 + 测试证据。核对中发现 live 缺口并已修复：`/ops/nodes/:id/{drain,undrain,restart,meta}` 此前只按 nodeId 解析（A scope 管理员可动 B scope 节点），本轮统一补 `requireAgentScope` fail-closed 门（`internal/api/ops/helpers.go`），`opsNodeMeta` 保持「不存在→404」契约；新增 3 个跨 scope 拒绝用例（`internal/api/ops/node_action_scope_test.go`：`TestOpsNodeDrain_CrossScopeRejected` / `TestOpsNodeRestartUndrain_CrossScopeRejected` / `TestOpsNodeMeta_CrossScopeRejected`）；6 个既有 drain/restart/undrain 用例补 RegistryStore 装配。同 ID 冲突证据 `TestHandleRegisterRequest_SameAgentIDCrossScopeRejected`（control_handler_scope_test.go:211，commit 3c5e44be）；跨 scope 指派拒绝 `TestDispatcher_pickAgentWithRouting_FiltersGameEnvironment`（dispatcher_test.go:784）。边界诚实：synthetics 离线读取 query-scope 已知边界（§3.1，不改行为）、devops 会话/回放 NOT IMPLEMENTED 无假数据（§3.2）、`/api/v1/nodes` 无 scope 面 by design（§3.3）、`ops.NodeService` 死实现（§3.4）。
  - 完成证据：矩阵文件 `docs/review/2026-11-01-agent-scope-isolation-matrix.md`；新测试 `internal/api/ops/node_action_scope_test.go`（3 用例）；修复 `internal/api/ops/helpers.go`（4 函数）；`go test ./internal/api/ops/ -count=1` 全绿、`internal/server` scope/注册用例全绿、`internal/platform/dispatch` 路由 scope 用例全绿；`gofmt` 干净；commit `9485f3d2`。

- [ ] **AGENT-002 · P1 · 实现 · devops 面板会话中继**
  - 依赖：—。交付与验收：接通已有 Broker/Hub 的创建/数据/窗口/关闭通道与 Server 授权、审计、背压；不新增 RDP/VNC；SSH 真目标端到端连接和断线清理可验。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需 SSH 测试主机/凭据/指纹；沙箱无 SSH 目标。缺 SSH 主机/凭据。

- [ ] **AGENT-003 · P1 · 实现 · devops 主机/会话/回放界面**
  - 依赖：AGENT-002。交付与验收：复用四类 Agent 公共页与现有 asciicast；真实会话列表、scope 权限、动作审计与回放可达，未实现能力不以假按钮替代。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需 SSH 测试主机（依赖 AGENT-002 会话中继未完成）；缺 SSH 主机。

## 探测验收与调查

- [ ] **PROBE-001 · P1 · 故障验证 · 真实游戏协议的多语言插件验收**
  - 依赖：—。交付与验收：复用 go-plugin 宿主、Go/Python/Node SDK；真实游戏目标验证 Describe/Check、凭证不回显、超时/取消/崩溃/熔断/恢复；不再规划 JSONL 等价运行时。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需真实游戏协议/目标/凭据；沙箱无真实游戏目标。缺真实协议/目标/凭据。

- [ ] **PROBE-002 · P1 · 故障验证 · 第三方服务多视角探活验收**
  - 依赖：PROBE-004, PROBE-005。交付与验收：针对各角色实际能力建立矩阵，分别核对 sidecar 的 Checker/策略热更、synthetics 原生 HTTP/ICMP/插件、面板阶段/窗口/故障恢复；模拟端口通但业务错、断线和恢复；不宣称所有角色共享全部 DB/私有协议 Checker
  - 状态／条件：BLOCKED（2026-11-01 复核）：动态同步/丢失语义仍有实现/决策前置（依赖 PROBE-005 方案批准），真实网络视角需目标/凭据；缺 PROBE-005 决策/真实目标凭据。本地机制部分待前置决策后开展。

- [ ] **PROBE-003 · P2 · 资料调查 · 玩家 Login 级适配边界**
  - 依赖：—。交付与验收：复用已有 Minecraft SLP 与私有协议骨架，收集 online-mode/自家游戏登录协议与账号条件；SLP 状态检查不能作完整玩家登录证据。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需 MC Java 账号/出网、自家游戏账号/协议，许可证变更另行授权（用户拍板）。缺账号/出网/协议授权。

- [x] **PROBE-004 · P1 · 实现 · synthetics 目标及策略版本同步**
  - 依赖：—。交付与验收：按既有角色设计补齐静态配置到受控版本拉取/热生效，复用 core/configsync 并绑定 gameId/env；坏版本保留 last-good，重连后取最新，禁用/删除目标释放资源；不凭 sidecar 的 puller 宣称 synthetics 已拥有同能力
  - 状态／条件：LOCAL VERIFIED（2026-11-01 复核）：policy/target 下行拉取均已落地（commit a10d4986），单测 policy_sync_test.go / targets_sync_test.go 通过；可本地实施项已实施。
  - 交付（policy sync）：probe.policy 下行拉取经 core/configsync 热生效，`agents/synthetics/runner/runner.go` 新增 `newPolicyPuller` + `intervalFor`（EffectiveIntervalFor 叠加策略），坏版本/拉取失败保留 last-good（`policy_sync_test.go`）；服务端 `internal/server/control_handler.go:725` handleProbePolicyConfig 读 config_versions probe.policy 按注册会话 gameId/env 下发。
  - 交付（target sync）：synthetics.targets 下行拉取经 core/configsync 热生效，`agents/synthetics/runner/runner.go` 新增 `newTargetsPuller` + `applyTargets`（验证后 diff 热替换，删除目标停 timer 释放资源），坏版本/校验失败保留 last-good，重连后 configsync 续拉取最新（`targets_sync_test.go`）；服务端 `internal/server/control_handler.go:789` handleSyntheticsTargetsConfig 读 config_versions synthetics.targets 按注册会话 gameId/env 下发（`control_handler_synthetics_targets_test.go`）；wire 新增 `MsgSyntheticsTargetsConfigRequest/Response`（0x040129/0x04012A，`proto/croupier/ops/v1/ops.proto`、`pkg/protocol/message.go`、`agents/controlconn/controlconn.go`）。
  - 完成证据：`agents/synthetics/runner/runner.go`、`agents/synthetics/runner/policy_sync_test.go`、`agents/synthetics/runner/targets_sync_test.go`、`internal/server/control_handler.go`

- [ ] **PROBE-005 · P1 · 方案决策 · 探测结果与故障恢复事件丢失语义**
  - 依赖：—。交付与验收：区分可丢的周期采样与必须可核对的故障/恢复跃迁；评审断链缓存/重传/新鲜度/缺测状态及容量预算，优先复用既有 report/告警机制，不默认持久化每条采样
  - 状态／条件：BLOCKED（2026-11-01 复核）：协议缺口需方案评审批准（用户拍板）；真实第三方环境不能补齐此协议缺口。缺方案批准。

## 待批准的运维方案

- [ ] **OPS-012 · P2 · 方案决策 · Agent 自升级四问**
  - 依赖：—。交付与验收：评审升级源/触发权限/重启接力/失败姿态，复用现有指纹及最后已知锚点；交付失败不砖化与带外回滚验收方案，等待既有要求的决策批准后实施。
  - 状态／条件：BLOCKED（2026-11-01 复核）：待方案批准（用户拍板）。缺方案批准。

- [ ] **OPS-013 · P2 · 方案决策 · 受管程序配置更新五问**
  - 依赖：—。交付与验收：明确路径沙箱/格式校验/原子性与重启回滚/体积带宽/审批闸，复用 configsync 先例；不声称现有探针配置拉取已支持程序文件分发。
  - 状态／条件：BLOCKED（2026-11-01 复核）：待方案批准（用户拍板）。缺方案批准。

## 部署事实

- [ ] **DEPLOY-001 · P1 · 资料调查 · 真实部署与外部依赖配置盘点**
  - 依赖：—。交付与验收：核对实际镜像摘要/角色/端口/TLS/协调后端/备份、Herald 与外部平台开关、OIDC/Vault/观测出口；交付脱敏拓扑和与模板差异，不把 Compose 存在当部署证据。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需真实部署只读权限与脱敏配置；沙箱无生产部署访问。缺部署权限/脱敏配置。

- [ ] **DEPLOY-002 · P1 · 故障验证 · 备份可恢复与业务凭据核对**
  - 依赖：DEPLOY-001。交付与验收：复用 backupexec 和既有备份/对象存储，在隔离环境实际恢复配置、契约/页面、审批/任务/执行凭据与角色数据，记录 RPO/RTO/校验和及未覆盖内存状态；验证副作用不因恢复重复执行，不新增未经设计的恢复服务
  - 状态／条件：BLOCKED（2026-11-01 复核）：需脱敏备份、隔离数据库/对象存储读权限；备份 API/文件存在不证明恢复成功。缺备份/隔离恢复环境。

## 容量验证

- [ ] **PERF-001 · P2 · 性能验证 · 会话与重连容量基线**
  - 依赖：—。交付与验收：给定 SDK/Agent 数、并发、消息大小和延迟目标，测 MuxConn 队列、重连风暴、连接峰值/P95 和资源释放；单独记录已知正确性缺口，不用吞吐通过替代安全结论
  - 状态／条件：BLOCKED（2026-11-01 复核）：需容量目标（用户拍板的并发/延迟目标）与隔离容量环境。缺容量目标/隔离环境。

- [ ] **PERF-002 · P2 · 性能验证 · 任务查询与 SSE 容量基线**
  - 依赖：—。交付与验收：给定并发任务/订阅者、事件频率和保留量测量事件写入/查询延迟、断线续读、慢客户端背压及资源释放；记录当前可靠性缺口，不能用吞吐替代 ARCH-015 验收
  - 状态／条件：BLOCKED（2026-11-01 复核）：需独立任务基准栈与延迟目标（用户拍板）。缺基准栈/延迟目标。

- [ ] **PERF-003 · P2 · 性能验证 · 插件并发与故障隔离容量**
  - 依赖：—。交付与验收：复用现有 go-plugin gate/backoff/breaker，给定插件数/并发/预算验证队列拒绝、超时、崩溃风暴、宿主资源和 Stop 释放；报告平台/版本，不做新的插件运行时选型
  - 状态／条件：BLOCKED（2026-11-01 复核）：需可控插件负载与资源预算（用户拍板的容量目标）。缺负载目标/资源预算。

## 共用核心分析与外部专业分析

- [ ] **DATA-001 · P1 · 方案决策 · 内置共用指标与外部专业分析边界**
  - 依赖：—。交付与验收：按用户确定的边界评审核心指标：新增、DAU/WAU/MAU、近实时在线、D1/D7/D30 留存、付费人数/订单/流水/退款/净额/付费率/ARPU/ARPPU；定义活跃事件、账号/角色去重、cohort/分母、UTC/业务时区、迟到更正、币种/单位及近似误差。复杂自助分析/归因/预测走外部，已有行为/关卡页保留但不扩建专业 BI
  - 状态／条件：BLOCKED（2026-11-01 复核）：方案决策待评审批准（用户拍板）；复用 docs/analytics/core-and-external.md 和既有 game-metrics-overview，口径未验收不称已可用。缺方案批准。

- [ ] **DATA-002 · P1 · 实现 · 共用事件接入与指标数据来源归一**
  - 依赖：DATA-001。交付与验收：复用 ingest/Server 事件入口/worker，提供游戏登录、注册、心跳、支付/退款的显式映射；每个指标声明权威来源、gameId/env/维度和新鲜度。统一 GORM 与 ClickHouse 读适配契约，不静默把平台 Player 数当游戏新增，不默认为两套库自动同步
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 的指标口径批准；缺方案批准。源码缺口保留：overview.go 读关系模型/Player，warehouse.go 读 ClickHouse；需接入样本。

- [x] **DATA-003 · P1 · 实现 · 活跃与在线聚合的版本和重放语义**
  - 依赖：DATA-001。交付与验收：修正 daily_users 的替换键/版本参数与分钟在线 HLL 快照重写策略，迁移已有表而不只改 initdb；同日多次 flush、迟到事件、重放和多 worker 的同一分钟结果可核对，不把全量 HLL 重写当增量相加。
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：010_analytics.sql 改 `ReplacingMergeTree(version)` 且 version 移出 ORDER BY；minute_online 由 SummingMergeTree 改 ReplacingMergeTree(version) 全量快照替换；worker.go 写 version；warehouse 查询加 FINAL；迁移脚本 scripts/clickhouse/011_fix_aggregate_tables.sql 重建已有表；真实 ClickHouse `TestDrill_DATA003_AggregateReplacement`（data003_drill_test.go:26）FINAL 取 95/55/6/1200/20/2 不叠加。
  - 完成证据：2026-11-01；`internal/analytics/worker/data003_drill_test.go`、`scripts/clickhouse/011_fix_aggregate_tables.sql`、`internal/analytics/worker/worker.go`；`go test ./internal/analytics/worker/... ./internal/api/analytics/...` 全绿；commit `5f1ba096`。

- [ ] **DATA-004 · P1 · 实现 · 付费聚合的币种、订单状态与累计口径**
  - 依赖：DATA-001。交付与验收：复用 payment 明细，按已批准币种/单位和订单唯一键处理重复成功、退款和状态变更；修复 revAgg flush 后删除而日表按替换读的增量/累计冲突，跨 flush/重启可回算，不能把不同币种原始金额直接相加
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 的币种/单位与订单状态口径批准；缺方案批准。源码缺口保留：touchRevenue 按 day/game/env 分组，未含 currency/order 去重，flush 后清空；真实金额样本与 ClickHouse 验证 NOT RUN。

- [ ] **DATA-005 · P1 · 实现 · 基础 cohort 留存的生产计算链**
  - 依赖：DATA-001, DATA-002。交付与验收：复用 RetentionModel，补齐注册 cohort 与回访事件的受控计算/回算任务；D1/D7/D30 明确分母、业务日、观察窗成熟与迟到更正；未成熟/缺数据返回不可用，不能伪造 0；用逻辑时钟和回放样本验算，无需等真实运行 30 天才写机制测试
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 的 cohort/回访口径批准与 DATA-002 事件映射；缺方案批准。源码缺口保留：生产路径只有 ListCohorts，未找到 UpsertCohort 的生产调用；前端概览 d1/d7/d30 固定 null。

- [ ] **DATA-006 · P1 · 实现 · 统一核心指标查询响应与 scope**
  - 依赖：DATA-002, DATA-003, DATA-004, DATA-005。交付与验收：复用现有 Analytics API/warehouse 查询，统一核心指标的窗口、维度、分母、source/freshness/approximation、币种和不可用原因；读取已有授权 scope，不让查询参数改到其它 game/env；缺配置/延迟/查询失败不能伪装零值
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-002~005 的有效聚合口径；缺上游方案与实现。

- [ ] **DATA-007 · P1 · 实现 · 运营核心分析页的真实数据闭环**
  - 依赖：DATA-006。交付与验收：复用 Overview/Realtime/Retention/Payments/Warehouse 组件，收敛为同口径核心视图；过滤条件/日期/时区/单位保持一致，展示延迟、缺测、未成熟和未启用状态，真实零值可区分；已有专业页不删除也不以硬编码值补齐
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-006 统一查询契约；缺上游实现。

- [ ] **DATA-008 · P1 · 故障验证 · 核心指标可核对样本与端到端验收**
  - 依赖：ARCH-011, DATA-007。交付与验收：以固定多 game/env/跨日/多币种/重复乱序/退款样本，从接入→Redis→worker→查询→页面逐指标对账；验证重启/迟到/跨 scope 拒绝与不可用状态，记录原始样本、期望、实际、版本和数据新鲜度
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-007 端到端视图；真实游戏仍需脱敏事件和支付对账样本；缺上游实现/验收样本。

- [ ] **DATA-009 · P2 · 性能验证 · 核心分析吞吐与查询容量**
  - 依赖：DATA-006。交付与验收：给定事件速率、保留量、查询并发与新鲜度预算，测写入/聚合/查询 P95、积压和回算对控制面的影响；复用现有独立分析管道，不自动引入新的流处理服务
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-006；需业务量级（用户拍板）和隔离 Redis/ClickHouse 环境。

- [ ] **DATA-010 · P1 · 资料调查 · 专业分析外部产品接入核查**
  - 依赖：—。交付与验收：按实际专业需求收集外部分析平台官方版本/许可、数据输入、授权 scope、身份/嵌入/深链、数据回流与成本限制；交付对接矩阵与单个首接目标，不在本轮选型或虚构任何供应商集成已完成
  - 状态／条件：BLOCKED（2026-11-01 复核）：需用户确定专业场景与候选外部产品；缺业务拍板/候选环境。

- [ ] **DATA-011 · P1 · 方案决策 · 外部专业分析的受控集成契约**
  - 依赖：DATA-001, DATA-010。交付与验收：复用 extensions 与既有数据出口，评审指标/事件/只读数据集的输出契约和来源、scope、字段映射、时间窗、版本、短期凭据/SSO/深链及审计；外部查询必须被服务端或受限数据集约束，不把浏览器过滤器当隔离，不泄露仓库凭据
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 指标边界与 DATA-010 首接产品；缺业务拍板/候选环境。

- [ ] **DATA-012 · P1 · 实现 · 首个专业分析平台适配与入口**
  - 依赖：DATA-011。交付与验收：按批准契约实现一个真实外部平台的配置/连接验证、受控数据输出和当前 scope 的报告入口；复用扩展安装/开关/审计，失联有明确降级信息，不阻塞 Croupier 核心运营与内置指标
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-011 批准契约；缺首接平台测试实例及最小权限凭据。

- [ ] **DATA-013 · P1 · 故障验证 · 外部分析授权与故障隔离验收**
  - 依赖：DATA-012。交付与验收：核对跨 game/env、撤权、过期凭据、外部故障、超时和重复导出；未授权数据不可见，导出可追溯，外部平台不可用时内置核心指标与运营执行可继续
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-012 适配实现；缺真实外部分析环境和多 scope 测试身份。

- [x] **DATA-014 · P1 · 实现 · 分析聚合共享状态串行化**
  - 依赖：—。交付与验收：复用 worker 的 touchedMinutes/touchedDays/revAgg；明确消费、pending 回收与 flush 的快照/同步所有权，失败重试不丢增量；收入并发写读及活跃桶增删在 race 下无竞争，固定样本核对总量，不仅给 ClickHouse batch 加锁
  - 状态／条件：LOCAL VERIFIED：flush 在 aggMu 内把 touchedMinutes/touchedDays/revAgg 整段快照转移并清空共享 map，随即释放锁再对快照做 ClickHouse I/O；失败项 remerge 回共享 map（写方已重建同 key 时 revAgg 累加而非覆盖），失败重试不丢增量。`TestSharedState_ConcurrentRevAggNoLoss` 固定样本核对已发送+剩余=总量，`-race` 无竞争。
  - 完成证据：2026-11-01；`internal/analytics/worker/worker.go`（flush 快照转移 + remerge 助手）；`internal/analytics/worker/shared_state_test.go`（并发写读总量核对）；`go test ./internal/analytics/worker/ -race -count=1` PASS。

- [x] **DATA-015 · P2 · 实现 · 分析 worker 退出等待与刷新资源收尾**
  - 依赖：—。交付与验收：Run/main 对 pending 回收、定时 flush 和最终 flush 建立有界退出/等待顺序；退出后不再读写 worker/测试配置，不在刷新未结束时关资源；复现既有 runFlushInterval 跨用例竞态并验证消失，阻塞刷新仍遵守总截止
  - 状态／条件：LOCAL VERIFIED：Run 用 workerCtx（WithoutCancel 派生）管理定时 flush 与 pending 回收子协程，主循环退出后 stopWorkers 并分别以 shutdownTimeout 上界等待；最终 flush 用带 finalFlushTimeout 截止的独立 ctx 并 select 等待，阻塞刷新仍守总截止。main 改为等待 Run 完整返回，不再固定 200ms 猜测。测试夹具用 runFlushInterval/shutdownTimeout/finalFlushTimeout 注入缩短。
  - 完成证据：2026-11-01；`internal/analytics/worker/worker.go`（Run 有界退出顺序 + waitGroupBounded）；`cmd/analytics-worker/main.go`（等待 runDone）；`internal/analytics/worker/coverage_v9_test.go`（`TestRunShutdownWaitsForPeriodicFlushAndHonorsDeadlineV9` 阻塞刷新有界退出）；`go test ./internal/analytics/worker/ -race -count=1` PASS。

## 前端审核复验

- [x] **REVIEW-001 · P1 · 故障验证 · 批量充值目标 ID 归一化复验**
  - 依赖：—。交付与验收：批量充值 demo 把选中行对象整体 `map(String)` 成 `[object Object]`，导致实际渲染路径无法匹配玩家 ID 却返回成功（web/src/components/PageRenderer/templates/playerManage.tsx:442）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收以实际批量执行入参为玩家 ID、回读影响人数与目标名单一致且无 [object Object] 为准。
  - 状态／条件：LOCAL VERIFIED：对象 selection 的精确余额增量、未选不变与 ID 兼容测试通过；真实环境另见 UI-001/003。
  - 完成证据：2026-10-30 复审；`web/src/components/PageRenderer/templates/gameOpsPublishFixture.test.tsx`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-002 · P1 · 故障验证 · 空白批量金额拒绝复验**
  - 依赖：—。交付与验收：`summarizeQuantityTotal` 把空串/全空白串转换成 0，违背“不确定输入不猜 0、不执行”注释并会确认并提交 0 充值（web/src/components/PageRenderer/batchSelection.ts:73）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收以空串/全空白均禁止确认与提交、合法有限数字仍可按显式语义执行为准。
  - 状态／条件：LOCAL VERIFIED：空串/空白阻断和合法金额确认后提交有直接断言，未重复实现 guard。
  - 完成证据：2026-10-30 复审；`web/src/components/PageRenderer/__tests__/ResourcePageRenderer.test.tsx`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-003 · P1 · 故障验证 · 错误响应体优先级一致性复验**
  - 依赖：—。交付与验收：`extractApiErrorCode` 声称与 `utils/errors` 同口径但优先 `.data` 再 `.response.data`，后者相反优先 `.response.data`，二者可能取到不同响应体（web/src/utils/apiError.ts:7）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收覆盖 data 与 response.data 并存且内容冲突时，extractApiErrorCode 与 utils/errors.extractErrorCode 对有效冲突响应采用同一响应体与稳定码。
  - 状态／条件：LOCAL VERIFIED：有效冲突 response.data/data 响应体优先级样本通过；不扩张为所有畸形响应解析统一。
  - 完成证据：2026-10-30 复审；`web/src/utils/__tests__/apiError.test.ts`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-004 · P1 · 故障验证 · 批量表单确认文案可达性复验**
  - 依赖：—。交付与验收：带表单批量动作进入表单后绕过原有 `confirm/confirmTitle/confirmDescription` 路径，批量充值模板声明的目标确认文案实际不可达（web/src/components/PageRenderer/ResourcePageRenderer.tsx:1246）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收覆盖带表单动作的确认标题/说明/目标可见且与实际提交一致，取消确认不会调用执行器。
  - 状态／条件：LOCAL VERIFIED：confirm=true 的标题/说明/目标/总量、取消零执行及正向提交参数测试通过。
  - 完成证据：2026-10-30 复审；`web/src/components/PageRenderer/__tests__/ResourcePageRenderer.test.tsx`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-005 · P1 · 实现 · 批量金额安全整数与乘积边界**
  - 依赖：—。交付与验收：为 summarizeQuantityTotal 与业务金额契约明确单位/倍率/最大值，拒绝非安全整数及乘积溢出；覆盖 MAX_SAFE_INTEGER 邻界、极大目标数与 Infinity，不显示可确认的失真总量
  - 状态／条件：LOCAL VERIFIED：`summarizeQuantityTotal` 收口为「有限非负安全整数 × 正整数安全整数 = 安全整数」，`batchTotalEntry` 复用同契约且移除 `toPrecision(12)` 失真显示；非法输入不显示/不提交。证据：`resourceBatchSelection.test.tsx` 与 `compositeBatchTotal.test.tsx` 覆盖 MAX_SAFE_INTEGER 邻界、溢出、Infinity、非整数拒绝；commit 0d74c456
  - 完成证据：2026-10-31；`web/src/components/PageRenderer/confirmContent.ts`（summarizeQuantityTotal 收口）、`web/src/components/PageRenderer/ResourcePageRenderer.tsx`（batchTotalEntry 复用同契约）、`web/src/components/PageRenderer/__tests__/compositeBatchTotal.test.tsx`、`web/src/components/PageRenderer/__tests__/resourceBatchSelection.test.tsx`（MAX_SAFE_INTEGER 邻界/溢出/Infinity/非整数拒绝用例）；`pnpm --dir web run tsc` 0 错误，`pnpm --dir web test -- compositeBatchTotal resourceBatchSelection` 绿；commit 0d74c456

## 依赖安全证据复核

- [ ] **SEC-001 · P1 · 资料调查 · 默认分支依赖告警适用性复核**
  - 依赖：—。交付与验收：获取 GitHub 默认分支未关闭告警的官方明细，映射 go.mod/go.sum、package.json/锁文件与 SDK/构建产物；区分运行时/开发/传递依赖、受影响版本与实际可达条件，逐项形成“需修复/不适用及依据/待验证”清单，再拆最小修复任务，不仅凭告警总数宣布存在可利用生产漏洞。
  - 状态／条件：BLOCKED（2026-11-01 复核）：读取 GitHub Dependabot 告警官方明细需要仓库 security_events 权限（API 或凭据/插件）；本沙箱无 GitHub 凭据。缺 GitHub 安全告警读取权限。

## 审核发现

- [x] 审核发现：分析 worker 把成功入批但未 Send 的 Redis 消息延迟 ACK，批 Send 失败时丢弃批并把消息留在 PENDING；reclaim 重新投递会再次执行 touchRevenue（相加累计 revenue/refunds），导致收入/退款聚合重复计数（internal/analytics/worker/worker.go:448, internal/analytics/worker/worker.go:625）
  - 修复：支付聚合增量延后到批次 Send 成功后提交；失败批次不残留增量。证据：2026-10-31；`go test ./internal/analytics/worker/... -count=1` 全绿；新增 `TestPaymentAggregationDefersUntilBatchSend`。
- [x] 审核发现：govern.go 注释声明门序与旧 Invoke 逐条对齐（输入契约为第 6 步、政策审批第 5 步），但实现把 ValidateInvokeInput 提前到 LoadCurrentAdmin/EnforceFunctionPolicy 之前（internal/platform/functioninvoke/govern.go:127），改变错误优先级（无效输入 400 会先于策略拒绝 403/404 返回），与注释及旧行为不一致（internal/platform/functioninvoke/govern.go:68）
  - 修复：full 执行链恢复 policy→input 顺序；schedule Create 定义期单独保留 input 校验。证据：2026-10-31；`go test ./internal/platform/functioninvoke/... -count=1` 全绿；新增 `TestGovernPolicyBeforeInputContract`。
- [x] 审核发现：`TestHandleInvoke_TaskContinuationBypassesClearedAgentLocal` 存在间歇性失败——断言 `resp.GetPayload()=="echo:"+string(data)` 与请求 `data` 的 proto 字段序列化顺序非确定有关（期望串与回显字节在 map 迭代/序列化时错位），全量 `go test -count=1 ./internal/agent/...` 多次复现 FAIL（internal/agent/provider_drain_test.go:451）；本测试为新引入用例（非基线既有），建议改为按字段解码断言而非全字节串比较。
  - 修复：该断言随 ARCH-016 重构（3d5312fc）重写——延续路由测试改为 `proto.Unmarshal` 解码后 `assert.NotEmpty(invokeResp.GetPayload())`（provider_drain_test.go:319）或固定字面量对比（`"echo:payload"` 对固定请求 `[]byte("payload")`，:266），全文件已无 `"echo:"+string(...)` / `GetPayload()==` 脆弱全字节串比较（`grep` 零命中）。证据：2026-10-31；`go test ./internal/agent/ -count=1` PASS（8.4s）；`go test ./internal/agent/ -run 'TestCallProvider_TaskContinuation|TestHandleInvoke_TaskContinuation|TestDrainProviderSession_WaitsForAcceptedTasks' -count=5` PASS。
- [x] 审核发现：govern.go full 链新增 CheckInvokePermission（R30-01）后，顶部「门序」注释列出 8 个唯一编号步骤（5=ACL、6=政策、7=输入），但实现内联注释把「函数权限/政策审批」与「输入契约」都标为 `// 7.`（两处并存），未同步为 6/7（internal/platform/functioninvoke/govern.go:166、internal/platform/functioninvoke/govern.go:175），与 ARCH-024 完成证据「全链统一」存在注释漂移。
  - 修复：`// 6.` 同步政策审批步号（原误标 `// 7.`）；`// 5.` ACL 与 `// 7.` 输入契约已一致，纯注释无功能行为变更。证据：2026-10-31；`go test ./internal/platform/functioninvoke/... -count=1` 全绿；`go build ./internal/platform/functioninvoke/` 过。
- [x] 审核发现：`reapStaleClaims`（R30-04 跨槽恢复）与 `resolveStaleClaim` 均把租约过期 claimed 解析为 `indeterminate`，但 `UpdateRunLog` 的 WHERE 仅含 `schedule_id+slot`、不含 `status=claimed` 再校验（internal/model/task_schedule_model.go:192-200）：并发路径下另一实例恰已完成派发并回写 dispatched 后，跨槽扫描仍按 createdAt<before 把它覆写成 indeterminate，可能覆盖真实终值（internal/tasks/scheduler/manager.go:186-200）。
  - 修复：`UpdateRunLog` 的 WHERE 增加 `AND status=claimed` 再校验——仅当行仍为 claimed 时才回填终值，否则 no-op 保留已落 dispatched/终态，陈旧 reclaim 不再覆写真实派发结果。证据：2026-10-31；`go test ./internal/model/ -run TestTaskScheduleModel_UpdateRunLog_GuardNonClaimed -count=1` PASS；`go test ./internal/model/ ./internal/tasks/scheduler/ -count=1` 全绿；新增 `TestTaskScheduleModel_UpdateRunLog_GuardNonClaimed`（正常回填 + 已派发后陈旧 reclaim no-op）。
- [x] 审核发现：本轮 diff 其余方面（机器 token 鉴权路径白名单与 scope 校验、RecordTaskEvent 单事务、任务在途/延续路由、依赖边界与架构 DAG 守卫、前端窄屏与本地化键、文档复核、CI 接线、blob→无类型字段迁移）未复现功能性问题，相关测试包均通过。
  - 证据：2026-11-01 复核属实——`go test ./internal/security/jwtutil/ -run 'TestSignMachine|TestParseMachine' -count=1`、`go test ./internal/model/ -run 'TestTaskRunModel_RecordTaskEvent' -count=1`、`go test ./internal/agent/ -run 'TestCallProvider_TaskContinuation|TestHandleInvoke_TaskContinuation|TestDrainProviderSession' -count=1`、`go test ./internal/svc/ -run 'TestAuthMiddleware_MachineToken_PathRestriction' -count=1`、`go test ./internal/svc/ -run 'TestMigrationsProbeEvents|TestApprovalDispatchClaimMigration' -count=1`、`go test ./internal/app/agent/ -run 'Test.*Boundary' -count=1` 均 PASS；未复现功能性问题，无需修复。
- [x] 审核发现：工作树存在 0 字节空文件 `internal/cluster/arch005_drill_test.go`（git 未跟踪），使 `go build ./...` 及任何 import `internal/cluster` 的包（internal/server、internal/platform/functioninvoke 等）报 `expected 'package', found 'EOF'` 构建失败——遗留调试/半成品物（internal/cluster/arch005_drill_test.go:1）。
  - 修复：删除该 0 字节空文件（未跟踪、无 git 变更可提交）；`go build ./...` 恢复 PASS。证据：2026-11-01；`internal/cluster/arch005_drill_test.go` 已不存在；`go build ./...` exit 0。
- [x] 审核发现：`cmd/server/cluster_extra_test.go:225` 遗留调试输出 `t.Logf("forward debug: OK=%v Error=%q NotOwner=%v", ...)`，非断言性调试日志，应移除或改为断言上下文（cmd/server/cluster_extra_test.go:225）。
  - 修复：移除该 `t.Logf`（原 :225，现删除后不再存在）。证据：2026-11-01；`go test ./cmd/server/... -count=1` PASS（556.7s）；commit `7412020a`。
- [x] 审核发现：896de78e 移除 `processMessage` 中 `touchRevenue(ctx, m)` 调用后，`internal/analytics/worker/worker.go:675` 的 `touchRevenue` 仅剩测试引用，成为生产死代码（internal/analytics/worker/worker.go:675）。
  - 修复：删除 `touchRevenue` 生产函数及对应纯内存测试（TestTouchRevenue_*）；`flush_agg_repro_test.go` 改用直接填充 `revAgg` 保持 flush 消费路径覆盖。证据：2026-11-01；`go test ./internal/analytics/worker/... -count=1` PASS（19.8s）；commit `55f79b82`。
- [x] 审核发现：f8d12842 将政策审批注释从 `// 7.` 改为 `// 6.`，但 `RequireGameEnvScope` 的 SubjectAutomation 复核块同样标注 `// 6.`（internal/platform/functioninvoke/govern.go:158），与顶部「门序」注释（EnforceFunctionPolicy=第 6 步、RequireGameEnvScope 有意不在链内）冲突，内联注释仍有两处 `// 6.` 漂移（internal/platform/functioninvoke/govern.go:158、internal/platform/functioninvoke/govern.go:166）。
  - 修复：`RequireGameEnvScope` 复核块改为「自动化主体补 scope 复核（有意不在 full 链内，见顶部门序）」标注，去掉 `// 6.` 编号，不再与链内 EnforceFunctionPolicy=第 6 步冲突；纯注释无功能变更。证据：2026-11-01；`go test ./internal/platform/functioninvoke/... -count=1` PASS（17.8s）；commit `aa1f74a3`。
- [x] 审核发现：`configs/server.yaml` 与 `internal/config` 均声明 `cluster.interconnectTls.enabled`（TLSClientConfig.Enabled），但 `cmd/server/cluster.go:143-147` 的 `useInterconnectTLS` 只按 cert/key/ca 三路径判断、从不读取 `.Enabled`——`enabled:true` 但缺证书时静默走明文，`enabled:false` 但证书齐备时仍启用 mTLS，配置面与实际行为不一致（cmd/server/cluster.go:143）。
  - 修复：`useInterconnectTLS` 增加 `cfg.InterconnectTLS.Enabled &&` 前置条件——仅 enabled 且 cert/key/ca 齐备才启用 mTLS，配置面与实际行为一致。证据：2026-11-01；`go test ./cmd/server/... -count=1` PASS（556.7s）；commit `2d9d5ba5`。
- [x] 审核发现：机器 token 鉴权路径复核发现遗漏——token 放入 `Authorization: Bearer` 时管理员 `Parse` 忽略未知 `tokenType` 字段，退化为空 Username/AdminID=0 的 claims 并绕过 token 版本查询，使机器 token 可冒充「空用户名管理员」身份。
  - 修复：`authenticate` 拒绝 AdminID=0 或 Username 空的退化管理员 claims，机器 token 不能经 Bearer 路径冒充管理员。证据：2026-11-01；`internal/svc/machine_auth_test.go:100` 的 `TestAuthMiddleware_MachineTokenInBearerRejected` 断言 401/authentication_failed；`internal/svc/service_context.go:1844` 拒绝退化 claims；`go test -count=1 -p 1 ./internal/svc/... ./internal/security/jwtutil/... ./internal/api/extension/... ./internal/app/agent/...` PASS。
- [x] 审核发现：opsNodeDrain/opsNodeUndrain 以 JSON 文本作为 MsgProviderDrainRequest 请求体发送（internal/api/ops/helpers.go:819、940），但 agent 侧用 proto.Unmarshal 解析该请求（internal/agent/local_handler.go:1084-1086、internal/agent/tcp_local_listener.go:396-398）；proto 解码 JSON 必失败（实测 `proto: cannot parse invalid wire-format data`），agent 侧 ProviderSession 的 draining 标记与在途等待永不生效，ARCH-016 drain/undrain 实际是空操作。 属实并已修：server 侧改 `proto.Marshal(&sdkv1.ProviderDrainRequest{})`（internal/api/ops/helpers.go:815-823），不再发 JSON 文本；`go test ./internal/api/ops -run TestOpsNodeDrainRegistryGate -count=1` 全绿；证据 commit `c949c20d`。
- [x] 审核发现：opsNodeUndrain 发送 `{"reason":"__undrain__"}` 依赖 reason 哨兵区分 undrain（internal/api/ops/helpers.go:940），agent 侧 DrainProviderSession 以 `req.Reason == ProviderDrainReasonUndrain` 判定（internal/agent/local_handler.go:1092）；但该哨兵经 JSON→proto 解码必然失败，且服务端发送时未 proto 编码，undrain 分支（清除 draining、恢复路由）永远无法触达。 属实并已修：undrain 下行改 `proto.Marshal(&sdkv1.ProviderDrainRequest{Reason: protocol.ProviderDrainReasonUndrain})`（internal/api/ops/helpers.go:938-945），哨兵随 proto 编码正常触达；`go test ./internal/api/ops -run TestOpsNodeDrainRegistryGate -count=1` 全绿；证据 commit `c949c20d`。
- [x] 审核发现：reapStaleClaims 每次仅 `ListStaleClaims(now-lease, 100)` 处理前 100 条且每条为单条无条件 UPDATE，无批量/分页游标；claim 速率高于清理速率时积压会长期滞留，且该扫描与 triggerSchedule 处理同一 `claimed` 槽位存在双重写入（internal/tasks/scheduler/manager.go:186-198）。 属实并已修：`reapStaleClaims` 改为按 `created_at` 游标分页循环（`ListStaleClaimsAfter`，internal/tasks/scheduler/manager.go:186-215、internal/model/task_schedule_model.go:202-222），积压>100 单轮全部收敛；新增 `TestManager_ReapStaleClaims_DrainsOver100`（internal/tasks/scheduler/arch023_test.go）验证 150 条单轮全清；`go test ./internal/tasks/scheduler ./internal/model -count=1` 全绿；证据 commit `5b4000dc`。
- [x] 审核发现：resolvePolicyRisk 中 RegistryStore.GetOpenAPI 的错误一律被忽略（`err == nil && op != nil`，internal/platform/functioninvoke/policy.go:50-58），但 GetOpenAPI 对「操作未找到」与「clone 序列化失败」都返回 error（internal/platform/registry/store.go:1126-1134）；clone 失败被当作「未声明风险」回落 medium，与 ARCH-025 fail-closed 精神不符（低概率但违背同函数注释「来源故障向上传播」）。 属实并已修：`GetOpenAPI` 引入可判别 `ErrOpenAPINotFound`（internal/platform/registry/store.go:1128-1142），`resolvePolicyRisk` 对未命中回落 medium、对 clone 序列化失败返回错误 fail-closed（internal/platform/functioninvoke/policy.go:56-67）；新增 `TestResolvePolicyRisk_OpenAPINotFoundFallsBack`、`TestGetOpenAPI_DistinguishesNotFoundAndCloneFailure`；`go test ./internal/platform/functioninvoke ./internal/platform/registry -count=1` 全绿；证据 commit `42b2437e`。
- [x] 审核发现：ClaimForDispatch 的 SQL WHERE 仅 `state='approved' AND claim_token=''`（internal/platform/approvals/sql_store.go:246-248），未含 `claim_status=''`；MemStore 判重用 `ClaimStatus!=''||ClaimToken!=''`（internal/platform/approvals/store.go:294-296），两者语义不一致——历史/异常行若 claim_token 空而 claim_status 非空，SQL 可二次领取而 MemStore 拒绝。 属实并已修：SQL WHERE 增加 `AND claim_status = ''`（internal/platform/approvals/sql_store.go:250），与 MemStore 判重语义一致；新增 `TestClaimForDispatch_SQLRejectsClaimStatusOnly`（internal/platform/approvals/cas_concurrency_test.go）验证二次领取被拒；`go test ./internal/platform/approvals -count=1` 全绿；证据 commit `e20ec075`。
- [x] 审核发现：SetTaskReaper 在 `s.mu.Lock()` 下写 watchdogBudget/watchdogInterval（internal/server/control_handler.go:258-264），而 runTaskWatchdog 在后台循环无锁读 `s.watchdogBudget`（internal/server/control_handler.go:1990-2000）；仅因文档规定「须在 StartBackgroundTasks 前调用」而靠 goroutine 建立 happens-before 免于现行 race，但字段读写无同步契约，属潜在数据竞争隐患。 属实并已修：`runTaskWatchdog` 在 `s.mu.RLock()` 下快照 taskReaper/watchdogBudget 后调用（internal/server/control_handler.go:2051-2060），`taskWatchdogLoop` 同样加读锁快照 interval（internal/server/control_handler.go:2066-2070）；新增 `TestControlService_WatchdogFields_RaceFreeRead`（internal/server/watchdog_test.go）；`go test -race ./internal/server -run Watchdog -count=1` 与 `go test ./internal/server -count=1` 全绿；证据 commit `e19e1dcb`。

- [ ] **DOC-001 · P2 · 文档 · 配置开关效果动态图**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。参考 kode todo C-24 同款流水线模式（2026-11-01 用户令全舰队推广）。
  - 交付与验收：枚举本仓全部「开启后有效果」的开关/配置项，自动流水线生成证据——GUI 类用 @vscode/test-electron 或应用截图（off 一张/on 一张+切换过程录屏），CLI 类用终端录屏（asciinema→GIF 或 ffmpeg），ffmpeg 缝合动态 GIF；docs「配置效果」页（双语）逐开关嵌入说明+off 图+on 图+GIF。验收：清单内每个开关都有 off/on+GIF；流水线可重跑（配置变更随时再生成）；兼作配置效果回归测试。

## 全仓代码深审（2026-11-02）——P2/P3 待办

- [x] **C-002 · P2 · 安全 · cloud-init runcmd 注入（root RCE）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`internal/ops/provision/template.go` 新增 `validateCloudInitExtra`/`validateCloudInitExtraLine`（逐行拒绝 shell 元字符 `$`/反引号/`;`/`|`/`&`/`<>`/`()`、`-` 前缀 YAML 嵌套、「解释器 + eval 标志」（`sh -c`/`bash -e`/列表 `[ sh, -c, ... ]` 绕过））；`renderCloudInit` 改返回 `(string, error)`（防御纵深，Extra 非法即拒渲染）；`orchestrator.go` `Validate` 边界拒绝非法 `CloudInitExtra`；`apply` 处理渲染 error。
  - 交付与验收：template.go 的 renderCloudInit 不再把 CloudInitExtra 原样拼入 runcmd——改为经 allow-list 校验（仅白名单命令）或拒绝含 shell 元字符的行；新增单测覆盖 Extra 含 `$(...)`/`;`/`|` 被拒；provision Validate 校验 CloudInitExtra 非空时格式；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。
  - 完成证据：`TestValidateCloudInitExtra_RejectsInjection`（11 类注入全拒：`$()`/反引号/`|`/`;`/`&&`/`>`/`()`/`[ sh, -c, ... ]`/`sh -c`/`bash -e`/`- `前缀 + 多行一行非法即拒）、`TestValidateCloudInitExtra_AllowsSafe`（列表 `[ systemctl, ... ]`/curl/bash 脚本/空行放行）、`TestRenderCloudInit_RejectsInjectionExtra`、`TestValidate_Matrix` 注入/放行子用例；`go test ./internal/ops/provision/` + `./internal/api/ops/` 全绿、`go build ./...` + `go vet` 通过；review 文档 C-002 行改「已修（2026-11-03）」。

- [x] **C-003 · P2 · SDK · JS SDK 畸形帧对端挂死（违 BUG-038 应答契约）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`sdks/js/src/index.ts`：`decodeInvokeRequest` 包 try/catch（protobuf decode 失败抛「malformed invoke request」）；`invokeInbound` 捕错回 `{error}` payload 帧；`handleInboundStartTask` 捕错回空 `taskId`（Agent 侧按失败处理）；`handleInboundCancelTask` 捕错回空响应。新增模块级 `InvokeRequestDecoded` 类型别名。与 Go/Py/C++ 行为一致（解析失败回错误帧而非静默挂死）。
  - 交付与验收：sdks/js 的 decodeInvokeRequest/handleInboundStartTask/handleInboundCancelTask 把 protobuf decode 移入 try/catch，解析失败时按契约回错误帧而非静默；与 Go/Py/C++ 行为一致；新增畸形帧单测断言对端收到错误响应而非挂起；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。
  - 完成证据：`inbound_client.test.ts` 新增 `describe("BasicClient malformed inbound frames (C-003)")`——非法 wire type 帧（`[0x0f]`）对 Invoke/StartTask/CancelTask 三型均回帧不挂死（invoke 回 `{error: /malformed invoke request/}`、start 回空 taskId、cancel 回空响应）；`npx tsc --noEmit` 0 错误；`npx jest` 全量 438 过/7 跳（含 inbound_client 10/10）；review 文档 C-003 行改「已修（2026-11-03）」。

- [x] **C-004 · P2 · 安全 · SSE token 入 query param**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`createEventSource` 改返回 fetch 实现 `SSEStream`（Authorization: Bearer 头 + SSE 帧解析 event/data/retry + 断连自动重连 + 非200 致命），JWT 不再进 URL；`useRealtimeStream` ref 类型改 `SSEStream`。LOCAL VERIFIED：`pnpm --dir web run tsc` 0 错误；`npx jest src/services/core/http.test.ts src/services/core/__tests__/http.node.test.ts src/pages/Analytics/Realtime src/services/api/messages.test.ts src/services/api/analytics.test.ts` 全绿（http 19/19 + Realtime 9 套件 95/95）；无其它 EventSource/token query 残留（grep 仅剩 2 个 createEventSource 调用点）。
  - 交付与验收：createEventSource 不再把 JWT 拼进 ?token= query（改走 EventSource 无法带 header 的替代方案：短效 stream token 或经 Authorization 的 fetch-stream）；token 不出现在 URL/日志/Referer；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-005 · P2 · 安全 · JWT 存 localStorage**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。**DEFERRED（2026-11-03）——架构级鉴权门改造，设计需用户签字**：修复必须 httpOnly cookie 迁移（localStorage 是唯一让 XSS 读走 token 的载体，等效安全存储只能是 cookie）；但改动横穿登录状态机（密码→MFA→mustChangePassword）、OAuth 回调 `?token=` 重定向交付（handler.go:320 经 oidcSuccessURL）、Logout 吊销、auth 中间件（service_context.go:1763 读 Authorization/?token=），外加 web 侧 Login/index.tsx / http.ts getToken / app.tsx / requestErrorConfig.ts 与各自测试。鉴权门是全站入口，cookie 属性（Secure/SameSite/CSRF 模型）与 OAuth 回调 token 交付方式属设计决策，须用户确认方向后再动（自治无人复核下改错即整站登录挂）。非静默丢弃——P2 优先级保留，待设计确认后回到本项。
  - 交付与验收：Login 改用 httpOnly+Secure cookie 或等效安全存储承载会话；localStorage 不再落 token；跨页面鉴权路径同步更新；新增单测断言 token 未写入 localStorage；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-006 · P2 · 安全 · 附件 URL 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`Detail.tsx` 附件 `a.url` 经 `safeExternalHref`（http/https 白名单）后才进 `<img src>` / `<a href>` / `window.open`；非法 scheme（javascript:/data:）渲染为 `<span>` 无链接文本；新增 C-006 用例断言 evil-js/evil-data 为 SPAN、无 `a[href^=javascript:]`/`a[href^=data:]`/`img[src^=data:]`、合法 http 仍链接、window.open 未调用。LOCAL VERIFIED：`npx jest src/pages/Support/Tickets/__tests__/Detail.test.tsx` 44/44 过（95s）+ `pnpm --dir web run tsc` 0 错误；review 文档 C-006 行改「已修（2026-11-03）」。
  - 交付与验收：Support/Tickets/Detail.tsx 附件 img src/a href/window.open 全部走 safeExternalHref（http/https 白名单）；非 http/https 渲染为无链接文本；新增单测覆盖 javascript:/data: 拒绝；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-007 · P2 · 安全 · 告警/图表 URL 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`Ops/Alerts/index.tsx` 5 处 `window.open`（grafana explore / AM `/#/alerts` / silences 深链 / drawer runbook / drawer grafana）全部先经 `safeExternalHref`（http/https 白名单），空结果不弹窗；alertmanagerUrl 缺省时 silences 深链不再向当前 origin 导航（此前打开不存在哈希路由）。LOCAL VERIFIED：`npx jest src/pages/Ops/Alerts/__tests__/index.test.tsx` 33/33 过（93s）+ `pnpm --dir web run tsc` 0 错误；review 文档 C-007 行改「已修（2026-11-03）」。
  - 交付与验收：Ops/Alerts 的 grafanaExploreUrl/alertmanagerUrl/runbook_url window.open 全部走 safeExternalHref；非法 scheme 不弹窗；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-025 · P2 · 正确性 · openapi provider 读已关 body 丢真实错误**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`provider.go:890` 关 body 仅在实际重试时（`i < RetryCount`），末次 5xx 留给 defer+ReadAll；`TestProviderCall_All5xxRetriesReturnRealError` 断言 3 次 5xx 后返回含 `500` 与真实响应体的 `API error`（非 read-on-closed），mutation 验证旧码必红。LOCAL VERIFIED：`go test ./internal/drivers/openapi/ -count=1` 绿（6.3s）+ `go build ./...` 过。
  - 交付与验收：provider.go 重试循环末次 5xx 后不提前关 resp.Body（延迟到 defer 或关前判 err）；新增单测：全 5xx 重试后返回真实 HTTP 状态/错误体而非 read-on-closed；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-026 · P2 · 正确性 · SlidingWindowLimiter.GetStats 越界 panic**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`advanced.go` GetStats 对空 `timestamps` 加 `len>0` 守卫返回零值 `LastRequest`；`TestSlidingWindowGetStats_EmptyWindow`（maxRequests=1、n=2 超限建空窗口后 GetStats 不 panic），mutation 验证旧码必红（advanced.go:230 index out of range）。LOCAL VERIFIED：`go test ./internal/platform/ratelimit/ -count=1` 绿（3.2s）+ `go build ./...` 过。
  - 交付与验收：advanced.go GetStats 对空 timestamps 加 len>0 守卫返回空/零值；新增单测：空窗口（超限建空/prune 到 0）调 GetStats 不 panic；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-008 · P3 · 安全 · agent exec 白名单仅匹配命令不匹配参数/环境**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`ops_server.go` 新增 `validateExecBoundary`（ExecuteCommand 内白名单命中后调用，fail-closed）：敏感环境键拒绝（`LD_PRELOAD`/`LD_LIBRARY_PATH`/`LD_AUDIT`/`DYLD_*`/`PATH`/`CDPATH`/`PYTHON*`/`BASH_ENV`/`ENV`/`SHELLOPTS`/`BASHOPTS`/`IFS`/`GIT_*`/`PERL*`/`NODE_OPTIONS`/`RUBY*`/`RUSTC_BOOTSTRAP`）；解释器（sh/bash/zsh/dash/ksh/fish/csh/tcsh/cmd/powershell/python/perl/ruby/node/php/lua）+ 内联代码开关（-c/-i/-p/-e/-r）拒绝；workingDir 必须绝对且不含 `..` 段。LOCAL VERIFIED：`go test ./internal/app/agent/ -run 'TestOpsServer_ExecuteCommand' -count=1` 全绿（含 C-008 三子例）+ 全包 `go test ./internal/app/agent/ -count=1` 绿（51s，首跑 79s FAIL 为已知 metrics 负载 flake 族，隔离全绿）+ `go build ./internal/app/agent/` 过；docs/operations/config-agent.md（en+zh）allowExec 节补第 4 条边界加固。
  - 交付与验收：ops_server exec 白名单校验扩展到 req.Args/req.WorkingDir/req.Env（PATH、LD_PRELOAD 等敏感键拒绝或白名单化）；新增单测：带 LD_PRELOAD/PATH 覆盖的请求被拒；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-009 · P3 · 安全 · DDL 拼接 dbName 仅反引号/方括号包裹**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`db.go` 新增 `validateDBName`（`^[A-Za-z0-9_-]+$` 严格白名单），`createMySQLDatabase`/`createPostgresDatabase`/`createSQLServerDatabase` 三者入口先校验再触碰 DSN/连接（fail-closed）；各方言断句字符（反引号/单引号/双引号/方括号/分号/空白）一律拒，不再依赖仅包裹符。LOCAL VERIFIED：`go test ./internal/svc/ -run 'TestValidateDBName_Allowlist|TestCreateDatabase_RejectsInvalidDBNameBeforeDSN' -count=1` 绿 + 全包 `go test ./internal/svc/ -count=1` 绿（50s，含既有 BadDSN 三例不破）+ `go build ./...` 过。
  - 交付与验收：db.go 建库 DDL 的 dbName 改经严格标识符校验（仅 [a-z0-9_-] 白名单）而非仅包裹符；SQL Server 遇 `]` 不再断；新增单测非法 dbName 被拒；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-010 · P3 · 安全 · asciicast sessionID 路径穿越（潜伏）**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`asciicast.go` 加 `validCastSessionID`（`^[A-Za-z0-9_-]+$`），`OpenCast` 在 `filepath.Join` 前 fail-closed 校验；新增 `asciicast_test.go` 四例（合法 id 落 dir、非法 id 全拒且 dir 外无文件、白名单独立断言、合法 id header 行落盘）；`go test ./agents/devops/session/` 全绿（0.011s）；docs/review/2026-11-01-code-review.md C-010 行已改已修（2026-11-03），汇总已修 13→14、余 19→18。
  - 交付与验收：asciicast.go 写 .cast 前校验 sessionID 仅 [A-Za-z0-9_-] 或经 filepath.Base；新增单测 sessionID 含 `../`/`/` 被拒；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-011 · P3 · 安全 · Java TlsSocketFactory 硬编码 keystore 口令**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`TlsSocketFactory.create` 签名加 `char[] keyStorePassword`（移除字面量 `"croupier"`），口令由调用方提供（无默认）；mTLS 配置而口令缺失 fail-closed 抛 `IOException`；全仓 6 个 TLS 测试类改用 4 参形式；新增 `TlsSocketFactoryEdgeTest.keyStorePasswordRequiredForMutualTls`（缺口令抛错+显式口令可构造）；`./gradlew test` 全绿（6 个 TLS 测试类 + 全量，BUILD SUCCESSFUL）；docs/review/2026-11-01-code-review.md C-011 行已改已修（2026-11-03），汇总已修 14→15、余 18→17。
  - 交付与验收：TlsSocketFactory keystore 密码改从配置/环境读取，移除字面量 "croupier"；无默认口令；新增单测无配置时启动失败而非静默用硬编码；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-012 · P3 · 正确性 · Python SDK varint 解码无边界检查**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`_decode_file_push_request` 三重 fail-closed——varint 解码前查 `idx >= n`（截断抛 `ValueError` 非 `IndexError`）、varint 超 10 字节抛 `ValueError`、length-delimited 长度超出 body 抛 `ValueError`（不再静默截断）；新增 `test_coverage_final.py` C-012 四例（截断 varint/varint 溢出/长度超 body/合法请求照常解码）；`pytest tests/` 全绿 551 passed；docs/review/2026-11-01-code-review.md C-012 行已改已修（2026-11-03），汇总已修 15→16、余 17→16。
  - 交付与验收：croupier/**init**.py varint 解码与切片加长度/边界校验，越界返回错误而非静默截断；新增畸形字节串单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-013 · P3 · 健壮性 · Go SDK 写错误丢弃**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`writeResponseFrame` 改返回 `error`，写失败即 `failAllPending()`（dead=true，在途 `Call` 即刻以 `errConnectionClosed` 失败）并回传错误；3 处调用方 `slog.Warn` 记录；新增 `tcp_client_whitebox_test.go` `TestWriteResponseFrameFailureFailsPending`；`go test ./pkg/croupier/...` 全绿；docs/review/2026-11-01-code-review.md C-013 行已改已修（2026-11-03），汇总已修 16→17、余 16→15。
  - 交付与验收：tcp_client.go Write 错误不再丢弃（回传调用方或断连）；新增单测写失败可感知；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-014 · P3 · 健壮性 · C++ SDK send best-effort**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`WriteResponseOnSocket` 移除裸 `(void)send`，捕获返回值——`sent < 0` 时 `std::cerr` 记录（errno/WSAGetLastError）；`catch(...)` 拆为 `catch(std::exception)` + `catch(...)` 均记录；新增 `test_coverage_boost4.cpp` `WriteResponseOnClosedSocketDoesNotThrow`（关闭 fd → send EBADF，不抛异常）。验证：`g++ -fsyntax-only src/tcp_transport.cpp` 干净；test 文件因既有 protobuf 版本约束（gencode protoc 34.x vs 本机 runtime 3.21.12/33.4.0 不兼容，非本变更）本地无法编译，用例仅用文件既有符号；docs/review/2026-11-01-code-review.md C-014 行已改已修（2026-11-03），汇总已修 17→18、余 15→14。
  - 交付与验收：tcp_transport.cpp send 返回值/异常不再整体吞掉（至少日志）；移除 (void)send 裸调用；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-015 · P3 · 健壮性 · Java ServerHttpInvoker 吞 IllegalArgumentException**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-015），修复方向 source-wins。
  - 完成证据：`serverErrorMessage` 的 catch 由 `catch (IllegalArgumentException ignored) {}` 改为 `catch (IllegalArgumentException parseError) { logger.warn(...) }`（slf4j `logger` 字段新增，对齐 InvokerImpl 模式），仍回退原始 body 不抛异常；新增用例 `sdks/java/.../ServerHttpInvokerServerErrorTest`（5 例：取 message / 空 message 取 error / 无二者回退原始 / 不可解析回退原始且不抛 / 空或 null 回退占位）；`./gradlew test` 全绿。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：捕获后记录日志/上抛，不再 ignored 静默；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-016 · P3 · 安全 · hostdriver 硬编码测试探针 token**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-016），修复方向 source-wins。
  - 完成证据：`probe-credentials` 的 `const secret = "hd-t0k3n-9f8e7d"` 移除，改为 `resolveProbeSecret()`——`HOSTDRIVER_PROBE_SECRET` 注入时透传，否则 `crypto/rand` 生成 24 字节（48 hex）每运行随机值；非测试路径不再含固定凭据。新增用例 `sdks/plugins/hostdriver/main_test.go` `TestResolveProbeSecret_Injected` / `TestResolveProbeSecret_RandomWhenUnset`（注入透传；未注入随机、非空、两调不撞、非旧字面量、48 hex 形态）；`go build ./sdks/plugins/hostdriver/` 过 + `go test` 绿 + `go vet` 干净。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：hostdriver 探针 secret 改从环境/配置注入，移除字面量 "hd-t0k3n-9f8e7d"；非测试路径不得含固定凭据；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-017 · P3 · 安全 · Dev/Tools tool.url 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-017），修复方向 source-wins。
  - 完成证据：`web/src/pages/Dev/Tools/index.tsx` 卡片 map 内 `const safeUrl = safeExternalHref(tool.url)`（http/https 白名单）；export 图标改 `if (safeUrl) window.open(safeUrl, '_blank', 'noreferrer')`；标题锚改 `safeUrl ? <a href={safeUrl}> : <span>`（非法 scheme 渲染为无链接文本）。新增用例 `Dev/Tools/__tests__/index.test.tsx` `C-017 非法 scheme 工具链接`（javascript:/data: 两工具：标题无 `<a>`、export 点击不 `window.open`）；`pnpm tsc` 0 错误 + `pnpm test -- --testPathPatterns pages/Dev/Tools` 14 passed。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：Dev/Tools window.open/a href 走 safeExternalHref；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-018 · P3 · 安全 · compositeRuntime navigate window.open 缺 noopener**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-018），修复方向 source-wins。
  - 完成证据：`web/src/components/PageRenderer/compositeRuntime.tsx` navigate 分支改 `const safeUrl = safeExternalHref(step.params?.url)`；`if (safeUrl) window.open(safeUrl, '_blank', 'noopener')`——非法 scheme（javascript:/data: 等）不开窗，新窗口不反向控制 opener。用例 `compositeRuntime.test.tsx`（更新既有断言加 noopener；新增 `C-018 navigate 非法 scheme：不开窗`——javascript:/data: 两步均被拦截、window.open 不触发）；`pnpm tsc` 0 错误 + `pnpm test -- --testPathPatterns PageRenderer/__tests__/compositeRuntime` 12 passed。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：compositeRuntime navigate 加 noopener 与 scheme 白名单（safeExternalHref）；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-019 · P3 · 安全 · Ops/Alerts 配置 URL 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-019），修复方向 source-wins。
  - 交付与验收：配置 URL window.open 走 safeExternalHref；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-020 · P3 · 健壮性 · useRealtimeStream try/finally 无 catch**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-020），修复方向 source-wins。
  - 交付与验收：JSON.parse/推送加 catch 处理并上报，finally 仅收尾不吞错；新增单测畸形消息不抛未捕获异常；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-021 · P3 · 测试 · TestMetricsCollector_StartReporting 负载 flake**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-021），修复方向 source-wins。
  - 交付与验收：复跑稳定或加 t.Parallel 隔离/放宽超时归因环境性；记录到 CI 基线说明（非回归，勿当红豁免）；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-027 · P3 · 正确性 · devops hub 写超时 goroutine 泄漏**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-027），修复方向 source-wins。
  - 交付与验收：hub.go write 超时路径取消/关闭阻塞写 goroutine（或改用带超时写原语）；done 通道超时路径读取；新增单测超时后 goroutine 数不增长；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-028 · P3 · 正确性 · mysql_pump flush 无 ctx.Done 阻塞**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-028），修复方向 source-wins。
  - 交付与验收：mysql_pump out<- 加 select ctx.Done()（对齐 PG pump）；新增单测消费者停止后 Close 可解除阻塞；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-029 · P3 · 正确性 · server/agent 跨 lane handler 字段竞态（低概率）**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-029），修复方向 source-wins。
  - 交付与验收：tcp_listener.go 与 tcp_local_listener.go 的 Handle 对 registered/agentID/sessionID 读改走 lane 内同步（或经 mux 保证 happens-before）；`go test -race` 相关用例绿；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-030 · P3 · 正确性 · externalplatform cache.Close 双关 panic**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-030），修复方向 source-wins。
  - 交付与验收：cache.Close 加 sync.Once/closeOnce 幂等；新增双调 Close 单测不 panic；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-031 · P3 · 健壮性 · telemetry 每 exporter 每 span 无界 goroutine**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-031），修复方向 source-wins。
  - 交付与验收：telemetry ExportSpans fan-out 加有界（semaphore/WaitGroup 上限）并处理错误；新增高 span 速率下 goroutine 有界单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-032 · P3 · 正确性 · mysql syncer.Close 错误丢弃**
  - 依赖：—。
  - 状态／条件：TODO（未开始）。2026-11-02 全仓代码深审发现（C-032），修复方向 source-wins。
  - 交付与验收：mysql.go StartSyncGTID 失败与 Close 路径记录 syncer.Close 错误；新增单测 close 失败可见；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

## 审核发现（ed541f6ed11ff1df5038d90fda9daa9711fa2cf9..HEAD，2026-11-02 只审不改）

- [ ] 审核发现：供给 cloud-init 的 CloudInitExtra（POST /ops/provision body 用户可控）逐行原样写入 runcmd，作为 root 在新建实例执行，可注入任意命令（RCE）。（internal/ops/provision/template.go:47-52）
- [ ] 审核发现：deploy 投递暂存路径 /tmp/croupier-deploy-<pid>-<version> 由未净化的用户 version 拼接，version 含 ../ 可致 curl -o 在 agent 主机任意路径写文件（任意文件写/代码执行）。（internal/api/ops/deploy_agent.go:43）
- [ ] 审核发现：供给提交直接采用 body 的 gameId/env，未与已认证 X-Game-ID/X-Env scope 比对，作用域受限用户可向越权游戏/环境供给云资源（scope 绕过）。（internal/api/ops/provision.go:120-121）
- [ ] 审核发现：batchop 创建在 body 提供 gameId/env 时覆盖中间件解析的授权 scope，可越权创建/派发到未授权游戏环境。（internal/api/ops/handler.go:956-961）
- [ ] 审核发现：analytics daily_users 刷新对 Redis PFCount 错误置 0 后直接以新 version 写入 ReplacingMergeTree(version)，0 行永久覆盖当日真实 DAU/NAU（与 minute_online 的 remerge 兜底路径不一致）。（internal/analytics/worker/worker.go:830-831）
- [ ] 审核发现：供给 ticket 模式续行丢失 HubAddr/BootstrapToken/CloudInitExtra——runFromModel 未恢复（json:"-" 不持久化）、ResumeAsync 不再注入 server 参数，续行渲染出空 hubAddr/token 的 cloud-init，agent 引导必然失败。（internal/svc/provision_wiring.go:93-126；internal/api/ops/provision.go:155）
- [ ] 审核发现：供给 user_data（多行 cloud-init YAML）经 tfexec.Var("-var user_data=<值>") 传入，terraform/tofu 按 HCL 表达式解析（#cloud-config 起始为注释）致真实 apply 失败；FakeRunner 测试掩盖该路径（测试缺口）。（internal/ops/provision/runner.go:52-53）
- [ ] 审核发现：logstore Ingest 写库前先推进内存 lastSeq（:91）且忽略 db.Create 错误（:104），写库失败批次被重传当作重复丢弃、persisted 虚报——单进程内该批日志永久丢失。（internal/platform/registry/logstore.go:91,104）
- [ ] 审核发现：供给异步 apply（spawnApply）在 Init/Apply 失败时只记日志不落 failed（apply 仅等注册分支置 failed），run 永久停在 applying 不可重试。（internal/ops/provision/orchestrator.go:194-255,362-369）
- [ ] 审核发现：供给 beginResume 为 Get→pending 检查→置 applying→Update 非原子读改写，并发 ResumeAsync 均可通过 pending 检查触发双 apply（applyMu 在 beginResume 之后才锁）。（internal/ops/provision/orchestrator.go:336-357）
- [ ] 审核发现：供给 DestroyResume 未先推进状态即执行 runDestroy（并发双销毁），且 Destroy/runDestroy 不持 applyMu、不校验 run 状态，可直接与在途 apply 对同一 workdir 竞态（terraform state 破坏）。（internal/ops/provision/orchestrator.go:375,411,437）
- [ ] 审核发现：供给请求 Count 无上限（仅校验 <1），可一次性创建海量云实例造成失控成本/资源。（internal/ops/provision/orchestrator.go:111-113）
- [ ] 审核发现：batchop Execute 无状态闸，已完成/运行中批次可被重复派发；且在请求 ctx 上执行、UpdateStatus 错误全被 `_ =` 丢弃，断开后批次停 Running 且终态不落库。（internal/ops/batchop/batchop.go:187,196,243）
- [ ] 审核发现：deploy Start 状态闸在调用方载入副本上非原子判断（:126），并发 Start 均可通过并双跑首批；且 Rollback 在 stage/apply 失败时仍标记 RolledBack 上报成功（假成功）。（internal/ops/deploy/deploy.go:126,229-242）
- [ ] 审核发现：deploy 编排在 HTTP 请求 ctx 上同步执行（deployments.go:195），客户端断开即取消整个发版；Advance 递归携带已取消 ctx 中途终止，且健康门禁 Healthy 错误被忽略。（internal/api/ops/deployments.go:195；internal/ops/deploy/deploy.go:216,271）
- [ ] 审核发现：deploy 包上传 io.ReadAll 无大小上限，可无界读入内存耗尽。（internal/api/ops/deployments.go:136）
- [ ] 审核发现：metrics 小时聚合磁盘 usage_percent 累加未除样本数（:657 累加、:713 直接 round2），可超 100% 且非均值，与 CPU/MEM/net（除 n）语义不一致。（internal/platform/registry/metrics.go:657,713）
- [ ] 审核发现：metrics SetDB 写 s.db 未持 s.mu，与 Add/persistToDB/getFromDB/RollupHourly 读侧数据竞态（LogStore.SetDB 有锁，此处不一致）。（internal/platform/registry/metrics.go:186-188）
- [ ] 审核发现：analytics processMessage 对 retry_count 做未检查类型断言（:1004），外部流消息若为非 float64 类型则 panic 崩 worker（Run 循环无 recover）。（internal/analytics/worker/worker.go:1004）
- [ ] 审核发现：scheduler reapStaleClaims 游标按非唯一 created_at 分页（task_schedule_model.go 用 >= cursor）且 UpdateRunLog 错误被 `_ =` 丢弃（manager.go:280），边界同时间戳+持续更新失败时可反复取同页空转。（internal/tasks/scheduler/manager.go:280-288）
- [ ] 审核发现：cluster 互联 TLS 现需 InterconnectTLS.Enabled=true 才启用（cluster.go:147），既有证书已配置但未显式 enabled 的部署会静默降级为明文互联（就绪面有降级记录，属行为变更需人工确认）。（cmd/server/cluster.go:147）
- [ ] 审核发现：供给 cloud-init 的 AgentInstallURL 未做 shell 引号（template.go:45），含 ;/|/$() 元字符可注入；该值来自 server 配置 provision.agentInstallUrl（配置级，非直接远端输入）。（internal/ops/provision/template.go:45）
- [ ] 审核发现：deploy 对象存储 key 由未净化的 role/name/version 拼接（deployments.go:245），../ 可逃逸 deploy/ 前缀写入任意对象键。（internal/api/ops/deployments.go:245）
- [ ] 审核发现：deploy 包上传的 bytesReader 每次 Read/Seek 都新建 bytes.Reader（不保持游标），Read 永不返回 EOF——objstore 各驱动 Put（file/s3/cos/oss 均 io.Copy 直到 EOF）将死循环，写无限增长（fileStore 撑爆磁盘 / S3 超大对象），且上传请求永久挂起。（internal/api/ops/deployments.go:250,424-429）
- [ ] 审核发现：deploy 包上传无对应 objstore.Put 集成测试（仅测了 model CRUD），bytesReader 死循环在测试面完全未覆盖。（internal/model/model_test.go:6222-6247；internal/api/ops/deployments.go:250）
- [ ] 审核发现：agent_metrics_hourly 表只建了非唯一复合索引 idx_agent_hour（迁移 0051 CreateTable），RollupHourly 的 find-or-create 非原子（metrics.go:612-617）——多 server 实例或清理 tick 与手动触发并发时同 (agent_id, hour_start) 会插重复行，90 天聚合被污染。（internal/platform/registry/metrics.go:56-57,612-617；internal/svc/migrations.go:1735-1737）
- [ ] 审核发现：RollupHourly 无并发测试（metrics_hourly_test.go 单线程），find-or-create 竞态与重复行未覆盖。（internal/platform/registry/metrics.go:572）
- [ ] 审核发现：PushFile 仅校验目标路径的父目录符号链接（EvalSymlinks(filepath.Dir)），未解析目标本身——预置符号链接于目标路径可让 os.WriteFile 写到 PushBaseDir 之外（写入前 exists/force 检查与写入间亦有 TOCTOU）。（internal/app/agent/ops_pushfile.go:68-73,75-79,97）
- [ ] 审核发现：batchop/agent exec 白名单仅按命令名精确匹配（ops_server.go:384-395），args 不受限；经 OPS-006 /ops/batch exec 类型，白名单内命令（如 curl）可携带任意参数（写文件/外联），白名单边界弱于「命令+参数」双约束。（internal/app/agent/ops_server.go:384-395；internal/ops/batchop/batchop.go:270-275）
- [ ] 审核发现：deploy 包同 (role,name,version) 重复上传覆盖同一对象键（deployments.go:245），历史 DeployPackage 记录仍引用该键但 checksum 与对象内容不再匹配——ApplyUpdate 以旧记录 Checksum 校验必失败，或（若跳过校验）旧记录被新内容静默替换，供应链完整性受损。（internal/api/ops/deployments.go:245,263-267）
- [ ] 审核发现：worker Run 退出序列在周期性 flush 协程超时（shutdownTimeout）未结束时仍启动最终 flush（worker.go:251-277），两个 flush 并发执行——I/O 段在 aggMu 外，同 key 双写 ClickHouse 产生重复/竞态。（internal/analytics/worker/worker.go:251-277）
- [ ] 审核覆盖：本轮（ed541f6e..HEAD，217 文件）逐文件审查已完成——重点覆盖 provision 引擎（runner/template/orchestrator/types）、deploy/costcontrol/batchop 编排、agent ops（pushfile/exec/log tailer）、registry metrics/logstore、analytics worker、tasks scheduler、api/ops（provision/deployments/logs/batchops/helpers）、svc wiring/migrations、前端 safeUrl/Bugs/NodeDetailDrawer、objstore（file/s3/cos/oss）。生成代码（pkg/pb、sdks/_/generated、ops.pb._）与纯文档/图片/测试文件（仅 model CRUD 类）未逐行审；结论见上方本轮与既有发现。未 commit、未 push、未改代码。
