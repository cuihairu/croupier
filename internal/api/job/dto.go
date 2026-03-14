package job

type JobCancelRequest struct {
	ID string `uri:"id"` // 任务ID
}

type JobCancelBodyRequest struct {
	ID string `json:"id"`
}

type JobCancelResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type JobItem struct {
	ID         string `json:"id"`
	FunctionID string `json:"function_id,omitempty"`
	Actor      string `json:"actor,omitempty"`
	State      string `json:"state,omitempty"`
	GameID     string `json:"game_id,omitempty"`
	Env        string `json:"env,omitempty"`
	RPCAddr    string `json:"rpc_addr,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	EndedAt    string `json:"ended_at,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

type JobListRequest struct {
	Status     string `form:"status"`
	FunctionID string `form:"function_id"`
	Actor      string `form:"actor"`
	GameID     string `form:"game_id"`
	Env        string `form:"env"`
	Page       int    `form:"page,optional,default=1"`
	Size       int    `form:"size,optional,default=20"`
}

type JobListResponse struct {
	Jobs  []JobItem `json:"jobs"`
	Total int       `json:"total"`
}

type JobResultRequest struct {
	ID string `uri:"id"` // 任务ID
}

// JobResultResponse is the response format used by the service layer
// with job-specific fields JobID, Done, and Events
type JobResultResponse struct {
	JobID  string                   `json:"jobId"`
	Done   bool                     `json:"done"`
	Events []map[string]interface{} `json:"events"`
}

type JobStartRequest struct {
	FunctionID string      `json:"functionId"` // 函数ID
	Params     interface{} `json:"params"`
}

// JobStartResponse is the response format used by the service layer
// with JobID field
type JobStartResponse struct {
	JobID string `json:"jobId"`
}

type StreamJobRequest struct {
	JobID string `uri:"jobId"`
}

// StreamJobResponse is the service-layer response format with Events and Done
type StreamJobResponse struct {
	Events []map[string]interface{} `json:"events"`
	Done   bool                     `json:"done"`
}

// StreamJobAPIResponse is the API-layer response format matching types.go
// Deprecated: Use StreamJobResponse for service layer, handlers wrap with response.Success()
type StreamJobAPIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Type aliases for backward compatibility with types.go
// Deprecated: Use StreamJobRequest directly
type TypesStreamJobRequest = StreamJobRequest

// Deprecated: Use StreamJobAPIResponse if you need API format, or StreamJobResponse for service layer
type TypesStreamJobResponse = StreamJobAPIResponse

type StreamMessagesRequest struct {
}

type StreamMessagesResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
