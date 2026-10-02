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
	ID string `uri:"id"`
	// OTP step-up 动态码：治理风险 high/danger 的审批必填（OPEN-ISSUES #75，
	// 受 security.approvalStepUpOtp 总开关控制，默认开——OPEN-ISSUES #61）。
	// 契约键 otp lowerCamelCase，与 Web/Mobile 既有发送形态一致；
	// 中低风险可选，带了则校验（错的拒绝）。
	OTP string `json:"otp,omitempty"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

4. step-up 稳定错误码（API Response Contract）

| 状态码 | error              | 语义                                                                |
| ------ | ------------------ | ------------------------------------------------------------------- |
| 403    | `otp_required`     | 高危审批未带 otp（开关关闭时不出现）                                |
| 400    | `otp_invalid`      | 带的 otp 未通过校验（含超长等畸形输入；开关关闭下带了错码同样拒绝） |
| 403    | `otp_not_enrolled` | 高危审批但账号未绑定 TOTP（提示回 Web「个人中心-安全设置」绑定）    |

otp 值永不写入审计/日志；批准审计行 `stepUp` 档位 ∈ `not_required` / `totp` /
`disabled`（`disabled` = 总开关关闭放行的高危审批，OPEN-ISSUES #61）。

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
