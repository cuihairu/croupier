package entity

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles the request to list entities
func (h *Handler) List(c *gin.Context) {
	var req EntitiesListRequest
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

// Create handles the request to create an entity
func (h *Handler) Create(c *gin.Context) {
	var req EntityCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Detail handles the request to get entity details
func (h *Handler) Detail(c *gin.Context) {
	var req EntityDetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Detail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Update handles the request to update an entity
func (h *Handler) Update(c *gin.Context) {
	var req EntityUpdateRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Update(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Delete handles the request to delete an entity
func (h *Handler) Delete(c *gin.Context) {
	var req EntityDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Delete(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Preview handles the request to preview an entity
func (h *Handler) Preview(c *gin.Context) {
	var req EntityPreviewRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Preview(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Validate handles the request to validate entity data
func (h *Handler) Validate(c *gin.Context) {
	var req EntityValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Validate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Get alias for route compatibility
func (h *Handler) Get(c *gin.Context) {
	h.Detail(c)
}
