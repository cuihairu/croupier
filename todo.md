# Dashboard vNext 原子重构看板

更新时间：2026-08-13

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

- [x] `A-001` 建立 Dashboard vNext guard 基线
      Owner: root
      Depends: []
      Scope: `scripts/dashboard_vnext_guard.sh`, guard 测试/CI 调用。
      Deliverable: guard 拒绝 Formily、form-render、旧 Function Form、注册侧 UI、旧页面 runtime 新增引用。
      Forbidden: 不得用 allowlist 掩盖业务代码的新命中。
      Verify: `bash "scripts/dashboard_vnext_guard.sh"` 返回 0；人为新增受禁标记时 guard 返回非 0。
      Handoff: 后续删除任务可依赖该 guard 防回流。

- [x] `A-002` 建立旧模型删除清单
      Owner: root
      Depends: []
      Scope: `docs/architecture/`, `scripts/`。
      Deliverable: 每个旧文件、路由、DTO、API、表/列有替代任务 ID 和删除前置条件。
      Forbidden: 不得把”废弃”当作删除完成，不得删除生产数据。
      Verify: 清单中每项均有替代任务、owner、E2E 前置条件；`bash “scripts/dashboard_vnext_guard.sh”` 通过。
      Handoff: `H-*` 删除任务逐项引用清单项。

## B. FunctionContract 与 CapabilitySemantics

- [x] `B-001` 注册边界只接受 FunctionContract 字段
      Owner: root
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

- [x] `B-003` OpenAPI REST 分类 collection/item/create/update/delete
      Owner: root
      Depends: [`B-001`]
      Scope: `internal/platform/openapi/`。
      Deliverable: method/path/schema 仅生成 capability semantic 与诊断。
      Forbidden: 不得生成菜单、列、页面 placement；不得仅靠 operationId 猜 CRUD。
      Verify: `go test ./internal/platform/openapi -count=1`。
      Handoff: `B-006` 可使用 openapi_rest 来源。

- [x] `B-004` SDK 显式 capability 贯通所有 SDK 描述符
      Owner: root
      Depends: [`B-001`]
      Scope: `proto/`, 非生成 SDK 源码、descriptor adapter。
      Deliverable: Go/JS/Python/Java/C#/C++ 都能表达受控 capability 与 sync/task execution。
      Forbidden: 不得手改 protobuf 生成文件，不得静默丢弃不支持字段。
      Verify: 各 SDK descriptor/parity 测试与生成命令通过。
      Handoff: `B-006` 可使用 sdk_explicit 来源。

- [x] `B-005` Resource Catalog 语义编辑不含 UI
      Owner: root
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

- [x] `B-007` 平台审核解决语义冲突并触发重算
      Owner: root
      Depends: [`B-005`, `B-006`]
      Scope: `internal/api/resourcecatalog/`, `web/src/pages/ResourceCatalog/`。
      Deliverable: 管理员选择冲突来源后更新 provenance/conflict resolution，并重建受影响 Proposal。
      Forbidden: 不得自动解决冲突，不得绕过版本与审计。
      Verify: resource catalog resolve-conflict 服务测试；解决后对应 Proposal 不再含该 conflict diagnostic。
      Handoff: `C-009` 可恢复 ready/basic 判定。

## C. Proposal 生成与质量

- [x] `C-001` Proposal 幂等身份与持久化
      Owner: root
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

- [x] `D-003` 发布冻结 BindingContractSnapshot
      Owner: unassigned
      Depends: [`D-001`, `C-001`]
      Scope: `internal/service/proposal_service.go`, `internal/model/page_spec.go`。
      Deliverable: 发布冻结 function version/schema digest/risk/permission/approval/execution/renderer version。
      Forbidden: 不得从运行时 registry 补全发布事实。
      Verify: proposal publish 服务测试验证 snapshot 内容。
      Handoff: `D-004`、`G-003` 使用同一 snapshot。

- [x] `D-004` stale 检测与执行拒绝
      Owner: unassigned
      Depends: [`D-003`, `D-002`]
      Scope: `internal/dashboard/freshness/`, `internal/api/console/`。
      Deliverable: 合同 schema/governance/execution 变化标记 stale；菜单可显示诊断，execute 被拒绝。
      Forbidden: 不得静默切换到最新合同。
      Verify: freshness 与 console stale execute 服务测试。
      Handoff: `F-003` 展示契约变更队列。

- [x] `D-005` 三方合并安全集
      Owner: unassigned
      Depends: [`D-004`]
      Scope: `internal/dashboard/merge/`, `internal/service/versioning/`。
      Deliverable: 自动合并仅限列显隐/排序、展示 label/help、widget hint、导航/分类 labels、icon/order。
      Forbidden: bindings/selectors/permissions/risk/identity/execution/approval/confirmation 必须人工确认。
      Verify: merge 服务测试覆盖 safe 与 conflict 字段。
      Handoff: `F-004` 可请求自动合并。

- [x] `D-006` 三方合并人工决策与回滚
      Owner: unassigned
      Depends: [`D-005`]
      Scope: `internal/service/versioning/`, `internal/api/page/`。
      Deliverable: 逐冲突选择、草稿回滚、发布回滚、重新发布均使用 revision 防并发覆盖。
      Forbidden: 不得自动确认冲突或覆盖 PublishedPageSpec。
      Verify: versioning 服务测试覆盖 manual merge、draft rollback、publish rollback。
      Handoff: `F-004` 提供 UI 操作入口。

## E. 唯一前端运行时

- [x] `E-001` 唯一 SchemaFormRenderer
      Owner: unassigned
      Depends: [`D-001`]
      Scope: `web/src/components/SchemaFormRenderer/`, `web/package.json`。
      Deliverable: `@rjsf/antd + @rjsf/validator-ajv8` 通过 FormPresentationSpec 派生只读展示配置。
      Forbidden: 不得引入 Formily/form-render/自研 ProForm field factory；不得持久化 rjsf uiSchema。
      Verify: SchemaFormRenderer Jest 测试覆盖 array/object/enum/format/default/嵌套；`rg` 无第二 runtime 命中。
      Handoff: `E-003` 至 `E-007` 复用此组件。

- [x] `E-002` Resource list/detail runtime
      Owner: root
      Depends: [`C-002`, `C-003`, `D-002`, `E-001`]
      Scope: `web/src/components/PageRenderer/ResourcePageRenderer.tsx`。
      Deliverable: ProTable query/filter/pagination/empty/error/refresh 与 ProDescriptions detail 只消费 page state patch。
      Forbidden: 不得读取 lastResult 或整行隐式数据总线。
      Verify: Resource renderer 单测和浏览器 POC：只读资源 list/detail/pagination。
      Handoff: `E-003`、`E-004` 在同一页面状态模型上扩展。

