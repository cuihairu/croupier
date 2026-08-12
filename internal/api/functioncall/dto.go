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
	TaskID     string      `json:"taskId"`
	FunctionID string      `json:"functionId"`
	GameID     string      `json:"gameId,omitempty"`
	Env        string      `json:"env,omitempty"`
	ActorID    string      `json:"actorId,omitempty"`
	ActorType  string      `json:"actorType,omitempty"`
	Status     string      `json:"status"`
	AgentID    string      `json:"agentId,omitempty"`
	ServiceID  string      `json:"serviceId,omitempty"`
	StartedAt  string      `json:"startedAt,omitempty"`
	FinishedAt string      `json:"finishedAt,omitempty"`
	DurationMs int64       `json:"durationMs,omitempty"`
	Payload    interface{} `json:"payload,omitempty"`
	Result     interface{} `json:"result,omitempty"`
	ErrorMsg   string      `json:"errorMsg,omitempty"`
	RetryCount int         `json:"retryCount,omitempty"`
	CreatedAt  string      `json:"createdAt,omitempty"`
}

type ListResponse struct {
	Calls    []Item `json:"calls"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
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
	AvgDurationMs float64 `json:"avgDurationMs"`
}

type RerunRequest struct {
	ID      string      `uri:"id" binding:"required"`
	Payload interface{} `json:"payload"`
}

type RerunResponse struct {
	TaskID string `json:"taskId"`
}
