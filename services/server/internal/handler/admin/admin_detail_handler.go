// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"
	
"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/services/server/internal/logic/admin"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// 获取管理员详情
func AdminDetailHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.AdminDetailRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := admin.NewAdminDetailLogic(c.Request.Context(), svcCtx)
		resp, err := l.AdminDetail(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