- [x] `E-003` Resource create/update/delete runtime
      Owner: root
      Depends: [`C-004`, `C-005`, `E-002`]
      Scope: `web/src/components/PageRenderer/ResourcePageRenderer.tsx`。
      Deliverable: create/update 使用 SchemaFormRenderer；delete/high-risk 使用确认；全部经 binding execute。
      Forbidden: 不得自建第二表单实现，不得前端补 functionId/target/scope。
      Verify: 浏览器 POC：CRUD 页面 create/update/delete 均成功并刷新列表。
      Handoff: `I-002` 可删除旧资源运行路径。

- [x] `E-004` Resource row/batch/toolbar action runtime
      Owner: root
      Depends: [`C-006`, `E-002`]
      Scope: `web/src/components/PageRenderer/ResourcePageRenderer.tsx`。
      Deliverable: 三类 action 使用 typed row/selection/form context 调 binding execute。
      Forbidden: 不得把整行/selection 原样传给后端。
      Verify: 浏览器 POC：row、batch、toolbar 各执行一次；不安全 action 不出现。
      Handoff: `G-003` 可审计 action context。

- [x] `E-005` Operation 与 Approval runtime
      Owner: root
      Depends: [`C-007`, `C-008`, `E-001`]
      Scope: `web/src/components/PageRenderer/OperationPageRenderer.tsx`。
      Deliverable: 表单、确认、结构化结果、pending/approved/rejected/expired 审批状态与 continuation。
      Forbidden: 不得把 approvalId 当作操作成功，不得显示原始 JSON 作为正式结果。
      Verify: 浏览器 POC：`mail.send` 与 high-risk approval operation。
      Handoff: `G-003` 接管最终执行。

- [x] `E-006` Task runtime
      Owner: root
      Depends: [`C-010`, `E-001`]
      Scope: `web/src/components/PageRenderer/TaskPageRenderer.tsx`。
      Deliverable: 真实 start/status/events/result/cancel；仅有显式 retry semantic 才显示 retry。
      Forbidden: 不得轮询不存在的 binding，不得提供假 retry。
      Verify: 浏览器 POC：任务启动、事件、失败/完成、取消与结果。
      Handoff: `G-003` 记录 taskId/traceId。

- [x] `E-007` Report runtime
      Owner: root
      Depends: [`C-011`, `E-001`]
      Scope: `web/src/components/PageRenderer/ReportPageRenderer.tsx`。
      Deliverable: QueryForm、真实 dataset table、line/bar/pie/area chart、空态/错误态和导出。
      Forbidden: 不得猜响应字段，不得在缺 dataset 时渲染图表。
      Verify: 浏览器 POC：报表查询、图表、表格、空态与数据错误。
      Handoff: `I-002` 可删除旧报表运行路径。

## F. Page Studio 与 Resource Catalog 产品路径

- [x] `F-001` Proposal Inbox 三队列
      Owner: root
      Depends: [`C-009`, `D-004`]
      Scope: `internal/service/proposal_service.go`, `internal/api/`, `web/src/components/ProposalInbox/`。
      Deliverable: publishable(ready/basic)、needs_review、blocked issue、contract changes 分队列返回和展示。
      Forbidden: 不得把 stale/blocked 混进 quality 枚举，不得前端自行推断队列。
      Verify: inbox 服务测试；浏览器 POC 显示四类记录与计数。
      Handoff: `F-002`、`F-004` 使用队列记录。

- [x] `F-002` Proposal 预览、接受与直接发布
      Owner: root
      Depends: [`F-001`, `D-003`]
      Scope: `internal/service/proposal_service.go`, `web/src/components/ProposalInbox/`。
      Deliverable: ready/basic 可预览、接受、直接发布；发布后刷新菜单并进入 Console 页面。
      Forbidden: needs_review/blocked 不得直接发布。
      Verify: 浏览器 E2E：`mail.send -> preview -> publish -> Console`。
      Handoff: `G-001` 消费发布快照。

- [x] `F-003` Resource Catalog 解释生成原因
      Owner: root
      Depends: [`B-005`, `C-009`]
      Scope: `web/src/pages/ResourceCatalog/`。
      Deliverable: 显示资源函数、来源、置信度、冲突、诊断、版本、Proposal 入口和受影响页面。
      Forbidden: 不得要求用户查看原始 JSON 才能理解为何不能生成。
      Verify: 浏览器 POC：缺 identity、来源冲突、blocked issue 三种状态。
      Handoff: `B-007`、`F-001` 的用户处理入口完整。

- [x] `F-004` Page Studio 语义化编辑与变更处理
      Owner: root
      Depends: [`D-006`, `F-001`, `E-001`]
      Scope: `web/src/pages/PageStudio/`, `web/src/components/PageEditor/`。
      Deliverable: 按 PageType 编辑导航、视图、展示字段、form presentation、action 与治理字段；显示 diff/merge/rollback/re-publish。
      Forbidden: 正常路径不得展示 PageSpec JSON、mapping JSON 或第二表单 schema。
      Verify: 浏览器 E2E：修改列 label，合同变化后自动合并/人工决策并重新发布。
      Handoff: `I-003` 可删除旧 PageSchemaEditor。

## G. Console 执行、菜单、审计与 OTel

- [x] `G-001` PublishedPageSpec 派生 Console 左侧菜单
      Owner: root
      Depends: [`D-003`, `F-002`]
      Scope: `internal/api/console/`, `web/src/app.tsx`, `web/src/utils/consoleMenu.ts`。
      Deliverable: 左侧菜单只读取 active published pages；路由为 `/console/:categoryKey/:pageKey`。
      Forbidden: 不得从 SDK/OpenAPI/Proposal/静态 locale 推断菜单。
      Verify: 浏览器 E2E：发布/取消发布/重新发布后菜单立即刷新。
      Handoff: `G-002` 只排序已发布菜单。

- [x] `G-002` 菜单 scope、分类与本地化规则
      Owner: root
      Depends: [`G-001`]
      Scope: `internal/api/console/`, `web/src/utils/consoleMenu.ts`。
      Deliverable: scope 切换失效旧菜单；同 category key labels 发布校验一致；category order 为已发布页面最小 order；显示当前 locale 后退系统默认语言。
      Forbidden: 运行时不得推断分类，不得写静态翻译字典。
      Verify: menu 服务测试和浏览器 POC：两 scope、同分类、locale fallback。
      Handoff: Console 导航规则冻结。

