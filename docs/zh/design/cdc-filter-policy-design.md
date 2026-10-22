---
title: CDC 过滤策略体系——统一策略契约、分层执行与下发确认（用户令 2026-10-11 整理）
---

# CDC 过滤策略体系（Filter Policy）

## 状态

- 状态: **契约已落码，执行层首闸已接线（2026-10-11 阶段一+二交付）**。本文是用户令的整理稿：分层过滤模型、四类规则、统一策略契约、下发确认链、位点关系、失败语义与可观测性。契约面（统一策略头/库表授权匹配默认拒绝/依赖检查/结果四分类）已落地 `agents/capture/filter/`（纯逻辑零依赖，单测锁定）；EventFilter（操作白名单）已接入 runner 流水线进闸前（阶段二，缺省=全放行向后兼容）；位点-策略联动已拍板（§4）。任务拆分见 `todo.md`「CDC 平台化批」；与 [capture-agent 简档](capture-agent-design.md)（现有三闸实现）、CDC Connector 架构（多数据库接入）、风控面板设计（面板归属）互链。
- **定调（用户令，2026-10-11）**：CDC Agent 不是过滤规则的最终管理中心——它是部署在数据源旁的采集与传输组件，负责尽早排除不需要的数据；规则的统一管理、版本控制和过滤语义由控制面负责。**统一的是策略定义与管理，不是强制所有事件回服务端过滤。**

## 0. 作用（先读这节）

解决三个问题：

1. **性能**：不需要的数据尽量在靠近数据源的位置丢弃（不回传服务端再判断）。
2. **一致性**：不同 Agent 不会各自实现一套互相冲突的过滤规则（语义统一在控制面）。
3. **可运维性**：规则统一配置、下发、审计、回滚，不逐台改 Agent。

**四层过滤视角**（同一次 `game.items` 变更依次经过）：

| 层             | 判断                                         | 执行位置          |
| -------------- | -------------------------------------------- | ----------------- |
| ① 采集范围过滤 | 这张表是否需要监控？                         | Agent（数据源旁） |
| ② 事件过滤     | 这类 INSERT/UPDATE/DELETE 是否进入处理流程？ | Agent             |
| ③ 业务规则判断 | 这次变更是否存在异常？                       | Agent（三道闸）   |
| ④ 输出过滤     | 哪些下游订阅者需要收到该事件/告警？          | Server            |

不区分这四层，后续增加 PostgreSQL、更多数据源或新业务规则时，采集逻辑、业务判断和推送逻辑就会混在一起。

## 1. 四类规则（统一策略模型，分类型承载）

**原则：统一策略模型 ≠ 把四类规则塞进同一个庞大 JSON。** 统一策略头部（identity/scope/revision/lifecycle）+ 分类型配置体：

| 规则类型             | 负责什么                                            | 默认执行位置   | 面板归属           |
| -------------------- | --------------------------------------------------- | -------------- | ------------------ |
| `SourceFilterPolicy` | 哪些数据库/Schema/表/操作需要采集；字段采集与脱敏   | 数据源 / Agent | 数据源（采集范围） |
| `EventFilterPolicy`  | 哪些变更事件进入处理流水线（廉价确定性条件）        | Agent          | 数据源 / 采集策略  |
| `DetectionPolicy`    | 如何识别异常变更（对账 / CEP / 血缘三闸）           | Agent          | 风控（检测规则）   |
| `DeliveryPolicy`     | 哪些事件/告警发给哪些订阅者（scope/权限/脱敏/路由） | Server         | 消息与数据访问策略 |

概念示例（非现有 API）：

```yaml
apiVersion: croupier.io/v1
kind: FilterPolicy
metadata:
  id: game-item-monitor
  revision: 12
scope:
  gameId: game-a
  env: prod
  sourceId: mysql-main
source:
  tables:
    include: [game.items, game.player_wallet, game.ledger]
  operations:
    include: [insert, update, delete]
event:
  operations:
    include: [insert, update, delete]
detection:
  reconcile: { enabled: true }
  cep: { enabled: true }
  lineage: { enabled: true }
```

### 1.1 SourceFilter（采集范围）

