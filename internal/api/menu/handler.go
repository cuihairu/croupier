package menu

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

// List handles GET /api/v1/menus (full menu tree of the scope).
func (h *Handler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Create handles POST /api/v1/menus.
func (h *Handler) Create(c *gin.Context) {
	var req CreateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, resp)
}

// Update handles PUT /api/v1/menus/:id.
func (h *Handler) Update(c *gin.Context) {
	var req UpdateMenuRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required 语义失败路径，保留填充语义
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

// Delete handles DELETE /api/v1/menus/:id.
func (h *Handler) Delete(c *gin.Context) {
	var req UpdateMenuRequest
	_ = c.ShouldBindUri(&req)
	if err := h.service.Delete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// UpdateSort handles PUT /api/v1/menus/:id/sort.
func (h *Handler) UpdateSort(c *gin.Context) {
	var req SortMenuRequest
	_ = c.ShouldBindUri(&req)
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.UpdateSort(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
