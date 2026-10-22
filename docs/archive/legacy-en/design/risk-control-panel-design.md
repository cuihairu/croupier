---
title: 风控面板设计——一级分类、四子模块与规则/执行分离（用户令 2026-10-11 整理）
---

# 风控面板设计（Risk Control）

## 状态

- 状态: **设计整理完成（2026-10-11）；实施进行中（CDC 平台化批阶段四）——§6 之 1+3+4+5 已落地**：菜单一级分类「风控」随首个真实页面「风控策略」（`/risk/strategies`）落地，空架子页不单独实施；告警中心已迁入 `/risk/alerts`（旧路径隐藏重定向兼容）；风险事件工作流四列已落库（§3.3 落地注记）。本文是用户令的整理稿：风控一级分类导航、四子模块、Filter 与 Detection 归属边界、规则定义与执行分离原则、明确不做清单。实施任务拆分见 `todo.md`「CDC 平台化批」；与 capture-agent 简档（三闸检测执行面）、CDC 过滤策略体系（DetectionPolicy 归属）、[CDC Connector 架构](cdc-connector-architecture.md)（能力透明展示）互链。
- **定调（用户令）**：轻量起步——先四子模块承载现有三闸告警链与策略下发，不做大平台。

## 1. 一级分类与四子模块

面板新增一级分类「风控」（与 运营/运维 同级），第一版四子模块：

| 子模块       | 内容                                         | 首版形态                                                                |
| ------------ | -------------------------------------------- | ----------------------------------------------------------------------- |
| **风控规则** | 规则模板/参数/适用范围/启停/测试             | 复用 /ops/capture JSON 编辑卡升级：reconcile/cep/lineage 三段结构化表单 |
| **风控策略** | 规则组合、版本发布、Agent 生效状态、回滚记录 | config_versions 版本列表 + desired/applied 双状态 + 回滚按钮            |
| **风险事件** | 命中规则/关联对象/证据/风险等级/状态流转     | 告警列表扩展「风险等级」+「状态流转」（待处理/已确认/已忽略/已升级）    |
| **告警中心** | 需人工处理的事件（风险事件不一定都告警）     | 现有 Alerts 页迁入/链接，不推翻                                         |

## 2. Filter 与 Detection 的归属边界（用户令边界表）

| 内容                           | 归属                                   |
| ------------------------------ | -------------------------------------- |
| 库表/操作筛选（哪些表采集）    | 数据源（采集范围）→ SourceFilterPolicy |
| 事件字段过滤                   | 数据源/采集策略 → EventFilterPolicy    |
| Reconcile / CEP / Lineage 规则 | **风控**（检测规则）→ DetectionPolicy  |
| 订阅范围/脱敏/路由             | 消息与数据访问策略 → DeliveryPolicy    |
| 风险事件和告警                 | **风控**                               |

原则：**规则定义与规则执行分离**——面板管定义/发布/审计/回滚，Agent 管执行（三闸留在 `agents/capture/gate/`，执行语义见过滤策略简档 §1.3）。

## 3. 子模块详案

### 3.1 风控规则

- 规则以模板 + 参数表达（不造通用规则引擎，见过滤策略简档 §1.3）：模板 = reconcile 双表对账 / cep 突发窗口 / lineage 允许账号三类；参数 = 表对、阈值、窗口、账号集。
- 每条规则：模板类型、参数、适用 scope（game+env，可选 sourceId）、启停开关、「测试」按钮（粘贴样例事件 → 面板调 server 校验接口做 dry-run，不触达生产 agent）。
- 启停是规则实例属性，属策略体的一部分（随策略 revision 发布，不单独直改 agent）。

### 3.2 风控策略

- 策略 = 该 scope 下全部启用规则的一次快照发布，不可变 revision（编辑→校验→发布→下发→确认六步，过滤策略简档 §2）。
- **生效状态双列**：desiredRevision（面板已发布）vs appliedRevision（agent 上报），不一致即「待生效」；全部对齐才显示「已生效」。
- 回滚 = 以历史 revision 内容再发一版（不可变日志，非指针回拨）。
- 版本列表含发布人/时间/message（现 config_versions 已有字段，直接复用）。

### 3.3 风险事件

- 数据来源 = 现有三闸告警链（alert → server → alerts 表 + outlet 推送），首版不新建事件表，在告警上扩展：
  - `severity`：现有 low/medium/high 扩「critical」（风险等级）；
  - `status` 流转：待处理 → 已确认/已忽略/已升级（+处理人/备注，审计字段）；
  - 关联对象：agent/game/env/表/账号（key/gtid/detail 已有，补结构化列或 JSON 索引约定）。
- 状态流转操作走权限面（现有 RBAC/两人员），高危流转（升级/封号相关）留审批钩子。
- **✅ 状态流转已落地（2026-10-12，§6 之 4）**。拍板：人工处理状态独立四列（`handle_status`/`handled_by`/`handled_remark`/`handled_at`，迁移 0042），**不混入** `Alert.Status`——该列已被 AlertManager 入站 lifecycle（firing/resolved）语义占用，两套状态正交：入站态看「告警是否还在响」，处理态看「人跟进了没有」。流转从简：任意合法动作可流转到不同目标态（处理人/时间/备注覆盖留痕，审计追责优先），高危审批钩子留后续批次；`handled_at` 以服务端库钟为准。API：`POST /api/v1/ops/alerts/:id/handle`（action=acknowledge/ignore/escalate + remark，非法动作 422、id 非法 400、不存在 404）；列表带 `handleStatus` 过滤；前端 Alerts 页新增处理状态列（tooltip 带审计三件套）、处理 Dropdown→流转 Modal、处理状态过滤器。severity critical 档复用 capture 链既有 `outlet.SeverityCritical`，无模型改动。

