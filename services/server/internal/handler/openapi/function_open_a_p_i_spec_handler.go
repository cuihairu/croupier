// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/openapi"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/gin-gonic/gin"
)

// 获取函数的 OpenAPI spec
func FunctionOpenAPISpecHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.OpenAPISpecRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := openapi.NewFunctionOpenAPISpecLogic(c.Request.Context(), svcCtx)
		resp, err := l.FunctionOpenAPISpec(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