- [x] `G-003` 受控 binding execute 与可观测性
      Owner: root
      Depends: [`D-002`, `D-004`, `E-003`, `E-004`, `E-005`, `E-006`, `E-007`]
      Scope: `internal/api/console/`, `internal/api/approval/`, telemetry/audit。
      Deliverable: execute 校验 binding/snapshot/stale/permission/risk/approval/task dispatch；audit/span 记录 scope/page/binding/function/semantic digest/proposal version/result/task/approval/trace。
      Forbidden: 浏览器不得提交 functionId/target/game/env；不得记录敏感 payload。
      Verify: 服务测试伪造 binding/function/target/scope 均失败；OTel collector E2E 可关联 audit 与 trace。
      Handoff: `J-001` 最终安全验收。

## H. 旧路径物理清理

- [x] `H-001` 删除 Formily 与 form-render 依赖和源文件
      Owner: root
      Depends: [`A-001`, `E-001`]
      Scope: `web/package.json`, `web/src/` 旧表单文件。
      Deliverable: 无 Formily/form-render runtime、类型、文案、lockfile 依赖。
      Forbidden: 不得保留 adapter/compatibility wrapper。
      Verify: `rg "@formily|components/formily|Formily|formily|form-render|FormRender" "web/src" "web/package.json"` 无命中；web build 通过。
      Handoff: guard 防止回流。

- [x] `H-002` 删除旧 Page renderer 与旧运行 registry
      Owner: root
      Depends: [`A-002`, `E-002`, `E-003`, `E-004`, `E-005`, `E-006`, `E-007`]
      Scope: 旧 renderer、旧运行 registry、旧页面路由。
      Deliverable: Console 仅使用 vNext PageRenderer。
      Forbidden: 不得保留 fallback renderer 或 feature flag 双路径。
      Verify: 全量浏览器 E2E 与 `bash "scripts/dashboard_vnext_guard.sh"` 通过。
      Handoff: `H-004` 可删除旧 API/DTO。

- [x] `H-003` 删除旧 Page schema validator/editor
      Owner: root
      Depends: [`A-002`, `D-002`, `F-004`]
      Scope: 旧 PageSchemaEditor、旧 validator、JSON page editor API。
      Deliverable: 页面编辑只使用强类型 DTO 与语义面板。
      Forbidden: 不得保留 JSON 编辑作为正常或隐藏兼容路径。
      Verify: 浏览器 E2E 编辑/发布通过；`rg` 无旧 editor/validator 引用。
      Handoff: `H-004` 可删除旧 API/DTO。

- [x] `H-004` 删除旧注册 UI 扩展、页面 API 与 DTO
      Owner: root
      Depends: [`B-001`, `H-002`, `H-003`]
      Scope: SDK/OpenAPI 页面扩展、旧页面 API/DTO、旧 workspace/object-page 配置。
      Deliverable: 注册与运行路径只剩 vNext 合同、语义、Proposal、PublishedPageSpec。
      Forbidden: 不得提供数据转换桥或 compatibility endpoint。
      Verify: guard 通过；SDK parity、服务集成和浏览器 E2E 不依赖旧接口。
      Handoff: `H-005` 可清理旧表/数据。

- [x] `H-005` 备份后删除旧页面表/列与历史数据
      Owner: root
      Depends: [`H-004`]
      Scope: `internal/model/migration_legacy_cleanup.go`, `internal/model/migration.go`。
      Deliverable: 版本化清理函数 `CleanupAllLegacy` 在启动时安全移除旧 `function_ui*` 表和 `functions` 表上的 UI 列；`LegacyCleanupReport` 提供 dry-run 报告。
      Forbidden: 未取得单独明确确认不得执行生产数据删除。
      Verify: `go test ./internal/model -run 'TestCleanup' -count=1 -v` 全部通过；`bash scripts/dashboard_vnext_guard.sh` 通过。
      Handoff: `J-001` 可声明无旧模型依赖。生产部署前需备份校验和 deployment dry-run。

## I. 注册一致性、运行时契约与真实浏览器闭环

> 这是针对已发现失效链路的修复与验收清单，不是“补几个页面”的清单。原 `I-001`～`I-003`
> 被拆为不可再以 mock、手工插库、按钮存在或单测替代的原子项。每个真实链路测试都必须启动
> Server、真实 Agent/SDK 或真实 OpenAPI provider，并通过 HTTP/UI 操作系统；fixture 可以提供
> 确定性业务数据，但不得预置 FunctionContract、CapabilitySemantics、Proposal 或 PageSpec。
>
> 以下任务完成前，不得宣称“函数注册后可自动生成可用 UI”。

### I-A. SDK 注册语义与注册快照

- [x] `I-001` 移除 SDK 注册主链的函数名 CRUD 推断
      Owner: root
      Depends: [`B-004`]
      Scope: `internal/platform/registry/store.go`, `internal/platform/registry/infer.go` 及直接测试。
      Deliverable: `UpsertAgent` 只持久化 SDK 显式提交的 `resource`、`operation`、`capability`；函数 ID 不再补全任何资源或 CRUD 语义。
      Forbidden: 不得以函数名、tag、summary 或 schema 名称猜测 CRUD；不得修改 OpenAPI REST 的受控推导。
      Verify: `go test ./internal/platform/registry -run 'TestUpsertAgentDoesNotInferSDKResourceOrCapability|TestRegistrationMaterializesDefaultOperationProposal' -count=1`。
      Handoff: `I-002` 可验证未标注 SDK 函数的确定性降级。

- [x] `I-002` 覆盖未标注 SDK 函数的 OperationPage 降级
      Owner: root
      Depends: [`I-001`, `C-007`]
      Scope: `internal/platform/registry/`, `internal/service/contract_service_test.go`。
      Deliverable: 仅有 `id + inputSchema + outputSchema` 的 SDK 函数生成一个 standalone Operation Proposal，不创建 ResourceCapability 或 Resource Proposal。
      Forbidden: 不得通过 Resource Catalog 预填语义、手工写 Contract 或仅断言 capability 为空替代完整 proposal 断言。
      Verify: `go test ./internal/platform/registry ./internal/service -run 'TestUpsertAgentDoesNotInferSDKResourceOrCapability|TestRegistrationMaterializesDefaultOperationProposal' -count=1`。
      Handoff: `I-022` 使用同一语义规则做真实注册验收。

