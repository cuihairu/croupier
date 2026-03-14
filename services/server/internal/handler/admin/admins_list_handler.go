package admin

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"
	"github.com/cuihairu/croupier/services/server/internal/logic/admin"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/gin-gonic/gin"
)

// 获取管理员列表
func AdminsListHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.AdminsListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := admin.NewAdminsListLogic(c.Request.Context(), svcCtx)
		resp, err := l.AdminsList(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
