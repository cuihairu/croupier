package function

import "encoding/json"

// Function management DTOs
// Extracted from internal/types/types.go

// Function represents a registered executable capability record.
type Function struct {
	Id          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Resource    string            `json:"resource"`
	GameId      string            `json:"gameId"`
	Status      int               `json:"status"`
	Version     string            `json:"version"`
	Instances   int               `json:"instances"`
	SpecFormat  string            `json:"specFormat"`
	Tags        []string          `json:"tags,omitempty"`
	Summary     map[string]string `json:"summary,omitempty"`
	OpenAPISpec json.RawMessage   `json:"openapiSpec,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

// FunctionActionRequest represents an action request on a function
type FunctionActionRequest struct {
	ID string `uri:"id"`
}

// FunctionAnalyticsRequest represents a request for function analytics
type FunctionAnalyticsRequest struct {
	ID string `uri:"id"`
}

// FunctionAnalyticsResponse represents analytics data for a function
type FunctionAnalyticsResponse struct {
	TotalCalls     int64   `json:"totalCalls"`
	SuccessRate    float64 `json:"successRate"`
	AvgLatency     float64 `json:"avgLatency"`
	CallsToday     int64   `json:"callsToday"`
	CallsThisWeek  int64   `json:"callsThisWeek"`
	CallsThisMonth int64   `json:"callsThisMonth"`
}

// FunctionCopyRequest represents a request to copy a function
type FunctionCopyRequest struct {
	ID string `uri:"id"`
}

// FunctionCopyResponse represents the response of a function copy operation
type FunctionCopyResponse struct {
	FunctionId string `json:"functionId"`
	NewId      string `json:"newId"`
}

// FunctionDescriptor represents the input/output schema of a function
type FunctionDescriptor struct {
	Input  json.RawMessage `json:"input,omitempty"`
	Output json.RawMessage `json:"output,omitempty"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

// FunctionDetailRequest represents a request to get function details
type FunctionDetailRequest struct {
	ID string `uri:"id"`
}

// FunctionDetailResponse represents the detailed response of a function
type FunctionDetailResponse struct {
	Function
	Descriptor FunctionDescriptor `json:"descriptor"`
}

// FunctionHistoryItem represents an item in the function history
type FunctionHistoryItem struct {
	ID        string          `json:"id"`
	Action    string          `json:"action"`
	Operator  string          `json:"operator"`
	Timestamp string          `json:"timestamp"`
	Details   json.RawMessage `json:"details,omitempty"`
}

// FunctionHistoryRequest represents a request for function history
type FunctionHistoryRequest struct {
	ID string `uri:"id"`
	// Limit caps the page size (default 5, max 100); 0 uses the default.
	Limit int `form:"limit"`
	// Offset skips older entries; newest-first ordering is preserved.
	Offset int `form:"offset"`
}

// FunctionHistoryResponse represents the response containing function history
type FunctionHistoryResponse struct {
	Items []FunctionHistoryItem `json:"items"`
	Total int                   `json:"total"`
}

// Function-history pagination bounds.
const (
	functionHistoryDefaultLimit = 5
	functionHistoryMaxLimit     = 100
)

// FunctionInstance represents an instance of a function on an agent
type FunctionInstance struct {
	AgentId   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

// FunctionInstancesAllRequest represents a request to get all function instances
type FunctionInstancesAllRequest struct {
}

// FunctionInstancesAllResponse represents the response containing all function instances
type FunctionInstancesAllResponse struct {
	Instances []FunctionInstanceSummary `json:"instances"`
}

// FunctionInstanceSummary represents one function registration on an agent.
// ServiceID/Addr/Version/SDK fields identify the provider (SDK service)
// instance that serves the function; a function can have multiple instances
// from different providers behind the same agent.
type FunctionInstanceSummary struct {
	FunctionID string `json:"functionId"`
	AgentID    string `json:"agentId"`
	AgentName  string `json:"agentName"`
	ServiceID  string `json:"serviceId"`
	Addr       string `json:"addr"`
	Version    string `json:"version"`
	SDKName    string `json:"sdkName"`
	SDKLang    string `json:"sdkLang"`
	SDKVersion string `json:"sdkVersion"`
	GameID     string `json:"gameId"`
	Env        string `json:"env"`
	Status     string `json:"status"`
	UpdatedAt  string `json:"updatedAt"`
}

// FunctionInstancesRequest represents a request for function instances
type FunctionInstancesRequest struct {
	ID string `uri:"id"`
}

// FunctionInstancesResponse represents the response containing function instances
type FunctionInstancesResponse struct {
	Items []FunctionInstance `json:"items"`
}

// FunctionInvokeRequest represents a request to invoke a function
type FunctionInvokeRequest struct {
	ID              string            `uri:"id"`
	Params          json.RawMessage   `json:"params,omitempty"`
	Payload         json.RawMessage   `json:"payload,omitempty"`
	GameID          string            `json:"gameId"`
	Env             string            `json:"env"`
	Mode            string            `json:"mode"`
	Route           string            `json:"route"`
	TargetServiceID string            `json:"targetServiceId"`
	HashKey         string            `json:"hashKey"`
	Metadata        map[string]string `json:"-"`
}

// FunctionInvokeResponse represents the response of a function invocation
type FunctionInvokeResponse struct {
	TaskId           string          `json:"taskId"`
	TaskID           string          `json:"taskID,omitempty"`
	Result           json.RawMessage `json:"result,omitempty"`
	ApprovalID       string          `json:"approvalId,omitempty"`
	ApprovalRequired bool            `json:"approvalRequired,omitempty"`
	ApprovalWorkflow string          `json:"approvalWorkflow,omitempty"`
	// TraceID identifies the OTel trace of this invocation so dashboards can
	// jump to Jaeger/Grafana for the full request path.
	TraceID string `json:"traceId,omitempty"`
	// ExecutionMetadata is server-internal dispatch context for audit/tracing.
	// It is intentionally not serialized to API clients.
	ExecutionMetadata map[string]string `json:"-"`
	// Broadcast carries per-agent outcomes when route=broadcast. Empty when
	// the call did not use broadcast routing.
	Broadcast *BroadcastResult `json:"broadcast,omitempty"`
}

// BroadcastResult aggregates per-agent outcomes from a broadcast invocation.
type BroadcastResult struct {
	Total   int                  `json:"total"`
	Success int                  `json:"success"`
	Failure int                  `json:"failure"`
	Results []BroadcastAgentItem `json:"results,omitempty"`
}

// BroadcastAgentItem captures one agent's outcome in a broadcast.
type BroadcastAgentItem struct {
	AgentID string          `json:"agentId"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// FunctionPermission represents a permission for a function
type FunctionPermission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Roles    []string `json:"roles"`
}

// FunctionPermissionsRequest represents a request for function permissions
type FunctionPermissionsRequest struct {
	ID string `uri:"id"`
}

// FunctionPermissionsResponse represents the response containing function permissions
type FunctionPermissionsResponse struct {
	Items []FunctionPermission `json:"items"`
}

// FunctionPermissionsUpdateRequest represents a request to update function permissions
type FunctionPermissionsUpdateRequest struct {
	ID          string               `uri:"id"`
	Permissions []FunctionPermission `json:"permissions"`
}

// FunctionPublishRequest represents a request to publish a function
type FunctionPublishRequest struct {
	ID string `uri:"id"`
}

// FunctionPublishResponse represents the response of a function publish operation
type FunctionPublishResponse struct {
	ApprovalId string `json:"approvalId,omitempty"` // 如果需要审批
	Published  bool   `json:"published"`
}

// FunctionWarningItem represents a warning for a function
type FunctionWarningItem struct {
	Key        string `json:"key"`
	GameID     string `json:"gameId"`
	Env        string `json:"env"`
	AgentID    string `json:"agentId"`
	FunctionID string `json:"functionId"`
	Version    string `json:"version"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Count      int    `json:"count"`
	FirstSeen  string `json:"firstSeen"`
	LastSeen   string `json:"lastSeen"`
}

// FunctionWarningsRequest represents a request for function warnings
type FunctionWarningsRequest struct {
	FunctionID string `form:"functionId"`
	AgentID    string `form:"agentId"`
	Code       string `form:"code"`
	Limit      int    `form:"limit,optional,default=100"`
}

// FunctionWarningsResponse represents the response containing function warnings
type FunctionWarningsResponse struct {
	Items []FunctionWarningItem `json:"items"`
}

// FunctionsListRequest represents a request to list functions
type FunctionsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	GameId   string `form:"gameId"`
	Resource string `form:"resource"`
	Status   int    `form:"status"`
}

// FunctionsListResponse represents the response containing a list of functions
type FunctionsListResponse struct {
	Items []Function `json:"items"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"pageSize"`
}

// FunctionsPendingRequest represents a request for pending functions
type FunctionsPendingRequest struct {
}

// FunctionsPendingResponse represents the response containing pending functions
type FunctionsPendingResponse struct {
	Items []PendingFunction `json:"items"`
}

// PendingFunction represents a function pending approval
type PendingFunction struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	GameId      string `json:"gameId"`
	Env         string `json:"env"`
	CreatedAt   string `json:"createdAt"`
}

