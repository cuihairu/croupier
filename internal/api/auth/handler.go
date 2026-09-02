package auth

import (
	"errors"
	"net/http"
	"net/url"

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

// Login 登录处理器
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
		if errors.Is(err, ErrMFARequired) {
			// CodeError → 401 + error=mfa_required（前端按稳定码分支
			// 展示二次验证码输入）；其余凭据错误保持 401 语义。
			response.Error(c, err)
			return
		}
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, resp)
}

// Logout 登出处理器
func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	resp, err := h.service.Logout(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
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

// Providers 返回已启用的登录方式，供登录页渲染入口。
func (h *Handler) Providers(c *gin.Context) {
	response.Success(c, gin.H{
		"local": true,
		"ldap":  h.service.LDAPEnabled(),
		"oidc":  h.service.OIDCEnabled(),
	})
}

// mfaUsername 从认证上下文取当前用户；未登录返回空串并由调用方 401。
func mfaUsername(c *gin.Context) string {
	if username, ok := c.Get("username"); ok {
		if s, ok := username.(string); ok {
			return s
		}
	}
	return ""
}

// MFASetup 生成 TOTP 密钥（POST /api/v1/auth/mfa/setup，需登录）。
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

// MFAConfirm 确认启用 TOTP（POST /api/v1/auth/mfa/confirm，需登录）。
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
	if err := h.service.MFAConfirm(c.Request.Context(), username, req.Code); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// MFADisable 关闭 TOTP（POST /api/v1/auth/mfa/disable，需登录，码+密码双确认）。
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

// OIDCLogin 生成跳转到身份源的授权 URL 并 302 重定向。
func (h *Handler) OIDCLogin(c *gin.Context) {
	url, err := h.service.OIDCAuthCodeURL()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.Redirect(http.StatusFound, url)
}

// OIDCCallback 处理身份源回调：换取身份并签发平台 token。
// 配置了 loginSuccessUrl 时携带 token 跳转前端；否则返回 JSON。
func (h *Handler) OIDCCallback(c *gin.Context) {
	req := &LoginRequest{
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}
	resp, err := h.service.OIDCLoginCallback(c.Request.Context(), c.Query("code"), c.Query("state"), req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	if target := h.service.OIDCSuccessURL(); target != "" {
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
