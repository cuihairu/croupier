package admin

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/admin"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 获取管理员游戏范围
func AdminGamesHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.AdminGamesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := admin.NewAdminGamesLogic(c.Request.Context(), svcCtx)
		resp, err := l.AdminGames(&req)
		if err != nil {
			response.Error(c, err)
			return
		}
		response.Success(c, resp)
	}
}
