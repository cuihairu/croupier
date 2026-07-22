# Dashboard Resource/Page 重构 TODO

更新时间：2026-07-22

本文是 Dashboard 动态页面、函数注册描述符、Page 工作台和运行控制台菜单的重构交接清单。执行 AI 必须按本文推进；审核 AI 以本文和权威设计文档为验收依据。

权威设计文档：

- `docs/architecture/dashboard-page-model.md`
- `docs/architecture/openapi-sdk-descriptor-v2.md`
- `docs/design/console-dynamic-menu.md`
- `docs/api/workspace.md`
- `docs/api/entity.md`

## 0. 硬约束

本次重构不是在旧 WorkspaceConfig 上继续打补丁，而是把 Dashboard UI 链路收敛为唯一模型：

```text
SDK / OpenAPI / DB Template
  -> RawFunctionDescriptor
  -> FunctionSpec
  -> ResourceSpec + OperationSpec
  -> GeneratedPageSpec
  -> PageSpecDraft
  -> PublishedPageSpec
  -> ConsoleMenuSpec
```

必须遵守：

- Function 不是 Page。函数注册只产生可执行能力和单函数 Formily 输入表单。
- Function UI 只负责单函数输入表单，不负责菜单、分页、表格、详情、页面布局。
- PageSpec 是唯一页面编排协议，`schema` 必须是 Formily JSON Schema。
- 运行控制台左侧菜单唯一来源是 `GET /api/v1/console/menu` 返回的 `ConsoleMenuSpec`。
- 动态分类、资源、页面标题必须来自 PageSpec metadata 的多语言 labels，不写入静态 locale 文件。
- 缺少 `operationKind`、`placement`、默认语言 labels、Formily schema、函数绑定时，发布必须失败或进入待编排状态。
- 没有 `input_schema` 时，单函数 UI 只能生成单个 `payload` 字段，禁止根据函数名猜 `player_id`、`reason` 等业务字段。
- 不保留多套 UI 协议，不兼容旧 `layout` 渲染协议，不新增字典作为动态菜单事实源。
- 不使用 TypeScript `any` 或 Go `interface{}` 承载核心 DTO；扩展字段必须使用明确的 JSON 类型别名或 `json.RawMessage`。
- 不把所有函数强行塞进对象管理页；必须区分 Entity Page、Operation Page、Task Page、Report Page。

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

## 1. 当前问题快照

### 1.1 后端 Descriptor 仍是旧模型

现状：

- `proto/croupier/sdk/v1/provider.proto` 的 `LocalFunctionDescriptor` 只有 `category/risk/entity/operation`。
- `operation` 注释仍写 CRUD/custom，和目标设计冲突。
- `internal/logic/function/descriptors_logic.go` 返回 `[]map[string]interface{}`，没有强类型 `FunctionSpec`。
- descriptor 解析只覆盖 `x-category/x-risk/x-entity/x-operation`，缺 `x-operation-kind/x-placement/x-*-display/page_hint`。
- `descriptors_logic.go` 中仍存在 `defaultMenu`、`menuSource`、`metadata.menu`、`inferCategory`、`applyEntityMenuDefaults` 等旧推断。
- `internal/api/function/dto.go` 仍有 `interface{}` DTO，Function UI response 仍可能暴露 `Layout/Components` 旧字段。

目标：

- 后端必须先归一化为强类型 `FunctionSpec/ResourceSpec/OperationSpec`，再生成 PageSpec 建议。
- 前端不再读取原始 OpenAPI 或函数 ID 推断页面。

### 1.2 Workspace / Entity 仍是旧页面模型

现状：

- `internal/api/workspace/dto.go` 仍定义 `WorkspaceConfig{ObjectKey, Layout interface{}, Category}`。
- `internal/api/workspace/service.go` 在配置不存在时自动创建默认 `tabs` layout。
- `internal/model/workspace_config.go` 仍以 `object_key` 管理页面配置。
- `web/src/types/workspace.ts` 仍定义 `WorkspaceConfig`、`WorkspaceLayout`、`TabLayout`、`FieldConfig` 等旧 layout 协议。
- `web/src/services/workspaceConfig.ts` 仍调用 `/api/v1/workspaces/*` 旧接口。
- `web/src/components/WorkspaceRenderer/*` 仍渲染旧 layout 协议。
- `internal/api/entity/*` 仍是旧实体接口，和文档中的 Resource API 目标不一致。

目标：

- Page 工作台 API 使用 `PageSpecDraft`，运行态使用 `PublishedPageSpec`。
- Resource API 只提供资源/操作归一化查询和诊断，不做通用实体 CRUD。

### 1.3 运行控制台菜单仍从前端旧数据注入

现状：

- `web/src/app.tsx` 在 `getInitialState` 加载 `workspaceConfigs`。
- ProLayout `menu.request` 调 `buildConsoleMenuData(defaultMenuData, workspaceConfigs)`。
- `web/src/services/workspace/menu.ts` 从 `WorkspaceConfig[]` 注入菜单。
- `web/src/services/workspace/navigation.ts` 使用 `objectKey` 推分类，并生成 `menu.ControlConsole.category.${categoryKey}`。
- `web/config/routes.ts` 的动态路由仍是 `/console/:categoryKey/:objectKey`。
- `web/src/pages/Console/Workspace.tsx` 仍按 `objectKey` 加载 WorkspaceRenderer。

