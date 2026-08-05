# Croupier Dashboard 产品重构计划

更新时间：2026-08-03

> 本文是下一版 Dashboard 的唯一实施计划和 AI 交接清单。任何实现、文档、SDK 或测试与本文冲突时，先修正文档和计划，再实现代码；不得并行维护两套模型。

## 0.0 重新审核基线

本轮重新审核后，所有历史勾选均已撤销。任何工作包不得因为“代码看起来已改”“局部测试通过”或“已有实现雏形”重新打勾；只有同时满足该工作包的代码、单元测试、真实浏览器 E2E、文档和旧路径物理清理验收后，才允许勾选。

当前事实基线：

- Dashboard 新版本只有一个模型：canonical `PageSpec`，不是新旧模型并存。
- 函数注册只描述能力；默认页面由平台根据 `FunctionContract + CapabilitySemantics` 生成 `PageProposal`。
- 用户路径必须是：生成默认 Proposal -> 预览 -> 接受/发布 -> Console 左侧动态菜单出现；只有不满意时才进入 Page Studio 编辑。
- 当前工作区存在大量未提交改动，不能把任一阶段声明为完成。
- 以下检查的历史“曾通过”记录不作为交接依据；必须重新执行并记录命令与结果后才可作为证据：Dashboard PageSpec guard、目标后端包测试、`web/tests/consoleMenu.test.ts`。
- 本轮（2026-08-03）已重新执行并通过的检查：`bash "scripts/dashboard_vnext_guard.sh"`；`GOCACHE="/tmp/croupier-go-build" go test ./internal/service ./internal/dashboard/generator ./internal/platform/registry ./internal/service/versioning ./internal/api/page ./internal/api/console ./internal/api/resource ./internal/api/resourcecatalog ./internal/dashboard/...`；`cd "web" && pnpm exec tsc --noEmit`；`rg "@formily|components/formily|Formily|formily|generateFormily|validateFormily" web/src web/package.json` 无命中。
- 未通过或未验收的检查：全量 Playwright E2E、docs build、SDK parity、部署验证、`web/tests/consoleMenu.test.ts` 重新执行、全量 `go test ./...`。其中 `go test ./internal/...` 在当前受限环境下会卡在 `internal/agent` 的 TCP listener 测试（`socket: operation not permitted`），不能作为完成证据。
- 仍需清理的误导项包括历史 split-model 命名、旧页面协议残留、失败测试产物、旧路径文件名和未经过真实验收的文档表述。

重新实现顺序：

1. 先完成 P0 的旧模型盘点、降级与防回流门禁，并完成 P7-a（删除清单与 guard 先行），确保后续 agent 不再沿旧设计实现。P7-b 的物理删除不前置：每个旧模块必须在替代路径通过真实 E2E 后才删除，随 P1–P6 验收滚动完成。
2. 再完成 P1 的注册能力、持久化 CapabilitySemantics 与 Resource Catalog 语义闭环。
3. 再完成 P3-a 的 canonical PageSpec、typed selector 和静态校验；这是 P2 生成器的硬前置，P2 不得在 P3-a 完成前生成或持久化页面。
4. 再完成 P2 的默认 Proposal 生成闭环，再完成 P3-b 的发布快照、stale 与三方合并。
5. 再完成 P4 的 ProComponents runtime，以及 P5/P6 的 Page Studio、Console 动态菜单、执行安全、审计和 OTel。
6. 最后用端到端验收矩阵逐项验证，全部通过前禁止声明“重构完成”。

## 0. 当前结论

### 0.1 产品目标没有取消

Croupier 要实现的不是函数调用目录，也不是手工拼 JSON 的低代码编辑器。目标是游戏运营后台的完整生成式产品：

```text
SDK / OpenAPI 注册能力
  -> 平台识别 CRUD 与非 CRUD 语义
  -> 生成可追溯的默认页面
  -> 用户直接发布，或只在不满意时编辑
  -> Console 左侧按已发布页面动态菜单运行
  -> 全部执行受权限、审批、审计、OTel 和发布快照约束
```

游戏后台必须同时支持：

| 主路径 | 示例 | 默认页面 |
| --- | --- | --- |
| Resource CRUD | 玩家、订单、邮件模板、公告、活动、配置、排行榜配置 | 列表、筛选、分页、详情、新建、编辑、删除、资源动作 |
| Resource Action | 玩家封禁、物品授予、订单补偿 | 行操作、批量操作或工具栏操作 |
| Operation | 全服邮件、刷新缓存、运维命令 | 独立输入表单、确认、结果 |
| Task | 批量发奖、导入导出、数据修复 | 提交、真实进度、事件、结果、取消/重试 |
| Report | 留存、付费、漏斗、经济指标 | 查询、真实图表、数据表、导出 |
| Approval | 高风险 GM 操作、灰度、发布 | 等待审批、审批状态和最终结果 |

### 0.2 已保留的基础资产

以下实现可作为重构基础，但都必须重新接入新模型，不能以“已有代码”阻止模型重构：

- `game_id + env + pageKey` scope 隔离、草稿 revision、发布快照和版本历史。
- `PublishedPageSpec -> ConsoleMenuSpec -> ProLayout` 的动态菜单方向。
- binding execute API、函数权限、审批、审计、OTel 和 traceId 透传。
- OpenAPI Source、Provider binding、函数目录、全局 scope 和权限体系。
- Ant Design Pro、ProComponents、ProLayout、ProTable、ProDescriptions、Modal/Drawer/Popconfirm；ProForm 只作为表单容器能力（ModalForm/DrawerForm 等）保留，不作为 JSON Schema renderer。
- JSON Schema 输入/输出契约和多语言 SDK 注册链路。

### 0.3 必须物理清理的实现

以下实现属于旧模型，重构到对应替代路径后必须物理删除，不新增转换桥：

- 组件树式页面协议、页面 schema validator、页面 schema editor 和原始 JSON 默认编辑路径。
- 注册侧页面编排扩展、任意映射 JSON 注册扩展。
- 旧页面 renderer、旧表单 registry、旧页面 DTO、旧页面 API 字段和旧数据库列。
- 从函数名、首批结果、任意 row JSON、静态 locale 或字典猜页面/菜单。
- 把所有函数都做成 CRUD，或把 CRUD Resource 从模型中排除。
- React Admin 的 `DataProvider/getList/getOne/create/update/delete` 成为后端协议。
- 现行模型外的工作区/对象页配置 API、数据模型、菜单和任何转换桥。

### 0.4 架构权威文档

- `docs/architecture/dashboard-page-model.md`
- `docs/architecture/openapi-sdk-descriptor-v2.md`
- `docs/architecture/ui-schema-spec.md`
- `docs/architecture/ui-generation.md`
- `docs/architecture/console-dynamic-menu.md`
- `docs/architecture/dashboard-glossary.md`

若本文与代码冲突，以权威文档为准；若权威文档彼此冲突，停止实现并先修正文档，不得并行维护两种模型。

## 1. 不可违反的边界

