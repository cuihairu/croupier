---
title: ProComponents 页面生成与运行时
icon: appstore
order: 8
category:
  - 系统架构
tag:
  - ProComponents
  - PageProposal
  - Page Studio
---

# ProComponents 页面生成与运行时

> **状态**：In progress -- 页面生成器（`internal/dashboard/generator/`）与唯一前端运行时（`web/src/components/PageRenderer/`、`SchemaFormRenderer`）已落地；真实浏览器 E2E 的 CI 门禁和全部场景验收仍以根目录 `todo.md` 为准。

> **组合模型总纲**：本文聚焦生成器与运行时实现。组合方式的心智模型、
> React 原语映射、表达力边界与路线见 [组合模型与表达力边界](./composition-model.md)。

## 端到端流程总览

从函数注册到用户操作页面的完整链路（括号内为关键实现位置）：

```text
① 注册
   SDK / OpenAPI 声明函数，inputSchema 可携带 x-ui-* 呈现 hints
   （presentation-hints.md「字段清单与映射」）
   ├─ registrationguard 拒绝页面级字段进入注册（internal/function/registrationguard）
   └─ FunctionContract 落库 function_contracts（internal/model/function_contract.go），
      schema digest 供 stale 检测

② 提案生成（确定性：相同输入摘要 + generator version ⇒ 相同 Proposal）
   ├─ 路径 A：契约变更触发重算 / 手动 POST /api/v1/pages/proposals/rebuild
   │   （internal/api/page/service.go RebuildAllProposals）
   └─ 路径 B：组合页编辑器保存 POST /api/v1/versioning/pages/composite
       （internal/service/versioning → CreateCompositeProposal）
   表单派生全页型同源（internal/dashboard/generator/）：
   ├─ operation/task/report → buildFormPresentation（generator.go）
   ├─ resource/composite    → buildFormFromContract（resource_generator.go）
   └─ 字段来源优先级：x-ui-* hints（form_hints.go）> 类型缺省控件
       （enum→Select、date→DatePicker、array(enum)→MultiSelect 等）>
       schema title > key 人性化

③ 审核发布
   ProposalInbox（web/src/components/ProposalInbox）→ accept-and-publish
   （internal/service/proposal_service.go）
   ├─ 质量门槛：error 级诊断拒绝发布；blocked/needs_review 需人工处理
   └─ published_page_specs 不可变快照 + page_versions 历史 + 提案置 accepted

④ 运行时渲染
   PageRenderer 按 PageSpec.type 分发（web/src/components/PageRenderer/）
   全部表单共用唯一运行时 SchemaFormRenderer（@rjsf/antd + ajv8）：
   发布页 inline/弹窗表单、编辑器预览（PreviewRuntime）、Invoke 调试页
   （web/src/pages/Functions/Invoke）——不允许第二套手写表单实现。
   hints 在渲染端二次生效：fields → rjsf uiSchema 仅内存派生，不持久化。

⑤ 受控执行
   POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute
   （internal/api/console/handler.go）
   ├─ 服务端解析 functionId/权限/scope；schema stale 拒绝执行
   └─ onSuccessRefresh / refreshOn / 事件绑定（events）完成区块间联动
```

流程约定速查：

| 阶段 | 事实源                            | 变更入口                                  |
| ---- | --------------------------------- | ----------------------------------------- |
| 注册 | `FunctionContract`（含 hints）    | SDK/OpenAPI 重新注册                      |
| 提案 | `PageProposal`（generator 产出）  | rebuild / 编辑器保存，人工不可直接改 spec |
| 展示 | `PageDraft`（可选人工调整）       | Page Studio，仅限展示类字段               |
| 运行 | `PublishedPageSpec`（不可变快照） | 只能通过新提案 → 再发布                   |

## 为什么不集成 React Admin（决策记录）

React Admin 的可借鉴之处是“资源语义 -> 默认后台页面 -> 局部覆盖”，而不是它的 UI 组件或 `DataProvider` 协议。Croupier 已有 Ant Design Pro/ProComponents，表格、表单、详情、抽屉、步骤、权限和布局能力足够且更贴合现有系统。

