---
title: 函数链路卡点清单
icon: warning
order: 6
category:
  - 系统架构
tag:
  - 函数注册
  - UI 生成
  - 执行
  - 已知问题
---

# 函数链路卡点清单（注册 → UI 生成 → 执行）

> **性质**：2026-09-22 全链路盘点快照。逐条给出现象、证据（file:line 为盘点时点）、影响面与处置状态。
> **处置标记**：✅ 已修（附修复说明）｜🚧 本次修复中｜📋 计划内暂缓（有明确方案，排期）｜⚖️ 需设计决策（方案影响协议/产品语义，先决策后动工）。

## 0. 结构性结论

1. **分库模式（`multiGame: true`）是被忽视的二等公民**：stale-heal、组件模板、清扫预算等多个子系统只在单库模式验证过，切分库即失效（U1 已修，U2 待决策）。线上单库暂时不触发，属最高优先级结构债。
2. **执行链存在「agent 静默死亡」劣化链**：E1（不 failover）+ E6（无熔断）+ E3（无看门狗）+ E7（cancel 吞错）组合，把一次网络分区放大为「同步调用 15s 卡死 → 无熔断摘除 → 异步任务幽灵 running → 取消永久挂起」。
3. **协议级能力三处「字段存在但断链」**：幂等键（E5）、取消传播（E4）、权限规则级 scope（已核实为死字段，见 §3 E0）——共同教训是新增协议字段时必须同 PR 打通「API 收 → 存 → 用」全程，否则即成静默死字段。
4. 注册面瞬态摘除的宽限机制（`function_contracts.removal_pending_at`，迁移 0031）本体设计闭环；本次盘点在其上发现两处自身缺陷（R3/R4）并已一并修复。

## 1. 注册链路（SDK → agent → ControlService → registry → 契约/默认页）

### R1 ⚖️ 同名函数跨注册方 latest-wins，无冲突检测（高）

两个 provider/agent 在同 scope 注册同名函数时，每次注册全量重建本会话全部函数契约，`(game_id, env, function_id)` 唯一索引上最后写者胜；schema 只与「同 agent 上次注册」比对（`PreviousFunctionSchema` 限定同 agent），跨注册方 schema 漂移无告警、无门槛。多语言/灰度共存时契约来回翻转 → 已发布页 digest 失配 → 全页 `input_schema_stale`（2026-09-20 事故根因；5a0f60869 只对 demo 契约加了 parity guard，服务端机制未加跨注册方防护）。

证据：`internal/platform/registry/store.go:525-530`（全量重建）、`internal/model/function_contract.go:17-19`（唯一索引）、`internal/server/control_handler.go:622`、`agentlocal/store.go:165-185`（schema 明确「最后注册者胜」）。

**处置（需设计决策）**：候选方案 a) 跨注册方 schema 不兼容时拒新保旧 + 注册告警；b) 冲突标 stale 并进告警中心人工裁决；c) provider 维度契约隔离（同一 function_id 多版本行）。需要先定「同名函数多 provider」是不是受支持场景。

### R2 ⚖️ HA 下 survivingFunctionMeta 是实例本地内存视图（中）

实例 1 上 agent 摘除函数 f，而 f 由实例 2 的 agent B 提供 → 实例 1 内存无 B 会话 → 误标 pending → 10 分钟后清扫真删仍在服务的契约。缓解：cluster 30s 回灌远程会话触发 `ClearRemovalPending`（`cmd/server/cluster.go:392-445`），但 `ListAliveOwners` 失败直接放弃回灌（`:393-397`）。归属表持续故障 + 跨实例重叠时误删（该 agent 重注册后自愈）。

证据：`store.go:531-546`、`:803-825`（遍历本实例 `s.agents`）。

**处置（需设计决策）**：回灌失败时跳过本轮打标（fail-open 保守化），或 pending 判定改读共享归属表而非本地内存。单活部署下不触发。

### R3 ✅ finalize 幸存资源的页面提案不重生成（中）

