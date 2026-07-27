# Dashboard Resource/Page 重构 TODO

更新时间：2026-07-26（P0 主链路已推进到 Resource/Page/Console/OpenAPI Source 基础闭环；Page Studio 基础前端已接入；旧 Workspace/Entity/PageGenerator 前端协议残留已物理清理；dashboard CI guard 已接入；剩余重点是 Page Studio 可视化编辑、权限审计、真实数据端到端验收和 Source binding 完整闭环）

本文是 Dashboard 动态页面、函数注册描述符、Page Studio 和运行控制台菜单的重构交接清单。执行 AI 必须按本文推进；审核 AI 以本文和权威设计文档为验收依据。

权威设计文档：

- `docs/architecture/dashboard-page-model.md`
- `docs/architecture/openapi-sdk-descriptor-v2.md`
- `docs/design/console-dynamic-menu.md`
- `docs/api/resource.md`
- `docs/architecture/ui-schema-spec.md`

## 执行进度

| 阶段 | 状态 | 完成日期 | 备注 |
|---|---|---|---|
| P0-1 冻结强类型模型和命名 | ✅ 核心完成 | 2026-07-26 | Go/TS Dashboard 强类型已新增；旧 Workspace/Entity 类型入口已删除；Function/API 核心 DTO 已收敛为 `json.RawMessage`/明确结构；剩余 `map[string]interface{}` 仅允许在 JSON AST、metadata 解析和 GORM update 边界 |
| P0-2 收敛 SDK/OpenAPI 能力契约 | ✅ 核心完成 | 2026-07-26 | SDK/OpenAPI 注册边界只保留 resource/operation/risk 等能力契约；display/operationKind/placement/pageHint/labels/x-labels 已从注册主链路删除 |
| P0-3 建立 Descriptor Normalizer | ✅ 核心完成 | 2026-07-26 | normalizer 已输出 FunctionSpec/ResourceSpec/OperationSpec；Function detail/invoke/UI/history DTO 已强类型化；collector 仅保留输入解析边界 JSON AST |
| P0-4 Function Form 收敛为单一 Formily | ✅ 核心完成 | 2026-07-24 | 注册/OpenAPI Source 解析拒绝 UI 字段；Function UI API/前端只传 Formily schema；旧 params 推断已停止 |
| P0-5 Resource API 替换旧 Entity API | ✅ 核心完成 | 2026-07-26 | Resource API 已接 generator；前端主入口已切到 Resources；`/api/v1/entities`、旧 Entity model/API 和 XEntityForm 已删除 |
| P0-6 Page Studio API | ✅ 核心完成 | 2026-07-25 | PageSpec 已包含 scope、draftRevision、binding、mapping、发布 contract snapshot；旧 Workspace API/model 已删除 |
| P0-7 Console API 和动态菜单 | ✅ 核心完成 | 2026-07-25 | ConsoleMenuSpec 已作为运行控制台菜单来源；前端不再从 workspaceConfigs 注入菜单 |
| P0-8 前端 PageSpec Formily Renderer | ✅ 核心完成 | 2026-07-25 | Renderer 只使用 bindingId 和受控 execute API；旧 queryFunctionId/lastResult/onQuery/onAction 已切断；服务端 Page 组件 ABI 校验已接入并与运行期字段组件 registry 对齐 |
| P0-9 PageSpec Generator | ✅ 核心完成 | 2026-07-26 | generator 已按明确 `PageContract/x-page-contract` 生成 Entity/Operation/Task/Report 四类候选；无可验证 mapping/分页/列/任务/报表契约时只产出 diagnostics，不标 ready |
| P0-10 受控 Page 执行与契约失效 | ✅ 核心完成 | 2026-07-25 | Page binding execute 已接 active PublishedPageSpec、contract digest stale 检查、traceId 返回；task/approval UI 细节仍在 P1 |
| P0-11 OpenAPI Source 上传与执行绑定 | ✅ 基础完成 | 2026-07-26 | Source API、diagnostics、revision update、provider binding、Source binding 到 PageCandidate 生成、固定管理入口、前端只读/写入裁剪、服务层 RBAC、审计和管理 span 已完成；httpConnector 与真实 E2E 后续实现 |
| P1-1 Page Studio 前端 | ✅ 核心完成 | 2026-07-26 | 已新增 `/system/functions/pages` Page Studio 基础入口，支持 PageSpec 草稿列表、Resource 生成候选、PageCandidate 落草稿、JSON 编辑、基础信息与 binding 结构化编辑、Page schema 顶层组件结构化编辑、组件 props 表单化编辑、DataTable columns/rowActions 和 ActionGroup actions 专用编辑、服务端组件 ABI 校验、校验、预览、发布/取消发布、版本查看、版本 diff、版本回滚和 409 revision 冲突提示；剩余真实端到端验收放入 P1-2/P1-4 |
| P1-2 系统菜单和信息架构收敛 | ⏳ 进行中 | | Console 动态菜单已接 ConsoleMenuSpec；“函数与页面”已提升为独立顶层入口；旧注册函数静态菜单翻译和 Provider entities 路由已清理；仍需真实发布数据端到端验收和最终文案验收 |
| P1-3 权限和审计模型迁移 | ⏳ 进行中 | | Resource/Page/Console/OpenAPI Source 后端服务层权限已接入；Page 保存/发布/取消发布/回滚、Console Page 执行、OpenAPI Source 上传/绑定/解绑审计已落地；Console 执行和 OpenAPI Source 管理 span 已接入；前端 access 已收敛到 Page/Resource/Console/OpenAPI Source 权限；剩余真实 OTel collector 字段验收 |
| P1-4 数据表和历史数据处理 | ⏳ 进行中 | | PageSpec model 已接入 migration；旧 workspace_configs 不作为兼容来源，历史数据只能人工导出/清理，禁止自动发布为新 Page |

## 0. 硬约束

本次重构不是在旧 WorkspaceConfig 上继续打补丁，而是把 Dashboard UI 链路收敛为唯一模型：

```text
SDK / OpenAPI / DB Template
  -> RawFunctionDescriptor
  -> FunctionSpec
  -> ResourceSpec + OperationSpec
  -> GeneratedPageCandidate
  -> PageDraft
  -> PublishedPageSpec
  -> ConsoleMenuSpec
  -> PageExecutionResult
```

必须遵守：

- Function 不是 Page。函数注册只产生可执行能力契约；Server 在注册后派生单函数 Formily Function Form。
- Function Form 只负责单函数输入表单，不负责菜单、分页、表格、详情、页面布局。
- PageSpec 是唯一页面编排协议，`schema` 必须是 Formily JSON Schema；Function Form 与 Page UI 有各自明确根节点和校验器。
- 运行控制台左侧菜单唯一来源是 `GET /api/v1/console/menu` 返回的 `ConsoleMenuSpec`。
- 动态分类、资源、页面标题必须来自 PageSpec 的强类型 `category.labels/title`，不写入静态 locale 文件，也不藏在不受约束的 metadata。
- 函数注册只要求可执行能力契约；缺少可验证 PageContract、mapping 或默认语言 labels 不得阻断注册，只能降低 PageCandidate 质量。缺少最终 PageSpec、mapping、默认语言 labels 或函数 binding 时，发布必须失败。
- 没有 `input_schema` 时，Server 只能派生单个 `payload` 字段的 Function Form，禁止根据函数名猜 `player_id`、`reason` 等业务字段。
- SDK/OpenAPI 注册不接受 `ui/x-ui`、Formily、菜单、路由、表格列、动态显示名、`operationKind`、`placement`、`pageHint` 或页面组件配置；Function Form 由 Server 派生，最终 UI 只在 Page Studio 确定。
- 不保留多套 UI 协议，不兼容旧 `layout` 渲染协议，不新增字典作为动态菜单事实源。
- 不使用 TypeScript `any` 或 Go `interface{}` 承载核心 DTO；扩展字段必须使用明确的 JSON 类型别名或 `json.RawMessage`。
- 不把所有函数强行塞进对象管理页；必须区分 Entity Page、Operation Page、Task Page、Report Page。
- PageIdentity 固定为全局 scope 的 `game_id + env + pageKey`；数据库唯一键、API 查询、菜单和发布快照必须按该身份隔离，页面内不得二次选择 scope。
- Page binding 必须有稳定 `bindingId`、页面用法、输入/输出映射和执行策略；Schema 组件只能引用 `bindingId`，禁止裸 `functionId`。
- 发布必须冻结 binding 的函数契约摘要（版本、schema digest、风险、权限、执行模式、renderer schema version）；不兼容变更必须标为 stale 并拒绝执行。
- Page 运行必须走受控 binding 执行 API，不能用浏览器直接拼 `/functions/:id/invoke`、`route`、`targetServiceID` 或 scope。
- Formily 不等于无约束 JSON：Page 组件 registry 和每个 `x-component-props` 都必须有版本化、服务端可校验 ABI；未知组件/props/schemaVersion 直接报错。

禁止继续维护的旧概念：

- `WorkspaceConfig.layout`
- `objectKey` 作为页面主标识
- `workspace_configs` 作为运行控制台菜单来源
- `/api/v1/workspaces/:objectKey/config`
- `/api/v1/workspaces/published`
- 旧实体 CRUD API 作为 Dashboard 页面模型
- `menu.ControlConsole.category.*` 动态分类静态 locale key
- `x-operation = create/read/update/delete/custom` 这类混用定义
- `metadata.menu`、`defaultMenu`、`termDict` 作为菜单主模型
- `PageFunctionBinding.role = OperationPlacement` 这种把注册期 placement 误作运行期 binding 语义的模型
- 运行时从“最近一次任意函数结果”、动态首批数据或裸 row JSON 猜数据映射

## 1. 当前问题快照

### 1.1 后端 Descriptor 已进入新模型但仍有弱类型边界

现状：

- `proto/croupier/sdk/v1/provider.proto` 的 `LocalFunctionDescriptor` 已收敛为 `resource/operation/risk/enabled/permission` 等能力契约字段，不包含分类显示、菜单、页面类型或页面放置。
- `internal/dashboard/normalizer` 已输出强类型 `FunctionSpec/ResourceSpec/OperationSpec`。
- `internal/logic/function/descriptors_logic.go` 的 V2 路径已返回强类型 `FunctionSpec`；`internal/logic/function/types.go` 的 Function detail、descriptor、invoke、UI、history 核心 DTO 已收敛为 `json.RawMessage`、`bool`、`string` 或明确结构。
- `internal/dashboard/descriptors/collector.go`、`internal/logic/function/ui_resolver.go` 和 Formily/OpenAPI schema 校验仍在解析 JSON metadata/extensions 边界使用 `map[string]interface{}`；这是 JSON AST 输入边界，不允许泄漏到核心 API DTO。
- 注册/导入边界已经拒绝 `x-operation-kind/x-placement/x-*-display/page_hint/ui/x-ui` 等越界字段；需要补 CI guard 防止回流。
- `term_dictionary` 已改为 `resource/operation` domain，旧 `entity` domain 会报错；字典只提供别名/展示提示，不参与动态导航事实源。

目标：

- 后端必须先归一化为强类型 `FunctionSpec/ResourceSpec/OperationSpec`，再生成 PageSpec 建议。
- 前端不再读取原始 OpenAPI 或函数 ID 推断页面。

### 1.2 旧 Workspace / Entity 页面模型已删除，Page Studio 已切到新模型

现状：

