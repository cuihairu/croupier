package alert

// Alert represents an alert
type Alert struct {
	Id        string      `json:"id"`
	Type      string      `json:"type"`
	Level     string      `json:"level"`
	Message   string      `json:"message"`
	Source    string      `json:"source"`
	Status    string      `json:"status"`
	Details   interface{} `json:"details"`
	CreatedAt string      `json:"createdAt"`
}

// Silence represents a silence rule
type Silence struct {
	Id        string      `json:"id"`
	AlertType string      `json:"alertType"`
	Matchers  interface{} `json:"matchers"`
	StartAt   string      `json:"startAt"`
	EndAt     string      `json:"endAt"`
	CreatedBy string      `json:"createdBy"`
}

// AlertsListRequest represents the request to list alerts
type AlertsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Level    string `form:"level"`
	Status   string `form:"status"`
}

// AlertsListResponse represents the response with a list of alerts
type AlertsListResponse struct {
	Items []Alert `json:"items"`
	Total int64   `json:"total"`
	Page  int     `json:"page"`
	Size  int     `json:"pageSize"`
}

// AlertSilenceRequest represents the request to silence an alert
type AlertSilenceRequest struct {
	ID       string `uri:"id"`
	Duration int    `json:"duration"` // 分钟
	Reason   string `json:"reason"`
}

// SilencesListRequest represents the request to list silences
type SilencesListRequest struct{}

// SilencesListResponse represents the response with a list of silences
type SilencesListResponse struct {
	Items []Silence `json:"items"`
}

// SilenceDeleteRequest represents the request to delete a silence
type SilenceDeleteRequest struct {
	ID string `uri:"id"`
}
