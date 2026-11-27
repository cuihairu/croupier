---
title: 高风险操作协议评审
---

# 高风险操作协议评审

日期：2026-11-22。状态：**方案待负责人拍板**。本页是 GAME-009 方案决策交付物，
评审[高风险业务操作执行契约设计](high-risk-operation-contract-design.md)（仍处
`待设计评审`），把其 §10「待评审决策」落成带备选对比的具体方案，供负责人拍板。
**本页不改任何 wire、模型或任务/审批行为**——那是 GAME-010 实现项，以本方案
批准为前提。

## 1. 风险面（现状缺什么）

平台目前没有业务级执行身份用于高风险写操作。代码实证：调用链
（`internal/platform/functioninvoke`）与任务模型（`internal/tasks`）带
`IdempotencyKey`/`TaskID`，页面执行有服务端幂等（`page_execute_idempotencies`，
迁移 0048）；`internal/` 中**不存在**任何 `Credential`/`Receipt`/`operationId`
持久化（已核：无 `BusinessOperation`/`OperationCredential` 模型）。具体失效模式：

| #   | 失效模式                         | 现状行为                                                               | 后果                                                        |
| --- | -------------------------------- | ---------------------------------------------------------------------- | ----------------------------------------------------------- |
| R1  | **响应丢失重复发放**             | 函数调用超时；平台无游戏侧持久去重契约，重试再次调用游戏函数           | 游戏发放两次（金币/物品/邮件/货币）。                       |
| R2  | **未知被误判为失败**             | `ExecutionLog.Status` 仅 `ok/error`；超时记 `error`                    | 响应丢失后的业务成功被报为失败；操作员补发或重试，放大 R1。 |
| R3  | **平台幂等 ≠ 游戏 exactly-once** | `taskId`/页面键幂等只去重平台重放，不约束游戏自身在重启/重连下的副作用 | exactly-once 只在传输层去重成立，被当成游戏级保证。         |
| R4  | **取消谎称回滚**                 | 任务取消置 canceled 态，无游戏确认副作用                               | 「已取消」被读作「已回滚」，即使资产已发放。                |
| R5  | **同 operationId 参数漂移**      | 游戏侧无快照指纹，重试可带改过的参数用同一身份                         | 绕过重新审批，已批准意图被静默篡改。                        |
| R6  | **无凭据 → 伪造成功**            | 无凭据/查询契约；未知结果靠重新执行而非查游戏原始结果收敛              | 重复发放（R1）+ 不可恢复的「未知」态无查询路径。            |

设计**不得**重建与 `taskId`/页面幂等重复的系统：两者保持为平台传输/页面重放层；
业务身份（`operationId`）与游戏侧持久去重是**独立的、游戏协作层**。两者互相引用
但不互相替代。

## 2. 协议设计（5 项待决决策的方案）

### D1 — operationId 编码

**方案：** `operationId = <gameId>:<env>:<domain>:<intentUUID>`，其中
`intentUUID` 为 UUIDv7（时间有序、索引友好）。最大长度 128 小写 ASCII；字符集
`[a-z0-9:-]`。平台在业务意图创建时生成；不可变，按 `(gameId, env, domain,
intentUUID)` 全局唯一。对玩家显示短码（UUID 后 8 位加校验），不暴露内部自增 ID。

**理由：** 把 scope+domain 编进身份使跨 game/scope 碰撞在结构上不可能，复合唯一
索引自然；UUIDv7 比 UUIDv4 随机对插入更友好。

### D2 — 凭据存储形式

**方案：** 专用 `business_operation_credentials` 表（`(game_id, env,
operation_id, request_fingerprint, outcome, game_receipt, result_json,
query_hint, created_at, updated_at)`），`(game_id, env, operation_id)` 唯一，
每 operationId 一行。`game_receipt` 展示脱敏，操作员/审计有受控全量读取路径。

**理由：** 把长生命周期业务凭据与短保留执行日志解耦；凭据按 `operationId` 可查
（R2/R6 收敛），日志清理后仍存活。扩展 `TaskRun`/`ExecutionLog` 列把凭据生命周期
绑到传输生命周期；纯结构化 `ResultPayload` 在日志内不具独立可查询性。

### D3 — SDK 结果信封

**方案：** 每个游戏 SDK 内置 `GameOperationResult` 信封（`operationId`,
`requestFingerprint`, `outcome`, `gameReceipt`, `result`, `retryableAfter?`,
`queryHint?`）作为高风险函数规范返回。代理 HTTP 接口的外部函数插件必须声明
`durableIdempotency` 能力；未声明则拒绝进入高风险执行目录。

**理由：** 单一信封类型让平台无需每游戏临时映射即可解析 outcome；能力声明是安全
准入外部插件的唯一途径（其内部无法检查幂等性）。

### D4 — 未知结果策略

**方案：** 默认每 60s 自动查询（最多 10 次）契约的读结果函数，随后 `unknown` 告警；
仅 `retryable_failed`/`unknown` 自动重试，复用同一 operationId 与快照，退避 1s→60s、
上限 8 次、并发上限、游戏健康门；超过自动重试上限不再补发——只告警与操作员查询。
未知**绝不**改写成成功/失败。

**理由：** 周期查询无需新执行即可收敛 R6；上限防无限查询循环。未知时补发即伪造成功。

### D5 — 取消协议