- `internal/api/workspace/*`、`internal/model/workspace_config.go`、`web/src/types/workspace.ts`、`web/src/services/workspaceConfig.ts`、`web/src/components/WorkspaceRenderer/*` 已删除。
- `internal/api/entity/*`、`internal/model/entity.go`、`internal/model/entity_model.go`、`web/src/pages/Entities`、`web/src/components/XEntityForm.tsx` 已删除。
- `web/src/pages/WorkspaceEditor/*` 已删除；当前 Page Studio 使用 PageSpec 草稿、预览、发布、版本、diff 和回滚链路，不恢复或包装旧 layout editor。
- Page Studio 前端基础闭环已接入，剩余工作是 PageContract/mapping 辅助编辑、真实端到端验收和体验细节。

目标：

- Page Studio API 使用 `PageSpecDraft`，运行态使用 `PublishedPageSpec`。
- Resource API 只提供资源/操作归一化查询和诊断，不做通用实体 CRUD。

### 1.3 运行控制台菜单已接 Server 产物，仍需端到端验收

现状：

- `web/src/app.tsx` 已通过 `GET /api/v1/console/menu` 合并 `ConsoleMenuSpec` 动态菜单，不再加载 `workspaceConfigs`。
- `web/config/routes.ts` 已使用 `/console/:categoryKey/:pageKey`。
- `web/src/pages/Console/Page.tsx` 已通过 `getPublishedPage(pageKey)` + `FormilyPageRenderer` 渲染已发布 PageSpec，并通过 `executePageBinding` 执行 binding。
- 仍需用真实 PublishedPageSpec 数据验收：发布后菜单出现、取消发布后消失、切换 scope 后菜单刷新、URL 分类不一致时跳转规范路径。

目标：

- 前端只消费 `ConsoleMenuSpec`。
- 动态菜单项 `locale: false`，直接使用当前语言解析出的 label。
- 路由改为 `/console/:categoryKey/:pageKey`，页面由 `PublishedPageSpec` + Formily 渲染。

### 1.4 SDK Demo 和 OpenAPI 类型边界仍需全量验收

现状：

- Go/JS/Python demo 已改为 `resource/operation` 能力契约；仍需逐一验收 Java/C#/C++ 示例、README 和 parity matrix。
- Go SDK 已提供 OpenAPI helper；其他 SDK 是否支持直接解析 OpenAPI 必须在 `sdks/SDK_FEATURE_MATRIX.md` 中真实标注，不得文档泛称“全部支持”。
- OpenAPI Source 上传、更新、diagnostics 和 Provider binding 已存在；仍需验收 Source/FunctionSpec 候选到 PageCandidate/Page Studio 发布的真实端到端路径。

目标：

- proto、SDK builder、OpenAPI Source 解析、demo、文档只表达函数能力契约。
- `operation` 表示业务动作 key；页面类型、组件位置、菜单分类和多语言标题只在 PageCandidate/PageSpec/Page Studio 中确定。

## 2. 目标模型定义

### 2.1 FunctionSpec

职责：

- 表示单个注册函数的可执行能力。
- 承载输入/输出 JSON Schema、风险、简介和业务归属 key。
- 提供单函数 Formily 输入表单。

必填字段：

```ts
type LocaleCode = 'zh-CN' | 'en-US' | string;
type LocalizedText = Record<LocaleCode, string>;
type JSONValue = null | boolean | number | string | JSONValue[] | { [key: string]: JSONValue };
type JSONSchema = { [key: string]: JSONValue };
type FormilySchema = JSONSchema;

interface FunctionSpec {
  id: string;
  version: string;
  enabled: boolean;
  inputSchema: JSONSchema;
  inputFormilySchema: FormilySchema;
  outputSchema?: JSONSchema;
  summary?: LocalizedText;
  description?: LocalizedText;
  resource?: string;
  operation?: string;
  risk?: RiskLevel;
  tags?: string[];
  diagnostics?: Diagnostic[];
}
```

约束：

- `inputFormilySchema` 必须是 Formily JSON Schema。
- `inputFormilySchema` 是 Server 派生或管理员 override 的结果，不是 SDK/OpenAPI 注册字段。
- `summary/description` 只能用于函数目录、搜索和候选说明，不能作为运行菜单、页面标题、按钮文案或分类 labels 的事实来源。

### 2.2 ResourceSpec

职责：

- 表示页面组织用的稳定业务资源或能力域。
- 不是数据库表，不是通用 CRUD 实体。

示例：

```ts
interface ResourceSpec {
  key: string;
  labels: LocalizedText;
  description?: LocalizedText;
  category: ResourceCategorySpec;
  order?: number;
  tags?: string[];
  operations: OperationSpec[];
  diagnostics?: Diagnostic[];
}
```

分类确定规则：

- PageSpec 显式 `category.key` 优先。
- 未显式分类时，使用 `resourceKey` 的第一个 `.` 前缀。
- 没有 `resourceKey` 时，使用 `pageKey` 的第一个 `.` 前缀。
- 没有 `.` 时，整个 `resourceKey` 或 `pageKey` 就是分类。
- 最终分类必须在 PageSpec 保存或发布时确定，运行控制台加载菜单时不推断。

### 2.3 OperationSpec

职责：

- 描述函数在 Resource 中的业务动作和候选诊断，不描述最终页面位置。

```ts
interface OperationSpec {
  functionId: string;
  resourceKey?: string;
  operation: string;
  risk?: RiskLevel;
  enabled: boolean;
  candidate?: PageCandidateSummary;
  diagnostics?: Diagnostic[];
}
```

理解方式：

- `operation` 是业务动作 key，例如 `ban/grant/send/list`。
- `candidate` 是 Server 生成器内部的页面候选摘要，不是 SDK/OpenAPI 注册字段。
- 最终页面类型、按钮位置、表格、详情、图表和 mapping 只由 PageSpec 保存。

### 2.4 PageSpec

职责：

- 运营人员实际使用的页面。
- 组合查询、分页、表格、详情、操作、任务进度、报表。
- 页面级 UI 只用 Formily JSON Schema。

```ts
type PageType = 'entity' | 'operation' | 'task' | 'report';

interface PageSpec {
  pageKey: string;
  type: PageType;
  resourceKey?: string;
  title: LocalizedText;
  description?: LocalizedText;
  category: PageCategorySpec;
  order?: number;
  icon?: string;
  schema: FormilySchema;
  bindings: PageFunctionBinding[];
  metadata?: Record<string, JSONValue>;
}

interface PageFunctionBinding {
  id: string;
  functionId: string;
  usage: 'query' | 'detail' | 'action' | 'task' | 'report';
  inputMapping?: JSONValue;
  outputMapping?: JSONValue;
  execution: {
    mode: 'sync' | 'task';
    requireConfirm?: boolean;
  };
}
```

分页约束：

- 分页是 Page 的职责，不是 Function Form 的职责。
- `pageField/pageSizeField/itemsField/totalField` 必须显式写入 PageSpec。
- 函数分页字段不同，不允许前端运行时猜测。
- 页面位置由 PageSpec schema 和 `PageFunctionBinding.usage` 决定；禁止把注册期 `placement` 当作 binding 语义。
- `DataTable` 必须引用 binding，声明稳定列定义或 `columnsPath`；禁止根据首批响应对象键临时生成已发布页面列。
- 行/详情/批量动作必须有 `inputMapping`，例如 `playerId <- row.id` 或 `ids <- selection[*].id`；禁止把整行对象盲传给函数。

### 2.6 Page scope、发布快照和执行模型

页面身份：

```text
PageIdentity = game_id + env + pageKey
```

约束：

- 全局 scope 由 `X-Game-ID/X-Env` 等统一请求上下文传递，服务端按当前用户权限授权；Page body、URL、binding payload 不得覆盖 scope。
- `page_specs`、`published_page_specs`、`page_versions` 的唯一查询和索引必须使用 `(game_id, env, page_key)`。
- `PageDraft` 保存必须有 `draftRevision` / `ETag` 乐观并发控制；不匹配返回 409，不能最后写入者静默覆盖。
- `PublishedPageSpec` 除完整 PageSpec 外必须冻结每个 binding 的 function version、input/output schema digest、风险、权限、执行模式和 `rendererSchemaVersion`。
- 函数契约不兼容变化后，页面状态为 `binding_stale`；菜单可保留但执行必须拒绝，并在 Page Studio 中显示重新校验/发布入口。
- 页面执行统一为 `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute`，响应为 `PageExecutionResult(kind, requestId, traceId, data/taskId/approvalId, diagnostics)`。
- 执行链路必须注入并审计 `game_id/env/page_key/publish_version/binding_id/function_id/actor/target/trace_id`；函数权限、风险、审批策略是下限，Page 只能加严。

### 2.5 ConsoleMenuSpec

职责：

- 表示运行控制台左侧菜单。
- 由已发布 PageSpec 生成，不保存业务配置。

```ts
interface ConsoleMenuSpec {
  items: ConsoleMenuItem[];
}

interface ConsoleMenuItem {
  key: string;
  path: string;
  title: LocalizedText;
  icon?: string;
  order?: number;
  children?: ConsoleMenuItem[];
}
```

约束：

- 菜单只展示 `PublishedPageSpec`。
- 动态菜单项必须 `locale: false`。
- 缺少默认语言标题时发布失败，不允许显示 key 继续上线。

## 3. 分阶段执行计划

### P0-1. 冻结强类型模型和命名

状态：2026-07-26 核心链路已完成，后续以 guard 和新增代码审查为主。

已落地：

- `internal/api/function/dto.go` 与 `internal/logic/function/types.go` 的 Function、FunctionDescriptor、FunctionInvoke、FunctionUI、FunctionHistory 核心 DTO 已使用 `json.RawMessage`、`bool`、`string` 或明确结构。
- Function UI 清理自定义 schema 的唯一契约为 `schema: null`；旧 `__clear_custom_ui` 标记已从前后端删除。
- `web/src/types/dashboard.ts` 已作为 Dashboard TypeScript 类型入口，核心类型未使用 `any`。
- 旧 `FunctionRouteConfig`、`FunctionRouteRequest/Response`、函数 metadata 菜单路由配置 DTO 已删除。

目标：

- 在后端和前端建立唯一模型词汇，先让代码有正确类型，避免继续扩散 `WorkspaceConfig/layout/objectKey`。

受影响路径：

- `internal/api/function/dto.go`
- `internal/logic/function/types.go`
- `internal/dashboard/spec/*`
- `web/src/types/dashboard.ts`
- `web/src/services/console.ts`
- `web/src/services/api/resources.ts`

实施要点：

- 新增 Go 强类型：`LocalizedText`、`JSONSchema`、`FormilySchema`、`FunctionSpec`、`ResourceSpec`、`OperationSpec`、`PageSpec`、`PublishedPageSpec`、`ConsoleMenuSpec`、`Diagnostic`。
- 新增 TS 强类型，和 Go JSON 字段名保持一致。
- 核心 DTO 禁止 `interface{}`；必须用 `json.RawMessage`、`map[string]string`、枚举 string type 或明确结构体。
- TS 禁止 `any`；临时未知 JSON 只能使用 `unknown` 或 `JSONValue`，并在边界处 parse/validate。
- `objectKey` 不允许出现在运行代码中；新 API 使用 `pageKey/resourceKey/functionId`。
- `WorkspaceConfigCanonical`、alias 类型、重复 DTO 必须删除。

验收标准：

- 后端存在可复用的强类型 spec 包，API DTO 不再重复定义同一套结构。
- 前端存在 `dashboard.ts` 或等价单一类型入口，页面、服务、renderer 共用该入口。
- 新增或重构后的核心 DTO 中没有 `interface{}`。
- 新增或重构后的前端 dashboard 类型中没有 `any`。

验证命令：

