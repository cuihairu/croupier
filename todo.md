# Dashboard vNext 原子重构看板

更新时间：2026-08-08

本文件是唯一执行看板，不是架构说明、历史日志或验收报告。领域定义只以
`docs/architecture/` 下的权威文档为准。任务未通过自身验收前必须保持未勾选；
不得因为代码存在、局部检查通过或其他 agent 的口头报告勾选。

## 产品完成态

```text
SDK / OpenAPI 注册 FunctionContract
-> 聚合 CapabilitySemantics
-> 生成 PageProposal
-> 用户预览并显式发布
-> PublishedPageSpec 派生 Console 左侧菜单
-> Console 受 snapshot / stale / permission / approval / audit / OTel 约束执行
```

函数注册仅声明能力、JSON Schema、资源、风险、权限与执行方式。它不得声明
菜单、分类标签、标题、列、组件、mapping 或按钮位置。默认页面由平台生成；用户
仅在不满意时通过 Page Studio 编辑。旧模型不兼容、不转换，替代路径验收后物理删除。

## 协作规则

- 每个任务只能有一个 `Owner`；开始前将 `unassigned` 改为 agent 标识。
- `Depends` 中所有任务必须已勾选，才可开始；`[]` 可并行。
- 修改范围外的文件必须在交接说明中解释。不得编辑 protobuf 生成文件。
- 不得恢复旧 renderer、Formily、form-render、注册侧 UI、旧 PageSpec 或转换桥。
- 完成者必须执行任务中的 `Verify`，记录命令和摘要到 PR/交接消息，不写回本文件。
- 审核者复跑 `Verify` 后才可勾选。验收失败必须撤销勾选并新建最小修复任务。
- 测试/E2E 由专门 agent 维护；功能 agent 不得用“缺测试”阻塞可独立完成的功能任务。

## 任务模板

```md
- [ ] `ID` 标题
      Owner: unassigned
      Depends: []
      Scope: `允许修改的模块`
      Deliverable: 单一可观察产物。
      Forbidden: 不得做的事情。
      Verify: `唯一验收命令或明确浏览器步骤`。
      Handoff: 完成后应存在的接口、数据或后续任务前置。
```

## A. 基线与防回流

- [ ] `A-001` 建立 Dashboard vNext guard 基线
      Owner: unassigned
      Depends: []
      Scope: `scripts/dashboard_vnext_guard.sh`, guard 测试/CI 调用。
      Deliverable: guard 拒绝 Formily、form-render、旧 Function Form、注册侧 UI、旧页面 runtime 新增引用。
      Forbidden: 不得用 allowlist 掩盖业务代码的新命中。
      Verify: `bash "scripts/dashboard_vnext_guard.sh"` 返回 0；人为新增受禁标记时 guard 返回非 0。
      Handoff: 后续删除任务可依赖该 guard 防回流。

- [ ] `A-002` 建立旧模型删除清单
      Owner: unassigned
      Depends: []
      Scope: `docs/architecture/`, `scripts/`。
      Deliverable: 每个旧文件、路由、DTO、API、表/列有替代任务 ID 和删除前置条件。
      Forbidden: 不得把“废弃”当作删除完成，不得删除生产数据。
      Verify: 清单中每项均有替代任务、owner、E2E 前置条件；`bash "scripts/dashboard_vnext_guard.sh"` 通过。
      Handoff: `H-*` 删除任务逐项引用清单项。

## B. FunctionContract 与 CapabilitySemantics

- [ ] `B-001` 注册边界只接受 FunctionContract 字段
      Owner: unassigned
      Depends: []
      Scope: `internal/platform/registry/`, `internal/api/openapi/`, SDK descriptor adapter。
      Deliverable: 注册只接受稳定 key、schema、resource/operation/capability/execution/risk/permission/approval。
      Forbidden: 不得接受 UI、菜单、标题、labels、列、分页、mapping、组件树或任务/图表页面路径。
      Verify: 注册边界目标测试；携带任一 UI 字段返回结构化错误。
      Handoff: `B-002`、`C-*` 只读取持久化合同。