目标：

- 前端只消费 `ConsoleMenuSpec`。
- 动态菜单项 `locale: false`，直接使用当前语言解析出的 label。
- 路由改为 `/console/:categoryKey/:pageKey`，页面由 `PublishedPageSpec` + Formily 渲染。

### 1.4 SDK Demo 和 OpenAPI 类型仍未表达 v2

现状：

- `web/src/services/api/openapi.ts` 仍可能把 `x-operation` 限定为 CRUD/custom。
- 多语言 SDK demo 未统一展示 `category_display/entity_display/operation_display/operation_kind/placement/page_hint`。
- SDK 注册文档和例子容易让执行者继续误解 `operation` 是页面类型。

目标：

- v2 字段在 proto、SDK builder、OpenAPI import、demo、文档中一致。
- `operation` 表示业务动作 key，`operationKind` 才表示页面生成语义。

## 2. 目标模型定义

### 2.1 FunctionSpec

职责：

- 表示单个注册函数的可执行能力。
- 承载输入/输出 JSON Schema、风险、简介、多语言显示名。
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
  displayName?: LocalizedText;
  summary?: LocalizedText;
  description?: LocalizedText;
  category?: string;
  categoryDisplay?: LocalizedText;
  entity?: string;
  entityDisplay?: LocalizedText;
  operation?: string;
  operationDisplay?: LocalizedText;
  operationKind?: OperationKind;
  placement?: OperationPlacement;
  pageHint?: string;
  risk?: RiskLevel;
  tags?: string[];
  diagnostics?: Diagnostic[];
}
```

约束：

- `inputFormilySchema` 必须是 Formily JSON Schema。
- `ui/x-ui` 如果不是 Formily，直接报错。
- `summary/description/displayName` 不能只做前端临时显示，必须进入强类型 DTO。

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

- 描述函数在 Resource/Page 中的业务动作、页面生成语义和放置位置。

```ts
type OperationKind =
  | 'list'
  | 'get'
  | 'create'
  | 'update'
  | 'delete'
  | 'action'
  | 'task'
  | 'report';

type OperationPlacement =
  | 'query'
  | 'tableData'
  | 'detailData'
  | 'rowAction'
  | 'detailAction'
  | 'toolbarAction'
  | 'batchAction'
  | 'standalone';

interface OperationSpec {
  functionId: string;
  resourceKey?: string;
  operation: string;
  kind: OperationKind;
  placement: OperationPlacement;
  labels: LocalizedText;
  risk?: RiskLevel;
  enabled: boolean;
  diagnostics?: Diagnostic[];
}
```

理解方式：

- `operation` 是业务动作 key，例如 `ban/grant/send/list`。
- `kind` 或 `operationKind` 是页面生成语义，例如 `list/action/task/report`。
- `placement` 是页面放置位置，例如 `tableData/rowAction/standalone`。

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
  functionId: string;
  role: OperationPlacement;
}
```

分页约束：

- 分页是 Page 的职责，不是 Function UI 的职责。
- `pageField/pageSizeField/itemsField/totalField` 必须显式写入 PageSpec。
- 函数分页字段不同，不允许前端运行时猜测。

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

目标：

- 在后端和前端建立唯一模型词汇，先让代码有正确类型，避免继续扩散 `WorkspaceConfig/layout/objectKey`。

受影响路径：

- `internal/api/function/dto.go`
- `internal/logic/function/types.go`
- `internal/api/workspace/dto.go`
- `internal/model/workspace_config.go`
- `web/src/types/workspace.ts`
- 新增建议：`internal/dashboard/spec/*`、`web/src/types/dashboard.ts`

实施要点：

- 新增 Go 强类型：`LocalizedText`、`JSONSchema`、`FormilySchema`、`FunctionSpec`、`ResourceSpec`、`OperationSpec`、`PageSpec`、`PublishedPageSpec`、`ConsoleMenuSpec`、`Diagnostic`。
- 新增 TS 强类型，和 Go JSON 字段名保持一致。
- 核心 DTO 禁止 `interface{}`；必须用 `json.RawMessage`、`map[string]string`、枚举 string type 或明确结构体。
- TS 禁止 `any`；临时未知 JSON 只能使用 `unknown` 或 `JSONValue`，并在边界处 parse/validate。
- `objectKey` 只允许出现在旧迁移脚本和删除阶段的 TODO 注释中，新 API 使用 `pageKey/resourceKey/functionId`。
- `WorkspaceConfigCanonical`、alias 类型、重复 DTO 必须删除。

验收标准：

- 后端存在可复用的强类型 spec 包，API DTO 不再重复定义同一套结构。
- 前端存在 `dashboard.ts` 或等价单一类型入口，页面、服务、renderer 共用该入口。
- 新增或重构后的核心 DTO 中没有 `interface{}`。
- 新增或重构后的前端 dashboard 类型中没有 `any`。

验证命令：

```bash
rg -n "interface\\{\\}|map\\[string\\]interface\\{|\\bany\\b" "internal/api" "internal/logic/function" "web/src/types" "web/src/services"
rg -n "WorkspaceConfigCanonical|SaveConfigRequestAlias|GetConfigRequestAlias" "internal/api/workspace"
```

