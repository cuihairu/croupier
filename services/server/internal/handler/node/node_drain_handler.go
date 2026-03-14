// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/node"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 排空节点
func NodeDrainHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.NodeDrainRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := node.NewNodeDrainLogic(c.Request.Context(), svcCtx)
		err := l.NodeDrain(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, gin.H{"message": "操作成功"})
		}
	}
}