- [x] `B-002` 持久化合同保留 enabled=false
      Owner: unassigned
      Depends: []
      Scope: `internal/model/function_contract.go`, `internal/model/function_contract_model.go`。
      Deliverable: 禁用合同持久化后读取仍为 `false`，不被 ORM 默认值改写。
      Forbidden: 不得将 disabled 合同视为可执行。
      Verify: `go test ./internal/service -run TestContractService_BlockedIssueIsScopedToFunction -count=1`。
      Handoff: `C-008` 可将 disabled 生成隔离为 blocked issue。

- [ ] `B-003` OpenAPI REST 分类 collection/item/create/update/delete
      Owner: unassigned
      Depends: [`B-001`]
      Scope: `internal/platform/openapi/`。
      Deliverable: method/path/schema 仅生成 capability semantic 与诊断。
      Forbidden: 不得生成菜单、列、页面 placement；不得仅靠 operationId 猜 CRUD。
      Verify: `go test ./internal/platform/openapi -count=1`。
      Handoff: `B-006` 可使用 openapi_rest 来源。

- [ ] `B-004` SDK 显式 capability 贯通所有 SDK 描述符
      Owner: unassigned
      Depends: [`B-001`]
      Scope: `proto/`, 非生成 SDK 源码、descriptor adapter。
      Deliverable: Go/JS/Python/Java/C#/C++ 都能表达受控 capability 与 sync/task execution。
      Forbidden: 不得手改 protobuf 生成文件，不得静默丢弃不支持字段。
      Verify: 各 SDK descriptor/parity 测试与生成命令通过。
      Handoff: `B-006` 可使用 sdk_explicit 来源。

- [ ] `B-005` Resource Catalog 语义编辑不含 UI
      Owner: unassigned
      Depends: [`B-001`]
      Scope: `internal/api/resourcecatalog/`, `web/src/pages/ResourceCatalog/`。
      Deliverable: 管理员可编辑 identity、lifecycle binding、action subject、task/report semantic。
      Forbidden: 不得编辑导航、列、表单布局、按钮位置或 PageSpec。
      Verify: resource catalog 服务测试；浏览器保存语义后 Proposal 重建请求发出。
      Handoff: `B-007`、`C-001` 消费持久化语义。

- [x] `B-006` 自动语义记录来源优先级与冲突
      Owner: unassigned
      Depends: [`B-003`, `B-004`]
      Scope: `internal/dashboard/normalizer/provenance.go`, `internal/service/contract_service.go`。
      Deliverable: `platform_review > sdk_explicit > openapi_rest` 逐字段决定有效值，来源不一致保留 unresolved conflict。
      Forbidden: 不得以单一 source 字段吞掉字段冲突。
      Verify: `go test ./internal/dashboard/normalizer ./internal/service -run 'TestSemanticProvenanceTracker|TestContractService_RebuildResourceCapabilityRecordsSourceConflict' -count=1`。
      Handoff: `C-009` 将 unresolved conflict 降级 Proposal。

- [ ] `B-007` 平台审核解决语义冲突并触发重算
      Owner: unassigned
      Depends: [`B-005`, `B-006`]
      Scope: `internal/api/resourcecatalog/`, `web/src/pages/ResourceCatalog/`。
      Deliverable: 管理员选择冲突来源后更新 provenance/conflict resolution，并重建受影响 Proposal。
      Forbidden: 不得自动解决冲突，不得绕过版本与审计。
      Verify: resource catalog resolve-conflict 服务测试；解决后对应 Proposal 不再含该 conflict diagnostic。
      Handoff: `C-009` 可恢复 ready/basic 判定。

## C. Proposal 生成与质量