禁止事项：

- 禁止为了快而在 DTO 上继续保留 `Layout interface{}`。
- 禁止定义一套 Go 模型、一套不一致 TS 模型。
- 禁止用 `Record<string, any>` 逃避建模。

### P0-2. 升级 SDK / OpenAPI Descriptor v2

目标：

- 函数注册能明确提供 Page 生成所需语义。

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

- `LocalFunctionDescriptor` 新增一等字段：
  - `map<string,string> category_display`
  - `map<string,string> entity_display`
  - `map<string,string> operation_display`
  - `string operation_kind`
  - `string placement`
  - `string page_hint`
  - `map<string,string> extensions`
- 更新 `operation` 注释为业务动作 key，不再写 CRUD/custom。
- 更新所有 generated protobuf 产物。
- SDK builder 增加 v2 字段设置方法，命名按各语言风格统一。
- SDK demo 至少覆盖 `player.list`、`player.get`、`player.ban`、`mail.send`、`reward.batchGrant`、`analytics.retention`。
- OpenAPI import 支持 `x-category-display`、`x-entity-display`、`x-operation-display`、`x-operation-kind`、`x-placement`、`x-page-hint`。
- `web/src/services/api/openapi.ts` 不再把 `x-operation` 限定为 CRUD/custom。

验收标准：

- Go/JS/Python/Java/C#/C++ demo 都能表达同一组 v2 字段。
- `operation=ban`、`operationKind=action`、`placement=rowAction` 这类组合可完整注册并在 API 中返回。
- 旧 SDK 不提供 v2 字段时，函数可注册，但 Page 自动发布必须 blocked。

验证命令：

```bash
rg -n "CRUD operation type|create.*read.*update.*delete.*custom|x-operation.*custom" "proto" "sdks" "web/src/services/api"
rg -n "operation_kind|operationKind|x-operation-kind|placement|x-placement|category_display|categoryDisplay" "proto" "sdks" "internal" "web/src"
```

禁止事项：

- 禁止继续把 `operation` 当页面类型。
- 禁止只在某一个 SDK demo 里补字段，其他语言缺失。
- 禁止用 `extensions` 永久替代一等字段；`extensions` 只能作为第三方扩展出口。

### P0-3. 建立 Descriptor Normalizer

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
- Locale key 统一归一化为 `zh-CN/en-US` 等完整 key。
- `x-ui` 或 `ui` 必须通过 Formily 校验。
- `input_schema` 存在时，由 JSON Schema 派生默认 Formily 输入表单。
- `input_schema` 不存在时，只生成 `payload` 单字段 Formily 表单，并给出 `input_schema_missing` warning。
- 生成 `OperationSpec` 时，缺 `operationKind` 或 `placement` 标记 `blocked/needs_review`，不能进入可发布 Page。
- 删除 `defaultMenu`、`menuSource`、`metadata.menu`、`inferCategory`、`applyEntityMenuDefaults` 作为页面/菜单来源的逻辑。
- `termDictModel` 只能用于枚举、状态、下拉选项，不参与动态导航事实源。

验收标准：

- `GET /api/v1/functions/descriptors` 返回强类型字段，包含 v2 语义和 diagnostics。
- 缺 `operationKind` 的函数能在函数目录看到，但不会生成 ready Page。
- 非 Formily UI 保存和发布都会失败。
- 分类推断只发生在归一化或 Page 保存/发布阶段，并写入 PageSpec。

验证命令：

```bash
go test ./internal/logic/function/... ./internal/api/function/...
rg -n "defaultMenu|menuSource|metadata\\.menu|inferCategory|applyEntityMenuDefaults" "internal/logic/function"
rg -n "\\[\\]map\\[string\\]interface\\{|map\\[string\\]interface\\{\\}" "internal/logic/function" "internal/api/function"
```

禁止事项：

- 禁止前端补齐 `operationKind/placement`。
- 禁止把字典重新引入为菜单主模型。
- 禁止 silent fallback 成健康页面。

### P0-4. Function UI 收敛为单一 Formily

目标：

- 单函数调用、弹窗输入、PageSpec 局部表单统一使用 Formily Schema。

受影响路径：

- `internal/logic/function/function_u_i_logic_v2.go`
- `internal/logic/function/function_u_i_update_logic.go`
- `internal/logic/function/function_u_i_rollback_logic.go`
- `internal/api/function/dto.go`
- `web/src/components/FunctionFormRenderer/index.tsx`
- `web/src/components/FunctionUIManager/index.tsx`
- `web/src/pages/Functions/SchemaDesigner/*`
- `web/src/utils/json.ts`

实施要点：

- Function UI DTO 只保留 `schema: FormilySchema`、版本、来源、诊断。
- 删除或拒绝 `layout/components/fields` 等第二套 UI 协议字段。
- `json.ts` 中 JSON Schema -> Formily 的推断逻辑必须输出严格类型，不使用 `any`。
- 单函数调用页面只渲染 Formily Schema。
- PageSpec 中的 `QueryForm/ActionButton` 通过 `formSchemaRef` 或内联 Formily Schema 引用函数 UI。

