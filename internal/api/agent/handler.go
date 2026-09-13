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
	// GET 请求无 body 时 ShouldBindJSON 返回 EOF，零值 req 即预期——显式丢弃绑定错误
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
	// GET 请求无 body 时 ShouldBindJSON 返回 EOF，零值 req 即预期——显式丢弃绑定错误
	_ = c.ShouldBindJSON(&req)

	resp, err := h.service.UpdateMeta(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}
