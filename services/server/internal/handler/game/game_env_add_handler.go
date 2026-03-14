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

// 添加游戏环境
func GameEnvAddHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.GameEnvAddRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := game.NewGameEnvAddLogic(c.Request.Context(), svcCtx)
		resp, err := l.GameEnvAdd(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