- [ ] `C-001` Proposal 幂等身份与持久化
      Owner: unassigned
      Depends: [`B-001`]
      Scope: `internal/model/page_proposal*`, `internal/service/contract_service.go`。
      Deliverable: proposalKey 固定为 `resource:<resourceKey>` 或 `<kind>:<functionId>`，pageKey 固定为 `resource--<resourceKey>` 或 `<kind>--<functionId>`。
      Forbidden: 不得从 title/labels/随机值生成 key。
      Verify: 合同服务 proposal 重建测试；相同输入重复重建的可比较摘要一致。
      Handoff: 所有 `C-*` 生成任务写入此模型。

- [x] `C-002` 只读 ResourcePage 生成
      Owner: unassigned
      Depends: [`B-006`, `C-001`]
      Scope: `internal/dashboard/generator/resource_generator.go`。
      Deliverable: collection query + identity 生成包含 list binding、identityKey、rowSchema、列和分页候选的 Resource Proposal。
      Forbidden: 不得要求 create/update/delete；无 collection 或 identity 时不得生成 ResourcePage。
      Verify: resource generator/contract service 测试覆盖只读资源为 `ready`。
      Handoff: `C-003` 至 `C-006` 叠加可选能力。

- [x] `C-003` Resource detail 生成
      Owner: unassigned
      Depends: [`C-002`]
      Scope: `internal/dashboard/generator/resource_generator.go`。
      Deliverable: item query 可仅由 row identity 安全填充时生成 detail binding；否则用 item schema 生成只读详情。
      Forbidden: 不得把整行 JSON 透传为 detail payload。
      Verify: generator 测试覆盖 item query 与 collection fallback 两种路径。
      Handoff: `E-002` 按 detail binding 渲染。

- [x] `C-004` Resource create/update 生成
      Owner: unassigned
      Depends: [`C-002`]
      Scope: `internal/dashboard/generator/resource_generator.go`。
      Deliverable: create/update 合同生成 FormPresentationSpec 与 binding；update identity 从表单剔除并由 row selector 填充。
      Forbidden: 不得让用户编辑未生效 identity。
      Verify: generator 测试验证 create/update form 与 selector。
      Handoff: `E-003` 可运行 create/update modal。

- [x] `C-005` Resource delete 治理生成
      Owner: unassigned
      Depends: [`C-002`]
      Scope: `internal/dashboard/generator/resource_generator.go`。
      Deliverable: delete ConfirmAction 与 binding 继承合同 permission/risk/approval，identity 来自 row selector。
      Forbidden: 不得硬编码 risk，不得省略 high/danger/approval 的确认。
      Verify: generator 测试验证 delete action 与 binding execution policy。
      Handoff: `E-003` 可安全运行 delete。

- [x] `C-006` Resource Action 生成
      Owner: unassigned
      Depends: [`C-002`, `B-005`]
      Scope: `internal/dashboard/generator/resource_generator.go`。
      Deliverable: `resource_item`、`resource_selection`、`none` 分别生成 row/batch/toolbar action。
      Forbidden: identity input 无法静态验证时不得内联，必须保留 standalone Operation Proposal。
      Verify: generator 测试覆盖三种 subject 与不安全降级。
      Handoff: `E-004` 运行三种 action。

- [x] `C-007` Operation Proposal 生成
      Owner: unassigned
      Depends: [`C-001`]
      Scope: `internal/dashboard/generator/generator.go`。
      Deliverable: 同步非 CRUD 合同生成 `basic` OperationPage、Schema form、结构化 ResultView。
      Forbidden: 不得用原始 JSON 结果面板，不得把 operation 塞进 ResourcePage。
      Verify: `go test ./internal/dashboard/generator -run TestGenerateForOperationCreatesBasicPage -count=1`。
      Handoff: `E-005` 运行 OperationPage。

- [x] `C-008` Operation 风险与审批确认生成
      Owner: unassigned
      Depends: [`C-007`]
      Scope: `internal/dashboard/generator/generator.go`。
      Deliverable: high/danger 或 approval.required 的 OperationPage 带 ConfirmAction 与合同权限/风险。
      Forbidden: 不得将“提交审批”显示为执行完成。
      Verify: generator 测试覆盖 high risk 与 approval required。
      Handoff: `E-005` 显示确认与审批等待态。

