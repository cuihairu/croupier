package assignment

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// AssignmentService defines the interface for assignment service operations
type AssignmentService interface {
	List(ctx context.Context, req *AssignmentsListRequest) (*AssignmentsListResponse, error)
	History(ctx context.Context, req *AssignmentsHistoryRequest) (*AssignmentsHistoryResponse, error)
	Update(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error)
}

type Handler struct {
	service AssignmentService
}

func NewHandler(service AssignmentService) *Handler {
	return &Handler{service: service}
}

// List handles the request to list assignments
func (h *Handler) List(c *gin.Context) {
	var req AssignmentsListRequest
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

// History handles the request to list assignment history
func (h *Handler) History(c *gin.Context) {
	var req AssignmentsHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.History(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Update handles the request to update assignments
func (h *Handler) Update(c *gin.Context) {
	var req AssignmentsUpdateRequest
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
