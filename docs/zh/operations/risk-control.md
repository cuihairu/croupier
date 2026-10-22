---
title: 风控说明
---

# 风控说明

本文只描述当前仓库已实现的能力。风控定义、发布、执行、命中与处置分属控制面和 Agent 执行面；规则定义与执行分离，当前平台不会在规则命中后自动阻断数据库写入、封禁账号或调用游戏处置接口。

## 能力总览

| 风控点       | 判定依据                                                                     | 当前状态                                   |
| ------------ | ---------------------------------------------------------------------------- | ------------------------------------------ |
| CDC 事件过滤 | `event.ops` 操作白名单；未列入的合法操作不进入后续闸，未知操作以错误计数丢弃 | 已实现，默认空列表全放行                   |
| 对账闸       | 道具表变更必须在时间窗内匹配业务流水变更，超窗未匹配即告警                   | 已实现，只告警不阻断                       |
| CEP 三模板   | 非白名单时段变更、单次或窗口累计数量突变、连续小额试探                       | 已实现，只告警不阻断                       |
| 血缘闸       | 受监控表写入账号必须命中白名单                                               | 降级实现：CDC 事件无会话账号，恒跳过并计数 |
| 规则发布     | `capture.gates` 版本化发布；dry-run、scope、采集表和连接器能力预检           | 已实现，无审批工作流                       |
| 风险事件处置 | 命中生成告警；确认、忽略、升级加备注留痕                                     | 已实现，自动封禁/补偿/阻断未实现           |
| 函数风险评级 | 函数与操作分 safe/warning/high/danger 四级；声明、归一化与治理解析三段口径   | 已实现                                     |
| 高危策略审批 | high/danger 调用强制进入审批流，两人复核加 TOTP 二次验证，全程审计           | 已实现；`capture.gates` 发布审批未实现     |

## 风控规则

### 事件过滤

- `agents/capture/filter/event.go:5` 定义只做廉价确定性判断的 `EventFilter`；`agents/capture/filter/event.go:11` 明确未知操作是 `ResultError`，需要计数丢弃，不能静默吞掉。
- `agents/capture/filter/event.go:18` 构造器只接受 `insert/update/delete`，空列表保持全放行；`agents/capture/filter/event.go:35` 执行判定。
- `agents/capture/config/gates_validate.go:43` 在规则集校验中构造事件过滤器，`agents/capture/config/gates_validate.go:18` 是服务端 dry-run 与 Agent 应用共用的校验真值。

### 对账闸

- `agents/capture/gate/reconcile.go:41` 定义“道具变更必须匹配业务流水”；`agents/capture/gate/reconcile.go:46` 说明道具侧取不到键入 never-match 哨兵，宁误报不漏报。
- `agents/capture/gate/reconcile.go:48` 规则字段包含双表、双侧键、`WindowSec`、`Severity`、豁免键和 `MaxPending`；`agents/capture/gate/reconcile.go:60` 校验必填、表形态、严重级和上限。
- `agents/capture/gate/reconcile.go:36` 设默认 300 秒窗口和单规则 10000 条在窗上限；`agents/capture/gate/reconcile.go:91` 和 `agents/capture/gate/reconcile.go:99` 应用显式配置或默认值。
- `agents/capture/gate/reconcile.go:107` 禁止同一表被两条对账规则重复路由；`agents/capture/gate/reconcile.go:164` 说明双向时间窗销账，超窗产生 `reconcile.missing_ledger` 告警。

### CEP 规则

- `agents/capture/gate/cep.go:13` 固定模板闭集为 `cep.off_hours`、`cep.burst_delta`、`cep.probing`；`agents/capture/gate/cep.go:24` 设累计桶和追踪键上限各 10000。
- 时段规则：`agents/capture/gate/cep.go:34` 定义表/操作乘白名单时段，`agents/capture/gate/cep.go:80` 校验时段、时区偏移和严重级。
- 突变规则：`agents/capture/gate/cep.go:45` 支持 `single` 与 `window`；`agents/capture/gate/cep.go:50` 要求 `Threshold > 0`，窗口模式另有 `WindowSec > 0`，`agents/capture/gate/cep.go:105` 完成静态校验。
- 连续试探：`agents/capture/gate/cep.go:62` 判定同表同键在窗口内的连续小变更；`agents/capture/gate/cep.go:67` 说明账号不可用时降级为行键，`agents/capture/gate/cep.go:137` 要求 `Count >= 2`、窗口大于 0。

