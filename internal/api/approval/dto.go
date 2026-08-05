package approval

import "encoding/json"

// ApprovalSummary represents an approval summary
type ApprovalSummary struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Actor           string `json:"actor"`
	FunctionID      string `json:"function_id"`
	GameID          string `json:"game_id"`
	Env             string `json:"env"`
	State           string `json:"state"`
	Mode            string `json:"mode"`
	Route           string `json:"route,omitempty"`
	IdempotencyKey  string `json:"idempotency_key,omitempty"`
	TargetServiceID string `json:"target_service_id,omitempty"`
	HashKey         string `json:"hash_key,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Continuation    bool   `json:"continuation,omitempty"`
	ResultKind      string `json:"result_kind,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	Result          string `json:"result,omitempty"`
}

// ApprovalDetail represents an approval detail
type ApprovalDetail struct {
	ApprovalSummary
	Payload        map[string]interface{} `json:"payload,omitempty"`
	PayloadPreview string                 `json:"payload_preview,omitempty"`
}

// ApprovalsListRequest represents the request to list approvals
type ApprovalsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Status   string `form:"status"`
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
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
	Actor           string                 `json:"actor"`
	FunctionID      string                 `json:"function_id"`
	GameID          string                 `json:"game_id"`
	Env             string                 `json:"env"`
	State           string                 `json:"state"`
	Mode            string                 `json:"mode"`
	Route           string                 `json:"route,omitempty"`
	IdempotencyKey  string                 `json:"idempotency_key,omitempty"`
	TargetServiceID string                 `json:"target_service_id,omitempty"`
	HashKey         string                 `json:"hash_key,omitempty"`
	Reason          string                 `json:"reason,omitempty"`
	Continuation    bool                   `json:"continuation,omitempty"`
	ResultKind      string                 `json:"result_kind,omitempty"`
	TaskID          string                 `json:"task_id,omitempty"`
	Result          string                 `json:"result,omitempty"`
	Payload         map[string]interface{} `json:"payload,omitempty"`
	PayloadPreview  string                 `json:"payload_preview,omitempty"`
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
	ResultKind   string          `json:"result_kind,omitempty"`
	TaskID       string          `json:"task_id,omitempty"`
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
