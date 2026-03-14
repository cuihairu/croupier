// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/platform"
	"github.com/cuihairu/croupier/services/server/internal/svc"

	"github.com/gin-gonic/gin"
)

// 重新加载平台配置
func ReloadPlatformConfigHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		l := platform.NewReloadPlatformConfigLogic(c.Request.Context(), svcCtx)
		resp, err := l.ReloadPlatformConfig()
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