- [ ] `I-003` 计算 Agent 注册函数快照的新增、变更和删除集
      Owner: unassigned
      Depends: []
      Scope: `internal/platform/registry/store.go`, Agent session 持久化读取层、registry 测试。
      Deliverable: 同一 `agentID + gameID + env` 再注册时，系统在写入前确定旧函数集和新函数集，产生稳定、排序的 added/changed/removed 集合及受影响 resource 集合。
      Forbidden: 不得把 `nil` functions 当作空快照删除所有函数；不得只比较函数数量或 map 遍历顺序。
      Verify: `go test ./internal/platform/registry -run '^TestUpsertAgentClassifiesFunctionSnapshotDiff$' -count=1`。
      Handoff: `I-004`、`I-005` 只消费该 diff，不各自重新读取不一致快照。

- [x] `I-004` 删除已从 Agent 快照消失的 FunctionContract
      Owner: root
      Depends: [`I-003`]
      Scope: `internal/platform/registry/`, `internal/service/contract_service.go`, `internal/model/function_contract_model.go`。
      Deliverable: removed 函数在同一 scope 中删除其 FunctionContract，且不会删除其他 Agent 或其他 scope 的同名函数。
      Forbidden: 不得软禁用代替删除；不得删除仍被当前 Agent 快照声明的合同。
      Verify: `go test ./internal/platform/registry -run 'TestUpsertAgentRemovesContractsAndProposalsAbsentFromSnapshot|TestUpsertAgentKeepsContractDeclaredByAnotherAgent' -count=1`。
      Handoff: `I-005` 基于删除后的合同集合重建派生对象。

- [x] `I-005` 重算受函数增删影响的 ResourceCapability 与 Proposal
      Owner: root
      Depends: [`I-004`]
      Scope: `internal/platform/registry/`, `internal/service/contract_service.go`, proposal/capability model 测试。
      Deliverable: 对旧 resource 和新 resource 的并集各重建一次语义聚合与 Resource/standalone Proposal；不再引用被删除 FunctionContract 的 binding。
      Forbidden: 不得只重建新快照中的 resource；不得保留引用不存在 function 的 proposal。
      Verify: `go test ./internal/platform/registry ./internal/service -run 'TestUpsertAgentRemovesContractsAndProposalsAbsentFromSnapshot|TestContractService_RebuildProposalsForResource|TestContractService_RebuildProposalForFunctionWithoutResource' -count=1`。
      Handoff: `I-006` 可处理已无法物化的历史 proposal。

- [ ] `I-006` 清理不再可物化的 Proposal 并使已发布页进入 stale
      Owner: unassigned
      Depends: [`I-005`, `D-004`]
      Scope: `internal/service/contract_service.go`, `internal/service/proposal_service.go`, proposal/page model、freshness 测试。
      Deliverable: 函数删除后，不再可物化的 draft/proposal 被删除或转为明确 blocked issue；所有引用该函数的 PublishedPageSpec 返回 `binding_function_missing` 并拒绝 execute。
      Forbidden: 不得保留空壳 ResourcePage；不得自动以其他同 capability 函数替换 binding；不得静默重写 PublishedPageSpec。
      Verify: `go test ./internal/service ./internal/api/console -run '^TestRemovedRegisteredFunctionInvalidatesProposalAndStalesPublishedPage$' -count=1`。
      Handoff: `I-012` 和 `I-039` 可验证删除/变化后的用户可见治理状态。

- [ ] `I-007` 使 Agent session 与派生注册状态持久化原子提交
      Owner: unassigned
      Depends: []
      Scope: `internal/platform/registry/store.go`, 事务/DB context 传递、相关 model/service 测试。
      Deliverable: Session snapshot、FunctionContract、CapabilitySemantics、Proposal/BlockedIssue 的本次注册变更要么全部提交，要么全部回滚；内存 registry 仅在提交后更新。
      Forbidden: 不得先写 `agent_sessions` 再尝试重建；不得用日志告警替代回滚；不得跨 scope 使用事务。
      Verify: `go test ./internal/platform/registry -run '^TestUpsertAgentRollsBackPersistentStateWhenMaterializationFails$' -count=1`。
      Handoff: `I-008` 可在失败路径验证重试一致性。

- [ ] `I-008` 覆盖注册失败后的重试与重启恢复一致性
      Owner: unassigned
      Depends: [`I-007`]
      Scope: `internal/platform/registry/`, session loader、server integration test。
      Deliverable: 强制一次合同/Proposal 重建失败后，同快照重试与进程重启恢复都得到单一一致的 Session、Contract 和 Proposal 集合。
      Forbidden: 不得通过清库、手工修复行或忽略失败实现测试通过。
      Verify: `go test ./internal/platform/registry ./cmd/server -run '^TestFailedAgentRegistrationRetryAndRestartAreConsistent$' -count=1`。
      Handoff: `I-023` 可安全使用真实 SDK fixture 重复注册。

### I-B. OpenAPI 解绑与派生页面回收

- [x] `I-009` 为 OpenAPI binding 建立可追溯的合同归属查询
      Owner: root
      Depends: [`B-003`]
      Scope: `internal/api/openapi/`, `internal/model/` 中 OpenAPI source binding/FunctionContract 关联和测试。
      Deliverable: 给定 `sourceID + bindingID + scope` 可精确列出该 binding 物化的 FunctionContract 与受影响 resource，不依赖模糊 function ID 前缀匹配。
      Forbidden: 不得跨 source、跨 game/env 关联；不得重新解析远程 OpenAPI 文档才知道删除目标。
      Verify: `go test ./internal/api/openapi -run 'Test(DeleteBindingRemovesOnlyOpenAPIContractAndResourceProposal|DeleteBindingRebuildsContractFromRemainingOpenAPIBinding)' -count=1`。
      Handoff: `I-010` 以该归属关系执行删除。

- [x] `I-010` 删除 OpenAPI binding 时删除其 FunctionContract
      Owner: root
      Depends: [`I-009`]
      Scope: `internal/api/openapi/service.go`, Contract 删除服务、OpenAPI service 测试。
      Deliverable: `DELETE /openapi/sources/:sourceId/bindings/:bindingId` 成功后，该 binding 的合同不再出现在 scope 合同列表。
      Forbidden: 不得只删 binding 行；不得影响同 source 的其他 binding 或同一 function 的 SDK 合同。
      Verify: `go test ./internal/api/openapi -run 'TestDeleteBindingRemovesOnlyOpenAPIContractAndResourceProposal' -count=1`。
      Handoff: `I-011` 可对受影响资源重建。