`FinalizeExpiredContractRemovals` 对「资源仍有其他存活契约」的情况只重建能力聚合、不重建资源页提案 → `resource:<key>` 提案 PageSpec 内残留已删函数的 binding/section，直到该资源下次注册或手动 rebuild。与即时移除路径「同净效果」的断言对提案内容不成立（旧路径经 `plan.Resources → RebuildProposalsForResource` 会重生成）。运营据此发布会产出 `binding_function_missing` 的必败绑定。

**修复**：finalize 命中且资源仍有存活契约时，`RebuildResourceCapability` 之后追加 `RebuildProposalsForResource`；整轮 finalize 收口处追加一次 `RegenerateContractTemplates`。修复后提案与模板与即时移除路径同净效果。

### R4 ✅ 摘除打标/清标触碰 updated_at，制造 digest 幽灵 churn（中）

`MarkRemovalPending` / `ClearRemovalPending` 用 gorm `Update`（自动触碰 `updated_at`），而 `computeDigest` 是整行序列化（`UpdatedAt` 无 `json:"-"`）→ 宽限内首轮 `RebuildProposalsForResource` 判定提案「已变化」：accepted 提案被打回 pending、多出 proposal/semantics 版本快照。与该改造「零版本噪音」的目标直接相抵（模型注释 `function_contract_model.go:47-53` 明确把 updated_at 幽灵扰动列为跳写机制的动机，加 `json:"-"` 时漏了 gorm 自动触碰）。

**修复**：两处改 `UpdateColumn(s)`（绕过 gorm 自动触碰，不动 `updated_at`）；补「打标后 digest 不变」回归用例。

### R5 ⚖️ 永久消失（agent 彻底下线）无契约清扫路径（中）

pending 只由「同 agent 重注册的 diff」触发；agent 崩溃不再注册时，会话清理链路（内存 ExpireAt 清理、DB 会话 DeleteExpired、断连 RemoveAgentIfStale、pruneOrphanSnapshots）全部不碰契约。死函数的 bound 契约+提案+页面无限期留存（实例 0、调用恒失败），也无人工删除入口（`FunctionDelete` 只删 legacy functions 表）。

**处置（需设计决策）**：候选 a) 会话过期时对其独占函数打 pending（复用宽限）；b) 函数目录增加人工摘除入口（走 `RemoveFunctionContract` 即时路径）。

### R6 ✅ finalize 认领后衍生动作失败被吞且无重试（中低）

真删后的 removed 历史 / 提案清理 / 资源重建失败仅 slog.Warn，行已认领、下轮不再处理、运维不可见（对比注册链有 registrationWarnings 进 UI）。

**修复**：衍生动作失败聚合返回（`errors.Join`，不中断整轮、其余候选继续清扫），调用方可感知残缺状态；残缺状态可由 `RebuildAllProposals` 手动兜底（端点已有）。registrationWarnings 同款告警面写入未做，见 E7 随看门狗统一治理。

### R7 ✅ 清扫循环工程性短板（低）

候选查询无 LIMIT、行级错误提前 return 顺延剩余候选、分库逐 scope 串行。

**修复**：`ListExpiredPendingRemovals` 加 limit 参数（最旧优先 LIMIT 截断，清扫单轮批量 `removalSweepBatchSize=200`，剩余候选下一轮继续；索引 `idx_function_contracts_removal_pending` 支持过滤+排序）；认领失败（`DeleteIfRemovalPending` 报错）不再提前收整轮，聚合进返回错误与衍生动作失败同口径（R6）。边界：分库逐 scope 串行扫描保留——当前量级单轮 30s 预算内完成，量大后再议并行/预算切片。

### R8 📋 重启/恢复全量物化放大（低）

实例重启后 `LoadFromDBFiltered` 恢复快照、首次注册 diff 全 Added → 全量契约/提案/模板重建 SELECT/UPDATE 往返；skip-write/digest 门控兜底了内容正确性，仅剩写放大与恢复行。历史 sqlite 事务死锁教训提示该路径对写锁敏感，量大后需增量对账。

### R9 📋 心跳自愈 reseed 用空 Functions 会话覆盖 DB 快照（低）