1. **函数注册只描述能力。** SDK/OpenAPI 可提供 JSON Schema、resource、operation、capability、risk、permission、execution；不得提供菜单、页面、组件库、列、mapping、按钮位置、标题或分类 labels。
2. **CRUD 是主路径。** Resource CRUD 是游戏后台大量场景的默认高质量生成路径。
3. **非 CRUD 是一等能力。** Operation、Task、Report、Approval 不得被强行套进 Resource CRUD。
4. **能力语义与页面 UI 分离。** `CapabilitySemantics` 描述 list/get/create/update/delete/action/task/report 与 identity，不描述页面布局；PageSpec 才描述页面编排。
5. **JSON Schema 不等于页面。** JSON Schema 生成字段、候选列和验证；不能单独判断行操作、分页路径、图表或任务状态。
6. **PageSpec 是强类型业务 DSL。** 它不保存 React props、组件树或 `ProTable` 名称；renderer adapter 才选择 ProComponents。
7. **表单协议唯一。** 表单使用 `JSON Schema + FormPresentationSpec + SchemaFormRenderer`（runtime 固定为 `@rjsf/antd + @rjsf/validator-ajv8` adapter）；不得同时维护第二套页面/表单 runtime，包括 Formily、form-render 或自研 ProForm field factory。
8. **PageProposal 与 PageDraft 分离。** Proposal 可重生；Draft 可编辑；Published 不可变。重新生成不能覆盖用户草稿或已发布页。
9. **菜单唯一来源不变。** `active PublishedPageSpec[] -> ConsoleMenuSpec -> ProLayout`；动态 labels 不进入静态 locale/字典。
10. **scope 唯一。** `game_id + env` 来自全局上下文，页面内不得二次选择或由 URL/payload 覆盖。
11. **执行唯一入口不变。** 页面只能用 active PublishedPageSpec binding execute API；浏览器不得传 functionId、route、target、game/env。
12. **无自动转换。** 不自动将现行模型外的历史页面配置转换为新页面；历史数据只能导出、备份和人工重建。
13. **数据库访问只用 GORM。** 新表和新增列以 GORM model + AutoMigrate 为唯一定义；查询、事务、关联一律使用 GORM API，禁止在业务代码中手写原生 SQL（`db.Raw`/`db.Exec`）。旧表/列的物理删除只能在 P7-b 通过验收、完成备份并取得明确确认后，由版本化清理函数调用 `db.Migrator().DropColumn/DropTable` 执行；AutoMigrate 不承担删除职责。GORM 无法表达的运维语句（建库、PRAGMA 等）除外，且必须集中在基础设施层并注明理由。`database/schema.sql` 与 `mysql.schema.sql` 只是参考文档，需与 GORM model 同步更新，不作为迁移执行脚本。
14. **TypeScript 类型不敷衍。** web 项目禁止用 `any`/`unknown` 绕过类型检查：API DTO、PageSpec、FormPresentationSpec、selector AST、ConsoleMenuSpec 等共享类型统一定义在 `web/src/types/`（与服务端 Go DTO 一一对应）并被引用，禁止在页面或组件内重复定义同名结构。`any` 只允许出现在第三方库边界等确实无法避免的位置并注明理由；`unknown` 必须配合类型收窄使用，禁止直接断言为具体类型了事。

## 2. 目标领域模型

### 2.1 FunctionContract

```ts
interface FunctionContract {
  id: string;
  version: string;
  enabled: boolean;
  summary?: LocalizedText;
  description?: LocalizedText;
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;
  resourceKey?: string;
  operationKey?: string;
  capability?: CapabilityKind;
  execution: 'sync' | 'task';
  approval: ApprovalPolicy;
  risk: RiskLevel;
  permission?: string;
}

type CapabilityKind =
  | 'collection_query' | 'item_query' | 'create' | 'update' | 'delete'
  | 'action' | 'task' | 'report';
```

基础 DTO（`Scope`、`FunctionRef`、`SourceDigest`、`Diagnostic`、`LocalizedText`、`JsonPointer`、`JSONSchema`、`JSONValue` 和 `ApprovalPolicy`）唯一以 `docs/architecture/dashboard-page-model.md` 的定义为准；此计划不得复制或扩展第二套结构。

`execution` 与 `approval` 正交：`execution: 'task'` 且 `approval.required: true` 表示审批通过后才启动异步任务；`approval` 不是页面类型，也不能替代任务状态语义。Approval 仍由 OperationPage 或 TaskPage 显示等待态、审批结果和后续执行结果。

### 2.2 CapabilitySemantics

```ts
interface CapabilitySemantics {
  resourceKey: string;
  identity?: IdentitySemantic;
  collection?: CollectionSemantic;
  item?: ItemSemantic;
  lifecycle: Partial<Record<'create' | 'update' | 'delete', FunctionRef>>;
  actions: ActionSemantic[];
  tasks: TaskSemantic[];
  reports: ReportSemantic[];
  sourceDigest: string;
  provenance: SemanticProvenance[];
  diagnostics: Diagnostic[];
}

interface IdentitySemantic {
  itemPath: JsonPointer; // canonical item schema 中的唯一标识字段
  valueType: JsonScalarType;
}

type JsonScalarType = 'string' | 'number' | 'integer' | 'boolean';

interface CollectionSemantic {
  query: FunctionRef;
  itemsPath: JsonPointer; // collection query 输出中项目数组的位置
  itemSchemaDigest: string;
  pagination?: OffsetPaginationSemantic | CursorPaginationSemantic;
}

interface OffsetPaginationSemantic {
  kind: 'offset';
  request: { offset: JsonPointer; limit: JsonPointer };
  response: { total?: JsonPointer; hasMore?: JsonPointer };
}

interface CursorPaginationSemantic {
  kind: 'cursor';
  request: { cursor: JsonPointer; limit?: JsonPointer };
  response: { nextCursor: JsonPointer; previousCursor?: JsonPointer; hasMore?: JsonPointer };
}

interface ActionSemantic {
  function: FunctionRef;
  subject: 'resource_item' | 'resource_selection' | 'none';
  identityInput?: JsonPointer;
}

interface TaskSemantic {
  start: FunctionRef;
  taskId: { resultPath: JsonPointer; valueType: JsonScalarType };
  status: { function: FunctionRef; taskIdInput: JsonPointer; statePath: JsonPointer };
  events?: { function: FunctionRef; taskIdInput: JsonPointer; eventsPath: JsonPointer };
  result?: { function: FunctionRef; taskIdInput: JsonPointer; resultPath: JsonPointer };
  cancel?: { function: FunctionRef; taskIdInput: JsonPointer };
  retry?: { function: FunctionRef; taskIdInput: JsonPointer };
}

interface ReportSemantic {
  query: FunctionRef;
  datasetPath: JsonPointer;
  dimensions: JsonPointer[];
  metrics: JsonPointer[];
}

interface SemanticProvenance {
  field: string;
  source: 'openapi_rest' | 'sdk_explicit' | 'platform_review';
  sourceDigest: string;
  confidence: 'high' | 'low';
  status: 'effective' | 'overridden' | 'conflict';
}
```

`IdentitySemantic.itemPath` 必须在 collection item 或 item query 的输出 schema 中唯一存在；item/update/delete/action 的 identity input 由 P3 typed selector 显式映射并校验，不得要求 collection query 的 input 包含 identity。`CollectionSemantic.pagination` 必须同时声明请求参数和响应元数据的 JSON Pointer；offset 分页必须至少提供 `total` 或 `hasMore`，cursor 分页必须提供 `nextCursor`；缺失时只生成不带分页控件的列表，不得猜测 offset/cursor 协议。

`ActionSemantic.subject` 是资源操作所需的业务上下文，不是按钮位置：`resource_item` 由生成器映射为行操作，`resource_selection` 映射为批量操作，`none` 映射为资源工具栏操作；无法安全判定 subject 或 identity input 时只生成独立 Operation Proposal。`TaskSemantic` 与 `ReportSemantic` 的所有 pointer 都必须可由对应 FunctionContract schema 验证，否则只能生成 `needs_review`。

### 2.3 PageProposal 与 PageSpec

```ts
type PageSpec = ResourcePageSpec | OperationPageSpec | TaskPageSpec | ReportPageSpec;

interface PageProposal {
  id: string;
  scope: Scope;
  proposalKey: string;
  pageKey: string;
  spec: PageSpec;
  quality: 'ready' | 'basic' | 'needs_review';
  generatorVersion: string;
  sourceDigests: SourceDigest[];
  diagnostics: Diagnostic[];
  createdAt: string;
}

// 不可物化的问题不是 Proposal：只保存诊断与修复指引，不携带 spec。
interface BlockedProposalIssue {
  id: string;
  scope: Scope;
  sourceDigests: SourceDigest[];
  diagnostics: Diagnostic[];
  repairHint: string;
}
```

