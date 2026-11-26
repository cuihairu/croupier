# 组合编辑器组件与组件模板逐项核对（SUI-046，2026-11-26）

任务：对照编辑器注册表（`web/src/pages/PageStudio/CompositeEditor/registry.tsx` + `components/builtin.tsx`）、数据库中的内置模板定义（`internal/api/component/generator.go` 生成、`internal/model/component_template.go` 落库）和发布渲染器（`web/src/components/PageRenderer/CompositeRenderer.tsx`），清点表单、表格、详情、动作等区块与模板；逐项记录属性、函数依赖、参数来源、输出与联动；标注预览和发布差异；用代表模板走一次插入、发布、执行，验证复制后的页面配置及模板更新提醒。

审核时源码 HEAD：`06040b3a2`。线上走查栈：deploy-host 平台栈（croupier-server :18780），scope `demo-game/demo`。证据目录：`docs/review/evidence/sui046-walkthrough-2026-11-26/`（9 份 API 产物 + README 索引）。本轮为审查任务，除新增 1 个回归测试外不改产品代码；发现列「发现」节，不在本轮擅动。

## 1. 编辑器注册表清点（9 组件）

`ComponentDef`（registry.tsx）：type/name/icon/category（`function`｜`basic`）/allowedParents/allowedChildren/propSchema（rjsf schema，函数域注入 fn/allFns）/scaffold/Preview/events。注册于 builtin.tsx，编译与发布映射见 `compiler/compile.ts` 与 `internal/service/contract_composite.go`（服务端生成链）。

| 组件 | category | 关键属性（propSchema） | 函数依赖 | 参数来源 | 编译产物（发布面） | 联动 |
| --- | --- | --- | --- | --- | --- | --- |
| fnTable 函数表格 | function | functionId、title、span、autoRun(默认 true)、columns（枚举=outputSchema properties）、rowActions、pagination（D3 服务端分页）、selection（D2 多选） | collection_query 能力函数 | 行操作 `{{row.<字段>}}`（行上下文）；工具栏/弹窗经 inputAssignments | `view=table` section（inline），key=函数 id（多实例 fid-2/fid-3） | rowActions→弹窗组；success 链 refreshNode；refreshOn 级联 |
| fnFields 字段卡 | function | functionId、span、autoRun | item_query 能力函数 | 行身份参数经目标弹窗的 rowAction params（落 context.form） | `view=fields` section（Descriptions 渲染） | 行操作打开详情弹窗时执行 |
| fnForm 函数表单 | function | functionId、span、display(inline/dialog)、onSuccess | 写能力函数（create/update/delete/action） | form.jsonSchema 从契约 inputSchema 生成；值经 context.form | `view=form` section；在 modal 内则 `display=dialog`+group | onSuccess→`{event:success, action:refreshNode}` |
| staticForm 常量表单 | basic | span、form.jsonSchema（编辑器设计期定义；默认 schema 为 env 枚举 prod/stage/dev） | 无（不执行、无审计面） | 表单值仅写入页面状态 | `view=form` + `static:true`，form.jsonSchema 原样落 spec | 值供 refreshOn/动作链消费 |
| button 按钮 | basic | title、btnStyle(default/primary/danger)、span、onClick | 无直接依赖 | onClick.target 指弹窗节点 id | 无独立 section——root 级 openModal 编译为**前一表格的 toolbarActions**；无动作或无可归属表格均=警告+**blocker（阻断保存）** | openModal→弹窗组 |
| modal 弹窗 | basic | title、width(narrow 420/medium 560/wide 720)；allowedChildren 仅 fnForm | 子表单的函数 | 子表单 context.form | 每弹窗一个 `display=dialog` section，group=声明 sectionKey 优先、否则 `modal-<id尾6>` 自动命名 | 被行操作/工具栏按钮/事件链指向 |
| container 分组容器 | basic | title、publishAs(flat/card)、span、onClick；allowedChildren 6 种函数/基础组件 | 子组件各自 | 子组件各自 | flat=平铺（不进 spec）；card=子区块 `display=card`+group+cardTitle | 卡片分组渲染 |
| tabs 页签容器 | basic | sectionKey（可选）、span；children 仅 container（每 container 一页，title=页签标签） | 页内组件各自 | 页内组件各自 | 平铺为 `display=tab`+group（`tabs-<id尾6>` 兜底）+tab 标签；发布端按 group/tab 聚合还原 Tabs | 页签切换为纯前端态 |
| text 文本 | basic | content、level(h2/h3/p)、span、onClick | 无 | — | **根级=编译 blocker（阻断保存）**；modal/tabs/card 内=静默跳过不进 spec | 无 |

