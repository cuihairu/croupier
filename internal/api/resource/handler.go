package resource

import (
	"errors"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// Handler handles Resource API requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new Resource Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/resources
func (h *Handler) List(c *gin.Context) {
	var req ResourceListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.List(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Detail handles GET /api/v1/resources/:resourceKey
func (h *Handler) Detail(c *gin.Context) {
	var req ResourceDetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Detail(c.Request.Context(), &req)
	if err != nil {
		var notFound *ResourceNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Operations handles GET /api/v1/resources/:resourceKey/operations
func (h *Handler) Operations(c *gin.Context) {
	var req ResourceOperationsRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Operations(c.Request.Context(), &req)
	if err != nil {
		var notFound *ResourceNotFoundError
		if errors.As(err, &notFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
