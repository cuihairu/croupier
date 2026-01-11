### 1. "获取提供者列表"

1. route definition

- Url: /api/v1/providers
- Method: GET
- Request: `ProvidersListRequest`
- Response: `ProvidersListResponse`

2. request definition



```golang
type ProvidersListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```


3. response definition



```golang
type ProvidersListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "获取提供者详情"

1. route definition

- Url: /api/v1/providers/:id
- Method: GET
- Request: `ProviderDetailRequest`
- Response: `ProviderDetailResponse`

2. request definition



```golang
type ProviderDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ProviderDetailResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "删除提供者"

1. route definition

- Url: /api/v1/providers/:id
- Method: DELETE
- Request: `ProviderActionRequest`
- Response: `ProviderDeleteResponse`

2. request definition



```golang
type ProviderActionRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ProviderDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "获取提供者实体"

1. route definition

- Url: /api/v1/providers/:id/entities
- Method: GET
- Request: `ProvidersEntitiesRequest`
- Response: `ProvidersEntitiesResponse`

2. request definition



```golang
type ProvidersEntitiesRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ProvidersEntitiesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 5. "重新加载提供者"

1. route definition

- Url: /api/v1/providers/:id/reload
- Method: POST
- Request: `ProviderActionRequest`
- Response: `ProviderReloadResponse`

2. request definition



```golang
type ProviderActionRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type ProviderReloadResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 6. "获取提供者能力"

1. route definition

- Url: /api/v1/providers/capabilities
- Method: GET
- Request: `ProvidersCapabilitiesRequest`
- Response: `ProvidersCapabilitiesResponse`

2. request definition



```golang
type ProvidersCapabilitiesRequest struct {
}
```


3. response definition



```golang
type ProvidersCapabilitiesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 7. "获取提供者描述符"

1. route definition

- Url: /api/v1/providers/descriptors
- Method: GET
- Request: `ProvidersDescriptorsRequest`
- Response: `ProvidersDescriptorsResponse`

2. request definition



```golang
type ProvidersDescriptorsRequest struct {
}
```


3. response definition



```golang
type ProvidersDescriptorsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