- [x] `C-009` Proposal quality 与 blocked 分流
      Owner: unassigned
      Depends: [`B-002`, `B-006`, `C-001`]
      Scope: `internal/dashboard/generator/`, `internal/service/contract_service.go`, `internal/model/page_proposal.go`。
      Deliverable: ready/basic/needs_review 仅用于 materialized Proposal；disabled/不可物化写 BlockedProposalIssue；unresolved semantic conflict 为 needs_review。
      Forbidden: 不得将 blocked/stale 写入 quality，不得保存 blocked 的 PageSpec。
      Verify: `go test ./internal/service -run 'TestContractService_BlockedIssueIsScopedToFunction|TestContractService_RebuildResourceCapabilityRecordsSourceConflict' -count=1`。
      Handoff: `F-001` 展示三队列。

- [x] `C-010` Task Proposal 生成
      Owner: unassigned
      Depends: [`C-001`, `B-005`]
      Scope: `internal/dashboard/generator/generator.go`。
      Deliverable: start/status/events/result/cancel semantic 完整时生成 TaskPage；缺任一必需语义为 needs_review。
      Forbidden: 无真实 retry semantic 时不得生成 retry。
      Verify: generator 测试覆盖完整与缺失 task semantic。
      Handoff: `E-006` 运行真实任务页面。

- [x] `C-011` Report Proposal 生成
      Owner: unassigned
      Depends: [`C-001`, `B-005`]
      Scope: `internal/dashboard/generator/generator.go`。
      Deliverable: dataset/dimension/metric semantic 完整时生成 ReportPage 图表/表格规格；缺失为 needs_review。
      Forbidden: 不得猜 `response.data.items`，不得在无 dataset 时 ready。
      Verify: generator 测试覆盖 dataset semantic 与缺失降级。
      Handoff: `E-007` 运行 ReportPage。

## D. PageSpec、发布、stale 与合并

- [x] `D-001` Canonical PageSpec DTO 前后端一致
      Owner: unassigned
      Depends: []
      Scope: `internal/dashboard/spec/`, `web/src/types/dashboard.ts`。
      Deliverable: Resource/Operation/Task/Report discriminated union、FormPresentation、navigation、view、action DTO 对齐。
      Forbidden: 不得新增组件树、React props 或未类型化 mapping DTO。
      Verify: Go selector/spec 测试与 web DTO 测试通过。
      Handoff: `D-002`、`E-*` 只消费该 DTO。

- [x] `D-002` Typed selector 静态校验
      Owner: unassigned
      Depends: [`D-001`, `B-001`]
      Scope: `internal/dashboard/spec/selector_ast.go`, publish validation, `web/src/types/dashboard.ts`。
      Deliverable: form/row/selection/detail/page_state/literal 的 JsonPointer selector 通过 schema/required/shape 校验。
      Forbidden: 不得接受 JSONPath、任意 transform、裸 row 或未定义 source。
      Verify: `go test ./internal/dashboard/spec -count=1` 与共享 selector vector 测试通过。
      Handoff: `C-002` 至 `C-011` 和 `G-003` 使用校验器。

- [ ] `D-003` 发布冻结 BindingContractSnapshot
      Owner: unassigned
      Depends: [`D-001`, `C-001`]
      Scope: `internal/service/proposal_service.go`, `internal/model/page_spec.go`。
      Deliverable: 发布冻结 function version/schema digest/risk/permission/approval/execution/renderer version。
      Forbidden: 不得从运行时 registry 补全发布事实。
      Verify: proposal publish 服务测试验证 snapshot 内容。
      Handoff: `D-004`、`G-003` 使用同一 snapshot。

- [ ] `D-004` stale 检测与执行拒绝
      Owner: unassigned
      Depends: [`D-003`, `D-002`]
      Scope: `internal/dashboard/freshness/`, `internal/api/console/`。
      Deliverable: 合同 schema/governance/execution 变化标记 stale；菜单可显示诊断，execute 被拒绝。
      Forbidden: 不得静默切换到最新合同。
      Verify: freshness 与 console stale execute 服务测试。
      Handoff: `F-003` 展示契约变更队列。

