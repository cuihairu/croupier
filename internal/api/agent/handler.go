package agent

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

// GetAnalyticsFilters handles the analytics filters request
func (h *Handler) GetAnalyticsFilters(c *gin.Context) {
	var req GetAnalyticsFiltersRequest
	// A GET request has no body, so ShouldBindJSON returns EOF and the zero-value req is
	// the expected state — the binding error is discarded on purpose
	_ = c.ShouldBindJSON(&req)

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
	// A GET request has no body, so ShouldBindJSON returns EOF and the zero-value req is
	// the expected state — the binding error is discarded on purpose
	_ = c.ShouldBindJSON(&req)

	resp, err := h.service.UpdateMeta(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}