`control_handler.go` 心跳 reseed `Functions: map{}` 经 `writeToDB` 把 `agent_sessions.functions` 写成空对象 → 后续 diff 基线失真（全 Added，多数场景无害）；跨实例「只升不降」回灌无法从空行恢复。边角低危，随 R8 一并处理。

## 2. UI 生成链路（契约 → 提案 → 草稿/发布 → 渲染）

### U1 ✅ 分库模式下 stale-heal 自动愈合整体失效（高）

heal 循环用 meta 库 `metaDB.Table("page_specs")` 枚举 scope，而 database-per-game 模式下 page_specs / published_page_specs 都在 game 库（`PageSpec` 在 GameModels），meta 迁移集不建该表 → 查询每轮报错返回，愈合永不执行。分库部署下契约漂移后已发布页面无人自动愈合、执行全阻断，只能全人工。

**修复**：scope 枚举按部署形态分流（`healScopes`）——单库模式（Router 未装配）行为不变，仍从 page_specs 取 distinct scope；分库模式改从 `game_envs` 绑定表枚举（meta 侧唯一事实源，与迁移 fanout 同源），页级查询经 `scopeDBContext`（scope + dbctx game 库覆盖）落到对应 game 库。回归用例用真实 Router 文件库 e2e 验证「绑定 scope 内已发布页漂移被同步、rename 语义项只同步草稿不发布、二轮幂等」+ 单库路径多 scope 枚举不回归。边界：绑定存在但 game 库尚未懒建的 scope 会空转一轮空查询（有发布页的 scope 必然建过库，无副作用）。

### U2 ⚖️ 分库模式下组件模板库错库（高）

component-templates REST 挂在非 scoped 组（`internal/handler/routes.go:111-117`）且自动重建闭包用 `serverCtx.DB`（meta 连接），但 `component_templates` 表在 GameModels（game 库）→ 分库模式下模板 API 与注册自动重建读写不存在的 meta 表，registrationWarnings 刷屏。

**处置（需设计决策）**：模板库是 per-game 还是全局？若全局，表应迁 meta 库（编号迁移）；若 per-game，REST 与重建闭包需 scoped 化。方向决策后动工。

### U3 📋 composite 页无契约变化再生成通道（中）

`RebuildAllProposals` / `RebuildProposalsForResource` 只覆盖 resource/standalone，`composite--` 提案只有编辑器重新保存才更新；已发布 composite 页契约漂移后 sync-selectors 恒 `manual_required`、heal 放不过 → 唯一出路是编辑器重开重存。

**处置（需设计决策）**：composite 再生成的语义（保人工布局、只替换漂移 binding 的 schema）需要独立设计，与「批量同步只写 draft」边界衔接。

### U4 ⚖️ versioning「重新生成」对 composite 页语义错位（中）

`RegenerateProposal` 对非 resource 页把 composite 页第一个 action/task/report binding 当「主函数」生成单函数 Operation 提案，强改 PageKey 覆盖原 composite 提案 → 一次 regenerateDraft/BulkRepublish 即把组合页替换成单函数页（两步丢内容）。

**处置**：composite 页应拒绝走 standalone 再生成（400），入口置灰。小改动，随 U3 一并修。

### U5 📋 accepted 提案打回 pending 后再 accept 恒 409（中）

草稿已存在的页，其提案随契约变化重置 pending 重进收件箱；用户点接受 → `page draft already exists` Conflict，只能拒绝或走破坏式 regenerate（与 sync-selectors 的保定制路径相悖）。

**处置（需设计决策）**：accept 已有草稿时的语义（合入 selector 变更？提示走 sync-selectors？）需要产品定夺。

### U6 📋 存量无 schema 页面无渲染端兜底（低）

c76f889d1 的单 payload 兜底只在服务端生成器；此前发布的空表单页/旧 composite「确认执行」按钮不随 heal 修复，需手动重发布。随下次全量重建自然收敛。

## 3. 执行链路（web → HTTP → dispatcher → agent → SDK）

### E0 ✅（前置核实）权限规则 gameId/env 是死字段

