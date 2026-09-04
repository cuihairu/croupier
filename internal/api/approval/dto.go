package approval

import "encoding/json"

// ApprovalSummary represents an approval summary
type ApprovalSummary struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	Actor           string `json:"actor"`
	FunctionID      string `json:"functionId"`
	GameID          string `json:"gameId"`
	Env             string `json:"env"`
	State           string `json:"state"`
	Mode            string `json:"mode"`
	Route           string `json:"route,omitempty"`
	IdempotencyKey  string `json:"idempotencyKey,omitempty"`
	TargetServiceID string `json:"targetServiceId,omitempty"`
	HashKey         string `json:"hashKey,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Continuation    bool   `json:"continuation,omitempty"`
	ResultKind      string `json:"resultKind,omitempty"`
	TaskID          string `json:"taskId,omitempty"`
	Result          string `json:"result,omitempty"`
}

// ApprovalDetail represents an approval detail
type ApprovalDetail struct {
	ApprovalSummary
	Payload        map[string]interface{} `json:"payload,omitempty"`
	PayloadPreview string                 `json:"payloadPreview,omitempty"`
}

// ApprovalsListRequest represents the request to list approvals
type ApprovalsListRequest struct {
	Page       int    `form:"page,optional,default=1"`
	PageSize   int    `form:"pageSize,optional,default=20"`
	Status     string `form:"status"`
	Actor      string `form:"actor"`
	FunctionID string `form:"functionId"`
	// Mine 为 true 时服务端忽略 actor 参数，强制按当前登录用户过滤
	Mine bool `form:"mine"`
}

// ApprovalsListResponse represents the response with a list of approvals
type ApprovalsListResponse struct {
	Approvals []ApprovalSummary `json:"approvals"`
	Total     int64             `json:"total"`
	Page      int               `json:"page"`
	Size      int               `json:"size"`
}

// ApprovalGetRequest represents the request to get approval details
type ApprovalGetRequest struct {
	ID string `uri:"id"`
}

// ApprovalGetResponse represents the response with approval details
type ApprovalGetResponse struct {
	Approval
}

// Approval represents an approval (for response)
type Approval struct {
	ID              string                 `json:"id"`
	CreatedAt       string                 `json:"createdAt"`
	UpdatedAt       string                 `json:"updatedAt"`
	Actor           string                 `json:"actor"`
	FunctionID      string                 `json:"functionId"`
	GameID          string                 `json:"gameId"`
	Env             string                 `json:"env"`
	State           string                 `json:"state"`
	Mode            string                 `json:"mode"`
	Route           string                 `json:"route,omitempty"`
	IdempotencyKey  string                 `json:"idempotencyKey,omitempty"`
	TargetServiceID string                 `json:"targetServiceId,omitempty"`
	HashKey         string                 `json:"hashKey,omitempty"`
	Reason          string                 `json:"reason,omitempty"`
	Continuation    bool                   `json:"continuation,omitempty"`
	ResultKind      string                 `json:"resultKind,omitempty"`
	TaskID          string                 `json:"taskId,omitempty"`
	Result          string                 `json:"result,omitempty"`
	Payload         map[string]interface{} `json:"payload,omitempty"`
	PayloadPreview  string                 `json:"payloadPreview,omitempty"`
}

// ApprovalApproveRequest represents the request to approve an approval
type ApprovalApproveRequest struct {
	ID string `uri:"id"`
}

// ApprovalApproveResponse represents the response after approving
type ApprovalApproveResponse struct {
	ID           string          `json:"id"`
	State        string          `json:"state"`
	Continuation bool            `json:"continuation"`
	ResultKind   string          `json:"resultKind,omitempty"`
	TaskID       string          `json:"taskId,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
}

// ApprovalRejectRequest represents the request to reject an approval
type ApprovalRejectRequest struct {
	ID     string `uri:"id"`
	Reason string `json:"reason"`
}

// ApprovalRejectResponse represents the response after rejecting
type ApprovalRejectResponse struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}
