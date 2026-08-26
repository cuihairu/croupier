package dbmon

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

// NewHandler creates a dbmon handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListSources handles GET /dbmon/sources.
func (h *Handler) ListSources(c *gin.Context) {
	resp, err := h.service.ListSources(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CreateSource handles POST /dbmon/sources.
func (h *Handler) CreateSource(c *gin.Context) {
	var req SourceUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.CreateSource(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UpdateSource handles PUT /dbmon/sources/:id.
func (h *Handler) UpdateSource(c *gin.Context) {
	var req SourceUpdateRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.UpdateSource(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DeleteSource handles DELETE /dbmon/sources/:id.
func (h *Handler) DeleteSource(c *gin.Context) {
	var req SourceDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.service.DeleteSource(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "删除成功"})
}

// ProbeAll handles POST /dbmon/probe.
func (h *Handler) ProbeAll(c *gin.Context) {
	resp, err := h.service.ProbeAll(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
