# TODO

更新：2026-10-29。当前执行清单以源码核对为依据；本轮重新按游戏运营后台复核代码与待办；实现、工作区在途修复、定向验证及真实环境验收分别记录，不把代码存在或测试总数作为完成依据。优先级 P0 > P1 > P2；依赖 `—` 表示无本清单前置，不代表无环境条件。每个复选框是独立交付；READY 表示可开展；NOT RUN 只表示该项验收未运行，不等于必须等待用户或外部环境；LOCAL VERIFIED 表示所注明的工作区定向证据已取得，未发布。

旧台账完整保存在 [整理前历史](docs/review/todo-history-2026-10-29.md)，已交付片段不再重列为实现任务。原 C2/E2/D2/D5 的重复父条、阻塞条合并为验收任务；过期行号依赖移除。架构审核 A01–A09 对应 ARCH-001–020，不表示修复完成。仅设计完成的记录不能当实现完成。2026-10-29 再审发现：原清单遗漏定时派发竞争、执行入口治理缺口、策略故障放行、游戏身份/凭据/批次协议落地与灾备验证；新增任务见下方稳定编号。

当前执行顺序：先收口 ARCH-023/024/025 的调度领取、统一治理与政策故障语义；真实高危游戏写入须具备 GAME-009/010 的批准协议与游戏持久收据。审批/任务/停机等既有可靠性缺口并行推进。分析先做 DATA-001 口径，再修现有聚合/接入闭环；DATA-010 可并行调查专业平台，避免在核心未可靠前扩建 BI 或继续堆页面。

## 架构与可靠性

- [ ] **ARCH-001 · P1 · 方案决策 · 审批并发与执行领取语义**
  - 依赖：—。交付与验收：基于现有 SQLStore 定义 pending 原子转换、续跑领取及重复请求规则；评审表覆盖批准/拒绝竞争、重复批准、执行结果未知。
  - 状态／条件：方案草案见 [中英文审批并发设计](docs/zh/design/approval-concurrency-design.md)；CAS 防并发重复派发与持久执行领取不同，持久意图/恢复仍待决策，未实现。
- [ ] **ARCH-002 · P1 · 实现 · 接入审批 SQL 存储**
  - 依赖：ARCH-001。交付与验收：复用 internal/platform/approvals/sql_store.go；A 创建、B 读取/批准、重启后可查，并发仅一决定且仅一执行领取；不能只改构造器。
  - 状态／条件：NOT RUN：需共享数据库双实例。
- [ ] **ARCH-003 · P1 · 实现 · 集群互联身份认证**
  - 依赖：—。交付与验收：增加专用 TLS 配置及身份绑定，拒绝无证书/错 CA/Agent 身份/未握手 forward；合法 invoke/start/cancel 保留 epoch fencing。
  - 状态／条件：NOT RUN：需测试 CA 与双实例。
- [ ] **ARCH-004 · P1 · 实现 · 集群就绪及失败降级语义**
  - 依赖：—。交付与验收：独立存活/就绪探针；配置要求集群时协调库、互联监听或租约不可用返回 ready=503，显式配置才允许 standalone 降级。
  - 状态／条件：READY。
- [ ] **ARCH-005 · P1 · 故障验证 · 集群故障摘流与重连**
  - 依赖：ARCH-003, ARCH-004。交付与验收：双实例注入协调库断连、互联占端口、owner 退出、旧 epoch；逐例记录拒绝、摘流、恢复及副作用次数。
  - 状态／条件：NOT RUN：需隔离网络和双实例。
- [ ] **ARCH-006 · P1 · 实现 · 物理数据库路由统一真值**
  - 依赖：—。交付与验收：复用授权后的 binding.database_name，消除前缀重算；验收旧名/自定义名/前缀变更/下划线碰撞/并发首次开库，已有绑定有显式迁移。
  - 状态／条件：READY。
- [ ] **ARCH-007 · P2 · 方案决策 · 版本化审计摘要与失败策略**
  - 依赖：—。交付与验收：列出 scope/Changes/Action 等规范化哈希字段、旧版本兼容和写失败策略；外部签名是否需要单列决策。
  - 状态／条件：READY。
- [ ] **ARCH-008 · P2 · 实现 · 落地审计完整性规则**
  - 依赖：ARCH-007。交付与验收：逐字段修改被检测、旧链可验证、多实例写入一致；审计库故障按已批准策略处理。
  - 状态／条件：READY。
