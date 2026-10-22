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
	ID       string `uri:"id"`
	AfterSeq int64  `form:"afterSeq"`
}

type EventsResponse struct {
	Items   []EventItem `json:"items"`
	NextSeq int64       `json:"nextSeq"`
	Done    bool        `json:"done"`
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
}
