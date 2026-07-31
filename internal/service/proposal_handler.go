package service

import (
	"github.com/cuihairu/croupier/internal/common/response"
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
	gameID := c.GetString("game_id")
	env := c.GetString("env")
	status := c.Query("status")

	var resp interface{}
	var err error

	if status != "" {
		resp, err = h.service.ListProposalsByStatus(c.Request.Context(), gameID, env, status)
	} else {
		resp, err = h.service.ListProposals(c.Request.Context(), gameID, env)
	}

	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// GetProposal handles GET /api/proposals/:proposalKey
func (h *ProposalHandler) GetProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	gameID := c.GetString("game_id")
	env := c.GetString("env")

	resp, err := h.service.GetProposal(c.Request.Context(), gameID, env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, resp)
}

// AcceptProposal handles POST /api/proposals/:proposalKey/accept
func (h *ProposalHandler) AcceptProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	gameID := c.GetString("game_id")
	env := c.GetString("env")

	err := h.service.AcceptProposal(c.Request.Context(), gameID, env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "proposal accepted"})
}

// RejectProposal handles POST /api/proposals/:proposalKey/reject
func (h *ProposalHandler) RejectProposal(c *gin.Context) {
	proposalKey := c.Param("proposalKey")
	gameID := c.GetString("game_id")
	env := c.GetString("env")

	err := h.service.RejectProposal(c.Request.Context(), gameID, env, proposalKey)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{"message": "proposal rejected"})
}
