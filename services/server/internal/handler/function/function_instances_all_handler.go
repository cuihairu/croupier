// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/function"
	"github.com/cuihairu/croupier/services/server/internal/svc"

	"github.com/gin-gonic/gin"
)

// 获取所有函数实例
func FunctionInstancesAllHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		l := function.NewFunctionInstancesAllLogic(c.Request.Context(), svcCtx)
		resp, err := l.FunctionInstancesAll()
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
