package auth

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"
	"github.com/cuihairu/croupier/services/server/internal/logic/auth"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/gin-gonic/gin"
)

// 用户登录
func LoginHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := auth.NewLoginLogic(c.Request.Context(), svcCtx)
		resp, err := l.Login(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