函数依赖与参数来源的服务端真相：客户端编译产物（`CompiledSection`）只声明 functionId/view/display/group/events/rowActions 等；**发布 spec 的 table.rowSchema、form.jsonSchema/fields/submitButton、bindings 输入分配全部由服务端生成链（`contract_composite.go` BuildCompositePageSpec）从函数契约现算**（走查实证：请求体无 rowSchema/fields，落库 spec 有，见 `sui046-published-spec.json`）。

## 2. 数据库内置模板清点

- 生成器三族（`generator.go`，契约注册与 `POST /component-templates/regenerate` 触发，`UpsertBuiltin` 内容门控落库）：
  - `fn--<functionId>` 单函数：view 按 capability 推导（collection_query→table span24/autoRun；item_query→fields；default→form display:inline）。
  - `crud--<resource>` 资源管理：list 表格（行操作 查看详情/编辑/删除，行身份参数 `{{row.<idkey>}}`，删除 danger）+ 详情弹窗（get fields）+ 新建按钮→新建弹窗（create form，成功刷新表格）+ 编辑弹窗（update form）+ 删除确认弹窗（delete form）。行身份字段取 crudInputKey：优先 id/uid/uuid，否则排序后首个 property。
  - `query--<functionId>.list` 查询组合：fnForm 条件区（display:inline、autoRun:false）+ fnTable 结果区，条件区 refreshOnNode 指向表格。
- 线上清点（demo-game/demo，2026-11-26，pageSize=100 全量）：**72 条模板 = builtin 族 68 + 自建 4**。builtin 族：crud--×5（order/ban/activity/mail_task/player）、query--×8（mail_task/ban/activity/player/order/mail/leaderboard/inventory）、fn--×55（含 openapi-provider 接入的 `fn--players.player.*`×6）；digest 齐全（64 位 sha256）；自建 4 条为 `consts--demo-*`（seed-demo-constants，builtin=false）。**68/68 builtin 全部 stale:true**——见发现 F1（服务端 stale 在 postgres 全量误报）。params 字段 72/72 均为空（builtin 族与自建都未使用 U6 参数化）。采样核对 `crud--player` 树与 generator 逻辑同构（证据 `sui046-crud-player.json`、`sui046-templates-stale.json`）。
- 模板新鲜度**双机制**（两条独立链路，勿混淆）：
  1. **服务端 stale**（模板 vs 契约漂移）：List 时 `computeStaleKeys` 按当前 scope 契约在内存重算三族模板，Tree 不一致 → DTO `stale:true`；仅 builtin 参与；契约不可用返回空集不误标。前端提示「从契约重新生成」。**当前线上该机制全量误报，见发现 F1。**
  2. **客户端更新提醒**（页面快照 vs 模板库）：实例化时登记 `componentTemplates:[{key,digest}]` 随保存并入 PageSpec（仅来源标记，不参与发布校验）；编辑器打开页面时 `computeStaleTemplateNames` 比对快照 digest 与模板当前 digest，不一致提示「所用模板有新版本」；快照/模板缺 digest 跳过不误报；**模板已删除不算有新版本**（复制语义，不自动同步）。
- 自建模板（`Builtin:false`）：POST/PUT/DELETE 全套 CRUD；Create/Update 时模型层计算 digest；params（U6）经 `validateTemplateParams` 与树内 nodeId→prop 校验。

## 3. 发布渲染器清点（CompositeRenderer）