`proposalKey` 是生成器幂等身份：一个 ResourceCapability 只能有 `resource:<resourceKey>`，每个独立 Operation/Task/Report 函数分别有 `<kind>:<functionId>`。`pageKey` 固定为 `resource--<resourceKey>` 或 `<kind>--<functionId>`，其中 source key 必须符合 `[a-z0-9][a-z0-9._-]*`；它是可读的路由与发布身份，不得从 summary、labels 或本次生成结果随机生成。分类建议对 ResourcePage 取 `resourceKey` 的第一个 `.` 前缀、对独立 Operation/Task/Report 页面取原始 `functionId` 的第一个 `.` 前缀（无 `.` 时取完整 key），不得从带 kind 前缀的 pageKey 推断。Resource action 只有在 `subject` 可验证时才并入唯一 ResourcePage；否则保留为函数自己的 Operation Proposal，避免重复菜单或覆盖资源页。

PageSpec 的 page kind、binding、导航、表单、列表、详情、动作、任务和报表节点必须使用明确 Go/TypeScript DTO；mapping 使用 typed selector AST，禁止 `map[string]any`、任意 JSONPath、组件 props 和裸 row 透传。Approval 不是第五种页面类型：`approval: 'required'` 可落在同步 OperationPage 或异步 TaskPage 上，以等待态、审批状态和后续结果呈现，不得发明 ApprovalPageSpec。

## 3. 执行顺序与工作包

每个工作包必须完成“代码、单元测试、文档”后才能标记完成；涉及用户路径的工作包还必须完成真实浏览器 E2E（P1 等纯后端契约包以服务端集成测试替代浏览器 E2E）。该工作包涉及的旧路径清理应同步完成，无法立即删除的必须登记为 P7-b 前置条件项，禁止无登记遗留。禁止仅因有服务层测试就勾选完成。

### P0. 架构冻结、现状降级与删除准备

状态：待重新审核。

#### P0-0. 先完成表单 runtime 替换

> 这是当前重构的第一步，优先级高于 PageSpec、Proposal 和 Console 后续改造。原因：函数调用测试、Page action、QueryForm、Modal/Drawer create/update 都依赖同一个表单 runtime；如果这里继续保留 Formily 或旧 Function Form，后续页面生成仍会反复返工。

当前本地事实：

- 当前表单 runtime 选定为 `@rjsf/antd + @rjsf/validator-ajv8`。
- `@rjsf/*` 只作为前端 renderer adapter；rjsf `uiSchema` 只能从 `FormPresentationSpec` 内存派生，不进入 SDK/OpenAPI/PageSpec/发布快照。
- 已重新验证：`rg "@formily|components/formily|Formily|formily|generateFormily|validateFormily" web/src web/package.json` 无命中；`rg "form-render|FormRender|FunctionFormManager" web/src web/package.json internal` 只剩 `SchemaFormRenderer` 正向命中。
- 待重新验证的观察项（不是完成证据）：Functions Invoke、PageRenderer 的 Operation/Resource/Task/Report/Approval 表单和 Assignments 弹窗似乎已接入同一个 `SchemaFormRenderer`。必须在浏览器 POC 中逐路径实测后才可作为证据。
- 已重新验证：目标 renderer 文件的 `pnpm --dir "web" exec eslint "src/components/PageRenderer/runtime.ts" "src/components/PageRenderer/index.tsx" "src/components/PageRenderer/ResourcePageRenderer.tsx" "src/components/PageRenderer/ReportPageRenderer.tsx" "src/types/dashboard.ts"` 通过。全量 `cd "web" && pnpm exec tsc --noEmit` 仍被仓库既有 `@umijs/max` 类型导出与旧页面 implicit any 阻断，不能作为 P0-0/P4/P6 完成证据；浏览器 POC、真实游戏 JSON Schema 和单路径复用验收仍未完成。

实施顺序：

- [ ] 固定唯一 JSON Schema 表单 runtime 为 `@rjsf/antd + @rjsf/validator-ajv8`；禁止保留 `form-render`、Formily 或自研 ProForm field factory 作为第二运行时。
- [ ] 用真实游戏 JSON Schema 验证 antd v5、React 18、Umi Max 构建、中文错误、array/object、enum、format、默认值和复杂嵌套。
- [ ] 建立 `FormPresentationSpec -> renderer presentation config` 的只读派生 adapter；renderer 私有 UI 配置不持久化、不进入 SDK/OpenAPI、不成为第二套页面协议。
- [ ] 让 Functions Invoke、QueryForm、Page action、create/update Modal/Drawer 和 Assignments 弹窗全部复用同一 renderer。
- [ ] 物理删除 `@formily/*` 依赖、`components/formily`、`generateFormilyFromJsonSchema`、`validateFormilySchema`、Formily 文案和所有 Formily 类型引用。
- [ ] 更新 `scripts/dashboard_vnext_guard.sh`，阻止 Formily runtime 和旧 Function Form 回流；只允许选定的唯一 JSON Schema 表单 runtime。
- [ ] 表单/runtime 相关的 TypeScript 检查、目标单测和浏览器 POC 验收通过后，才允许继续 P1+；全仓库既有 TS 错误（Analytics、Profile、Support、Storage 等）归入第 5 节最终门禁，不阻塞后端契约重构。

验收：`rg "@formily|components/formily|Formily|formily|generateFormily|validateFormily" web/src web/package.json` 无运行代码命中；复杂 input schema 的函数调用、创建、编辑、查询和动作弹窗使用同一 renderer；不持久化 renderer 私有 UI 配置；前后端校验结果一致。

#### P0-1. 冻结新模型与术语

- [ ] 重写 Dashboard、Descriptor、UI Schema、UI Generation 权威文档。
- [ ] 建立 `docs/architecture/dashboard-glossary.md`，仅定义当前术语：FunctionContract、ResourceCapability、CapabilitySemantics、PageProposal、PageDraft、PublishedPageSpec、ResourcePage、OperationPage、TaskPage、ReportPage、FormPresentationSpec、binding。
- [ ] 更新所有 guide/API/SDK 文档，只保留当前模型术语和正向边界。
- [ ] 文档中明确历史页面配置无自动迁移路径，避免 agent 再加入转换桥。

验收：Dashboard PageSpec guard 通过；产品、架构、API、SDK 文档只出现当前模型术语。脚本暂沿用 `scripts/dashboard_vnext_guard.sh` 历史文件名以保持 CI 调用稳定，但检查内容必须是唯一 canonical PageSpec 模型。

#### P0-2. 盘点保留/删除代码

当前盘点事实（2026-08-04，本轮验证）：

- 已物理删除旧 split-model 页面入口：`web/src/pages/PageStudioV2`。
- 已物理删除独立 `ApprovalPageRenderer` 文件；Approval 仍必须作为 OperationPage/TaskPage 的治理状态实现，不得恢复第五种页面类型。
- 已移除后端旧 `FunctionForm*` DTO、`BuildFallbackFormSchema`、fallback Formily component 映射和相关空测试；fallback 只允许输出纯输入 JSON Schema。
- 已重写 `web/README.md` 与 `web/src/pages/Functions/README.md` 中的旧 Function Form/Formily 文案。
- 已增强 `scripts/dashboard_vnext_guard.sh`：阻止 Formily/form-render/FunctionFormManager/旧 fallback form API 和 `PageStudioV2` 回流，并验证唯一 `SchemaFormRenderer` 使用 `@rjsf/antd + @rjsf/validator-ajv8`。
- 已将 FunctionContract/descriptor 层 `execution` 收敛为 `sync|task`：删除 `FunctionExecutionApproval`、generator approval execution 分支、OpenAPI `x-execution=approval` 校验提示和 proto 注释残留。`approval` 仅保留为执行结果 kind、审计 trace 字段和独立治理策略语义。
- Protobuf 生成文件不得手工修改：本轮只保留 `.proto` 源文件与非生成 SDK 源码注释变更；`pkg/pb/**`、`sdks/go/pkg/pb/**`、C#/C++/Python generated protobuf 产物必须由统一生成命令刷新。
- 当前已重新验证命令（2026-08-04）：`bash "scripts/dashboard_vnext_guard.sh"`；`rg "@formily|components/formily|Formily|formily|generateFormily|validateFormily" web/src web/package.json`；`rg "form-render|FormRender|FunctionFormManager" web/src web/package.json internal`；`GOCACHE="/tmp/croupier-go-build" go test ./internal/service ./internal/dashboard/generator ./internal/platform/registry ./internal/service/versioning ./internal/api/page ./internal/api/console ./internal/api/resource ./internal/api/resourcecatalog ./internal/dashboard/...`；`pnpm --dir "web" exec eslint "src/components/PageRenderer/runtime.ts" "src/components/PageRenderer/index.tsx" "src/components/PageRenderer/ResourcePageRenderer.tsx" "src/components/PageRenderer/ReportPageRenderer.tsx" "src/types/dashboard.ts"`。
- 当前未完成检查：全量 Playwright E2E、docs build、SDK parity、部署验证；全量 `go test ./...`/`go test ./internal/...` 仍不能在当前受限环境下作为完成证据，因为 `internal/agent` 的 TCP listener 测试会遇到 `socket: operation not permitted`。