- [ ] **ARCH-009 · P2 · 实现 · 数据库首次打开的取消与预算**
  - 依赖：—。交付与验收：让 resolver/open/migrate 消费 context；取消等待及时返回、同库 singleflight、不同库隔离，暴露池数/首开耗时；与 ARCH-006 路由命名修复可并行，合并时核对绑定行为
  - 状态／条件：READY；不需要先获得生产数据库权限即可做确定性验证。

- [ ] **ARCH-010 · P2 · 性能验证 · 多 game/env 连接池容量**
  - 依赖：ARCH-009。交付与验收：在给定 game/env 数、并发、数据库连接配额下记录池数/连接峰值/P95 首开耗时；给出回收阈值与容量结论。
  - 状态／条件：NOT RUN：需容量目标和独立数据库。
- [ ] **ARCH-011 · P1 · 实现 · 分析 worker 持久化后确认**
  - 依赖：—。交付与验收：修正 worker.go 批量 Append 尚未 Send 即 XACK、重入队错误未核对的问题；杀进程及写库/入队故障下每条消息可核对，说明允许重复与去重策略。
  - 状态／条件：READY。
- [ ] **ARCH-012 · P1 · 故障验证 · 分析管道崩溃恢复**
  - 依赖：ARCH-011。交付与验收：在 Append 后/Send 前、Send 后/ACK 前、重入队失败时中断；比对源消息、PEL、死信与 ClickHouse 行，证实无静默丢失。
  - 状态／条件：NOT RUN：需 Redis 与 ClickHouse 可控环境。
- [ ] **ARCH-013 · P2 · 实现 · 阻止不支持的分析组合**
  - 依赖：—。交付与验收：配置/启动预检识别 Kafka 发布配内置 Redis-only worker；默认 Redis 链不受影响，不扩建 Kafka consumer。
  - 状态／条件：READY。
- [ ] **ARCH-014 · P1 · 方案决策 · 异步任务重试与终态恢复协议**
  - 依赖：—。交付与验收：基于 sidecar TaskRunner 定义同 taskId 重复 Start、事件 ACK/持久重放及 Agent 重启后的结果未知语义；不把页面幂等等同游戏副作用去重。
  - 状态／条件：READY。
- [ ] **ARCH-015 · P1 · 实现 · 任务可靠事件与重复启动约束**
  - 依赖：ARCH-014。交付与验收：按已评审协议收口 TaskRunner 重复 Start 和事件重放/终态收敛，复用 task_runs/task_events；验证旧协程清理不删除同 ID 新任务，取消不回退终态；派发前落库失败由 ARCH-026 单独处理
  - 状态／条件：READY；该能力尚未完整实现，并非只缺真实环境验收。

- [ ] **ARCH-016 · P1 · 实现 · Provider drain 真正摘流**
  - 依赖：—。交付与验收：当前 handleDrain 仅记录并回空 ACK；实现停止新路由、等待在途及超时退出语义，验证 ACK 语义与执行结束条件。
  - 状态／条件：READY。
- [ ] **ARCH-017 · P1 · 实现 · Server 停机收尾顺序**
  - 依赖：—。交付与验收：控制 listener.Close 等活跃会话时不阻塞后续 HTTP 停机/根取消；保证 ServiceContext.Close 冲刷留痕，设置有界清理并验证 socket/goroutine/池释放。
  - 状态／条件：READY。
- [ ] **ARCH-018 · P1 · 故障验证 · 执行中断与停机演练**
  - 依赖：ARCH-015, ARCH-016, ARCH-017, ARCH-026。交付与验收：执行中注入断链、drain、SIGTERM；逐项记录新请求拒绝、终态/未知态、截止时间及资源释放；不把测试 fixture 的游戏副作用结果当真实游戏恢复凭据
  - 状态／条件：NOT RUN；本地可搭计数执行器/完整栈，真实游戏另按 GAME-010 验收。

- [ ] **ARCH-019 · P2 · 实现 · 执行领域依赖方向收敛**
  - 依赖：—。交付与验收：复用 assignment gate 下沉领域包，functioninvoke 不再 import internal/api；core/agents 的依赖边界检查可执行且原行为一致。
  - 状态／条件：READY。
