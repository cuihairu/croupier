// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/platform"
	"github.com/cuihairu/croupier/services/server/internal/svc"

"github.com/gin-gonic/gin"
)

// 获取所有可用的第三方平台列表
func ListPlatformsHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		l := platform.NewListPlatformsLogic(c.Request.Context(), svcCtx)
		resp, err := l.ListPlatforms()
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
