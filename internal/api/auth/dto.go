package auth

// LoginRequest 登录请求
type LoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	ClientIP  string `json:"-"`
	UserAgent string `json:"-"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	Roles    []string `json:"roles"`
}

// LogoutRequest 登出请求
type LogoutRequest struct{}

// LogoutResponse 登出响应
type LogoutResponse struct{}

type CheckRequest struct {
	Resource string `json:"resource" binding:"required"`
	Action   string `json:"action" binding:"required"`
	GameID   string `json:"gameId"`
	Env      string `json:"env"`
}

type CheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type BatchCheckRequest struct {
	Checks []CheckRequest `json:"checks" binding:"required"`
}

type BatchCheckResponse struct {
	Results []CheckResponse `json:"results"`
}
