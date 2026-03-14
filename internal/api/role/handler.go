package role

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

// RoleCreate handles role creation requests.
func (h *Handler) RoleCreate(c *gin.Context) {
	var req RoleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RoleCreate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RoleDelete handles role deletion requests.
func (h *Handler) RoleDelete(c *gin.Context) {
	var req RoleDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.RoleDelete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// RoleDetail handles role detail requests.
func (h *Handler) RoleDetail(c *gin.Context) {
	var req RoleDetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RoleDetail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RoleUpdate handles role update requests.
func (h *Handler) RoleUpdate(c *gin.Context) {
	var req RoleUpdateRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RoleUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RolesList handles role list requests.
func (h *Handler) RolesList(c *gin.Context) {
	var req RolesListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RolesList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
