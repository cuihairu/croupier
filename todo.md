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

- [ ] **ARCH-005 · P1 · 故障验证 · 集群故障摘流与重连**
  - 依赖：ARCH-003, ARCH-004。交付与验收：双实例注入协调库断连、互联占端口、owner 退出、旧 epoch；逐例记录拒绝、摘流、恢复及副作用次数。
  - 状态／条件：BLOCKED：需双实例 + 隔离网络（本沙箱单实例，无法注入协调库断连/互联占端口/owner 退出/旧 epoch 的真实故障验证）；缺真实双实例环境。

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

- [ ] **ARCH-009 · P1 · 实现 · 数据库首次打开的取消与预算**
  - 依赖：—。交付与验收：让 resolver/open/migrate 消费 context；取消等待及时返回、同库 singleflight、不同库隔离，暴露池数/首开耗时；与 ARCH-006 路由命名修复可并行，合并时核对绑定行为
  - 状态／条件：R30-06 CONFIRMED：取消等待、ensure/migrate context、singleflight 和统计已实现；5ms OpenBudget 下阻塞的 cfg.Open 在 40ms 后仍不返回，释放后才报超时。须约束实际 Open/驱动初始化，不能仅在后续 PingContext 补检查。 详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。

- [ ] **ARCH-010 · P2 · 性能验证 · 多 game/env 连接池容量**
  - 依赖：ARCH-009。交付与验收：在给定 game/env 数、并发、数据库连接配额下记录池数/连接峰值/P95 首开耗时；给出回收阈值与容量结论。
  - 状态／条件：NOT RUN：需容量目标和独立数据库。

- [ ] **ARCH-011 · P1 · 实现 · 分析 worker 持久化后确认**
  - 依赖：—。交付与验收：修正 worker.go 批量 Append 尚未 Send 即 XACK、重入队错误未核对的问题；杀进程及写库/入队故障下每条消息可核对，说明允许重复与去重策略。
  - 状态／条件：局部修复已验证：Send 成功后 ACK、失败保留 pending、重入队/死信 XADD 错误不 ACK。原验收中的真实杀进程/写库故障逐条核对尚未运行（ARCH-012），不据局部单测勾完整验收；聚合字段竞态另由 DATA-014 修复。 详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。

- [ ] **ARCH-012 · P1 · 故障验证 · 分析管道崩溃恢复**
  - 依赖：ARCH-011。交付与验收：在 Append 后/Send 前、Send 后/ACK 前、重入队失败时中断；比对源消息、PEL、死信与 ClickHouse 行，证实无静默丢失。
  - 状态／条件：NOT RUN：需 Redis 与 ClickHouse 可控环境。

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

- [ ] **ARCH-016 · P1 · 实现 · Provider drain 真正摘流**
  - 依赖：—。交付与验收：当前 handleDrain 仅记录并回空 ACK；实现停止新路由、等待在途及超时退出语义，验证 ACK 语义与执行结束条件。
  - 状态／条件：R30-09 CONFIRMED：已具 Draining/RPC 在途计数/路由门；但 StartTask 已接收而未进入 Provider RPC 时，drain 立即 ACK 并摘注册，后续 pickInstance 报未注册。必须覆盖已接收任务与固定 Provider 归属；原子准入竞争验证 NOT RUN。 详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。

- [ ] **ARCH-017 · P1 · 实现 · Server 停机收尾顺序**
  - 依赖：—。交付与验收：控制 listener.Close 等活跃会话时不阻塞后续 HTTP 停机/根取消；保证 ServiceContext.Close 冲刷留痕，设置有界清理并验证 socket/goroutine/池释放。
  - 状态／条件：R30-07 源码时序：停收→HTTP drain→rootCancel→等待→Close 的正常路径有回归通过；但外层 30s 返回后仍执行无界 defer StopScheduler，阻塞 tick 可令最终返回继续等待。完整阻塞调度/总截止/池释放故障验收 NOT RUN，不宣称一律有界。 详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。

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
  - 状态／条件：NOT RUN；需要 GAME-002 接入的权威玩家函数；demo 验收不能代替此项。

