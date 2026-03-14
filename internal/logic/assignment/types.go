package assignment

// Assignment related types

type AssignmentsHistoryRequest struct {
	GameId   string `json:"gameId"`
	Env      string `json:"env"`
	Action   string `json:"action"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type AssignmentsHistoryResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type AssignmentsListRequest struct {
	GameId   string `json:"gameId"`
	Env      string `json:"env"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type AssignmentsListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type AssignmentsUpdateRequest struct {
	GameId    string      `json:"gameId"`
	Env       string      `json:"env"`
	Action    string      `json:"action"`
	Config    interface{} `json:"config"`
	Functions []string    `json:"functions"`
}

type AssignmentsUpdateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
