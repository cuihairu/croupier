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

// 删除函数
func FunctionDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.FunctionActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := function.NewFunctionDeleteLogic(c.Request.Context(), svcCtx)
		err := l.FunctionDelete(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, gin.H{"message": "操作成功"})
		}
	}
}