- [ ] **UI-002 · P1 · 故障验证 · 示例异常状态全链（旧 E2）**
  - 依赖：—。交付与验收：复用现有 Go gameops-demo 的 33 函数及内存状态，独立 Server/sidecar/demo 栈验证发布、审批、report.generate、失败/未知/取消/幂等重试；既有 fixture 只证机制，不证明进程重启后游戏侧持久去重
  - 状态／条件：NOT RUN；可用仓库样例在本地搭栈，不以缺真实游戏账号阻塞此项。

- [ ] **UI-003 · P1 · 故障验证 · 批量确认与默认呈现实页验收（旧 D2/D5）**
  - 依赖：UI-002。交付与验收：复用 perTarget、字段语义和五模板；跨页选人、筛选变化、总量确认与实际提交一致，发布回读保留身份列/单位/时区/枚举；不重复规划已实现 UI。
  - 状态／条件：代码侧 2026-10-31 收口：合并确认框（总量+动作级目标同框）、MAX_SAFE_INTEGER/乘积溢出 fail-closed、players 补偿空目标 fail-closed；`resourceBatchSelection.test.tsx` 覆盖跨页选人/筛选变更/清除/翻页不误报/目标预填/确认内容与提交一致；仍需发布态 demo 栈实页验收。commit 0d74c456

## 真实游戏接入

- [ ] **GAME-001 · P1 · 资料调查 · 真实游戏权威数据与执行契约**
  - 依赖：—。交付与验收：向接入方整理玩家/角色/封禁/订单/邮件/活动权威读写入口、身份映射、凭据与测试目标；核对现有 ownership/high-risk 设计，交付缺口矩阵。
  - 状态／条件：NOT RUN：需接入方契约/样本/测试账号。

- [ ] **GAME-002 · P1 · 实现 · 接入真实玩家工作台**
  - 依赖：GAME-001, GAME-008。交付与验收：复用现有玩家模板和发布链，列表/详情/操作以权威游戏回读及执行凭据验收，失败不显示成功。
  - 状态／条件：NOT RUN：需目标游戏环境。

- [ ] **GAME-003 · P1 · 实现 · 接入真实邮件与补偿**
  - 依赖：GAME-001, GAME-008, GAME-010, GAME-012, ARCH-024。交付与验收：复用模板/审批/任务，按对象凭据核对发放、部分失败重试和重复操作不重发。
  - 状态／条件：NOT RUN：需目标游戏环境及受控写权限。

- [ ] **GAME-004 · P1 · 实现 · 接入真实封禁管理**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用封禁模板，身份/期限/原因批准快照一致，封禁解封后权威回读可核对。
  - 状态／条件：NOT RUN：需目标游戏环境及受控写权限。

- [ ] **GAME-005 · P1 · 实现 · 接入真实订单核查**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用订单模板，订单/发货权威状态一致，补发有凭据且重试不重复发货。
  - 状态／条件：NOT RUN：需测试订单与受控补发权限。

- [ ] **GAME-006 · P1 · 实现 · 接入真实活动配置**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用活动模板，发布/回读版本一致，失败保留旧版本并有审计。
  - 状态／条件：NOT RUN：需测试配置和发布权限。

- [ ] **GAME-007 · P1 · 方案决策 · 评审游戏权威归属与统一目标引用**
  - 依赖：GAME-001。交付与验收：复用已有 game-authority-ownership 和 player-identity-target-model 设计，决定本地 Player/Balance 的演示/副本定位以及账号/角色/区服/渠道/合服映射来源；交付批准的映射与迁移边界，禁止另建平台游戏经济真值
  - 状态／条件：READY；既有设计仍待评审，不能把文档已完成当业务已实现。

