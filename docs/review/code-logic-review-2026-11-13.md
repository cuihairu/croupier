# Croupier 代码逻辑续审（2026-11-13）

基线：`083d23829`。此提交仅记录 todo，生产代码与上一轮 `7247a5db4` 一致；本轮关键文件指纹见 [source-sha256.json](evidence/2026-11-13/source-sha256.json)。

按用户要求，本轮重点是代码实现的业务正确性：持久化适配、状态机、取消、并发调度、插件生命周期和正常协议处理。没有继续执行权限绕过或攻击式探针，没有调用真实云平台、更新程序或电源设备。

本轮确认 **10 项逻辑问题**：3 项为已有导入清单的重新核实（AUD-019/020/021），7 项为本次深入调用链后补充的发现（AUD-022..028）。8 项用正常业务回归用例验证，2 项为跨文件静态确认。没有修改业务实现，所有条目仍待修。

## 优先处理

### AUD-019 · P1 · 供给运行的正常状态更新实际变成重复插入

- 入口链：`provisionDBStore.Create/Update` → `runToModel` → `ProvisionRunModel.Update`。
- `internal/svc/provision_wiring.go:26-27` 每次 Update 都重新转换一份模型；`:67` 只设置 RunID，不回填数值主键 ID。
- `internal/model/provision_model.go:62-63` 用 GORM Save；ID=0 时执行 INSERT，而不是按 RunID 更新。
- 结果：Create 成功后，正常将状态从 pending 改成 applying，Update 就返回 `UNIQUE constraint failed: provision_runs.run_id`。不是“可能多一行”：现有唯一键会直接阻止更新。

影响包括 ticket 申请落审批关联、apply 后记录云资源输出、推进等待注册与最终状态等需要 Update 的路径。外部操作可能完成而状态保存失败。

`provision_wiring_test.go:76` 的现有测试注释明确跳过 adapter.Update；仅验证映射与 CAS 的测试全绿不能证明生产持久化链可用。应按 RunID 条件更新或正确保留主键，并增加真正的 Create→Update→Get 集成测试，包含零值字段清除和不存在记录处理。

验证：`TestReviewProvisionAdapterCanUpdateExistingRun` 断言正常 Update 应成功；当前失败，日志显示实际 INSERT 和唯一键冲突。

### AUD-027 · P1 · 供给 CAS 的竞争失败分支发生空指针 panic

`internal/model/provision_model.go:68-90` 在 CAS 未命中时能够返回当前行，但 `internal/svc/provision_wiring.go:30-34` 遇 ok=false 一律改为 `(false,nil,nil)`。

上层 `internal/ops/provision/orchestrator.go:391/430/493` 在失败分支仍读取 `current.State`。这三个点分别影响审批续行、直接销毁和审批后销毁。

正常并发场景：两个请求都已经读取 pending，另一请求先赢得 CAS；当前请求的 CAS 失败后应返回状态冲突，实际 panic。现有 fakeStore 在 CAS 失败时返回非空当前行，与生产 adapter 返回契约不同，因而掩盖问题。

验证：使用真实数据库适配器，在 Get 与本次 CAS 之间安排另一领取先完成；`ResumeAsync` 的 `require.NotPanics` 失败，堆栈落在 orchestrator.go:391。未启动 Terraform 或后台 apply。

修复应统一接口返回契约并在调用方防御 nil；三个失败分支都要覆盖，不能只修当前测试触发的一个位置。

### AUD-028 · P1 · 同一次每日计划跨分钟被领取两次

`internal/tasks/scheduler/manager.go:309` 用 `Slot(now)` 计算领取键，唯一约束为 `(schedule_id,slot)`。这里的 now 是本次扫描/处理时刻，而不是这条已读取记录的 NextTriggeredAt。

两个实例都读到了同一条 10:30 的每日待执行记录；一个在 10:30:59 处理，另一个在 10:31:00 处理。它们会使用两个不同的分钟键，分别成功领取并派发。第一个实例推进下次时间，不能撤销另一个实例已经读取的快照。

验证：两份同一到期快照、两个 Manager、共享领取 Store；正常调用 triggerSchedule 后，期望派发 1 次，实际 `player.cleanup` 派发 2 次。没有依赖机器时钟漂移、线程休眠或外部服务。

