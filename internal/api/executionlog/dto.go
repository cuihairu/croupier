package executionlog

import (
	"encoding/json"
	"time"
)

// ExecutionLogItem 执行留痕列表项（不含载荷，载荷走详情）。
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

// ExecutionLogDetail 含请求/响应载荷（存储时已脱敏）。
type ExecutionLogDetail struct {
	ExecutionLogItem
	RequestPayload json.RawMessage `json:"requestPayload,omitempty"`
	ResponseBody   json.RawMessage `json:"responseBody,omitempty"`
}

// ListRequest GET /api/v1/execution-logs 查询参数。
type ListRequest struct {
	Page       int    `form:"page,optional,default=1"`
	PageSize   int    `form:"pageSize,optional,default=20"`
	FunctionID string `form:"functionId"`
	// Actor 按申请人过滤——仅在非 mine 分支（需审计权限）生效；
	// mine=true 时服务端强制 actor=当前登录人并忽略本参数。
	Actor   string `form:"actor"`
	Source  string `form:"source"`
	Status  string `form:"status"`
	TraceID string `form:"traceId"`
	// From/To 为 RFC3339 时间（可只传其一）。
	From string `form:"from"`
	To   string `form:"to"`
	// Mine 为 true 时强制按当前登录用户过滤，无需 audit:read 权限。
	Mine bool `form:"mine"`
}

// ListResponse 分页响应。
type ListResponse struct {
	Items []ExecutionLogItem `json:"items"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}

// GetRequest GET /api/v1/execution-logs/:id。
type GetRequest struct {
	ID int64 `uri:"id"`
}
