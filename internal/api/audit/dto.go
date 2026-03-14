package audit

// AuditRequest audit log request
type AuditRequest struct {
	Page     int    `form:"page" json:"page"`         // 页码
	PageSize int    `form:"pageSize" json:"pageSize"` // 每页数量
	Size     int    `form:"size" json:"size"`         // 每页数量别名（兼容前端）
	Action   string `form:"action" json:"action"`     // 操作类型过滤
	UserID   string `form:"userId" json:"userId"`     // 用户ID过滤
}

// AuditResponse audit log response
type AuditResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
