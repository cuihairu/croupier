# 配置文件 API

### 1. "获取当前用户资料"

1. route definition

- Url: /api/v1/profile
- Method: GET
- Request: `ProfileGetRequest`
- Response: `ProfileGetResponse`

2. request definition



```golang
type ProfileGetRequest struct {
}
```


3. response definition



```golang
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



```golang
type ProfileUpdateRequest struct {
	Nickname string `json:"nickname,optional"`
	Email string `json:"email,optional"`
	Phone string `json:"phone,optional"`
	Avatar string `json:"avatar,optional"`
}
```


3. response definition



```golang
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



```golang
type ProfileGamesRequest struct {
}
```


3. response definition



```golang
type ProfileGamesResponse struct {
	Games []ProfileGame `json:"games"`
}
```

### 4. "修改密码"

1. route definition

- Url: /api/v1/profile/password
- Method: PUT
- Request: `ProfilePasswordRequest`
- Response: `-`

2. request definition



```golang
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



```golang
type ProfilePermissionsRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```


3. response definition



```golang
type ProfilePermissionsResponse struct {
	Permissions []ProfilePermission `json:"permissions"`
	Admin bool `json:"admin"`
	Roles []string `json:"roles"`
	PermissionIDs []string `json:"permissionIDs,omitempty"`
}
```