- 库/Schema/表 include/exclude；操作类型；字段采集与脱敏策略；数据源能力允许时下推给连接器/日志订阅机制。
- 现状锚点：`agents/capture/source` 的 `MySQLConfig.Tables` 是本层初步实现（本地 YAML），须提升为中心管理策略。契约面已落码：`filter.TableMatcher`（include/exclude + 默认拒绝 + 排除优先 + 冲突构造即错）+ `MissingDependencies` 依赖检查。
- **重要限制**：不为省事件量漏掉业务流水表——对账规则依赖 `game.items` + `game.ledger` 时，`ledger` 即使不单独告警也必须采集（规则依赖校验在发布时做，见 §3）。

### 1.2 EventFilter（事件粗过滤）

- 只做廉价确定性判断：忽略不受监控的操作类型、跳过明确不属于流水线的事件、按事件元数据（来源/环境/类别）筛、判断后续规则必要元数据是否存在。
- **不能在粗过滤丢弃可能触发异常检测的事件**：某条 UPDATE 没有可解析的数量字段，不能因此丢弃——它仍可能命中非白名单时段规则（现状三闸实现已遵守：delta 取不到是跳过计数 `SkippedNoDelta` 而非吞事件，其它闸照常评估）。
- **v1 已落码并接线（2026-10-11 阶段二起步）**：`filter.EventFilter` 仅操作类型白名单（`event.ops` 段，缺省空=全放行向后兼容）；runner consume 进闸前过滤，`FilteredNoMatch`/`FilteredError` 计数；未知操作=Error 计数丢弃（形态不可评估，可观测不静默）。表级不进事件过滤（放行语义 vs 授权语义不同，见 §1.1）。
- 职责边界：不代替权限校验；普通用户配置的宽泛规则不得覆盖安全约束。

### 1.3 DetectionPolicy（业务异常检测）

- 复用现有三道闸（`agents/capture/gate/` 的 reconcile/cep/lineage），**保留在 Agent 端执行**：对账和 CEP 需要窗口/计数器/待匹配状态，全部搬服务端会增加网络依赖并把实时检测变成中心负担。
- `gate` 包继续作为规则执行引擎（规则集已可原子热替换，见简档 §8.5 gateSet 快照），**不造通用规则引擎，不引 CEL/Drools/Flink**——参数化模板 + 确定性匹配足够。
- 规则的配置、版本、启停、审计归 Server 统一管理（本批前由 `capture.gates` config_versions 键承载，策略化后迁入 DetectionPolicy，见 §7 迁移路径）。

### 1.4 DeliveryPolicy（输出与订阅）

- Server 侧：同一条异常事件——安全运营收完整告警、普通运营收脱敏详情、分析系统收聚合统计、其他游戏/环境不收。
- 涉及 scope、订阅者权限、数据脱敏，**不能依赖 Agent 粗过滤实现**；复用现有告警体系（alerts + outlet），单独管理输出范围与订阅权限。

## 2. 策略生命周期（desiredRevision vs appliedRevision）

下发六步：**编辑 → 校验 → 发布（不可变 revision，记录发布人/时间/前一版本）→ 下发（只发匹配 gameId+env+sourceId 的 Agent）→ 应用（Agent 本地全量验证 + 原子切换）→ 确认（上报实际生效版本）**。

- Agent 缓存已发布策略，正常运行完全离线执行；**不逐事件向 Server 请求过滤判断**。
- 现状通道锚点：下行拉取 wire 对 `0x04011B/0x04011C` + `core/configsync` 版本轮询（简档 §8.5）就是第 4/5 步的雏形；**第 6 步已通（2026-10-11）**：agent 生效后异步重注册，`captureGatesRevision` label 捎带 appliedRevision，`/ops/capture` 「生效对账」行对账 desired vs applied（滞后容忍）；第 3 步的不可变 revision 链已显式化（乐观锁 baseVersion，冲突 409 断链拒绝，简档 §8.10）——「统一策略头嵌入」归按类型分键时再做。
- **发布状态必须区分 `desiredRevision`（期望）与 `appliedRevision`（实际生效）**——面板不得在只有 desired 时显示"已生效"。
- 策略变更两特例：
  1. 改 CEP 阈值：安全边界内原子切换规则实例；是否保留原窗口状态须明确规定（当前实现：整体换 gateSet 快照，窗口状态不保留、重新起算，简档 §8.2 时序边界）。
  2. 新增采集表：必须定义从哪个位点生效——**不能只更新白名单**，否则新表历史变更永远无法补采（拍板见 §4：当前位点增量生效，历史走独立回补）。

## 3. 发布时校验（能力感知 + 依赖感知）

