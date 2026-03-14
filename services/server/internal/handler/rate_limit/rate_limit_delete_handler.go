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

// 删除限流规则
func RateLimitDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RateLimitDeleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := rate_limit.NewRateLimitDeleteLogic(c.Request.Context(), svcCtx)
		err := l.RateLimitDelete(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, gin.H{"message": "操作成功"})
		}
	}
}