```bash
rg -n "type Function struct|type FunctionDescriptor struct|type FunctionInvokeRequest struct|type FunctionInvokeResponse struct|type FunctionUIResponse struct|type FunctionUIUpdateRequest struct|type FunctionUIRollbackResponse struct|interface\\{\\}" "internal/logic/function/types.go" "internal/api/function/dto.go"
rg -n "\\bany\\b" "web/src/types" "web/src/services/api/functions.ts" "web/src/services/api/terms.ts" "web/src/pages/Ops/Terms/index.tsx"
rg -n "WorkspaceConfigCanonical|SaveConfigRequestAlias|GetConfigRequestAlias|WorkspaceConfig|objectKey" "internal" "web/src"
```

禁止事项：

- 禁止为了快而在 DTO 上继续保留 `Layout interface{}`。
- 禁止定义一套 Go 模型、一套不一致 TS 模型。
- 禁止用 `Record<string, any>` 逃避建模。

### P0-2. 收敛 SDK / OpenAPI 能力契约

目标：

- 函数注册只提供可执行能力契约，不承担 UI、菜单、分类显示、页面类型或页面放置责任。

受影响路径：

- `proto/croupier/sdk/v1/provider.proto`
- `sdks/go/function/*`
- `sdks/js/src/*`
- `sdks/python/src/*`
- `sdks/java/*`
- `sdks/csharp/*`
- `sdks/cpp/*`
- `web/src/services/api/openapi.ts`
- `internal/api/openapi/*`
- `internal/api/provider/helpers.go`
- `docs/architecture/openapi-sdk-descriptor-v2.md`

实施要点：

- `LocalFunctionDescriptor` 只保留或新增能力契约字段：
  - `id`
  - `version`
  - `tags`
  - `summary`
  - `description`
  - `operation_id`
  - `deprecated`
  - `input_schema`
  - `output_schema`
  - `resource`
  - `operation`
  - `risk`
  - `enabled`
  - `permission`
  - `extensions`
- 更新 `operation` 注释为业务动作 key，不再写 CRUD/custom。
- 更新所有 generated protobuf 产物。
- SDK builder 不得增加分类显示、资源显示、操作显示、页面类型、页面放置或 page hint 字段；已加的必须回滚。
- Go SDK 实现并验证 `RegisterFromOpenAPI(spec, handlers)`；其他语言在实现等价 helper 前必须在 SDK parity matrix 明确标为未支持，禁止文档泛称“所有 SDK 支持”。
- SDK demo 至少覆盖 `player.list`、`player.get`、`player.ban`、`mail.send`、`reward.batchGrant`、`analytics.retention`；示例只提供函数契约，不能嵌入 Formily/Page UI。
- OpenAPI Source 解析支持标准 `operationId/tags/summary/description/requestBody/responses/deprecated`，以及 `x-version/x-resource/x-operation/x-risk/x-enabled/x-permission`。
- OpenAPI Source 解析遇到 `x-category-display/x-entity-display/x-operation-display/x-operation-kind/x-placement/x-page-hint/ui/x-ui/Formily/menu/routes/table columns/page schema` 必须拒绝该 operation 或 Source，并返回迁移到 Page Studio 的 diagnostics。
- OpenAPI 上传重构为 Source + ExecutionBinding：支持 JSON/YAML、多版本/diagnostics/scope；未绑定 Provider 或受控 HTTP Connector 时只形成契约目录和 PageCandidate，不能产生可执行 Function。
- `web/src/services/api/openapi.ts` 不再把 `x-operation` 限定为 CRUD/custom。

验收标准：

- Go/JS/Python/Java/C#/C++ demo 都能表达同一组基础函数契约；各 SDK 的 OpenAPI helper 能力在 parity matrix 中可验证。
- `resource=player`、`operation=ban`、`risk=danger`、输入/输出 schema 这类能力契约可完整注册并在 API 中返回。
- 用户上传 OpenAPI 后可看到 Source diagnostics 和 PageCandidate；只有显式 ExecutionBinding 后才能调用函数。
- SDK/OpenAPI 输入包含 UI、显示、多语言菜单、页面类型或页面放置字段时，注册或导入失败，并返回可读 diagnostics。
- 缺少 PageContract 或 mapping 时，函数和 Server 派生 Form 仍可用；PageCandidate 必须带 diagnostics，绝不自动发布。

验证命令：

```bash
rg -n "CRUD operation type|create.*read.*update.*delete.*custom|x-operation.*custom" "proto" "sdks" "web/src/services/api"
rg -n "operation_kind|operationKind|x-operation-kind|placement|x-placement|category_display|categoryDisplay|entity_display|operation_display|page_hint|pageHint" "proto" "sdks" "internal" "web/src"
```

禁止事项：

- 禁止继续把 `operation` 当页面类型。
- 禁止用 SDK/UI 兼容层把 `ui/x-ui` 回流为注册字段。
- 禁止用 `extensions` 承载平台 UI、显示、多语言菜单、页面类型、页面放置或 PageSpec 语义；`extensions` 只能作为第三方非核心扩展出口。

### P0-3. 建立 Descriptor Normalizer

状态：2026-07-26 核心链路已完成，后续以 CI guard、collector 边界审计和端到端验收为主。

已落地：

- `internal/dashboard/normalizer` 已输出 `FunctionSpec/ResourceSpec/OperationSpec`。
- `internal/logic/function/types.go` 的 Function detail、descriptor、invoke、UI、history 返回不再用 `interface{}` 承载 JSON payload。
- Function invoke 保留 dispatcher 原始 JSON payload；非 JSON 响应会编码为 JSON string，不再反序列化为任意 `interface{}`。
- Function UI 获取、更新、历史、回滚统一以 Formily schema `json.RawMessage` 对外；保存前显式解析为 JSON AST 并校验 Formily。
- Function UI 清理自定义 override 使用 `schema: null`，旧 `__clear_custom_ui` 标记已清理。

目标：

- Server 统一把 SDK/OpenAPI/DB template 转成 `FunctionSpec/ResourceSpec/OperationSpec`。

受影响路径：

- `internal/logic/function/descriptors_logic.go`
- `internal/logic/function/ui_resolver.go`
- `internal/logic/function/formily_schema.go`
- `internal/api/function/*`
- 新增建议：`internal/dashboard/normalizer/*`

实施要点：

- 将 `[]map[string]interface{}` 替换为 `[]FunctionSpec` 或明确响应 DTO。
- Normalizer 输入支持 SDK descriptor、OpenAPI operation、DB template，但输出只有强类型 spec。
- Locale key 统一归一化只发生在 PageSpec / Function Form override 等平台 UI 配置边界；函数注册的 `summary/description` 不承担动态菜单 labels。
- 注册输入出现 `x-ui` 或 `ui` 必须直接拒绝，不能把 UI 责任带回 SDK/OpenAPI。
- 注册输入出现 display labels、`operationKind`、`placement`、`pageHint`、menu、route、Page schema 等字段也必须直接拒绝或在 Source diagnostics 中标为越界。
- `input_schema` 存在时，由 JSON Schema 派生默认 Function Form。
- `input_schema` 不存在时，只生成 `payload` 单字段 Function Form，并给出 `input_schema_missing` warning。
- 生成 PageCandidate 时，缺少可验证 PageContract、分页字段、列定义、输入/输出 mapping、任务追踪或图表契约标记 `needs_review/blocked`；仍可生成保守 PageCandidate，不能自动发布 Page。
- 删除 `defaultMenu`、`menuSource`、`metadata.menu`、`inferCategory`、`applyEntityMenuDefaults` 作为页面/菜单来源的逻辑。
- `termDictModel` 只能用于枚举、状态、下拉选项，不参与动态导航事实源。

验收标准：

- `GET /api/v1/functions/descriptors` 返回强类型能力契约字段和 diagnostics，不返回页面显示或 placement 注册字段。
- 缺少 PageContract/mapping 的函数能在函数目录看到，但不会生成 ready Page。
- 只有管理员保存的 Function Form override 或 PageSpec 的 Formily schema 才做 Formily 校验；注册输入不接收 UI。
- 分类最终只发生在 PageSpec 保存或发布阶段，并写入 PageSpec；归一化阶段最多生成候选建议。

验证命令：

```bash
go test ./internal/logic/function/... ./internal/api/function/...
rg -n "defaultMenu|menuSource|metadata\\.menu|inferCategory|applyEntityMenuDefaults" "internal/logic/function"
rg -n "interface\\{\\}" "internal/logic/function/types.go" "internal/api/function/dto.go"
rg -n "schemaInputFromRaw|rawSchemaFromAny|boolFromAny|stringFromAny|__clear_custom_ui" "internal/api/function" "internal/logic/function" "web/src/services/api/functions.ts"
rg -n "ImportFromOpenAPI|importOpenAPISpec|/metadata/functions/import/openapi" "internal" "web/src" && exit 1
```

禁止事项：

- 禁止前端补齐或猜测页面类型、按钮位置、表格列、分页字段或 mapping。
- 禁止把字典重新引入为菜单主模型。
- 禁止 silent fallback 成健康页面。

### P0-4. Function Form 收敛为单一 Formily

状态：2026-07-24 核心链路已完成。

已落地：

- SDK 注册 descriptor extensions 中出现 `ui/x-ui/x_formily/layout/components` 等 UI 字段时，注册函数被跳过并产生 `function_ui_not_allowed` warning。
- OpenAPI Source 的 Operation extensions 中出现 UI 字段时，该 Source 校验失败并返回 diagnostics。
- Function UI resolver 不再读取 `openapi_spec.x-ui`，默认表单只从 `input_schema` 派生；缺少 `input_schema` 时只生成保守 fallback。
- HTTP Function UI update 只接受 `schema`；`ui/layout/components/x-ui` 等字段直接返回 400。
- 前端 Function UI Manager 和 API service 只读取/提交 `schema`，不再传播 `layout/components/openapi_x_ui`。
- SchemaDesigner 不再从旧 `params` 推断字段；没有 `input_schema` 时使用单个 `payload` 默认字段。
- `web/src/utils/json.ts` 中 JSON Schema -> Formily 推断核心路径已使用递归 JSON Schema 类型和 `FormilySchema` 返回类型。

目标：

- 单函数调用和 PageSpec binding 弹窗都使用 Server 派生或管理员 override 的 Function Form；SDK/OpenAPI 注册不包含 UI。

受影响路径：

- `internal/logic/function/function_u_i_logic_v2.go`
- `internal/logic/function/function_u_i_update_logic.go`
- `internal/logic/function/function_u_i_rollback_logic.go`
- `internal/api/function/dto.go`
- `web/src/components/formily/SchemaRenderer.tsx`
- `web/src/components/FunctionUIManager/index.tsx`
- `web/src/pages/Functions/SchemaDesigner/*`
- `web/src/utils/json.ts`

实施要点：

- Function Form DTO 只保留 `schema: FormilySchema`、版本、来源、诊断。
- 删除或拒绝 `layout/components/fields/ui/x-ui` 等第二套 UI 协议字段；其中 `ui/x-ui` 在函数注册边界直接拒绝。
- `json.ts` 中 JSON Schema -> Formily 的推断逻辑必须输出严格类型，不使用 `any`。
- 单函数调用页面只渲染 Function Form。
- PageSpec 中的 `QueryForm/ActionButton` 只能通过 binding/form reference 引用 Server 的 Function Form，禁止内联来自 SDK/OpenAPI 的 UI。

验收标准：

- UI 更新接口提交非 Formily schema 返回 400。
- 所有函数调用表单和 Page 动作弹窗使用同一个 Formily renderer。
- 代码中不存在同时维护 `FieldConfig` 和 Formily Schema 的新链路。

验证命令：