- [ ] **GAME-008 · P1 · 实现 · 统一游戏目标引用与歧义阻断**
  - 依赖：GAME-007。交付与验收：实现已批准的 TargetRef 解析并供页面/审批/任务/审计复用；同角色 ID 多区服、账号多角色、合服旧引用明确解析，歧义拒绝，不以 gameId/env 或主机代替业务区服
  - 状态／条件：NOT RUN；方案批准后实施，需接入方身份映射样本。

- [ ] **GAME-009 · P0 · 方案决策 · 评审高风险操作凭据与未知结果协议**
  - 依赖：GAME-001。交付与验收：复用 high-risk-operation-contract-design，决定稳定 operationId、参数/目标指纹、批准快照、游戏持久去重与收据查询职责；明确响应丢失后先查凭据及重试/取消/补偿边界，不重建与 taskId/page idempotency 重复的系统
  - 状态／条件：READY；设计存在但实现协议未批准，禁止将平台去重当游戏 exactly-once。

- [ ] **GAME-010 · P0 · 实现 · 游戏持久执行凭据与未知结果收敛**
  - 依赖：GAME-009, GAME-008。交付与验收：按批准协议贯通游戏适配器、执行结果和凭据查询；同 operationId 改参数拒绝、响应丢失/服务重启不重复发奖、未知结果可查询收敛；取消不谎称回滚，补偿引用原凭据
  - 状态／条件：NOT RUN；需游戏方持久去重/查询实现与受控写环境；Go demo 的内存记录不能替代。

- [ ] **GAME-011 · P1 · 方案决策 · 评审批量运营冻结与预算恢复规则**
  - 依赖：GAME-009。交付与验收：复用 batch-operations-compensation-design，评审名单快照、预演人数/预算、审批后变化处理、单位/倍率、速率并发、失败阈值、暂停恢复和逐对象凭据；冻结后的对象/金额变更重新审批
  - 状态／条件：READY；前端跨页选择和 perTarget 确认不等于服务端批次协议。

- [ ] **GAME-012 · P1 · 实现 · 服务端批次对象账本与受控恢复**
  - 依赖：GAME-011, GAME-010。交付与验收：复用任务体系实施批准的快照/逐对象状态与预算约束；变更名单不可偷换批准范围，部分失败只重试可确认未执行对象；暂停恢复/阈值停止/最终核对可验，不以一次 onExecute 返回成功代替批次完成
  - 状态／条件：NOT RUN；需批准的批次方案和游戏执行收据。

## Agent 与隔离

- [ ] **AGENT-001 · P0 · 故障验证 · 四类 Agent 当前接口的 scope 隔离矩阵**
  - 依赖：—。交付与验收：按当前已支持的注册/配置/列表/统计/详情/动作/日志/离线读取逐项核对；复用非空 scope 和绑定测试，覆盖同 ID 冲突及跨 scope 指派；devops 未接通的会话/回放列 NOT IMPLEMENTED，不用假数据当验收
  - 状态／条件：NOT RUN；本地模拟与四角色栈可以开展；未来 devops 会话隔离随 AGENT-002/003 验收，不把缺中继误写成只缺环境。

- [ ] **AGENT-002 · P1 · 实现 · devops 面板会话中继**
  - 依赖：—。交付与验收：接通已有 Broker/Hub 的创建/数据/窗口/关闭通道与 Server 授权、审计、背压；不新增 RDP/VNC；SSH 真目标端到端连接和断线清理可验。
  - 状态／条件：NOT RUN：需 SSH 测试主机/凭据/指纹。

- [ ] **AGENT-003 · P1 · 实现 · devops 主机/会话/回放界面**
  - 依赖：AGENT-002。交付与验收：复用四类 Agent 公共页与现有 asciicast；真实会话列表、scope 权限、动作审计与回放可达，未实现能力不以假按钮替代。
  - 状态／条件：NOT RUN：需 SSH 测试主机。

## 探测验收与调查