仍需继续修复的旧模型事实：

- 已补齐 `ApprovalPolicy{required, policyKey}` 契约字段：Go/TS `FunctionSpec`、`OperationSpec`、OpenAPI Source operation DTO、normalizer、ContractService、`FunctionContract.Approval` 持久化字段和 Resource generator contract 转换均已接入。
- 已修复 Agent 本地注册元数据快照丢失 `capability/execution` 的问题：`internal/platform/agentlocal.LocalStore.FunctionMetadata()` 现在会保留能力与执行模式，否则上游 Register 会把 SDK 显式语义清空。
- 已修复 JS SDK 手写 ProviderConnect schema 与注册请求丢失 `capability/execution` 的问题，并补充目标测试断言；JS 测试因本地 pnpm 版本切换/签名校验失败未完成执行，不能作为完成证据。
- 已补充 `ContractService` 测试，验证 `ApprovalPolicy{required, policyKey}` 会写入 `FunctionContract.Approval`。
- 已同步 `database/schema.sql` 与 `database/mysql.schema.sql` 的 P1 参考表：`function_contracts`、`resource_capabilities`、`capability_semantics`、`capability_semantic_versions`。参考 SQL 不是迁移执行源，迁移仍以 GORM model + AutoMigrate 为准。
- 仍未完成：SDK/Agent wire protobuf 正式新增 `ApprovalPolicy` 字段并通过统一生成命令刷新 generated 产物；OperationPage/TaskPage 审批等待态、审批后同步/任务结果展示、PageSpec 发布快照冻结 approval、执行前 snapshot approval 校验。
- `web/src/components/page-schema/*` 仍被 Functions Directory 与 Assignments 系统管理页复用。它不是运行控制台 PageSpec，但名称容易与旧组件树 Page schema 混淆；P7-b 必须在替代普通 UI helper 后物理删除或重命名，禁止把它接入 Dashboard PageSpec。
- `web/src/types/dashboard.ts` 当前仍含 `metadata?: Record<string, JSONValue>`、部分 `unknown` 收窄边界和可能过宽 DTO；P3-a/P7-a 类型 guard 需要继续收紧。

- [ ] 为 Dashboard 生成器、spec、页面 schema validator、旧 Page renderer、Page schema editor 和旧页面表单 registry 建立删除清单与调用图。
- [ ] 标注保留模块：scope、PageVersion、PublishedPage、ConsoleMenu、Console execute、OpenAPI Source、Audit/OTel。
- [ ] 删除前先新增新模型替代路径；替代路径通过 E2E 后物理删除旧文件、路由、DTO、数据库列和 CI allowlist。
- [ ] 删除历史页面数据前，导出只读报告和备份方案；任何生产数据删除另行取得明确确认，禁止自动执行。

验收：删除清单逐项有 owner、替代模块、测试和删除 PR；无”暂时保留”项。

### P1. FunctionContract 与 CapabilitySemantics

状态：待重新审核。

#### P1-1. 替换 descriptor 核心 DTO

- [ ] 在 proto、SDK、OpenAPI Source、DB 中定义 `capability` 和 `execution` 的受控枚举。
- [ ] 将代码中的 `FunctionExecution` 和数据库 `Execution` 收敛为 `sync|task`；审批改为独立 `ApprovalPolicy` 字段，禁止继续接受或存储 `execution=approval`。
- [ ] 在注册边界校验 `functionId`、`resourceKey`、`operationKey` 使用稳定小写 key（`[a-z0-9][a-z0-9._-]*`）；不符合格式必须返回结构化错误，避免 proposalKey/pageKey 和动态分类出现不可路由或不可复现的身份。
- [ ] 保留 `resourceKey/operationKey/risk/permission/inputSchema/outputSchema`，删除注册侧页面扩展及其所有解析、透传、测试和文档。
- [ ] 为各 SDK 建立 capability 支持矩阵；不支持时明确失败或标记未支持，禁止无声丢弃。
- [ ] 注册边界严格拒绝 UI、页面、mapping、分页、列、任务路径、图表路径与多语言页面显示字段。
- [ ] 声明 JSON Schema 受支持子集：object/array/scalar、`required`、`enum`、format hint、本地 `$defs`/`$ref`；`oneOf`/`anyOf`/`discriminator`、远程 `$ref` 不影响注册，但必须标注为后续生成降级来源。

验收：SDK/OpenAPI 只能构建 FunctionContract；任意被禁止字段返回结构化错误；所有官方 SDK demo 可注册至少一个新模型示例。

#### P1-2. 持久化能力和语义

当前本地事实（2026-08-03，本轮验证）：

- 已实现并通过目标测试的主链路：注册写入规范化 `FunctionContract`，`SourceDigest` 为完整 SHA-256；注册后会继续重建对应 `CapabilitySemantics` 与 `PageProposal`。
- `ResourceCapability` 不再由注册侧填充 labels；基础语义会从 collection output schema 推导 `items/total/page/page_size` 默认字段和最小可用 identity。
- 仍未完成：字段级 `SemanticProvenance`、多来源冲突持久化与优先级收敛、Resource Catalog 管理端、OTel/audit 完整闭环；因此本工作包不能勾选完成。

- [ ] 新建 scope 化 `function_contracts`、`resource_capabilities`、`capability_semantics`、`capability_semantic_versions` 数据模型；新表和新增列以 GORM model + AutoMigrate 定义（边界 13），并同步更新 `database/schema.sql` 参考文档。旧表/列删除不在此阶段执行，统一进入 P7-b 的确认后 GORM Migrator 清理。
- [ ] Function 注册/Source 更新后异步或事务内重建对应 scope 的 FunctionContract；不再由 Resource API 请求时临时拼装唯一事实。
- [ ] 保存有效语义、字段级 `SemanticProvenance`、sourceDigest、版本、诊断、更新时间和操作者；所有变更审计、OTel。不得以单一 `source` 字段掩盖多个来源或冲突。
- [ ] Resource Catalog API 读取持久化聚合，并展示”已识别、待确认、冲突、不可执行”。

验收：同一函数更新后可查询旧/新语义版本和来源；不同 game/env 不串数据；重启 server 后资源语义不依赖内存注册表才能解释历史 Proposal。

#### P1-3. OpenAPI REST 语义分类器

- [ ] 从 method/path/path parameter/request/response schema 分类 `collection_query/item_query/create/update/delete`。
- [ ] 明确 collection response、identity field、分页参数、响应对象的置信度规则和 diagnostics。
- [ ] REST 规则只生成 capability semantic，不生成列、按钮位置、最终 mapping 或菜单。
- [ ] path/schema 不完整时只产出低置信度 `action` semantic 和 diagnostics，绝不凭 operationId 名称猜 CRUD；`ready`/`basic`/`needs_review` 是 PageProposal quality，只能由 P2 生成器判定，不得出现在 CapabilitySemantics 层。

验收：`/players`、`/players/{playerId}` 的标准 OpenAPI 生成 CRUD 语义；非标准 REST 和普通 SDK 函数不被误判。

