### 1. "获取审计日志"

1. route definition

- Url: /api/v1/audit
- Method: GET
- Request: `AuditRequest`
- Response: `AuditResponse`

2. request definition



```golang
type AuditRequest struct {
	Page int `form:"page,optional"` // 页码
	PageSize int `form:"pageSize,optional"` // 每页数量
	Action string `form:"action,optional"` // 操作类型过滤
	UserID string `form:"userId,optional"` // 用户ID过滤
}
```


3. response definition



```golang
type AuditResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

