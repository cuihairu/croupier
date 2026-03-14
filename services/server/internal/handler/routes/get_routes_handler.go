// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package routes

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"
	"github.com/cuihairu/croupier/services/server/internal/logic/routes"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/gin-gonic/gin"
)

// 获取动态路由配置
func GetRoutesHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		l := routes.NewGetRoutesLogic(c.Request.Context(), svcCtx)
		resp, err := l.GetRoutes()
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
