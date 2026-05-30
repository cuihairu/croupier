# Schema API

### 1. "获取模式列表"

1. route definition

- Url: /api/v1/schemas
- Method: GET
- Request: `SchemasListRequest`
- Response: `SchemasListResponse`

2. request definition



```go
type SchemasListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```


3. response definition



```go
type SchemasListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "创建模式"

1. route definition

- Url: /api/v1/schemas
- Method: POST
- Request: `SchemaCreateRequest`
- Response: `SchemaCreateResponse`

2. request definition



```go
type SchemaCreateRequest struct {
	Name string `json:"name"`
	Schema interface{} `json:"schema"`
}
```


3. response definition



```go
type SchemaCreateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "获取模式详情"

1. route definition

- Url: /api/v1/schemas/:id
- Method: GET
- Request: `SchemaDetailRequest`
- Response: `SchemaDetailResponse`

2. request definition



```go
type SchemaDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```go
type SchemaDetailResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "更新模式"

1. route definition

- Url: /api/v1/schemas/:id
- Method: PUT
- Request: `SchemaUpdateRequest`
- Response: `SchemaUpdateResponse`

2. request definition



```go
type SchemaUpdateRequest struct {
	ID string `path:"id"`
	Schema interface{} `json:"schema"`
}
```


3. response definition



```go
type SchemaUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 5. "删除模式"

1. route definition

- Url: /api/v1/schemas/:id
- Method: DELETE
- Request: `SchemaDeleteRequest`
- Response: `SchemaDeleteResponse`

2. request definition



```go
type SchemaDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```go
type SchemaDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 6. "获取模式UI配置"

1. route definition

- Url: /api/v1/schemas/:id/ui-config
- Method: GET
- Request: `SchemaUIConfigRequest`
- Response: `SchemaUIConfigResponse`

2. request definition



```go
type SchemaUIConfigRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```go
type SchemaUIConfigResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 7. "更新模式UI配置"

1. route definition

- Url: /api/v1/schemas/:id/ui-config
- Method: PUT
- Request: `SchemaUIConfigUpdateRequest`
- Response: `SchemaUIConfigUpdateResponse`

2. request definition



```go
type SchemaUIConfigUpdateRequest struct {
	ID string `path:"id"`
	Config interface{} `json:"config"`
}
```


3. response definition



```go
type SchemaUIConfigUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 8. "验证模式数据"

1. route definition

- Url: /api/v1/schemas/:id/validate
- Method: POST
- Request: `SchemaValidateRequest`
- Response: `SchemaValidateResponse`

2. request definition



```go
type SchemaValidateRequest struct {
	ID string `path:"id"`
	Data interface{} `json:"data"`
}
```


3. response definition



```go
type SchemaValidateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 9. "原始模式验证"

1. route definition

- Url: /api/v1/schemas/raw-validate
- Method: POST
- Request: `SchemaRawValidateRequest`
- Response: `SchemaRawValidateResponse`

2. request definition



```go
type SchemaRawValidateRequest struct {
	Schema interface{} `json:"schema"`
	Data interface{} `json:"data"`
}
```


3. response definition



```go
type SchemaRawValidateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

