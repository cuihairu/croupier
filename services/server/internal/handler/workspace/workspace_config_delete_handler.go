// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/workspace"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 删除 Workspace 配置
func WorkspaceConfigDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.WorkspaceConfigDeleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := workspace.NewWorkspaceConfigDeleteLogic(c.Request.Context(), svcCtx)
		resp, err := l.WorkspaceConfigDelete(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
