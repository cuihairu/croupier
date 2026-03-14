// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/storage"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 列出对象
func ListObjectsHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.ListObjectsRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := storage.NewListObjectsLogic(c.Request.Context(), svcCtx)
		resp, err := l.ListObjects(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
