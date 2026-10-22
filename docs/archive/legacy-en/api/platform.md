# 平台 API

### 1. "获取所有可用的第三方平台列表"

1. route definition

- Url: /api/v1/platforms
- Method: GET
- Request: `-`
- Response: `ListPlatformsResponse`

2. request definition

3. response definition

```go
type ListPlatformsResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Platforms []PlatformInfo `json:"platforms,omitempty"`
}
```

### 2. "获取指定平台支持的方法列表"

1. route definition

- Url: /api/v1/platforms/:platform/methods
- Method: GET
- Request: `-`
- Response: `ListPlatformMethodsResponse`

2. request definition

3. response definition

```go
type ListPlatformMethodsResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Methods []string `json:"methods,omitempty"`
}
```

### 3. "调用第三方平台 API"

1. route definition

- Url: /api/v1/platforms/call
- Method: POST
- Request: `CallPlatformRequest`
- Response: `CallPlatformResponse`

2. request definition

```go
type CallPlatformRequest struct {
	Platform string `json:"platform"` // 平台名称，如 "quicksdk"
	Method string `json:"method"` // API 方法名
	Request string `json:"request"` // 请求参数（JSON 字符串格式）
}
```

3. response definition

```go
type CallPlatformResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Response interface{} `json:"response,omitempty"` // 平台返回的响应
}
```
