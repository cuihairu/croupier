package profile

// ProfileGame 游戏资料
type ProfileGame struct {
	GameId      string      `json:"gameId"`
	GameName    string      `json:"gameName"`
	Color       string      `json:"color"`
	Envs        []string    `json:"envs"`
	EnvMeta     interface{} `json:"envMeta"`
	Permissions []string    `json:"permissions"`
}

// ProfileGamesRequest 获取我的游戏请求
type ProfileGamesRequest struct {
}

// ProfileGamesResponse 获取我的游戏响应
type ProfileGamesResponse struct {
	Games []ProfileGame `json:"games"`
}

// ProfileGetRequest 获取个人资料请求
type ProfileGetRequest struct {
}

// ProfileGetResponse 获取个人资料响应
type ProfileGetResponse struct {
	ProfileInfo
}

// ProfileInfo 个人资料信息
type ProfileInfo struct {
	Id          int64    `json:"id"`
	Username    string   `json:"username"`
	Nickname    string   `json:"nickname"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Active      bool     `json:"active"`
	Roles       []string `json:"roles"`
	Avatar      string   `json:"avatar"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	LastLoginAt string   `json:"lastLoginAt,omitempty"`
}

// ProfilePasswordRequest 修改密码请求
type ProfilePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// ProfilePermission 权限
type ProfilePermission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	GameId   string   `json:"gameId,omitempty"`
	Env      string   `json:"env,omitempty"`
}

// ProfilePermissionsRequest 获取权限列表请求
type ProfilePermissionsRequest struct {
	GameId string `form:"gameId"`
	Env    string `form:"env"`
}

// ProfilePermissionsResponse 获取权限列表响应
type ProfilePermissionsResponse struct {
	Permissions   []ProfilePermission `json:"permissions"`
	Admin         bool                `json:"admin"`
	Roles         []string            `json:"roles"`
	PermissionIDs []string            `json:"permissionIDs,omitempty"`
}

// ProfileUpdateRequest 更新个人资料请求。
//
// 全部使用指针 + omitempty：指针为 nil 表示「请求未携带该字段」，应保留库里的
// 现有值。此前用裸 string 时，Go 绑定 JSON 会把缺失字段解成空串，再被 service
// 无条件写库——于是「只改昵称」会把头像/邮箱/手机清空，「只改头像」会把昵称/
// 邮箱/手机清空（docs/BUGS.md BUG-012）。docs/api/profile.md 早已按部分更新语义
// 描述该接口，代码却没实现。
type ProfileUpdateRequest struct {
	Nickname *string `json:"nickname"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	Avatar   *string `json:"avatar"`
}

// ProfileUpdateResponse 更新个人资料响应
type ProfileUpdateResponse struct {
	Ok bool `json:"ok"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// ChangePasswordResponse 修改密码响应
type ChangePasswordResponse struct {
	Ok bool `json:"ok"`
}

// GameInfo 游戏信息
type GameInfo struct {
	GameID   string `json:"gameId"`
	GameName string `json:"gameName"`
}

// ScopeUpdateRequest 更新当前游戏/环境选择请求
type ScopeUpdateRequest struct {
	GameID string `json:"gameId" binding:"required"`
	Env    string `json:"env" binding:"required"`
}

// ScopeUpdateResponse 更新当前游戏/环境选择响应
type ScopeUpdateResponse struct {
	Ok bool `json:"ok"`
}