- 五种 view：table（列渲染 + rowActions + toolbar.actions + 服务端分页/多选）、fields（Descriptions）、form（SchemaFormRenderer + mergeStaticValues + externalErrors 回显）、toolbar（openDialog/runChain/confirmDanger）、actions。display：inline/dialog（同 group 聚合一弹窗）/tab（同 group+tab 聚合 Tabs）/card（同 group+cardTitle 聚合卡片）。
- 联动执行：区块输出落页面状态（output stateKey）；inputAssignments（page_state/literal，U8 缺省兜底 transform.default）；refreshOn 级联（U9 cascadePolicy clear/keep/pause）；visibleWhen 区块级条件显示（U10，false 不渲染、执行不变）。
- 执行门禁（服务端，`console_execute.go`）：已发布页与草稿预览共用同一执行核心——权限、executor bound、binding freshness、契约 permission、schema 校验、审批闸、危险操作 requireConfirm、幂等键（Idempotency-Key 头）。渲染端只交 bindingId + renderer context（form/row/selection/detail/pageState），**不交 functionId/routing/scope**。
- 危险弹窗重确认（D2）：danger 行操作打开的弹窗带确认基线；提交参数相对基线有改动时按最终值重新确认后才执行（compositeDangerReconfirm.test.tsx 覆盖 5 例）。

## 4. 预览 vs 发布差异（逐项标注）

| 面 | 编辑器预览（PreviewPane/PreviewNode） | 发布（CompositeRenderer + console execute） |
| --- | --- | --- |
| 数据 | Preview 组件吃 mock 数据（previewMockData 套件；无 schema 时 previewMockNoSchema 兜底） | 真实执行：bindings 经执行门禁下发 agent→SDK，行/详情为真实返回 |
| text | 画布渲染 Typography 标题/正文 | 根级根本存不进 spec（blocker）；嵌套的静默丢失（发现 F2） |
| modal | 画布内虚线卡片内联展开（拖入提示） | display=dialog 独立弹窗，由行操作/工具栏/事件链触发 |
| button | 画布渲染按钮本体 | 无本体——编译为表格工具栏按钮（toolbarActions） |
| container | 虚线框分组 | flat 不产生任何 spec 结构；card 产出卡片分组 |
| tabs | 真 antd Tabs 可切换（activeTab 编辑态轻量写 props，不进 undo/编译产物） | group+tab 聚合还原 Tabs，页签结构来自 spec |
| spec 形态 | 编译产物=CompositeSectionRequest（functionId 直引，合法仅此一处） | 落库 spec=CompositeSection（bindingId 引用 + 服务端契约富集），functionId 直引在发布 spec 非法 |

预览与发布的一致性由两条链路背书：预览执行走 `POST /pages/composite/preview/execute`（与发布同一生成链重建草稿 spec、同一执行门禁，不落库）；发布 payload 与落库 spec 的区块结构一致性由走查实证（§5）。

## 5. 代表模板走查：crud--player 插入 → 发布 → 执行 → 更新提醒

线上栈 demo-game/demo，全程 API 级取证（证据见 `docs/review/evidence/sui046-walkthrough-2026-11-26/`）：

