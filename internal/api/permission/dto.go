package permission

// Permission represents a permission with its attributes
type Permission struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Category    string `json:"category"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// PendingFunction represents a function that is pending approval
type PendingFunction struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Requester string `json:"requester"`
	CreatedAt string `json:"createdAt"`
}

// PermissionDetailRequest is the request for getting a permission by ID
type PermissionDetailRequest struct {
	ID string `uri:"id"`
}

// PermissionDetailResponse is the response for getting a permission by ID
type PermissionDetailResponse struct {
	Permission
}

// PermissionsListRequest is the request for listing permissions with filters
type PermissionsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Category string `form:"category"`
	Resource string `form:"resource"`
}

// PermissionsListResponse is the response for listing permissions
type PermissionsListResponse struct {
	Items []Permission `json:"items"`
	Total int64        `json:"total"`
	Page  int          `json:"page"`
	Size  int          `json:"pageSize"`
}