- [ ] `D-005` 三方合并安全集
      Owner: unassigned
      Depends: [`D-004`]
      Scope: `internal/dashboard/merge/`, `internal/service/versioning/`。
      Deliverable: 自动合并仅限列显隐/排序、展示 label/help、widget hint、导航/分类 labels、icon/order。
      Forbidden: bindings/selectors/permissions/risk/identity/execution/approval/confirmation 必须人工确认。
      Verify: merge 服务测试覆盖 safe 与 conflict 字段。
      Handoff: `F-004` 可请求自动合并。

- [ ] `D-006` 三方合并人工决策与回滚
      Owner: unassigned
      Depends: [`D-005`]
      Scope: `internal/service/versioning/`, `internal/api/page/`。
      Deliverable: 逐冲突选择、草稿回滚、发布回滚、重新发布均使用 revision 防并发覆盖。
      Forbidden: 不得自动确认冲突或覆盖 PublishedPageSpec。
      Verify: versioning 服务测试覆盖 manual merge、draft rollback、publish rollback。
      Handoff: `F-004` 提供 UI 操作入口。

## E. 唯一前端运行时

- [ ] `E-001` 唯一 SchemaFormRenderer
      Owner: unassigned
      Depends: [`D-001`]
      Scope: `web/src/components/SchemaFormRenderer/`, `web/package.json`。
      Deliverable: `@rjsf/antd + @rjsf/validator-ajv8` 通过 FormPresentationSpec 派生只读展示配置。
      Forbidden: 不得引入 Formily/form-render/自研 ProForm field factory；不得持久化 rjsf uiSchema。
      Verify: SchemaFormRenderer Jest 测试覆盖 array/object/enum/format/default/嵌套；`rg` 无第二 runtime 命中。
      Handoff: `E-003` 至 `E-007` 复用此组件。

- [ ] `E-002` Resource list/detail runtime
      Owner: unassigned
      Depends: [`C-002`, `C-003`, `D-002`, `E-001`]
      Scope: `web/src/components/PageRenderer/ResourcePageRenderer.tsx`。
      Deliverable: ProTable query/filter/pagination/empty/error/refresh 与 ProDescriptions detail 只消费 page state patch。
      Forbidden: 不得读取 lastResult 或整行隐式数据总线。
      Verify: Resource renderer 单测和浏览器 POC：只读资源 list/detail/pagination。
      Handoff: `E-003`、`E-004` 在同一页面状态模型上扩展。

- [ ] `E-003` Resource create/update/delete runtime
      Owner: unassigned
      Depends: [`C-004`, `C-005`, `E-002`]
      Scope: `web/src/components/PageRenderer/ResourcePageRenderer.tsx`。
      Deliverable: create/update 使用 SchemaFormRenderer；delete/high-risk 使用确认；全部经 binding execute。
      Forbidden: 不得自建第二表单实现，不得前端补 functionId/target/scope。
      Verify: 浏览器 POC：CRUD 页面 create/update/delete 均成功并刷新列表。
      Handoff: `I-002` 可删除旧资源运行路径。

- [ ] `E-004` Resource row/batch/toolbar action runtime
      Owner: unassigned
      Depends: [`C-006`, `E-002`]
      Scope: `web/src/components/PageRenderer/ResourcePageRenderer.tsx`。
      Deliverable: 三类 action 使用 typed row/selection/form context 调 binding execute。
      Forbidden: 不得把整行/selection 原样传给后端。
      Verify: 浏览器 POC：row、batch、toolbar 各执行一次；不安全 action 不出现。
      Handoff: `G-003` 可审计 action context。

- [ ] `E-005` Operation 与 Approval runtime
      Owner: unassigned
      Depends: [`C-007`, `C-008`, `E-001`]
      Scope: `web/src/components/PageRenderer/OperationPageRenderer.tsx`。
      Deliverable: 表单、确认、结构化结果、pending/approved/rejected/expired 审批状态与 continuation。
      Forbidden: 不得把 approvalId 当作操作成功，不得显示原始 JSON 作为正式结果。
      Verify: 浏览器 POC：`mail.send` 与 high-risk approval operation。
      Handoff: `G-003` 接管最终执行。

