package meta

import (
	"github.com/cuihairu/croupier/internal/pkg2/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
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
