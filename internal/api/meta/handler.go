package meta

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// RootProvider 是 Root 元信息查询的最小接口缝隙。
// 生产实现为 *Service（恒返回 nil error），接口化使 handler 的
// 错误分支可在测试中注入触达。
type RootProvider interface {
	Root(ctx context.Context) (*RootResponse, error)
}

type Handler struct {
	service RootProvider
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Root handles the root path - returns API information and version
func (h *Handler) Root(c *gin.Context) {
	resp, err := h.service.Root(c.Request.Context())
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
}