| React Admin 概念                      | Croupier 对应实现                                           |
| ------------------------------------- | ----------------------------------------------------------- |
| `Resource`                            | ResourceCapability + CapabilitySemantics                    |
| `getList/getOne/create/update/delete` | PublishedPageSpec 的受控 PageBinding                        |
| List/Edit/Create/Show                 | ProTable、SchemaFormRenderer、Modal/Drawer、ProDescriptions |
| DataProvider                          | 服务端 controlled binding execute API                       |
| 自动路由/菜单                         | PublishedPageSpec -> ConsoleMenuSpec -> ProLayout           |

不能照搬 React Admin 的 CRUD-only 模型；Croupier 在同一平台中还必须支持 Operation、Task、Report 和审批动作。

## 生成器职责

生成器从持久化 FunctionContract 与 CapabilitySemantics 生成 PageProposal，且必须是确定性的：相同输入摘要和 generator version 产生相同 Proposal。

生成兜底的显示名使用 humanize 规则：分隔符（`.` `_` `-` 空格）与 camelCase 边界拆词、每个词首字母大写后以空格连接（如 `player.ban` → `Player Ban`、`HTTPServer` → `HTTP Server`）。实现位于生成器 `HumanizeKey`；页面发布后仍可在 Page Studio 覆盖。

生成器同时负责产出默认 NavigationSpec：ResourcePage `title` 取 humanize `resourceKey`；Operation/Task/Report 的 `title` 取主 binding 的 `summary[systemDefaultLocale]`，缺失时 humanize `operationKey`，再缺失时 humanize 原始 `functionId`；`category.key` 对 ResourcePage 取 `resourceKey` 的第一个 `.` 前缀、对独立 Operation/Task/Report 页面取原始 `functionId` 的第一个 `.` 前缀（无 `.` 时取完整 key），`category.labels` 取该 key 的 humanize 结果。不得从 `operation--mail.send` 这类 pageKey 推断分类。显式 labels 只能来自 Page Studio 人工编辑，SDK/OpenAPI 注册不得提供 labels。`LocalizedText` 显示时按当前界面语言、系统默认语言依次回退；生成器只保证系统默认语言。只有能产出系统默认语言 labels 的 Proposal 才允许标记为 `ready`/`basic`；labels 不齐备时必须降级并记录 diagnostic，不得让“可直接发布”的 Proposal 到发布时才失败。

### Resource CRUD 模板

当检测到 collection query 与 identity 时生成 ResourcePage；create/update/delete 是可选能力。只有查询能力的资源生成只读 ResourcePage，不得错误降级为 OperationPage：

```text
collection_query -> ListViewSpec -> ProTable
item_query       -> DetailViewSpec -> ProDescriptions
create/update    -> FormPresentationSpec（CreateForm/UpdateForm） -> Modal / Drawer + SchemaFormRenderer
delete           -> ConfirmActionSpec -> Popconfirm
action           -> row / toolbar / batch action
```

列表字段、详情字段和表单字段由 JSON Schema 提供候选；生成器必须记录选择理由。`ActionSemantic.subject=resource_item`、`resource_selection`、`none` 分别确定性映射为 row、batch、toolbar action；前两者缺少可验证 identity input 时不得猜测位置，生成独立 OperationPage Proposal，`none` 不要求 identity input。没有可靠 identity 或 collection 语义时不得伪造 CRUD 页，降级为 OperationPage Proposal。

### 非 CRUD 模板

| 能力                              | 默认页面            | 必须是真实实现                                                                   |
| --------------------------------- | ------------------- | -------------------------------------------------------------------------------- |
| `action` / 无 resource 的同步函数 | OperationPage       | 表单、确认、受控执行、结构化结果                                                 |
| `task`                            | TaskPage            | 启动、状态、事件、取消/重试、结果                                                |
| `report`                          | ReportPage          | 查询、数据集、指标/维度、图表或表格                                              |
| `approval.required=true`          | Operation/Task Page | 等待态、approvalId、审批状态，以及审批通过后的同步结果或任务状态；不得显示为完成 |

## Page Studio

Page Studio 的第一屏是 Proposal Inbox，而不是 JSON 编辑器：

