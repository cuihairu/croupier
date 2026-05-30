// Package policy provides API handlers for function policy management.
package policy

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	manager *policy.Manager
}

func NewHandler(manager *policy.Manager) *Handler {
	return &Handler{manager: manager}
}

// GetPolicy retrieves the effective policy for a function.
// GET /api/v1/functions/:function_id/policy
func (h *Handler) GetPolicy(c *gin.Context) {
	functionID := c.Param("function_id")
	if functionID == "" {
		response.BadRequest(c, "function_id is required")
		return
	}

	riskLevel := c.DefaultQuery("risk_level", "")

	policy, err := h.manager.GetPolicy(c.Request.Context(), functionID, policy.RiskLevel(riskLevel))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, policy)
}

// SetPolicy sets or updates a database override policy for a function.
// PUT /api/v1/functions/:function_id/policy
func (h *Handler) SetPolicy(c *gin.Context) {
	functionID := c.Param("function_id")
	if functionID == "" {
		response.BadRequest(c, "function_id is required")
		return
	}

	var req policy.Policy
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	req.FunctionID = functionID

	if err := h.manager.SetOverride(c.Request.Context(), functionID, &req); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, req)
}

// DeletePolicy removes the database override for a function.
// After deletion, the function will use the default risk-based policy.
// DELETE /api/v1/functions/:function_id/policy
func (h *Handler) DeletePolicy(c *gin.Context) {
	functionID := c.Param("function_id")
	if functionID == "" {
		response.BadRequest(c, "function_id is required")
		return
	}

	if err := h.manager.DeleteOverride(c.Request.Context(), functionID); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Policy deleted, using default risk-based policy"})
}

// ListOverrides returns all manual policy overrides.
// GET /api/v1/policies/overrides
func (h *Handler) ListOverrides(c *gin.Context) {
	policies, err := h.manager.ListOverrides(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"policies": policies})
}

// ReloadConfig reloads the default policy configuration from file.
// POST /api/v1/policies/reload
func (h *Handler) ReloadConfig(c *gin.Context) {
	if err := h.manager.ReloadConfig(); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Configuration reloaded"})
}

// GetDefaultPolicies returns the current default policies for all risk levels.
// GET /api/v1/policies/defaults
func (h *Handler) GetDefaultPolicies(c *gin.Context) {
	policies := gin.H{
		"low":    h.manager.GetDefaultPolicy(policy.RiskLow),
		"medium": h.manager.GetDefaultPolicy(policy.RiskMedium),
		"high":   h.manager.GetDefaultPolicy(policy.RiskHigh),
		"danger": h.manager.GetDefaultPolicy(policy.RiskDanger),
	}
	response.Success(c, policies)
}

// Request/Response types

type SetPolicyRequest struct {
	RequireApproval  bool     `json:"require_approval"`
	ApprovalWorkflow string   `json:"approval_workflow"`
	RequireAudit     bool     `json:"require_audit"`
	AllowedRoles     []string `json:"allowed_roles"`
}
