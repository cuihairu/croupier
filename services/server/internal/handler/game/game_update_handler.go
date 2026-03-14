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

// 更新游戏
func GameUpdateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.GameUpdateRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := game.NewGameUpdateLogic(c.Request.Context(), svcCtx)
		resp, err := l.GameUpdate(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