```bash
go test ./internal/logic/function/... ./internal/api/function/...
./web/node_modules/.bin/eslint "src/components/formily/SchemaRenderer.tsx" "src/components/FunctionUIManager/index.tsx" "src/pages/Functions/SchemaDesigner/index.tsx" "src/pages/Functions/Invoke/index.tsx" "src/services/api/functions.ts" "src/services/schema/index.ts" "src/utils/json.ts"
rg -n "openapi_x_ui|uiConfig\\.layout|uiConfig\\.components|layout\\?: Record<string, unknown>|components\\?: Record<string, unknown>|parseInputSchema\\([^\\)]*,|generatedFromParams" "web/src/components/FunctionUIManager/index.tsx" "web/src/services/api/functions.ts" "web/src/services/schema/index.ts" "web/src/pages/Functions/SchemaDesigner/index.tsx" "web/src/utils/json.ts"
```

当前环境备注：

- `./web/node_modules/.bin/eslint ...` 已通过。
- `git diff --check` 已通过。
- Go 定向测试在复用已有模块缓存时通过；切换空 `GOMODCACHE` 会因 sandbox 禁止访问 `proxy.golang.org` 失败，不是代码失败。

禁止事项：

- 禁止为了兼容旧 UI 同时渲染两套结构。
- 禁止 schema 校验失败后自动降级为空表单。
- 禁止把函数表单 override 误用为 PageSpec、菜单或表格配置。

### P0-5. Resource API 替换旧 Entity API

状态：2026-07-26 核心链路已完成。

已落地：

- `internal/api/entity/*`、`internal/model/entity.go`、`internal/model/entity_model.go`、`internal/logic/utils/entity_helpers.go` 已删除。
- `web/src/pages/Entities`、`web/src/components/XEntityForm.tsx`、`web/src/services/api/entities.ts` 已删除。
- 前端系统主入口已使用 Resources 页面；Resource API 只读归一化资源、操作和生成候选，不做业务对象 CRUD。
- 权限与菜单文案仍需在 P1-2/P1-3 做最终核验。

目标：

- 建立 Dashboard 页面生成需要的 Resource/Operation 查询接口，删除“实体管理就是页面”的误导。

已清理路径：

- `internal/api/entity/*`
- `internal/model/entity.go`
- `internal/model/entity_model.go`
- `internal/logic/utils/entity_helpers.go`
- `internal/api/openapi/service.go`
- `internal/router/router.go`
- `web/config/routes.ts`
- `web/src/pages/*Entity*`
- `web/src/components/XEntityForm.tsx`
- `configs/permissions.json`

后续要点：

- 验收 `/api/v1/resources`：
  - `GET /api/v1/resources`
  - `GET /api/v1/resources/:resourceKey`
  - `GET /api/v1/resources/:resourceKey/operations`
  - `GET /api/v1/resources/:resourceKey/pages/generated`
- Resource API 只读归一化 spec 和 diagnostics，不提供业务对象 CRUD。
- 旧 `/api/v1/entities` 不允许保留兼容路由。
- 前端必须清楚说明 Resource 不是数据库实体。
- 权限从 `entities:*` 收敛为 `resources:read`、`resources:diagnose` 或按实际需要命名。

验收标准：

- 用户在系统菜单中不会再看到误导性的“实体管理”作为业务对象 CRUD。
- Resource 页面能看到 player/mail/reward/analytics 等资源及其 operations、diagnostics。
- Resource API 不出现 create/update/delete 业务对象接口。

验证命令：

```bash
go test ./internal/api/resource/... ./internal/api/openapi/... ./internal/router/...
pnpm --dir "web" exec eslint "src/pages" "src/services"
rg -n "/api/v1/entities|entities:|entity:write|实体管理|XEntityForm" "internal" "web/src" "configs"
```

禁止事项：

- 禁止把 Resource API 做成新的通用 CRUD。
- 禁止继续在运行控制台里出现 `game_id/env` 二次选择。

### P0-6. Page Studio API 替换 WorkspaceConfig API

状态：2026-07-25 后端核心链路已完成；旧 Workspace API/model 已删除。

已落地：

- `internal/api/workspace/*`、`internal/model/workspace_config.go` 已删除。
- Page API/model 已提供 scope、draftRevision、binding、mapping、发布 contract snapshot。
- 旧 `/api/v1/workspaces/:objectKey/config` 和 `/api/v1/workspaces/pages` 不允许作为兼容主路径存在。

目标：

- 后端以带 scope、revision 和 binding snapshot 的 `PageDraft/PublishedPageSpec` 管理页面草稿、预览、校验、发布、版本和回滚。

受影响路径：

- `internal/api/page/*`
- `internal/model/config_version.go`
- `internal/router/router.go`

实施要点：

- API 使用文档目标，禁止恢复 `/api/v1/workspaces/pages` 迁移期路径：
  - `GET /api/v1/pages/generated?resourceKey=...`
  - `GET /api/v1/pages`
  - `GET /api/v1/pages/:pageKey`
  - `PUT /api/v1/pages/:pageKey`
  - `POST /api/v1/pages/:pageKey/validate`
  - `POST /api/v1/pages/:pageKey/preview`
  - `POST /api/v1/pages/:pageKey/publish`
  - `POST /api/v1/pages/:pageKey/unpublish`
  - `GET /api/v1/pages/:pageKey/versions`
  - `GET /api/v1/pages/:pageKey/versions/:versionId`
  - `POST /api/v1/pages/:pageKey/rollback`
- `Save` 不再保存 `layout`，只保存完整 `PageSpec`。
- `Get` 不存在时返回 404，不自动创建默认 tabs layout。
- model 增加 `GameID/Env/DraftRevision`；唯一索引、find/list/upsert/active snapshot 和 version lookup 全部使用 scope + pageKey。
- Save 要求 `If-Match` / `draftRevision`；冲突返回 `409 page_draft_conflict` 和当前 revision，不能静默覆盖。
- Page binding 改为 `id/functionId/usage/inputMapping/outputMapping/execution`，删除 `Role OperationPlacement`；Schema 只能引用 `bindingId`。
- 发布前执行完整校验：Formily Page root、schemaVersion、组件 props ABI、labels、category、binding mapping、输入/输出契约、权限和 task/report 数据源。
- 发布时生成不可变 `PublishedPageSpec` 快照并冻结 binding contract snapshot；不得在运行时从最新 FunctionSpec 静默补齐。
- 版本 key 使用 `page:{gameId}:{env}:{pageKey}`。

验收标准：

- 旧 `/api/v1/workspaces/:objectKey/config` 和 `/api/v1/workspaces/pages` 不再被前端调用或保留兼容路由。
- 缺配置不会自动创建页面。
- 发布失败能返回可读 diagnostics。
- PublishedPageSpec 可以在同 scope 内独立供运行控制台读取，不能跨 scope 碰撞。
- 同时编辑同一页面的两个请求，旧 revision 必定 409；不会覆盖新草稿。

验证命令：

```bash
go test ./internal/api/page/... ./internal/model/... ./internal/router/...
rg -n "WorkspaceConfig|objectKey|layout|/api/v1/workspaces" "internal/api" "internal/model" "web/src"
```

禁止事项：

- 禁止保留 `WorkspaceConfig` 兼容 DTO。
- 禁止不存在草稿时自动生成并保存默认 layout。
- 禁止发布时对缺失字段做静默补全。
- 禁止用 `page_key` 全局唯一或从 Page JSON/payload 读取 scope。
- 禁止继续以 `binding.Role == FunctionSpec.Placement` 校验页面绑定。

### P0-7. Console API 和动态菜单

状态：2026-07-26 核心链路已完成；仍需真实部署/数据端到端验收。

目标：

- 运行控制台菜单从 Server 发布产物读取，不再由前端组装。

受影响路径：

- `internal/router/router.go`
- `internal/api/console/*`
- `web/src/app.tsx`
- `web/src/services/console.ts`
- `web/config/routes.ts`
- `web/src/pages/Console/index.tsx`
- `web/src/pages/Console/Page.tsx`

实施要点：

- API：
  - `GET /api/v1/console/menu`
  - `GET /api/v1/console/pages`
  - `GET /api/v1/console/pages/:pageKey`
- `console/menu` 从 `PublishedPageSpec[]` 生成 `ConsoleMenuSpec`。
- `console/pages/:pageKey` 返回单个 `PublishedPageSpec`，不返回草稿。
- 所有 Console 查询使用当前请求 scope，只加载 `(game_id, env)` 下 active snapshot；切换全局 scope 后必须重新请求菜单与页面。
- 菜单按 `category.order/page.order/title` 排序。
- 菜单标题按请求语言或用户语言解析；也可返回完整 labels 由前端选择，但动态项必须 `locale: false`。
- `web/src/app.tsx` 删除 `workspaceConfigs` initialState。
- ProLayout `menu.request` 请求 `ConsoleMenuSpec`，并合并固定系统菜单。
- 固定路由：
  - `/console/home`
  - `/console/:categoryKey`
  - `/console/:categoryKey/:pageKey`
- 若 URL 分类与 PageSpec 发布分类不一致，前端必须跳转规范路径，并且跳转期间不得继续渲染错误分类下的页面。

验收标准：

- 新增动态分类不需要改前端代码和 locale 文件。
- 发布 Page 后刷新页面，左侧菜单出现对应分类和页面。
- 取消发布 Page 后刷新页面，菜单消失。
- 不同 scope 发布同名 pageKey 时，菜单和页面不会相互泄漏。
- 没有发布页面时，运行控制台只显示首页或空状态。
- 浏览器控制台不再输出调试日志 `[menuDataRender]`。

验证命令：

```bash
go test ./internal/api/console/... ./internal/router/...
pnpm --dir "web" exec eslint "src/app.tsx" "src/pages/Console" "src/services"
rg -n "workspaceConfigs|buildConsoleMenuData|menuDataRender|menu\\.ControlConsole\\.category|objectKey" "web/src/app.tsx" "web/src/pages/Console" "web/src/services"
```

禁止事项：

- 禁止在前端从函数目录、Resource 列表或 workspace 草稿生成运行菜单。
- 禁止动态分类写静态 locale。
- 禁止保留 `objectKey` 路由参数。
- 禁止通过 category/pageKey URL 参数推断或覆盖 scope。

### P0-8. 前端 PageSpec Formily Renderer

目标：

- 运行控制台渲染已发布 PageSpec，页面结构由 Formily schema 驱动。

受影响路径：

- `web/src/pages/Console/*`
- `web/src/components/WorkspaceRenderer/*`
- `web/src/components/page-schema/PageSchemaRenderer.tsx`
- 新增建议：`web/src/components/FormilyPageRenderer/*`
- 新增建议：`web/src/components/console-page-components/*`

实施要点：

- 新建 `FormilyPageRenderer`，输入只接受 `PublishedPageSpec` 或 `PageSpec`。
- 注册页面级 Formily 组件：
  - `ConsolePage`
  - `QueryForm`
  - `DataTable`
  - `DetailPanel`
  - `ActionButton`
  - `ActionGroup`
  - `ResultPanel`
  - `TaskTimeline`
  - `ChartPanel`