`GET/PUT /api/v1/functions/:id/permissions` 的 DTO 无 `gameId`/`env`：表单提交值 JSON 绑定时静默丢弃、GET 不回显；线上所有规则恒为全局规则。模型列与 `FunctionActionAllowed` 的规则级 scope 匹配逻辑均已支持，断链只在 REST 两端。死输入框已移除，三层 scope 关系与断链记录见 `game-environment-scope.md` §12.7；补通为功能级变更，暂缓。

### E1 ⚖️ 半开连接上的调用失败不 failover（高）

`errAgentUnreachable` 只覆盖「session 不存在/转发失败」；已建 session 上的超时/连接重置整单返回。agent 无 FIN 死亡（断电/网络分区）时候选存活至 TTL 5 分钟，期间每次调用吃满 15s 超时。结合 HA 熔断未启用（装配硬编码 `NewDispatcherWithHA(..., false, ...)`、`SetHAEnabled` 无调用方）无摘除兜底。

**处置（需设计决策）**：超时类错误是否纳入 failover 候选（涉及重复副作用风险，与 E5 幂等联动）；同时评估启用 HA 熔断器。

### E2 ✅ 「禁用函数」不影响执行（高）

disable 只改 functions 表 `status`（`model.StatusDisabled=0`），执行链路（dispatcher 候选、invoke、/tasks Start）无一处读它 → UI 禁用后函数仍可调用，语义与预期相反。

**修复**：共享守卫 `utils.EnsureFunctionEnabled`（`functionInvoke` 在路由校验后、遥测/审计/执行留痕之前；`/tasks` Start 在契约校验后），禁用返回 409 `function_disabled`（与页面生成器同名诊断同一词汇表）。边界：functions 表无物化行不拦（与禁用写路径同一张真值表）；dispatcher 候选选择仍不看 status（直连 SDK 调用不受守卫约束）。

### E3 ⚖️ 异步任务无看门狗（高）

run 状态只能由 agent 事件推进；agent 掉线后 run 行永远停在 dispatching/running，`timed_out` 无写入方，前端 1.5s 轮询永不终止，调用统计按 status 聚合失真。

**处置（需设计决策）**：看门狗的归属（server 端扫 task_runs 超时 vs agent 会话断连时批量标记 interrupted）与终态语义（`timed_out` vs `interrupted`）。建议随任务模型 v2 一并设计。

### E4 ⚖️ 取消传播只到 agent 传输层，到不了游戏函数（高）

agent 不向 provider 转发取消；SDK 入站 handler 用 `context.Background()` 执行 → 「取消」= 放弃等待而非停止执行，游戏侧副作用（扣款/发邮件）照常发生且无补偿记录。SDK 侧 startTask/cancelTask/streamTask 处理器在平台链路是死代码。

**处置（协议级）**：wire 协议加 provider 取消帧 + SDK handler 贯穿 ctx；六语言 SDK 同步改。成本高，需单独排期。

### E5 ⚖️ 幂等键全链路无落地（高）

proto 字段、`task_runs` 列、StartRequest 字段三处存在，但 API 不收、BuildInvokeRequest 不设、CreateRunWithMeta 不写不查、无去重存储 → 重复点击/网络重试 = 重复执行（异步任务尤其危险）；failover 每轮新 taskID 也源于此。

**处置（协议级）**：与 E1 联动设计——API 层收 `Idempotency-Key`（REST 头或 body）、task_runs 唯一索引、去重窗口语义。是执行链正确性的地基，建议最先排期。

### E6 ⚖️ 审批存储是进程内存 MemStore（中）

PG/SQLite 实现存在但未接线；重启丢全部 pending 审批；审批无 TTL，旧单可无限期后批准，续期执行带 `approvalBypass=approved` 跳过全部 policy。

**处置（需设计决策）**：接线 PG/SQLite store 的时机（涉及 approvals 表迁移与 HA 语义——两实例共享审批池后两人规则的「两人」判定要跨实例）。

### E7 📋 错误静默吞掉集合（中）

policy 查询失败 → nil 放行（fail-open）；审计写失败仅 fmt.Printf；task_runs 建行失败 best-effort（事件无处落且 0 行更新时连事件都丢弃）；REST cancel 的 agent 转发失败被 `_ =` 吞 → cancel_requested 成事实终态。随 E3 看门狗统一治理：fail-open 项改 fail-closed 或显式告警。

