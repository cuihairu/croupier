---
title: 函数注册与默认界面
order: 4
category:
  - 核心概念
tag:
  - 函数注册
  - 默认界面
  - ProComponents
---

# 函数注册与默认界面

> **状态**：Target -- 本文说明下一版产品路径。函数注册只提交能力契约；默认页面由 Server 生成。

## 一句话说明

服务开发者注册的是函数能力；平台从能力生成默认页面；运营人员可以直接发布默认页面，只有不满意时才在 Page Studio 编辑业务配置。

```text
FunctionContract
  -> CapabilitySemantics
  -> PageProposal
  -> PageDraft / PublishedPageSpec
  -> ProComponents 页面
```

函数注册者不设计菜单、列、按钮位置、表格、页面布局或 React schema。

## 注册信息

| 字段 | 作用 | 是否页面 UI |
| --- | --- | --- |
| `id`、`version` | 稳定函数身份与变更追踪 | 否 |
| `summary`、`description`、`tags` | 函数目录、搜索、诊断 | 否 |
| `input_schema`、`output_schema` | 表单字段、验证、候选列和详情字段 | 否 |
| `resource`、`operation` | 业务资源与动作归属 | 否 |
| `capability` | `collection_query/item_query/create/update/delete/action/task/report` | 否 |
| `execution`、`approval`、`risk`、`permission` | 调度、审批与治理；审批可与同步/异步组合 | 否 |
| 分类、标题、列、动作位置、mapping、页面类型 | PageProposal/PageSpec | 是，不能注册 |

示例：

```ts
client.registerFunction({
  id: 'player.update',
  version: '1.0.0',
  resource: 'player',
  operation: 'update',
  capability: 'update',
  risk: 'warning',
  input_schema: PlayerUpdateSchema,
  output_schema: PlayerSchema,
}, updatePlayer);
```

`capability` 是能力语义，不是 UI：它只说明 `player.update` 可以参与玩家资源的更新生命周期，不说明它显示为行按钮、详情弹窗还是独立页。这些由 PageProposal 生成，随后由 Page Studio 确定。

## JSON Schema 能自动做什么

JSON Schema 是自动界面的基础，但不是完整页面定义：

| JSON Schema 信息 | 平台可自动生成 |
| --- | --- |
| 输入字段、required、enum、format、默认值 | SchemaFormRenderer 字段与校验；Modal/Drawer 仅作为容器 |
| 输出对象字段 | 详情字段、结果字段候选 |
| 输出 collection item schema | ProTable 列候选 |
| REST path + method + path parameter | CRUD capability 与 identity 高置信度建议 |

JSON Schema 不能自行判断：

- `playerId` 来自手工表单、选中行还是详情上下文。
- 一个数组是表格、下拉数据源还是图表数据。
- 一个函数应作为行操作、批量操作、工具栏操作或独立页面。
- 分页、任务事件、报表指标的业务语义。

这些由 `CapabilitySemantics` 处理；它是平台版本化、可审计的能力语义层，不是 SDK 中的页面配置。

## 默认页面生成

### CRUD Resource

当平台识别到同一资源的 collection query 与 identity 时，生成 ResourcePage Proposal；写 capability 是可选的，因此只读资源也可直接发布：

```text
GET /players                    -> collection_query -> ProTable
GET /players/{playerId}         -> item_query       -> ProDescriptions
POST /players                   -> create           -> Modal/Drawer + SchemaFormRenderer
PATCH /players/{playerId}       -> update           -> Modal/Drawer + SchemaFormRenderer
DELETE /players/{playerId}      -> delete           -> Popconfirm
POST /players/{playerId}/ban    -> action           -> 行操作候选
```

标准 OpenAPI REST 形态可以自动识别；SDK 函数通过受控 `capability` 或 Resource Catalog 的管理员补充获得同样语义。平台不从 `player.list` 等函数名猜测。

### 非 CRUD 函数

| 函数 | 默认 Proposal | 直接发布条件 |
| --- | --- | --- |
| `mail.send` | OperationPage | 有可执行 binding、输入表单、风险/权限与导航默认值 |
| `player.ban` | Resource action 或 OperationPage | identity/context 明确时为资源动作，否则独立操作页 |
| `reward.batchGrant` | TaskPage | 真实 task 状态、事件和结果语义完整 |
| `analytics.retention` | ReportPage | dataset、维度、指标和图表/表格语义完整 |

页面质量：

- `ready`：完整、类型可验证，可直接接受并发布。
- `basic`：安全的同步 OperationPage，可直接接受并发布。
- `needs_review`：缺少不可自动确定的语义或配置，先在 Page Studio 确认。

函数不可执行、契约违法或治理不安全时不生成 Proposal，而是 BlockedProposalIssue（只含诊断与修复指引，不携带页面定义）；`blocked` 不是页面质量。

注册成功不等于自动上线。`ready/basic` 仍由用户显式“接受并发布”；系统绝不自动在运行控制台暴露菜单。

## 表单与页面的边界

函数输入表单使用：

```text
JSON Schema + FormPresentationSpec -> JSON Schema form adapter
```

表单展示只负责一次查询、创建、编辑或动作调用的字段、验证和展示。页面负责组合列表、详情、CRUD、动作、任务与报表。PageSpec 不保存 React 组件树；`@rjsf/antd` 或 ProComponents 都只是 renderer 实现细节。

## 契约变化

函数契约或能力语义变化后：

1. Server 保存新摘要并生成新的 PageProposal。
2. 已发布页面标记 stale，危险 binding 立即拒绝执行。
3. Page Studio 显示 Proposal、当前 Draft 与 PublishedPageSpec 的差异。
4. 用户接受自动安全合并或解决冲突，再发布新快照。

已发布页面永远不会因为函数注册更新而静默改变。

## 注册边界

- SDK/OpenAPI 不提交 UI schema、组件树、页面协议或菜单信息。
- 表格列、分页绑定、typed selector、图表配置、任务事件绑定、菜单、页面标题、分类和按钮位置都在 PageProposal/PageSpec 中确定。
- 平台不通过函数名猜字段、对象 ID、CRUD 或页面位置。
- 用户正常发布路径是接受 Proposal、语义化编辑、校验和发布；原始 JSON 只用于导入导出和受控诊断。

详见 [Dashboard Resource/Page 模型](../../architecture/dashboard-page-model.md) 和 [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)。
