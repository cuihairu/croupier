// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package role

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/role"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
"github.com/gin-gonic/gin"
)

// 更新角色
func RoleUpdateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.RoleUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := role.NewRoleUpdateLogic(c.Request.Context(), svcCtx)
		resp, err := l.RoleUpdate(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
