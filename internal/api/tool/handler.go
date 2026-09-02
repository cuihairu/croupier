package tool

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

// NewHandler creates a tool handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /tools.
func (h *Handler) List(c *gin.Context) {
	var req ToolListRequest
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

// Create handles POST /tools.
func (h *Handler) Create(c *gin.Context) {
	var req ToolCreateRequest
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

// Update handles PUT /tools/:id.
func (h *Handler) Update(c *gin.Context) {
	var req ToolUpdateRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
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

// Delete handles DELETE /tools/:id.
func (h *Handler) Delete(c *gin.Context) {
	var req ToolDeleteRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := h.service.Delete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "删除成功"})
}