1. 显示“可直接发布 / 需要处理 / 契约变更”三个队列：可直接发布 = `ready` + `basic` Proposal；需要处理 = `needs_review` Proposal 和 BlockedProposalIssue（只含诊断与修复指引，不携带 spec）；契约变更 = source digest 变化导致 stale 的 Draft 和 PublishedPageSpec。
2. `ready/basic` Proposal 可查看预览、接受、发布。
3. 编辑 ResourcePage 时提供列表、列、详情、表单、动作、导航和权限面板。
4. 编辑 Task/Report 时提供任务/数据集/图表的受控配置面板。
5. JSON 只允许导入、导出、诊断和受权限保护的高级模式，不能是默认编辑器。
6. 函数或语义变化时展示 Proposal 与 Draft/PublishedPageSpec 的三方 diff；用户选择合并或修复后发布。

## PageSpec 节点与运行时组件

PageSpec 节点到 ProComponents 的对应关系固定如下（renderer adapter 的唯一映射表）：

| PageSpec 概念                                                          | 运行时实现                                                             |
| ---------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `ListViewSpec`                                                         | `ProTable`，包含筛选、分页、列设置、批量选择和 toolbar                 |
| `DetailViewSpec`                                                       | `ProDescriptions` / `Descriptions`                                     |
| `FormPresentationSpec`（create/update/operation/task/report 查询表单） | `SchemaFormRenderer`，基于 `@rjsf/antd + @rjsf/validator-ajv8`         |
| `ConfirmActionSpec`                                                    | `Popconfirm` / `Modal.confirm` + 后端风险与审批策略                    |
| `ActionSpec`（row/batch/toolbar）                                      | 表格行内按钮 / 批量栏 / 工具栏按钮，经 binding execute                 |
| `TaskViewSpec`                                                         | 真实 Task binding（status/events/result/cancel）的状态、事件和结果视图 |
| `DatasetSpec` + `ChartSpec[]`                                          | `@ant-design/charts` 或等价 AntV renderer；表格用 `ProTable`           |
| `ResultViewSpec`                                                       | 结构化结果区（键值/集合/标量），禁止原始 JSON 直出                     |
| `ConsoleMenuSpec`                                                      | ProLayout 的动态左侧菜单                                               |

## 运行时约束

- ProLayout 只合并固定系统菜单和 `ConsoleMenuSpec`，不从 Resource/Function 推断运行菜单。
- 每个 Page renderer 只消费 PublishedPageSpec，不读取草稿或最新 descriptor 来补字段。
- 每次调用只走 binding execute API；运行时状态按 page instance 隔离。
- `TaskView` 连接真实 Task API；`ReportView` 使用真实图表 renderer；禁止 JSON 占位进入发布态。
- 所有 loading/error/empty/permission/stale 状态由统一 renderer shell 处理，避免每个页面重新实现。

## 表单渲染

Function input、查询、创建、编辑、动作弹窗（含组合页 dialog 区块）、编辑器
预览（`PreviewRuntime`）和 Invoke 调试页共用 `SchemaFormRenderer`。表单
runtime 固定为 `@rjsf/antd + @rjsf/validator-ajv8`：

```text
JSON Schema + FormPresentationSpec
  -> SchemaFormRenderer
  -> rjsf uiSchema derived in memory
  -> JSON Schema validation
  -> typed PageBinding input assignments
```

这层不得耦合 PageSpec 的页面布局。renderer 私有 `uiSchema` 只能由 FormPresentationSpec 在前端临时派生，不能持久化、不能进入 SDK/OpenAPI、不能成为第二套页面协议。项目内禁止保留 Formily、form-render 或自研 ProForm field factory 作为并行运行时；**同样禁止在渲染路径上手写原生 `<input>` 表单**（如旧版弹窗/预览实现，已于 2026-09 移除，统一走 SchemaFormRenderer）。

## 真实验收

以下项目必须由真实浏览器 E2E（`web/e2e/`，`real-dashboard` Playwright 项目）验证，回归入口见 [真实 Dashboard E2E](../development/real-dashboard-e2e.md)：

- OpenAPI REST 和 SDK capability 各生成一个 ResourcePage，并可发布执行；覆盖完整 CRUD（`@openapi-*`）与 SDK 显式资源（`@sdk-*`）。
- 无 CRUD 语义函数生成 `basic` OperationPage，并可直接发布。
- 任务页能持续显示真实状态和事件。
- 报表页渲染真实图表或表格数据。
- 函数变更后页面被 stale 阻断，用户完成 diff、合并和重新发布（`@schema-change-*`、`@stale-*`、`@safe-auto-merge`、`@republish-*`）。
- 切换 game/env 后菜单、页面和执行严格隔离。
