package agent

import (
	"github.com/cuihairu/croupier/internal/pkg2/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetAnalyticsFilters handles the analytics filters request
func (h *Handler) GetAnalyticsFilters(c *gin.Context) {
	var req GetAnalyticsFiltersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Handle binding error - for GET requests with no body, we can ignore
	}

	resp, err := h.service.GetAnalyticsFilters(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// UpdateMeta handles the agent metadata update request
func (h *Handler) UpdateMeta(c *gin.Context) {
	var req UpdateMetaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Handle binding error - for GET requests with no body, we can ignore
	}

	resp, err := h.service.UpdateMeta(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}
