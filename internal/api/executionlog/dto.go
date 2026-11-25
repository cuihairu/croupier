package executionlog

import (
	"encoding/json"
	"time"
)

// ExecutionLogItem is an execution trace list row (no payloads; payloads are
// served by the detail endpoint).
type ExecutionLogItem struct {
	ID         int64     `json:"id"`
	GameID     string    `json:"gameId"`
	Env        string    `json:"env"`
	Source     string    `json:"source"`
	FunctionID string    `json:"functionId"`
	PageKey    string    `json:"pageKey,omitempty"`
	BindingID  string    `json:"bindingId,omitempty"`
	Actor      string    `json:"actor"`
	Route      string    `json:"route,omitempty"`
	Status     string    `json:"status"`
	DurationMs int64     `json:"durationMs"`
	TraceID    string    `json:"traceId,omitempty"`
	Truncated  bool      `json:"truncated"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ExecutionLogDetail includes the request/response payloads (already masked at
// storage time).
type ExecutionLogDetail struct {
	ExecutionLogItem
	RequestPayload json.RawMessage `json:"requestPayload,omitempty"`
	ResponseBody   json.RawMessage `json:"responseBody,omitempty"`
}

// ListRequest carries the query parameters for GET /api/v1/execution-logs.
type ListRequest struct {
	Page       int    `form:"page,optional,default=1"`
	PageSize   int    `form:"pageSize,optional,default=20"`
	FunctionID string `form:"functionId"`
	// Actor filters by applicant -- effective only in the non-mine branch
	// (which requires audit permission); when mine=true the server forces
	// actor to the currently logged-in user and ignores this parameter.
	Actor   string `form:"actor"`
	Source  string `form:"source"`
	Status  string `form:"status"`
	TraceID string `form:"traceId"`
	// From/To are RFC3339 timestamps (either one may be sent alone).
	From string `form:"from"`
	To   string `form:"to"`
	// When Mine is true, filtering is forced to the current logged-in user and
	// requires no audit:read permission.
	Mine bool `form:"mine"`
}

// ListResponse is the paginated response.
type ListResponse struct {
	Items []ExecutionLogItem `json:"items"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}

// GetRequest is the request for GET /api/v1/execution-logs/:id.
type GetRequest struct {
	ID int64 `uri:"id"`
}

// OperatorOptionsItem is one option row of GET
// /api/v1/execution-logs/operator-options.
type OperatorOptionsItem struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Count int64  `json:"count"`
}

// OperatorOptionsResponse is the operator-aggregation options response (#33).
type OperatorOptionsResponse struct {
	Items []OperatorOptionsItem `json:"items"`
}
