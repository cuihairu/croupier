package meta

// RootRequest is the root path request (empty).
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

// AgentMetaReportRequest is the agent metadata report request.
type AgentMetaReportRequest struct{}

// AgentMetaResponse is the agent metadata response.
type AgentMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NodeMetaRequest is the node metadata request.
type NodeMetaRequest struct {
	ID string `uri:"id"`
}

// NodeMetaResponse is the node metadata response.
type NodeMetaResponse struct {
	Meta interface{} `json:"meta"`
}

// NodeMetaUpdateRequest is the node metadata update request.
type NodeMetaUpdateRequest struct {
	ID   string      `uri:"id"`
	Meta interface{} `json:"meta"`
}

// OpsAgentMetaResponse is the ops agent metadata response.
type OpsAgentMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OpsAgentMetaUpdateRequest is the ops agent metadata update request.
type OpsAgentMetaUpdateRequest struct {
	AgentID string      `json:"agentId"`
	Meta    interface{} `json:"meta"`
}

// OpsNodeMetaRequest is the ops node metadata request.
type OpsNodeMetaRequest struct {
	NodeID string `uri:"nodeId"`
}

// OpsNodeMetaResponse is the ops node metadata response.
type OpsNodeMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OpsServiceMetadata is the ops service metadata.
type OpsServiceMetadata struct {
	Processes      []OpsServiceProcess `json:"processes"`
	ProcessesCount int                 `json:"processesCount"`
}

// OpsServiceProcess is ops service process information.
type OpsServiceProcess struct {
	ServiceID    string   `json:"serviceId"`
	Addr         string   `json:"addr"`
	Version      string   `json:"version"`
	LastSeenUnix int64    `json:"lastSeenUnix"`
	FunctionIDs  []string `json:"functionIds"`
	Functions    int      `json:"functions"`
}
