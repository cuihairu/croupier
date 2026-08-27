package configexplorer

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/gin-gonic/gin"
)

// Handler exposes the config explorer HTTP API.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListBindings handles GET /config-explorer/sources.
func (h *Handler) ListBindings(c *gin.Context) {
	bindings, err := h.service.ListBindings(c.Request.Context(),
		c.Query("gameId"), c.Query("env"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"items": bindings})
}

// UpsertBinding handles POST /config-explorer/sources.
func (h *Handler) UpsertBinding(c *gin.Context) {
	var req BindingUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	dto, err := h.service.UpsertBinding(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, dto)
}

// DeleteBinding handles DELETE /config-explorer/sources/:id.
func (h *Handler) DeleteBinding(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的数据源 ID")
		return
	}
	if err := h.service.DeleteBinding(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// List handles GET /config-explorer/tree.
func (h *Handler) List(c *gin.Context) {
	sourceID, err := parseID(c.Query("sourceId"))
	if err != nil {
		response.BadRequest(c, "无效的 sourceId")
		return
	}
	entries, err := h.service.List(c.Request.Context(), sourceID, c.Query("dir"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"items": entries})
}

// Read handles GET /config-explorer/file.
func (h *Handler) Read(c *gin.Context) {
	sourceID, err := parseID(c.Query("sourceId"))
	if err != nil {
		response.BadRequest(c, "无效的 sourceId")
		return
	}
	file, err := h.service.Read(c.Request.Context(), sourceID, c.Query("path"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, file)
}

// Write handles PUT /config-explorer/file（应急编辑）.
func (h *Handler) Write(c *gin.Context) {
	var req WriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	actor, _ := utils.CurrentUsername(c.Request.Context())
	if err := h.service.Write(c.Request.Context(), &req, actor); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
