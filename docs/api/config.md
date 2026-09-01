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
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
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
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
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
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
