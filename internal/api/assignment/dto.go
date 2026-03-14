package assignment

// AssignmentsListRequest represents a request to list assignments
type AssignmentsListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	GameId   string `form:"game_id"`
	Env      string `form:"env"`
}

// AssignmentsListResponse represents the response for listing assignments
type AssignmentsListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// AssignmentsHistoryRequest represents a request to list assignment history
type AssignmentsHistoryRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	GameId   string `form:"game_id"`
	Env      string `form:"env"`
	Action   string `form:"action"`
}

// AssignmentsHistoryResponse represents the response for listing assignment history
type AssignmentsHistoryResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// AssignmentsUpdateRequest represents a request to update assignments
type AssignmentsUpdateRequest struct {
	GameId    string   `json:"game_id"`
	Env       string   `json:"env"`
	Action    string   `json:"action"` // assign/clone/remove
	Functions []string `json:"functions"`
}

// AssignmentsUpdateResponse represents the response for updating assignments
type AssignmentsUpdateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