- [ ] **ARCH-020 · P2 · 实现 · 架构文档防漂移检查入 CI**
  - 依赖：—。交付与验收：复用 scripts/docs/check-architecture.py、check-architecture.cjs 与生成器，核对 SVG、路径、首页资源和 TODO DAG；新链接失败即报告，避免直接开启全站 strict 阻断旧页面。
  - 状态／条件：READY。

- [ ] **ARCH-021 · P1 · 实现 · 扩展同步身份与 scope 接入**
  - 依赖：—。交付与验收：复用现有 HTTP puller/版本 payload 与 protected 路由，为 Agent 建立受限机器身份及 scope 授权；无凭据/跨 scope 拒绝，合法 Agent 实际拉取生效且坏版本保持 last-good。证据：internal/app/agent/extension_sync_puller.go 未设认证头，internal/handler/routes.go 受 protected 保护，AgentSyncPayload 检查读取权限；不通过取消服务端授权绕过。
  - 状态／条件：READY；真实接入验收 NOT RUN，需受控机器凭据与完整栈。

- [ ] **ARCH-022 · P2 · 实现 · 注册与重连的显式请求预算**
  - 依赖：—。交付与验收：复用 core/register，为 DialAndRegister/SyncOnce/SyncAttempts 明确一致的注册超时边界及调用者 deadline 优先级；核对心跳 tick 重拨与首拨退避差异，验证无注册响应时按预算退出、Stop 可释放连接与 goroutine，不引入重复并行重连。
  - 状态／条件：READY；无响应服务端故障验证 NOT RUN。证据：core/register/registration.go 的 SyncAttempts 才包 requestTimeout，loops.go 的 DialAndRegister 直接调用 registerOnce。

- [ ] **ARCH-023 · P0 · 实现 · 定时触发先领取再派发**
  - 依赖：—。交付与验收：复用 task_schedule_run_logs 唯一键，派发前原子领取 schedule+slot 并明确领取租约/崩溃恢复；多实例同槽仅一派发，记录失败不得执行；覆盖领取后/派发前和派发后/回写前中断，业务最终副作用去重仍由 GAME-010 承担
  - 状态／条件：VERIFIED 缺陷：manager.go 先 HasRunLog，再 StartTask，最后 CreateRunLog 且忽略冲突；双 Manager 诊断实际派发 2 次、仅 1 条唯一记录。

- [ ] **ARCH-024 · P0 · 实现 · 任务及调度入口统一执行治理**
  - 依赖：—。交付与验收：让 /tasks Start、schedule Create/TriggerNow/自动触发复用可注入的执行治理服务；不把模块级 JWT/scope 误作函数政策。对同一高危函数，普通/页面/任务/手动调度/定时入口一致执行禁用/分配/函数权限/政策审批/输入契约；自动化主体和批准快照显式定义，撤权后不继续裸派发
  - 状态／条件：VERIFIED 装配缺口：task.Service.Start 只做部分 gate；schedule 和 svc.dispatcherAdapter 直连 Dispatcher。不可据统一 transport 宣称统一治理。

- [ ] **ARCH-025 · P0 · 实现 · 策略查询和初始化故障禁止静默放行**
  - 依赖：—。交付与验收：区分无覆盖策略与策略存储/配置故障；高风险执行无法获得有效政策时拒绝或使用明确批准的 last-good，不返回 nil,nil 绕过审批/审计；覆盖 GetPolicy SQL 错误及 PolicyManager 初始化失败
  - 状态／条件：VERIFIED 诊断：缺失 function_policies 表时 GetPolicy 返回错误，而 EnforceFunctionPolicy 返回 policy=nil/error=nil；初始化失败分支同样缺有效政策。高危失败策略须先明确再落地。

- [ ] **ARCH-026 · P1 · 实现 · 任务主记录持久化失败时禁止派发**
  - 依赖：—。交付与验收：Dispatcher 在 CreateRunWithMeta 失败时返回可定位错误并不得发送 StartTask；验证 DB 失败时执行器计数为 0、无孤立事件/不可取消任务；不与 Agent 事件重放重复实现
  - 状态／条件：VERIFIED 代码：dispatcher.go:startTaskOnPickedAgent 忽略 CreateRunWithMeta 错误仍派发；现有文字“先写任务”不等于写成功才执行。

