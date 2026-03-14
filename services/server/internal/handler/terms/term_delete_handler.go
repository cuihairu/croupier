// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package terms

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/terms"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 删除术语
func TermDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.TermDeleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := terms.NewTermDeleteLogic(c.Request.Context(), svcCtx)
		resp, err := l.TermDelete(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
