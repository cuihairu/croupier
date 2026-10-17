# 证书 API

### 1. "获取证书列表"

1. route definition

- Url: /api/v1/certificates
- Method: GET
- Request: `CertificatesListRequest`
- Response: `CertificatesListResponse`

2. request definition

```go
type CertificatesListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
	Status string `form:"status,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 2. "添加证书"

1. route definition

- Url: /api/v1/certificates
- Method: POST
- Request: `CertificateAddRequest`
- Response: `CertificateAddResponse`

2. request definition

```go
type CertificateAddRequest struct {
	Domain string `json:"domain"`
	Certificate string `json:"certificate,optional"`
	PrivateKey string `json:"privateKey,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 3. "获取证书详情"

1. route definition

- Url: /api/v1/certificates/:id
- Method: GET
- Request: `CertificateDetailRequest`
- Response: `CertificateDetailResponse`

2. request definition

```go
type CertificateDetailRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 4. "删除证书"

1. route definition

- Url: /api/v1/certificates/:id
- Method: DELETE
- Request: `CertificateDeleteRequest`
- Response: `CertificateDeleteResponse`

2. request definition

```go
type CertificateDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 5. "检查证书状态"

1. route definition

- Url: /api/v1/certificates/:id/check
- Method: POST
- Request: `CertificateCheckRequest`
- Response: `CertificateCheckResponse`

2. request definition

```go
type CertificateCheckRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 6. "添加证书告警"

1. route definition

- Url: /api/v1/certificates/alerts
- Method: POST
- Request: `CertificateAlertAddRequest`
- Response: `CertificateAlertAddResponse`

2. request definition

```go
type CertificateAlertAddRequest struct {
	Domain string `json:"domain"`
	Threshold int `json:"threshold,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 7. "获取证书告警列表"

1. route definition

- Url: /api/v1/certificates/alerts
- Method: GET
- Request: `CertificateAlertsListRequest`
- Response: `CertificateAlertsListResponse`

2. request definition

```go
type CertificateAlertsListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 8. "检查所有证书"

1. route definition

- Url: /api/v1/certificates/check-all
- Method: POST
- Request: `CertificateCheckAllRequest`
- Response: `CertificateCheckAllResponse`

2. request definition

```go
type CertificateCheckAllRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 9. "获取域名证书信息"

1. route definition

- Url: /api/v1/certificates/domain-info
- Method: GET
- Request: `CertificateDomainInfoRequest`
- Response: `CertificateDomainInfoResponse`

2. request definition

```go
type CertificateDomainInfoRequest struct {
	Domain string `form:"domain"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 10. "获取即将过期的证书"

1. route definition

- Url: /api/v1/certificates/expiring
- Method: GET
- Request: `CertificateExpiringRequest`
- Response: `CertificateExpiringResponse`

2. request definition

```go
type CertificateExpiringRequest struct {
	Days int `form:"days,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 11. "获取证书统计"

1. route definition

- Url: /api/v1/certificates/stats
- Method: GET
- Request: `CertificateStatsRequest`
- Response: `CertificateStatsResponse`

2. request definition

```go
type CertificateStatsRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