- [ ] `E-006` Task runtime
      Owner: unassigned
      Depends: [`C-010`, `E-001`]
      Scope: `web/src/components/PageRenderer/TaskPageRenderer.tsx`。
      Deliverable: 真实 start/status/events/result/cancel；仅有显式 retry semantic 才显示 retry。
      Forbidden: 不得轮询不存在的 binding，不得提供假 retry。
      Verify: 浏览器 POC：任务启动、事件、失败/完成、取消与结果。
      Handoff: `G-003` 记录 taskId/traceId。

- [ ] `E-007` Report runtime
      Owner: unassigned
      Depends: [`C-011`, `E-001`]
      Scope: `web/src/components/PageRenderer/ReportPageRenderer.tsx`。
      Deliverable: QueryForm、真实 dataset table、line/bar/pie/area chart、空态/错误态和导出。
      Forbidden: 不得猜响应字段，不得在缺 dataset 时渲染图表。
      Verify: 浏览器 POC：报表查询、图表、表格、空态与数据错误。
      Handoff: `I-002` 可删除旧报表运行路径。

## F. Page Studio 与 Resource Catalog 产品路径

- [ ] `F-001` Proposal Inbox 三队列
      Owner: unassigned
      Depends: [`C-009`, `D-004`]
      Scope: `internal/service/proposal_service.go`, `internal/api/`, `web/src/components/ProposalInbox/`。
      Deliverable: publishable(ready/basic)、needs_review、blocked issue、contract changes 分队列返回和展示。
      Forbidden: 不得把 stale/blocked 混进 quality 枚举，不得前端自行推断队列。
      Verify: inbox 服务测试；浏览器 POC 显示四类记录与计数。
      Handoff: `F-002`、`F-004` 使用队列记录。

- [ ] `F-002` Proposal 预览、接受与直接发布
      Owner: unassigned
      Depends: [`F-001`, `D-003`]
      Scope: `internal/service/proposal_service.go`, `web/src/components/ProposalInbox/`。
      Deliverable: ready/basic 可预览、接受、直接发布；发布后刷新菜单并进入 Console 页面。
      Forbidden: needs_review/blocked 不得直接发布。
      Verify: 浏览器 E2E：`mail.send -> preview -> publish -> Console`。
      Handoff: `G-001` 消费发布快照。

- [ ] `F-003` Resource Catalog 解释生成原因
      Owner: unassigned
      Depends: [`B-005`, `C-009`]
      Scope: `web/src/pages/ResourceCatalog/`。
      Deliverable: 显示资源函数、来源、置信度、冲突、诊断、版本、Proposal 入口和受影响页面。
      Forbidden: 不得要求用户查看原始 JSON 才能理解为何不能生成。
      Verify: 浏览器 POC：缺 identity、来源冲突、blocked issue 三种状态。
      Handoff: `B-007`、`F-001` 的用户处理入口完整。

- [ ] `F-004` Page Studio 语义化编辑与变更处理
      Owner: unassigned
      Depends: [`D-006`, `F-001`, `E-001`]
      Scope: `web/src/pages/PageStudio/`, `web/src/components/PageEditor/`。
      Deliverable: 按 PageType 编辑导航、视图、展示字段、form presentation、action 与治理字段；显示 diff/merge/rollback/re-publish。
      Forbidden: 正常路径不得展示 PageSpec JSON、mapping JSON 或第二表单 schema。
      Verify: 浏览器 E2E：修改列 label，合同变化后自动合并/人工决策并重新发布。
      Handoff: `I-003` 可删除旧 PageSchemaEditor。

## G. Console 执行、菜单、审计与 OTel

