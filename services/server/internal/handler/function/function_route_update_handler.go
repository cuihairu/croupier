// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/function"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
"github.com/gin-gonic/gin"
)

// 更新函数路由配置
func FunctionRouteUpdateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.FunctionRouteUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := function.NewFunctionRouteUpdateLogic(c.Request.Context(), svcCtx)
		resp, err := l.FunctionRouteUpdate(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
