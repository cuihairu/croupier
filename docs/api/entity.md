# 实体 API

### 1. "获取实体列表"

1. route definition

- Url: /api/v1/entities
- Method: GET
- Request: `EntitiesListRequest`
- Response: `EntitiesListResponse`

2. request definition



```golang
type EntitiesListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
	Type string `form:"type,optional"`
}
```


3. response definition



```golang
type EntitiesListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "创建实体"

1. route definition

- Url: /api/v1/entities
- Method: POST
- Request: `EntityCreateRequest`
- Response: `EntityCreateResponse`

2. request definition



```golang
type EntityCreateRequest struct {
	Type string `json:"type"`
	Data interface{} `json:"data"`
}
```


3. response definition



```golang
type EntityCreateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "获取实体详情"

1. route definition

- Url: /api/v1/entities/:id
- Method: GET
- Request: `EntityDetailRequest`
- Response: `EntityDetailResponse`

2. request definition



```golang
type EntityDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type EntityDetailResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "更新实体"

1. route definition

- Url: /api/v1/entities/:id
- Method: PUT
- Request: `EntityUpdateRequest`
- Response: `EntityUpdateResponse`

2. request definition



```golang
type EntityUpdateRequest struct {
	ID string `path:"id"`
	Data interface{} `json:"data"`
}
```


3. response definition



```golang
type EntityUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 5. "删除实体"

1. route definition

- Url: /api/v1/entities/:id
- Method: DELETE
- Request: `EntityDeleteRequest`
- Response: `EntityDeleteResponse`

2. request definition



```golang
type EntityDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type EntityDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 6. "预览实体"

1. route definition

- Url: /api/v1/entities/:id/preview
- Method: GET
- Request: `EntityPreviewRequest`
- Response: `EntityPreviewResponse`

2. request definition



```golang
type EntityPreviewRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type EntityPreviewResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 7. "验证实体"

1. route definition

- Url: /api/v1/entities/validate
- Method: POST
- Request: `EntityValidateRequest`
- Response: `EntityValidateResponse`

2. request definition



```golang
type EntityValidateRequest struct {
	Type string `json:"type"`
	Data interface{} `json:"data"`
}
```


3. response definition



```golang
type EntityValidateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

