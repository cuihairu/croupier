package schema

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

// List handles the request to list schemas
func (h *Handler) List(c *gin.Context) {
	var req ListRequest
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

// Create handles the request to create a schema
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
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

// Get handles the request to get schema details
func (h *Handler) Get(c *gin.Context) {
	var req GetRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	resp, err := h.service.Get(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Update handles the request to update a schema
func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
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

// Delete handles the request to delete a schema
func (h *Handler) Delete(c *gin.Context) {
	var req DeleteRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	if err := h.service.Delete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// Validate handles the request to validate data against a schema
func (h *Handler) Validate(c *gin.Context) {
	var req ValidateRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
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

// RawValidate handles the request to validate data against a raw schema
func (h *Handler) RawValidate(c *gin.Context) {
	var req RawValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RawValidate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetUIConfig handles the request to get schema UI config
func (h *Handler) GetUIConfig(c *gin.Context) {
	var req GetUIConfigRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义

	resp, err := h.service.GetUIConfig(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UpdateUIConfig handles the request to update schema UI config
func (h *Handler) UpdateUIConfig(c *gin.Context) {
	var req UpdateUIConfigRequest
	_ = c.ShouldBindUri(&req) // uri 字段均为 string 且无 required：绑定不会失败，保留填充语义
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.UpdateUIConfig(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
