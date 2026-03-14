package admin

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/admin"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 更新管理员游戏范围
func AdminGamesUpdateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.AdminGamesUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := admin.NewAdminGamesUpdateLogic(c.Request.Context(), svcCtx)
		resp, err := l.AdminGamesUpdate(&req)
		if err != nil {
			response.Error(c, err)
			return
		}
		response.Success(c, resp)
	}
}
