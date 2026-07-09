package function

// Local types for function logic - previously from internal/types

// Function represents a function in the system
type Function struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	GameId      string      `json:"gameId"`
	Status      int         `json:"status"`
	Version     string      `json:"version"`
	Instances   int         `json:"instances"`
	Category    string      `json:"category"`
	SpecFormat  string      `json:"specFormat"`
	OpenAPISpec interface{} `json:"openapiSpec"`
}

// FunctionsListRequest represents a request to list functions
type FunctionsListRequest struct {
	GameId   string `json:"gameId"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Status   int    `json:"status"`
	Category string `json:"category"`
}

// FunctionsListResponse represents the response for listing functions
type FunctionsListResponse struct {
	Items []Function `json:"items"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

// FunctionDetailRequest represents a request to get function details
type FunctionDetailRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionDetailResponse represents the response for function details
type FunctionDetailResponse struct {
	Function    Function              `json:"function"`
	Descriptor  FunctionDescriptor    `json:"descriptor,omitempty"`
	Instances   []FunctionInstance    `json:"instances,omitempty"`
	Permissions []FunctionPermission  `json:"permissions,omitempty"`
	Routes      []FunctionRouteConfig `json:"routes,omitempty"`
}

// FunctionDescriptor represents a function descriptor
type FunctionDescriptor struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Spec        interface{} `json:"spec"`
	Schema      interface{} `json:"schema"`
	Input       interface{} `json:"input"`
	Output      interface{} `json:"output"`
}

// FunctionInstance represents a function instance
type FunctionInstance struct {
	AgentId   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Status    string `json:"status"`
}

// FunctionPermission represents a function permission
type FunctionPermission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Roles    []string `json:"roles"`
}

// FunctionRouteConfig represents a function route configuration
type FunctionRouteConfig struct {
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Enabled     bool              `json:"enabled"`
	RateLimit   int               `json:"rateLimit"`
	Timeout     int               `json:"timeout"`
	Middlewares []string          `json:"middlewares"`
	Metadata    map[string]string `json:"metadata"`
	Nodes       []string          `json:"nodes"`
	Order       int               `json:"order"`
	Hidden      bool              `json:"hidden"`
}