- [x] `I-011` 删除 OpenAPI binding 后重算资源页面与 standalone 页面
      Owner: root
      Depends: [`I-010`, `I-005`]
      Scope: `internal/api/openapi/`, `internal/service/contract_service.go`、proposal 测试。
      Deliverable: binding 删除后，受影响 resource proposal 的 bindings、质量和 blocked issue 根据剩余合同重新生成；独立 operation proposal 同步回收或刷新。
      Forbidden: 不得将删除前 PageSpec 原样保留；不得依赖下次 Agent 心跳才修复。
      Verify: `go test ./internal/api/openapi -run 'Test(DeleteBindingRestoresSDKContractAndProposal|DeleteBindingRemovesOnlyOpenAPIContractAndResourceProposal|DeleteBindingRebuildsContractFromRemainingOpenAPIBinding)' -count=1`。
      Handoff: `I-012` 可将已发布页面标记为不可执行。

- [ ] `I-012` 删除 OpenAPI binding 后使已发布页 stale 且可解释
      Owner: unassigned
      Depends: [`I-011`, `D-004`, `F-001`]
      Scope: `internal/api/openapi/`, `internal/service/proposal_service.go`, `internal/api/console/`、服务测试。
      Deliverable: 已发布页在 inbox/console 中显示明确的 missing binding/function 诊断，execute 返回 stale 拒绝而非空结果或 500。
      Forbidden: 不得自动取消发布、替换到最新 binding 或隐藏诊断。
      Verify: `go test ./internal/api/openapi ./internal/api/console ./internal/service -run '^TestDeleteBindingStalesPublishedPageAndRejectsExecution$' -count=1`。
      Handoff: `I-039` 使用同一 stale 行为做真实浏览器验收。

### I-C. 前后端协议、页面解析与测试可信度

- [ ] `I-013` 统一 Console menu mock 与真实 `ConsoleMenuSpec`
      Owner: unassigned
      Depends: [`D-001`, `G-001`]
      Scope: `web/mock/dashboard.ts`, `web/src/types/dashboard.ts`、mock contract 测试。
      Deliverable: mock 和真实 `/api/v1/console/menu` 都返回 `{ items: [...] }` 的同一 DTO；mock 只作为开发替身，不再定义第二种 `categories` 协议。
      Forbidden: 不得在前端添加 `items ?? categories` 兼容分支；不得改动服务端协议以迁就 mock。
      Verify: `pnpm --dir "web" exec jest --runInBand mock/dashboard.contract.test.ts`。
      Handoff: `I-020` 的非 mock 测试可使用同一解析器。

- [ ] `I-014` 修正页面 regenerate 客户端与后端路由的唯一契约
      Owner: unassigned
      Depends: []
      Scope: `web/src/services/api/pages.ts`, Page Studio 调用方、`internal/handler/routes.go`/`internal/router/router.go` 及 API contract 测试。
      Deliverable: 页面再生成只保留一个已注册 API 路径、request/response DTO 和调用方；从浏览器发起的请求不再 404 或命中语义不同的 handler。
      Forbidden: 不得保留双路由、前端 fallback 或“未使用所以不修”的死接口。
      Verify: `go test ./internal/api/page ./internal/service/versioning -run '^TestRegenerateRouteMatchesWebClientContract$' -count=1`。
      Handoff: Page Studio 的重新生成按钮可纳入真实 E2E。

- [ ] `I-015` 处理未注册的 OpenAPI validate 客户端接口
      Owner: unassigned
      Depends: []
      Scope: `web/src/services/api/openapi.ts`, OpenAPI routes/handler、调用方和契约测试。
      Deliverable: `validateOpenAPISpec` 要么调用一个已注册且有测试的后端验证端点，要么连同所有调用方删除；仓库不存在悬空 API helper。
      Forbidden: 不得保留会返回 404 的 helper；不得用前端假成功掩盖缺路由。
      Verify: `rg -n '/api/v1/openapi/validate' "web/src" "internal"` 的命中要么同时含注册路由与 handler 测试，要么为 0。
      Handoff: OpenAPI 导入 UI 的错误反馈可被真实 E2E 断言。

- [ ] `I-016` 为 ResourcePage 输出 selector 缺失建立显式失败态
      Owner: unassigned
      Depends: [`D-002`, `E-002`]
      Scope: `web/src/components/PageRenderer/ResourcePageRenderer.tsx`, selector/runtime 测试。
      Deliverable: list/detail 结果无法按 PublishedPageSpec selector 映射时，页面显示可定位诊断并保持空/错误态；有效 `items`、`total`、detail selector 映射出真实数据。
      Forbidden: 不得猜 `response.data.items`、吞掉 selector 错误或只渲染空表格。
      Verify: `pnpm --dir "web" exec jest --runInBand src/components/PageRenderer/ResourcePageRenderer.test.tsx`。
      Handoff: `I-032`、`I-033` 可断言列表和详情的数据而非 DOM 空壳。

- [ ] `I-017` 修复 Jest TypeScript 项目边界并恢复运行时单测
      Owner: unassigned
      Depends: []
      Scope: `web/tsconfig.jest.json`, `web/jest.config.ts`，不包含业务页面重写。
      Deliverable: Jest 不再因 `TS5011 rootDir` 在测试执行前失败，且只包含 unit/integration test 所需的源码与 setup 文件。
      Forbidden: 不得用关闭 ts-jest 诊断、`--passWithNoTests` 或移除测试文件伪造通过。
      Verify: `pnpm --dir "web" exec jest --runInBand src/components/PageRenderer src/components/SchemaFormRenderer`。
      Handoff: `E-001`、`E-002` 的既有 Verify 可重新获得有效证据。

- [ ] `I-018` 消除同名真实页与 Placeholder 页的路由歧义
      Owner: unassigned
      Depends: []
      Scope: `web/config/routes.ts`, `web/src/pages/Admin/`, `web/src/pages/Support/Tickets/` 及路由测试。
      Deliverable: 每个配置路由都显式解析到唯一实际组件；`Admin/LoginLogs`、`Admin/OperationLogs`、`Support/Tickets/Detail` 不会因同名目录 `index.tsx` 命中占位页面。
      Forbidden: 不得通过修改路由顺序碰巧规避；不得保留无法到达的重复页面实现。
      Verify: `pnpm --dir "web" exec jest --runInBand src/pages/routes-resolution.test.tsx`，并断言三个路由均未显示“建设中”占位内容。
      Handoff: 后台页面可与 Console 生成页面分开诊断，不再混淆为空壳 UI 问题。