### 血缘闸

- `agents/capture/gate/lineage.go:12` 定义白名单账号外写入受监控表为异常。
- `agents/capture/gate/lineage.go:15` 声明当前 binlog/WAL 事件不带会话账号；`agents/capture/gate/lineage.go:38` 要求 `allowAccounts` 非空，但运行时 `agents/capture/gate/lineage.go:89` 对空账号跳过并递增 `Degraded` 计数。
- 因此血缘闸当前不产生有效风险事件；启用它只代表白名单配置与降级计数可观测。

## 风险评级

函数与操作的风险分四级，语义在 `internal/dashboard/spec/types.go:78` 定死：`safe` 只读无副作用、`warning` 中等副作用、`high` 显著副作用需确认、`danger` 破坏性或不可逆、要求审批（`internal/dashboard/spec/types.go:87`）。

- 声明入口是 OpenAPI 的 `x-risk` 扩展：`internal/drivers/openapi/provider.go:192` 定义该字段随函数契约上传。
- 归一化在 `internal/dashboard/normalizer/normalizer.go:300`：接受 legacy 别名 `low/medium/critical`（与 `internal/dbenum/p1_enums.go:264` 的 `ParseRisk` 保持同步），未知值归空、落库为 safe。
- 治理解析在 `internal/platform/functioninvoke/policy.go:35`：优先级为内存 OpenAPI 的 `x-risk` → 契约行非 safe risk（safe 零值不采信）→ 兜底 medium；`internal/platform/functioninvoke/policy.go:72` 的 `ResolvePolicyRisk` 把同一口径暴露给审批路径，禁止各包私抄解析逻辑。
- 评级直接驱动生成页面：`internal/dashboard/generator/resource_generator.go:851` 对 high/danger 生成 danger 级按钮，`internal/dashboard/generator/resource_generator.go:860` 对 high/danger 强制二次确认，审批策略 Required 时同样确认。
- 运行策略可按函数覆盖：`internal/model/function.go:72` 区分 `default`（按风险级自动生成）与 `manual` 两种来源，覆盖项含 `RequireApproval`、`ApprovalWorkflow` 与 `RequireAudit`。

## 策略审批

高危调用不直接执行：`internal/platform/functioninvoke/invoke.go:125` 在函数策略 `RequireApproval` 且审批存储就绪时把调用转入审批流，调用方拿到审批单号。审批面与规则：

- 审批 API：`internal/handler/routes.go:229` 挂载 `/approvals`，`internal/handler/routes.go:856` 至 `internal/handler/routes.go:859` 注册 List/Get/Approve/Reject 四个端点；实现分别位于 `internal/api/approval/service.go:38`、`:159`、`:198`、`:273`。
- 两人规则：`internal/api/approval/service.go:120` 拒绝自审批并落审计（`:138`）；`internal/config/config.go:865` 的 `ApprovalConfig.AllowSelfApprove` 默认 false，仅建议单管理员部署显式打开。
- 高危 step-up TOTP：`internal/api/approval/stepup_otp.go:23` 规定治理风险 high/danger（与调用路径共用 `ResolvePolicyRisk` 口径，`:66`）的批准必须二次验证；已绑定 TOTP 的账号 otp 必填且校验通过，未绑定账号以 `otp_not_enrolled` 直接拒绝（`internal/api/approval/stepup_otp.go:123`），高危批准禁止静默放行。
- 批准后续接：`internal/api/approval/service.go:494` 执行批准后的函数调用续接；`:536` 校验页面类审批的已发布快照仍新鲜，审批策略变更后旧快照不得放行。
- scope 隔离与审计：`internal/api/approval/service.go:369` 强制审批操作携带 game/env scope；`:325` 把每次动作写入审计链。
- 待审模型：`internal/model/function.go:84` 的 `PendingFunction` 保存等待审批的函数注册。

