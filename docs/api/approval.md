# 审批 API

### 1. "获取审批列表"

1. route definition

- Url: /api/v1/approvals
- Method: GET
- Request: `ApprovalsListRequest`
- Response: `ApprovalsListResponse`

2. request definition

```go
type ApprovalsListRequest struct {
	Page int `form:"page,optional"` // 页码
	PageSize int `form:"pageSize,optional"` // 每页数量
	Status string `form:"status,optional"` // 状态过滤
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 2. "获取审批详情"

1. route definition

- Url: /api/v1/approvals/:id
- Method: GET
- Request: `ApprovalGetRequest`
- Response: `ApprovalGetResponse`

2. request definition

```go
type ApprovalGetRequest struct {
	ID string `path:"id"` // 审批ID
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 3. "通过审批"

1. route definition

- Url: /api/v1/approvals/:id/approve
- Method: POST
- Request: `ApprovalApproveRequest`
- Response: `ApprovalApproveResponse`

2. request definition

```go
type ApprovalApproveRequest struct {
	ID string `path:"id"` // 审批ID
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 4. "拒绝审批"

1. route definition

- Url: /api/v1/approvals/:id/reject
- Method: POST
- Request: `ApprovalRejectRequest`
- Response: `ApprovalRejectResponse`

2. request definition

```go
type ApprovalRejectRequest struct {
	ID string `path:"id"` // 审批ID
	Reason string `json:"reason"` // 拒绝原因
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