- Page renderer 只接受已发布 PageSpec；schema root 必须有受支持 `schemaVersion` 和 `ConsolePage`。
- 建立一个版本化 Page component registry，同时给每个组件提供 TypeScript props、JSON Schema validator 和服务端发布校验；未知组件/props 直接失败。
- `QueryForm` 通过 `bindingId` 调受控执行 API，将 `inputMapping/outputMapping` 写入显式 page state。
- `DataTable` 只读取 binding/page state，显式使用 `itemsPath/totalPath/pageField/pageSizeField` 和稳定 columns/columnsPath；不能动态按首批结果推列。
- `ActionButton` 根据 binding 的 Function Form 打开 Formily 弹窗，按 inputMapping 组装 payload，再调受控执行 API；前端确认只可加严，风险仍以后端为准。
- `TaskTimeline` 对接现有 Task start/events/result 链路，以返回的 taskId 订阅/轮询；不能用 `lastResult` 占位。
- `ChartPanel` 只读取 PageSpec 中明确声明的 stateKey/dataPath/chart type/字段映射。
- 删除旧 `WorkspaceRenderer` 或将其彻底迁移为 Formily renderer，不保留旧 layout renderer。
- `PageSchemaRenderer.tsx` 如果不是目标 Formily PageSpec renderer，删除或改名，避免误用。

验收标准：

- `/console/:categoryKey/:pageKey` 能渲染 Entity/Operation/Task/Report 四类页面的最小可用版本，并且所有调用都来自 binding execute API。
- 分页、行操作、工具栏操作、批量操作都来自 PageSpec 显式配置。
- 旧 `TabsLayout/FormDetailRenderer/ListRenderer/GridRenderer/...` 不再被运行控制台引用。
- 页面 schema 的未知 version/component/props、裸 functionId、未知 bindingId、缺 mapping 都会在验证和运行时可读报错。

验证命令：

```bash
pnpm --dir "web" exec eslint "src/pages/Console" "src/components"
rg -n "WorkspaceRenderer|TabsLayout|FormDetailRenderer|ListRenderer|GridRenderer|KanbanRenderer|WizardRenderer|DashboardRenderer" "web/src"
rg -n "\\bany\\b" "web/src/components/FormilyPageRenderer" "web/src/components/console-page-components" "web/src/pages/Console"
```

禁止事项：

- 禁止运行时把 PageSpec 转成旧 layout 再渲染。
- 禁止缺字段时随便显示一个空页面并当作成功。
- 禁止 renderer 直接调用 `/api/v1/functions/:id/invoke` 或让浏览器传 route/target/scope。
- 禁止用任意 `lastResult` 或整个 row 对象作为跨组件数据总线。

### P0-9. PageSpec Generator

状态：2026-07-26 核心链路已完成。

已落地：

- `OperationSpec` 增加 `pageContract`，`OpenAPI Source` 支持读取 `x-page-contract`，runtime descriptor extensions 可用 JSON 字符串携带同一契约。
- Generator 不再写死 `page/pageSize/itemsPath/totalPath/columnsPath`，只有 `PageContract.pagination + table.columns/columnsPath + input/outputMapping` 可验证时才生成 `DataTable`。
- 缺少 `pageContract`、分页字段、列定义、输入/输出 mapping、task tracking 或 report chart contract 时，候选只标为 `needs_review` 或 `blocked`，并返回结构化 diagnostics。
- 生成的 `PageFunctionBinding` 会复制 `pageContract.inputMapping/outputMapping/executionMode`，schema 仍只引用 `bindingId`。
- Generator 已按明确契约生成四类候选：
  - `player.manage` Entity Page：只在列表操作具备 `pagination + table + input/outputMapping` 时生成 `DataTable`，行操作必须显式引用 `row.*` 或 `selection.*`。
  - `mail.send` Operation Page：没有列表/任务/报表契约时保持独立表单 + 结果页面，不强行塞进 Entity Page。
  - `reward.batchGrant` Task Page：只在 `executionMode=task` 或 `task` contract 明确时生成 `TaskTimeline`。
  - `analytics.retention` Report Page：只在 `report` contract 明确 chart type、category/series/value 路径时生成 `ChartPanel`。
- 单测覆盖“无 pageContract 不猜 DataTable”、“有 pageContract 才生成 DataTable/rowAction binding”、“缺契约只 needs_review”、“四类候选”和“页面类型不按函数名推断”。

未完成：

- 真实演示数据端到端 fixture 仍需补到 SDK demo/OpenAPI Source 管理 UI，验证从上传/注册到 Page Studio 发布的完整用户路径。
- Page Studio 已能消费 GeneratedPageSpec 并保存为草稿；mapping 补齐仍是 JSON 文本/基础表单，后续可增强为按 Function Form 和 PageContract 辅助编辑。

目标：

- 根据归一化后的 Resource/Operation 生成默认 PageSpec 建议，让函数注册后有可预览的默认界面。

受影响路径：

- `internal/dashboard/generator/*`
- `internal/api/resource/*`
- `internal/api/page/*`
- `internal/api/openapi/*`
- `internal/logic/function/*`

实施要点：

- 输入接受 `ResourceSpec + OperationSpec[] + FunctionSpec + PageContract`；没有输入/输出契约和可验证 PageContract 不能标为 ready。
- 输出 `GeneratedPageSpec[]`，带 `quality=ready/needs_review/blocked` 和 diagnostics。
- Entity Page 生成规则：
  - 有可验证列表契约时生成 `DataTable`。
  - 有可验证查询输入契约时生成 `QueryForm`。
  - 有可验证详情契约时生成 `DetailPanel`。
  - 有可验证行、详情、工具栏或批量 action mapping 时生成对应 `ActionGroup`。
  - 分页字段、响应 items/total、列定义或 columnsPath 可验证时才生成 ready DataTable。
  - 行/详情/批量操作的 inputMapping 不可验证时 `needs_review` 或 `blocked`，不硬猜。
- Operation Page 生成规则：
  - 同步执行契约且不依赖表格/详情上下文时，可生成单页表单 + 结果面板候选。
- Task Page 生成规则：
  - 任务执行契约明确 task status/events/result 来源时，可生成参数表单 + TaskTimeline + ResultPanel 候选。
- Report Page 生成规则：
  - 报表或图表契约明确 chart type、series/category/value 路径时，可生成筛选表单 + ChartPanel/DataTable 候选；无显式 chart contract 时不得生成 ready 图表。
- `mail.send`、`cache.refresh` 这类没有对象生命周期的函数生成 Operation Page，不强行进入 Entity Page。
- `reward.batchGrant` 这类批量异步操作优先 Task Page。

验收标准：

- 示例函数能生成：
  - `player.manage` Entity Page
  - `mail.send` Operation Page
  - `reward.batchGrant` Task Page
  - `analytics.retention` Report Page
- 缺少可验证 PageContract、mapping、分页、列、任务追踪或图表契约的函数只生成 `needs_review/blocked` diagnostics。
- Generator 单测覆盖分类默认规则、labels 缺失、分页字段映射、列/输入 mapping、任务/图表 contract、独立操作页。

验证命令：

```bash
GOCACHE="/tmp/croupier-go-build" GOMODCACHE="/tmp/croupier-go-mod" go test ./internal/dashboard/... ./internal/api/resource ./internal/api/openapi ./internal/api/page ./internal/api/console
rg -n "strings\\.HasSuffix|strings\\.Contains|split.*function|function.*split|player_id|reason" "internal/dashboard" "web/src/pages/Console"
```

禁止事项：

- 禁止根据函数名后缀推断正式 Page。
- 禁止把所有函数都塞到同一个对象管理页。
- 禁止没有 output schema 时硬生成表格列。

### P0-10. 受控 Page 执行、契约失效与追踪

目标：

- 让运行控制台的每一次页面操作都可按发布版本复现、授权、审批、审计和追踪；Page renderer 不能绕过发布 binding 直接调用函数。

受影响路径：

- 新增建议：`internal/api/consoleexecution/*` 或 `internal/api/console/*`
- `internal/api/function/*`
- `internal/api/task/*`
- `internal/policy/*`
- `internal/audit/*`
- `internal/telemetry/*`
- `internal/dashboard/spec/*`
- `web/src/services/console.ts`
- `web/src/components/FormilyPageRenderer/*`

实施要点：

- 新增 `POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute`；服务端从 active PublishedPageSpec 和当前 scope 查 binding，客户端请求体只允许 mapping 后的业务输入。
- 删除 Page renderer 对 `/api/v1/functions/:id/invoke` 的依赖；保留该接口只给函数目录、API 调试和非 Page 场景。
- 定义强类型 `BindingContractSnapshot`：function ID/version、input/output schema digest、risk、permission、execution mode、descriptor/page contract version。
- Publish 生成 snapshot；execute 比较当前 FunctionSpec，兼容策略必须显式定义。默认安全策略是版本或 digest 不匹配即 `binding_stale`，拒绝执行，需重新校验并发布。
- 定义 `PageExecutionResult`：`kind=sync/task/approval`、`requestId`、`traceId`、`data`、`taskId`、`approvalId`、diagnostics；不向 renderer 暴露无结构的任意 response。
- Page binding 执行沿用函数权限、风险、审批、审计和 dispatcher，另增加 page visibility/execute 权限；Page 只可加严不能降级。
- OTel span/audit event 统一记录 `game_id/env/page_key/publish_version/binding_id/function_id/actor/target/task_id/approval_id/trace_id`，禁止记录完整敏感 payload。
- Task Page 必须以 taskId 对接 status/events/result；审批返回 approvalId 后展示等待态，不能报告“操作完成”。

验收标准：

- 伪造 pageKey/bindingId/functionId/route/target/scope 的请求不能执行未绑定函数或越权目标。
- `player.ban` 经过现有审批策略时返回 approval 状态，审计和 trace 能关联到 page publish version。
- `reward.batchGrant` 返回 taskId，TaskTimeline 能读取真实 task 状态和事件。
- Function schema/risk/permission 发生不兼容变化后，旧页面执行返回 `binding_stale`，不会静默使用新函数。
- Page 执行和函数目录直接 invoke 都有明确用途、独立路由和测试，互不伪装。

验证命令：

```bash
go test ./internal/api/console/... ./internal/api/function/... ./internal/api/task/... ./internal/policy/... ./internal/audit/... ./internal/telemetry/...
pnpm --dir "web" exec eslint "src/services/console.ts" "src/components/FormilyPageRenderer" "src/pages/Console"
rg -n "/functions/.*/invoke|invokeFunction\(" "web/src/components/FormilyPageRenderer" "web/src/pages/Console"
```

禁止事项：

- 禁止通过浏览器参数选择任意 `functionId`、`route`、`targetServiceID`、`gameId` 或 `env`。
- 禁止页面通过 mock/lastResult 假装 Task、Approval 或分页已闭环。
- 禁止把原始敏感 payload 写入 OTel attributes、审计 details 或前端错误上报。

### P0-11. OpenAPI Source 上传与执行绑定

目标：

- 让用户可以上传 OpenAPI 3.x JSON/YAML 作为契约 Source，同时把“文档可解析”和“函数可执行”严格分开；不要求上传者提供 UI。

当前状态：

