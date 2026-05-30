# 认证 API

### 1. "用户登录"

1. route definition

- Url: /api/v1/auth/login
- Method: POST
- Request: `LoginRequest`
- Response: `LoginResponse`

2. request definition



```go
type LoginRequest struct {
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
}
```


3. response definition



```go
type LoginResponse struct {
	Token string `json:"token"`
	User UserInfo `json:"user"`
}

type UserInfo struct {
	Username string `json:"username"`
	Roles []string `json:"roles"`
	Nickname string `json:"nickname,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}
```

### 2. "用户登出"

1. route definition

- Url: /api/v1/auth/logout
- Method: POST
- Request: `LogoutRequest`
- Response: `LogoutResponse`

2. request definition



```go
type LogoutRequest struct {
}
```


3. response definition



```go
type LogoutResponse struct {
}
```

### 3. "权限校验"

1. route definition

- Url: /api/v1/auth/check
- Method: POST
- Auth: Bearer Token
- Request: `CheckRequest`
- Response: `CheckResponse`

2. request definition

```go
type CheckRequest struct {
	Resource string `json:"resource"` // 例如 roles、games、functions
	Action   string `json:"action"`   // 例如 read、write、manage
	GameID   string `json:"gameId,omitempty"`
	Env      string `json:"env,omitempty"`
}
```

3. response definition

```go
type CheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}
```

### 4. "批量权限校验"

1. route definition

- Url: /api/v1/auth/check/batch
- Method: POST
- Auth: Bearer Token
- Request: `BatchCheckRequest`
- Response: `BatchCheckResponse`

2. request definition

```go
type BatchCheckRequest struct {
	Checks []CheckRequest `json:"checks"`
}
```

3. response definition

```go
type BatchCheckResponse struct {
	Results []CheckResponse `json:"results"`
}
```

### 说明

- 校验基于当前登录用户在数据库中的角色和权限，不依赖前端缓存。
- `gameId` 与 `env` 字段当前保留用于兼容前端请求结构，现阶段权限判断主要按资源和动作执行。

