package alert

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles the request to list alerts
func (h *Handler) List(c *gin.Context) {
	var req AlertsListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Silence handles the request to silence an alert
func (h *Handler) Silence(c *gin.Context) {
	var req AlertSilenceRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.Silence(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// SilencesList handles the request to list silence rules
func (h *Handler) SilencesList(c *gin.Context) {
	var req SilencesListRequest
	// No parameters to bind

	resp, err := h.service.SilencesList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// SilenceDelete handles the request to delete a silence rule
func (h *Handler) SilenceDelete(c *gin.Context) {
	var req SilenceDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.SilenceDelete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}
