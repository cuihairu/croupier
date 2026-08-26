package auth

import (
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
