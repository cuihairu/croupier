// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/game"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 删除游戏环境
func GameEnvDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.GameEnvDeleteRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := game.NewGameEnvDeleteLogic(c.Request.Context(), svcCtx)
		resp, err := l.GameEnvDelete(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
