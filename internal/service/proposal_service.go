package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// ProposalService manages page proposals.
type ProposalService struct {
	db            *gorm.DB
	proposalModel *model.PageProposalModel
}

// NewProposalService creates the service.
func NewProposalService(db *gorm.DB) *ProposalService {
	return &ProposalService{
		db:            db,
		proposalModel: model.NewPageProposalModel(db),
	}
}

// ListProposals lists all proposals in a scope.
func (s *ProposalService) ListProposals(ctx context.Context, gameID, env string) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByScope(ctx, gameID, env)
}

// ListProposalsByStatus lists proposals by status.
func (s *ProposalService) ListProposalsByStatus(ctx context.Context, gameID, env, status string) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByStatus(ctx, gameID, env, status)
}

// GetProposal gets a proposal by key.
func (s *ProposalService) GetProposal(ctx context.Context, gameID, env, proposalKey string) (*model.PageProposal, error) {
	return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
}

// AcceptProposal accepts a proposal and creates a draft.
// Blocked proposals cannot be accepted - they require manual resolution.
func (s *ProposalService) AcceptProposal(ctx context.Context, gameID, env, proposalKey string) error {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != "pending" {
		return fmt.Errorf("proposal is not pending")
	}

	// Block acceptance if quality is blocked
	if proposal.Quality == "blocked" {
		return fmt.Errorf("proposal is blocked due to semantic conflicts or validation errors; resolve issues before accepting")
	}

	// Block acceptance if there are error-level diagnostics
	if hasBlockingDiagnostics(proposal.Diagnostics) {
		return fmt.Errorf("proposal has blocking diagnostics; resolve errors before accepting")
	}

	// Update status to accepted
	proposal.Status = "accepted"
	return s.proposalModel.UpsertProposal(ctx, proposal)
}

// RejectProposal rejects a proposal.
func (s *ProposalService) RejectProposal(ctx context.Context, gameID, env, proposalKey string) error {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != "pending" {
		return fmt.Errorf("proposal is not pending")
	}

	// Update status to rejected
	proposal.Status = "rejected"
	return s.proposalModel.UpsertProposal(ctx, proposal)
}

// hasBlockingDiagnostics checks if diagnostics contain error-level items.
func hasBlockingDiagnostics(diagnosticsJSON []byte) bool {
	if len(diagnosticsJSON) == 0 {
		return false
	}

	var diagnostics []struct {
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal(diagnosticsJSON, &diagnostics); err != nil {
		return false
	}

	for _, d := range diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}
