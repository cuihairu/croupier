package meta

// RootRequest 根路径请求（空）
type RootRequest struct{}

// RootResponse is the canonical REST response for GET /api/v1.
type RootResponse struct {
	Service     string            `json:"service"`
	Version     string            `json:"version"`
	Environment string            `json:"environment"`
	Timestamp   string            `json:"timestamp"`
	Features    []string          `json:"features"`
	Profiles    []string          `json:"profiles"`
	Links       map[string]string `json:"links"`
}

// AgentMetaReportRequest Agent元数据上报请求
type AgentMetaReportRequest struct{}

// AgentMetaResponse Agent元数据响应
type AgentMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NodeMetaRequest Node元数据请求
type NodeMetaRequest struct {
	ID string `uri:"id"`
}

// NodeMetaResponse Node元数据响应
type NodeMetaResponse struct {
	Meta interface{} `json:"meta"`
}

// NodeMetaUpdateRequest Node元数据更新请求
type NodeMetaUpdateRequest struct {
	ID   string      `uri:"id"`
	Meta interface{} `json:"meta"`
}

// OpsAgentMetaResponse Ops Agent元数据响应
type OpsAgentMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OpsAgentMetaUpdateRequest Ops Agent元数据更新请求
type OpsAgentMetaUpdateRequest struct {
	AgentID string      `json:"agentId"`
	Meta    interface{} `json:"meta"`
}

// OpsNodeMetaRequest Ops Node元数据请求
type OpsNodeMetaRequest struct {
	NodeID string `uri:"nodeId"`
}

// OpsNodeMetaResponse Ops Node元数据响应
type OpsNodeMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OpsServiceMetadata Ops服务元数据
type OpsServiceMetadata struct {
	Processes      []OpsServiceProcess `json:"processes"`
	ProcessesCount int                 `json:"processesCount"`
}

// OpsServiceProcess Ops服务进程信息
type OpsServiceProcess struct {
	ServiceID    string   `json:"service_id"`
	Addr         string   `json:"addr"`
	Version      string   `json:"version"`
	LastSeenUnix int64    `json:"last_seen_unix"`
	FunctionIDs  []string `json:"function_ids"`
	Functions    int      `json:"functions"`
}
