package provider

import (
	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func bindProviderRequest(c *gin.Context, req interface{}) error {
	if c.Request.Method == "GET" {
		return requestbind.BindQueryCompat(c, req)
	}
	return c.ShouldBindJSON(req)
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles the request to list providers
func (h *Handler) List(c *gin.Context) {
	var req ProvidersListRequest
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

// Capabilities handles the request to get provider capabilities
func (h *Handler) Capabilities(c *gin.Context) {
	var req ProvidersCapabilitiesRequest
	if err := bindProviderRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Capabilities(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Descriptors handles the request to get provider descriptors
func (h *Handler) Descriptors(c *gin.Context) {
	var req ProvidersDescriptorsRequest
	if err := bindProviderRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Descriptors(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Detail handles the request to get provider details
func (h *Handler) Detail(c *gin.Context) {
	var req ProviderDetailRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	resp, err := h.service.Detail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Resources handles the request to get provider resources
func (h *Handler) Resources(c *gin.Context) {
	var req ProvidersResourcesRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	resp, err := h.service.Resources(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Delete handles the request to delete a provider
func (h *Handler) Delete(c *gin.Context) {
	var req ProviderActionRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	resp, err := h.service.Delete(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Reload handles the request to reload a provider
func (h *Handler) Reload(c *gin.Context) {
	var req ProviderActionRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	resp, err := h.service.Reload(c.Request.Context(), &req)
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

// SdkStats handles the request to get SDK language/version distribution
// across online provider sessions.
func (h *Handler) SdkStats(c *gin.Context) {
	resp, err := h.service.SdkStats(c.Request.Context(), &SdkStatsRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
