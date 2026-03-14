// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package adminGames

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/adminGames"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"


"github.com/gin-gonic/gin"
)

// 获取管理员的游戏访问权限
func AdminGamesHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.AdminGamesRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := adminGames.NewAdminGamesLogic(c.Request.Context(), svcCtx)
		resp, err := l.AdminGames(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