- [ ] **PROBE-001 · P1 · 故障验证 · 真实游戏协议的多语言插件验收**
  - 依赖：—。交付与验收：复用 go-plugin 宿主、Go/Python/Node SDK；真实游戏目标验证 Describe/Check、凭证不回显、超时/取消/崩溃/熔断/恢复；不再规划 JSONL 等价运行时。
  - 状态／条件：NOT RUN：需真实协议、目标与凭据。

- [ ] **PROBE-002 · P1 · 故障验证 · 第三方服务多视角探活验收**
  - 依赖：PROBE-004, PROBE-005。交付与验收：针对各角色实际能力建立矩阵，分别核对 sidecar 的 Checker/策略热更、synthetics 原生 HTTP/ICMP/插件、面板阶段/窗口/故障恢复；模拟端口通但业务错、断线和恢复；不宣称所有角色共享全部 DB/私有协议 Checker
  - 状态／条件：NOT RUN；本地目标可先做机制验证，真实网络视角需目标/凭据；动态同步和丢失语义仍有实现/决策前置。

- [ ] **PROBE-003 · P2 · 资料调查 · 玩家 Login 级适配边界**
  - 依赖：—。交付与验收：复用已有 Minecraft SLP 与私有协议骨架，收集 online-mode/自家游戏登录协议与账号条件；SLP 状态检查不能作完整玩家登录证据。
  - 状态／条件：NOT RUN：需 MC Java 账号/出网、自家游戏账号/协议；许可证变更另行授权。

- [ ] **PROBE-004 · P1 · 实现 · synthetics 目标及策略版本同步**
  - 依赖：—。交付与验收：按既有角色设计补齐静态配置到受控版本拉取/热生效，复用 core/configsync 并绑定 gameId/env；坏版本保留 last-good，重连后取最新，禁用/删除目标释放资源；不凭 sidecar 的 puller 宣称 synthetics 已拥有同能力
  - 状态／条件：VERIFIED 未实现：synthetics Runner 仅遍历 cfg.Synthetics.Targets，不存在 configsync/中心策略拉取装配；可本地实施。

- [ ] **PROBE-005 · P1 · 方案决策 · 探测结果与故障恢复事件丢失语义**
  - 依赖：—。交付与验收：区分可丢的周期采样与必须可核对的故障/恢复跃迁；评审断链缓存/重传/新鲜度/缺测状态及容量预算，优先复用既有 report/告警机制，不默认持久化每条采样
  - 状态／条件：READY；synthetics.execute 断链只日志且通知无应用 ACK；真实第三方环境不能补齐此协议缺口。

## 待批准的运维方案

- [ ] **OPS-001 · P2 · 方案决策 · Agent 自升级四问**
  - 依赖：—。交付与验收：评审升级源/触发权限/重启接力/失败姿态，复用现有指纹及最后已知锚点；交付失败不砖化与带外回滚验收方案，等待既有要求的决策批准后实施。
  - 状态／条件：READY：待方案批准。

- [ ] **OPS-002 · P2 · 方案决策 · 受管程序配置更新五问**
  - 依赖：—。交付与验收：明确路径沙箱/格式校验/原子性与重启回滚/体积带宽/审批闸，复用 configsync 先例；不声称现有探针配置拉取已支持程序文件分发。
  - 状态／条件：READY：待方案批准。

## 部署事实

- [ ] **DEPLOY-001 · P1 · 资料调查 · 真实部署与外部依赖配置盘点**
  - 依赖：—。交付与验收：核对实际镜像摘要/角色/端口/TLS/协调后端/备份、Herald 与外部平台开关、OIDC/Vault/观测出口；交付脱敏拓扑和与模板差异，不把 Compose 存在当部署证据。
  - 状态／条件：NOT RUN：需只读部署权限与脱敏配置。

