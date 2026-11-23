package auth

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// TOTPCode 是已启用 MFA 的本地账号登录时必须携带的二次验证码。
	TOTPCode string `json:"totpCode,omitempty"`
	// UseRefresh 选择启用刷新令牌轮换（AUD-052）：响应携带短寿命 access
	// （2h）+ 刷新令牌（7d，一次性、兑换即轮换、重放整族吊销）。缺省
	// false = legacy 单 24h token，既有客户端（移动端）行为完全不变。
	UseRefresh bool   `json:"useRefresh,omitempty"`
	ClientIP   string `json:"-"`
	UserAgent  string `json:"-"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token      string   `json:"token"`
	User       UserInfo `json:"user"`
	LastGameID string   `json:"lastGameId,omitempty"`
	LastEnv    string   `json:"lastEnv,omitempty"`
	// RefreshToken 仅在请求 useRefresh=true 时返回（AUD-052）；明文只在
	// 签发/轮换响应出现一次，服务端只存 sha256。
	RefreshToken string `json:"refreshToken,omitempty"`
	// ExpiresIn 是 Token 的有效期（秒）：useRefresh 路径为 2h，legacy 为 24h。
	ExpiresIn int64 `json:"expiresIn,omitempty"`
	// MustChangePassword 账号被标记「登录后必须修改密码」或密码已过有效期
	// 时为 true：token 照常签发（改密接口需要认证态），前端据此强制进入
	// 改密流程后再继续使用（OPEN-ISSUES #20）。
	MustChangePassword bool `json:"mustChangePassword,omitempty"`
	// MFASetupRequired 账号安全策略 security.mfaRequired 开启且该 local
	// 账号尚未绑定 TOTP 时为 true：token 照常签发（MFA 绑定接口需要认证态），
	// 前端据此强制引导绑定二次验证；服务端鉴权中间件同步拦截其余 API
	//（403 mfa_required），与 mustChangePassword 同构。
	MFASetupRequired bool `json:"mfaSetupRequired,omitempty"`
}

// RefreshRequest 刷新令牌兑换请求（AUD-052，匿名端点：刷新令牌本身即凭据）。
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
	ClientIP     string `json:"-"`
	UserAgent    string `json:"-"`
}

// RefreshResponse 刷新令牌兑换响应：新 access + 新刷新令牌（轮换），
// 旧刷新令牌自此作废。
type RefreshResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	Username     string `json:"username"`
}

// UserInfo 用户信息
type UserInfo struct {
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	Roles    []string `json:"roles"`
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	// Username 由 handler 从认证上下文注入（不从请求体绑定）。
	Username string `json:"-"`
}

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
