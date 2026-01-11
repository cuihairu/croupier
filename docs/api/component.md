### 1. "获取组件列表"

1. route definition

- Url: /api/v1/components
- Method: GET
- Request: `ComponentsListRequest`
- Response: `ComponentsListResponse`

2. request definition



```golang
type ComponentsListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```


3. response definition



```golang
type ComponentsListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "获取组件详情"

1. route definition

- Url: /api/v1/components/:id
- Method: GET
- Request: `ComponentDetailRequest`
- Response: `ComponentsDetailResponse`

2. request definition



```golang
type ComponentDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ComponentsDetailResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "删除组件"

1. route definition

- Url: /api/v1/components/:id
- Method: DELETE
- Request: `ComponentActionRequest`
- Response: `ComponentsDeleteResponse`

2. request definition



```golang
type ComponentActionRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ComponentsDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "更新组件配置"

1. route definition

- Url: /api/v1/components/:id
- Method: PATCH
- Request: `ComponentPatchRequest`
- Response: `ComponentsPatchResponse`

2. request definition



```golang
type ComponentPatchRequest struct {
	ID string `path:"id"`
	Patch interface{} `json:"patch"`
}
```


3. response definition



```golang
type ComponentsPatchResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 5. "禁用组件"

1. route definition

- Url: /api/v1/components/:id/disable
- Method: POST
- Request: `ComponentActionRequest`
- Response: `ComponentsDisableResponse`

2. request definition



```golang
type ComponentActionRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ComponentsDisableResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 6. "启用组件"

1. route definition

- Url: /api/v1/components/:id/enable
- Method: POST
- Request: `ComponentActionRequest`
- Response: `ComponentsEnableResponse`

2. request definition



```golang
type ComponentActionRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ComponentsEnableResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 7. "安装组件"

1. route definition

- Url: /api/v1/components/install
- Method: POST
- Request: `ComponentsInstallRequest`
- Response: `ComponentsInstallResponse`

2. request definition



```golang
type ComponentsInstallRequest struct {
	Name string `json:"name"`
	Version string `json:"version,optional"`
}
```


3. response definition



```golang
type ComponentsInstallResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

