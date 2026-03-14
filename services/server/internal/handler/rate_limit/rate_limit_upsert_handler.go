// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/rate_limit"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 创建/更新限流规则
func RateLimitUpsertHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RateLimitUpsertRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := rate_limit.NewRateLimitUpsertLogic(c.Request.Context(), svcCtx)
		resp, err := l.RateLimitUpsert(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