1. **取模板**：`GET /api/v1/component-templates/crud--player` → 6 顶层节点树 + digest `f375e468…8244`（`sui046-crud-player.json`）。
2. **插入（客户端链，真实代码）**：`instantiateTemplateDetailed`（jest 走查测试，`crudTemplateWalkthrough.test.ts` 2/2 绿）——复制后的页面配置：6 节点 id 全量重分配；三个 modal 的模板 sectionKey 剥离（编译期自动重派）；行操作 targetSection / 新建按钮 onClick.target / 写表单 onSuccess.refreshNode.target 全部按 idMap 重映射**无悬空**；`{{row.id}}` 参数与五个 functionId 逐项保留。
3. **发布 payload（真实编译器）**：`compileTree` → blockers/warnings 空；表格 section autoRun、三个行操作+新建工具栏目标解析为自动弹窗组 `modal-<新id尾6>`；四个 dialog 区块（get/create/update/delete）组名与目标一一对应；写操作成功链编译为 `refreshNode→player.list`（`sui046-create-payload.json`）。
4. **发布落库**：`POST /api/v1/versioning/pages/composite` → 提案 `composite--sui046-crud-walkthrough`（demo 为人工接受策略）→ `POST /api/v1/proposals/.../accept-and-publish` → publishedVersion 1。落库 spec 5 sections 与请求一致（组名 modal-1tr792/795/797/799 原样），**服务端富集**出 table.rowSchema（10 字段）、form.jsonSchema/fields（含 widget/placeholder/提交取消按钮）、bindings 输入分配（`/id`←form `/id`）、delete `requireConfirm:true`；componentTemplates 快照原样落库（`sui046-published-spec.json`）。
5. **执行**：表格绑定 `POST /api/v1/console/pages/.../bindings/player.list/execute`（空 body）→ 200 + 10 行玩家；行操作链路绑定 player.get（`context.form={"id":"20001"}`，即 `{{row.id}}` 的解析来源）→ 200 + 玩家详情（`sui046-execute-list.json` / `sui046-execute-get.json`）。
6. **模板更新提醒**：自建模板 `sui046-freshness-tpl` Create 落 digest `af504827…`→PUT 更新后 `dcec4634…`（v1→v2）；用旧 digest 实例化的 mini 页发布后，页面快照 digest ≠ 模板当前 digest，即 `computeStaleTemplateNames` 的触发条件（判定逻辑由 `templateFreshness.test.ts` 7 例覆盖，含「删除不算新版本」「缺 digest 不误报」）。
7. **清理**：两页 + 自建模板经 API 删除并复核 404，线上无残留。

## 6. 发现

### F1 · P1 · 服务端 stale 检测在 postgres 上全量误报（raw 字节对比 vs jsonb 键序归一），regenerate 无法清除

- **现象**：线上 68/68 builtin 模板全部 `stale:true`（「契约已变化，需重新生成」），包括刚刚由本栈契约生成、内容无任何漂移的模板。sqlite/dev/CI（text 列）不复现——典型的「本地全绿、线上全红」形态。
- **根因**：`computeStaleKeys`（`internal/api/component/handler.go`）用**原始字节**比较 `string(item.Tree) != exp`：左边是 postgres **jsonb 列读回**的归一字节（jsonb 不保留键序/空白，按「键长→字典序」重排），右边是 Go `json.Marshal` 现算树（map 按字典序、结构体按字段序）。两者键序结构性不同——线上取证 `table.props` jsonb 读回键序为 `span,title,autoRun,columns,functionId,rowActions`，Go marshal 同对象为 `autoRun,columns,functionId,rowActions,span,title`——只要 props 键集在两种排序下不一致，比较恒假。
- **误报不可自愈**：`POST /component-templates/regenerate` 也清不掉——`UpsertBuiltin` 写入的树落 jsonb 再次归一，读回永远是 jsonb 序；且内容门控 `builtinContentEqual` 用 canonical 比较（`jsonEquivalent`）判定「内容没变，跳写」，存储字节依旧。真正的契约漂移与误报从此无法区分（告警疲劳：flag 永真即无信息量）。
- **证据**：`sui046-templates-stale.json`（72 条中 68 builtin 全 stale，含 `crud--player` 刚验证过同构的树）；列型取证 `information_schema.columns`：`tree jsonb`；对照模型自身的 canonical 基建（`model.ComputeTemplateDigest` sha256(canonical JSON)、`jsonEquivalent`）——同文件内两套比较语义不一致。
- **建议方向**：`computeStaleKeys` 改用与内容门控同一的 canonical 比较（`ComputeTemplateDigest(item.Tree) != ComputeTemplateDigest(exp)` 或 `jsonEquivalent`），同 PR 补一条 postgres 形态回归（手工 CREATE TABLE jsonb 列 + 注入 jsonb 归一字节，复刻存量库行为）。**本轮未改**。

### F2 · P2 · 嵌套 text 编译期静默丢弃，发布后内容无告知丢失

