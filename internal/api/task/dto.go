package task

type CancelRequest struct {
	ID string `uri:"id"`
}

type CancelBodyRequest struct {
	ID string `json:"id"`
}

type Item struct {
	ID         string `json:"id"`
	FunctionID string `json:"function_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Progress   int32  `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	GameID     string `json:"game_id,omitempty"`
	Env        string `json:"env,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ListRequest struct {
	Status     string `form:"status"`
	FunctionID string `form:"function_id"`
	GameID     string `form:"game_id"`
	Env        string `form:"env"`
	Page       int    `form:"page,optional,default=1"`
	Size       int    `form:"size,optional,default=20"`
}

type ListResponse struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
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
	CreatedAt string      `json:"created_at"`
}

type DetailResponse struct {
	ID         string      `json:"id"`
	FunctionID string      `json:"function_id,omitempty"`
	Status     string      `json:"status,omitempty"`
	Progress   int32       `json:"progress,omitempty"`
	Message    string      `json:"message,omitempty"`
	GameID     string      `json:"game_id,omitempty"`
	Env        string      `json:"env,omitempty"`
	AgentID    string      `json:"agent_id,omitempty"`
	Result     interface{} `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
	StartedAt  string      `json:"started_at,omitempty"`
	FinishedAt string      `json:"finished_at,omitempty"`
	CreatedAt  string      `json:"created_at,omitempty"`
	UpdatedAt  string      `json:"updated_at,omitempty"`
}

type EventsRequest struct {
	ID       string `uri:"id"`
	AfterSeq int64  `form:"after_seq"`
}

type EventsResponse struct {
	Items   []EventItem `json:"items"`
	NextSeq int64       `json:"next_seq"`
	Done    bool        `json:"done"`
}

type StartRequest struct {
	FunctionID string      `json:"function_id"`
	Params     interface{} `json:"params"`
}

type StartResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}
