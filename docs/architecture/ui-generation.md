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

> **状态**：Target -- Croupier 的页面运行时使用 Ant Design Pro/ProComponents。

## 为什么不集成 React Admin

React Admin 的可借鉴之处是“资源语义 -> 默认后台页面 -> 局部覆盖”，而不是它的 UI 组件或 `DataProvider` 协议。Croupier 已有 Ant Design Pro/ProComponents，表格、表单、详情、抽屉、步骤、权限和布局能力足够且更贴合现有系统。

| React Admin 概念 | Croupier 对应实现 |
| --- | --- |
| `Resource` | ResourceCapability + CapabilitySemantics |
| `getList/getOne/create/update/delete` | PublishedPageSpec 的受控 PageBinding |
| List/Edit/Create/Show | ProTable、ProForm、ModalForm、DrawerForm、ProDescriptions |
| DataProvider | 服务端 controlled binding execute API |
| 自动路由/菜单 | PublishedPageSpec -> ConsoleMenuSpec -> ProLayout |

不能照搬 React Admin 的 CRUD-only 模型；Croupier 在同一平台中还必须支持 Operation、Task、Report 和审批动作。

## 生成器职责

生成器从持久化 FunctionContract 与 CapabilitySemantics 生成 PageProposal，且必须是确定性的：相同输入摘要和 generator version 产生相同 Proposal。

### Resource CRUD 模板

当检测到 collection query、identity 和生命周期能力时生成 ResourcePage：

```text
collection_query -> ListViewSpec -> ProTable
item_query       -> DetailViewSpec -> ProDescriptions
create/update    -> FormActionSpec -> ModalForm / DrawerForm
delete           -> ConfirmActionSpec -> Popconfirm
action           -> row / toolbar / batch action
```

列表字段、详情字段和表单字段由 JSON Schema 提供候选；生成器必须记录选择理由。没有可靠 identity 或 collection 语义时不得伪造 CRUD 页，降级为 OperationPage Proposal。

### 非 CRUD 模板

| 能力 | 默认页面 | 必须是真实实现 |
| --- | --- | --- |
| `action` / 无 resource 的同步函数 | OperationPage | 表单、确认、受控执行、结构化结果 |
| `task` | TaskPage | 启动、状态、事件、取消/重试、结果 |
| `report` | ReportPage | 查询、数据集、指标/维度、图表或表格 |
| `approval` | Operation/Task Page | 等待态、approvalId、审批状态，不得显示为完成 |

## Page Studio

Page Studio 的第一屏是 Proposal Inbox，而不是 JSON 编辑器：

1. 显示“可直接发布 / 需要处理 / 契约变更”三个队列。
2. `ready/basic` Proposal 可查看预览、接受、发布。
3. 编辑 ResourcePage 时提供列表、列、详情、表单、动作、导航和权限面板。
4. 编辑 Task/Report 时提供任务/数据集/图表的受控配置面板。
5. JSON 只允许导入、导出、诊断和受权限保护的高级模式，不能是默认编辑器。
6. 函数或语义变化时展示 Proposal 与 Draft/PublishedPageSpec 的三方 diff；用户选择合并或修复后发布。

## 运行时约束

- ProLayout 只合并固定系统菜单和 `ConsoleMenuSpec`，不从 Resource/Function 推断运行菜单。
- 每个 Page renderer 只消费 PublishedPageSpec，不读取草稿或最新 descriptor 来补字段。
- 每次调用只走 binding execute API；运行时状态按 page instance 隔离。
- `TaskView` 连接真实 Task API；`ReportView` 使用真实图表 renderer；禁止 JSON 占位进入发布态。
- 所有 loading/error/empty/permission/stale 状态由统一 renderer shell 处理，避免每个页面重新实现。

## 表单渲染

Function input、查询、创建、编辑和动作弹窗共用 `SchemaFormRenderer`：

```text
JSON Schema + FormPresentationSpec
  -> ProForm field factory
  -> JSON Schema validation
  -> typed PageBinding input assignments
```

这层不得耦合 PageSpec 的页面布局。未来替换表单库只替换 SchemaFormRenderer adapter，不改变页面、发布或执行模型。

## 真实验收

以下项目必须在浏览器 E2E 中验证，服务层单测不能替代：

- OpenAPI REST 和 SDK capability 各生成一个 Resource CRUD Page，并可发布执行。
- 无 CRUD 语义函数生成 `basic` OperationPage，并可直接发布。
- 任务页能持续显示真实状态和事件。
- 报表页渲染真实图表或表格数据。
- 函数变更后页面被 stale 阻断，用户完成 diff、合并和重新发布。
- 切换 game/env 后菜单、页面和执行严格隔离。
