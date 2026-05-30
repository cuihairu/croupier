# 元数据 API

### 1. "根路径 - API 信息和版本"

1. route definition

- Url: /api/v1
- Method: GET
- Request: `RootRequest`
- Response: `RootResponse`

2. request definition



```go
type RootRequest struct {
}
```


3. response definition



```go
type RootResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

