# 函数 API

## 通用类型

```go
type JSONValue = json.RawMessage
type JSONSchema = json.RawMessage
type FormPresentationSpec = json.RawMessage
type OpenAPIOperation = json.RawMessage
```

说明：

- `JSONValue` 仅表示业务 payload 或函数返回值，结构必须由函数 `inputSchema` / `outputSchema` 约束。
- `JSONSchema` 仅表示 JSON Schema / OpenAPI Schema。
- `FormPresentationSpec` 表示 JSON Schema 表单的受控展示配置，不能承载页面布局、菜单或任意组件 props。
- `OpenAPIOperation` 只用于契约查看，不用于运行控制台直接生成页面。
- `json.RawMessage` 只是 HTTP 边界上的 JSON 承载类型，服务端必须在保存或执行前完成结构校验。

### 1. "获取函数列表"

1. route definition

- Url: /api/v1/functions
- Method: GET
- Request: `FunctionsListRequest`
- Response: `FunctionsListResponse`

2. request definition

```go
type FunctionsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	GameId string `form:"gameId,optional"`
	Category string `form:"category,optional"`
	Status int `form:"status,optional"`
}
```

3. response definition

```go
type FunctionsListResponse struct {
	Items []Function `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "获取函数详情"

1. route definition

- Url: /api/v1/functions/:id
- Method: GET
- Request: `FunctionDetailRequest`
- Response: `FunctionDetailResponse`

2. request definition

```go
type FunctionDetailRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionDetailResponse struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	GameId string `json:"gameId"`
	Status int `json:"status"`
	Version string `json:"version"`
	Instances int `json:"instances"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Descriptor FunctionDescriptor `json:"descriptor"`
}

type Function struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Category string `json:"category"`
	GameId string `json:"gameId"`
	Status int `json:"status"`
	Version string `json:"version"`
	Instances int `json:"instances"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type FunctionDescriptor struct {
	Input JSONSchema `json:"input"`
	Output JSONSchema `json:"output"`
	Schema JSONSchema `json:"schema"`
}
```

### 3. "删除函数"

1. route definition

- Url: /api/v1/functions/:id
- Method: DELETE
- Request: `FunctionActionRequest`
- Response: `-`

2. request definition

```go
type FunctionActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

### 4. "复制函数"

1. route definition

- Url: /api/v1/functions/:id/copy
- Method: POST
- Request: `FunctionCopyRequest`
- Response: `FunctionCopyResponse`

2. request definition

```go
type FunctionCopyRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionCopyResponse struct {
	FunctionId string `json:"functionId"`
	NewId string `json:"newId"`
}
```

### 5. "禁用函数"

1. route definition

- Url: /api/v1/functions/:id/disable
- Method: POST
- Request: `FunctionActionRequest`
- Response: `-`

2. request definition

```go
type FunctionActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

### 6. "启用函数"

1. route definition

- Url: /api/v1/functions/:id/enable
- Method: POST
- Request: `FunctionActionRequest`
- Response: `-`

2. request definition

