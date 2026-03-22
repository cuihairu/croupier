package assignment

// AssignmentsListRequest represents a request to list assignments.
type AssignmentsListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	GameId   string `form:"game_id"`
	Env      string `form:"env"`
}

// AssignmentsListResponse represents the canonical list response for assignments.
type AssignmentsListResponse struct {
	Assignments map[string][]string `json:"assignments"`
	Total       int                 `json:"total"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"pageSize"`
}

// AssignmentsHistoryRequest represents a request to list assignment history.
type AssignmentsHistoryRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	GameId   string `form:"game_id"`
	Env      string `form:"env"`
	Action   string `form:"action"`
}

// AssignmentsHistoryResponse represents the canonical history list response.
type AssignmentsHistoryResponse struct {
	Items    []assignmentHistoryEntry `json:"items"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}

// AssignmentsUpdateRequest represents a request to update assignments.
type AssignmentsUpdateRequest struct {
	GameId    string   `json:"game_id"`
	Env       string   `json:"env"`
	Action    string   `json:"action"` // assign/clone/remove
	Functions []string `json:"functions"`
}

// AssignmentsUpdateResponse represents the canonical update response.
type AssignmentsUpdateResponse struct {
	OK          bool                `json:"ok"`
	Unknown     []string            `json:"unknown,omitempty"`
	Assignments map[string][]string `json:"assignments,omitempty"`
}