- [ ] `I-019` 建立不启用 Mock 的 Playwright 启动配置
      Owner: unassigned
      Depends: []
      Scope: `web/playwright.config.ts`, E2E 启动脚本、测试环境变量文档。
      Deliverable: 提供命名的真实链路项目/命令，启动 Web 时不设置 `MOCK=all`，并要求显式 Server 基地址和 fixture 生命周期。
      Forbidden: 不得修改默认 mock 项目来冒充真实项目；不得让真实项目在 Server 不可达时降级到 mock。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard --list`，输出的 webServer command 不含 `MOCK=all`。
      Handoff: `I-021` 至 `I-043` 统一使用 `real-dashboard` 项目。

- [ ] `I-020` 禁止 E2E 跳过式与空数据式成功断言
      Owner: unassigned
      Depends: [`I-019`]
      Scope: `web/e2e/`, E2E helper、review guard 测试。
      Deliverable: Dashboard 真实链路 spec 中不存在 `test.skip`、元素缺失后提前 return、`rows >= 0` 或只断言容器存在；每一步断言具体按钮、HTTP 成功、可识别业务记录或期望错误码。
      Forbidden: 不得把断言移到 mock 专用 spec；不得以截图存在替代状态/数据断言。
      Verify: `rg -n 'test\\.skip|\\.skip\\(|rows\\s*>=\\s*0|if \\(!.*button.*\\).*return' "web/e2e"` 无命中，且 `pnpm --dir "web" exec playwright test --project=real-dashboard --list` 成功。
      Handoff: 所有后续浏览器任务的失败都具有诊断价值。

### I-D. 真实 SDK、OpenAPI 与 stale 浏览器验收

- [ ] `I-021` 提供可重复清理的真实 Server/Agent/Provider E2E fixture
      Owner: unassigned
      Depends: [`I-007`, `I-019`]
      Scope: server integration fixture、测试 Agent/SDK、测试 OpenAPI provider、Playwright global setup/teardown。
      Deliverable: 一个命名 fixture 能以干净 scope 启动 Server、连接真实 Agent、暴露确定性 SDK 与 `/players` provider 数据，并在测试结束后仅清理本 fixture 的 scope。
      Forbidden: 不得访问生产服务；不得直接插入 dashboard 派生表；不得清空共享数据库或使用宽泛删除。
      Verify: `go test ./cmd/server -run '^TestRealDashboardFixtureHealth$' -count=1` 与 `pnpm --dir "web" exec playwright test --project=real-dashboard --grep '@fixture-health'`。
      Handoff: `I-022` 至 `I-043` 可在同一受控环境运行。

- [ ] `I-022` 真实 SDK 未标注函数注册并生成 Operation Proposal
      Owner: unassigned
      Depends: [`I-002`, `I-021`]
      Scope: SDK fixture、Server registration integration、`web/e2e/operation.spec.ts`。
      Deliverable: fixture 调用真实 SDK 注册 `mail.send`（仅 ID/schema）后，Server API 返回唯一 `operation--mail.send` basic Proposal，且不存在 `resource--mail`。
      Forbidden: 不得调用 Contract/Proposal repository 建数据；不得给该 fixture 额外 capability 或 Resource Catalog 语义。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/operation.spec.ts --grep '@sdk-unannotated-proposal'`。
      Handoff: `I-023` 可从真实 proposal 而非 mock 页面预览。

- [ ] `I-023` 真实 SDK Operation Proposal 的预览与发布
      Owner: unassigned
      Depends: [`I-022`, `F-002`]
      Scope: `web/e2e/operation.spec.ts`, fixture API 断言。
      Deliverable: 用户在 Proposal Inbox 预览并发布 `operation--mail.send`；发布后 PublishedPageSpec 含冻结 binding snapshot。
      Forbidden: 不得用页面 API 直接发布预置 PageSpec；不得仅检查 toast。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/operation.spec.ts --grep '@sdk-operation-publish'`，断言 preview API、publish API 和 snapshot 内容。
      Handoff: `I-024` 可验证菜单与路由。

- [ ] `I-024` 真实 SDK 发布后菜单和 Console 路由可达
      Owner: unassigned
      Depends: [`I-023`, `G-001`]
      Scope: `web/e2e/operation.spec.ts`, Console menu/route 断言。
      Deliverable: 发布的 `mail.send` 仅通过 Console menu 返回并可导航至其 published Console 页面。
      Forbidden: 不得从静态导航、proposal 列表或 mock 菜单进入；不得只验证 URL 变化。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/operation.spec.ts --grep '@sdk-operation-menu'`，断言 `/api/v1/console/menu` 的 item 与页面标题一致。
      Handoff: `I-025` 可从 published binding 执行。

- [ ] `I-025` 真实 SDK Operation 执行并渲染结构化结果
      Owner: unassigned
      Depends: [`I-024`, `G-003`]
      Scope: SDK fixture、`web/e2e/operation.spec.ts`、必要的 Console API 断言。
      Deliverable: 用户提交 `mail.send` 表单后，fixture 收到 published binding execute，请求和响应按 selector 映射为可见结构化结果。
      Forbidden: 不得从浏览器提交 functionId/target/scope；不得只断言执行按钮或网络 200；不得展示原始 JSON 作为正式结果。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/operation.spec.ts --grep '@sdk-operation-execute'`，断言 fixture 调用计数、audit binding ID 和结果字段文本。
      Handoff: 证明最小 SDK Operation 产品链路。

- [ ] `I-026` 真实 SDK 显式资源能力生成 Resource Proposal
      Owner: unassigned
      Depends: [`I-021`, `B-004`, `C-002`]
      Scope: SDK fixture、Server registration integration、`web/e2e/sdk-crud.spec.ts`。
      Deliverable: 显式 `resource + capability` 且 identity 可验证的 SDK 合同，生成 Resource Proposal；缺必要 identity 时降级/blocked，而非名称猜测。
      Forbidden: 不得复用 OpenAPI path 推导；不得以手工语义覆盖 fixture 的 SDK 输入。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/sdk-crud.spec.ts --grep '@sdk-explicit-resource-proposal'`。
      Handoff: 证明 SDK 与 OpenAPI 以同一页面模型收敛。

