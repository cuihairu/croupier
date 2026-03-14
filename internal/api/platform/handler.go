package platform

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

// Call handles the request to call a platform method
func (h *Handler) Call(c *gin.Context) {
	var req CallPlatformRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Call(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ListPlatforms handles the request to list platforms
func (h *Handler) ListPlatforms(c *gin.Context) {
	resp, err := h.service.ListPlatforms(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ListMethods handles the request to list platform methods
func (h *Handler) ListMethods(c *gin.Context) {
	platform := c.Param("platform")
	resp, err := h.service.ListMethods(c.Request.Context(), platform)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ReloadConfig handles the request to reload platform configuration
func (h *Handler) ReloadConfig(c *gin.Context) {
	resp, err := h.service.ReloadConfig(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Alias methods for route compatibility

func (h *Handler) List(c *gin.Context) {
	h.ListPlatforms(c)
}

func (h *Handler) Methods(c *gin.Context) {
	h.ListMethods(c)
}

func (h *Handler) Reload(c *gin.Context) {
	h.ReloadConfig(c)
}
