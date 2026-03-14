package role

// Role represents an RBAC role with its permissions.
type Role struct {
	Id          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// RoleCreateRequest defines the request body for creating a role.
type RoleCreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Permissions []string `json:"permissions"`
}

// RoleCreateResponse defines the response body for role creation.
type RoleCreateResponse struct {
	Role
}

// RoleDeleteRequest defines the request parameters for deleting a role.
type RoleDeleteRequest struct {
	ID string `uri:"id"`
}

// RoleDetailRequest defines the request parameters for getting role details.
type RoleDetailRequest struct {
	ID string `uri:"id"`
}

// RoleDetailResponse defines the response body for role details.
type RoleDetailResponse struct {
	Role
}

// RoleUpdateRequest defines the request body for updating a role.
type RoleUpdateRequest struct {
	ID          string   `uri:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Permissions []string `json:"permissions"`
}

// RoleUpdateResponse defines the response body for role update.
type RoleUpdateResponse struct {
	Role
}

// RolesListRequest defines the query parameters for listing roles.
type RolesListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Category string `form:"category"`
	Search   string `form:"search"`
}

// RolesListResponse defines the response body for listing roles.
type RolesListResponse struct {
	Items []Role `json:"items"`
	Total int64  `json:"total"`
	Page  int    `json:"page"`
	Size  int    `json:"pageSize"`
}
