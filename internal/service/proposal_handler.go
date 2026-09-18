package service

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// ProposalHandler provides HTTP handlers for Proposal API.
type ProposalHandler struct {
	service *ProposalService
}

// NewProposalHandler creates a new handler.
func NewProposalHandler(service *ProposalService) *ProposalHandler {
	return &ProposalHandler{service: service}
}

// ListProposals handles GET /api/proposals
func (h *ProposalHandler) ListProposals(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	resp, err := h.service.ListProposalDTOs(c.Request.Context(), scope.GameID, scope.Env, ProposalListFilter{
		Status:      c.Query("status"),
		ResourceKey: c.Query("resourceKey"),
	})

	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// Inbox handles GET /api/proposals/inbox.
func (h *ProposalHandler) Inbox(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	resp, err := h.service.Inbox(c.Request.Context(), scope.GameID, scope.Env, ProposalListFilter{
		ResourceKey: c.Query("resourceKey"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// GetProposal handles GET /api/proposals/:proposalKey
func (h *ProposalHandler) GetProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	scope := svc.GameScopeFromContext(c.Request.Context())

	resp, err := h.service.GetProposalDTO(c.Request.Context(), scope.GameID, scope.Env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// AcceptProposal handles POST /api/proposals/:proposalKey/accept
// 发布分级（T10 扩展）：auto env 下接续发布结果随响应带回——
// published 恒存在；publishError 仅失败时出现（对齐 versioning composite
// 响应模式，前端按字段分支提示）。
func (h *ProposalHandler) AcceptProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	scope := svc.GameScopeFromContext(c.Request.Context())

	outcome, err := h.service.AcceptProposal(c.Request.Context(), scope.GameID, scope.Env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	payload := gin.H{"message": "proposal accepted", "published": outcome.Published}
	if outcome.PublishError != "" {
		payload["publishError"] = outcome.PublishError
	}
	response.Success(c, payload)
}

// AcceptAndPublishProposal handles POST /api/proposals/:proposalKey/accept-and-publish
func (h *ProposalHandler) AcceptAndPublishProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	scope := svc.GameScopeFromContext(c.Request.Context())

	resp, err := h.service.AcceptAndPublishProposal(c.Request.Context(), scope.GameID, scope.Env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// RejectProposal handles POST /api/proposals/:proposalKey/reject
func (h *ProposalHandler) RejectProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	scope := svc.GameScopeFromContext(c.Request.Context())

	err := h.service.RejectProposal(c.Request.Context(), scope.GameID, scope.Env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "proposal rejected"})
}
