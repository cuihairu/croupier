package audit

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

// GetAuditLogs retrieves audit logs with filtering and pagination
// Supports both GET (query params) and POST (JSON body) requests
func (h *Handler) GetAuditLogs(c *gin.Context) {
	var req AuditRequest

	// Try to bind from query parameters first (for GET requests)
	if err := c.ShouldBindQuery(&req); err != nil {
		// If that fails, try JSON body (for POST requests)
		if err := c.ShouldBindJSON(&req); err != nil {
			// If both fail, use default values
			req = AuditRequest{}
		}
	}

	// Use size as alias for pageSize (frontend compatibility)
	if req.Size > 0 && req.PageSize == 0 {
		req.PageSize = req.Size
	}
	// Set default page size if not specified
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	// Set default page if not specified
	if req.Page == 0 {
		req.Page = 1
	}

	resp, err := h.service.GetAuditLogs(c.Request.Context(), &req)
	if err != nil {
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, resp)
}