- [ ] **DEPLOY-002 · P1 · 故障验证 · 备份可恢复与业务凭据核对**
  - 依赖：DEPLOY-001。交付与验收：复用 backupexec 和既有备份/对象存储，在隔离环境实际恢复配置、契约/页面、审批/任务/执行凭据与角色数据，记录 RPO/RTO/校验和及未覆盖内存状态；验证副作用不因恢复重复执行，不新增未经设计的恢复服务
  - 状态／条件：NOT RUN；需脱敏备份、隔离数据库/对象存储读权限；备份 API/文件存在不证明恢复成功。

## 容量验证

- [ ] **PERF-001 · P2 · 性能验证 · 会话与重连容量基线**
  - 依赖：—。交付与验收：给定 SDK/Agent 数、并发、消息大小和延迟目标，测 MuxConn 队列、重连风暴、连接峰值/P95 和资源释放；单独记录已知正确性缺口，不用吞吐通过替代安全结论
  - 状态／条件：NOT RUN；需容量目标与隔离环境；任务 SSE 和插件容量拆为 PERF-002/003。

- [ ] **PERF-002 · P2 · 性能验证 · 任务查询与 SSE 容量基线**
  - 依赖：—。交付与验收：给定并发任务/订阅者、事件频率和保留量测量事件写入/查询延迟、断线续读、慢客户端背压及资源释放；记录当前可靠性缺口，不能用吞吐替代 ARCH-015 验收
  - 状态／条件：NOT RUN；需独立任务基准栈与延迟目标。

- [ ] **PERF-003 · P2 · 性能验证 · 插件并发与故障隔离容量**
  - 依赖：—。交付与验收：复用现有 go-plugin gate/backoff/breaker，给定插件数/并发/预算验证队列拒绝、超时、崩溃风暴、宿主资源和 Stop 释放；报告平台/版本，不做新的插件运行时选型
  - 状态／条件：NOT RUN；需可控插件负载与资源预算。

## 共用核心分析与外部专业分析

- [ ] **DATA-001 · P1 · 方案决策 · 内置共用指标与外部专业分析边界**
  - 依赖：—。交付与验收：按用户确定的边界评审核心指标：新增、DAU/WAU/MAU、近实时在线、D1/D7/D30 留存、付费人数/订单/流水/退款/净额/付费率/ARPU/ARPPU；定义活跃事件、账号/角色去重、cohort/分母、UTC/业务时区、迟到更正、币种/单位及近似误差。复杂自助分析/归因/预测走外部，已有行为/关卡页保留但不扩建专业 BI
  - 状态／条件：READY；复用 docs/analytics/core-and-external.md 和既有 game-metrics-overview，口径未验收不称已可用。

- [ ] **DATA-002 · P1 · 实现 · 共用事件接入与指标数据来源归一**
  - 依赖：DATA-001。交付与验收：复用 ingest/Server 事件入口/worker，提供游戏登录、注册、心跳、支付/退款的显式映射；每个指标声明权威来源、gameId/env/维度和新鲜度。统一 GORM 与 ClickHouse 读适配契约，不静默把平台 Player 数当游戏新增，不默认为两套库自动同步
  - 状态／条件：VERIFIED 缺口：overview.go 读关系模型/Player，warehouse.go 读 ClickHouse；需接入样本，代码可先本地实施。

- [ ] **DATA-003 · P1 · 实现 · 活跃与在线聚合的版本和重放语义**
  - 依赖：DATA-001。交付与验收：修正 daily_users 的替换键/版本参数与分钟在线 HLL 快照重写策略，迁移已有表而不只改 initdb；同日多次 flush、迟到事件、重放和多 worker 的同一分钟结果可核对，不把全量 HLL 重写当增量相加
  - 状态／条件：VERIFIED 源码与官方语义：010_analytics.sql 把 version 放入 ORDER BY 且未作为 ReplacingMergeTree 参数；minute_online 用 SummingMergeTree 接收 HLL 全量快照。真实 ClickHouse 验证 NOT RUN。

