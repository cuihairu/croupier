# 存储 API

### 1. "获取签名URL"

1. route definition

- Url: /api/v1/storage/signed-url
- Method: GET
- Request: `SignedUrlRequest`
- Response: `SignedUrlResponse`

2. request definition

```go
type SignedUrlRequest struct {
	Path string `form:"path"`
	Expire int `form:"expire,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