// FunctionAnalyticsRequest represents a request for function analytics
type FunctionAnalyticsRequest struct {
	ID        string `json:"id" binding:"required"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// FunctionAnalyticsResponse represents the response for function analytics
type FunctionAnalyticsResponse struct {
	Calls          int64            `json:"calls"`
	Errors         int64            `json:"errors"`
	AvgLatency     float64          `json:"avgLatency"`
	P95Latency     float64          `json:"p95Latency"`
	P99Latency     float64          `json:"p99Latency"`
	SuccessRate    float64          `json:"successRate"`
	TotalCalls     int64            `json:"totalCalls"`
	CallsToday     int64            `json:"callsToday"`
	CallsThisWeek  int64            `json:"callsThisWeek"`
	CallsThisMonth int64            `json:"callsThisMonth"`
	Timeline       []AnalyticsPoint `json:"timeline,omitempty"`
}

// AnalyticsPoint represents a single analytics data point
type AnalyticsPoint struct {
	Timestamp  string  `json:"timestamp"`
	Calls      int64   `json:"calls"`
	Errors     int64   `json:"errors"`
	AvgLatency float64 `json:"avgLatency"`
}

// FunctionCopyRequest represents a request to copy a function
type FunctionCopyRequest struct {
	ID          string `json:"id" binding:"required"`
	NewId       string `json:"newId" binding:"required"`
	NewName     string `json:"newName"`
	Destination string `json:"destination"`
}

// FunctionCopyResponse represents the response for copying a function
type FunctionCopyResponse struct {
	FunctionId string `json:"functionId"`
	NewId      string `json:"newId"`
	Copied     bool   `json:"copied"`
}

// FunctionActionRequest represents a request for function actions (delete, enable, disable)
type FunctionActionRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionHistoryRequest represents a request for function history
type FunctionHistoryRequest struct {
	ID     string `json:"id" binding:"required"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// FunctionHistoryItem represents a single function history item
type FunctionHistoryItem struct {
	ID        string      `json:"id"`
	Timestamp string      `json:"timestamp"`
	Action    string      `json:"action"`
	User      string      `json:"user"`
	Operator  string      `json:"operator"`
	Details   interface{} `json:"details"`
}

// FunctionPermissionsRequest represents a request to get function permissions
type FunctionPermissionsRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionPermissionsResponse represents the response for function permissions
type FunctionPermissionsResponse struct {
	Permissions []FunctionPermission `json:"permissions"`
	Items       []FunctionPermission `json:"items"`
}

// FunctionPermissionsUpdateRequest represents a request to update function permissions
type FunctionPermissionsUpdateRequest struct {
	ID          string               `json:"id" binding:"required"`
	Permissions []FunctionPermission `json:"permissions" binding:"required"`
}

// FunctionPublishRequest represents a request to publish a function
type FunctionPublishRequest struct {
	ID          string `json:"id" binding:"required"`
	Version     string `json:"version"`
	Destination string `json:"destination"`
}

// FunctionPublishResponse represents the response for publishing a function
type FunctionPublishResponse struct {
	Published   bool   `json:"published"`
	Version     string `json:"version"`
	Destination string `json:"destination"`
}

// FunctionRouteRequest represents a request to get function routes
type FunctionRouteRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionRouteResponse represents the response for function routes
type FunctionRouteResponse struct {
	Routes []FunctionRouteConfig `json:"routes"`
	Menu   interface{}           `json:"menu"`
	Source string                `json:"source"`
}

// FunctionRouteUpdateRequest represents a request to update function routes
type FunctionRouteUpdateRequest struct {
	ID     string                `json:"id" binding:"required"`
	Routes []FunctionRouteConfig `json:"routes" binding:"required"`
	Nodes  []string              `json:"nodes"`
	Path   string                `json:"path"`
	Order  int                   `json:"order"`
	Hidden bool                  `json:"hidden"`
}

// FunctionInstancesRequest represents a request to get function instances
type FunctionInstancesRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionInvokeRequest represents a request to invoke a function
type FunctionInvokeRequest struct {
	ID              string                 `json:"id" binding:"required"`
	Payload         interface{}            `json:"payload"`
	PayloadBytes    []byte                 `json:"-"`
	Metadata        map[string]string      `json:"metadata"`
	GameID          string                 `json:"gameId"`
	Env             string                 `json:"env"`
	Params          map[string]interface{} `json:"params"`
	Mode            string                 `json:"mode"`
	Route           string                 `json:"route"`
	TargetServiceID string                 `json:"targetServiceId"`
	HashKey         string                 `json:"hashKey"`
}

// FunctionInvokeResponse represents the response for function invocation
type FunctionInvokeResponse struct {
	Result    interface{} `json:"result"`
	Error     string      `json:"error,omitempty"`
	Duration  int64       `json:"duration"`
	Timestamp string      `json:"timestamp"`
	TaskId    string      `json:"taskId"`
	TaskID    string      `json:"taskID"`
	// Broadcast carries per-agent outcomes when route=broadcast. The legacy
	// Result field is also populated with the first successful response so
	// existing callers keep working without reading Broadcast.
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
	AgentID string      `json:"agentId"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// FunctionUIRequest represents a request for function UI
type FunctionUIRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionUIResponse represents the response for function UI
type FunctionUIResponse struct {
	UI             interface{} `json:"ui"`
	Active         bool        `json:"active"`
	Schema         interface{} `json:"schema"`
	Layout         interface{} `json:"layout"`
	Components     interface{} `json:"components"`
	Custom         interface{} `json:"custom"`
	HasDefault     bool        `json:"hasDefault"`
	UISource       string      `json:"uiSource"`
	UISourceDetail interface{} `json:"uiSourceDetail"`
}

// FunctionUIUpdateRequest represents a request to update function UI
type FunctionUIUpdateRequest struct {
	ID         string      `json:"id" binding:"required"`
	UI         interface{} `json:"ui" binding:"required"`
	Active     bool        `json:"active"`
	Schema     interface{} `json:"schema"`
	Layout     interface{} `json:"layout"`
	Components interface{} `json:"components"`
}

// FunctionUIHistoryRequest represents a request for function UI history
type FunctionUIHistoryRequest struct {
	ID     string `json:"id" binding:"required"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// FunctionUIHistoryResponse represents the response for function UI history
type FunctionUIHistoryResponse struct {
	Items []FunctionUIHistoryItem `json:"items"`
	Total int64                   `json:"total"`
}

// FunctionUIHistoryItem represents a single function UI history item
type FunctionUIHistoryItem struct {
	ID         string      `json:"id"`
	Timestamp  string      `json:"timestamp"`
	User       string      `json:"user"`
	UI         interface{} `json:"ui"`
	Active     bool        `json:"active"`
	Version    int         `json:"version"`
	Message    string      `json:"message"`
	CreatedBy  string      `json:"createdBy"`
	CreatedAt  string      `json:"createdAt"`
	Schema     interface{} `json:"schema"`
	Layout     interface{} `json:"layout"`
	Components interface{} `json:"components"`
}

// FunctionUIRollbackRequest represents a request to rollback function UI
type FunctionUIRollbackRequest struct {
	ID        string `json:"id" binding:"required"`
	HistoryId string `json:"historyId" binding:"required"`
	Version   int    `json:"version"`
}

// FunctionUIRollbackResponse represents the response for function UI rollback
type FunctionUIRollbackResponse struct {
	RolledBack     bool        `json:"rolledBack"`
	UI             interface{} `json:"ui"`
	AppliedVersion int         `json:"appliedVersion"`
	Current        interface{} `json:"current"`
}

// FunctionsPendingRequest represents a request to get pending functions
type FunctionsPendingRequest struct {
	GameId string `json:"gameId"`
}

// FunctionsPendingResponse represents the response for pending functions
type FunctionsPendingResponse struct {
	Functions []PendingFunction `json:"functions"`
	Total     int               `json:"total"`
	Items     []PendingFunction `json:"items"`
}

// PendingFunction represents a pending function
type PendingFunction struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	GameId    string `json:"gameId"`
	Status    string `json:"status"`
	Since     string `json:"since"`
	Requester string `json:"requester"`
	CreatedAt string `json:"createdAt"`
}

// FunctionWarningsRequest represents a request for function warnings
type FunctionWarningsRequest struct {
	ID         string `json:"id" binding:"required"`
	FunctionID string `json:"functionId"`
	AgentID    string `json:"agentId"`
	Code       string `json:"code"`
	Limit      int    `json:"limit"`
}

// FunctionWarningsResponse represents the response for function warnings
type FunctionWarningsResponse struct {
	Warnings []FunctionWarningItem `json:"warnings"`
	Items    []FunctionWarningItem `json:"items"`
}

// FunctionWarningItem represents a function warning
type FunctionWarningItem struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Level      string `json:"level"`
	Key        string `json:"key"`
	AgentID    string `json:"agentId"`
	FunctionID string `json:"functionId"`
	Version    string `json:"version"`
	Code       string `json:"code"`
	Count      int    `json:"count"`
	FirstSeen  string `json:"firstSeen"`
	LastSeen   string `json:"lastSeen"`
}