- [ ] `I-027` 提供符合 schema 的真实 OpenAPI `/players` provider 数据
      Owner: unassigned
      Depends: [`I-021`]
      Scope: OpenAPI provider fixture、OpenAPI document fixture、fixture health tests。
      Deliverable: fixture 提供 list/detail/create/update/delete 和一个 row action；每个响应符合导入的 request/response schema，并至少含两条可区分 player 记录。
      Forbidden: 不得将 provider 响应写进前端 mock；不得缺少 identity、分页或 action 所需数据后仍标称 CRUD fixture。
      Verify: `go test ./cmd/server -run '^TestPlayersOpenAPIProviderFixtureContract$' -count=1`。
      Handoff: `I-028` 可真实导入与绑定该 provider。

- [ ] `I-028` 从真实 OpenAPI source 导入并绑定 `/players`
      Owner: unassigned
      Depends: [`I-027`, `B-003`]
      Scope: OpenAPI source integration fixture、`web/e2e/openapi-source.spec.ts`。
      Deliverable: 经 OpenAPI API/界面导入文档并建立 provider binding，Server 物化对应 FunctionContract；未预置 Contract、CapabilitySemantics 或 Proposal。
      Forbidden: 不得绕过 source/binding 路由；不得在导入后手工修改数据库以补充 CRUD。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-source.spec.ts --grep '@openapi-import-bind'`。
      Handoff: `I-029` 可检查生成质量。

- [ ] `I-029` 真实 OpenAPI `/players` 生成 ready Resource Proposal
      Owner: unassigned
      Depends: [`I-028`, `C-002`, `C-003`, `C-004`, `C-005`, `C-006`]
      Scope: `web/e2e/openapi-crud.spec.ts`, proposal API 断言。
      Deliverable: `/players` 的 REST 语义、identity 和 selector 通过验证后生成单一 ready `resource--players` Proposal，包含 list/detail/create/update/delete/row action binding。
      Forbidden: 不得预置 PageSpec；不得把缺 selector 的 proposal 标记 ready；不得将每个 CRUD operation 生成为独立资源页面。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-ready-proposal'`。
      Handoff: `I-030` 可发布同一个 proposal。

- [ ] `I-030` 发布真实 OpenAPI Resource Proposal 并确认菜单来源
      Owner: unassigned
      Depends: [`I-029`, `F-002`, `G-001`]
      Scope: `web/e2e/openapi-crud.spec.ts`, Console menu API 断言。
      Deliverable: Inbox 发布 `resource--players` 后，Console 菜单包含此 published page，且没有通过 source/contract 直接生成的额外菜单项。
      Forbidden: 不得由静态 locale 或 OpenAPI tag 构造菜单；不得只检查 publish 成功提示。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-resource-publish'`。
      Handoff: `I-031` 至 `I-036` 从 published Console 页面操作。

- [ ] `I-031` 真实 OpenAPI 列表、分页和刷新渲染业务数据
      Owner: unassigned
      Depends: [`I-030`, `I-016`]
      Scope: `web/e2e/openapi-crud.spec.ts`, provider fixture 调用记录。
      Deliverable: Console 列表显示 fixture 的两条 player 数据、正确 total 和分页；刷新发起新的 published list binding execute。
      Forbidden: 不得仅断言表格节点、行数非负或 mock 数据；不得从 lastResult 读取整行。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-list-pagination'`。
      Handoff: `I-032` 可选择真实行 identity。

- [ ] `I-032` 真实 OpenAPI 详情按 row identity 获取并渲染
      Owner: unassigned
      Depends: [`I-031`]
      Scope: `web/e2e/openapi-crud.spec.ts`, provider fixture 请求记录。
      Deliverable: 点击指定 player 后，detail binding 只接收该行 selector 提取的 identity，页面展示 provider 返回的详情字段。
      Forbidden: 不得透传整行 JSON；不得把 collection 响应伪装成 detail 成功。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-detail-identity'`。
      Handoff: 详情 selector 已获真实证据。

- [ ] `I-033` 真实 OpenAPI create 后刷新列表
      Owner: unassigned
      Depends: [`I-031`, `E-003`]
      Scope: `web/e2e/openapi-crud.spec.ts`, provider fixture 状态。
      Deliverable: 填写生成表单创建 player，provider 持久化该记录，成功后 Console 列表出现新 identity 和名称。
      Forbidden: 不得使用第二套表单；不得靠前端乐观插行或 fixture 预先含有该记录。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-create-refresh'`。
      Handoff: `I-034` 可编辑刚创建的真实记录。

- [ ] `I-034` 真实 OpenAPI update 只提交 selector identity 与可编辑字段
      Owner: unassigned
      Depends: [`I-033`, `E-003`]
      Scope: `web/e2e/openapi-crud.spec.ts`, provider fixture 请求断言。
      Deliverable: 更新表单不暴露 identity 输入；execute payload 包含 row selector 的 identity 和已修改字段，刷新后页面显示 provider 的更新值。
      Forbidden: 不得让用户编辑 identity；不得传递整行或未修改的隐藏字段作为 payload。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-update-selector'`。
      Handoff: CRUD 更新路径已获真实证据。

- [ ] `I-035` 真实 OpenAPI delete 经确认后移除记录
      Owner: unassigned
      Depends: [`I-034`, `E-003`]
      Scope: `web/e2e/openapi-crud.spec.ts`, provider fixture 状态。
      Deliverable: 删除动作先展示生成的确认信息，确认后仅删除选中 identity，列表刷新后不再显示该记录。
      Forbidden: 不得跳过确认；不得删错行或用前端隐藏代替 provider 删除。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-delete-confirm'`。
      Handoff: delete 治理和数据刷新已获真实证据。

- [ ] `I-036` 真实 OpenAPI row action 只接收受控上下文
      Owner: unassigned
      Depends: [`I-031`, `E-004`, `G-003`]
      Scope: `web/e2e/openapi-crud.spec.ts`, provider fixture/audit 断言。
      Deliverable: 对指定 player 执行 row action 时，provider 和 audit 收到 published binding ID 与 selector 派生 identity，结果反馈到该页面。
      Forbidden: 不得由浏览器传 functionId、target、scope 或整行 payload；不得把 action 误生成为 toolbar action。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/openapi-crud.spec.ts --grep '@openapi-row-action'`。
      Handoff: OpenAPI CRUD 真实主路径完成。

- [ ] `I-037` 真实重注册 schema 变化使页面进入 stale
      Owner: unassigned
      Depends: [`I-025`, `I-021`, `D-004`]
      Scope: SDK fixture、`web/e2e/contract-change.spec.ts`、proposal/console API 断言。
      Deliverable: 真实 Agent 对已发布 `mail.send` 更改 input 或 output schema 后，Server 生成新 proposal，published page 显示对应 schema stale diagnostic。
      Forbidden: 不得直接 update FunctionContract；不得自动覆盖 draft 或 PublishedPageSpec。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/contract-change.spec.ts --grep '@schema-change-stale'`。
      Handoff: `I-038` 可验证 UI 执行拒绝。

