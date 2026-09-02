package announcement

import (
	"strconv"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ---- 管理端 ----

// List serves GET /admin/announcements.
func (h *Handler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Create serves POST /admin/announcements.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, resp)
}

// Update serves PUT /admin/announcements/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的公告 ID")
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Update(c.Request.Context(), uint(id), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Delete serves DELETE /admin/announcements/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的公告 ID")
		return
	}
	if err := h.service.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// ---- 用户侧 ----

// Active serves GET /announcements/active：当前用户可见的生效公告，
// shouldPopup=true 的条目前端在登录后弹窗展示直至确认。
func (h *Handler) Active(c *gin.Context) {
	username, roles, err := utils.LoadCurrentAdmin(c.Request.Context(), h.service.svcCtx)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.ActiveForUser(c.Request.Context(), username.Username, utils.RoleNamesFromModels(roles))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Dismiss serves POST /announcements/:id/dismiss：确认公告（不再弹窗，幂等）。
func (h *Handler) Dismiss(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的公告 ID")
		return
	}
	username, _, err := utils.LoadCurrentAdmin(c.Request.Context(), h.service.svcCtx)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Dismiss(c.Request.Context(), username.Username, uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
