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

// 批量删除函数
func BatchDeleteFunctionsHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.BatchDeleteFunctionsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := function.NewBatchDeleteFunctionsLogic(c.Request.Context(), svcCtx)
		resp, err := l.BatchDeleteFunctions(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