- [ ] **ARCH-027 · P1 · 实现 · 输入契约读取失败的明确拒绝语义**
  - 依赖：—。交付与验收：区分旧函数确无 schema 与 descriptor 查询失败；契约读取故障不得作为跳过验证依据；固定已批准快照/版本，覆盖 schema DB 失败、required 缺失和版本漂移；复用 ValidateInvokeInput
  - 状态／条件：VERIFIED 代码：functioninvoke.ValidateInvokeInput 在 ListDescriptors 出错时返回 nil，治理状态需先统一故障语义。

## 页面链验收

- [ ] **UI-001 · P1 · 故障验证 · 真实游戏页面发布全链（旧 C2）**
  - 依赖：GAME-002, GAME-010, ARCH-024。交付与验收：独立真实游戏环境走契约→模板→编辑保存→接受提案→发布→binding 执行；验证列表/详情/弹窗/刷新、筛选/契约变化/人工草稿隔离，记录落库快照与游戏调用凭据
  - 状态／条件：NOT RUN；需要 GAME-002 接入的权威玩家函数；demo 验收不能代替此项。

- [ ] **UI-002 · P1 · 故障验证 · 示例异常状态全链（旧 E2）**
  - 依赖：—。交付与验收：复用现有 Go gameops-demo 的 33 函数及内存状态，独立 Server/sidecar/demo 栈验证发布、审批、report.generate、失败/未知/取消/幂等重试；既有 fixture 只证机制，不证明进程重启后游戏侧持久去重
  - 状态／条件：NOT RUN；可用仓库样例在本地搭栈，不以缺真实游戏账号阻塞此项。

- [ ] **UI-003 · P1 · 故障验证 · 批量确认与默认呈现实页验收（旧 D2/D5）**
  - 依赖：UI-002。交付与验收：复用 perTarget、字段语义和五模板；跨页选人、筛选变化、总量确认与实际提交一致，发布回读保留身份列/单位/时区/枚举；不重复规划已实现 UI。
  - 状态／条件：NOT RUN：需发布态 demo 栈。

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

验收记录要求：逐项写触发输入、期望、实际、代码版本、环境、证据位置及未覆盖边界；总测试数和文件存在不是验收。完成后保留稳定编号，不复用或重排编号。尚未立项的产品/平台专题只登记调查，不自动扩建服务或替换 SDK。

## 共用核心分析与外部专业分析

产品边界（用户确认方向，2026-10-29）：内置共用核心运营指标；专业自助分析/复杂归因/预测通过外部集成。现有分析代码与页面继续复用，不把“管道存在”当作指标已验收。

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

## 并行审核复验（保留在途修复）

- [x] **REVIEW-001 · P1 · 故障验证 · 批量充值目标 ID 归一化复验**
  - 依赖：—。交付与验收：批量充值 demo 把选中行对象整体 `map(String)` 成 `[object Object]`，导致实际渲染路径无法匹配玩家 ID 却返回成功（web/src/components/PageRenderer/templates/playerManage.tsx:442）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收以实际批量执行入参为玩家 ID、回读影响人数与目标名单一致且无 [object Object] 为准。
  - 状态／条件：LOCAL VERIFIED：gameOpsPublishFixture.test.tsx 已断言对象 selection（ResourcePageRenderer 实际提交形态 `{form, selection}`）的两个目标各精确增加 100、未选目标不变且仅两项变化；裸 ID 直调兼容用例通过。执行器入参契约与 selector 投影（`selection → /id`）已核对，不以 count/total 字段代替副作用回读。
  - 完成证据：2026-10-29 定向运行通过；`web/src/components/PageRenderer/templates/gameOpsPublishFixture.test.tsx`，验收断言与边界见上述状态。

- [x] **REVIEW-002 · P1 · 故障验证 · 空白批量金额拒绝复验**
  - 依赖：—。交付与验收：`summarizePerTargetTotal` 把空串/全空白串转换成 0，违背“不确定输入不猜 0、不执行”注释并会确认并提交 0 充值（web/src/components/PageRenderer/batchSelection.ts:73）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收以空串/全空白均禁止确认与提交、合法有限数字仍可按显式语义执行为准。
  - 状态／条件：LOCAL VERIFIED：ResourcePageRenderer.test.tsx 的 REVIEW-002 用例验证空串/全空白提交不调用执行器，合法 10 经确认后按原 form 与 selection 提交；resourceBatchSelection.test.tsx 覆盖空白 guard。已完成本项本地边界验收；真实游戏接入另见既有任务。

  - 完成证据：2026-10-29 定向运行通过；`web/src/components/PageRenderer/__tests__/ResourcePageRenderer.test.tsx`，验收断言与边界见上述状态。