#### P1-4. SDK 显式语义与 Resource Catalog 补充

- [ ] SDK builder 支持受控 `capability`，并与 proto/所有语言 SDK 对齐。
- [ ] Resource Catalog 为管理员提供语义补充：identity、collection、生命周期 capability 绑定、task/report 数据语义；独立版本、权限、审计。
- [ ] 补充不是 Page Studio，不包含导航、列、表单布局或动作位置。
- [ ] 语义来源冲突优先级固定为 `platform_review > sdk_explicit > openapi_rest`；按该顺序计算字段级有效语义，但冲突必须保留结构化 diagnostics，并触发受影响 ResourceCapability 与 PageProposal 重算。
- [ ] 存在未解决语义冲突时，受影响 Proposal 必须降级为 `needs_review` 且禁止发布；管理员以版本化 `platform_review` 明确选择后才消除冲突。所有 Proposal 本就需要用户显式接受并发布，不得使用“自动发布”作为冲突控制表述。

验收：纯 SDK Resource 能通过显式 capability 或 Catalog 补充生成 CRUD Proposal；没有补充时仍生成 basic Operation Proposal。

### P3-a. 强类型 PageSpec、FormPresentation 与静态校验

状态：待重新审核。

> P3-a 是 P2 的硬前置。任何 P2 Proposal、Page Studio 保存或发布 API 在 P3-a 的 DTO、selector 和服务端静态校验闭环前都不得上线。P3-b 的 stale/diff/三方合并依赖 P2 已持久化 Proposal，因此不得提前实现。

#### P3-a-1. Canonical PageSpec DTO 与数据库

- [ ] 定义 Go/TypeScript 一致的 discriminated union：ResourcePage、OperationPage、TaskPage、ReportPage。
- [ ] 定义 NavigationSpec、QueryViewSpec、ListViewSpec、DetailViewSpec、FormActionSpec、ConfirmActionSpec、TaskViewSpec、ReportViewSpec、ResultViewSpec，以及 pagination、identity、action subject、taskId、dataset/dimension/metric 的强类型引用；`FormFieldSpec` 必须包含 `visibleWhen?: ConditionSpec`（与 P3-b 合并规则一致）。
- [ ] 定义 `FormPresentationSpec`，以 JSON Schema + 受控 widget hints 表达表单显示。
- [ ] `page_specs/page_versions` 按 scope 写入 canonical PageSpec、`pageSpecVersion` 和完整 FormPresentation snapshot；历史数据不转换。PublishedPageSpec 的冻结、stale 与发布版本表由 P3-b 在 P2 Proposal 可用后实现。

验收：核心 DTO 不含 `any`、`interface{}`、任意 JSON props、组件树字段或注册侧页面扩展。

#### P3-a-2. Typed selector AST 与静态校验

当前本地事实（2026-08-04，本轮验证）：

- selector DTO 已改为文档要求的形状：`InputAssignment.target/source.path` 使用 JSON Pointer，`ValueSource.kind` 替代旧 `source.type`，`page_state` 显式携带 `key`，`BindingSelectors.output` 使用独立 `OutputAssignment[]`。
- 服务端发布校验已覆盖 input selector required/path/type 和 output selector source/stateKey/shape；旧点分 path 会被拒绝。`ValidateOutputAssignments` 现在还会校验 output shape 与 schema 对齐：`collection/dataset` 必须映射数组，`object/task` 必须映射对象。
- 已新增 `SelectorContextForBinding`：发布校验会区分 query/create/update/report/task 的 formSchema，并对 ResourcePage 注入 `rowSchema`；update/delete/row action 的 row selector 不再被当作普通 form selector 误判。
- 已新增 `testdata/dashboard_selector_vectors.json` 作为前后端共享 selector 协议向量；Go spec 测试和前端 Jest DTO 测试均读取同一份向量。
- 已新增 schema field diff、低置信 field rename candidate 和 selector stale diagnostics helper；freshness 诊断会在 schema digest 变化时附带 selector 失效原因。
- 仍未完成：CapabilitySemantics 语义校验、Page Studio 中的 selector 冲突处理体验和浏览器 E2E 还没有闭环，P3-a-2 不能勾选完成。

- [ ] 以 typed selector AST 替换 binding 任意映射 JSON object；AST 形状必须与 `docs/architecture/ui-schema-spec.md` 对齐：path 为 JsonPointer、`page_state` 为 `{key, path?}`、`OutputAssignment` 含 `stateKey`/`shape`。现有实现（点分 path、无 key、输入输出复用同一 AST）若与文档不一致，必须先修正实现或修正文档，不得并存。
- [ ] 支持来源：form、row、selection、detail、page_state、literal；禁止任意 JSONPath 和 undefined source。
- [ ] 根据 FunctionContract JSON Schema、CapabilitySemantics 和页面状态进行路径/类型/required 校验。
- [ ] 支持 field rename、schema diff 和 selector stale diagnostics。

验收：非法路径、整行盲传、类型不匹配、缺少 required assignment 在保存/发布前可读报错；前端和后端共享同一 selector 行为测试向量。

### P2. PageProposal 生成器

状态：待重新审核。

> 硬前置：P3-a 的 canonical PageSpec DTO、selector AST 和服务端静态校验必须已经通过目标测试。P2 只能生成已被 P3-a 校验器接受的 PageSpec，不得在生成器中另建页面结构或 mapping 规则。

当前本地事实（2026-08-04，本轮验证）：

- 后端已打通一条受控的 Proposal 主链路：`FunctionContract + CapabilitySemantics -> PageProposal`，并已通过目标 Go 测试与 API 集成测试。
- `proposalKey/pageKey` 已收敛为 `resource:<resourceKey>` / `<kind>:<functionId>` 与 `resource--<resourceKey>` / `<kind>--<functionId>`；重复生成走稳定 key，不再沿用 `<resource>.manage` 或 `<resource>.<operation>`。
- `blocked` 已从 Proposal quality 中移除；当前阻断接受/发布依赖 error diagnostics，而不是 quality 枚举。
- 已实现发布时同 scope、同 `category.key` 的 `category.labels` 完全一致性校验；冲突会阻断发布，避免动态菜单分类文本由运行时仲裁。
- 已实现 BlockedProposalIssue 物化：`spec.BlockedProposalIssue` 类型、GORM 模型、`ShouldBlockProposal` 和 `CreateBlockedProposalIssue` 函数。
- Resource generator 现在会为 `ListViewSpec` 生成 `identityKey`、`rowSchema`、默认筛选字段候选，并且只在输入契约同时存在 `page/page_size` 时启用分页；collection query selector 会把函数输入的 `page/page_size` 映射到前端查询上下文的 `current/pageSize`。
- Resource generator 现在还会从 `item_query` 或 collection item schema 生成 `DetailView`；只有当 `item_query` 的输入可以仅靠当前行 identity 填满、且输出根对象可静态验证 identity 字段时，才会额外生成 `detail` binding，并要求它把对象结果映射到 `pageState.detail`。`update` 的默认入口也已从无效的顶层 `resource.actions` 收敛到 `listView.rowActions`，避免生成后前端没有编辑入口。
- Resource Catalog 已提供 `actions` 语义录入入口：只保存 `functionId`、`subject(resource_item/resource_selection/none)` 和 `identityInput`，不保存按钮位置、菜单、标题或页面 UI。后端会校验 action 函数属于当前 resource 且 capability=action，`resource_item/resource_selection` 必须提供 JSON Pointer identityInput，保存后触发 Proposal 重算。
- Resource generator 已开始消费持久化 `CapabilitySemantics.actions`：当前只接受当前格式的 `functionId/subject/identityInput` 数组，不再解析旧 nested action 兼容格式；只保守内联可静态证明安全的动作，`resource_item` 映射为 row action，`resource_selection` 映射为 batch action，`none` 映射为 toolbar action；无法静态证明 selector 安全的动作仍保留独立 `OperationPage`。Proposal 重建时也会跳过已被 ResourcePage 吸收的非 CRUD 动作函数，避免同一函数同时生成资源内按钮和独立菜单。
- Operation/Task generator 已从 output schema 生成结构化 `ResultViewSpec.fields`；Report generator 已从 array dataset output schema 推导 dimensions、metrics 和默认 chart，推导失败仍降级 `needs_review`。Resource/Report/Operation/Task 发布校验已收紧为 renderer ABI：ResourcePage 必须有 `listView.identityKey` 且引用实际列；Resource query 必须把集合结果映射到 `pageState.items`；ReportPage 发布前必须有 `dataset.dimensions`、`dataset.metrics`，且 Report binding 必须把数组结果映射到 `pageState.dataset`；Operation/Task 的 resultView 字段一旦声明必须具备 key/title/type。Task retry 没有真实 retry function 语义前禁止发布 `retryable=true`，前端也不展示假重试入口。
- P3-a 已有局部实现：Canonical PageSpec DTO、Typed selector AST、发布校验、schema field diff 和 selector stale diagnostics 已落地；CapabilitySemantics 语义校验与 Page Studio 中的 selector/语义冲突处理仍未完成产品闭环。
- P3-b 已有后端雏形：ThreeWayMerge、安全集/冲突集类型与部分服务入口已存在；Draft/Published stale 队列、冲突确认 UI、回滚/重新发布真实闭环仍未验收。
- 仍未完成：Task retry function semantic、审批通过后继续执行的闭环、Task/Report/Approval 的真实浏览器路径、P3-a/P3-b 产品闭环和浏览器 E2E；因此 P2 仍不能勾选完成。