- [ ] **DATA-004 · P1 · 实现 · 付费聚合的币种、订单状态与累计口径**
  - 依赖：DATA-001。交付与验收：复用 payment 明细，按已批准币种/单位和订单唯一键处理重复成功、退款和状态变更；修复 revAgg flush 后删除而日表按替换读的增量/累计冲突，跨 flush/重启可回算，不能把不同币种原始金额直接相加
  - 状态／条件：VERIFIED 代码 touchRevenue 按 day/game/env 分组，未含 currency/order 去重，flush 后清空；真实金额样本与 ClickHouse 验证 NOT RUN。

- [ ] **DATA-005 · P1 · 实现 · 基础 cohort 留存的生产计算链**
  - 依赖：DATA-001, DATA-002。交付与验收：复用 RetentionModel，补齐注册 cohort 与回访事件的受控计算/回算任务；D1/D7/D30 明确分母、业务日、观察窗成熟与迟到更正；未成熟/缺数据返回不可用，不能伪造 0；用逻辑时钟和回放样本验算，无需等真实运行 30 天才写机制测试
  - 状态／条件：VERIFIED 缺口：生产路径只有 ListCohorts，未找到 UpsertCohort 的生产调用；前端概览 d1/d7/d30 固定 null。

- [ ] **DATA-006 · P1 · 实现 · 统一核心指标查询响应与 scope**
  - 依赖：DATA-002, DATA-003, DATA-004, DATA-005。交付与验收：复用现有 Analytics API/warehouse 查询，统一核心指标的窗口、维度、分母、source/freshness/approximation、币种和不可用原因；读取已有授权 scope，不让查询参数改到其它 game/env；缺配置/延迟/查询失败不能伪装零值
  - 状态／条件：READY；依赖有效聚合口径后交付，复用既有函数不建第二套同名 API。

- [ ] **DATA-007 · P1 · 实现 · 运营核心分析页的真实数据闭环**
  - 依赖：DATA-006。交付与验收：复用 Overview/Realtime/Retention/Payments/Warehouse 组件，收敛为同口径核心视图；过滤条件/日期/时区/单位保持一致，展示延迟、缺测、未成熟和未启用状态，真实零值可区分；已有专业页不删除也不以硬编码值补齐
  - 状态／条件：READY；现有页面/API 是可复用资产，当前不宣称完整产品闭环。

- [ ] **DATA-008 · P1 · 故障验证 · 核心指标可核对样本与端到端验收**
  - 依赖：ARCH-011, DATA-007。交付与验收：以固定多 game/env/跨日/多币种/重复乱序/退款样本，从接入→Redis→worker→查询→页面逐指标对账；验证重启/迟到/跨 scope 拒绝与不可用状态，记录原始样本、期望、实际、版本和数据新鲜度
  - 状态／条件：NOT RUN；可先本地构建样本栈，真实游戏以脱敏事件和支付对账样本补验；不以现有单测总数代替。

- [ ] **DATA-009 · P2 · 性能验证 · 核心分析吞吐与查询容量**
  - 依赖：DATA-006。交付与验收：给定事件速率、保留量、查询并发与新鲜度预算，测写入/聚合/查询 P95、积压和回算对控制面的影响；复用现有独立分析管道，不自动引入新的流处理服务
  - 状态／条件：NOT RUN；需业务量级和隔离 Redis/ClickHouse 环境。

- [ ] **DATA-010 · P1 · 资料调查 · 专业分析外部产品接入核查**
  - 依赖：—。交付与验收：按实际专业需求收集外部分析平台官方版本/许可、数据输入、授权 scope、身份/嵌入/深链、数据回流与成本限制；交付对接矩阵与单个首接目标，不在本轮选型或虚构任何供应商集成已完成
  - 状态／条件：NOT RUN；需专业场景与候选外部环境，优先官方资料，记录核查日期。

