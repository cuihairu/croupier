// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/agent"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/gin-gonic/gin"
)

// 获取分析过滤器
func AgentAnalyticsFiltersHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.AnalyticsFiltersQuery
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := agent.NewAgentAnalyticsFiltersLogic(c.Request.Context(), svcCtx)
		resp, err := l.AgentAnalyticsFilters(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
