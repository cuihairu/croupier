package task

type CancelRequest struct {
	ID string `uri:"id"`
}

type CancelBodyRequest struct {
	ID string `json:"id"`
}

type Item struct {
	ID         string `json:"id"`
	FunctionID string `json:"functionId,omitempty"`
	Status     string `json:"status,omitempty"`
	Progress   int32  `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	GameID     string `json:"gameId,omitempty"`
	Env        string `json:"env,omitempty"`
	AgentID    string `json:"agentId,omitempty"`
	Actor      string `json:"actor,omitempty"`      // 操作者
	Addr       string `json:"addr,omitempty"`       // Agent 服务地址
	TraceID    string `json:"traceId,omitempty"`    // 链路追踪 ID
	DurationMs int64  `json:"durationMs,omitempty"` // 耗时（毫秒）
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ListRequest struct {
	Status     string `form:"status"`
	FunctionID string `form:"functionId"`
	GameID     string `form:"gameId"`
	Env        string `form:"env"`
	// Actor 按操作者过滤（#23：此前前端发本参数但服务端 DTO 未声明，
	// BindQueryCompat 不认识即静默丢弃，页面退化为客户端兜底过滤）。
	Actor string `form:"actor"`
	Page  int    `form:"page,optional,default=1"`
	Size  int    `form:"size,optional,default=20"`
}

type ListResponse struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
	Page  int    `json:"page"`
	Size  int    `json:"pageSize"`
}

// OperatorOptionsItem GET /api/v1/tasks/operator-options 的选项行：
// 操作者 + 其留痕条数（value 即 actor 本身）。
type OperatorOptionsItem struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Count int64  `json:"count"`
}

type OperatorOptionsResponse struct {
	Items []OperatorOptionsItem `json:"items"`
}

type DetailRequest struct {
	ID string `uri:"id"`
}

type EventItem struct {
	Seq       int64       `json:"seq"`
	Type      string      `json:"type"`
	Progress  int32       `json:"progress"`
	Message   string      `json:"message"`
	Payload   interface{} `json:"payload"`
	CreatedAt string      `json:"createdAt"`
}

type DetailResponse struct {
	ID         string      `json:"id"`
	FunctionID string      `json:"functionId,omitempty"`
	Status     string      `json:"status,omitempty"`
	Progress   int32       `json:"progress,omitempty"`
	Message    string      `json:"message,omitempty"`
	GameID     string      `json:"gameId,omitempty"`
	Env        string      `json:"env,omitempty"`
	AgentID    string      `json:"agentId,omitempty"`
	Actor      string      `json:"actor,omitempty"`
	Addr       string      `json:"addr,omitempty"`
	TraceID    string      `json:"traceId,omitempty"`
	DurationMs int64       `json:"durationMs,omitempty"`
	Result     interface{} `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
	StartedAt  string      `json:"startedAt,omitempty"`
	FinishedAt string      `json:"finishedAt,omitempty"`
	CreatedAt  string      `json:"createdAt,omitempty"`
	UpdatedAt  string      `json:"updatedAt,omitempty"`
}

type EventsRequest struct {
	ID string `uri:"id"`
	// Legacy polling cursor: the server filters with seq > AfterSeq. Callers
	// that pass NextSeq back here keep their historical behaviour (which can
	// skip an event appended after that response); see AUD-089.
	AfterSeq int64 `form:"afterSeq"`
	// Cursor is the new, directly round-trippable continuation cursor. It is
	// the last sequence number the caller has read, so seq > Cursor is the
	// next page. A pointer distinguishes an explicit cursor=0 from an absent
	// cursor; when present it takes precedence over AfterSeq/after_seq.
	Cursor *int64 `form:"cursor"`
}

type EventsResponse struct {
	Items []EventItem `json:"items"`
	// Cursor is the highest sequence number included in Items (or the
	// starting cursor when Items is empty). Pass it back as `cursor` to read
	// only strictly newer events, with no duplicates and no skipped sequence.
	Cursor int64 `json:"cursor"`
	// NextSeq is the legacy cursor (highest seq + 1) and is intentionally
	// unchanged. It is NOT a drop-in value for AfterSeq when new events may
	// have been appended; new callers should use Cursor instead.
	NextSeq int64 `json:"nextSeq"`
	Done    bool  `json:"done"`
}

type StartRequest struct {
	FunctionID string      `json:"functionId"`
	Params     interface{} `json:"params"`
	GameID     string      `json:"gameId,omitempty"`
	Env        string      `json:"env,omitempty"`
}

type StartResponse struct {
	TaskID string `json:"taskId"`
	Status string `json:"status"`
	// ApprovalID/ApprovalWorkflow：政策要求审批时返回（ARCH-024，与
	// functionInvoke 的 ApprovalRequired 响应同构），任务未派发。
	ApprovalID       string `json:"approvalId,omitempty"`
	ApprovalWorkflow string `json:"approvalWorkflow,omitempty"`
}
