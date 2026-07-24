---
title: Page Studio API
icon: appstore
order: 99
category:
  - API 参考
tag:
  - page-studio
  - PageSpec
  - Formily
---

# Page Studio API

Page Studio 负责管理 Dashboard 业务页面的草稿、预览、校验、发布、版本和回滚。历史 URL/包名中的 `workspace` 仅是迁移期实现残留，不是目标 API 或领域术语。

本文只记录目标模型 API。页面配置统一使用 `PageSpec`，页面运行时 UI 统一使用 Formily JSON Schema。

## 模型边界

| 模型 | 职责 |
| --- | --- |
| `GeneratedPageSpec` | Server 根据 FunctionSpec / ResourceSpec / OperationSpec 生成的默认页面建议 |
| `PageSpecDraft` | 用户在 Page Studio 编辑中的页面草稿及其修订 |
| `PublishedPageSpec` | 已校验、已发布、冻结 binding 契约、运行控制台可消费的页面快照 |
| `ConsoleMenuSpec` | 由 PublishedPageSpec 生成的运行控制台左侧菜单 |

Page Studio 不负责函数语义归一化，也不负责运行控制台菜单推断。

所有 Page API 都从全局选择器经统一请求头传递、再由服务端按认证用户权限解析的 `game_id + env` scope 获取页面身份。客户端不得在 body/query、页面 payload 或 URL 中覆盖 scope；同一个 `pageKey` 可以存在于不同 scope，数据库和服务端查询必须以 `(game_id, env, page_key)` 隔离。

## PageSpec

```go
type FormilySchema = json.RawMessage
type PageMetadata = json.RawMessage

type PageSpec struct {
	PageKey     string                 `json:"pageKey"`
	Type        string                 `json:"type"` // entity / operation / task / report
	ResourceKey string                 `json:"resourceKey,omitempty"`
	Title       map[string]string      `json:"title"`
	Description map[string]string      `json:"description,omitempty"`
	Category    PageCategorySpec       `json:"category"`
	Order       int                    `json:"order,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Schema      FormilySchema          `json:"schema"`
	Bindings    []PageFunctionBinding  `json:"bindings"`
    Metadata    PageMetadata           `json:"metadata,omitempty"` // 仅非执行展示扩展
}

type PageCategorySpec struct {
	Key    string            `json:"key"`
	Labels map[string]string `json:"labels"`
	Order  int               `json:"order,omitempty"`
}

type PageFunctionBinding struct {
    ID            string          `json:"id"`
    FunctionID    string          `json:"functionId"`
    Usage         string          `json:"usage"` // query / detail / action / task / report
    InputMapping  json.RawMessage `json:"inputMapping,omitempty"`
    OutputMapping json.RawMessage `json:"outputMapping,omitempty"`
    Execution     BindingExecution `json:"execution"`
}

type BindingExecution struct {
    Mode          string `json:"mode"` // sync / task
    RequireConfirm bool   `json:"requireConfirm,omitempty"` // only strengthens function policy
}
```

约束：

- `schema` 必须是 Formily JSON Schema。
- Page UI Schema 根节点必须是 `schemaVersion + ConsolePage`，每个页面组件的 props 必须通过版本化 ABI 校验。
- `title`、`category.labels` 必须包含系统默认语言。
- `bindings` 的 `id` 必须在 Page 内唯一；Schema 只能引用 `bindingId`，不能写裸 `functionId`。
- `usage` 表示页面实际消费函数的方式；不能复用注册期 `placement`，后者只是生成候选的推荐位置。
- `bindings` 必须引用当前 scope 内已注册且可调用的函数；输入/输出映射必须可由函数契约验证。
- `type`、`category.key`、`pageKey` 发布后必须保持稳定。
- 分页、表格数据、详情数据和操作位置必须显式写在 `schema` 或 `bindings` 中，不由前端运行时推断。

## 获取默认页面建议

```http
GET /api/v1/pages/generated?resourceKey={resourceKey}
```

返回 Server 基于当前函数注册生成的 PageSpec 建议。建议可预览和复制到草稿，但不是发布产物。

```go
type GeneratedPagesResponse struct {
	Items       []GeneratedPageSpec `json:"items"`
	Diagnostics []PageDiagnostic   `json:"diagnostics,omitempty"`
}

