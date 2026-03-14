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

// 更新节点元数据
func NodeMetaUpdateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.NodeMetaUpdateRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := node.NewNodeMetaUpdateLogic(c.Request.Context(), svcCtx)
		resp, err := l.NodeMetaUpdate(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