// Additional DTOs for batch operations

// FunctionDeleteRequest represents a request to delete a function
type FunctionDeleteRequest struct {
	FunctionId string `json:"functionId"`
}

// FunctionDisableRequest represents a request to disable a function
type FunctionDisableRequest struct {
	FunctionId string `json:"functionId"`
}

// FunctionEnableRequest represents a request to enable a function
type FunctionEnableRequest struct {
	FunctionId string `json:"functionId"`
}

// Batch operations DTOs

type BatchCopyFunctionsRequest struct {
	Functions []FunctionCopyRequest `json:"functions"`
}

type BatchCopyFunctionsResponse struct {
	Results []FunctionCopyResponse `json:"results"`
}

type BatchDeleteFunctionsRequest struct {
	FunctionIds []string `json:"functionIds"`
}

type BatchDeleteFunctionsResponse struct {
	Deleted []string `json:"deleted"`
	Failed  []string `json:"failed"`
}

type BatchUpdateFunctionsRequest struct {
	FunctionIds []string `json:"functionIds"`
	Enabled     bool     `json:"enabled"`
}

type BatchUpdateFunctionsResponse struct {
	Updated int      `json:"updated"`
	Failed  []string `json:"failed"`
}

// Descriptors DTOs

type DescriptorsRequest struct {
	GameId string `json:"gameId"`
	Env    string `json:"env"`
}