- **已落码（2026-10-13 服务端化，capture-agent 简档 §8.13）**：发布路径 = 结构 lint + `config.ValidateGates` 语义真值 + 依赖感知（对账双表/CEP 突变表 ⊆ 采集范围，范围外发布即拒点名表名）+ 能力感知（对账/CEP 要求 `afterImage`）。能力集与采集范围经注册 labels 自报（`captureTables` / `captureConnectorCapabilities`，拍板：labels 通道零 proto；事实不可知跳过 lint 滞后容忍）。
- include/exclude 冲突在发布时检测，不靠各 Agent 自行解释。
- **默认拒绝新增采集范围**：未明确授权的库/表不自动加入。
- 业务安全规则不能被普通事件过滤规则覆盖。
- 每个策略绑定 gameId+env，生产/测试不串。

## 4. 过滤与 CDC 位点的关系（已拍板 2026-10-11）

场景：Agent 只监控 `game.items` 运行一周，今天新增 `game.ledger`——全局复制流位点可能已越过 ledger 历史变更，更新白名单**不会自动补回过去的数据**。

拍板四条（均以现有代码路径为锚，把现状定为契约）：

1. **位点所有权唯一归事件源，策略层无位点概念。** 位点只由 `source.Provider.Bookmark()` / `connector.SourceConnector.Checkpoint()` 产生与推进（runner 周期落盘 `source.Store`）；策略热换走 gateSet atomic 快照，两条路径互不触碰。策略发布/回滚/退役在任何 revision 下都**不移动位点、不触发重放**。
2. **新规则/新表生效起点 = 当前位点。** 发布确认后的增量开始可见；发布不隐式补历史，白名单更新不回放过去的数据。
3. **需要历史 = 独立回补流程，显式触发。** 回补是独立动作（独立任务 + 独立审计），挂 Connector `snapshot` 能力（契约已定：snapshot 无 `resumableCheckpoint` = 非法组合）；**永不随策略发布顺带执行**。
4. **切换审计先落日志口径**：apply 成功时记录 `capture gates version applied revision=N position=<kind>:<value>`（位点串非敏感）；server 侧切换审计随阶段三策略资产化扩展，复用 appliedRevision 捎带模式、不新增 wire 对。

**底线检验（代码现状已满足，定为契约）：过滤规则变更不破坏 CDC 可恢复性。** 策略非法 keep last-good 时复制流照常推进、事件照常用旧规则评估（位点推进不因策略失败而停）；策略切换也不反过来重置位点。空位点/位点过期仍走既有降级路径（从可得起点重扫，binlog 保留期约束不变）。

## 5. 失败语义（不能一律丢弃）

策略执行结果四分类：

| 结果          | 语义                                             |
| ------------- | ------------------------------------------------ |
| `MATCH`       | 命中规则                                         |
| `NO_MATCH`    | 规则正常执行，条件不满足                         |
| `ERROR`       | 规则无法正常执行（解析失败/字段缺失/版本不兼容） |
| `UNSUPPORTED` | 当前 Agent 不支持该策略能力                      |

- SourceFilter 与安全相关策略：默认拒绝未经授权的采集范围。
- 已采集事件检测失败：**优先保留事件、记录错误、产生可观测信号，不静默丢弃**（现状三闸口径一致：校验失败整体拒绝应用 keep last-good，运行期错误计数上报）。

## 6. 字段过滤与敏感数据

- 「用于规则计算的字段」与「最终可输出的字段」分开：规则需要的字段在受控 Agent 内部使用；告警载荷只留必要字段；面板/订阅按权限脱敏；不需要的敏感字段尽量不采集不传输。
- **字段过滤不得误删对账关联键、主键、变更前像**（否则破坏检测能力）。
- 现状锚点：告警 Details 里的 key/gtid/detail 已按最小面设计；行像是位置数组 JSON（简档 §9），列名映射（information_schema 摄取）落地时同步设计字段白名单。

## 7. 落到现有代码（不重写 gate/）

| 模块                     | 改造                                                                                                                                             |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `agents/capture/source/` | 保留数据源适配与采集范围过滤；位点所有权归事件源（`Bookmark`/`Checkpoint`），策略变更不移动位点（§4 已拍板）                                     |
| `agents/capture/gate/`   | 保留三闸执行逻辑；规则集已可替换实例（gateSet 快照）                                                                                             |
| `agents/capture/config/` | 本地连接配置与中心下发策略已分层：`capture.gates.*` 文件规则 = fallback（无中心/未发布时兜底，中心版本拉取生效即整体覆盖，启动日志显式标注）     |
| `agents/capture/runner/` | 装配过滤流水线、原子切换、统计过滤结果、上报生效版本（新增 `agents/capture/filter/` 纯逻辑包：事件级匹配与过滤决策，不依赖 MySQL 驱动/面板 API） |
| Server Agent 控制链      | 策略下发确认、版本同步、失败状态（appliedRevision 上行）                                                                                         |
| 面板                     | 策略编辑、预览校验、发布、回滚、Agent 生效状态（归属见风控面板设计）                                                                             |
| 告警与订阅               | 复用现有告警体系，单独管理输出范围与订阅权限（DeliveryPolicy）                                                                                   |

