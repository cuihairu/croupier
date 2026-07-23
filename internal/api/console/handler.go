package console

import (
	"errors"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// Handler handles Console API requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new Console Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Menu handles GET /api/v1/console/menu
func (h *Handler) Menu(c *gin.Context) {
	var req ConsoleMenuRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Menu(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Pages handles GET /api/v1/console/pages
func (h *Handler) Pages(c *gin.Context) {
	var req ConsolePagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Pages(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Page handles GET /api/v1/console/pages/:pageKey
func (h *Handler) Page(c *gin.Context) {
	var req ConsolePageRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Page(c.Request.Context(), &req)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
