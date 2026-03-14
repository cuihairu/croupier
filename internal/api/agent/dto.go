package agent

// GetAnalyticsFiltersRequest represents the request to get analytics filters
type GetAnalyticsFiltersRequest struct{}

// AnalyticsFilter represents a single analytics filter
type AnalyticsFilter struct {
	GameId  string      `json:"gameId"`
	Filters interface{} `json:"filters"`
}

// GetAnalyticsFiltersResponse represents the response with analytics filters
type GetAnalyticsFiltersResponse struct {
	Items []AnalyticsFilter `json:"items"`
	Count int               `json:"count"`
}

// UpdateMetaRequest represents the request to update agent metadata
type UpdateMetaRequest struct{}

// AgentSnapshot represents a single agent's metadata snapshot
type AgentSnapshot map[string]interface{}

// UpdateMetaResponse represents the response with agent metadata
type UpdateMetaResponse struct {
	Agents    []AgentSnapshot `json:"agents"`
	Count     int             `json:"count"`
	Timestamp string          `json:"timestamp"`
}

// AgentAnalyticsFiltersResponse represents the response for agent analytics filters
type AgentAnalyticsFiltersResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// AgentMetaReportRequest represents the request for agent meta report
type AgentMetaReportRequest struct {
}

// AgentMetaResponse represents the response for agent metadata
type AgentMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
