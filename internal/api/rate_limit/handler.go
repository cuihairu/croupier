package rate_limit

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

// List handles the request to list rate limits
func (h *Handler) List(c *gin.Context) {
	var req RateLimitsListRequest
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

// Get handles the request to get a rate limit
func (h *Handler) Get(c *gin.Context) {
	var req RateLimitGetRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Get(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Upsert handles the request to upsert a rate limit
func (h *Handler) Upsert(c *gin.Context) {
	var req RateLimitUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Upsert(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Delete handles the request to delete a rate limit
func (h *Handler) Delete(c *gin.Context) {
	var req RateLimitDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// Preview handles the request to preview rate limit impact
func (h *Handler) Preview(c *gin.Context) {
	var req RateLimitPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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
