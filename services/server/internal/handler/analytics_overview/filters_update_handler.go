// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/analytics_overview"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 更新分析过滤器
func FiltersUpdateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.FiltersUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := analytics_overview.NewFiltersUpdateLogic(c.Request.Context(), svcCtx)
		resp, err := l.FiltersUpdate(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
