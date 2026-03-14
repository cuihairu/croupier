package functioncall

type ListRequest struct {
	FunctionID string `form:"function_id"`
	GameID     string `form:"game_id"`
	Env        string `form:"env"`
	Status     string `form:"status"`
	ActorID    string `form:"actor_id"`
	AgentID    string `form:"agent_id"`
	StartTime  string `form:"start_time"`
	EndTime    string `form:"end_time"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

type Item struct {
	ID         string      `json:"id"`
	JobID      string      `json:"job_id"`
	FunctionID string      `json:"function_id"`
	GameID     string      `json:"game_id,omitempty"`
	Env        string      `json:"env,omitempty"`
	ActorID    string      `json:"actor_id,omitempty"`
	ActorType  string      `json:"actor_type,omitempty"`
	Status     string      `json:"status"`
	AgentID    string      `json:"agent_id,omitempty"`
	ServiceID  string      `json:"service_id,omitempty"`
	StartedAt  string      `json:"started_at,omitempty"`
	FinishedAt string      `json:"finished_at,omitempty"`
	DurationMs int64       `json:"duration_ms,omitempty"`
	Payload    interface{} `json:"payload,omitempty"`
	Result     interface{} `json:"result,omitempty"`
	ErrorMsg   string      `json:"error_msg,omitempty"`
	RetryCount int         `json:"retry_count,omitempty"`
	CreatedAt  string      `json:"created_at,omitempty"`
}

type ListResponse struct {
	Calls    []Item `json:"calls"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type DetailRequest struct {
	ID string `uri:"id" binding:"required"`
}

type StatsResponse struct {
	Total         int     `json:"total"`
	Succeeded     int     `json:"succeeded"`
	Failed        int     `json:"failed"`
	Running       int     `json:"running"`
	Cancelled     int     `json:"cancelled"`
	Timeout       int     `json:"timeout"`
	Other         int     `json:"other"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

type RerunRequest struct {
	ID      string      `uri:"id" binding:"required"`
	Payload interface{} `json:"payload"`
}

type RerunResponse struct {
	JobID string `json:"job_id"`
}
