// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/ops"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 获取函数列表
func OpsFunctionsHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.OpsFunctionsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := ops.NewOpsFunctionsLogic(c.Request.Context(), svcCtx)
		resp, err := l.OpsFunctions(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