- [ ] `G-001` PublishedPageSpec 派生 Console 左侧菜单
      Owner: unassigned
      Depends: [`D-003`, `F-002`]
      Scope: `internal/api/console/`, `web/src/app.tsx`, `web/src/utils/consoleMenu.ts`。
      Deliverable: 左侧菜单只读取 active published pages；路由为 `/console/:categoryKey/:pageKey`。
      Forbidden: 不得从 SDK/OpenAPI/Proposal/静态 locale 推断菜单。
      Verify: 浏览器 E2E：发布/取消发布/重新发布后菜单立即刷新。
      Handoff: `G-002` 只排序已发布菜单。

- [ ] `G-002` 菜单 scope、分类与本地化规则
      Owner: unassigned
      Depends: [`G-001`]
      Scope: `internal/api/console/`, `web/src/utils/consoleMenu.ts`。
      Deliverable: scope 切换失效旧菜单；同 category key labels 发布校验一致；category order 为已发布页面最小 order；显示当前 locale 后退系统默认语言。
      Forbidden: 运行时不得推断分类，不得写静态翻译字典。
      Verify: menu 服务测试和浏览器 POC：两 scope、同分类、locale fallback。
      Handoff: Console 导航规则冻结。

- [ ] `G-003` 受控 binding execute 与可观测性
      Owner: unassigned
      Depends: [`D-002`, `D-004`, `E-003`, `E-004`, `E-005`, `E-006`, `E-007`]
      Scope: `internal/api/console/`, `internal/api/approval/`, telemetry/audit。
      Deliverable: execute 校验 binding/snapshot/stale/permission/risk/approval/task dispatch；audit/span 记录 scope/page/binding/function/semantic digest/proposal version/result/task/approval/trace。
      Forbidden: 浏览器不得提交 functionId/target/game/env；不得记录敏感 payload。
      Verify: 服务测试伪造 binding/function/target/scope 均失败；OTel collector E2E 可关联 audit 与 trace。
      Handoff: `J-001` 最终安全验收。

## H. 旧路径物理清理

- [ ] `H-001` 删除 Formily 与 form-render 依赖和源文件
      Owner: unassigned
      Depends: [`A-001`, `E-001`]
      Scope: `web/package.json`, `web/src/` 旧表单文件。
      Deliverable: 无 Formily/form-render runtime、类型、文案、lockfile 依赖。
      Forbidden: 不得保留 adapter/compatibility wrapper。
      Verify: `rg "@formily|components/formily|Formily|formily|form-render|FormRender" "web/src" "web/package.json"` 无命中；web build 通过。
      Handoff: guard 防止回流。

- [ ] `H-002` 删除旧 Page renderer 与旧运行 registry
      Owner: unassigned
      Depends: [`A-002`, `E-002`, `E-003`, `E-004`, `E-005`, `E-006`, `E-007`]
      Scope: 旧 renderer、旧运行 registry、旧页面路由。
      Deliverable: Console 仅使用 vNext PageRenderer。
      Forbidden: 不得保留 fallback renderer 或 feature flag 双路径。
      Verify: 全量浏览器 E2E 与 `bash "scripts/dashboard_vnext_guard.sh"` 通过。
      Handoff: `H-004` 可删除旧 API/DTO。

- [ ] `H-003` 删除旧 Page schema validator/editor
      Owner: unassigned
      Depends: [`A-002`, `D-002`, `F-004`]
      Scope: 旧 PageSchemaEditor、旧 validator、JSON page editor API。
      Deliverable: 页面编辑只使用强类型 DTO 与语义面板。
      Forbidden: 不得保留 JSON 编辑作为正常或隐藏兼容路径。
      Verify: 浏览器 E2E 编辑/发布通过；`rg` 无旧 editor/validator 引用。
      Handoff: `H-004` 可删除旧 API/DTO。