- 已完成：`GET/POST /api/v1/openapi/sources`、`PUT /api/v1/openapi/sources/:sourceId`、`GET /api/v1/openapi/sources/:sourceId`、`GET /api/v1/openapi/sources/:sourceId/diagnostics`、`POST/DELETE /api/v1/openapi/sources/:sourceId/bindings`。
- 已完成：Source 按 `game_id + env` 保存，支持 multipart file、raw JSON/YAML 和 `{ name, spec }`，保存 content hash、operation 清单和 diagnostics。
- 已完成：上传拒绝外部 `$ref`、缺失/重复 `operationId`、无效 OpenAPI 版本、`x-ui/ui/Formily/layout/components/menu/routes/table columns` 等 UI 字段。
- 已完成：Source 上传不会写入 runtime registry；Provider binding 必须显式绑定当前已注册函数。
- 已完成：固定系统入口 `/system/functions/openapi-sources`，支持 Source 列表、上传、详情、diagnostics、operations、Provider binding 和解绑；只读用户只能查看，写入操作按权限裁剪。
- 已完成：前端和服务层权限已接入 `openapi_sources:read/openapi_sources:write`，并允许资源诊断、函数管理或页面编辑权限执行 Source 管理。
- 已完成：Source 更新会创建新 revision、刷新 operation 清单和 diagnostics、保留显式 binding 记录但不会把已不存在的 operation 标记为 bound，并写 `openapi_source.update` 审计与 `openapi.source.update` span。
- 已完成：已绑定 Provider 的 Source operation 会合并到同一 `FunctionSpec/ResourceSpec/OperationSpec` 视图，Page Studio 的 Resource generator 能消费 Source 中的 `x-page-contract` 生成 PageCandidate；未绑定 Source 不会伪造成可执行函数。
- 未完成：受控 `httpConnector`、OpenAPI Source 到 Page Studio 发布的真实浏览器端到端验收、真实 OTel collector 字段验收。

受影响路径：

- `internal/api/openapi/*`
- `internal/model/*openapi*`
- `internal/dashboard/descriptors/*`
- `internal/dashboard/normalizer/*`
- `internal/platform/registry/*`
- `web/src/pages/OpenAPISources/*`
- `web/src/services/api/openapi.ts`
- `docs/guide/integrations/openapi-registration.md`
- `docs/sdks/sdk-parity-matrix.md`

实施要点：

- 将历史 `POST /api/v1/openapi/import` 替换为 Source API：`POST/GET /api/v1/openapi/sources`、`PUT /api/v1/openapi/sources/:sourceId`、`GET /api/v1/openapi/sources/:sourceId`、`POST/DELETE .../bindings`、`GET .../diagnostics`；不保留旧 import 兼容路由。
- 上传支持 multipart file 和 raw JSON/YAML；服务端限制大小、禁用远程 `$ref`、解析前后校验 OpenAPI 版本、规范化 operationId 并保存内容 hash、source version、scope、diagnostics 和操作清单。
- Source 只产生 RawFunctionDescriptor / FunctionSpec 候选；只有显式 Provider binding 后才进入 Resource/PageCandidate 生成视图，不能直接注册为可调度 Function，更不能自动发布 Page。
- `ExecutionBinding` 必须显式选择 `provider`（当前 scope 的 operationId -> SDK Handler）或受控 `httpConnector`；当前仅启用 `provider`，`httpConnector` 必须等 allowlist base URL、SecretRef、超时/重试/请求策略落地后开放。
- Source 更新创建新 revision；已发布 Page 的 binding snapshot 比较 source/contract digest，发生不兼容变更后标记 stale。
- Go SDK 的 `RegisterFromOpenAPI(spec, handlers)` 继续作为本地 Provider 注册 helper；其他语言在提供等价 helper 前明确标为未支持，不能伪造 parity。
- 上传、绑定、解绑、Source 更新都要权限、审计和 OTel；当前上传/更新/绑定/解绑权限、审计和管理 span 已落地，真实 collector 验收仍需补齐；不记录文档中的凭据或业务敏感示例 payload。

验收标准：

- 上传 JSON/YAML 文档成功后能列出 operation、Function Form 和 PageCandidate diagnostics，但不能在无 ExecutionBinding 时调用。
- 恶意/错误文档（超大、远程 ref、缺 operationId、无效 schema）返回结构化 diagnostics，不写入半成品 Source。
- 绑定 Provider 后可调用对应函数；解绑或 scope 不匹配后执行被拒绝。
- HTTP Connector 不能由用户填写任意内网地址、Header、Secret 或绕过 scope；所有调用有审计/trace。
- UI 只在 Server Function Form 与 Page Studio 生成，上传文档中含 `x-ui`/Formily/Page schema 时被拒绝。

验证命令：

```bash
GOCACHE="/tmp/croupier-go-build" GOMODCACHE="/tmp/croupier-go-mod" go test ./internal/api/openapi ./internal/api/page ./internal/api/console ./internal/api/resource ./internal/dashboard/...
./web/node_modules/.bin/eslint "web/src/services/api/openapi.ts" "web/src/services/api/functions.ts" "web/src/services/api/functions-enhanced.ts" "web/src/services/api/resources.ts"
./web/node_modules/.bin/eslint "web/src/pages/OpenAPISources/index.tsx"
./web/node_modules/.bin/tsc --noEmit -p "web/tsconfig.json" --pretty false
rg -n "x-ui|\"ui\"|/api/v1/openapi/import" "internal/api/openapi" "web/src/services/api/openapi.ts" "docs/guide/integrations"
```

禁止事项：

- 禁止把上传文档直接写进 runtime registry 后称为“注册成功可调用”。
- 禁止 Source upload 下载远程 `$ref`、允许任意 SSRF Connector，或把 Secret 放入 OpenAPI/PageSpec/浏览器。
- 禁止要求 OpenAPI 上传者或 SDK 开发者填写 Formily、菜单、路由、表格列、页面组件或最终 mapping。

## 4. P1 前端工作台重构

### P1-1. 新建 Page Studio

状态：2026-07-26 核心完成；旧 WorkspaceEditor 已删除；已支持版本查看、版本 diff、版本回滚、409 revision 冲突提示、服务端 Page 组件 ABI 校验、Page schema 顶层组件结构化编辑、组件 props 表单化编辑、DataTable columns/rowActions 和 ActionGroup actions 专用编辑。后续只做真实端到端验收和体验细节，不恢复旧 editor。

目标：

- 用户在 Page Studio 中管理当前 scope 的 PageSpec 草稿，而不是编辑旧 layout。

受影响路径：

- `web/src/pages/PageStudio/*`
- `web/src/services/api/pages.ts`
- `web/src/services/api/resources.ts`
- `web/config/routes.ts`
- `web/src/types/dashboard.ts`

实施要点：

- 路由改为 `/system/functions/pages`、`/system/functions/pages/:pageKey` 或符合现有信息架构的 Page Studio 路径。
- 列表页展示 Page 草稿、发布状态、分类、类型、诊断、更新时间。
- 支持从 Resource 生成默认 PageSpec 建议并复制为草稿。（已接入基础流程）
- 编辑器只编辑 PageSpec Formily Page Schema、bindingId/usage/inputMapping/outputMapping/execution 和非执行展示 metadata。（已接入顶层 Page 组件结构化编辑、组件 props 表单化编辑、DataTable columns/rowActions 和 ActionGroup actions 专用编辑。）
- 保存时携带 revision，409 时显示当前/本地 revision 并允许加载最新草稿；不得覆盖。（已接入基础冲突提示和版本 diff。）
- 编辑器必须使用与发布端相同的组件 registry/props validator，不能出现“编辑器能保存、运行期不能渲染”。（服务端 ABI validator 已接入；可视化 schema 编辑器后续复用该 diagnostics。）
- 预览调用 `/preview`，不影响运行控制台。
- 发布调用 `/publish`，失败展示 diagnostics。
- 历史版本、diff、rollback 使用 PageSpec 结构，不比较旧 layout。（版本查看、diff 和 rollback 已接入）
- 禁止恢复 `TabEditor/LayoutDesigner/CanvasEditor/schemaToLayout` 等旧 layout 专用概念。

验收标准：

- 用户可以从 Resource 生成 PageCandidate 草稿、预览、校验，并在确认后发布。
- 发布后当前 scope 的运行控制台菜单出现，取消发布后消失。
- 编辑器页面没有 objectKey、tabs layout、sections layout 的概念。

验证命令：

```bash
./web/node_modules/.bin/eslint "src/pages/PageStudio/index.tsx" "src/services/api/pages.ts" "src/services/api/resources.ts"
test ! -d "web/src/pages/PageStudio" || ! rg -n "objectKey|WorkspaceConfig|TabEditor|LayoutDesigner|schemaToLayout|WorkspaceLayout|PageGenerator" "web/src/pages/PageStudio"
test ! -f "web/src/services/api/pages.ts" || ! rg -n "objectKey|WorkspaceConfig|TabEditor|LayoutDesigner|schemaToLayout|WorkspaceLayout|PageGenerator" "web/src/services/api/pages.ts"
```

禁止事项：

- 禁止把旧 layout editor 包一层标题后继续使用。
- 禁止工作台二次选择 `game_id/env`；当前 scope 只来自全局上下文。

### P1-2. 系统菜单和信息架构收敛

目标：

- 让后台菜单表达清晰边界，用户能理解每个入口做什么。

状态：2026-07-26 已完成第一轮菜单重排；`FunctionsAndPages` 为独立顶层入口，`SystemConfig` 只保留系统基础配置和扩展配置；动态运行菜单仍只由 `ConsoleMenuSpec` 注入。

受影响路径：

- `web/config/routes.ts`
- `web/src/locales/*/menu.ts`
- `web/src/pages/Console/index.tsx`
- `web/src/pages/Functions/*`
- `web/src/pages/ComponentManagement/*`
- `web/src/pages/PageStudio/*`

目标菜单：

```text
运行控制台
  首页
  动态分类
    已发布 Page

函数与页面
  函数目录
  函数调用
  函数实例
  函数告警
  资源/操作
  Page Studio

系统管理
  游戏/环境
  权限/用户/角色
  运维/Ops
```

实施要点：

- 固定系统菜单仍可使用静态 locale。
- 动态运行菜单不使用静态 locale。
- “实体管理”重命名或删除，替换为“资源/操作”。
- “页面管理”和“运行控制台”要区分：
  - Page Studio：草稿、生成、编辑、发布。
  - 运行控制台：只执行已发布页面。
- 前端不在 Page 内再出现一套 `game_id/env` 选择；所有 API 请求使用全局选择的 scope。
- 顶层“函数与页面”访问控制必须允许 `functions:*`、`resources:*` 或 `pages:*` 任一能力进入，子路由再按具体权限裁剪；不能让只有 `pages:read` 的用户看不到 Page Studio。

验收标准：

- 用户从左侧菜单能明确区分“编辑页面”和“执行页面”。
- 运行控制台动态分类显示用户配置的多语言标题。
- 任意 Page 内不会出现和全局冲突的 game/env 选择器。

验证命令：

```bash
pnpm --dir "web" exec eslint "src"
rg -n "实体管理|对象工作台|workspace|Workspace|game_id|env" "web/config/routes.ts" "web/src/pages" "web/src/locales"
./web/node_modules/.bin/jest --config "web/jest.config.ts" --runTestsByPath "web/tests/access.test.ts" --runInBand
```

禁止事项：

- 禁止因为动态菜单缺翻译就去改 `web/src/locales/*/menu.ts`。
- 禁止同一个概念同时叫 Entity、Object、Workspace、Resource。

### P1-3. 权限和审计模型迁移

目标：

- 权限命名跟 Page/Resource/Function 模型一致。

受影响路径：

- `configs/permissions.json`
- `internal/api/policy/*`
- `internal/logic/utils/function_acl.go`
- `web/src/utils/access.ts`
- `web/src/access.ts`

实施要点：

- 保留函数权限：`functions:read`、`function:invoke` 等按现状核对。
- 已新增或迁移：
  - `resources:read`
  - `resources:diagnose`
  - `pages:read`
  - `pages:edit`
  - `pages:publish`
  - `pages:rollback`
  - `pages:delete`
  - `console:read`
  - `openapi_sources:read`
  - `openapi_sources:write`
