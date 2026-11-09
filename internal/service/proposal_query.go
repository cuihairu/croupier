package service

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// ListProposals lists all proposals in a scope.
func (s *ProposalService) ListProposals(ctx context.Context, gameID, env string) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByScope(ctx, gameID, env)
}

// ListProposalDTOs lists proposals using stable API DTOs.
func (s *ProposalService) ListProposalDTOs(ctx context.Context, gameID, env string, filter ProposalListFilter) ([]ProposalDTO, error) {
	resourceKey := strings.TrimSpace(filter.ResourceKey)
	status := dbenum.ProposalStatus(-1)
	if raw := strings.TrimSpace(filter.Status); raw != "" {
		if parsed, err := dbenum.ParseProposalStatus(strings.ToLower(raw)); err == nil {
			status = parsed
		}
	}

	var proposals []*model.PageProposal
	var err error
	switch {
	case status >= 0 && resourceKey != "":
		proposals, err = s.proposalModel.ListByScopeStatusAndResourceKey(ctx, gameID, env, status, resourceKey)
	case status >= 0:
		proposals, err = s.proposalModel.ListByStatus(ctx, gameID, env, status)
	case resourceKey != "":
		proposals, err = s.proposalModel.ListByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	default:
		proposals, err = s.proposalModel.ListByScope(ctx, gameID, env)
	}
	if err != nil {
		return nil, err
	}

	items := make([]ProposalDTO, 0, len(proposals))
	for _, proposal := range proposals {
		item, err := proposalDTOFromModel(proposal)
		if err != nil {
			return nil, err
		}
		// 页面是否已物化：Inbox 需要据此把「发布」换成「去 Page Studio」。
		if _, err := s.pageModel.FindByScopeAndPageKey(ctx, gameID, env, proposal.PageKey); err == nil {
			item.PageExists = true
		}
		items = append(items, item)
	}
	return items, nil
}

// Inbox returns the canonical Page Studio queues:
// - publishable ready/basic proposals
// - needs_review proposals
// - blocked generation issues
// - stale draft/published pages caused by contract drift
func (s *ProposalService) Inbox(ctx context.Context, gameID, env string, filter ProposalListFilter) (ProposalInboxResponse, error) {
	proposals, err := s.ListProposalDTOs(ctx, gameID, env, filter)
	if err != nil {
		return ProposalInboxResponse{}, err
	}

	resp := ProposalInboxResponse{
		Publishable:     []ProposalDTO{},
		NeedsReview:     []ProposalDTO{},
		BlockedIssues:   []BlockedProposalIssueDTO{},
		ContractChanges: []ContractChangeDTO{},
	}
	for _, proposal := range proposals {
		if proposal.Status != "pending" {
			continue
		}
		if proposal.Quality == "ready" || proposal.Quality == "basic" {
			if !hasDiagnosticSeverity(proposal.Diagnostics, spec.SeverityError) {
				resp.Publishable = append(resp.Publishable, proposal)
				continue
			}
		}
		resp.NeedsReview = append(resp.NeedsReview, proposal)
	}

	blocked, err := s.listBlockedIssueDTOs(ctx, gameID, env, filter.ResourceKey)
	if err != nil {
		return ProposalInboxResponse{}, err
	}
	resp.BlockedIssues = blocked

	contractChanges, err := s.listContractChanges(ctx, gameID, env, filter.ResourceKey)
	if err != nil {
		return ProposalInboxResponse{}, err
	}
	resp.ContractChanges = contractChanges
	resp.Summary = ProposalInboxSummary{
		Publishable:     len(resp.Publishable),
		NeedsReview:     len(resp.NeedsReview),
		BlockedIssues:   len(resp.BlockedIssues),
		ContractChanges: len(resp.ContractChanges),
	}
	return resp, nil
}

// ListProposalsByStatus lists proposals by status.
func (s *ProposalService) ListProposalsByStatus(ctx context.Context, gameID, env string, status dbenum.ProposalStatus) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByStatus(ctx, gameID, env, status)
}

// GetProposal gets a proposal by key.
func (s *ProposalService) GetProposal(ctx context.Context, gameID, env, proposalKey string) (*model.PageProposal, error) {
	return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
}

// GetProposalDTO gets a proposal by key using the stable API DTO.
func (s *ProposalService) GetProposalDTO(ctx context.Context, gameID, env, proposalKey string) (ProposalDTO, error) {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return ProposalDTO{}, err
	}
	return proposalDTOFromModel(proposal)
}