**方案：** 取消是高风险函数的最低集成能力（查询/确认游戏取消请求）；函数可另行声明
`supportsUndo`。平台发游戏取消请求并要求游戏确认后才报 `canceled`；`cancel_failed`
保留原始结果，只经撤销/补偿（新审批 + 新 operationId）处理。

**理由：** 无确认则 `canceled` 伪造 R4。设为强制（非可选）让每个高风险函数的状态机
诚实；`supportsUndo` 保持可选，因为原生撤销是游戏专属。

## 3. 备选对比

| 决策           | 推荐                                         | 备选                                                           | 为何否                                                                                       |
| -------------- | -------------------------------------------- | -------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| D1 operationId | 复合 `gameId:env:domain:UUIDv7`              | 裸 UUIDv4；服务端自增；仅短码                                  | UUIDv4 丢时间序；自增泄漏规模且非全局唯一；仅短码非唯一键。                                  |
| D2 凭据        | 专用 `business_operation_credentials` 表     | 扩展 `TaskRun`/`ExecutionLog` 列；日志内结构化 `ResultPayload` | 日志保留短于凭据；凭据生命周期绑传输生命周期会在日志清理后断收敛；日志载荷不具独立可查询性。 |
| D3 信封        | SDK `GameOperationResult` 信封 + 能力声明    | 仅结构化错误码映射；插件自报安全                               | 错误映射无法表达 `unknown`/`conflict`/`queryHint`；无能力检查的自报安全插件不可审计。        |
| D4 未知        | 自动查询（60s×10）→ 告警；重试上限 8；不补发 | 无限重试；N 次后判失败；仅操作员手动查询                       | 无限重试死循环；判失败伪造失败；仅查询收敛慢。                                               |
| D5 取消        | 强制确认；`supportsUndo` 可选                | 可选取消协议；取消即隐式回滚                                   | 可选让函数跳过确认复现 R4；隐式回滚正是设计所禁。                                            |

## 4. 验收映射（设计 → 契约场景）

契约 §9 验收场景满足方式：场景 1（响应丢失不重复发放）由 D2+D3 持久去重 + D4
查询；场景 2（同 ID 改参拒绝）由 D1 指纹 + D3 `conflict`；场景 3（审批后 scope
变化拦截）由契约 §3.2 快照/复核（此处不变）；场景 4（未知不误判）由 D4；场景 5
（取消不伪造回滚）由 D5；场景 6（追踪链完整）由 D1 身份 + D2 凭据关联；场景 7
（过期不复活）由契约 §4.4 保留期 + D2。

## 5. GAME-010 实现范围

批准后，GAME-010 实现方案**平台侧**：`operationId` 生成、
`business_operation_credentials` 持久化（模型 + 编号迁移）、Go SDK 的
`GameOperationResult` 信封、读结果查询路径、自动查询/重试策略（D4）、取消确认
边界（D5）。明确**超出范围直至有真实游戏环境**：游戏侧持久去重存储、对真实游戏的
端到端验收（被 GAME-002/GAME-008 与真实集成环境阻塞，见 todo.md）。Go
内存/state-file 演示不能替代游戏侧持久去重。

## 6. 待负责人拍板

**状态（2026-11-27）：D1–D5 已获用户批准（2026-11-27），GAME-010 平台侧已实现。** 平台侧交付切片：

- `2ae38cc86` — D1/D2：operationId 复合标识 + `business_operation_credentials` 唯一凭据存储（模型 + 迁移 0069）
- `cfca4c5df` — D3：OpenAPI provider 导入路径的 `x-durable-idempotency` 能力声明 + 调用链登记/准入（高危函数未声明即 fail-closed 拒绝；broadcast 拒绝）
- `9f7fdaacc` — D4：自动收敛循环（查询 60s×10、同 operationId+同快照重试 1s→60s 封顶 ×8、耗尽告警、unknown 绝不改写）+ 迁移 0070（参数快照列）
- `403014a24` — D5：游戏确认取消（只接受 `canceled`/`cancel_failed`；`cancel_failed` 保留原结果；终态凭据拒绝取消）+ 运维端点 `GET/POST /ops/business-operations/:operationId(/cancel)`
- SDK 半边：proto 两个函数描述符新增 `durable_query`/`durable_cancel`/`durable_undo`，Go SDK `SetDurableIdempotency` builder + OpenAPI 导入提取 + `GameOperationResult` 信封便捷构造（`sdks/go/pkg/croupier/durable.go`）；线上契约见 `docs/architecture/sdk-wire-protocol.md`「持久操作信封」。

已知边界（诚实范围）：游戏侧持久去重存储与真实环境端到端验收仍以 GAME-002/GAME-008 与真实接入环境为门禁；当前仅 OpenAPI provider 导入路径与 SDK 描述符路径可声明能力（函数契约目录尚未透出该面）；其余语言 SDK 尚未随附信封便捷构造。Go 内存/state-file demo 不能替代游戏侧持久去重。

1. ~~批准或修订 D1–D5 如本方案。~~ **已于 2026-11-27 批准。**
2. 确认凭据保留期（方案：180 天，可配置）——仍开放；凭据当前无限期保留（无 TTL 清理）。
3. 确认即使暂无真实游戏环境，GAME-010 也先做平台侧（游戏侧另立门禁）——已由 2026-11-27 批准回答。