- **现象**：根级 text 产出编译 blocker（「无法无损映射为 PageSpec，已阻断保存」）+ 警告；但 modal/tabs/card 内的 text 子节点走 `continue` 静默跳过（compile.ts:287/326/368，`compiler.test.ts:714` 明确固化该行为「卡内 text 同弹窗/页签静默跳过」）。用户在弹窗/页签里放的说明文本保存成功、预览有（画布渲染）、发布后消失，无任何提示。
- **影响**：内容静默丢失，违反「边界诚实」交付原则的直观预期。
- **建议方向**：嵌套 text 至少补编译 warning（与根级一致的文案），或把 text 降为可发布形态（spec 无对应载体前先只加警告）。**本轮未改**。

### F3 · P2 · 手动更新内置模板后 builtin 标记永久丢失，stale 门控随之失效

- **现象链**：`PUT /component-templates/:key`（handler.go Update）把 `builtin:false` 写死进 updates → builtin 行被降级为 custom；此后 `UpsertBuiltin`（model）虽在契约注册时继续覆盖内容（existing.Builtin=false 跳过内容门控），但 `Save(&existing)` 不回写 Builtin 字段——**降级不可逆**：该模板从此脱离 `computeStaleKeys` 的 builtin-only stale 检测，契约再演化时不再有「需重新生成」提示，且用户的手工编辑会被下一次契约注册静默覆盖（内容被覆盖 + 标记仍是 custom）。
- **证据**：`internal/api/component/handler.go` Update 的 `updates["builtin"] = false`；`internal/model/component_template.go` UpsertBuiltin 只在 Create 分支用 `t.Builtin=true`，Save 分支沿用 existing 的 Builtin=false。
- **建议方向**：UpsertBuiltin 的 Save 分支对同 key 行恢复 `existing.Builtin = true`（契约再生即视为 builtin 回归），或 Update 拒绝对 builtin 行的降级（提示先复制为 custom）。**本轮未改**。

### F4 · 观察 · 线上执行偶发 dispatch 超时（环境性，非代码缺陷）

- player.get 首次执行 `context deadline exceeded`（schema 校验已过、门禁已过，死在下游 dispatch），同参数重试即 200。伴随观察到 croupier-agent-2 日志存在 provider TCP 每 10s 连接即断的重试环（`Provider TCP connection accepted/closed` 成对刷屏）。属栈上 session 健康问题，与组件/模板/渲染链无关；记录供运维侧关注，不动代码。

### O1 · 观察 · 客户端编译产物与服务端落库 spec 字段面不同属设计使然

- 客户端只发 view/display/group/events/rowActions（CompositeSectionRequest），rowSchema/form fields/bindings 分配由服务端从契约现算——两端契约不同名（`CompiledSection` vs `CompositeSection`），预览/发布一致性靠「同一生成链」保证而非字段直传。核对编译器与服务端生成链语义时须分别对照两侧代码，勿假设同构。

### O2 · 观察 · 模板实例化是纯复制语义（与文档声明一致）

- 实例化只登记来源快照（key+digest），不建立任何回写关系；模板删除不算「有新版本」，缺 digest 双向不误报。走查实证与 templateFreshness 单测一致。

## 7. 已知边界（本轮核对确认仍在）

- text 组件不参与发布（根级阻断、嵌套静默丢，F2）；button 无独立 spec 载体（依赖前置表格）。
- 模板 params（U6）线上 72/72 全部未使用（builtin 族与自建均无 params 字段），实例化参数化能力当前零使用面。
- 模板更新提醒是纯提示：不自动同步、不阻断编辑（拉取失败静默）。
- 服务端 stale 仅覆盖 builtin 族；custom 模板无契约漂移检测（合理——custom 不由契约生成）。当前该 flag 因 F1 全量永真，恢复可信前不作为漂移信号消费。

## 8. 门禁与验证记录

- 新增回归测试 `web/src/pages/PageStudio/CompositeEditor/__tests__/crudTemplateWalkthrough.test.ts`（SUI-046 锚：插入 remap 链 + 发布 payload 对齐），`pnpm exec jest` 2/2 绿。
- 线上走查六步证据：`docs/review/evidence/sui046-walkthrough-2026-11-26/`（README 索引）。
- `pnpm --dir web run tsc`、全量 jest、`go test ./internal/...`、`dashboard_vnext_guard.sh`、docs build：见 todo.md SUI-046 勾行（LOCAL VERIFIED 记录）。