- 删除或替换 `workspace:*`、`entities:*` 中误导的权限描述。
- 保存、发布、回滚、取消发布、执行 Page binding 必须写审计日志，记录 pageKey、版本、操作者、诊断结果或执行结果。
- Resource API 必须在服务层校验 `resources:read/resources:diagnose`；Page API 必须在服务层校验 `pages:read/edit/publish/rollback`；Console API 必须在服务层校验 `console:read/pages:read/function:invoke`，执行 binding 还必须具备 `function:invoke`。
- OpenAPI Source API 必须在服务层校验 `openapi_sources:read/openapi_sources:write`，不得只靠前端隐藏上传或绑定按钮。
- OpenAPI Source 上传、Provider binding 创建和删除必须写审计日志，记录 `game_id/env/source_id/revision/binding_id/operation_id/function_id/provider_id` 中适用字段。
- 浏览器提交的操作者字段不可信；Page 保存、发布、回滚的 `updatedBy/publishedBy/createdBy` 只能从服务端上下文 `username` 获取。
- 前端 `access.ts` 不得用 `functions:read` 打开运行控制台；函数目录权限、Page Studio 权限和运行控制台权限必须分别判断。
- Console Page binding 执行必须生成 Page 层 `requestId`，并写入返回结果、审计 `AuditContext` 和下游 Function invoke metadata；metadata 至少包含 `page_key/binding_id/page_request_id/page_runtime_api/publish_version`。

验收标准：

- 前端按钮权限和后端 API 权限一致。
- 权限描述不再出现“对象工作台配置”。
- 审计能追踪 Page 发布和回滚。
- 审计能追踪 Page binding 执行，至少包含 `game_id/env/page_key/publish_version/binding_id/function_id/request_id/trace_id/actor/result_kind`。
- 无 Page 权限的用户不能保存、发布、回滚草稿；无 `function:invoke` 的用户不能执行运行控制台 binding。
- `updatedBy/publishedBy` 不再来自请求体。

验证命令：

```bash
rg -n "workspace:|entities:|entity:write|对象工作台|实体定义" "configs" "internal" "web/src"
GOCACHE="/tmp/croupier-go-build" GOMODCACHE="/tmp/croupier-go-mod" go test ./internal/api/page ./internal/api/resource ./internal/api/console
./web/node_modules/.bin/jest --config "web/jest.config.ts" --runTestsByPath "web/tests/access.test.ts" --runInBand
```

禁止事项：

- 禁止权限名和页面模型继续混用。

## 5. P1 数据迁移

### P1-4. 数据表和历史数据处理

目标：

- PageSpec / PublishedPageSpec / PageVersion 表结构稳定；历史旧 `workspace_configs` 不再作为兼容来源。
- 如果生产库仍存在旧 `workspace_configs` 数据，只能人工导出审计、生成报告或清理，禁止自动转换为可发布 Page。

受影响路径：

- `internal/model/config_version.go`
- `internal/model/*migration*`
- `internal/router/router.go`
- 数据库 migration/seed 目录，按项目现有结构定位

目标表建议：

```text
page_specs
  game_id
  env
  page_key
  type
  resource_key
  title_json
  description_json
  category_json
  order
  icon
  schema_json
  bindings_json
  metadata_json
  status
  draft_revision
  published_version
  created_at
  updated_at
  updated_by

published_page_specs
  game_id
  env
  page_key
  version
  spec_json
  binding_contracts_json
  renderer_schema_version
  active
  published_at
  published_by

page_versions
  game_id
  env
  page_key
  version
  spec_json
  status
  message
  created_at
  created_by
```

实施要点：

- 确认 `page_specs/published_page_specs/page_versions` 的唯一键均包含 `(game_id, env, page_key)`。
- 如果保留 `config_versions`，它只能记录 PageSpec 版本或非 Dashboard 配置，不能继续承载旧 workspace layout。
- 提供只读诊断脚本时，只输出报告，不写 PageSpec。
- 清理历史旧表前必须先备份并由用户确认；清理完成后运行态不再读取旧表。
- 不允许用默认 labels、默认 binding、默认 mapping 把旧 layout 补成可发布页面。

验收标准：

- 旧数据不会静默变成新页面。
- 历史旧数据不会出现在运行控制台。
- 不同 game/env 的同名 pageKey 读取、发布、版本和取消发布互不影响。

验证命令：

```bash
go test ./internal/model/... ./internal/api/page/...
rg -n "workspace_configs|WorkspaceConfigModel|FindByObjectKey|SetPublished|object_key" "internal" "web/src"
```

禁止事项：

- 禁止把旧 `published=true` 映射为新发布态。
- 禁止迁移时补猜缺失 labels 和 bindings 后直接发布。
- 禁止把无法确定 scope 的旧页面迁移为全局 pageKey。

## 6. P2 清理旧代码和防回归

### P2-1. 删除旧 Workspace Renderer 和 Mock

状态：2026-07-26 已物理删除；后续只保留防回归扫描。

目标：

- 清理旧 layout 渲染体系，避免后续 AI 继续误用。

受影响路径：

- `web/src/components/WorkspaceRenderer/*`
- `web/src/services/mock/workspaceMock.ts`
- `web/src/services/workspaceConfig.ts`
- `web/src/services/workspace/*`
- `web/src/pages/Workspaces/*`
- `web/src/pages/ComponentManagement/components/FunctionWorkspace.tsx`

已落地：

- `web/src/components/WorkspaceRenderer/*` 已删除。
- `web/src/services/mock/workspaceMock.ts` 已删除。
- `web/src/services/workspaceConfig.ts` 和 `web/src/services/workspace/*` 已删除。
- `web/src/pages/Workspaces/*`、`web/src/pages/WorkspaceEditor/*` 已删除。
- `web/src/pages/ComponentManagement/components/FunctionWorkspace.tsx` 已删除。
- `web/src/components/PageGenerator/*`、`web/src/components/FunctionFormRenderer/*`、`web/src/components/XUISchema.tsx` 已删除。
- `web/docs` 中旧 Workspace 用户/开发/API/治理文档已删除。
- `web/dist` 已重建，旧 Workspace/PageGenerator 运行产物已清理。

后续要点：

- 新建 Page Studio 时不得复用旧 layout renderer、workspace mock 或 workspaceConfig service。
- CI guard 必须阻止这些文件名和类型重新出现。

验收标准：

- 代码中没有运行态引用 `WorkspaceRenderer`。
- 没有 mock workspace 配置参与真实菜单和页面。

验证命令：

```bash
rg -n "WorkspaceRenderer|workspaceMock|workspaceConfig|WorkspaceConfig|WorkspaceLayout|TabLayout|PageGenerator|FunctionFormRenderer|XUISchema" "web/src"
pnpm --dir "web" exec eslint "src"
```

禁止事项：

- 禁止保留“deprecated but still works”的旧 renderer。

### P2-2. 删除旧后端 Workspace/Entity 模型残留

状态：2026-07-26 已物理删除；后续只保留路由/权限/guard 验收。

目标：

- 后端只保留 Page/Resource 模型。

受影响路径：

- `internal/api/workspace/*`
- `internal/api/entity/*`
- `internal/model/workspace_config.go`
- `internal/model/entity.go`
- `internal/model/entity_model.go`
- `internal/router/router.go`
- `internal/api/routes/service.go`

已落地：

- `internal/api/workspace/*` 已删除。
- `internal/api/entity/*` 已删除。
- `internal/model/workspace_config.go` 已删除。
- `internal/model/entity.go`、`internal/model/entity_model.go` 已删除。
- Provider 只读聚合接口已从 `/api/v1/providers/:id/entities` 改为 `/api/v1/providers/:id/resources`，Provider manifest 计数字段也改为 `resources`；不再读取 `x-entities`。

后续要点：

- 更新路由发现接口，确保没有旧 `Entities/Workspaces` 误导项。
- 若业务未来需要实体定义，必须另开独立 bounded context，不能混入 Dashboard Page 模型。

验收标准：

- `rg "WorkspaceConfig|EntityModel|/api/v1/entities"` 在运行代码中无命中。
- 测试不依赖旧 API。

验证命令：

```bash
go test ./...
rg -n "WorkspaceConfig|EntityModel|/api/v1/entities|workspace_configs|object_key" "internal" "web/src" "configs"
```

禁止事项：

- 禁止把旧代码移动到新目录继续被编译。

### P2-3. CI Guard

状态：2026-07-26 已新增 `.github/scripts/dashboard_guard.sh` 并接入 `.github/workflows/ci-dashboard.yml`；已覆盖旧前端路径、旧文档、旧 dist 产物和 Function UI 兼容 wrapper；后续可继续扩展 guard 覆盖后端 DTO 和 SDK 越界字段。

目标：

- 防止旧模式被重新引入。

受影响路径：

- `.github/workflows/ci-dashboard.yml`
- `.github/scripts/dashboard_guard.sh`

Guard 必须检查：

```bash
rg -n "WorkspaceConfig\\.layout|WorkspaceLayout|TabLayout" "web/src" "internal" && exit 1
rg -n "objectKey" "web/src/pages/Console" "web/src/services" "internal/api/page" && exit 1
rg -n "menu\\.ControlConsole\\.category\\." "web/src" && exit 1
rg -n "x-operation.*custom|CRUD operation type" "proto" "sdks" "web/src" && exit 1
test ! -f "web/src/services/pageStudio.ts" || ! rg -n "\\bany\\b" "web/src/services/pageStudio.ts"
rg -n "\\bany\\b" "web/src/types" "web/src/services/console.ts" && exit 1
rg -n "interface\\{\\}|map\\[string\\]interface\\{" "internal/api/page" "internal/api/function" && exit 1
rg -n '"functionId"' "web/src/components/FormilyPageRenderer" "web/src/pages/Console" && exit 1
rg -n "PageFunctionBinding.*Role|binding\\.Role|Role:.*Placement" "internal/dashboard" "internal/api/page" "internal/model/page_spec.go" && exit 1
rg -n "/api/v1/workspaces/pages|/api/v1/workspaces/:objectKey/config" "internal" "web/src" && exit 1
rg -n "category_display|entity_display|operation_display|operation_kind|page_hint|x-labels" "proto" "sdks" "internal" "web/src" \
  --glob "!internal/function/uicontract/reject.go" \
  --glob "!sdks/js/src/index.test.ts" && exit 1
rg -n "\"domain\"\\s*:\\s*\"entity\"|domain=entity|Domain:\\s*\"entity\"" "configs" "internal" "web/src" \
  --glob "!internal/api/terms/handler_test.go" \
  --glob "!internal/model/model_test.go" \
  --glob "!internal/svc/service_context_test.go" && exit 1
rg -n "ProvidersEntities|openAPIDocEntities|aggregateEntities|/providers/.*/entities|/:id/entities|x-entities" \
  "internal/api/provider" "internal/handler" "internal/router" "docs/api/provider.md" && exit 1
```

验收标准：

- CI 在旧概念重新出现时失败。
- guard 脚本有单独 README 或注释说明每条规则对应的设计约束。

验证命令：

```bash
bash .github/scripts/dashboard_guard.sh
```

禁止事项：

- 禁止 guard 只输出 warning 不失败。
- 禁止把旧词加入 allowlist 绕过检查，除非只匹配归档文档。

### P2-4. 文档和示例最终清理

目标：

- 文档不再误导执行者回到旧设计。

受影响路径：

- `docs/*`
- `sdks/*/README.md`
- `sdks/*/examples/*`
- `examples/*`

实施要点：

- 删除或重写旧 “对象工作台 / 实体管理 / WorkspaceConfig layout” 文档。
- SDK README 统一解释 descriptor v2。
- Dashboard 文档只保留 FunctionSpec -> ResourceSpec -> OperationSpec -> PageSpec -> PublishedPageSpec -> ConsoleMenuSpec -> PageExecutionResult。
- 示例必须覆盖中文/英文 labels。