// DescriptorsRequest represents a request to get descriptors
type DescriptorsRequest struct {
	GameId   string `json:"gameId"`
	Category string `json:"category"`
	Type     string `json:"type"`
}

// BatchCopyFunctionsRequest represents a request to batch copy functions
type BatchCopyFunctionsRequest struct {
	FunctionIds []string `json:"functionIds" binding:"required"`
}

// BatchCopyFunctionsResponse represents the response for batch copying functions
type BatchCopyFunctionsResponse struct {
	Updated int      `json:"updated"`
	Failed  []string `json:"failed"`
	Copied  []string `json:"copied"`
}

// BatchUpdateFunctionsRequest represents a request to batch update functions
type BatchUpdateFunctionsRequest struct {
	FunctionIds []string `json:"functionIds" binding:"required"`
	Enabled     bool     `json:"enabled"`
}

// BatchUpdateFunctionsResponse represents the response for batch updating functions
type BatchUpdateFunctionsResponse struct {
	Updated int      `json:"updated"`
	Failed  []string `json:"failed"`
}

// BatchDeleteFunctionsRequest represents a request to batch delete functions
type BatchDeleteFunctionsRequest struct {
	FunctionIds []string `json:"functionIds" binding:"required"`
}

// BatchDeleteFunctionsResponse represents the response for batch deleting functions
type BatchDeleteFunctionsResponse struct {
	Updated int      `json:"updated"`
	Failed  []string `json:"failed"`
}

// JSONMap represents a generic JSON map
type JSONMap map[string]interface{}
