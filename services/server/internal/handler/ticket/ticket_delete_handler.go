// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/ticket"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 删除工单
func TicketDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.TicketDeleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := ticket.NewTicketDeleteLogic(c.Request.Context(), svcCtx)
		err := l.TicketDelete(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, gin.H{"message": "操作成功"})
		}
	}
}
