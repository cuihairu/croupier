package audit

// AuditRequest audit log request
type AuditRequest struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	Size     int    `form:"size" json:"size"`
	Action   string `form:"action" json:"action"`
	UserID   string `form:"userId" json:"userId"`
	Actor    string `form:"actor" json:"actor"`
	Kind     string `form:"kind" json:"kind"`
	Kinds    string `form:"kinds" json:"kinds"`
	GameID   string `form:"game_id" json:"game_id"`
	Env      string `form:"env" json:"env"`
	IP       string `form:"ip" json:"ip"`
	Start    string `form:"start" json:"start"`
	End      string `form:"end" json:"end"`
}

// AuditResponse audit log response
type AuditResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
