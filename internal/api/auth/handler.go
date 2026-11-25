package auth

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Login is the login handler.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.ClientIP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	resp, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrMFARequired) || errors.Is(err, ErrEmailNotVerified) {
			// CodeError → 401+mfa_required / 403+email_not_verified (the
			// frontend branches on the stable code to show the second-factor
			// code input or the resend-verification-email entry); other
			// credential errors keep 401 semantics.
			response.Error(c, err)
			return
		}
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, resp)
}

// Refresh is the refresh-token exchange handler (AUD-052, anonymous endpoint:
// the refresh token is itself the credential). Every failure uniformly maps
// to 401 (including the message shown after replay revocation); internal
// errors like a storage backend not being wired are presented as 500 without
// leaking details to the anonymous caller.
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Unauthorized(c, "刷新令牌无效或已过期")
		return
	}
	req.ClientIP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	resp, err := h.service.Refresh(c.Request.Context(), &req)
	if err != nil {
		var codeErr *errorx.CodeError
		if errors.As(err, &codeErr) {
			response.Error(c, err)
			return
		}
		response.Unauthorized(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Logout is the logout handler.
func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	// service.Logout's signature was tightened (the implementation has no error
	// path), so there is no error branch here anymore.
	response.Success(c, h.service.Logout(c.Request.Context(), &req))
}

func (h *Handler) Check(c *gin.Context) {
	username, ok := c.Get("username")
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	var req CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	resp, err := h.service.Check(c.Request.Context(), username.(string), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Providers returns the enabled login methods, for the login page to render
// its entry points.
func (h *Handler) Providers(c *gin.Context) {
	response.Success(c, gin.H{
		"local":        h.service.LocalEnabled(),
		"ldap":         h.service.LDAPEnabled(),
		"oidc":         h.service.OIDCEnabled(),
		"github":       h.service.GitHubEnabled(),
		"wechat":       h.service.WeChatEnabled(),
		"genericoauth": h.service.GenericOAuthEnabled(),
		"register":     h.service.RegisterEnabled(),
	})
}

// Register serves self-service registration (POST /api/v1/auth/register,
// anonymous; the switch defaults to off — when off it answers 403
// registration_disabled and the frontend renders no registration entry).
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.ClientIP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")

	admin, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		if h.service.RegisterEnabled() {
			// switch on: a business validation failure (username shape/duplicate/
			// weak password)
			response.BadRequest(c, err.Error())
			return
		}
		response.Error(c, &errorx.CodeError{Code: http.StatusForbidden, Message: err.Error(), StableCode: "registration_disabled"})
		return
	}
	response.Success(c, gin.H{"username": admin.Username, "nickname": admin.Nickname})
}

// VerifyEmail serves GET /api/v1/auth/verify-email?token=... (anonymous, the
// token is itself the credential). A successful check lands
// admins.email_verified=true; invalid/expired/already-used all answer 400
// with the same wording (the reason is not distinguished, to prevent probing
// for token validity).
func (h *Handler) VerifyEmail(c *gin.Context) {
	if err := h.service.VerifyEmailToken(c.Request.Context(), c.Query("token")); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"verified": true})
}

// ResendVerification serves POST /api/v1/auth/resend-verification (anonymous).
// Anti-enumeration: username+email mismatch / already verified / within the
// rate limit all answer 200 {resent:false}; mail goes out and {resent:true}
// is returned only on a real match that is still unverified.
func (h *Handler) ResendVerification(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.service.ResendVerification(c.Request.Context(), req.Username, req.Email); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"resent": true})
}

// mfaUsername pulls the current user from the auth context; when not logged
// in it returns the empty string and the caller answers 401.
func mfaUsername(c *gin.Context) string {
	if username, ok := c.Get("username"); ok {
		if s, ok := username.(string); ok {
			return s
		}
	}
	return ""
}

// MFASetup generates a TOTP secret (POST /api/v1/auth/mfa/setup, login
// required).
func (h *Handler) MFASetup(c *gin.Context) {
	username := mfaUsername(c)
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}
	resp, err := h.service.MFASetup(c.Request.Context(), username)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// MFAConfirm confirms enabling TOTP (POST /api/v1/auth/mfa/confirm, login
// required).
func (h *Handler) MFAConfirm(c *gin.Context) {
	username := mfaUsername(c)
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}
	var req MFAConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.service.MFAConfirm(c.Request.Context(), username, req.Code)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// The plaintext backup recovery codes are returned exactly once, right
	// here (only hashes are stored), and must be passed through for the
	// frontend to display.
	response.Success(c, gin.H{"ok": true, "recoveryCodes": resp.RecoveryCodes})
}

