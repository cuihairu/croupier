# 函数 API

## 通用类型

```go
type JSONValue = json.RawMessage
type JSONSchema = json.RawMessage
type FormPresentationSpec = json.RawMessage
type OpenAPIOperation = json.RawMessage
```

说明：

- `JSONValue` 仅表示业务 payload 或函数返回值，结构必须由函数 `input_schema` / `output_schema` 约束。
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
	FunctionId string `json:"function_id"`
	NewId string `json:"new_id"`
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
	GameID string `json:"gameId,optional"`
	Env string `json:"env,optional"`
	Mode string `json:"mode,optional"`
	Route string `json:"route,optional"`
	TargetServiceID string `json:"target_service_id,optional"`
	HashKey string `json:"hash_key,optional"`
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

### 12. "获取函数表单配置"

1. route definition

- Url: /api/v1/functions/:id/form
- Method: GET
- Request: `FunctionFormRequest`
- Response: `FunctionFormResponse`

2. request definition



```go
type FunctionFormRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```go
type FunctionFormResponse struct {
	Schema JSONSchema `json:"schema,omitempty"`
	Presentation FormPresentationSpec `json:"presentation,omitempty"`
	Custom bool `json:"custom"`
	HasDefault bool `json:"hasDefault"`
	FormSource string `json:"formSource"` // custom_metadata / config_file_override / generated_default / none
	FormSourceDetail string `json:"formSourceDetail"`
}
```

### 13. "更新函数表单配置"

1. route definition

- Url: /api/v1/functions/:id/form
- Method: PUT
- Request: `FunctionFormUpdateRequest`
- Response: `FunctionFormResponse`

2. request definition



```go
type FunctionFormUpdateRequest struct {
	ID string `path:"id"`
	Presentation FormPresentationSpec `json:"presentation"`
}
```


3. response definition



```go
type FunctionFormResponse struct {
	Schema JSONSchema `json:"schema,omitempty"`
	Presentation FormPresentationSpec `json:"presentation,omitempty"`
	Custom bool `json:"custom"`
	HasDefault bool `json:"hasDefault"`
	FormSource string `json:"formSource"`
	FormSourceDetail string `json:"formSourceDetail"`
}
```

**说明：**

- 函数表单读取 FunctionContract JSON Schema，并只读写单函数 FormPresentationSpec。
- FormPresentationSpec 不承载独立页面 layout、组件树或任意 React props。
- 非法 schema/presentation 必须返回校验错误，不能转换、猜测或静默降级。
- Page 级布局、分页、表格、详情和动作编排属于 Page Studio API，不属于函数表单 API。

### 14. "获取函数表单配置历史"

1. route definition

- Url: /api/v1/functions/:id/form/history
- Method: GET
- Request: `FunctionFormHistoryRequest`
- Response: `FunctionFormHistoryResponse`

2. request definition

```go
type FunctionFormHistoryRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type FunctionFormHistoryResponse struct {
	Items []FunctionFormHistoryItem `json:"items"`
}
```

### 15. "回滚函数表单配置"

1. route definition

- Url: /api/v1/functions/:id/form/rollback
- Method: POST
- Request: `FunctionFormRollbackRequest`
- Response: `FunctionFormRollbackResponse`

2. request definition

```go
type FunctionFormRollbackRequest struct {
	ID      string `path:"id"`
	Version int    `json:"version"`
}
```

3. response definition

```go
type FunctionFormRollbackResponse struct {
	AppliedVersion int                 `json:"appliedVersion"`
	Current        *FunctionFormResponse `json:"current"`
}
```

### 16. "批量复制函数"

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

- Url: /api/v1/functions/_openapi-batch
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

## 函数政策 API

### 22. "获取函数政策"

1. route definition

- Url: /api/v1/functions/:function_id/policy
- Method: GET
- Request: -
- Response: `Policy`

2. request definition

```
function_id: path parameter
risk_level: query parameter (optional, default: medium)
```

3. response definition

```go
type Policy struct {
	FunctionID       string   `json:"function_id"`
	RequireApproval  bool     `json:"require_approval"`
	ApprovalWorkflow string   `json:"approval_workflow"`
	RequireAudit     bool     `json:"require_audit"`
	AllowedRoles     []string `json:"allowed_roles"`
	Source           string   `json:"source"`       // "default" 或 "manual"
	IsOverride       bool     `json:"is_override"`
	DefaultRiskLevel string   `json:"default_risk_level"`
}
```

**说明：**
- 返回函数的有效政策（优先使用数据库覆盖，否则使用默认风险等级政策）
- 风险等级可选值：`low`、`medium`、`high`、`danger`

### 23. "设置函数政策覆盖"

1. route definition

- Url: /api/v1/functions/:function_id/policy
- Method: PUT
- Request: `SetPolicyRequest`
- Response: `Policy`

2. request definition

```go
type SetPolicyRequest struct {
	RequireApproval  bool     `json:"require_approval"`
	ApprovalWorkflow string   `json:"approval_workflow"`
	RequireAudit     bool     `json:"require_audit"`
	AllowedRoles     []string `json:"allowed_roles"`
}
```

3. response definition

```go
type Policy struct {
	FunctionID       string   `json:"function_id"`
	RequireApproval  bool     `json:"require_approval"`
	ApprovalWorkflow string   `json:"approval_workflow"`
	RequireAudit     bool     `json:"require_audit"`
	AllowedRoles     []string `json:"allowed_roles"`
	Source           string   `json:"source"`
	IsOverride       bool     `json:"is_override"`
}
```

**说明：**
- 为函数设置数据库覆盖政策，覆盖默认风险等级政策
- `AllowedRoles` 为空时表示无角色限制
- 设置后，`Source` 为 `"manual"`，`IsOverride` 为 `true`

### 24. "删除函数政策覆盖"

1. route definition

- Url: /api/v1/functions/:function_id/policy
- Method: DELETE
- Request: -
- Response: `{"message": "..."}`

2. request definition

```
function_id: path parameter
```

3. response definition

```go
{
  "message": "Policy deleted, using default risk-based policy"
}
```

**说明：**
- 删除函数的数据库覆盖政策
- 删除后，函数将恢复使用默认风险等级政策

## 系统政策 API

### 25. "获取所有政策覆盖"

1. route definition

- Url: /api/v1/policies/overrides
- Method: GET
- Request: -
- Response: `{"policies": [...]}`

2. response definition

```go
{
  "policies": []Policy  // 所有手动设置的覆盖政策
}
```

**说明：**
- 返回所有手动设置的函数政策覆盖列表
- 不包括默认风险等级政策

### 26. "获取默认政策配置"

1. route definition

- Url: /api/v1/policies/defaults
- Method: GET
- Request: -
- Response: `{"low": ..., "medium": ..., "high": ..., "danger": ...}`

2. response definition

```go
{
  "low": Policy,    // 低风险默认政策
  "medium": Policy, // 中风险默认政策
  "high": Policy,   // 高风险默认政策
  "danger": Policy  // 危险风险默认政策
}
```

**说明：**
- 返回所有风险等级的默认政策配置
- 这些配置来自 `configs/default-policies.yaml` 文件

### 27. "重新加载政策配置"

1. route definition

- Url: /api/v1/policies/reload
- Method: POST
- Request: -
- Response: `{"message": "..."}`

2. response definition

```go
{
  "message": "Configuration reloaded"
}
```

**说明：**
- 重新从 `configs/default-policies.yaml` 加载默认政策配置
- 不影响已设置的数据库覆盖政策