#### P2-1. Proposal 数据模型和生成作业

- [ ] 新建 `page_proposals`、`page_proposal_versions`，按 `(game_id, env, proposal_key, version)` 隔离；proposalKey 固定为 `resource:<resourceKey>` 或 `<kind>:<functionId>`，pageKey 固定为 `resource--<resourceKey>` 或 `<kind>--<functionId>`，source key 限制为 `[a-z0-9][a-z0-9._-]*`，禁止由 title、labels 或本次输出随机决定。
- [ ] Proposal 记录 FunctionContract/CapabilitySemantics/generator source digest、generator version、质量、诊断、生成时间。
- [ ] 注册、OpenAPI Source update、Catalog semantic update 触发增量重算；页面工作台只读取持久化 Proposal。
- [ ] 相同输入摘要重算结果必须字节级稳定；为生成器加入 golden tests。

验收：重复生成不产生随机 diff；Proposal 可以显示“因何变化”；Proposal 从不覆盖 Draft/Published 页面。

#### P2-2. Resource CRUD 模板

- [ ] 按 Collection + Identity 生成 ResourcePage Proposal；只有 collection/identity 时生成只读 ResourcePage，create/update/delete 是可选生命周期能力，不得因缺少写操作降级为 OperationPage。
- [ ] ListView 从 output JSON Schema 提取字段候选、可展示类型、默认筛选、分页候选；无可靠 collection/identity 时不生成 ResourcePage。
- [ ] DetailView 从 item query 或 collection item schema 生成描述字段候选。
- [ ] create/update 使用 input schema 生成 FormPresentationSpec；delete 生成受风险/审批约束的 ConfirmAction。
- [ ] action 只按 `ActionSemantic.subject` 确定性生成 row、batch 或 toolbar 候选；`resource_item`/`resource_selection` 的 identity input 不能静态验证时只生成独立 Operation Proposal，`none` 不要求 identity input；不得猜测页面位置。
- [ ] input/output schema 超出受支持子集（`oneOf`/`anyOf`/`discriminator`、远程 `$ref`，见 P1-1）时，表单与列生成降级 `needs_review` 并记录 diagnostic，不得生成不可用的默认页面。

验收：OpenAPI REST `players` 生成可直接发布 ResourcePage；列表、详情、create/update/delete 和 `ban` 行操作的选择理由可展示且类型可验证。只读资源生成可直接发布的查询/详情页面，不出现虚构的写操作。

#### P2-3. Operation、Task、Report、Approval 模板

- [ ] 同步非 CRUD 函数生成 `basic` OperationPage：表单、确认、受控执行、结构化 ResultView。
- [ ] task semantic 生成 TaskPage，要求 start/status/events/result/cancel 的真实 API 语义；retry 必须由显式 retry function semantic 提供，缺失时不得显示或发布 retry 入口。
- [ ] report semantic 生成 ReportPage，要求 dataset、dimension、metric、chart/table 的类型化语义；缺失时 needs_review。
- [ ] `approval.required: true` 为 OperationPage 或 TaskPage 生成明确等待态、审批状态刷新和审批通过后的同步/任务结果；禁止把“已提交审批”显示为完成。

验收：`mail.send`、`reward.batchGrant`、`analytics.retention` 都生成独立且真实可运行的 Proposal；没有占位 JSON 面板。

#### P2-4. 质量与直接发布规则

- [ ] `ready`：页面所有已声明能力均有可验证 binding、selector、navigation labels、权限、风险、页面节点和 renderer ABI；ResourcePage 的 create/update/delete/action 是可选能力，collection + identity 的只读 ResourcePage 满足上述条件时也可直接发布。
- [ ] `basic`：安全同步 OperationPage 完整，可带 `approval.required: true`，并具有可验证的等待态与结果路径；可直接发布。
- [ ] `needs_review`：可预览但必须确认/补充语义；不可发布。
- [ ] 不可物化的问题（函数不可执行、schema/selector 违法、权限或 scope 不安全）生成 BlockedProposalIssue，只保存诊断与修复指引；`blocked` 不是 Proposal quality，不得要求 spec。
- [ ] 所有自动 Proposal 仍需用户显式”接受并发布”；禁止注册后自动上线菜单。
- [ ] 默认 labels 分三套规则：ResourcePage `title` 取 humanize `resourceKey`；Operation/Task/Report 的 `title` 取主 binding 的 `summary[systemDefaultLocale]`，缺失时 humanize `operationKey`，再缺失时 humanize 原始 `functionId`；`category.key` 对 ResourcePage 取 `resourceKey` 的第一个 `.` 前缀、对独立页面取原始 `functionId` 的第一个 `.` 前缀（无 `.` 时取完整 key），`category.labels` 取该 key 的 humanize 结果。显式 labels 只能来自 PageSpec/Page Studio 人工编辑，SDK/OpenAPI 注册不得提供 labels。系统默认语言必须在生成阶段补齐，否则 Proposal 不得标记 `ready`/`basic`。
- [ ] `LocalizedText` 的显示顺序固定为当前界面语言、系统默认语言；生成器只保证系统默认语言，Page Studio 可补充其他语言。此 fallback 只读取 PublishedPageSpec 文本，不得访问静态 locale/字典。
- [ ] 发布时校验同 scope 内相同 `category.key` 的 `category.labels` 完全一致；不一致则发布失败，由管理员在 Page Studio 统一后重发。

验收：质量判断由后端唯一计算；前端不可自行提高质量或绕过发布校验。

### P3-b. 发布快照、stale 与三方合并

状态：待重新审核。

> 硬前置：P2 Proposal 生成、持久化和 source digest 追溯已通过目标测试。P3-b 只消费 P2 Proposal 与 P3-a PageSpec/selector 校验器，不得在变更流程中重新实现生成规则。

- [ ] 新建 `published_page_specs/page_versions`，冻结 page spec、form presentation、function contract digest、semantic digest、risk、permission、execution、approval、renderer/generator version。
- [ ] 变化产生新 Proposal，比较 base Proposal、当前 Draft、最新 Proposal，输出自动合并项和冲突项。
- [ ] 自动合并安全集仅限展示字段：列顺序与显隐、字段 label/help、order、group、widget hint、导航标题、分类 labels、图标、排序。`visibleWhen` 只有经校验证明不影响 required 输入、binding payload 和 selector 引用时才允许自动合并，否则归入冲突集。
- [ ] 冲突集必须显式确认，禁止自动合并：bindings、functionId、input/output assignment、selector、confirmation、permissions、risk、identity、execution、approval。
- [ ] stale 页面继续显示菜单和诊断，但 binding execute 必须拒绝执行，直到重新发布。

