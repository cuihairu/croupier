# 函数 API

### 1. "获取函数列表"

1. route definition

- Url: /api/v1/functions
- Method: GET
- Request: `FunctionsListRequest`
- Response: `FunctionsListResponse`

2. request definition



```golang
type FunctionsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	GameId string `form:"gameId,optional"`
	Category string `form:"category,optional"`
	Status int `form:"status,optional"`
}
```


3. response definition



```golang
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



```golang
type FunctionDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
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
	Input interface{} `json:"input"`
	Output interface{} `json:"output"`
	Schema interface{} `json:"schema"`
}
```

### 3. "删除函数"

1. route definition

- Url: /api/v1/functions/:id
- Method: DELETE
- Request: `FunctionActionRequest`
- Response: `-`

2. request definition



```golang
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



```golang
type FunctionCopyRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
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



```golang
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



```golang
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



```golang
type FunctionInstancesRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
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



```golang
type FunctionInvokeRequest struct {
	ID string `path:"id"`
	Params interface{} `json:"params,optional"`
	Payload interface{} `json:"payload,optional"`
	GameID string `json:"gameId,optional"`
	Env string `json:"env,optional"`
	Mode string `json:"mode,optional"`
	Route string `json:"route,optional"`
	TargetServiceID string `json:"target_service_id,optional"`
	HashKey string `json:"hash_key,optional"`
}
```


3. response definition



```golang
type FunctionInvokeResponse struct {
	JobId string `json:"jobId"`
	JobID string `json:"jobID,omitempty"`
	Result interface{} `json:"result,omitempty"`
}
```

### 9. "获取函数权限"

1. route definition

- Url: /api/v1/functions/:id/permissions
- Method: GET
- Request: `FunctionPermissionsRequest`
- Response: `FunctionPermissionsResponse`

2. request definition



```golang
type FunctionPermissionsRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
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



```golang
type FunctionPermissionsUpdateRequest struct {
	ID string `path:"id"`
	Permissions []FunctionPermission `json:"permissions"`
}
```


3. response definition



```golang
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



```golang
type FunctionPublishRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type FunctionPublishResponse struct {
	ApprovalId string `json:"approvalId,omitempty"` // 如果需要审批
	Published bool `json:"published"`
}
```

### 12. "获取函数UI配置"

1. route definition

- Url: /api/v1/functions/:id/ui
- Method: GET
- Request: `FunctionUIRequest`
- Response: `FunctionUIResponse`

2. request definition



```golang
type FunctionUIRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type FunctionUIResponse struct {
	Schema interface{} `json:"schema"`
	Layout interface{} `json:"layout"`
	Components interface{} `json:"components"`
}
```

### 13. "更新函数UI配置"

1. route definition

- Url: /api/v1/functions/:id/ui
- Method: PUT
- Request: `FunctionUIUpdateRequest`
- Response: `FunctionUIResponse`

2. request definition



```golang
type FunctionUIUpdateRequest struct {
	ID string `path:"id"`
	Schema interface{} `json:"schema,optional"`
	Layout interface{} `json:"layout,optional"`
	Components interface{} `json:"components,optional"`
}
```


3. response definition



```golang
type FunctionUIResponse struct {
	Schema interface{} `json:"schema"`
	Layout interface{} `json:"layout"`
	Components interface{} `json:"components"`
}
```

### 14. "获取函数UI配置历史"

1. route definition

- Url: /api/v1/functions/:id/ui/history
- Method: GET
- Request: `FunctionUIHistoryRequest`
- Response: `FunctionUIHistoryResponse`

2. request definition

```golang
type FunctionUIHistoryRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```golang
type FunctionUIHistoryResponse struct {
	Items []FunctionUIHistoryItem `json:"items"`
}
```

### 15. "回滚函数UI配置"

1. route definition

- Url: /api/v1/functions/:id/ui/rollback
- Method: POST
- Request: `FunctionUIRollbackRequest`
- Response: `FunctionUIRollbackResponse`

2. request definition

```golang
type FunctionUIRollbackRequest struct {
	ID      string `path:"id"`
	Version int    `json:"version"`
}
```

3. response definition

```golang
type FunctionUIRollbackResponse struct {
	AppliedVersion int                 `json:"appliedVersion"`
	Current        *FunctionUIResponse `json:"current"`
}
```

### 16. "批量复制函数"

1. route definition

- Url: /api/v1/functions/batch-copy
- Method: POST
- Request: `BatchCopyFunctionsRequest`
- Response: `BatchCopyFunctionsResponse`

2. request definition



```golang
type BatchCopyFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
}
```


3. response definition



```golang
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



```golang
type BatchDeleteFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
}
```


3. response definition



```golang
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



```golang
type BatchUpdateFunctionsRequest struct {
	FunctionIds []string `json:"function_ids"`
	Enabled bool `json:"enabled"`
}
```


3. response definition



```golang
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



```golang
type DescriptorsRequest struct {
	Type string `form:"type,optional"`
	GameId string `form:"gameId,optional"`
}
```


3. response definition



```golang
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



```golang
type FunctionsPendingRequest struct {
}
```


3. response definition



```golang
type FunctionsPendingResponse struct {
	Items []PendingFunction `json:"items"`
}
```