type Descriptor struct {
	Id          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
}

type DescriptorsResponse struct {
	Items []Descriptor `json:"items"`
}

// Type aliases for backward compatibility with service code

type ListRequest = FunctionsListRequest
type ListResponse = FunctionsListResponse
type DetailRequest = FunctionDetailRequest
type DetailResponse = FunctionDetailResponse
type DeleteRequest = FunctionDeleteRequest
type EnableRequest = FunctionEnableRequest
type DisableRequest = FunctionDisableRequest
type CopyRequest = FunctionCopyRequest
type CopyResponse = FunctionCopyResponse
type InvokeRequest = FunctionInvokeRequest
type InvokeResponse = FunctionInvokeResponse
type PublishRequest = FunctionPublishRequest
type PublishResponse = FunctionPublishResponse
type InstancesRequest = FunctionInstancesRequest
type InstancesResponse = FunctionInstancesResponse
type InstancesAllRequest = FunctionInstancesAllRequest
type InstancesAllResponse = FunctionInstancesAllResponse
type PermissionsRequest = FunctionPermissionsRequest
type PermissionsResponse = FunctionPermissionsResponse
type PermissionsUpdateRequest = FunctionPermissionsUpdateRequest
type HistoryRequest = FunctionHistoryRequest
type HistoryResponse = FunctionHistoryResponse
type AnalyticsRequest = FunctionAnalyticsRequest
type AnalyticsResponse = FunctionAnalyticsResponse
type WarningsRequest = FunctionWarningsRequest
type WarningsResponse = FunctionWarningsResponse
type PendingRequest = FunctionsPendingRequest
type PendingResponse = FunctionsPendingResponse
