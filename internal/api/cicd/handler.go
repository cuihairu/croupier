package cicd

import (
	"strconv"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

// NewHandler creates a cicd admin handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func pathID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// List handles GET /cicd/integrations.
func (h *Handler) List(c *gin.Context) {
	var req IntegrationListRequest
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

// Create handles POST /cicd/integrations.
func (h *Handler) Create(c *gin.Context) {
	var req IntegrationCreateRequest
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

// Update handles PUT /cicd/integrations/:id.
func (h *Handler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		response.Error(c, errBadPathID)
		return
	}
	var req IntegrationUpdateRequest
	req.ID = id
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

// Delete handles DELETE /cicd/integrations/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		response.Error(c, errBadPathID)
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "删除成功"})
}

// Test handles POST /cicd/integrations/:id/test.
func (h *Handler) Test(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		response.Error(c, errBadPathID)
		return
	}
	resp, err := h.service.TestConnection(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Trigger handles POST /cicd/integrations/:id/trigger.
func (h *Handler) Trigger(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		response.Error(c, errBadPathID)
		return
	}
	var req TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Trigger(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Builds handles GET /cicd/builds（跨接入构建列表，供打包记录页）。
func (h *Handler) Builds(c *gin.Context) {
	var req BuildListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.ListBuilds(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RefreshBuild handles POST /cicd/builds/:id/refresh.
func (h *Handler) RefreshBuild(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, errBadPathID)
		return
	}
	resp, err := h.service.RefreshBuild(c.Request.Context(), uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
