package function

// Function management DTOs
// Extracted from internal/types/types.go

// Function represents a function entity
type Function struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	GameId      string      `json:"gameId"`
	Status      int         `json:"status"`
	Version     string      `json:"version"`
	Instances   int         `json:"instances"`
	SpecFormat  string      `json:"specFormat"`
	OpenAPISpec interface{} `json:"openapiSpec"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
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
	FunctionId string `json:"function_id"`
	NewId      string `json:"new_id"`
}

// FunctionDescriptor represents the input/output schema of a function
type FunctionDescriptor struct {
	Input  interface{} `json:"input"`
	Output interface{} `json:"output"`
	Schema interface{} `json:"schema"`
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
	ID        string      `json:"id"`
	Action    string      `json:"action"`
	Operator  string      `json:"operator"`
	Timestamp string      `json:"timestamp"`
	Details   interface{} `json:"details"`
}

// FunctionHistoryRequest represents a request for function history
type FunctionHistoryRequest struct {
	ID string `uri:"id"`
}

// FunctionHistoryResponse represents the response containing function history
type FunctionHistoryResponse struct {
	Items []FunctionHistoryItem `json:"items"`
}

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
	Instances []map[string]interface{} `json:"instances"`
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
	ID              string      `uri:"id"`
	Params          interface{} `json:"params"`
	Payload         interface{} `json:"payload"`
	GameID          string      `json:"gameId"`
	Env             string      `json:"env"`
	Mode            string      `json:"mode"`
	Route           string      `json:"route"`
	TargetServiceID string      `json:"target_service_id"`
	HashKey         string      `json:"hash_key"`
}

// FunctionInvokeResponse represents the response of a function invocation
type FunctionInvokeResponse struct {
	JobId  string      `json:"jobId"`
	JobID  string      `json:"jobID,omitempty"`
	Result interface{} `json:"result,omitempty"`
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

// FunctionRouteConfig represents the route configuration for a function
type FunctionRouteConfig struct {
	Nodes  []string `json:"nodes"`
	Path   string   `json:"path"`
	Order  int      `json:"order"`
	Hidden bool     `json:"hidden"`
}

// FunctionRouteRequest represents a request for function route configuration
type FunctionRouteRequest struct {
	ID string `uri:"id"`
}

// FunctionRouteResponse represents the response containing function route configuration
type FunctionRouteResponse struct {
	Menu   FunctionRouteConfig `json:"menu"`
	Source string              `json:"source"`
}

// FunctionRouteUpdateRequest represents a request to update function route configuration
type FunctionRouteUpdateRequest struct {
	ID     string   `uri:"id"`
	Nodes  []string `json:"nodes"`
	Path   string   `json:"path"`
	Order  int      `json:"order"`
	Hidden bool     `json:"hidden"`
}

// FunctionUIHistoryItem represents an item in the function UI history
type FunctionUIHistoryItem struct {
	Version    int         `json:"version"`
	Schema     interface{} `json:"schema"`
	Layout     interface{} `json:"layout"`
	Components interface{} `json:"components"`
	Message    string      `json:"message"`
	CreatedBy  string      `json:"createdBy"`
	CreatedAt  string      `json:"createdAt"`
}

// FunctionUIHistoryRequest represents a request for function UI history
type FunctionUIHistoryRequest struct {
	ID string `uri:"id"`
}

// FunctionUIHistoryResponse represents the response containing function UI history
type FunctionUIHistoryResponse struct {
	Items []FunctionUIHistoryItem `json:"items"`
}

// FunctionUIRequest represents a request for function UI configuration
type FunctionUIRequest struct {
	ID string `uri:"id"`
}

// FunctionUIResponse represents the response containing function UI configuration
type FunctionUIResponse struct {
	Schema         interface{} `json:"schema"`
	Layout         interface{} `json:"layout"`
	Components     interface{} `json:"components"`
	Custom         bool        `json:"custom"`
	HasDefault     bool        `json:"hasDefault"`
	UISource       string      `json:"uiSource"`       // custom_metadata/config_file_override/openapi_x_ui/none
	UISourceDetail string      `json:"uiSourceDetail"` // human-readable source description
}

// FunctionUIRollbackRequest represents a request to rollback function UI
type FunctionUIRollbackRequest struct {
	ID      string `uri:"id"`
	Version int    `json:"version"`
}

// FunctionUIRollbackResponse represents the response of a function UI rollback
type FunctionUIRollbackResponse struct {
	AppliedVersion int                 `json:"appliedVersion"`
	Current        *FunctionUIResponse `json:"current"`
}

// FunctionUIUpdateRequest represents a request to update function UI
type FunctionUIUpdateRequest struct {
	ID         string      `uri:"id"`
	Schema     interface{} `json:"schema"`
	Layout     interface{} `json:"layout"`
	Components interface{} `json:"components"`
}

// FunctionWarningItem represents a warning for a function
type FunctionWarningItem struct {
	Key        string `json:"key"`
	AgentID    string `json:"agent_id"`
	FunctionID string `json:"function_id"`
	Version    string `json:"version"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Count      int    `json:"count"`
	FirstSeen  string `json:"first_seen"`
	LastSeen   string `json:"last_seen"`
}

// FunctionWarningsRequest represents a request for function warnings
type FunctionWarningsRequest struct {
	FunctionID string `form:"function_id"`
	AgentID    string `form:"agent_id"`
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
	Category string `form:"category"`
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
	Updates []FunctionRouteUpdateRequest `json:"updates"`
}

type BatchUpdateFunctionsResponse struct {
	Results []FunctionRouteResponse `json:"results"`
}

// Descriptors DTOs

type DescriptorsRequest struct {
	GameId string `json:"gameId"`
	Env    string `json:"env"`
}

type Descriptor struct {
	Id          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
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
type UIRequest = FunctionUIRequest
type UIResponse = FunctionUIResponse
type UIUpdateRequest = FunctionUIUpdateRequest
type UIHistoryRequest = FunctionUIHistoryRequest
type UIHistoryResponse = FunctionUIHistoryResponse
type UIRollbackRequest = FunctionUIRollbackRequest
type RouteRequest = FunctionRouteRequest
type RouteResponse = FunctionRouteResponse
type RouteUpdateRequest = FunctionRouteUpdateRequest
type HistoryRequest = FunctionHistoryRequest
type HistoryResponse = FunctionHistoryResponse
type AnalyticsRequest = FunctionAnalyticsRequest
type AnalyticsResponse = FunctionAnalyticsResponse
type WarningsRequest = FunctionWarningsRequest
type WarningsResponse = FunctionWarningsResponse
type PendingRequest = FunctionsPendingRequest
type PendingResponse = FunctionsPendingResponse