### E8 ✅ 前端任务轮询单次错误永久停止（中）

任务事件轮询单次网络错误/401 即永久停止不重试，进度面板假死需重新发起。

**修复**：`subscribeTaskEvents` 连续失败 5 次内重试（间隔仍 1500ms，未做指数退避），成功即复位计数；每次失败仍回调 onError（消费方当日志行渲染，瞬断可见），超限才停止。终局失败与「暂时不可达」在上层日志里按次数可分辨。

### E9 ✅ 版本门槛拦截无执行侧可解释性（中）

门槛拒掉的函数不进会话函数集，调用报 503 `no_live_agent`，与「agent 真没活」不可区分；门槛调高后已物化函数继续可调至下次注册。

**修复**：dispatcher 收口「调度无候选」时（`noLiveAgentError` 升级为方法，同步/广播三处调用点共用），对「存在 `function_version_below_minimum` 注册警告 **且** 门槛仍配置」的函数返回专属错误码 `function_version_below_minimum`（503，HTTP 契约 `error` 字段即该码），消息带被拦版本与两条出路（升级重注册 / 调门槛）。双条件防误述：警告是内存面、删门槛不自动清警告，门槛已删时回落 `no_live_agent` 原语义。配套：拦截警告补 `FunctionID`/`Version` 字段（此前为空，按函数过滤失效）；达标版本注册即清该函数历史拦截警告（生命周期跟随注册行为，对齐 provider_scope_mismatch 修复闭环），保证「警告存在 ⟹ 最近注册仍被拦」。边界：门槛调高后已物化函数继续可调至下次注册的原语义未变（防契约回退靠注册面拦截）；HA 下警告是实例内存面，跨实例归属转发场景由对端实例按自己的警告面归因。

### E10 📋 其余工程性短板（中低/低）

同步热路径每次 2 次 DB 查询无缓存（契约查询抖动时超时预算静默回落 15s）；agent 侧 30s 心跳窗内无健康实例时降级路由 `arr[0]`；task_events seq 读最大+1 非原子且无唯一索引；HTTP 层无 per-request 超时中间件（WriteTimeout 10min）。随对应模块迭代消化。

## 4. 优先级建议

1. **本次已修**：R3、R4、R6、R7、E0、E2、E8、E9、U1 + analytics 占位实现接真实聚合（见 §5）。
2. **需决策后动工**：R1、R2、R5、U2、U3、U4、U5、E1、E3、E4、E5、E6（每条已给候选方案）。
3. **随量增长消化**：R8、R9、U6、E7、E10。

## 5. 修复记录

- 2026-09-22（第二批）：U1（分库 stale-heal 按部署形态分流：单库直查 page_specs 不变，分库从 game_envs 绑定枚举 scope、页级查询经 Router 落 game 库，含真实 Router 文件库 e2e 与单库不回归用例）；E9（dispatcher 对门槛拦截函数返回专属错误码 `function_version_below_minimum`，警告补 FunctionID/Version、达标注册清陈旧警告）；R7（清扫候选最旧优先 LIMIT 批量 200/轮 + 认领失败不中断整轮聚合返回）。

- 2026-09-22（第一批）：E0（roles 下拉接 `listRoles`、死字段移除、§12.7 文档）；R3（finalize 追加资源提案重生成 + 模板收口）；R4（摘除打标/清标改 UpdateColumn 消除 digest churn，含 updated_at/digest 不变回归用例，0031 迁移补列 + 清扫候选索引 `idx_function_contracts_removal_pending`）；R6（finalize 衍生失败 errors.Join 聚合返回）；E2（`EnsureFunctionEnabled` 双入口禁用拦截，409 function_disabled）；E8（任务事件轮询连续 5 次失败内重试，间隔不变）；函数 analytics 从「配置版本数 + 硬编码 100%」改为 `execution_logs` 真实聚合（总调用/成功率/平均延迟/今日·周·月，零数据时成功率显示「—」不伪造）；console/registry 摘除测试与 api 层 analytics 旧口径测试同步适配宽限与新统计语义。