### 3.4 告警中心

- 首版 = 现有 Alerts 页迁入风控分类（路由迁移 + 菜单改挂），数据面不动。
- 与风险事件的分工：告警中心是「需要人看的队列」（渠道推送 + 状态流转入口），风险事件是「发生过什么的台账」（全量记录含未告警的低风险命中）。

## 4. 与现有页面的关系

| 现有                                | 处置                                                                 |
| ----------------------------------- | -------------------------------------------------------------------- |
| `/ops/capture`（闸规则下发卡）      | 升级为风控规则/策略两页，`/ops/capture` 保留跳转或下线（拍板项）     |
| Ops/Nodes「捕获」列（血缘降级徽标） | 保留，另在风控总览加源能力标签（Connector 能力声明）                 |
| Alerts 页                           | 迁入风控分类作为告警中心                                             |
| config_versions（capture.gates）    | 继续承载策略 revision 存储；策略头加统一 metadata（过滤策略简档 §1） |

## 5. 明确不做（用户令）

- ❌ ML 反作弊
- ❌ 自动封号
- ❌ 玩家画像平台
- ❌ 实时决策引擎

风控 = 规则检测 + 告警 + 人工处理工作台；不做自动化处置，封号等高危操作永远走人工审批（两人员）。

## 6. 实施顺序（并入 CDC 平台化批阶段四）

1. ~~菜单一级分类「风控」+ 空架子页（路由/locales/access）~~——✅ 已落地（2026-10-11）。**拍板：空架子不单独落增量**，一级菜单随首个真实页面（风控策略页）一起上线，避免占位页（`routes-resolution.test.tsx` 断言真实页面而非占位）。
2. 风控规则页（三段结构化表单 + 测试 dry-run 接口）——dry-run 接口 v1 已先行落地（2026-10-12，capture-agent 简档 §8.12：`POST /capture/gates/validate`，校验真值与 agent apply 单一化，`/ops/capture` 已挂「校验」按钮）；结构化表单页待实施。
3. ~~风控策略页（版本列表 + desired/applied 状态 + 回滚）~~——✅ 已落地（2026-10-11，`/risk/strategies`）。版本历史走新增 `GET /api/v1/ops/capture/gates/versions`；回滚走既有 `POST /capture/gates/rollback`；生效对账复用 appliedRevision 捎带通道（`summarizeGatesApply` util 与 `/ops/capture` 共用）。
4. ~~风险事件扩展（severity/status 字段与流转 API + 迁移）~~——✅ 已落地（2026-10-12，形态见 §3.3 落地注记）：独立四列 + `POST /ops/alerts/:id/handle` + 列表 `handleStatus` 过滤 + Alerts 页处理列/Dropdown/Modal/过滤器，迁移 0042（`migrateAlertHandlingColumns`：HasColumn/AddColumn 逐列 + 存量回填 pending + 缺表跳过幂等，回归 `TestAlertHandlingColumnsMigration_LegacyTableShape` 复刻老库形态）。
5. ~~告警中心迁移（Alerts 挂到风控分类）~~——✅ 已落地（2026-10-11）：`/risk/alerts` 复用 `./Ops/Alerts` 组件原地不动（数据面零改动）；旧 `/ops/alerts` 留 `hideInMenu` 重定向兼容书签，`/ops/maintenance` 重定向一步到位直指新路径。

## 7. 已知边界

- 本文为设计整理稿（2026-10-11 用户令）；已实施：§6 之 1+3+4+5（风控一级分类 + 风控策略页 + 风险事件工作流 + 告警中心迁移）；风控规则页仅落地 dry-run 接口（§6 之 2），结构化表单页未实施，首版形态以上表为准。
- 风控策略页边界：发布成功 ≠ 已生效——「生效对账」为 agent 自报（30s 轮询 + 重注册滞后，非强一致）；版本历史/节点对账接口失败时页面退化空表，不断主面。
- ~~dry-run 测试接口未设计~~——✅ 已解决（2026-10-12，§6 之 2 先行）：`POST /capture/gates/validate`，校验真值单一化为 agent apply 与 server dry-run 共用的 `config.ValidateGates`（见 capture-agent 简档 §8.12）。
- ~~dry-run 测试接口未设计~~——✅ 已落地 v1（2026-10-12，capture-agent 简档 §8.12）：`POST /capture/gates/validate` = lint + `config.ValidateGates`（与 agent apply 同一校验真值，runner pinning 测试锁镜像）；`/ops/capture`「校验」按钮就地呈现。结构化三段表单页（§6 之 2）仍未实施，落地时复用该端点作「测试」底座。
- 风险事件 severity/status 扩展涉及 alerts 模型加列——按仓库迁移红线须同 PR 配 goose 编号迁移（migrations.go + MinimumRequiredVersion + migrate_test 三处同步）。
- `/ops/capture` 旧入口去留为拍板项，倾向保留为「快速发布」入口（dry-run 校验按钮落地后其价值上升：JSON 直编 + 预检 + 发布的最短路径）。