- [ ] `I-038` 真实 risk/approval 变化使页面进入 governance stale
      Owner: unassigned
      Depends: [`I-025`, `I-021`, `D-004`]
      Scope: SDK fixture、`web/e2e/contract-change.spec.ts`、console API 断言。
      Deliverable: 真实 Agent 更改已发布函数的 risk 或 approval 后，页面显示 governance/approval stale diagnostic，不能继续沿用旧治理快照。
      Forbidden: 不得只测试 version 字符串变化；不得自动接受更高风险或审批要求。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/contract-change.spec.ts --grep '@governance-change-stale'`。
      Handoff: `I-039` 可测试统一的 stale 拒绝。

- [ ] `I-039` 真实 stale 页面执行被拒绝且保留处理入口
      Owner: unassigned
      Depends: [`I-037`, `I-038`, `G-003`, `F-001`]
      Scope: `web/e2e/contract-change.spec.ts`, Console/inbox API 断言。
      Deliverable: stale 页的 execute 返回明确 stale 错误，不调用 Agent；用户能从 Console 或 Inbox 打开 diff/merge/re-publish 处理入口。
      Forbidden: 不得仅禁用按钮而不校验服务端；不得返回泛化 500 或静默执行最新合同。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/contract-change.spec.ts --grep '@stale-execute-rejected'`，断言 Agent 调用计数保持不变。
      Handoff: `I-040`、`I-041` 可处理变化。

- [ ] `I-040` 真实展示类变化自动合并并保留执行快照边界
      Owner: unassigned
      Depends: [`I-039`, `D-005`, `F-004`]
      Scope: `web/e2e/contract-change.spec.ts`, versioning API 断言。
      Deliverable: 仅列 label/order 等安全展示字段变化时，Page Studio 显示自动合并结果；bindings/selectors/risk/permission 未被改写。
      Forbidden: 不得将任何执行字段归入安全集；不得直接覆盖 published 版本。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/contract-change.spec.ts --grep '@safe-auto-merge'`。
      Handoff: `I-042` 可重新发布安全合并结果。

- [ ] `I-041` 真实 identity 或 selector 变化要求人工冲突决策
      Owner: unassigned
      Depends: [`I-039`, `D-006`, `F-004`]
      Scope: SDK/OpenAPI fixture、`web/e2e/contract-change.spec.ts`、versioning API 断言。
      Deliverable: identity、binding 或 selector 变化出现在人工冲突列表；未选择解决方案前不能发布。
      Forbidden: 不得自动合并、隐式替换 identity 或让 stale 页恢复执行。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/contract-change.spec.ts --grep '@identity-conflict-manual'`。
      Handoff: `I-042` 可基于用户显式决策产生新版本。

- [ ] `I-042` 真实重发布后恢复执行且使用新快照
      Owner: unassigned
      Depends: [`I-040`, `I-041`, `D-003`, `G-003`]
      Scope: `web/e2e/contract-change.spec.ts`, Agent fixture、snapshot/audit 断言。
      Deliverable: 自动合并或人工决策后的新版本发布成功，旧 stale snapshot 不再执行，新 binding snapshot 的 schema/governance 生效并可完成一次真实 execute。
      Forbidden: 不得通过取消 stale 标记恢复；不得复用旧 snapshot 或跳过 publish。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard web/e2e/contract-change.spec.ts --grep '@republish-restores-execution'`，断言执行 audit 的 proposal/page version 已更新。
      Handoff: 证明变更治理闭环。

- [ ] `I-043` 真实链路回归套件以零 Mock、零跳过方式运行
      Owner: unassigned
      Depends: [`I-020`, `I-025`, `I-026`, `I-036`, `I-042`]
      Scope: `web/e2e/`, fixture lifecycle、CI workflow。
      Deliverable: SDK Operation、SDK 显式 Resource、OpenAPI CRUD、合同 stale/merge/re-publish 四组命名场景可在干净环境连续运行，任一失败使 CI 失败。
      Forbidden: 不得并入 mock 项目、不允许 retry 掩盖确定性失败、不允许选择性跳过 scenario。
      Verify: `pnpm --dir "web" exec playwright test --project=real-dashboard --grep '@sdk-|@openapi-|@schema-change|@governance-change|@stale-|@safe-|@identity-|@republish-'`。
      Handoff: `J-001` 获得真实产品闭环的浏览器证据。

## J. 最终门禁

- [ ] `J-001` vNext 发布候选验收
      Owner: root
      Depends: [`A-001`, `A-002`, `B-001`, `B-002`, `B-003`, `B-004`, `B-005`, `B-006`, `B-007`, `C-001`, `C-002`, `C-003`, `C-004`, `C-005`, `C-006`, `C-007`, `C-008`, `C-009`, `C-010`, `C-011`, `D-001`, `D-002`, `D-003`, `D-004`, `D-005`, `D-006`, `E-001`, `E-002`, `E-003`, `E-004`, `E-005`, `E-006`, `E-007`, `F-001`, `F-002`, `F-003`, `F-004`, `G-001`, `G-002`, `G-003`, `H-001`, `H-002`, `H-003`, `H-004`, `I-001`, `I-002`, `I-003`, `I-004`, `I-005`, `I-006`, `I-007`, `I-008`, `I-009`, `I-010`, `I-011`, `I-012`, `I-013`, `I-014`, `I-015`, `I-016`, `I-017`, `I-018`, `I-019`, `I-020`, `I-021`, `I-022`, `I-023`, `I-024`, `I-025`, `I-026`, `I-027`, `I-028`, `I-029`, `I-030`, `I-031`, `I-032`, `I-033`, `I-034`, `I-035`, `I-036`, `I-037`, `I-038`, `I-039`, `I-040`, `I-041`, `I-042`, `I-043`]
      Scope: CI、部署验证、SDK parity、docs build。
      Deliverable: 所有产品链路、质量门禁和物理清理任务完成，可声明 vNext 重构完成。
      Forbidden: 不得将历史记录、单测通过或未部署构建当最终验收。
      Verify: Go/web/docs/SDK/Playwright/OTel collector/deployment 验收矩阵全部绿，且 `H-005` 的生产删除另有明确确认。
      Handoff: 产出正式发布候选审计报告。当前被 `I-001` 至 `I-043` 的注册一致性、协议可信度与真实环境闭环验收阻塞。