验收标准：

- UI 更新接口提交非 Formily schema 返回 400。
- 所有函数调用表单和 Page 动作弹窗使用同一个 Formily renderer。
- 代码中不存在同时维护 `FieldConfig` 和 Formily Schema 的新链路。

验证命令：

```bash
go test ./internal/logic/function/... ./internal/api/function/...
pnpm --dir "web" exec eslint "src/components/FunctionFormRenderer/index.tsx" "src/components/FunctionUIManager/index.tsx" "src/pages/Functions/SchemaDesigner"
rg -n "layout|components|FieldConfig" "internal/api/function" "web/src/components/FunctionUIManager" "web/src/components/FunctionFormRenderer"
```

禁止事项：

- 禁止为了兼容旧 UI 同时渲染两套结构。
- 禁止 schema 校验失败后自动降级为空表单。

### P0-5. Resource API 替换旧 Entity API

目标：

- 建立 Dashboard 页面生成需要的 Resource/Operation 查询接口，删除“实体管理就是页面”的误导。

受影响路径：

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

实施要点：

- 新增或重命名为 `/api/v1/resources`：
  - `GET /api/v1/resources`
  - `GET /api/v1/resources/:resourceKey`
  - `GET /api/v1/resources/:resourceKey/operations`
  - `GET /api/v1/resources/:resourceKey/pages/generated`
- Resource API 只读归一化 spec 和 diagnostics，不提供业务对象 CRUD。
- 旧 `/api/v1/entities` 删除或改为非 Dashboard 内部接口；如果当前代码仍依赖，必须先迁移依赖再删。
- 前端“实体管理”改为“资源建模”或“资源/操作”，页面解释清楚 Resource 不是数据库实体。
- 权限从 `entities:*` 收敛为 `resources:read`、`resources:diagnose` 或按实际需要命名。

验收标准：

- 用户在系统菜单中不会再看到误导性的“实体管理”作为业务对象 CRUD。
- Resource 页面能看到 player/mail/reward/analytics 等资源及其 operations、diagnostics。
- Resource API 不出现 create/update/delete 业务对象接口。

验证命令：

```bash
go test ./internal/api/entity/... ./internal/api/openapi/... ./internal/router/...
pnpm --dir "web" exec eslint "src/pages" "src/services"
rg -n "/api/v1/entities|entities:|entity:write|实体管理|XEntityForm" "internal" "web/src" "configs"
```

禁止事项：

- 禁止把 Resource API 做成新的通用 CRUD。
- 禁止继续在运行控制台里出现 `game_id/env` 二次选择。

### P0-6. Page 工作台 API 替换 WorkspaceConfig API

目标：

- 后端以 `PageSpecDraft/PublishedPageSpec` 管理页面草稿、预览、校验、发布、版本和回滚。

受影响路径：

- `internal/api/workspace/*`
- `internal/model/workspace_config.go`
- `internal/model/config_version.go`
- `internal/router/router.go`
- 新增建议：`internal/api/page/*` 或保留包名 `workspace` 但 DTO 全部改为 Page 工作台模型

实施要点：

- API 改为文档目标：
  - `GET /api/v1/pages/generated?resourceKey=...`
  - `GET /api/v1/workspaces/pages`
  - `GET /api/v1/workspaces/pages/:pageKey`
  - `PUT /api/v1/workspaces/pages/:pageKey`
  - `POST /api/v1/workspaces/pages/:pageKey/validate`
  - `POST /api/v1/workspaces/pages/:pageKey/preview`
  - `POST /api/v1/workspaces/pages/:pageKey/publish`
  - `POST /api/v1/workspaces/pages/:pageKey/unpublish`
  - `GET /api/v1/workspaces/pages/:pageKey/versions`
  - `GET /api/v1/workspaces/pages/:pageKey/versions/:versionId`
  - `POST /api/v1/workspaces/pages/:pageKey/rollback`
- `Save` 不再保存 `layout`，只保存完整 `PageSpec`。
- `Get` 不存在时返回 404，不自动创建默认 tabs layout。
- 发布前执行完整校验：Formily、labels、category、bindings、输出映射、权限。
- 发布时生成不可变 `PublishedPageSpec` 快照。
- 版本 key 从 `workspace:{objectKey}` 迁移为 `page:{pageKey}`。
- 如果保留 `workspace` 包名作为“Page 工作台”，包内不允许再出现 `WorkspaceConfig` 类型。

验收标准：

- 旧 `/api/v1/workspaces/:objectKey/config` 不再被前端调用。
- 缺配置不会自动创建页面。
- 发布失败能返回可读 diagnostics。
- PublishedPageSpec 可以独立供运行控制台读取。

验证命令：

```bash
go test ./internal/api/workspace/... ./internal/model/... ./internal/router/...
rg -n "WorkspaceConfig|objectKey|layout|/api/v1/workspaces/[^\\\"]*/config|workspaces/published" "internal/api/workspace" "internal/model" "web/src"
```

禁止事项：

- 禁止保留 `WorkspaceConfig` 兼容 DTO。
- 禁止不存在草稿时自动生成并保存默认 layout。
- 禁止发布时对缺失字段做静默补全。

### P0-7. Console API 和动态菜单

