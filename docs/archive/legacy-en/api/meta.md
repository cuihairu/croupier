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
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