// MFAStatus queries the current account's two-factor status
// (GET /api/v1/auth/mfa/status).
func (h *Handler) MFAStatus(c *gin.Context) {
	username := mfaUsername(c)
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}
	// service.MFAStatus's signature was tightened (the implementation has no
	// error path), so there is no error branch here anymore.
	response.Success(c, h.service.MFAStatus(c.Request.Context(), username))
}

// MFADisable disables TOTP (POST /api/v1/auth/mfa/disable, login required,
// confirmed by code + password).
func (h *Handler) MFADisable(c *gin.Context) {
	username := mfaUsername(c)
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}
	var req MFADisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.service.MFADisable(c.Request.Context(), username, req.Code, req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// OIDCLogin builds the authorization URL pointing at the IdP and 302-redirects
// to it.
func (h *Handler) OIDCLogin(c *gin.Context) {
	url, err := h.service.OIDCAuthCodeURL()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.Redirect(http.StatusFound, url)
}

// OIDCCallback handles the IdP callback: it exchanges the identity and issues
// a platform token. With loginSuccessUrl configured it redirects to the
// frontend carrying the token; otherwise it returns JSON.
func (h *Handler) OIDCCallback(c *gin.Context) {
	h.oauthCallback(c,
		func(code, state string, req *LoginRequest) (*LoginResponse, error) {
			return h.service.OIDCLoginCallback(c.Request.Context(), code, state, req)
		},
		h.service.OIDCSuccessURL)
}

// GitHubLogin builds the GitHub authorization URL and 302-redirects to it.
func (h *Handler) GitHubLogin(c *gin.Context) {
	u, err := h.service.GitHubAuthCodeURL()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.Redirect(http.StatusFound, u)
}

// GitHubCallback handles the GitHub OAuth2 callback (success-redirect
// semantics same as OIDC).
func (h *Handler) GitHubCallback(c *gin.Context) {
	h.oauthCallback(c,
		func(code, state string, req *LoginRequest) (*LoginResponse, error) {
			return h.service.GitHubLoginCallback(c.Request.Context(), code, state, req)
		},
		h.service.GitHubSuccessURL)
}

// WeChatLogin builds the WeChat QR-scan login page URL and 302-redirects to
// it (#51, third batch).
func (h *Handler) WeChatLogin(c *gin.Context) {
	u, err := h.service.WeChatAuthCodeURL()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.Redirect(http.StatusFound, u)
}

// WeChatCallback handles the WeChat QR-scan callback (success-redirect
// semantics same as OIDC/GitHub).
func (h *Handler) WeChatCallback(c *gin.Context) {
	h.oauthCallback(c,
		func(code, state string, req *LoginRequest) (*LoginResponse, error) {
			return h.service.WeChatLoginCallback(c.Request.Context(), code, state, req)
		},
		h.service.WeChatSuccessURL)
}

// GenericOAuthLogin builds the custom OAuth authorization URL and
// 302-redirects to it (#51, third batch).
func (h *Handler) GenericOAuthLogin(c *gin.Context) {
	u, err := h.service.GenericOAuthAuthCodeURL()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.Redirect(http.StatusFound, u)
}

// GenericOAuthCallback handles the custom OAuth callback (success-redirect
// semantics same as OIDC/GitHub).
func (h *Handler) GenericOAuthCallback(c *gin.Context) {
	h.oauthCallback(c,
		func(code, state string, req *LoginRequest) (*LoginResponse, error) {
			return h.service.GenericOAuthLoginCallback(c.Request.Context(), code, state, req)
		},
		h.service.GenericOAuthSuccessURL)
}

// oauthCallback is the shared exit for redirect-style OAuth callback handlers:
// on success with loginSuccessUrl configured it 302-redirects to the frontend
// carrying the token, otherwise it returns JSON (four isomorphic paths:
// OIDC/GitHub/WeChat/custom OAuth).
func (h *Handler) oauthCallback(c *gin.Context,
	callback func(code, state string, req *LoginRequest) (*LoginResponse, error),
	successURL func() string,
) {
	req := &LoginRequest{
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}
	resp, err := callback(c.Query("code"), c.Query("state"), req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	if target := successURL(); target != "" {
		u, parseErr := url.Parse(target)
		if parseErr != nil {
			response.InternalServerError(c, "loginSuccessUrl 配置无效")
			return
		}
		q := u.Query()
		q.Set("token", resp.Token)
		u.RawQuery = q.Encode()
		c.Redirect(http.StatusFound, u.String())
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BatchCheck(c *gin.Context) {
	username, ok := c.Get("username")
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	var req BatchCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	resp, err := h.service.BatchCheck(c.Request.Context(), username.(string), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}