type GeneratedPageSpec struct {
	PageSpec
	Quality     string           `json:"quality"` // ready / needs_review / blocked
	Diagnostics []PageDiagnostic `json:"diagnostics,omitempty"`
}

type PageDiagnostic struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"` // error / warning / info
	Message    string `json:"message"`
	FunctionID string `json:"functionId,omitempty"`
	Field      string `json:"field,omitempty"`
}
```

## 获取页面草稿列表

```http
GET /api/v1/pages
```

```go
type PageDraftListResponse struct {
	Items []PageSpecDraftSummary `json:"items"`
}

type PageSpecDraftSummary struct {
	PageKey     string            `json:"pageKey"`
	Type        string            `json:"type"`
	ResourceKey string            `json:"resourceKey,omitempty"`
	Title       map[string]string `json:"title"`
	Category    PageCategorySpec  `json:"category"`
	Status      string            `json:"status"` // draft / published / archived
	DraftVersion int              `json:"draftVersion"`
	PublishedVersion int          `json:"publishedVersion,omitempty"`
	UpdatedAt   string            `json:"updatedAt"`
	UpdatedBy   string            `json:"updatedBy,omitempty"`
}
```

## 获取页面草稿

```http
GET /api/v1/pages/{pageKey}
```

```go
type PageDraftResponse struct {
	PageSpec
	Status          string           `json:"status"`
	DraftVersion    int              `json:"draftVersion"`
	PublishedVersion int             `json:"publishedVersion,omitempty"`
	Diagnostics     []PageDiagnostic `json:"diagnostics,omitempty"`
	UpdatedAt       string           `json:"updatedAt"`
	UpdatedBy       string           `json:"updatedBy,omitempty"`
}
```

## 保存页面草稿

```http
PUT /api/v1/pages/{pageKey}
```

请求体为完整 `PageSpec`，并必须通过 `If-Match: <draftRevision>`（或等价显式 revision 字段）提交乐观并发版本。

保存规则：

- 保存时校验 `pageKey` 与路径一致。
- revision 不匹配返回 `409 page_draft_conflict`，并返回当前草稿摘要；禁止静默覆盖。
- 保存时校验 `schema` 是否为 Formily JSON Schema。
- 保存时可以存在 warning 诊断，但 error 诊断必须阻止保存。
- 保存草稿不会影响运行控制台。

## 校验页面草稿

```http
POST /api/v1/pages/{pageKey}/validate
```

```go
type PageValidateResponse struct {
	Valid       bool             `json:"valid"`
	Diagnostics []PageDiagnostic `json:"diagnostics"`
}
```

发布前必须通过校验。

## 预览页面草稿

```http
POST /api/v1/pages/{pageKey}/preview
```

预览只渲染当前草稿，不写入运行控制台菜单。

```go
type PagePreviewResponse struct {
	Page PageSpec `json:"page"`
}
```

## 发布页面

```http
POST /api/v1/pages/{pageKey}/publish
```

发布成功后生成 `PublishedPageSpec` 快照，并使运行控制台菜单在下一次加载时可见。

```go
type PagePublishResponse struct {
	PageKey          string `json:"pageKey"`
	Published       bool   `json:"published"`
	PublishedVersion int   `json:"publishedVersion"`
}
```

发布必须满足：

- `schema` 是 Formily JSON Schema。
- `title` 和 `category.labels` 覆盖系统启用语言。
- `category.key` 已确定。
- `bindings` 引用的函数存在且可调用，并冻结 function version、输入/输出 schema digest、风险、权限和执行模式。
- 需要表格、详情或报表时，响应结构有明确字段映射。

## 取消发布

```http
POST /api/v1/pages/{pageKey}/unpublish
```

取消发布只影响运行控制台可见性，不删除草稿。

## 页面版本

```http
GET /api/v1/pages/{pageKey}/versions
GET /api/v1/pages/{pageKey}/versions/{versionId}
POST /api/v1/pages/{pageKey}/rollback
```

```go
type PageVersionListResponse struct {
	CurrentDraftVersion     int               `json:"currentDraftVersion"`
	CurrentPublishedVersion int               `json:"currentPublishedVersion,omitempty"`
	Items                   []PageVersionItem `json:"items"`
}