```go
type FunctionActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

### 7. "获取函数实例"

1. route definition

- Url: /api/v1/functions/:id/instances
- Method: GET
- Request: `FunctionInstancesRequest`
- Response: `FunctionInstancesResponse`

2. request definition

```go
type FunctionInstancesRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionInstancesResponse struct {
	Items []FunctionInstance `json:"items"`
}
```

### 8. "调用函数"

1. route definition

- Url: /api/v1/functions/:id/invoke
- Method: POST
- Request: `FunctionInvokeRequest`
- Response: `FunctionInvokeResponse`

2. request definition

```go
type FunctionInvokeRequest struct {
	ID string `path:"id"`
	Params JSONValue `json:"params,optional"`
	Payload JSONValue `json:"payload,optional"`
	GameID string `json:"gameId,optional"` // 兼容字段；生效 scope 以 X-Game-ID/X-Env 请求头为准
	Env string `json:"env,optional"`     // 兼容字段；生效 scope 以 X-Game-ID/X-Env 请求头为准
	Mode string `json:"mode,optional"`
	Route string `json:"route,optional"`
	TargetServiceID string `json:"targetServiceId,optional"`
	HashKey string `json:"hashKey,optional"`
}
```

3. response definition

```go
type FunctionInvokeResponse struct {
	TaskId           string      `json:"taskId"`
	TaskID           string      `json:"taskID,omitempty"`
	Result           JSONValue   `json:"result,omitempty"`
	ApprovalID       string      `json:"approval_id,omitempty"`       // 审批请求 ID（当需要审批时返回）
	ApprovalRequired bool        `json:"approval_required,omitempty"` // 是否需要审批
	ApprovalWorkflow string      `json:"approval_workflow,omitempty"` // 审批流程类型（single_admin/two_person）
}
```

**说明：**

- 当函数政策需要审批时（`RequireApproval=true`），调用会创建审批请求并返回 `ApprovalID`
- 需要审批的调用不会立即执行，需等待审批通过后执行
- `ApprovalWorkflow` 表示审批流程类型：
  - `single_admin`: 单个管理员审批即可
  - `two_person`: 需要双人审批

### 9. "获取函数权限"

1. route definition

- Url: /api/v1/functions/:id/permissions
- Method: GET
- Request: `FunctionPermissionsRequest`
- Response: `FunctionPermissionsResponse`

2. request definition

```go
type FunctionPermissionsRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionPermissionsResponse struct {
	Items []FunctionPermission `json:"items"`
}
```

### 10. "更新函数权限"

1. route definition

- Url: /api/v1/functions/:id/permissions
- Method: PUT
- Request: `FunctionPermissionsUpdateRequest`
- Response: `FunctionPermissionsResponse`

2. request definition

```go
type FunctionPermissionsUpdateRequest struct {
	ID string `path:"id"`
	Permissions []FunctionPermission `json:"permissions"`
}
```

3. response definition

```go
type FunctionPermissionsResponse struct {
	Items []FunctionPermission `json:"items"`
}
```

### 11. "发布函数"

1. route definition

- Url: /api/v1/functions/:id/publish
- Method: POST
- Request: `FunctionPublishRequest`
- Response: `FunctionPublishResponse`

2. request definition

```go
type FunctionPublishRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionPublishResponse struct {
	ApprovalId string `json:"approvalId,omitempty"` // 如果需要审批
	Published bool `json:"published"`
}
```

### 12. "批量复制函数"

1. route definition

- Url: /api/v1/functions/batch-copy
- Method: POST
- Request: `BatchCopyFunctionsRequest`
- Response: `BatchCopyFunctionsResponse`

2. request definition

```go
type BatchCopyFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
}
```

3. response definition

```go
type BatchCopyFunctionsResponse struct {
	Updated int `json:"updated"`
	Failed []string `json:"failed"`
	Copied []string `json:"copied"` // 新复制的函数ID列表
}
```

### 17. "批量删除函数"

1. route definition

- Url: /api/v1/functions/batch-delete
- Method: POST
- Request: `BatchDeleteFunctionsRequest`
- Response: `BatchDeleteFunctionsResponse`

2. request definition

```go
type BatchDeleteFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
}
```

3. response definition

```go
type BatchDeleteFunctionsResponse struct {
	Updated int `json:"updated"`
	Failed []string `json:"failed"`
}
```

### 18. "批量更新函数状态"

1. route definition

- Url: /api/v1/functions/batch-update
- Method: POST
- Request: `BatchUpdateFunctionsRequest`
- Response: `BatchUpdateFunctionsResponse`

2. request definition

```go
type BatchUpdateFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
	Enabled bool `json:"enabled"`
}
```

3. response definition

```go
type BatchUpdateFunctionsResponse struct {
	Updated int `json:"updated"`
	Failed []string `json:"failed"`
}
```

### 19. "获取函数描述符列表"

1. route definition

- Url: /api/v1/functions/descriptors
- Method: GET
- Request: `DescriptorsRequest`
- Response: `DescriptorsResponse`

2. request definition

```go
type DescriptorsRequest struct {
	Type string `form:"type,optional"`
	GameId string `form:"gameId,optional"`
}
```

3. response definition

```go
type DescriptorsResponse struct {
	Items []Descriptor `json:"items"`
}
```

### 20. "获取待处理函数"

1. route definition

- Url: /api/v1/functions/pending
- Method: GET
- Request: `FunctionsPendingRequest`
- Response: `FunctionsPendingResponse`

2. request definition

```go
type FunctionsPendingRequest struct {
}
```

3. response definition

```go
type FunctionsPendingResponse struct {
	Items []PendingFunction `json:"items"`
}
```

### 21. "批量获取函数 OpenAPI"

1. route definition

- Url: /api/v1/functions/\_openapi-batch
- Method: POST
- Request: `BatchGetSpecRequest`
- Response: `map[string]OpenAPIOperation`

2. request definition

```go
type BatchGetSpecRequest struct {
	FunctionIDs []string `json:"function_ids"`
}
```

3. response definition

```go
// key 为 function id，value 为对应的 OpenAPI Operation；未找到时返回 null
map[string]OpenAPIOperation
```

### 说明

- 该接口用于 Dashboard 批量读取函数 OpenAPI，避免逐个请求。
- 当前返回值直接透传注册表中的 OpenAPI operation 对象。

## 函数政策 API（未接线）

函数政策（`GET/PUT/DELETE /api/v1/functions/:function_id/policy`）与系统政策（`/api/v1/policies/*`）端点当前**未在生效路由**（`internal/handler/routes.go`）注册，仅存在于并行注册文件 `internal/router/router.go` 中，不对外提供。政策行为由函数合同的 risk/approval 字段与执行链路治理承载；本节历史文档已删除，待端点接线后再恢复。

## 补充端点

以下已注册端点是函数域 canonical 集合的一部分：

```http
GET /api/v1/functions/{id}/analytics   # 函数调用分析
GET /api/v1/functions/instances        # 全量函数实例
GET /api/v1/functions/warnings         # 注册警告列表
GET /api/v1/functions/:id/openapi      # 函数 OpenAPI spec（公开）
POST /api/v1/functions/_openapi-batch  # 批量获取 OpenAPI spec（公开）
GET /api/v1/openapi/spec               # 全局 OpenAPI 文档（公开）
```
