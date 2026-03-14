// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"
	
"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/services/server/internal/logic/admin"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// 重置管理员密码
func AdminPasswordResetHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.AdminPasswordResetRequest
		if err := c.ShouldBindUri(&req); err != nil {
			response.Error(c, err)
			return
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := admin.NewAdminPasswordResetLogic(c.Request.Context(), svcCtx)
		err := l.AdminPasswordReset(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, gin.H{"message": "操作成功"})
		}
	}
}
