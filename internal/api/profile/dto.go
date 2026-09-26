package profile

// ProfileGame 游戏资料
type ProfileGame struct {
	GameId      string      `json:"gameId"`
	GameName    string      `json:"gameName"`
	Color       string      `json:"color"`
	Envs        []string    `json:"envs"`
	EnvMeta     interface{} `json:"envMeta"`
	Permissions []string    `json:"permissions"`
	// AccessLevel 是访问级别：full（持通配，等价全部权限）/ scoped（有显式
	// 权限集）/ none（无任何显式权限）。
	//
	// 此前 Permissions 对所有用户所有游戏恒为 []string{}（字面量），admin 的
	// 「游戏访问权限」因此永远空白且无从解释（docs/BUGS.md BUG-018）。有了
	// AccessLevel，前端才能区分「全部权限」「部分权限」「无显式权限」三种语义。
	AccessLevel string `json:"accessLevel,omitempty"`
	// PermissionScope 说明权限的授权维度，固定为 "role"。
	//
	// RBAC 挂在角色上、不按游戏切分；按游戏/环境切分的是「可见游戏与可见环境」
	// （GetUserGames 已按 admin_game_env_scopes 过滤）。把这个事实下发给前端，
	// 避免用户以为每个游戏可以单独授权、进而把空列表当成 bug。
	PermissionScope string `json:"permissionScope,omitempty"`
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
	// Permissions 是资源 → 操作的真实分组（按字典序）。
	//
	// 此前这里是「一个角色一条、resource 恒为字面量 "role"、actions 塞角色名」
	// 的编造数据，根本不是权限列表（docs/BUGS.md BUG-019）。
	Permissions []ProfilePermission `json:"permissions"`
	Admin       bool                `json:"admin"`
	Roles       []string            `json:"roles"`
	// PermissionIDs 用户真实持有的权限 id，**不含角色名**（同 BUG-019）。
	// 前端用它与权限目录做差集，渲染「已授权 / 未授权」两态。
	PermissionIDs []string `json:"permissionIDs,omitempty"`
	// AccessLevel 访问级别：full / scoped / none。
	AccessLevel string `json:"accessLevel,omitempty"`
	// PermissionScope 授权维度，固定 "role"（RBAC 不按游戏切分）。
	PermissionScope string `json:"permissionScope,omitempty"`
	// FullAccess 持有通配权限（* / admin:all）。为 true 时前端不应把任何
	// 「未授权」条目解读为「该操作真的不可用」。
	FullAccess bool `json:"fullAccess,omitempty"`
	// RolePermissions 是「哪个角色授予了哪些权限 id」，用于渲染
	// 角色 → 资源 → 操作 的权限树。
	//
	// 之前只有并集（PermissionIDs），无法回答「这个操作是哪个角色给的」——
	// 树就只能退化成单层列表。角色未挂任何权限时也会出现（空数组），
	// 否则「有角色但零权限」的账号在树上根本看不到自己有哪些角色。
	RolePermissions []RolePermissionGrant `json:"rolePermissions,omitempty"`
}

// RolePermissionGrant 单个角色的权限授予明细。
type RolePermissionGrant struct {
	// Role 角色名（展示用，保留原大小写）。
	Role string `json:"role"`
	// PermissionIDs 该角色挂上的权限 id（去重排序）。
	PermissionIDs []string `json:"permissionIds"`
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
