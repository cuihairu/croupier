// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/platform"
	"github.com/cuihairu/croupier/services/server/internal/svc"

"github.com/gin-gonic/gin"
)

// 获取指定平台支持的方法列表
func ListPlatformMethodsHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		platformName := c.Param("platform")
		l := platform.NewListPlatformMethodsLogic(c.Request.Context(), svcCtx)
		resp, err := l.ListPlatformMethods(platformName)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
