package extension

import (
	"strconv"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CatalogList(c *gin.Context) {
	var req ExtensionCatalogListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.CatalogList(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CatalogDetail(c *gin.Context) {
	resp, err := h.service.CatalogDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CatalogReleases(c *gin.Context) {
	resp, err := h.service.CatalogReleases(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Install(c *gin.Context) {
	var req ExtensionInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Install(c.Request.Context(), req, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) InstallationList(c *gin.Context) {
	var req ExtensionInstallationListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.InstallationList(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) InstallationDetail(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.InstallationDetail(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req ExtensionConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.UpdateConfig(c.Request.Context(), id, req, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ConfigSchema(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.ConfigSchema(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Config(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Config(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) TestConnection(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.TestConnection(c.Request.Context(), id, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Capabilities(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Capabilities(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Pages(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Pages(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) HealthCheck(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.HealthCheck(c.Request.Context(), id, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Enable(c *gin.Context) {
	h.action(c, func(id uint) (any, error) { return h.service.Enable(c.Request.Context(), id, h.operator(c)) })
}

func (h *Handler) Disable(c *gin.Context) {
	h.action(c, func(id uint) (any, error) { return h.service.Disable(c.Request.Context(), id, h.operator(c)) })
}

func (h *Handler) Upgrade(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req ExtensionUpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Upgrade(c.Request.Context(), id, req.ReleaseVersion, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Reconcile(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Reconcile(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) Uninstall(c *gin.Context) {
	h.action(c, func(id uint) (any, error) { return h.service.Uninstall(c.Request.Context(), id, h.operator(c)) })
}

func (h *Handler) Events(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req ExtensionEventListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Events(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AgentSyncPayload(c *gin.Context) {
	agentID := c.Param("agentId")
	h.respondAgentSyncPayload(c, agentID)
}

func (h *Handler) AgentExtensions(c *gin.Context) {
	agentID := c.Param("id")
	h.respondAgentSyncPayload(c, agentID)
}

func (h *Handler) AgentExtensionsSync(c *gin.Context) {
	agentID := c.Param("id")
	h.respondAgentSyncPayload(c, agentID)
}

func (h *Handler) CompatConfigSchema(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.ConfigSchema(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatConfig(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Config(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatUpdateConfig(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var req ExtensionConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.UpdateConfig(c.Request.Context(), id, req, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatTestConnection(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.TestConnection(c.Request.Context(), id, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatCapabilities(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Capabilities(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatPages(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Pages(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatHealthCheck(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.HealthCheck(c.Request.Context(), id, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatEnable(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Enable(c.Request.Context(), id, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatDisable(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Disable(c.Request.Context(), id, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatUpgrade(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var req ExtensionUpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Upgrade(c.Request.Context(), id, req.ReleaseVersion, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatReconcile(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Reconcile(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatUninstall(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Uninstall(c.Request.Context(), id, h.operator(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CompatEvents(c *gin.Context) {
	id, err := h.resolveCompatInstallationID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var req ExtensionEventListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.Events(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) respondAgentSyncPayload(c *gin.Context, agentID string) {
	resp, err := h.service.AgentSyncPayload(c.Request.Context(), agentID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) resolveCompatInstallationID(c *gin.Context) (uint, error) {
	return h.service.ResolveInstallationID(c.Request.Context(), c.Param("id"))
}

func (h *Handler) operator(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "system"
}

func (h *Handler) action(c *gin.Context, fn func(id uint) (any, error)) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := fn(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func parseUintParam(c *gin.Context, key string) (uint, error) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		return 0, errorx.NewBadRequest("invalid path parameter: " + key)
	}
	return uint(value), nil
}
