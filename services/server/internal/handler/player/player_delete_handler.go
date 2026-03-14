// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/player"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 删除玩家
func PlayerDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.PlayerDeleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := player.NewPlayerDeleteLogic(c.Request.Context(), svcCtx)
		err := l.PlayerDelete(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, gin.H{"message": "操作成功"})
		}
	}
}