- [ ] **DATA-011 · P1 · 方案决策 · 外部专业分析的受控集成契约**
  - 依赖：DATA-001, DATA-010。交付与验收：复用 extensions 与既有数据出口，评审指标/事件/只读数据集的输出契约和来源、scope、字段映射、时间窗、版本、短期凭据/SSO/深链及审计；外部查询必须被服务端或受限数据集约束，不把浏览器过滤器当隔离，不泄露仓库凭据
  - 状态／条件：READY；方案依首接产品能力核对，不新增通用 BI/任意 SQL 平台。

- [ ] **DATA-012 · P1 · 实现 · 首个专业分析平台适配与入口**
  - 依赖：DATA-011。交付与验收：按批准契约实现一个真实外部平台的配置/连接验证、受控数据输出和当前 scope 的报告入口；复用扩展安装/开关/审计，失联有明确降级信息，不阻塞 Croupier 核心运营与内置指标
  - 状态／条件：NOT RUN；需首接平台测试实例及最小权限凭据。

- [ ] **DATA-013 · P1 · 故障验证 · 外部分析授权与故障隔离验收**
  - 依赖：DATA-012。交付与验收：核对跨 game/env、撤权、过期凭据、外部故障、超时和重复导出；未授权数据不可见，导出可追溯，外部平台不可用时内置核心指标与运营执行可继续
  - 状态／条件：NOT RUN；需真实外部分析环境和多 scope 测试身份。

- [ ] **DATA-014 · P1 · 实现 · 分析聚合共享状态串行化**
  - 依赖：—。交付与验收：复用 worker 的 touchedMinutes/touchedDays/revAgg；明确消费、pending 回收与 flush 的快照/同步所有权，失败重试不丢增量；收入并发写读及活跃桶增删在 race 下无竞争，固定样本核对总量，不仅给 ClickHouse batch 加锁
  - 状态／条件：R30-05 CONFIRMED：独立 race 诊断命中 touchRevenue 写与 flush 读同一字段；本轮只登记，不改聚合程序。

- [ ] **DATA-015 · P2 · 实现 · 分析 worker 退出等待与刷新资源收尾**
  - 依赖：—。交付与验收：Run/main 对 pending 回收、定时 flush 和最终 flush 建立有界退出/等待顺序；退出后不再读写 worker/测试配置，不在刷新未结束时关资源；复现既有 runFlushInterval 跨用例竞态并验证消失，阻塞刷新仍遵守总截止
  - 状态／条件：VERIFIED：既有 Run 定向 race 出现 runFlushInterval 写/迟到后台读取竞争；Run 不 join 子协程，main 仅等待固定 200ms。需区分测试夹具与真实生产生命周期证据。

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

## 依赖安全证据复核

- [ ] **SEC-001 · P1 · 资料调查 · 默认分支依赖告警适用性复核**
  - 依赖：—。交付与验收：获取 GitHub 默认分支未关闭告警的官方明细，映射 go.mod/go.sum、package.json/锁文件与 SDK/构建产物；区分运行时/开发/传递依赖、受影响版本与实际可达条件，逐项形成“需修复/不适用及依据/待验证”清单，再拆最小修复任务，不仅凭告警总数宣布存在可利用生产漏洞。
  - 状态／条件：NOT RUN；2026-10-29 推送 56c85dbb 的 GitHub 回执报告 19 条（1 high、10 moderate、8 low），仅回执已观察，告警详情/适用性尚未核实；读取详情可能需要仓库安全告警权限。

## 审核发现

