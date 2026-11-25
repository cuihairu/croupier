package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// LBStatsQueryRequest is a restricted PromQL instant-query request.
type LBStatsQueryRequest struct {
	Query string `json:"query"`
}

// LBStatsQuery handles POST /ops/cluster/lb-stats: proxy an allowlisted PromQL
// query to the configured Prometheus (an explicit error when it is not configured).
func (h *Handler) LBStatsQuery(c *gin.Context) {
	var req LBStatsQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	result, err := h.service.LBStatsQuery(c.Request.Context(), req.Query)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, result)
}