目标：

- 运行控制台菜单从 Server 发布产物读取，不再由前端组装。

受影响路径：

- `internal/router/router.go`
- 新增建议：`internal/api/console/*`
- `web/src/app.tsx`
- `web/src/services/workspace/menu.ts`
- `web/src/services/workspace/navigation.ts`
- `web/config/routes.ts`
- `web/src/pages/Console/index.tsx`
- `web/src/pages/Console/Workspace.tsx`

实施要点：

- 新增：
  - `GET /api/v1/console/menu`
  - `GET /api/v1/console/pages`
  - `GET /api/v1/console/pages/:pageKey`
- `console/menu` 从 `PublishedPageSpec[]` 生成 `ConsoleMenuSpec`。
- `console/pages/:pageKey` 返回单个 `PublishedPageSpec`，不返回草稿。
- 菜单按 `category.order/page.order/title` 排序。
- 菜单标题按请求语言或用户语言解析；也可返回完整 labels 由前端选择，但动态项必须 `locale: false`。
- `web/src/app.tsx` 删除 `workspaceConfigs` initialState。
- ProLayout `menu.request` 请求 `ConsoleMenuSpec`，并合并固定系统菜单。
- 固定路由：
  - `/console/home`
  - `/console/:categoryKey`
  - `/console/:categoryKey/:pageKey`
- 若 URL 分类与 PageSpec 发布分类不一致，跳转规范路径。

验收标准：

- 新增动态分类不需要改前端代码和 locale 文件。
- 发布 Page 后刷新页面，左侧菜单出现对应分类和页面。
- 取消发布 Page 后刷新页面，菜单消失。
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
- `QueryForm` 负责提交查询函数，写入页面上下文状态。
- `DataTable` 显式读取 `itemsField/totalField`，维护分页并触发 query function。
- `ActionButton` 根据 binding 打开 Formily 弹窗并调用函数。
- `TaskTimeline` 对接现有 Task start/events/result 链路。
- `ChartPanel` 只读取 PageSpec 中明确声明的数据路径。
- 删除旧 `WorkspaceRenderer` 或将其彻底迁移为 Formily renderer，不保留旧 layout renderer。
- `PageSchemaRenderer.tsx` 如果不是目标 Formily PageSpec renderer，删除或改名，避免误用。

验收标准：

- `/console/:categoryKey/:pageKey` 能渲染 Entity/Operation/Task/Report 四类页面的最小可用版本。
- 分页、行操作、工具栏操作、批量操作都来自 PageSpec 显式配置。
- 旧 `TabsLayout/FormDetailRenderer/ListRenderer/GridRenderer/...` 不再被运行控制台引用。

验证命令：

```bash
pnpm --dir "web" exec eslint "src/pages/Console" "src/components"
rg -n "WorkspaceRenderer|TabsLayout|FormDetailRenderer|ListRenderer|GridRenderer|KanbanRenderer|WizardRenderer|DashboardRenderer" "web/src"
rg -n "\\bany\\b" "web/src/components/FormilyPageRenderer" "web/src/components/console-page-components" "web/src/pages/Console"
```

禁止事项：

- 禁止运行时把 PageSpec 转成旧 layout 再渲染。
- 禁止缺字段时随便显示一个空页面并当作成功。

### P0-9. PageSpec Generator

目标：

- 根据归一化后的 Resource/Operation 生成默认 PageSpec 建议，让函数注册后有可预览的默认界面。

受影响路径：

- 新增建议：`internal/dashboard/generator/*`
- `internal/api/workspace/*`
- `internal/api/entity/*` 或 Resource API 包
- `internal/logic/function/*`

实施要点：

- 输入只接受 `ResourceSpec + OperationSpec[]`。
- 输出 `GeneratedPageSpec[]`，带 `quality=ready/needs_review/blocked` 和 diagnostics。
- Entity Page 生成规则：
  - 有 `list/tableData` 时生成 `DataTable`。
  - 有 `query` 时生成 `QueryForm`。
  - 有 `get/detailData` 时生成 `DetailPanel`。
  - `rowAction/detailAction/toolbarAction/batchAction` 生成对应 `ActionGroup`。
  - 响应字段映射不明确时 `needs_review` 或 `blocked`，不硬猜。
- Operation Page 生成规则：
  - `operationKind=action` 且 `placement=standalone` 生成单页表单 + 结果面板。
- Task Page 生成规则：
  - `operationKind=task` 生成参数表单 + TaskTimeline + ResultPanel。
- Report Page 生成规则：
  - `operationKind=report` 生成筛选表单 + ChartPanel/DataTable 候选。
- `mail.send`、`cache.refresh` 这类没有对象生命周期的函数生成 Operation Page，不强行进入 Entity Page。
- `reward.batchGrant` 这类批量异步操作优先 Task Page。

验收标准：

- 示例函数能生成：
  - `player.manage` Entity Page
  - `mail.send` Operation Page
  - `reward.batchGrant` Task Page
  - `analytics.retention` Report Page
- 缺 `operationKind` 或 `placement` 的函数只生成 blocked diagnostics。
- Generator 单测覆盖分类默认规则、labels 缺失、分页字段映射、独立操作页。

验证命令：

