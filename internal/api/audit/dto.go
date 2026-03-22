package audit

// AuditRequest is the query/body contract for listing audit events.
type AuditRequest struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	Size     int    `form:"size" json:"size"`
	Action   string `form:"action" json:"action"`
	UserID   string `form:"userId" json:"userId"`
	Actor    string `form:"actor" json:"actor"`
	Kind     string `form:"kind" json:"kind"`
	Kinds    string `form:"kinds" json:"kinds"`
	GameID   string `form:"gameId" json:"gameId"`
	Env      string `form:"env" json:"env"`
	IP       string `form:"ip" json:"ip"`
	Start    string `form:"start" json:"start"`
	End      string `form:"end" json:"end"`
}

// AuditItem is the canonical REST response item for audit events.
type AuditItem struct {
	ID        string                 `json:"id"`
	Action    string                 `json:"action"`
	UserID    string                 `json:"userId"`
	GameID    string                 `json:"gameId,omitempty"`
	Env       string                 `json:"env,omitempty"`
	Target    string                 `json:"target,omitempty"`
	Result    string                 `json:"result,omitempty"`
	TraceID   string                 `json:"traceId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"createdAt"`
}

// AuditListResponse is the canonical REST list envelope for audit events.
type AuditListResponse struct {
	Items    []AuditItem `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}
