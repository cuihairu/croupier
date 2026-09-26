# 配置文件 API

### 1. "获取当前用户资料"

1. route definition

- Url: /api/v1/profile
- Method: GET
- Request: `ProfileGetRequest`
- Response: `ProfileGetResponse`

2. request definition



```go
type ProfileGetRequest struct {
}
```


3. response definition



```go
type ProfileGetResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Avatar string `json:"avatar"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type ProfileInfo struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Avatar string `json:"avatar"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 2. "更新当前用户资料"

1. route definition

- Url: /api/v1/profile
- Method: PUT
- Request: `ProfileUpdateRequest`
- Response: `ProfileGetResponse`

2. request definition



```go
type ProfileUpdateRequest struct {
	Nickname string `json:"nickname,optional"`
	Email string `json:"email,optional"`
	Phone string `json:"phone,optional"`
	Avatar string `json:"avatar,optional"`
}
```


3. response definition



```go
type ProfileGetResponse struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Avatar string `json:"avatar"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type ProfileInfo struct {
	Id int64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Roles []string `json:"roles"`
	Avatar string `json:"avatar"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 3. "获取我的游戏"

1. route definition

- Url: /api/v1/profile/games
- Method: GET
- Request: `ProfileGamesRequest`
- Response: `ProfileGamesResponse`

2. request definition



```go
type ProfileGamesRequest struct {
}
```


3. response definition



```go
type ProfileGamesResponse struct {
	Games []ProfileGame `json:"games"`
}

type ProfileGame struct {
	// ...省略
	Permissions     []string `json:"permissions"`             // full 时为 ["*"]
	AccessLevel     string   `json:"accessLevel,omitempty"`  // full / scoped / none
	PermissionScope string   `json:"permissionScope,omitempty"` // 固定 "role"
}
```

`accessLevel` 让「空权限」成为**可解释状态**：修复前 `Permissions` 被硬编码成
`[]string{}`，对所有用户所有游戏恒为空，admin 的「游戏访问权限」因此永远是一块
空白（docs/BUGS.md BUG-018）。`none` 时前端必须给出说明而不是留白。

### 4. "修改密码"

1. route definition

- Url: /api/v1/profile/password
- Method: PUT
- Request: `ProfilePasswordRequest`
- Response: `-`

2. request definition



```go
type ProfilePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}
```


3. response definition


### 5. "获取当前用户权限"

1. route definition

- Url: /api/v1/profile/permissions
- Method: GET
- Request: `ProfilePermissionsRequest`
- Response: `ProfilePermissionsResponse`

2. request definition



```go
type ProfilePermissionsRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```


3. response definition



```go
type ProfilePermissionsResponse struct {
	Permissions   []ProfilePermission       `json:"permissions"`
	Admin         bool                      `json:"admin"`
	Roles         []string                  `json:"roles"`
	PermissionIDs []string                  `json:"permissionIDs,omitempty"`
	AccessLevel       string `json:"accessLevel,omitempty"`       // full / scoped / none
	PermissionScope   string `json:"permissionScope,omitempty"`   // 固定 "role"
	FullAccess        bool   `json:"fullAccess,omitempty"`
	RolePermissions   []RolePermissionGrant `json:"rolePermissions,omitempty"`
}

type RolePermissionGrant struct {
	Role          string   `json:"role"`
	PermissionIDs []string `json:"permissionIds"`
}
```

字段语义（docs/BUGS.md BUG-019 / BUG-020）：

- `permissions[]` 是**真实的资源 → 操作分组**，`resource` 取自权限 id 的前缀
  （`pages:write` → `resource=pages`），**不是** `permissions` 表的 `resource` 列
  （那一列存 module，38 条目录会塌缩成 7 个值）。
- `permissionIDs` 只含**权限 id**，不含角色名。角色名只在 `roles` 里。
- `permissionScope` 恒为 `role`：**RBAC 挂在角色上、不按游戏维度切分**；按游戏/
  环境切分的是「可见游戏与可见环境」（`admin_game_env_scopes`）。
- `fullAccess` 为 true 当且仅当持有**两个维度都通配**的权限（`*` / `admin:all`）。
  `user:*` 只是「user 资源的全部操作」，不算 `fullAccess`。
- `rolePermissions` 给出逐角色明细，供前端渲染「角色 → 资源 → 操作」的权限树；
  角色未挂任何权限时也会出现（空数组）。

