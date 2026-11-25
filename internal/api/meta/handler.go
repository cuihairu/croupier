package meta

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// RootProvider is the minimal interface seam for querying Root meta info.
// Production uses *Service (which always returns a nil error); going through the
// interface lets tests reach the handler's error branch by injection.
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
