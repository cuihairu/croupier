package profile

import (
	"github.com/cuihairu/croupier/services/server/internal/pkg/response"
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

// GetProfile 获取个人资料
func (h *Handler) GetProfile(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}

	resp, err := h.service.GetProfile(c.Request.Context(), username)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
}

// GetGames 获取我的游戏列表
func (h *Handler) GetGames(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}

	resp, err := h.service.GetUserGames(c.Request.Context(), username)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
}

// UpdateProfile 更新个人资料
func (h *Handler) UpdateProfile(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}

	var req ProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	resp, err := h.service.UpdateProfile(c.Request.Context(), username, &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
}

// ChangePassword 修改密码
func (h *Handler) ChangePassword(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		response.Unauthorized(c, "未授权")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	resp, err := h.service.ChangePassword(c.Request.Context(), username, &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
}