```bash
go test ./internal/dashboard/... ./internal/api/workspace/... ./internal/api/entity/...
rg -n "strings\\.HasSuffix|strings\\.Contains|split.*function|function.*split|player_id|reason" "internal/dashboard" "web/src/pages/Console"
```

禁止事项：

- 禁止根据函数名后缀推断正式 Page。
- 禁止把所有函数都塞到同一个对象管理页。
- 禁止没有 output schema 时硬生成表格列。

## 4. P1 前端工作台重构

### P1-1. WorkspaceEditor 改为 Page 工作台

目标：

- 用户在工作台中管理 PageSpec 草稿，而不是编辑旧 layout。

受影响路径：

- `web/src/pages/WorkspaceEditor/index.tsx`
- `web/src/pages/WorkspaceEditor/components/*`
- `web/src/pages/WorkspaceEditor/hooks/*`
- `web/src/pages/WorkspaceEditor/utils/*`
- `web/src/services/workspaceConfig.ts`
- 新增建议：`web/src/services/pageWorkspace.ts`

实施要点：

- 路由改为 `/system/functions/pages`、`/system/functions/pages/:pageKey` 或符合现有信息架构的 Page 工作台路径。
- 列表页展示 Page 草稿、发布状态、分类、类型、诊断、更新时间。
- 支持从 Resource 生成默认 PageSpec 建议并复制为草稿。
- 编辑器只编辑 PageSpec Formily schema、bindings、metadata。
- 预览调用 `/preview`，不影响运行控制台。
- 发布调用 `/publish`，失败展示 diagnostics。
- 历史版本、diff、rollback 使用 PageSpec 结构，不比较旧 layout。
- 删除 `TabEditor/LayoutDesigner/CanvasEditor/schemaToLayout` 等旧 layout 专用概念，或重写为 Formily PageSpec 编辑器。

验收标准：

- 用户可以从 `player` Resource 生成 `player.manage` 草稿、预览、发布。
- 发布后运行控制台菜单出现，取消发布后消失。
- 编辑器页面没有 objectKey、tabs layout、sections layout 的概念。

验证命令：

```bash
pnpm --dir "web" exec eslint "src/pages/WorkspaceEditor" "src/services"
rg -n "objectKey|WorkspaceConfig|TabEditor|LayoutDesigner|schemaToLayout|tabs|sections|wizard|dashboard" "web/src/pages/WorkspaceEditor" "web/src/services"
```

禁止事项：

- 禁止把旧 layout editor 包一层标题后继续使用。
- 禁止工作台二次选择 `game_id/env`；当前 scope 只来自全局上下文。

### P1-2. 系统菜单和信息架构收敛

目标：

- 让后台菜单表达清晰边界，用户能理解每个入口做什么。

受影响路径：

- `web/config/routes.ts`
- `web/src/locales/*/menu.ts`
- `web/src/pages/Console/index.tsx`
- `web/src/pages/Functions/*`
- `web/src/pages/Workspaces/*`
- `web/src/pages/ComponentManagement/*`

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
  Page 工作台

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
  - Page 工作台：草稿、生成、编辑、发布。
  - 运行控制台：只执行已发布页面。
- 前端不在 Page 内再出现一套 `game_id/env` 选择；所有 API 请求使用全局选择的 scope。

验收标准：

- 用户从左侧菜单能明确区分“编辑页面”和“执行页面”。
- 运行控制台动态分类显示用户配置的多语言标题。
- 任意 Page 内不会出现和全局冲突的 game/env 选择器。

验证命令：