验收标准：

- `docs` 搜索不到把 WorkspaceConfig 作为当前设计的内容。
- SDK 示例能被开发者复制后注册出可生成 PageSpec 建议的函数。

验证命令：

```bash
rg -n "WorkspaceConfig|对象工作台|实体管理|layout 协议|x-operation.*custom" "docs" "sdks"
pnpm --dir "docs" build
```

禁止事项：

- 禁止在文档中保留“兼容旧版”的主路径描述。

## 7. 最小端到端验收场景

执行 AI 完成重构后，必须至少交付以下场景。这里的 Entity Page 是 Page 类型，和已删除的旧 Entity CRUD API 不是同一概念。

### 场景 A：玩家管理 Entity Page

输入函数：

- `player.list`
  - `resource=player`
  - `operation=list`
  - input 有分页字段
  - output 有 `items/total`
  - PageContract 有分页、列、items/total 和 outputMapping
- `player.get`
  - `operation=get`
  - output 有玩家详情 schema
- `player.ban`
  - `operation=ban`
  - `risk=danger`
  - PageContract 或 Page Studio mapping 明确 `playerId <- row.id`

期望：

- Resource API 返回 `player` Resource 和 3 个 Operation。
- Generated Page 返回 `player.manage`，质量至少 `needs_review`，字段完整时为 `ready`。
- Page Studio 可预览、保存、发布。
- 运行控制台菜单显示“玩家管理”。
- 页面支持查询、分页、详情、封禁行操作。
- `player.list` 的分页输入、items/total、稳定列和 `player.ban` 的 `playerId <- row.id` 映射必须可由契约验证。
- 执行经过 `bindingId` 受控入口，审计与 trace 记录页面发布版本。

### 场景 B：邮件发送 Operation Page

输入函数：

- `mail.send`
  - `resource=mail` 或无 resource
  - `operation=send`
  - input/output schema 完整

期望：

- 不强行进入玩家/对象管理页。
- 生成独立 Operation Page。
- 菜单分类按显式 category 或 `mail` 默认规则。
- 页面是表单 + 结果，不出现表格分页。

### 场景 C：奖励批量发放 Task Page

输入函数：

- `reward.batchGrant`
  - `operation=batchGrant`
  - execution mode 或 PageContract 明确为 task
  - task status/events/result 来源明确

期望：

- 生成 Task Page。
- 页面可启动任务、查看任务事件、查看结果。
- 长耗时状态不伪装成同步 action。
- taskId 来自受控 binding execute，TaskTimeline 对接真实 task status/events/result，而不是 renderer 内存状态。

### 场景 D：留存分析 Report Page

输入函数：

- `analytics.retention`
  - `operation=retention`
  - output 有报表数据结构
  - PageContract 明确 chart type、series/category/value 路径

期望：

- 生成 Report Page。
- PageSpec 明确筛选字段、series/items 映射。
- 图表组件不猜 response shape。
- 缺少 chart type 或字段映射时只能 `needs_review/blocked`，不能生成 ready 图表。

### 场景 E：缺字段函数待编排

输入函数：

- `cache.refresh`
  - 只有 `id/summary/input_schema`
  - 缺 `resource/operation/output_schema/PageContract`

期望：

- 函数目录可见。
- Resource/Generated Page 返回 diagnostics。
- 不能发布到运行控制台。
- 前端不偷偷生成菜单。

## 8. 全局 Scope 规则

背景：

- 当前前端已有全局 `game_id/env` 选择。
- 页面内部再次出现 `game_id/env` 会造成用户不知道以哪个为准。

目标：

- Dashboard、Page Studio、运行控制台所有 API 通过统一请求头与服务端权限解析使用全局 scope。
- PageSpec 中不内置 scope selector。跨环境运维属于独立、显式设计的聚合能力，不能借 metadata 绕过标准 scope。

实施要点：

- 请求层统一注入当前 `gameId/env`。
- Page binding 调用函数时由受控执行服务从全局 scope 取 game/env。
- Page Studio 保存和发布也使用全局 scope。
- 后端按 scope 隔离 PageSpec、PublishedPageSpec、ResourceSpec 视图；PageIdentity 是 `(game_id, env, page_key)`。

验收标准：

- 任意运行控制台 Page 内没有第二套 game/env 选择器。
- 切换全局 game/env 后，菜单和页面按新 scope 重新加载。
- API 层测试覆盖 scope 隔离。
- 同一个 pageKey 在两个 scope 中可同时存在，读取、发布、版本与取消发布互不影响。

验证命令：

```bash
rg -n "game_id|gameId|env" "web/src/pages/Console" "web/src/components/FormilyPageRenderer"
test ! -d "web/src/pages/PageStudio" || ! rg -n "game_id|gameId|env" "web/src/pages/PageStudio"
go test ./internal/api/console/... ./internal/api/page/... ./internal/api/resource/...
```

禁止事项：

- 禁止 Page 内部默认渲染 game/env 字段。
- 禁止前端把 URL 中的 category/pageKey 当作 scope。
- 禁止 Page request body/payload 覆盖请求上下文 scope。

## 9. 可观测性与执行追踪

目标：

- Page 操作、函数调用、任务执行能接入已有 tracing/metrics/logging，方便定位运营问题。

受影响路径：

- `internal/api/functioncall/*`
- `internal/api/task/*`
- `internal/logic/function/function_invoke_logic.go`
- `web/src/services/*`
- OTEL 初始化相关目录，按现有实现定位

实施要点：

- Page renderer 通过 binding execute API 传递 `pageKey/bindingId`，由服务端解析 functionId/usage；浏览器不传任意 route、target 或 scope。
- 后端函数调用 span 记录：
  - `game_id`
  - `env`
  - `page_key`
  - `function_id`
  - `page_binding_usage`
  - `page_component`
  - `publish_version`
  - `binding_id`
  - `task_id`
  - `risk`
- 前端错误上报包含 PageSpec version 和 component path。
- 发布 Page 时记录校验 diagnostics 和版本。

验收标准：

- 能从一次运行控制台按钮点击追踪到后端函数调用或任务。
- Task Page 能关联 task events 和 pageKey。
- Approval Page execution 能关联 approvalId、Page publish version 和函数策略。
- 日志不打印敏感 payload。

验证命令：

```bash
rg -n "page_key|pageKey|function_id|functionId|binding_id|bindingId|trace|otel" "internal" "web/src"
go test ./internal/api/functioncall/... ./internal/api/task/... ./internal/logic/function/...
```

禁止事项：

- 禁止把完整请求 payload 放进 span attribute 或日志。

## 10. 审核清单

审核 AI 必须按以下规则拒绝不合格提交。

必须拒绝：

- 仍然从 `WorkspaceConfig[]` 生成运行控制台菜单。
- 仍然在 `web/src/locales/*/menu.ts` 添加动态分类。
- 仍然用 `objectKey` 作为运行控制台页面主参数。
- 仍然保留旧 `layout` 协议作为运行时页面协议。
- 仍然把 Function Form 和 Page UI 做成两套协议。
- 把 Function Form 的 `type: object` 根节点规则错误套用到 Page UI 的 `type: void + ConsolePage` 根节点，或反过来放宽 Function Form 校验。
- 仍然使用 `any/interface{}` 承载核心 DTO。
- 仍然根据函数名猜字段、猜分页、猜页面类型。
- 仍然把所有函数塞进对象管理页。
- 仍然让缺 PageContract、mapping、默认语言 labels 或 binding 的页面发布成功。
- 仍然在 Page 内重复选择 `game_id/env`。
- 仍然让 `page_key` 全局唯一、让 Page body/URL/payload 覆盖 scope，或不按 `(game_id, env, page_key)` 隔离查询与版本。
- 仍然用 `PageFunctionBinding.role` 复用 `OperationPlacement`，或让 schema 组件写裸 `functionId`。
- 仍然在发布后从最新函数 descriptor 静默读取输入/输出/风险/权限，未检测 binding stale。
- 仍然让 Page renderer 调用通用 function invoke API，或由浏览器选择 route/target/scope。
- 仍然允许未知 Page schemaVersion、未知组件或未知关键 props 发布/渲染。
- 仍然用首批返回字段、整行 row JSON 或“最后结果”隐式推断表格列、动作输入和跨组件数据流。
- 仍然把字典作为动态菜单分类事实源。
- 仍然只在本地写死 mock 数据验证动态菜单。

必须确认：

- `FunctionSpec -> ResourceSpec + OperationSpec -> PageSpec -> PublishedPageSpec -> ConsoleMenuSpec` 链路可运行。
- Page Studio 和运行控制台边界清楚。
- Page Studio、运行控制台和函数目录的执行边界清楚。
- 函数注册后能生成默认 PageSpec 建议，但不会未经确认自动发布。
- 多语言动态标题来自 PageSpec 的 `title/category.labels`。
- 全局 scope 是唯一 game/env 来源。
- PublishedPageSpec 含 binding contract snapshot；函数契约变更能检测 stale 并阻止危险静默执行。
- 页面执行的 audit/trace 可关联 pageKey、publishVersion、bindingId、functionId、scope 和 task/approval。
- Entity/Operation/Task/Report 四类页面都有最小端到端用例。
- CI guard 会阻断旧模型回流。

## 11. 建议执行顺序

建议按以下顺序提交，避免大爆炸后无法审核：

1. 提交强类型模型、PageIdentity、binding 模型和 descriptor v2，不改运行页面行为。
2. 提交 normalizer、`x-page-contract` 和 Resource/Operation API，补诊断测试。
3. 提交 PageSpec ABI validator（schemaVersion、组件 props、mapping）和 generator；此时禁止 ready 页面使用猜测 mapping。
4. 提交带 scope/revision/binding snapshot 的 Page Studio API、数据模型和迁移。
5. 提交 Console API，按 scope 读取 active PublishedPageSpec 和 ConsoleMenuSpec。
6. 提交受控 binding execute API、审批/任务/审计/Otel contract 与后端测试。
7. 提交前端 ConsoleMenuSpec 菜单接入，删除 workspaceConfigs 菜单注入。
8. 提交 binding/page-state 驱动的 FormilyPageRenderer 和四类最小端到端页面。
9. 提交新 Page Studio 前端与冲突/契约失效交互。
10. 提交旧 WorkspaceRenderer/Entity/WorkspaceConfig 防回归 guard。
11. 提交 CI guard、文档和 SDK demo 收尾。

每个提交都必须包含：

- 变更目标。
- 影响路径。
- 验证命令和结果。
- 是否还有 diagnostics 或 blocked 场景。

## 12. 完成定义

本次重构完成必须同时满足：

- 后端 API 不再暴露旧 WorkspaceConfig 作为当前页面模型。
- 前端运行控制台不再依赖 `workspaceConfigs`。
- 动态左侧菜单来自 `ConsoleMenuSpec`。
- Page Studio 保存、预览、发布的是 PageSpec。
- Page Studio 的保存按 scope + revision 隔离，发布产物冻结 binding contract，运行控制台通过 binding execute API 执行。
- Function Form 和 Page UI 都是 Formily JSON Schema。
- Function Form 与 Page UI 的根节点、组件 ABI、schema version 和校验规则明确且可机器验证。
- SDK/OpenAPI descriptor 只表达资源、业务动作、风险、权限和输入/输出契约；分类、多语言页面标题、页面语义和放置位置只在 PageSpec/Page Studio 中确定。
- 缺语义的函数不会被自动发布。
- 文档、示例、权限、CI guard 都与新模型一致。
- `go test ./...`、`pnpm --dir "web" exec eslint "src"`、`pnpm --dir "docs" build` 通过。