应把领取身份绑定到同一次计划发生时间，并结合版本/条件更新处理陈旧快照。需要保留目前“先领取后派发”和结果未知时不自动重派的正确语义。

### AUD-020 · P1 · 事故 details 创建能成功，编辑却报数据库类型错误

`internal/api/incident/service.go:331-333` 将 `req.Details` 的裸 `map[string]interface{}` 放入 Updates；`internal/model/incident.go:137` 直接交给 GORM。普通 map 没有字段声明中的 `datatypes.JSONMap` 的序列化行为。

验证：正常创建事故，随后提交 `details:{summary:"new"}` 更新，返回 `sql: converting argument $1 type: unsupported type map[string]interface {}, a map`。相同内容在创建路径可用，不是请求 JSON 格式错误。

应在更新边界使用合适的 JSON 类型/序列化，再测试嵌套对象、空对象及重读结果。该问题已在导入清单，本轮把“建议修改”落实为完整调用链和失败证据。

### AUD-023 · P1 · 活跃发版单可在未做任何操作时显示回滚成功

`internal/ops/deploy/deploy.go:230-270` 的 Rollback 没有发版状态判断或 CAS；只遍历 healthy/failed 目标。发版刚开始时，数据库可能只有 deploying 单据和 pending 目标（目标进度按批写入）。

此时收到另一个回滚请求，Rollback 会跳过全部目标，最后仍写 rolled_back、返回 nil。原发版执行不会因此被取消，之后还能继续换包并写 complete。

验证：构造正常的 deploying/pending 快照并调用 Rollback，未发生任何 Agent Apply，仍无错误并进入 rolled_back；“应拒绝或协调正在发版的请求”的回归断言失败。

应明确发版和回滚的互斥领取与可转换状态，保证回滚时使用实际最新进度；没有执行任何回滚不能掩盖仍在运行的发版。

## 其他逻辑与生命周期问题

### AUD-021 · P2 · 合法 IPv6 ping 应答被当成失败

`core/healthprobe/icmp.go:96-101` 的 requestType/replyType 由 IPv4 常量推导为 `ipv4.ICMPType`，IPv6 分支仍强制转换成该类型。后续 `icmp.ParseMessage(58, ...)` 返回的应答 Type 是 `ipv6.ICMPType`。

`received.Type == replyType` 比较接口时不仅比较数值，也比较动态类型，因此合法的 IPv6 EchoReply 永远不相等。

验证：使用现有 PacketConn 测试替身返回 ID、Seq、Data 均匹配的标准 IPv6 应答，期望成功，实际返回 `network: ICMP echo reply code 0`。没有发包到真实网络。

应分别使用正确的 IPv4/IPv6 类型，并分别保留正常应答测试。读 Echo.Data 时还应核对长度后再读 uint64；本轮没有继续构造异常报文测试。

### AUD-022 · P2 · 部署在批次等待取消时丢失已完成目标进度

`internal/ops/deploy/deploy.go:161-210` 已经完成 Stage/Apply/健康检查，但只修改局部 targets；`:213-220` 先进行可取消的 BatchWait，之后才 saveTargets 和 Update。

取消发生在换包已完成、等待尚未结束时，函数直接返回 ctx.Err，存储仍将目标标为 pending。恢复方看不到已经完成的实际操作。

验证：正常 fake Agent 完成一次换包及健康检查后取消编排上下文；期望持久结果为 healthy，实际 pending。

**范围限定**：BatchWait 默认是 0，本测试主动设置该可选等待。HTTP DeploymentStart 使用 WithoutCancel，因此不能把本例说成“默认 HTTP 请求一断开就发生”。这是编排器可取消等待路径的持久化顺序问题。应先落已完成事实，再进入等待，并定义中断状态；不应通过去掉全部取消支持绕过问题。

### AUD-024 · P2 · Runbook 的“共享恢复循环”实际按请求重新创建

跨文件链路：

1. `internal/api/ops/runbooks.go:90-101` 每次服务调用都 `runbook.New(...)`，没有复用编排器。
2. `internal/ops/runbook/runbook.go` 的 New 为每个编排器创建独立 recoveryLoop。
3. 终态连续写入失败时 `:398` 启动该对象的后台 loop。
4. `:405-447` 在写入成功、pending 清空后仍永久等待 wake，没有停止入口或生命周期 context。