```bash
pnpm --dir "web" exec eslint "src"
rg -n "实体管理|对象工作台|workspace|Workspace|game_id|env" "web/config/routes.ts" "web/src/pages" "web/src/locales"
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
- 新增或迁移：
  - `resources:read`
  - `pages:read`
  - `pages:edit`
  - `pages:publish`
  - `pages:rollback`
  - `pages:delete`
  - `console:read`
- 删除或替换 `workspace:*`、`entities:*` 中误导的权限描述。
- 发布、回滚、取消发布必须写审计日志，记录 pageKey、版本、操作者、诊断结果。

验收标准：

- 前端按钮权限和后端 API 权限一致。
- 权限描述不再出现“对象工作台配置”。
- 审计能追踪 Page 发布和回滚。

验证命令：

```bash
rg -n "workspace:|entities:|entity:write|对象工作台|实体定义" "configs" "internal" "web/src"
go test ./internal/api/policy/... ./internal/logic/...
```

禁止事项：

- 禁止权限名和页面模型继续混用。

## 5. P1 数据迁移

### P1-4. 数据表和迁移脚本

目标：

- 从旧 `workspace_configs` 迁移到 PageSpec 存储，失败不发布。

受影响路径：

- `internal/model/workspace_config.go`
- `internal/model/config_version.go`
- `internal/model/*migration*`
- `internal/router/router.go`
- 数据库 migration/seed 目录，按项目现有结构定位

目标表建议：

```text
page_specs
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
  draft_version
  published_version
  created_at
  updated_at
  updated_by

published_page_specs
  page_key
  version
  spec_json
  published_at
  published_by

page_versions
  page_key
  version
  spec_json
  status
  message
  created_at
  created_by
```

实施要点：

- 写一次性迁移：旧 `workspace_configs` 可转换则生成 PageSpec draft。
- 旧 layout 无法转换时生成 blocked diagnostics，不发布。
- 旧 published=true 不等于新 published；只有通过新校验才能写入 `published_page_specs`。
- 迁移日志必须列出成功、失败、跳过的 pageKey/objectKey。
- 迁移完成后运行态不再读取 `workspace_configs`。

验收标准：

- 旧数据不会静默变成错误的新页面。
- 无法迁移的数据不会出现在运行控制台。
- 可以安全重复执行迁移，不重复插入版本。

验证命令：

```bash
go test ./internal/model/... ./internal/api/workspace/...
rg -n "workspace_configs|WorkspaceConfigModel|FindByObjectKey|SetPublished" "internal" "web/src"
```

禁止事项：

- 禁止把旧 `published=true` 直接映射为新发布态。
- 禁止迁移时补猜缺失 labels 和 bindings 后直接发布。

## 6. P2 清理旧代码和防回归

### P2-1. 删除旧 Workspace Renderer 和 Mock

目标：

- 清理旧 layout 渲染体系，避免后续 AI 继续误用。

受影响路径：

- `web/src/components/WorkspaceRenderer/*`
- `web/src/services/mock/workspaceMock.ts`
- `web/src/services/workspaceConfig.ts`
- `web/src/services/workspace/*`
- `web/src/pages/Workspaces/*`
- `web/src/pages/ComponentManagement/components/FunctionWorkspace.tsx`

实施要点：

- 删除旧 renderer，或迁移为 Formily renderer 后改名。
- 删除旧 workspace mock。
- 删除 `workspaceConfig.ts`，用 `pageWorkspace.ts`、`console.ts`、`resources.ts` 替代。
- 删除质量报告里基于旧 layout 的校验。

验收标准：

- 代码中没有运行态引用 `WorkspaceRenderer`。
- 没有 mock workspace 配置参与真实菜单和页面。

验证命令：

```bash
rg -n "WorkspaceRenderer|workspaceMock|workspaceConfig|WorkspaceConfig|WorkspaceLayout|TabLayout" "web/src"
pnpm --dir "web" exec eslint "src"
```

禁止事项：

- 禁止保留“deprecated but still works”的旧 renderer。

### P2-2. 删除旧后端 Workspace/Entity 模型残留

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

实施要点：

- 删除旧 WorkspaceConfig model 或改名迁移为 PageSpec model。
- 删除旧 Entity CRUD model，若业务还有实体定义需求，另开独立 bounded context，不能混入 Dashboard Page 模型。
- 更新路由发现接口，删除旧 `Entities/Workspaces` 误导项。

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

目标：

- 防止旧模式被重新引入。

受影响路径：

- `.github/workflows/ci.yml`
- `scripts/*`
- 新增建议：`scripts/check-dashboard-model.sh`

Guard 必须检查：

```bash
rg -n "WorkspaceConfig\\.layout|WorkspaceLayout|TabLayout" "web/src" "internal" && exit 1
rg -n "objectKey" "web/src/pages/Console" "web/src/services" "internal/api/workspace" && exit 1
rg -n "menu\\.ControlConsole\\.category\\." "web/src" && exit 1
rg -n "x-operation.*custom|CRUD operation type" "proto" "sdks" "web/src" && exit 1
rg -n "\\bany\\b" "web/src/types" "web/src/services/console.ts" "web/src/services/pageWorkspace.ts" && exit 1
rg -n "interface\\{\\}|map\\[string\\]interface\\{" "internal/api/workspace" "internal/api/entity" "internal/api/function" && exit 1
```

验收标准：

- CI 在旧概念重新出现时失败。
- guard 脚本有单独 README 或注释说明每条规则对应的设计约束。

验证命令：

```bash
bash scripts/check-dashboard-model.sh
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
- Dashboard 文档只保留 FunctionSpec -> ResourceSpec -> OperationSpec -> PageSpec -> PublishedPageSpec -> ConsoleMenuSpec。
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

执行 AI 完成重构后，必须至少交付以下场景。

### 场景 A：玩家管理 Entity Page

输入函数：

- `player.list`
  - `entity=player`
  - `operation=list`
  - `operationKind=list`
  - `placement=tableData`
  - output 有 `items/total`
- `player.get`
  - `operation=get`
  - `operationKind=get`
  - `placement=detailData`
- `player.ban`
  - `operation=ban`
  - `operationKind=action`
  - `placement=rowAction`

期望：

- Resource API 返回 `player` Resource 和 3 个 Operation。
- Generated Page 返回 `player.manage`，质量至少 `needs_review`，字段完整时为 `ready`。
- Page 工作台可预览、保存、发布。
- 运行控制台菜单显示“玩家管理”。
- 页面支持查询、分页、详情、封禁行操作。

### 场景 B：邮件发送 Operation Page

输入函数：

- `mail.send`
  - `entity=mail` 或无 entity 但 `pageHint=mail.send`
  - `operation=send`
  - `operationKind=action`
  - `placement=standalone`

期望：

- 不强行进入玩家/对象管理页。
- 生成独立 Operation Page。
- 菜单分类按显式 category 或 `mail` 默认规则。
- 页面是表单 + 结果，不出现表格分页。

### 场景 C：奖励批量发放 Task Page

输入函数：

- `reward.batchGrant`
  - `operation=batchGrant`
  - `operationKind=task`
  - `placement=standalone` 或 `batchAction`

期望：

- 生成 Task Page。
- 页面可启动任务、查看任务事件、查看结果。
- 长耗时状态不伪装成同步 action。

### 场景 D：留存分析 Report Page

输入函数：

- `analytics.retention`
  - `operation=retention`
  - `operationKind=report`
  - `placement=standalone`

期望：

- 生成 Report Page。
- PageSpec 明确筛选字段、series/items 映射。
- 图表组件不猜 response shape。

### 场景 E：缺字段函数待编排

输入函数：

- `cache.refresh`
  - 只有 `id/summary/input_schema`
  - 缺 `operationKind/placement/categoryDisplay`

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

- Dashboard、Page 工作台、运行控制台所有 API 默认使用全局 scope。
- PageSpec 中不内置 scope selector，除非 Page 明确是跨环境运维报表，并在 metadata 中声明。

实施要点：

- 请求层统一注入当前 `gameId/env`。
- PageSpec binding 调用函数时从全局 scope 取 game/env。
- Page 工作台保存和发布也带全局 scope。
- 后端按 scope 隔离 PageSpec、PublishedPageSpec、ResourceSpec 视图。

验收标准：

- 任意运行控制台 Page 内没有第二套 game/env 选择器。
- 切换全局 game/env 后，菜单和页面按新 scope 重新加载。
- API 层测试覆盖 scope 隔离。

验证命令：

```bash
rg -n "game_id|gameId|env" "web/src/pages/Console" "web/src/components/FormilyPageRenderer" "web/src/pages/WorkspaceEditor"
go test ./internal/api/console/... ./internal/api/workspace/... ./internal/api/entity/...
```

禁止事项：

- 禁止 Page 内部默认渲染 game/env 字段。
- 禁止前端把 URL 中的 category/pageKey 当作 scope。

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

- Page renderer 调用函数时传递 `pageKey/componentKey/functionId/actionRole`。
- 后端函数调用 span 记录：
  - `game_id`
  - `env`
  - `page_key`
  - `function_id`
  - `operation_kind`
  - `placement`
  - `task_id`
  - `risk`
- 前端错误上报包含 PageSpec version 和 component path。
- 发布 Page 时记录校验 diagnostics 和版本。

验收标准：

- 能从一次运行控制台按钮点击追踪到后端函数调用或任务。
- Task Page 能关联 task events 和 pageKey。
- 日志不打印敏感 payload。

验证命令：

```bash
rg -n "page_key|pageKey|function_id|functionId|operation_kind|operationKind|placement|trace|otel" "internal" "web/src"
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
- 仍然把 Function UI 和 Page UI 做成两套协议。
- 仍然使用 `any/interface{}` 承载核心 DTO。
- 仍然根据函数名猜字段、猜分页、猜页面类型。
- 仍然把所有函数塞进对象管理页。
- 仍然让缺 `operationKind/placement/labels` 的页面发布成功。
- 仍然在 Page 内重复选择 `game_id/env`。
- 仍然把字典作为动态菜单分类事实源。
- 仍然只在本地写死 mock 数据验证动态菜单。

必须确认：

- `FunctionSpec -> ResourceSpec + OperationSpec -> PageSpec -> PublishedPageSpec -> ConsoleMenuSpec` 链路可运行。
- Page 工作台和运行控制台边界清楚。
- 函数注册后能生成默认 PageSpec 建议，但不会未经确认自动发布。
- 多语言动态标题来自 PageSpec metadata。
- 全局 scope 是唯一 game/env 来源。
- Entity/Operation/Task/Report 四类页面都有最小端到端用例。
- CI guard 会阻断旧模型回流。

## 11. 建议执行顺序

建议按以下顺序提交，避免大爆炸后无法审核：

1. 提交强类型模型和 descriptor v2，不改页面行为。
2. 提交 normalizer 和 Resource/Operation API，补诊断测试。
3. 提交 PageSpec draft/publish API 和数据模型。
4. 提交 Console API，先让菜单从 Server 返回静态构造的 PublishedPageSpec 测试数据。
5. 提交前端 ConsoleMenuSpec 菜单接入，删除 workspaceConfigs 菜单注入。
6. 提交 FormilyPageRenderer 最小组件集。
7. 提交 PageSpec Generator 和四类默认页面。
8. 提交 Page 工作台重构。
9. 提交旧 WorkspaceRenderer/Entity/WorkspaceConfig 清理。
10. 提交 CI guard、文档和 SDK demo 收尾。

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
- Page 工作台保存、预览、发布的是 PageSpec。
- Function UI 和 Page UI 都是 Formily JSON Schema。
- SDK/OpenAPI descriptor v2 能表达分类、多语言、资源、业务动作、页面语义和放置位置。
- 缺语义的函数不会被自动发布。
- 文档、示例、权限、CI guard 都与新模型一致。
- `go test ./...`、`pnpm --dir "web" exec eslint "src"`、`pnpm --dir "docs" build` 通过。
