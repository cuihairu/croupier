# Provider API

### 1. "获取提供者列表"

1. route definition

- Url: /api/v1/providers
- Method: GET
- Request: `ProvidersListRequest`
- Response: `ProvidersListResponse`

2. request definition

```go
type ProvidersListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 2. "获取提供者详情"

1. route definition

- Url: /api/v1/providers/:id
- Method: GET
- Request: `ProviderDetailRequest`
- Response: `ProviderDetailResponse`

2. request definition

```go
type ProviderDetailRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 3. "删除提供者"

1. route definition

- Url: /api/v1/providers/:id
- Method: DELETE
- Request: `ProviderActionRequest`
- Response: `ProviderDeleteResponse`

2. request definition

```go
type ProviderActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 4. "获取提供者资源"

1. route definition

- Url: /api/v1/providers/:id/resources
- Method: GET
- Request: `ProvidersResourcesRequest`
- Response: `ProvidersResourcesResponse`

2. request definition

```go
type ProvidersResourcesRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 5. "重新加载提供者"

1. route definition

- Url: /api/v1/providers/:id/reload
- Method: POST
- Request: `ProviderActionRequest`
- Response: `ProviderReloadResponse`

2. request definition

```go
type ProviderActionRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 6. "获取提供者能力"

1. route definition

- Url: /api/v1/providers/capabilities
- Method: GET
- Request: `ProvidersCapabilitiesRequest`
- Response: `ProvidersCapabilitiesResponse`

2. request definition

```go
type ProvidersCapabilitiesRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 7. "获取提供者描述符"

1. route definition

- Url: /api/v1/providers/descriptors
- Method: GET
- Request: `ProvidersDescriptorsRequest`
- Response: `ProvidersDescriptorsResponse`

2. request definition

```go
type ProvidersDescriptorsRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
