### 1. "用户登录"

1. route definition

- Url: /api/v1/auth/login
- Method: POST
- Request: `LoginRequest`
- Response: `LoginResponse`

2. request definition



```golang
type LoginRequest struct {
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
}
```


3. response definition



```golang
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



```golang
type LogoutRequest struct {
}
```


3. response definition



```golang
type LogoutResponse struct {
}
```