验收：函数新增 optional 字段、删除 required 字段、风险提高、审批要求变化、identity 变化、列表字段变更都有明确 diff 和可测试结果；不发生静默覆盖。

### P4. ProComponents 运行时与表单适配器

状态：待重新审核。

#### P4-1. JSON Schema Form Renderer 选型与 POC

- [ ] P4-1 的选型和 Formily 清理已前移到 P0-0；本阶段只接入已选定的 `@rjsf/antd + @rjsf/validator-ajv8` renderer 到 ResourcePage/OperationPage/TaskPage/ReportPage。
- [ ] 前端 JSON Schema 校验入口必须基于 P0-0 选定 runtime 的 Ajv/validator；服务端仍必须做最终 JSON Schema 校验，禁止前端独自解释 payload。
- [ ] 支持 QueryForm、Modal/Drawer 中的 create/update/action 表单；函数调用测试与 Page action 复用同一 renderer。

验收：复杂 input schema 的函数调用、创建、编辑、查询和动作弹窗使用同一 renderer；不含 Formily runtime import；不持久化 renderer 私有 UI 配置；前后端校验结果一致。

#### P4-2. ResourcePageRenderer

- [ ] `ProTable` 实现查询、分页、列设置、筛选、空态、错误态、刷新、批量选择和 toolbar。
- [ ] `ProDescriptions` 实现详情；`ModalForm/DrawerForm` 实现 create/update；`Popconfirm/Modal.confirm` 实现 delete/high-risk action。
- [ ] 每个行为通过 binding execute API；数据状态按 page instance/binding 隔离。
- [ ] 对所有选择、详情、列表状态使用 typed selector，禁止 `lastResult` 或整行隐式数据总线。

验收：一个真实 Resource CRUD 页面不需要页面特例代码即可完成 list/detail/create/update/delete、分页和 row action。

#### P4-3. Task/Report/Approval Renderer

- [ ] Task renderer 对接真实 task status/events/result/cancel API；retry 必须等显式 retry function semantic 和真实 API 存在后再启用，禁止假重试按钮。
- [ ] Report renderer 接入 `@ant-design/charts` 或确认的 AntV renderer，按 ReportViewSpec 渲染 line/bar/pie/table；无真实数据集不得发布。
- [ ] Approval renderer 显示 pending/approved/rejected/expired 和后续结果，不以 API 返回成功替代业务完成。
- [ ] 移除所有 `最小实现`、`JSON.stringify` 结果面板作为 Task/Report 正式 renderer。

验收：浏览器 E2E 验证真实 task events、真实图表和审批状态。

#### P4-4. 删除旧 Page runtime

- [ ] 删除旧 Page renderer、运行 Page 的旧 registry、Page schema validator、PageSchemaEditor 和相应 API/schema 字段。
- [ ] 删除旧表单/页面 runtime 依赖；如没有其他运行用途则完全删除，禁止保留。
- [ ] 删除 `form-render` 未使用依赖；不允许引入第二个 schema runtime。
- [ ] CI guard 阻止旧 Page runtime、注册侧页面扩展、组件树页面协议回流，并永久阻止第二套表单 runtime 回流。
- [ ] 旧 form 路径物理删除；历史表单数据只允许导出、备份和在 Page Studio 人工重建，不提供导入或自动转换桥。

验收：Dashboard PageSpec guard 通过；前端 build 与 E2E 通过。

### P5. Page Studio 与 Resource Catalog 产品化

状态：待重新审核。

#### P5-1. Resource Catalog

- [ ] 以持久化 ResourceCapability/CapabilitySemantics 展示资源、函数、识别来源、置信度、诊断和变更历史。
- [ ] 提供管理员语义补充表单，但不允许编辑页面 UI。
- [ ] 提供 Proposal 入口和受影响页面列表。

验收：用户可以理解“函数属于什么资源、是否能组成 CRUD、为何不能生成”，无需查看原始 JSON。

#### P5-2. Proposal Inbox 与直接发布

- [ ] Page Studio 首屏分三队列，而不是空白 PageSpec 列表：可直接发布（`ready`/`basic` Proposal）、需要处理（`needs_review` Proposal 和 BlockedProposalIssue）、契约变更（source digest 变化导致 stale 的 Draft/PublishedPageSpec）。`stale` 和 `blocked` 都不是 Proposal quality，不得混入 quality 枚举或模型字段。
- [ ] ready/basic 支持预览、接受、发布；needs_review 打开对应缺失语义/页面配置步骤；BlockedProposalIssue 只展示诊断与修复指引。
- [ ] Resource 页面和 Operation/Task/Report 页面都有明确入口，不把所有候选塞入一个资源抽屉。

验收：新注册 `mail.send` 后，用户无需写 JSON：找到 basic Proposal -> 预览 -> 发布 -> Console 菜单出现 -> 可执行。

#### P5-3. 语义化页面编辑器

- [ ] ResourcePage 面板：导航、列表/筛选、列、详情、create/update/delete、row/batch/toolbar actions、权限。
- [ ] OperationPage 面板：表单、确认、结果、权限。
- [ ] TaskPage 面板：启动参数、任务状态、事件、取消/重试、结果。
- [ ] ReportPage 面板：查询、dataset、维度、指标、图表、表格、导出。
- [ ] 所有编辑器读取强类型 DTO；高级 JSON 仅导入/导出/诊断，需单独权限且修改后仍经过同一校验。

验收：正常操作不展示原始 PageSpec JSON、第二套表单 schema 或自定义 mapping JSON；所有配置可通过选择器完成。

#### P5-4. 变更处理与版本体验

- [ ] 展示 FunctionContract/CapabilitySemantics/Proposal/Draft/Published 的差异链。
- [ ] 提供自动安全合并、逐冲突决策、回滚草稿、回滚发布、重新生成 Proposal、重新发布。
- [ ] 任何覆盖操作都显示影响 binding、菜单和执行状态；使用乐观 revision 防止并发覆盖。

验收：契约变化后用户可在 UI 解决冲突，无需手写 JSON 或猜为何 stale。

### P6. Console、权限、审计与 OTel 收口

状态：待重新审核。

- 当前事实（2026-08-05）：Console execute 已切到 `context` 协议，浏览器只提交 form/row/selection/detail/pageState 来源值；服务端在 stale/snapshot 校验通过后，按 PublishedPageSpec 的 typed selector 构造函数 payload，再用当前 FunctionSpec input schema 做最终 JSON Schema 校验。ResourcePage renderer 已停止裸 payload/整行透传；query/create/operation/task/report 传 `form`，update/delete/row action 传 `row`，batch action 传 `selection`；Console selector 现在仅支持受控 `selection + pick` transform，把选中行 identity 提取为数组输入，禁止开放任意 transform。ResourcePage 生成器会把 update/delete/detail 的 identity selector 切到 row 来源，并从 update form 中剔除 identity 字段，避免用户编辑一个不会生效的字段。ResourcePage renderer 现在使用 `listView.identityKey` 作为 rowKey，详情入口会优先执行 `detail` binding 并只读取 selector 映射后的 `pageState.detail`；列表结果只读取 `pageState.items/total`，并已补上 batch/toolbar action 的基础运行时。Operation/Task/Resource 详情结果不再把原始 JSON 作为正式面板，只按 ResultView/DetailView 字段展示结构化摘要；Task retry 因缺少真实 retry function semantic 已在 renderer/editor/publish guard 中禁用。ReportPage renderer 不再猜 `response.data.items`，缺少 dataset 语义或 output selector 时直接显示配置错误。已验证 `GOCACHE="/tmp/croupier-go-build" GOMODCACHE="/tmp/croupier-go-mod" go test ./internal/dashboard/generator ./internal/dashboard/spec ./internal/api/console ./internal/api/resourcecatalog ./internal/service -run '^$'`；前端目标 eslint 在当前受限环境下被 `pnpm` 触发重装依赖后卡在网络拉包，不能作为完成证据。全量 `cd "web" && pnpm exec tsc --noEmit` 仍被仓库既有 `@umijs/max` 类型导出与旧页面 implicit any 阻断，不是 P6 完成证据；approval/task dispatch、OTel collector E2E、真实浏览器路径和选择/详情/page_state 复杂上下文仍未闭环。

