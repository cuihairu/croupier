package service

import (
	"context"
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
func (s *ProposalService) AcceptProposal(ctx context.Context, gameID, env, proposalKey string) error {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != "pending" {
		return fmt.Errorf("proposal is not pending")
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