- [x] **REVIEW-003 · P1 · 故障验证 · 错误响应体优先级一致性复验**
  - 依赖：—。交付与验收：`extractApiErrorCode` 声称与 `utils/errors` 同口径但优先 `.data` 再 `.response.data`，后者相反优先 `.response.data`，二者可能取到不同响应体（web/src/utils/apiError.ts:7）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收覆盖 data 与 response.data 并存且内容冲突时，extractApiErrorCode 与 utils/errors.extractErrorCode 对有效冲突响应采用同一响应体与稳定码。
  - 状态／条件：LOCAL VERIFIED：apiError.test.ts 冲突样本断言 response.data.error 优先；源码核对 apiError.ts 与 errors.ts 的有效响应体优先级一致。本项限定两字段均为有效响应对象的冲突情况，不宣称所有解析入口/畸形响应完全统一。

  - 完成证据：2026-10-29 定向运行通过；`web/src/utils/__tests__/apiError.test.ts`，验收断言与边界见上述状态。

- [x] **REVIEW-004 · P1 · 故障验证 · 批量表单确认文案可达性复验**
  - 依赖：—。交付与验收：带表单批量动作进入表单后绕过原有 `confirm/confirmTitle/confirmDescription` 路径，批量充值模板声明的目标确认文案实际不可达（web/src/components/PageRenderer/ResourcePageRenderer.tsx:1246）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收覆盖带表单动作的确认标题/说明/目标可见且与实际提交一致，取消确认不会调用执行器。
  - 状态／条件：LOCAL VERIFIED：ResourcePageRenderer.test.tsx 的 REVIEW-004 用例已限定确认框作用域，验证 confirm=true 带表单动作的标题/说明/目标/总量可见；取消不调用执行器；正向用例确认后提交完整 selection 与表单值一致。
  - 完成证据：2026-10-29 定向运行通过；`web/src/components/PageRenderer/__tests__/ResourcePageRenderer.test.tsx`，验收断言与边界见上述状态。

- [ ] **REVIEW-005 · P1 · 实现 · 批量金额安全整数与乘积边界**
  - 依赖：—。交付与验收：为 summarizePerTargetTotal 与业务金额契约明确单位/倍率/最大值，拒绝非安全整数及乘积溢出；覆盖 MAX_SAFE_INTEGER 邻界、极大目标数与 Infinity，不显示可确认的失真总量
  - 状态／条件：VERIFIED 代码只检查 Number.isInteger/Finite 输入，未检查安全整数或最终乘积；原空白输入修复不覆盖此问题。

## 依赖安全证据复核

- [ ] **SEC-001 · P1 · 资料调查 · 默认分支依赖告警适用性复核**
  - 依赖：—。交付与验收：获取 GitHub 默认分支未关闭告警的官方明细，映射 go.mod/go.sum、package.json/锁文件与 SDK/构建产物；区分运行时/开发/传递依赖、受影响版本与实际可达条件，逐项形成“需修复/不适用及依据/待验证”清单，再拆最小修复任务，不仅凭告警总数宣布存在可利用生产漏洞。
  - 状态／条件：NOT RUN；2026-10-29 推送 56c85dbb 的 GitHub 回执报告 19 条（1 high、10 moderate、8 low），仅回执已观察，告警详情/适用性尚未核实；读取详情可能需要仓库安全告警权限。
