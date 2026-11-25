package node

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

// List handles the request to list nodes
func (h *Handler) List(c *gin.Context) {
	var req NodesListRequest
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

// GetMeta handles the request to get node metadata
func (h *Handler) GetMeta(c *gin.Context) {
	var req NodeMetaRequest
	_ = c.ShouldBindUri(&req) // uri fields are all strings with no required: binding cannot fail, keep populate-only semantics

	resp, err := h.service.GetMeta(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UpdateMeta handles the request to update node metadata
func (h *Handler) UpdateMeta(c *gin.Context) {
	var req NodeMetaUpdateRequest
	_ = c.ShouldBindUri(&req) // uri fields are all strings with no required: binding cannot fail, keep populate-only semantics
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.UpdateMeta(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Drain handles the request to drain a node
func (h *Handler) Drain(c *gin.Context) {
	var req NodeDrainRequest
	_ = c.ShouldBindUri(&req) // uri fields are all strings with no required: binding cannot fail, keep populate-only semantics
	// timeout lives in an optional body and is only bound when a request body
	// exists (legacy callers with an empty body — scripts/other services — can
	// still drain without a body; historically not binding it silently ignored
	// the timeout parameter).
	if c.Request != nil && c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}
	}

	if err := h.service.Drain(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// Undrain handles the request to undrain a node
func (h *Handler) Undrain(c *gin.Context) {
	var req NodeActionRequest
	_ = c.ShouldBindUri(&req) // uri fields are all strings with no required: binding cannot fail, keep populate-only semantics

	if err := h.service.Undrain(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// Restart handles the request to restart a node
func (h *Handler) Restart(c *gin.Context) {
	var req NodeActionRequest
	_ = c.ShouldBindUri(&req) // uri fields are all strings with no required: binding cannot fail, keep populate-only semantics

	if err := h.service.Restart(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// ListCommands handles the request to list node commands
func (h *Handler) ListCommands(c *gin.Context) {
	var req NodeCommandsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.ListCommands(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ListCronJobs reads the cron jobs of the host the node runs on (crontab +
// /etc/cron.d).
func (h *Handler) ListCronJobs(c *gin.Context) {
	id := c.Param("id")
	jobs, err := h.service.ListCronJobs(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"items": jobs, "total": len(jobs)})
}

// ListAllCronJobs aggregates the host cron jobs of all nodes (#24): the data
// source /ops/schedules groups and displays by source (platform/host).
func (h *Handler) ListAllCronJobs(c *gin.Context) {
	reports, err := h.service.ListAllCronJobs(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"items": reports, "total": len(reports)})
}

// Commands alias for route compatibility
func (h *Handler) Commands(c *gin.Context) {
	h.ListCommands(c)
}
