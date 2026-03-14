package certificate

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

// List handles the request to list certificates
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

// Add handles the request to add a certificate
func (h *Handler) Add(c *gin.Context) {
	var req AddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Add(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Get handles the request to get certificate details
func (h *Handler) Get(c *gin.Context) {
	var req GetRequest
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

// Check handles the request to check a certificate
func (h *Handler) Check(c *gin.Context) {
	var req CheckRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Check(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Delete handles the request to delete a certificate
func (h *Handler) Delete(c *gin.Context) {
	var req DeleteRequest
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

// Stats handles the request to get certificate statistics
func (h *Handler) Stats(c *gin.Context) {
	resp, err := h.service.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// AddAlert handles the request to add a certificate alert
func (h *Handler) AddAlert(c *gin.Context) {
	var req AddAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.AddAlert(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ListAlerts handles the request to list certificate alerts
func (h *Handler) ListAlerts(c *gin.Context) {
	var req ListAlertsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.ListAlerts(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CheckAll handles the request to check all certificates
func (h *Handler) CheckAll(c *gin.Context) {
	resp, err := h.service.CheckAll(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetDomainInfo handles the request to get domain certificate info
func (h *Handler) GetDomainInfo(c *gin.Context) {
	var req DomainInfoRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.GetDomainInfo(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// GetExpiring handles the request to get expiring certificates
func (h *Handler) GetExpiring(c *gin.Context) {
	var req ExpiringRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.GetExpiring(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Alias methods for route compatibility

func (h *Handler) AlertsList(c *gin.Context) {
	h.ListAlerts(c)
}

func (h *Handler) DomainInfo(c *gin.Context) {
	h.GetDomainInfo(c)
}

func (h *Handler) Expiring(c *gin.Context) {
	h.GetExpiring(c)
}
