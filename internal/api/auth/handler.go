package auth

import (
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
