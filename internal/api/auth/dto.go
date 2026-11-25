package auth

// LoginRequest is the login request payload.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// TOTPCode is the second-factor code local accounts with MFA enabled must
	// present at login.
	TOTPCode string `json:"totpCode,omitempty"`
	// UseRefresh opts into refresh token rotation (AUD-052): the response
	// carries a short-lived access token (2h) plus a refresh token (7d;
	// single-use, rotated on redemption, replay revokes the whole family).
	// Default false = legacy single 24h token, so existing clients (mobile)
	// behave exactly as before.
	UseRefresh bool   `json:"useRefresh,omitempty"`
	ClientIP   string `json:"-"`
	UserAgent  string `json:"-"`
}

// LoginResponse is the login response payload.
type LoginResponse struct {
	Token      string   `json:"token"`
	User       UserInfo `json:"user"`
	LastGameID string   `json:"lastGameId,omitempty"`
	LastEnv    string   `json:"lastEnv,omitempty"`
	// RefreshToken is only returned when the request sets useRefresh=true
	// (AUD-052); the cleartext value appears exactly once, in the issue or
	// rotation response, and the server only stores its sha256.
	RefreshToken string `json:"refreshToken,omitempty"`
	// ExpiresIn is the token's lifetime in seconds: 2h on the useRefresh
	// path, 24h for legacy.
	ExpiresIn int64 `json:"expiresIn,omitempty"`
	// MustChangePassword is true when the account is flagged as requiring a
	// password change after login, or its password is past its expiry: the
	// token is still issued (the change-password endpoint needs an
	// authenticated session) and the frontend forces the change-password flow
	// before further use (OPEN-ISSUES #20).
	MustChangePassword bool `json:"mustChangePassword,omitempty"`
	// MFASetupRequired is true when the security policy
	// security.mfaRequired is on and this local account has no TOTP bound
	// yet: the token is still issued (the MFA binding endpoint needs an
	// authenticated session) and the frontend forces second-factor binding;
	// the server-side auth middleware blocks all other APIs in step (403
	// mfa_required), structurally the same as mustChangePassword.
	MFASetupRequired bool `json:"mfaSetupRequired,omitempty"`
}

// RefreshRequest is the refresh token redemption request (AUD-052; anonymous
// endpoint: the refresh token itself is the credential).
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
	ClientIP     string `json:"-"`
	UserAgent    string `json:"-"`
}

// RefreshResponse is the refresh token redemption response: new access token
// plus a new (rotated) refresh token; the old refresh token is void from here on.
type RefreshResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	Username     string `json:"username"`
}

// UserInfo is the user information payload.
type UserInfo struct {
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	Roles    []string `json:"roles"`
}

// LogoutRequest is the logout request payload.
type LogoutRequest struct {
	// Username is injected by the handler from the auth context (not bound
	// from the request body).
	Username string `json:"-"`
}

// LogoutResponse is the logout response payload.
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