当前未实现：`capture.gates` 风控规则发布复用该审批模型（见下节计划）。

## 发布与校验

- 控制面键是 `capture.gates`：`internal/api/ops/capture_gates.go:30` 定义常量，`internal/api/ops/capture_gates.go:226` 要求 game/env 非空，随后进行 JSON 结构 lint、语义校验和 Agent 事实预检。
- 发布预检：`internal/api/ops/capture_gates.go:189` dry-run 不落库；`internal/api/ops/capture_gates.go:209` 调用 Agent 同款 `ValidateGates`。
- 采集事实预检：`internal/api/ops/capture_gates.go:379` 按同 scope 在线 capture Agent 的 labels 收集表并集和能力交集；`internal/api/ops/capture_gates.go:484` 拒绝对账、CEP 突变表不在采集范围或缺少 after image 的版本。Agent 事实不可知时 `internal/api/ops/capture_gates.go:485` 跳过这一层，不拦发布。
- 版本与回滚：`internal/api/ops/capture_gates.go:266` 回滚是取历史内容再发新版本；`internal/api/ops/capture_gates.go:286` 重新过当前 lint。前端在 `web/src/pages/Risk/Strategies/index.tsx:253` 提示发布成功不等于已生效，生效以 Agent 自报对账为准，约 30 秒轮询加滞后。
- 当前未实现：风控策略发布独立审批、双人复核、发布熔断和“坏版本回滚后自动重放原版本”。

**计划**：为 `capture.gates` 发布接入既有审批模型，要求高危 scope 或大规则集变更提供审批快照；增加发布后首个周期失败率熔断，避免 bad revision 继续等待 Agent 轮询。

## 告警与处置

### 命中上报

- `agents/capture/runner/runner.go:573` 把三闸 `Hit` 组包为 `CaptureRuleHitReport`；`internal/server/control_handler.go:492` 把 `critical/warn` 归一为固定严重级，`internal/server/control_handler.go:536` 通过 `AlertModel.UpsertByAlertID` 幂等落库。
- `internal/model/alert_model.go:51` 按唯一 `alert_id` 去重；`internal/model/alert_model.go:157` 对当前 game/env scope 告警加平台级告警合并展示。

### 状态流转

- `internal/api/ops/dto.go:221` 定义确认、忽略、升级三种动作；`internal/api/ops/helpers.go:589` 映射目标状态，`internal/api/ops/helpers.go:595` 校验动作与告警存在性。
- `internal/api/ops/helpers.go:620` 将状态、处理人、备注与库钟时间一次落库；`internal/model/alert_model.go:116` 保证审计字段同语句更新。路由在 `internal/handler/routes.go:579`。
- 当前未实现：升级动作的审批钩子和后续系统动作。`internal/api/ops/helpers.go:586` 已把“高危流转审批钩子”标记为后续批次。

**计划**：给 `escalate` 增加可配置审批链；命中证据展开表名、行键、规则快照和关联事件；提供可选处置适配器，先支持只读核查，再经审批接封禁、补偿或游戏侧操作。

## 明确未实现

- 执行阻断：CDC 规则命中不回滚或阻止源库变更；探测与执行面隔离，`internal/app/agent/probe_manager.go:25` 明确探测不触发恢复动作。
- 账号封禁、冻结、回滚、补偿等风控动作：当前只有人工告警状态流转。
- 风险评分与自适应阈值：没有跨规则综合评分、历史基线或机器学习模型。
- 完整会话血缘：CDC 事件账号恒空，血缘闸降级。
- 自动恢复/自动升级闭环：命中不会创建任务、工单或触发外部系统。

## 最小运行建议

1. 发布前对同一 game/env 配齐 `captureTables` 与连接器能力 labels，确保依赖预检不是跳过状态。
2. 对账规则覆盖道具表和业务流水表，设置可理解的 `WindowSec`；不为避免噪声把窗口压得过短。
3. CEP 突变使用单事件阈值加滑动窗口组合；试探规则至少 2 次命中。
4. 血缘闸只用于审计降级计数，不能作为当前防护控制。
5. 命中处理使用确认/忽略/升级加备注留痕；升级后的真实处置须另走受控运维流程。
