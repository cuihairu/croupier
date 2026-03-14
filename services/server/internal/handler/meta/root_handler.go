// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package meta

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/meta"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 根路径 - API 信息和版本
func RootHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RootRequest
		// No parameters to bind

		l := meta.NewRootLogic(c.Request.Context(), svcCtx)
		resp, err := l.Root(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
