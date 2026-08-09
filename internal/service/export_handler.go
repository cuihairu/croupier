package service

import (
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// ExportHandler provides HTTP handlers for data export API.
type ExportHandler struct {
	service *DataExportService
}

// NewExportHandler creates a new handler.
func NewExportHandler(service *DataExportService) *ExportHandler {
	return &ExportHandler{service: service}
}

// ExportPages handles GET /api/export/pages
func (h *ExportHandler) ExportPages(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())

	data, err := h.service.ExportToJSON(c.Request.Context(), scope.GameID, scope.Env)
	if err != nil {
		response.Error(c, err)
		return
	}

	// Set headers for file download
	filename := fmt.Sprintf("page-export-%s-%s-%s.json", scope.GameID, scope.Env, time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	c.Data(200, "application/json", data)
}