**现状基线（2026-10-11 已交付）**：`capture.gates` 下行拉取链（wire 0x04011B/C + configsync 30s + gateSet 原子热换 + 服务端结构 lint 面板发布）= DetectionPolicy 下发链的 v1；面板 JSON 编辑卡 = 策略编辑器 v1。策略化改造在此基线上演进，不推翻。

## 8. 可观测性（过滤统计必须上报）

| 指标                           | 用途                         |
| ------------------------------ | ---------------------------- |
| `events_received_total`        | 收到的原始变更事件数         |
| `events_filtered_total`        | 被过滤的事件数（按原因分类） |
| `events_evaluated_total`       | 进入规则引擎的事件数         |
| `policy_apply_errors_total`    | 策略应用失败次数             |
| `rule_evaluation_errors_total` | 规则执行错误次数             |
| `policy_applied_revision`      | 当前实际生效版本             |

用途：判断过滤是否过严、规则是否失效、Agent 是否因配置不兼容漏检。现状锚点：Stats.Total/Insert/Update/Delete/Skipped/GateHits/AlertDropped 之外，`FilteredNoMatch`/`FilteredError`（events_filtered_total 按原因分类的首批）、`Evaluated`（events_evaluated_total，粗过滤后进闸评估才计）、`PolicyApplyErrors`（policy_apply_errors_total：面板下发解析/校验失败 keep last-good 时计数）、`RuleEvalErrors`（rule_evaluation_errors_total：CEP 检测失败保留事件只计门上计数，sweep 周期聚合，SkippedNoDelta 口径推广）已落码（2026-10-13 阶段二收尾）；汇总上报与 revision 上报随阶段三。

## 9. 实施顺序（四阶段）

1. **契约（✅ 2026-10-11 交付）**：四类规则边界（`filter.PolicyKind` 闭集）、统一事件模型（`connector.ChangeEvent` 含能力差异字段）、include/exclude 语义（精确+库级通配，排除优先）、默认拒绝、失败四分类（`filter.Result`）。
2. **Agent 执行层（2026-10-11 起步）**：EventFilter v1 已落码接线（操作白名单+计数）；余项：元数据存在性筛、闸层评估错误计数、过滤层单测扩展。
3. **中心策略管理**：策略资产化（统一策略头 + 分类型体）、校验（含能力/依赖感知）、版本化、下发确认、回滚；移除业务规则手工 YAML 依赖。
4. **面板与验证**：策略管理界面；测试规则更新、断线恢复、位点恢复、新表启用、过滤错误、跨游戏/环境隔离。

任务拆分见 `todo.md`「CDC 平台化批」。

## 10. 已知边界（诚实清单）

- 阶段一契约面已落码（`agents/capture/filter/`：policy.go 统一策略头+生命周期+revision 推进语义、match.go 库表授权+默认拒绝+依赖检查+结果四分类，单测 7 组）；「建议/应当」其余各项均待实施。落码时的契约决定：include/exclude 冲突只认精确表项同列两侧（库级 exclude 压 include=合法关整库手势，排除确定性取胜）；`MissingDependencies` 对 nil matcher（无授权信息）保守返回全部缺失。
- 位点-策略联动决策已定（§4，2026-10-11）：位点归事件源、新表从当前位点增量生效、历史走独立回补流程；回补流程本身未实现（依赖 Connector `snapshot` 能力，随 Connector 批次落地），策略切换审计先落日志口径（agent 日志，server 侧审计随资产化扩展）。
- appliedRevision 上报通道已通（2026-10-11，注册 labels 捎带 + 面板生效对账行）；滞后窗口 = 轮询 30s + 重注册，不作强一致读。策略资产化（统一策略头/不可变 revision 存储）仍是阶段三余项。
- DeliveryPolicy 所在的订阅/脱敏面未设计详案（复用 alerts + outlet 的现有权限模型，扩展另批）。
