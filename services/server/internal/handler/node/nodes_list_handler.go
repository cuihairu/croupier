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

// 获取节点列表
func NodesListHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.NodesListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := node.NewNodesListLogic(c.Request.Context(), svcCtx)
		resp, err := l.NodesList(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
