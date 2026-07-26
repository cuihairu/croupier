package function

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// Local types for function logic - previously from internal/types

// Function represents a function in the system
type Function struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	GameId      string          `json:"gameId"`
	Status      int             `json:"status"`
	Version     string          `json:"version"`
	Instances   int             `json:"instances"`
	Resource    string          `json:"resource"`
	SpecFormat  string          `json:"specFormat"`
	OpenAPISpec json.RawMessage `json:"openapiSpec,omitempty"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

// FunctionsListRequest represents a request to list functions
type FunctionsListRequest struct {
	GameId   string `json:"gameId"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Status   int    `json:"status"`
	Resource string `json:"resource"`
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
	Function    Function             `json:"function"`
	Descriptor  FunctionDescriptor   `json:"descriptor,omitempty"`
	Instances   []FunctionInstance   `json:"instances,omitempty"`
	Permissions []FunctionPermission `json:"permissions,omitempty"`
}

// FunctionDescriptor represents a function descriptor
type FunctionDescriptor struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Spec        json.RawMessage `json:"spec,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
}

// FunctionInstance represents a function instance
type FunctionInstance struct {
	AgentId   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

// FunctionPermission represents a function permission
type FunctionPermission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Roles    []string `json:"roles"`
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
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	Action    string          `json:"action"`
	User      string          `json:"user"`
	Operator  string          `json:"operator"`
	Details   json.RawMessage `json:"details,omitempty"`
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

// FunctionInstancesRequest represents a request to get function instances
type FunctionInstancesRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionInstancesResponse represents function instances for one function.
type FunctionInstancesResponse struct {
	Items     []FunctionInstance        `json:"items,omitempty"`
	Instances []RuntimeFunctionInstance `json:"instances,omitempty"`
}

// FunctionInstancesAllResponse represents all runtime function instances.
type FunctionInstancesAllResponse struct {
	Instances []RuntimeFunctionInstance `json:"instances"`
}

// RuntimeFunctionInstance captures one function registration on a runtime provider.
type RuntimeFunctionInstance struct {
	FunctionID string `json:"function_id"`
	AgentID    string `json:"agent_id"`
	ProviderID string `json:"provider_id,omitempty"`
	Addr       string `json:"addr,omitempty"`
	Version    string `json:"version,omitempty"`
	LastSeen   string `json:"last_seen,omitempty"`
	Healthy    bool   `json:"healthy"`
	GameID     string `json:"game_id,omitempty"`
	Env        string `json:"env,omitempty"`
}

// FunctionInvokeRequest represents a request to invoke a function
type FunctionInvokeRequest struct {
	ID              string            `json:"id" binding:"required"`
	Payload         json.RawMessage   `json:"payload,omitempty"`
	Params          json.RawMessage   `json:"params,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	GameID          string            `json:"gameId"`
	Env             string            `json:"env"`
	Mode            string            `json:"mode"`
	Route           string            `json:"route"`
	TargetServiceID string            `json:"targetServiceId"`
	HashKey         string            `json:"hashKey"`
}

// FunctionInvokeResponse represents the response for function invocation
type FunctionInvokeResponse struct {
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Duration  int64           `json:"duration"`
	Timestamp string          `json:"timestamp"`
	TaskId    string          `json:"taskId"`
	TaskID    string          `json:"taskID"`
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
	AgentID string          `json:"agentId"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// FunctionUIRequest represents a request for function UI
type FunctionUIRequest struct {
	ID string `json:"id" binding:"required"`
}

// FunctionUIResponse represents the response for function UI
type FunctionUIResponse struct {
	Schema         json.RawMessage `json:"schema,omitempty"`
	Custom         bool            `json:"custom"`
	HasDefault     bool            `json:"hasDefault"`
	UISource       string          `json:"uiSource"`
	UISourceDetail string          `json:"uiSourceDetail"`
}

// FunctionUIUpdateRequest represents a request to update function UI
type FunctionUIUpdateRequest struct {
	ID     string          `json:"id" binding:"required"`
	Schema json.RawMessage `json:"schema" binding:"required"`
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
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	User      string          `json:"user"`
	UI        json.RawMessage `json:"ui,omitempty"`
	Active    bool            `json:"active"`
	Version   int             `json:"version"`
	Message   string          `json:"message"`
	CreatedBy string          `json:"createdBy"`
	CreatedAt string          `json:"createdAt"`
	Schema    json.RawMessage `json:"schema,omitempty"`
}

// FunctionUIRollbackRequest represents a request to rollback function UI
type FunctionUIRollbackRequest struct {
	ID        string `json:"id" binding:"required"`
	HistoryId string `json:"historyId" binding:"required"`
	Version   int    `json:"version"`
}

// FunctionUIRollbackResponse represents the response for function UI rollback
type FunctionUIRollbackResponse struct {
	RolledBack     bool                `json:"rolledBack"`
	UI             json.RawMessage     `json:"ui,omitempty"`
	AppliedVersion int                 `json:"appliedVersion"`
	Current        *FunctionUIResponse `json:"current,omitempty"`
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
	Resource string `json:"resource"`
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

// DescriptorsV2Result represents the result of DescriptorsV2
type DescriptorsV2Result struct {
	Functions []spec.FunctionSpec `json:"functions"`
}
