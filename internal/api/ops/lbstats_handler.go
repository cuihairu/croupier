package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// LBStatsQueryRequest 是受限 PromQL 即时查询请求。
type LBStatsQueryRequest struct {
	Query string `json:"query"`
}

// LBStatsQuery handles POST /ops/cluster/lb-stats：代理白名单内的
// PromQL 查询到配置的 Prometheus（未配置时明确报错）。
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