- [x] 审核发现：`GET /api/v1/tasks` 的 `pageSize` 回显原始请求 `size`，未反映服务端 `model.NewPagination` 钳制后的实际值（请求 `?size=200` 服务端实际返回 100 条但回显 200；`size=0` 实际返回 20 条但回显 0），响应契约与真实分页不一致（internal/api/task/handler.go:29、internal/model/vars.go:70、internal/api/task/task_center_read_test.go:42 未覆盖钳制回显）。证据：`ListResponse` 增补 `Page`/`Size` 字段并回显 `model.NewPagination` 钳制后的值（service.go 复用一次构造、handler.go 传 `resp.Page`/`resp.Size`），新增 `TestTaskCenterListEchoesClampedPagination` 覆盖 over-max→100、zero→20、zero-page→1 三档；`go test ./internal/api/task/` 全绿（含 372 用例套件）、`go build ./...` + `go vet ./...` 绿。
- [x] 审核发现：页面 binding 执行（`executePageBinding`）未设 `skipErrorHandler`，403 `permission_denied` 会先被全局 `errorHandler` `msgWarn` 并 `history.push('/403')` 且不重新抛出，`describeExecFailure` 的权限原因行在真实页面不可见（web/src/requestErrorConfig.ts:207、web/src/services/console.ts:123）。证据：`executePageBinding` 与同口径的 `executeDraftCompositeBinding` 均加 `skipErrorHandler: true`，令原始错误穿透全局 `errorHandler` 重新抛出，由 `describeExecFailure` 在真实页面渲染权限原因；新增两则接线测试断言 `config.skipErrorHandler === true` 且原始错误原样 `rejects`；`jest src/services/` 603 用例全绿、`tsc --noEmit` 0 错误、`bash scripts/dashboard_vnext_guard.sh` PASSED。
- [x] 审核发现：CDC schema `History.Clone()` 无锁保护，而 `Record` 在 decoder `Feed` 中并发写入同一 `tables` map/slice；文档声称 `Clone()` 可安全交给并发消费者，但 `Clone` 遍历 map 与 `Record` 写 map 存在竞态（pkg/cdc/schema/schema.go:189、agents/capture/connector/pg/connector.go:817、agents/capture/connector/pg/decoder.go:79）。证据：`History` 增加 `sync.RWMutex`，`Record` 取写锁、`At`/`Versions`/`Tables`/`Validate`/`Clone` 取读锁，彻底消除读（Clone 遍历 map 交付并发消费者）与写（Record append map/slice）竞态；新增 `TestHistoryConcurrentRecordAndRead`（4 写 × 200 表并发 Record，同时 4 读者并发 Clone/At/Versions/Tables/Validate）；`go test ./pkg/cdc/... ./agents/capture/connector/pg/... -race` 全绿、`go build ./...` + `go vet ./pkg/cdc/...` 绿。
- [x] 审核发现：英文 `pagespec-protocol.md` 仍声明「there is no tasks route so a pending task shows the taskId without a navigation target」，与中文侧（已写 `/ops/jobs` 任务中心入口）及代码（CompositeRenderer/OperationPageRenderer/TaskPageRenderer 均 `history.push('/ops/jobs')`）不一致（docs/architecture/pagespec-protocol.md:226、docs/zh/architecture/pagespec-protocol.md:318）。证据：英文 D4 执行结果反馈改为「task center entry (`/ops/jobs`), following the task and execution-log chain」并把边界改写为「task center route is `/ops/jobs`」；中文侧同步消除自相矛盾的「无任务中心路由」边界（任务中心路由 `/ops/jobs`，渲染器跳转该入口），与 `web/config/routes.ts:516` 的 `/ops/jobs` 路由及三个渲染器的 `history.push('/ops/jobs')` 对齐；`pnpm --dir docs build` 通过、`bash scripts/check-docs-links.sh` 605 链接全解析。
- [x] 审核发现：`x-field-per-target` 批量总量仅有纯函数/单测覆盖，五个 gameops demo 模板及 demo 契约均未声明 `perTarget` 字段，D2 跨目标总量在实际页面不可达（web/src/components/PageRenderer/templates/_.tsx、sdks/go/examples/gameops-demo/_.go 无 per-target 声明；web/src/components/PageRenderer/**tests**/compositeBatchTotal.test.tsx 仅覆盖手工构造的 COIN_FORM）。属实：修复为玩家管理模板 `batchActions` 新增 `action.batchRecharge`（声明 `perTarget` 字段 amount）并接入 `ResourcePageRenderer` 的 D2 跨目标总量换算（`summarizePerTargetTotal`，批量表单提交时按已选目标数核算总量并二次确认）；Go demo 契约同步声明 `player.batchRecharge`（`playerIds`+`amount`）。证据：commit（见 git log）；`go test ./examples/gameops-demo/...` 绿、`jest` playerManage/templates/capabilities/ResourcePageRenderer/compositeBatchTotal 全绿、`tsc --noEmit` 0 错误。
- [x] 审核发现：`handleSessionOrScopeError` 的 401 分支丢失非 skip 路径的 `payload` 结构化门控，任何响应体非空的非 2xx 错误都会命中 `errorCode === 'unauthorized'` 分支清 token 并跳登录，覆盖非认证语义错误（web/src/requestErrorConfig.ts:217）。属实（防御一致性缺口）：`handleSessionOrScopeError` 的 `errorCode === 'unauthorized'` 分支未限定 `status === 401`，与主 `errorHandler` 的 payload 结构化门控不一致；修复为仅 `status === 401 && errorCode === 'unauthorized'` 时清 token 跳登录（后端 `unauthorized` 稳定码仅出自 401，当前不误触发，但补齐门控防止非 401 语义码劫持）。证据：commit（见 git log）；`requestErrorConfig.test.ts` 新增「非 401 但 body.error=unauthorized 不命中」用例，套件绿、`tsc --noEmit` 0 错误。
- [x] 审核发现：登录接口 `mfa_required` 401 带 `skipErrorHandler` 本应由登录页自管错误展示，现在会先被全局清 token 并跳 `/user/login`，契约与 `services/api/auth.ts` 注释冲突（web/src/services/api/auth.ts:82、web/src/requestErrorConfig.ts:199）。属实：`handleSessionOrScopeError` 的 401 分支把 `auth/login`（非 profile/messages 白名单）当作 token 失效清 token 跳登录，劫持登录页自管错误展示（含 `mfa_required` 二次验证码分支）；修复为 `handleSessionOrScopeError` 显式跳过认证入口接口（`/auth/login`、`/auth/register`、`/auth/verify-email`、`/auth/resend-verification`），完整交给调用方 catch。证据：commit（见 git log）；`requestErrorConfig.test.ts` 新增「登录接口 401+mfa_required 带 skipErrorHandler 不自管劫持」用例，`Login/login`+`services/api/auth` 套件绿、`tsc --noEmit` 0 错误。
- [x] 审核发现：新增页内无效 scope 原因依赖 `extractApiErrorCode` 读取 `err.data`，但真实 axios 1.x `ResponseError` 响应体在 `err.response.data`，生产 scope 403 的原因行不可达；新增测试构造 `err.data` 掩盖该契约不一致（web/src/utils/apiError.ts:12、web/src/components/PageRenderer/execFeedback.ts:84、web/src/components/PageRenderer/**tests**/compositeExecFeedback.test.tsx:37）。属实：`extractApiErrorCode` 只读 `err.data`，而 `execFeedback.errorBody` 与 `utils/errors` 均回退 `.response.data`，若错误形态为 `.response.data` 则稳定码取不到、无效 scope 原因行不可达；修复为 `extractApiErrorCode` 同口径回退 `.response.data`（`data ?? response?.data`）。证据：commit（见 git log）；`apiError.test.ts` 新增 `response.data` 与 `data` 优先两用例、`compositeExecFeedback.test.tsx` 新增 `response.data` 无效 scope 用例，套件绿、`tsc --noEmit` 0 错误。
- [x] 审核发现：`banManagement` 将 `serverId` 标记为可查询但 demo 列表未消费 `form.serverId`，筛选控件无效（web/src/components/PageRenderer/templates/banManagement.tsx:72）。属实并已修：demo 执行器 list 分支补 `form.serverId` 精确过滤（banManagement.tsx:365），`banManagement.test.tsx` 新增 s2 筛选用例（total=8 且逐行 serverId=s2），证据 commit（见 git log）；`pnpm --dir web test --testPathPatterns PageRenderer` 46 套 563 用例全绿、`pnpm --dir web run tsc` 0 错误。
- 不属实：组合页行级危险 targetSection 动作未传 targetCount（CompositeRenderer.tsx:589）——行级动作只作用于单行，per-target 总量条目仅在 targetCount>1 时追加（confirmContent.ts:87），缺省 1 与工具栏单行选择行为一致，无用户可见差异；行级传 targetCount 反误报批量总量，故维持不传。
- [x] 审核发现：新增操作页任务中心链接后，中文协议仍写“无任务中心路由/不给导航目标”，实现与文档冲突（web/src/components/PageRenderer/OperationPageRenderer.tsx:679）。属实并已修：同步中英四份协议文档（docs/zh/architecture/pagespec-protocol.md、docs/zh/dashboard/composite-editor-v3.md、docs/architecture/pagespec-protocol.md、docs/dashboard/composite-editor-v3.md）把「无任务中心路由/不给导航目标」改为「前往任务中心入口（/ops/jobs）+ 查询执行状态」；证据 commit（见 git log）；`bash scripts/check-docs-links.sh` 605 链接全解析、prettier 净。
- [x] 审核发现：`pkg/cdc/schema.History` 无内部同步，PG 连接器 pump 循环在 `Feed→cacheRelation→schemaHist.Record` 中并发写表映射（pkg/cdc/schema/schema.go:91），而恢复侧 `SchemaAt`/`CurrentSchemaVersions→schemaHistorySnapshot→History.Clone` 无锁读同一映射（agents/capture/connector/pg/connector.go:817-823、pkg/cdc/schema/schema.go:189）；`schemaMu` 只保护指针交换（connector.go:687-689）不保护历史内容，捕获流与恢复校验并发时存在数据竞争（Go 运行时可致命报 "concurrent map read and map write"）。`Clone` 注释「safe to hand to a concurrent consumer that may read while the connector keeps recording」（schema.go:187-188）与实现不符——Clone 本身在并发 Record 下不安全；另外 Clone 对 `Version.Schema.Columns` 只是浅拷贝（append 复制 Version 切片，未深拷 Column 切片），与「deep copy」注释不符。 属实并已修：`History` 增加 `sync.RWMutex` 保护全部表映射读写（pkg/cdc/schema/schema.go:80，Record 93 / At 137 / Versions 162 / Tables 169 / Validate 184），`Clone` 深拷贝每版本 `Schema.Columns`（schema.go:200-212），与「safe to hand to a concurrent consumer」注释一致；证据 commit（见 git log）；`go test -race ./pkg/cdc/schema ./agents/capture/connector/pg` 全绿（含新增 `TestHistoryConcurrentRecordAndRead` 并发读、`TestHistoryCloneDeepCopiesColumns` 深拷贝）、`go vet` 绿。
- [x] 审核发现：`decoder.go` 的 `curBeginLSN` 仅在 Begin 消息赋值（agents/capture/connector/pg/decoder.go:112）、Commit 后不重置；`cacheRelation` 对事务外且 WALStart=0/空 的 Relation 消息回退到「上一个事务」的 Begin 位点（decoder.go:331），与该处「事务外且位点仍未知：跳过（不猜）」注释不符——会把新 schema 版本记到早于实际变更的位点，若该表后续版本已记录则触发 Record「not after」致命错误断流，或使恢复时点校验在陈旧区间误判列集（fail-open 方向）。 属实并已修：Commit 分支清空事务 Begin 位点（agents/capture/connector/pg/decoder.go:129），事务外且位点未知的 Relation 不再回退到上一个事务陈旧位点、走 cacheRelation「跳过（不猜）」；证据 commit（见 git log）；`go test -race ./agents/capture/connector/pg` 全绿（含新增 `TestDecoderBeginLSNResetAfterCommit`/`TestDecoderRelationWithOwnLSNOutsideTxn`）、`go vet` 绿。
- [x] 审核发现：`gameOpsTemplateCapabilities.test.ts`「定位危险操作」断言是恒真式（web/src/components/PageRenderer/templates/gameOpsTemplateCapabilities.test.ts:158-160）——`if (a.type === 'danger')` 内再断言 `allActions.some(x => x.type === 'danger')`，循环体进入时必然为真、无法失败；且 `allActions` 仅由列表动作区拼接，检验不了测试命名声称的「不藏在详情/表单深处」，属无效测试断言。 属实并已修：改为断言深处（detailView.actions / res.actions）不得含 danger、列表区每个 danger 确为 danger（web/src/components/PageRenderer/templates/gameOpsTemplateCapabilities.test.ts:152-167），不再是恒真式；证据 commit（见 git log）；`pnpm --dir web test --testPathPatterns gameOpsTemplateCapabilities` 1 套 19 用例全绿、`pnpm --dir web run tsc` 0 错误。