type PageVersionItem struct {
	Version            int    `json:"version"`
	Status             string `json:"status"`
	Message            string `json:"message,omitempty"`
	IsCurrentDraft     bool   `json:"isCurrentDraft"`
	IsCurrentPublished bool   `json:"isCurrentPublished"`
	CreatedAt          string `json:"createdAt"`
	CreatedBy          string `json:"createdBy,omitempty"`
}
```

回滚只恢复草稿。需要进入运行控制台时，必须再次发布。

## 运行控制台读取接口

```http
GET /api/v1/console/pages
GET /api/v1/console/pages/{pageKey}
GET /api/v1/console/menu
```

运行控制台只读取已发布页面和菜单，不读取草稿。

`ConsoleMenuSpec` 由 `PublishedPageSpec[]` 生成：

```go
type ConsoleMenuSpec struct {
	Items []ConsoleMenuItem `json:"items"`
}

type ConsoleMenuItem struct {
	Key      string            `json:"key"`
	Path     string            `json:"path"`
	Name     string            `json:"name"`
	Labels   map[string]string `json:"labels"`
	Locale   bool              `json:"locale"` // 动态菜单固定为 false
	Icon     string            `json:"icon,omitempty"`
	Order    int               `json:"order,omitempty"`
	Children []ConsoleMenuItem `json:"children,omitempty"`
}
```

动态分类和页面标题必须来自 `PublishedPageSpec.category.labels` 与 `PublishedPageSpec.title`，不写入前端静态 locale 文件。

## 运行控制台受控执行

```http
POST /api/v1/console/pages/{pageKey}/bindings/{bindingId}/execute
```

该接口加载当前 scope 的 active PublishedPageSpec，按 binding snapshot 校验输入并执行。客户端不能提交任意 `functionId`、`route`、`targetServiceID`、`gameId` 或 `env`。

```go
type PageExecutionResult struct {
    Kind        string            `json:"kind"` // sync / task / approval
    RequestID   string            `json:"requestId"`
    TraceID     string            `json:"traceId,omitempty"`
    Data        json.RawMessage   `json:"data,omitempty"`
    TaskID      string            `json:"taskId,omitempty"`
    ApprovalID  string            `json:"approvalId,omitempty"`
    Diagnostics []PageDiagnostic  `json:"diagnostics,omitempty"`
}
```

执行时必须保留和审计 `game_id`、`env`、`pageKey`、`publishedVersion`、`bindingId`、`functionId`、操作者、目标与 trace ID。函数权限、风险和审批策略是下限，Page 不能放宽。

## 错误结构

```json
{
  "code": "page_spec_invalid",
  "error": "page_spec_invalid",
  "message": "page spec is invalid",
  "request_id": "1741331289799108000",
  "details": {
    "diagnostics": []
  }
}
```

常见错误码：

- `page_spec_not_found`
- `page_spec_invalid`
- `page_spec_publish_failed`
- `page_spec_version_not_found`
- `function_binding_invalid`
- `formily_schema_invalid`
- `localized_text_missing`
- `page_draft_conflict`
- `binding_stale`
- `page_binding_not_found`
- `page_execution_forbidden`

## 审计覆盖

以下动作必须写入审计日志：

- `page.save`
- `page.validate`
- `page.preview`
- `page.publish`
- `page.unpublish`
- `page.rollback`
