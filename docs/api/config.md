# 配置 API

### 1. "创建或更新配置"

1. route definition

- Url: /api/v1/configs
- Method: POST
- Request: `ConfigUpsertRequest`
- Response: `ConfigUpsertResponse`

2. request definition



```go
type ConfigUpsertRequest struct {
	Key string `json:"key"` // 配置键
	Value string `json:"value"` // 配置值
}
```


3. response definition



```go
type ConfigUpsertResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "获取配置版本详情"

1. route definition

- Url: /api/v1/configs/version
- Method: GET
- Request: `ConfigVersionDetailRequest`
- Response: `ConfigVersionDetailResponse`

2. request definition



```go
type ConfigVersionDetailRequest struct {
	Key string `form:"key"` // 配置键
	Version int `form:"version"` // 版本号
}
```


3. response definition



```go
type ConfigVersionDetailResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "获取配置版本列表"

1. route definition

- Url: /api/v1/configs/versions
- Method: GET
- Request: `ConfigVersionsRequest`
- Response: `ConfigVersionsResponse`

2. request definition



```go
type ConfigVersionsRequest struct {
	Key string `form:"key"` // 配置键
}
```


3. response definition



```go
type ConfigVersionsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

