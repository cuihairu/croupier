// Package admin provides DTOs for admin API operations.
package admin

// Admin represents an admin user.
type Admin struct {
	Id        int64    `json:"id"`
	Username  string   `json:"username"`
	Nickname  string   `json:"nickname"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	Roles     []string `json:"roles"`
	Status    int      `json:"status"` // 1:active 0:disabled
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// AdminCreateRequest represents the request to create an admin.
type AdminCreateRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Nickname string   `json:"nickname"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Roles    []string `json:"roles"`
}

// AdminCreateResponse represents the response after creating an admin.
type AdminCreateResponse struct {
	Admin
}

// AdminDeleteRequest represents the request to delete an admin.
type AdminDeleteRequest struct {
	ID string `uri:"id"`
}

// AdminDeleteResponse represents the response after deleting an admin.
type AdminDeleteResponse struct {
	Message string `json:"message"`
}

// AdminDetailRequest represents the request to get admin details.
type AdminDetailRequest struct {
	ID string `uri:"id"`
}

// AdminDetailResponse represents the response with admin details.
type AdminDetailResponse struct {
	Admin
}

// AdminGame represents a game scope for an admin.
type AdminGame struct {
	GameId   string   `json:"gameId"`
	GameName string   `json:"gameName"`
	Envs     []string `json:"envs"`
}

// AdminGamesRequest represents the request to get admin game scopes.
type AdminGamesRequest struct {
	ID string `uri:"id"`
}

// AdminGamesResponse represents the response with admin game scopes.
type AdminGamesResponse struct {
	Games []AdminGame `json:"games"`
}

// AdminGamesUpdateRequest represents the request to update admin game scopes.
type AdminGamesUpdateRequest struct {
	ID    string      `uri:"id"`
	Games []AdminGame `json:"games"`
}

// AdminPasswordResetRequest represents the request to reset admin password.
type AdminPasswordResetRequest struct {
	ID          string `uri:"id"`
	NewPassword string `json:"newPassword"`
}

// AdminPasswordResetResponse represents the response after password reset.
type AdminPasswordResetResponse struct {
	Message string `json:"message"`
}

// AdminUpdateRequest represents the request to update an admin.
type AdminUpdateRequest struct {
	ID       string   `uri:"id"`
	Nickname string   `json:"nickname"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Roles    []string `json:"roles"`
	Status   int      `json:"status"`
}

// AdminUpdateResponse represents the response after updating an admin.
type AdminUpdateResponse struct {
	Admin
}

// AdminsListRequest represents the request to list admins.
type AdminsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Search   string `form:"search"`
	Role     string `form:"role"`
	Status   int    `form:"status"`
}

// AdminsListResponse represents the response with a list of admins.
type AdminsListResponse struct {
	Items []Admin `json:"items"`
	Total int64   `json:"total"`
	Page  int     `json:"page"`
	Size  int     `json:"pageSize"`
}

// Permission represents a permission entry.
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

// PermissionDetailRequest represents the request to get permission details.
type PermissionDetailRequest struct {
	ID string `uri:"id"`
}

// PermissionDetailResponse represents the response with permission details.
type PermissionDetailResponse struct {
	Permission
}

// PermissionsListRequest represents the request to list permissions.
type PermissionsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Category string `form:"category"`
	Resource string `form:"resource"`
}

// PermissionsListResponse represents the response with a list of permissions.
type PermissionsListResponse struct {
	Items []Permission `json:"items"`
	Total int64        `json:"total"`
	Page  int          `json:"page"`
	Size  int          `json:"pageSize"`
}

// Type aliases for backward compatibility with existing code.
type ListRequest = AdminsListRequest
type ListResponse = AdminsListResponse
type CreateRequest = AdminCreateRequest
type CreateResponse = AdminCreateResponse
type GetRequest = AdminDetailRequest
type GetResponse = AdminDetailResponse
type UpdateRequest = AdminUpdateRequest
type UpdateResponse = AdminUpdateResponse
type DeleteRequest = AdminDeleteRequest
type PasswordResetRequest = AdminPasswordResetRequest
type GetGamesRequest = AdminGamesRequest
type GetGamesResponse = AdminGamesResponse
type UpdateGamesRequest = AdminGamesUpdateRequest