- [ ] `H-004` 删除旧注册 UI 扩展、页面 API 与 DTO
      Owner: unassigned
      Depends: [`B-001`, `H-002`, `H-003`]
      Scope: SDK/OpenAPI 页面扩展、旧页面 API/DTO、旧 workspace/object-page 配置。
      Deliverable: 注册与运行路径只剩 vNext 合同、语义、Proposal、PublishedPageSpec。
      Forbidden: 不得提供数据转换桥或 compatibility endpoint。
      Verify: guard 通过；SDK parity、服务集成和浏览器 E2E 不依赖旧接口。
      Handoff: `H-005` 可清理旧表/数据。

- [ ] `H-005` 备份后删除旧页面表/列与历史数据
      Owner: unassigned
      Depends: [`H-004`]
      Scope: GORM migration cleanup、运维备份流程。
      Deliverable: 版本化清理函数通过 `db.Migrator().DropColumn/DropTable` 删除已替代结构。
      Forbidden: 未取得单独明确确认不得执行生产数据删除；不得用 AutoMigrate/raw SQL 删除。
      Verify: 备份校验记录、迁移测试、全量 E2E 和 deployment dry-run。
      Handoff: `J-001` 可声明无旧模型依赖。

## I. 跨链路浏览器验收

- [ ] `I-001` SDK Operation 直接发布链路
      Owner: unassigned
      Depends: [`C-007`, `C-008`, `D-003`, `E-005`, `F-002`, `G-001`]
      Scope: Playwright/server integration fixtures。
      Deliverable: `mail.send -> basic Proposal -> preview -> publish -> Console -> structured result`。
      Forbidden: 不得通过 mock page 或手工插库绕过注册。
      Verify: 命名 Playwright E2E 和 server integration test 均通过。
      Handoff: 证明最小产品主链路。

- [ ] `I-002` OpenAPI CRUD 直接发布链路
      Owner: unassigned
      Depends: [`B-003`, `C-002`, `C-003`, `C-004`, `C-005`, `C-006`, `D-003`, `E-004`, `F-002`, `G-001`]
      Scope: Playwright/server integration fixtures。
      Deliverable: OpenAPI `/players` + provider binding -> ready Resource Proposal -> publish -> list/detail/CRUD/row action。
      Forbidden: 不得用页面特例、旧对象页或预置 PageSpec。
      Verify: 命名 Playwright E2E 和 server integration test 均通过。
      Handoff: 证明游戏 CRUD 主路径。

- [ ] `I-003` 合同变化到重新发布链路
      Owner: unassigned
      Depends: [`D-004`, `D-005`, `D-006`, `F-004`, `G-003`]
      Scope: Playwright/server integration fixtures。
      Deliverable: schema/risk/identity 变化 -> stale -> execute 拒绝 -> diff/merge -> republish -> execute 恢复。
      Forbidden: 不得自动覆盖 draft/published，不得允许 stale execute。
      Verify: 命名 Playwright E2E 和 server integration test 均通过。
      Handoff: 证明发布快照与变更治理闭环。

## J. 最终门禁

- [ ] `J-001` vNext 发布候选验收
      Owner: unassigned
      Depends: [`A-001`, `A-002`, `B-001`, `B-002`, `B-003`, `B-004`, `B-005`, `B-006`, `B-007`, `C-001`, `C-002`, `C-003`, `C-004`, `C-005`, `C-006`, `C-007`, `C-008`, `C-009`, `C-010`, `C-011`, `D-001`, `D-002`, `D-003`, `D-004`, `D-005`, `D-006`, `E-001`, `E-002`, `E-003`, `E-004`, `E-005`, `E-006`, `E-007`, `F-001`, `F-002`, `F-003`, `F-004`, `G-001`, `G-002`, `G-003`, `H-001`, `H-002`, `H-003`, `H-004`, `I-001`, `I-002`, `I-003`]
      Scope: CI、部署验证、SDK parity、docs build。
      Deliverable: 所有产品链路、质量门禁和物理清理任务完成，可声明 vNext 重构完成。
      Forbidden: 不得将历史记录、单测通过或未部署构建当最终验收。
      Verify: Go/web/docs/SDK/Playwright/OTel collector/deployment 验收矩阵全部绿，且 `H-005` 的生产删除另有明确确认。
      Handoff: 产出正式发布候选审计报告。