因此“一个共享有界 worker”的保证只在一个临时对象内部成立。多个 HTTP 运行分别遇到暂时 DB 错误，会各自留下独立后台 worker；DB 恢复后仍不能回收这些等待者。

本项为静态调用链确认，没有给出未经测量的泄漏数量或内存增长数据。修复应把恢复器按 Server 生命周期装配并关闭，或让无待办且无人再使用的循环能够退出；要用真实 Service 的多次调用验证，而不只测试手工复用同一 Orchestrator。

### AUD-025 · P2 · Runbook 重启恢复通常跳过最近遗留记录，之后不再扫描

`cmd/server/root.go:309-317` 只在启动时执行一次 RecoverStaleRunbookRuns，筛选边界是 now−24h；`internal/api/ops/runbooks.go:163` 将宽限定为 24 小时。

正常短时宕机重启时，上个进程最近留下的 running 记录都在宽限内，会被跳过。当前进程之后没有周期恢复调用；即使运行一周，这些行也不会因“已超过宽限”自动重新进入检查。

源码中的 24h 宽限是为了避免误处理其他实例活跃任务，应保留这个考虑；不能简单改成启动时全部失败。需要进程/执行所有权、租约或等效的安全恢复机制，再安排后续巡检。

本项为静态确认，不声称测试过真实双实例宕机。

### AUD-026 · P2 · 插件启动持全局状态锁，其他调用的 deadline 不能生效

`core/pluginhost/engine.go:267-350` 的 ensure 持 Controller.mu 调用 startLocked，后者同步启动子进程、Dispense、Describe。Call 开头的 isStopped 也需要这个锁；后面的 acquire 超时和 per-call context 尚未开始保护它。

验证：使用已有真实 echo 测试子进程，将第一次 Describe 暂停；第二个正常调用携带 10ms deadline，在 100ms 后仍阻塞于全局锁。释放 Describe 后才继续返回。测试结束会停止子进程。

这也会使其他插件的入口与 Stop 等待同一把锁。配置的启动超时可以限制部分启动步骤，但不等于调用方 deadline 受到遵守。

应将进程启动/握手 I/O 移出全局状态锁，以按插件的首次启动合并与可取消等待保持一致性；不应仅把业务 RPC timeout 再调小。

## 本轮检查过但没有新增结论的路径

- 供给 beginResume 已有 CAS；缺陷在实际 adapter 与失败分支契约，不能继续报成“完全没有 CAS”。
- 批操作 Execute 已在派发前领取 draft→running；dry-run 在当前设计中是终态，没有仅凭“预览后不能复用同一批次”就判缺陷。
- Runbook 对 ContinueOnFailure 有 partial 聚合，终态写入有重试；问题在生产装配的 worker 所有权与恢复调度，不是说完全没有错误处理。
- 插件 Host 的 Stop 会等待 reconcile、Controller 有停止/排空；本轮新增问题为启动锁持有范围，没有把正常有界等待直接判为死锁。
- Agent 更新流程已做哈希检查、停后换包、备份与稳定窗口；游戏进程维护窗口和兼容检查仍按已有明确边界记录，没有凭猜测补设计需求。

## 验证记录与后续覆盖

[回归用例源码与日志](evidence/2026-11-13/README.md) 已保存。测试断言的是正确业务行为，**当前 8 个新增回归断言均失败**；这表示已定位实现缺陷，不能宣称新测试通过或功能已修复。测试通过 Go overlay 注入，没有往业务目录提交失败测试。

原有 9 个相关包测试通过（incident、deploy、provision、runbook、healthprobe、metrics、pluginhost、funcplugin、scheduler）。原有测试通过与新增用例失败可以同时成立，说明的是覆盖缺口。

该轮只深入上述模块与调用链。全部 REST 页面、六语言 SDK 的所有行为、真实游戏接入、外部依赖故障、压力长跑及原生实机仍未完整审完，AUD-COV 覆盖任务保持未完成；不能将这份续审报告当成全仓最终签收。
