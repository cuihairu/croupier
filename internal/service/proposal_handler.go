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
	gameID, env := svc.GameScopeFromContext(c.Request.Context())
	resp, err := h.service.ListProposalDTOs(c.Request.Context(), gameID, env, ProposalListFilter{
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
	gameID, env := svc.GameScopeFromContext(c.Request.Context())
	resp, err := h.service.Inbox(c.Request.Context(), gameID, env, ProposalListFilter{
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
	gameID, env := svc.GameScopeFromContext(c.Request.Context())

	resp, err := h.service.GetProposalDTO(c.Request.Context(), gameID, env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// AcceptProposal handles POST /api/proposals/:proposalKey/accept
func (h *ProposalHandler) AcceptProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	gameID, env := svc.GameScopeFromContext(c.Request.Context())

	err := h.service.AcceptProposal(c.Request.Context(), gameID, env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "proposal accepted"})
}

// AcceptAndPublishProposal handles POST /api/proposals/:proposalKey/accept-and-publish
func (h *ProposalHandler) AcceptAndPublishProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	gameID, env := svc.GameScopeFromContext(c.Request.Context())

	resp, err := h.service.AcceptAndPublishProposal(c.Request.Context(), gameID, env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// RejectProposal handles POST /api/proposals/:proposalKey/reject
func (h *ProposalHandler) RejectProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	gameID, env := svc.GameScopeFromContext(c.Request.Context())

	err := h.service.RejectProposal(c.Request.Context(), gameID, env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "proposal rejected"})
}
