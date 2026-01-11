### 1. "获取证书列表"

1. route definition

- Url: /api/v1/certificates
- Method: GET
- Request: `CertificatesListRequest`
- Response: `CertificatesListResponse`

2. request definition



```golang
type CertificatesListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
	Status string `form:"status,optional"`
}
```


3. response definition



```golang
type CertificatesListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "添加证书"

1. route definition

- Url: /api/v1/certificates
- Method: POST
- Request: `CertificateAddRequest`
- Response: `CertificateAddResponse`

2. request definition



```golang
type CertificateAddRequest struct {
	Domain string `json:"domain"`
	Certificate string `json:"certificate,optional"`
	PrivateKey string `json:"privateKey,optional"`
}
```


3. response definition



```golang
type CertificateAddResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "获取证书详情"

1. route definition

- Url: /api/v1/certificates/:id
- Method: GET
- Request: `CertificateDetailRequest`
- Response: `CertificateDetailResponse`

2. request definition



```golang
type CertificateDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type CertificateDetailResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "删除证书"

1. route definition

- Url: /api/v1/certificates/:id
- Method: DELETE
- Request: `CertificateDeleteRequest`
- Response: `CertificateDeleteResponse`

2. request definition



```golang
type CertificateDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type CertificateDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 5. "检查证书状态"

1. route definition

- Url: /api/v1/certificates/:id/check
- Method: POST
- Request: `CertificateCheckRequest`
- Response: `CertificateCheckResponse`

2. request definition



```golang
type CertificateCheckRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type CertificateCheckResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 6. "添加证书告警"

1. route definition

- Url: /api/v1/certificates/alerts
- Method: POST
- Request: `CertificateAlertAddRequest`
- Response: `CertificateAlertAddResponse`

2. request definition



```golang
type CertificateAlertAddRequest struct {
	Domain string `json:"domain"`
	Threshold int `json:"threshold,optional"`
}
```


3. response definition



```golang
type CertificateAlertAddResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 7. "获取证书告警列表"

1. route definition

- Url: /api/v1/certificates/alerts
- Method: GET
- Request: `CertificateAlertsListRequest`
- Response: `CertificateAlertsListResponse`

2. request definition



```golang
type CertificateAlertsListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```


3. response definition



```golang
type CertificateAlertsListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 8. "检查所有证书"

1. route definition

- Url: /api/v1/certificates/check-all
- Method: POST
- Request: `CertificateCheckAllRequest`
- Response: `CertificateCheckAllResponse`

2. request definition



```golang
type CertificateCheckAllRequest struct {
}
```


3. response definition



```golang
type CertificateCheckAllResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 9. "获取域名证书信息"

1. route definition

- Url: /api/v1/certificates/domain-info
- Method: GET
- Request: `CertificateDomainInfoRequest`
- Response: `CertificateDomainInfoResponse`

2. request definition



```golang
type CertificateDomainInfoRequest struct {
	Domain string `form:"domain"`
}
```


3. response definition



```golang
type CertificateDomainInfoResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 10. "获取即将过期的证书"

1. route definition

- Url: /api/v1/certificates/expiring
- Method: GET
- Request: `CertificateExpiringRequest`
- Response: `CertificateExpiringResponse`

2. request definition



```golang
type CertificateExpiringRequest struct {
	Days int `form:"days,optional"`
}
```


3. response definition



```golang
type CertificateExpiringResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 11. "获取证书统计"

1. route definition

- Url: /api/v1/certificates/stats
- Method: GET
- Request: `CertificateStatsRequest`
- Response: `CertificateStatsResponse`

2. request definition



```golang
type CertificateStatsRequest struct {
}
```


3. response definition



```golang
type CertificateStatsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

