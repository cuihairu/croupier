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

// ---- Admin side ----

// List serves GET /admin/announcements?gameId=: when gameId is non-empty,
// only announcements that apply to that game are returned (unbound = visible
// server-wide, or bound to that game, #45).
func (h *Handler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context(), c.Query("gameId"))
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

// ---- User side ----

// Active serves GET /announcements/active: active announcements visible to
// the current user; items with shouldPopup=true are shown by the frontend as
// a popup after login until acknowledged.
// The game filter (#45) reads the X-Game-ID header (the top bar's global
// scope is injected by the frontend interceptor; this route is not in the
// scoped group and has no GameDBMiddleware, so the header is read directly).
func (h *Handler) Active(c *gin.Context) {
	username, roles, err := utils.LoadCurrentAdmin(c.Request.Context(), h.service.svcCtx)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.ActiveForUser(c.Request.Context(), username.Username,
		utils.RoleNamesFromModels(roles), c.GetHeader("X-Game-ID"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Dismiss serves POST /announcements/:id/dismiss: acknowledge an
// announcement (stops the popup, idempotent).
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