- [ ] Console 只读取当前模型 PublishedPageSpec 和 ConsoleMenuSpec；路由仍为 `/console/:categoryKey/:pageKey`。
- [ ] ProLayout 动态菜单使用 NavigationSpec labels 与 `locale:false`；切 scope 后强制失效旧 menu/page query。
- [ ] 菜单分类仲裁：分类 order 取该分类下已发布页面 order 的最小值；运行时菜单只读已发布的 `category.key`/`category.labels`，不推断分类（同 key labels 一致性发布校验见 P2-4）。
- [ ] execute API 校验 PageSpec binding、selector payload、snapshot、permission、risk、approval、task dispatch 和 stale。
- [ ] span/audit 统一记录 scope、pageKey、publishVersion、bindingId、functionId、semantic digest、proposal version、target、result kind、taskId/approvalId；不记录敏感 payload。
- [ ] OTel collector 环境验证 trace 和 audit 的关联字段，不只依赖本地 span recorder。

验收：伪造 page/binding/function/target/scope 请求全部失败；真实调用可从 UI 点击关联至 trace、audit、task/approval。

### P7-a. 删除清单与防回流 guard（随 P0 先行）

状态：待重新审核。

- [ ] 建立旧模型文件、路由、DTO、API、数据库列的完整删除清单与调用图，逐项标注替代路径、owner 和删除前置条件。
- [ ] 先行上线 CI guard：旧模型任何新命中即失败，allowlist 只减不增。
- [ ] guard 同步拒绝业务代码中的原生 SQL：`internal/` 业务包中 `db.Raw(`/`db.Exec(` 命中即失败，allowlist 仅豁免 `internal/svc` 等基础设施层和测试文件，且豁免项只减不增（边界 13）。
- [ ] guard 同步拒绝 web 业务代码中未收窄的 `unknown`、`as any` 和页面/组件内重复定义的共享 DTO：以当前存量为 baseline 建立 allowlist，新命中即失败，allowlist 只减不增（边界 14）。

### P7-b. 按模块替换验收后物理删除

状态：待重新审核。

> 前置条件：被删模块的替代路径已通过该模块的真实浏览器 E2E。禁止在替代路径验收前删除仍在服务的旧模块，禁止一次性“大爆炸”删除。

- [ ] 删除注册侧页面扩展 DTO、OpenAPI extension、descriptor collector 解析、SDK/docs/demo、generator、测试。
- [ ] 删除组件树 PageSpec DTO、旧 Page renderer、Page schema validator/editor、旧模型 Page APIs 与数据库列。
- [ ] 删除历史 page spec 数据，不迁移；数据清理前备份并取得单独确认。
- [ ] 旧表/列物理删除只能由版本化清理函数调用 `db.Migrator().DropColumn/DropTable`，前置条件为替代路径 E2E、备份校验和明确确认；禁止依赖 AutoMigrate 或原生 SQL 隐式删除。
- [ ] 删除旧模型 `workspace_configs` 诊断/命令、对象页配置术语和防回流代码。
- [ ] 删除不再使用的旧页面/表单 runtime 依赖、锁文件项与构建配置。
- [ ] 更新 CI guard：旧模型命中必须失败，禁止 allowlist。

验收：`go test ./...`、web/docs build 与全量 E2E 不依赖任意旧模型文件、表、API 或 package。

## 4. 端到端验收矩阵

下面每个场景必须同时有 server integration test 和真实浏览器 E2E；任何“mock preview”不算验收。

| 场景 | 输入 | 预期 Proposal | 发布和运行验收 |
| --- | --- | --- | --- |
| OpenAPI CRUD | `/players` 与 `/players/{id}` 的标准 REST + schemas | ready ResourcePage | ProTable 分页、详情、create/update/delete、row ban action、动态菜单、受控执行 |
| SDK CRUD | 显式 `resource=inventory` + capability | ready ResourcePage | 与 OpenAPI CRUD 同等体验，不依赖 REST path |
| 只读资源 | collection + identity，无写 capability | ready ResourcePage | 查询、详情、筛选和可用分页，不出现虚构写操作 |
| 独立操作 | `mail.send`，只有 input/output schema | basic OperationPage | 无 JSON 编辑即可发布，表单/确认/结果可用 |
| 高风险动作 | `player.ban`，risk/approval | Resource action 或 OperationPage | 高风险确认、approval pending、审计/trace 可关联 |
| 异步任务 | `reward.batchGrant` + task semantic | ready/needs_review TaskPage | 真实 task event、失败、取消/重试、结果 |
| 报表 | `analytics.retention` + report semantic | ready/needs_review ReportPage | 真实图表/表格、筛选、空态和数据错误 |
| 契约变化 | 删除字段、改类型、提高 risk、改 identity | stale + 新 Proposal | 页面拒绝执行、diff/合并/重新发布闭环 |
| Scope 隔离 | 相同 pageKey 两个 game/env | 各自 Proposal/Published | 菜单、页面、版本和执行绝不串 scope |
| OpenAPI Source | 上传 -> provider binding -> CRUD/action | 与 SDK 同等 Proposal | 无 binding 不可执行/发布；绑定后进入完整闭环 |

## 5. CI 与质量门禁

### 强制检查

- Go：`go test ./...`、关键模型 migration/integration tests、race 适用测试。
- Web：`pnpm --dir "web" exec tsc --noEmit`、`pnpm --dir "web" exec eslint "src"`、单元测试、Playwright 浏览器 E2E。
- Docs：`pnpm --dir "docs" build`、术语/死链检查。
- SDK：capability contract/parity matrix tests；至少 Go/JS/Python/Java/C#/C++ 的编译与 descriptor 字段一致性检查。
- Security：禁止 browser 选择 target/scope/function、禁止任意 connector、禁止 trace/audit 记录完整 payload。

### Guard 必须拒绝

```text
注册侧页面扩展字段和对应 OpenAPI extension
旧 Page renderer、组件树页面协议、页面 schema editor
旧模型工作区/对象页配置 API 与 objectKey 页面身份
动态菜单静态 locale 或字典事实源
Page binding 的裸 functionId、route、target、game/env
核心 DTO 中的 TypeScript any 或 Go interface{}
web 业务代码中未收窄的 unknown、绕过类型的 as any、在页面/组件内重复定义共享 DTO
以 mock/JSON placeholder 声称 Task/Report 完成
旧模型转换桥、第二套 runtime、自动数据迁移
```

## 6. 完成定义

只有全部满足以下条件，这次重构才算完成：

1. 当前权威模型已在代码、文档、SDK 和数据库中一致实现。
2. OpenAPI REST 和 SDK capability 都可自动生成可发布 ResourcePage；声明了写 capability 时生成完整 CRUD，未声明写 capability 时生成只读 ResourcePage；非 CRUD 可自动生成可发布 Operation 页面。
3. 用户正常路径不写原始 PageSpec JSON 或自定义 mapping JSON。
4. Task/Report/Approval 是真实运行能力，不存在最小实现和 JSON 占位。
5. 运行菜单唯一来自 PublishedPageSpec，scope、权限、审计、OTel、stale 和发布版本均闭环。
6. 函数/语义变更可生成 Proposal diff 并由用户合并，绝不静默覆盖。
7. 已物理删除注册侧页面扩展、旧 Page runtime、旧模型工作区/对象页配置及所有转换桥代码和文档。
8. 全量单元、集成、浏览器 E2E、docs、SDK parity 和部署验证通过。

在此之前，禁止把任何阶段标记为“Dashboard 重构完成”或“可发布版本”。