- [x] 审核发现：`govern.go` 注释声称「门序（与 Invoke 既有顺序逐条对齐）」且将 `ValidateInvokeInput` 列为第 6 步（在 `EnforceFunctionPolicy` 之后），但实现实际在 `EnsureFunctionAssigned` 之后（第 3 步）即调用 `ValidateInvokeInput`（internal/platform/functioninvoke/govern.go:127），先于 `LoadCurrentAdmin`（:137）与 `EnforceFunctionPolicy`（:157）；同时注释称 `RequireGameEnvScope`「有意不在此链内」，实现却在 `SubjectAutomation` 分支内调用（internal/platform/functioninvoke/govern.go:149）。后果：原 `Invoke` 顺序为「政策审批→输入契约」，Govern 改为「输入契约→政策审批」，无权且载荷非法的调用方由 403（权限）变为 400（输入契约）——文档与实现、以及与原 `Invoke` 行为均不一致。属实并已修：full 阶段 `ValidateInvokeInput` 移回政策审批后（第 8 步），`phaseInvocability` 保持禁用/分配/契约；`RequireGameEnvScope` 注释改为「仅 SubjectAutomation 补复核（第 6 步）」，不再宣称入口 scope 有意不在此链。证据：`TestGovern_PolicyPrecedesInputContract` 断言无权+非法载荷仍 403；commit 0d74c456
- [x] 审核发现：`ResourcePageRenderer.submitActionForm` 对带 `perTarget`/quantity 字段且 `action.confirm`/`requireConfirm` 的批量动作（如 `player.batchRecharge`）现在依次弹两个确认框——先「核对跨目标总量」再「确认操作」（web/src/components/PageRenderer/ResourcePageRenderer.tsx:607、web/src/components/PageRenderer/ResourcePageRenderer.tsx:664）；改动前仅弹总量确认框（且 total 未定义时静默跳过），改动后总量框不再可跳过且多出第二个动作确认框，形成双重确认 UI。属实并已修：合并为单次「确认操作」弹窗，总量条目（每人数量/目标数量/总量）与动作级确认文案+目标对象同框，取消即中止；无动作级确认时保留总量确认标题。证据：`resourceBatchSelection.test.tsx` 与 `ResourcePageRenderer.test.tsx` REVIEW-004 用例改断言单次确认框+总量+目标；commit 0d74c456
- [x] 审核发现：`mailCompensation` 与 Go 端 `mailTaskCreate` 对 `receiverRange==='players'` 且目标数组为空时，`targetCount` 回退为 1、`targetPlayerIds=[]`、`targetPlayerId=null`，生成「声明 1 个目标但无任何目标标识」的任务（web/src/components/PageRenderer/templates/mailCompensation.tsx:435、sdks/go/examples/gameops-demo/executors.go:392）；表单 `required` 门仅部分缓解，空玩家级选择仍可能落成 targetCount=1 的失真快照。属实并已修：Web/Go 双端 fail-closed 拒绝 players 补偿无目标创建（明确错误），不落任务/操作记录/审批；原有 all/server 补偿默认 1 目标行为与 legacy 单值反填保持不变。证据：`mailCompensation.test.tsx` fail-closed 用例断言任务/操作记录零新增、`TestGameOpsMailTaskCreatePlayersWithoutIDsRejected`；commit 0d74c456
- [x] 审核发现：新增 `batchTotalEntry`（web/src/components/PageRenderer/confirmContent.ts:80）与 `ResourcePageRenderer` 总量计算均以 JS 浮点直接 `num * targetCount`（web/src/components/PageRenderer/confirmContent.ts:97），未做安全整数/乘积溢出检查，仅用 `toPrecision(12)` 做显示收敛——与已挂起 REVIEW-005（拒绝非安全整数与乘积溢出）同类缺口，新的复合批量总量路径未覆盖该契约。属实并已修：`summarizeQuantityTotal` 收口为「有限非负安全整数 × 正整数安全整数 = 安全整数」单一契约，`batchTotalEntry` 复用同函数并移除 `toPrecision(12)` 失真显示；非法输入一律不显示可确认总量，ResourcePageRenderer 同口径阻断提交。证据：`compositeBatchTotal.test.tsx` 与 `resourceBatchSelection.test.tsx` 新增 MAX_SAFE_INTEGER 邻界/溢出/Infinity 用例；commit 0d74c456
