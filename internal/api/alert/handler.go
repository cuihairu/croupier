package alert

import (
	"strconv"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles the request to list alerts
func (h *Handler) List(c *gin.Context) {
	var req AlertsListRequest
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

// Silence handles the request to silence an alert
func (h *Handler) Silence(c *gin.Context) {
	var req AlertSilenceRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.Silence(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// SilencesList handles the request to list silence rules
func (h *Handler) SilencesList(c *gin.Context) {
	var req SilencesListRequest
	// No parameters to bind

	resp, err := h.service.SilencesList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// SilenceDelete handles the request to delete a silence rule
func (h *Handler) SilenceDelete(c *gin.Context) {
	var req SilenceDeleteRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.SilenceDelete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// RulesList handles GET /alerts/rules.
func (h *Handler) RulesList(c *gin.Context) {
	var req RulesListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.RulesList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RulesCreate handles POST /alerts/rules.
func (h *Handler) RulesCreate(c *gin.Context) {
	var req RuleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.RulesCreate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RulesUpdate handles PUT /alerts/rules/:id.
func (h *Handler) RulesUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "无效的规则 ID")
		return
	}
	var req RuleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.RulesUpdate(c.Request.Context(), uint(id), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RulesDelete handles DELETE /alerts/rules/:id.
func (h *Handler) RulesDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "无效的规则 ID")
		return
	}
	if err := h.service.RulesDelete(c.Request.Context(), uint(id)); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
