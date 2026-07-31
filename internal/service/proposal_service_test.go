package service

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProposalService_ListProposals(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create proposals
	proposals := []*model.PageProposal{
		{
			GameID:      "demo-game",
			Env:         "development",
			ProposalKey: "player.manage",
			PageKey:     "player.manage",
			PageType:    "resource",
			ResourceKey: "player",
			Quality:     "ready",
			Status:      "pending",
		},
		{
			GameID:      "demo-game",
			Env:         "development",
			ProposalKey: "mail.send",
			PageKey:     "mail.send",
			PageType:    "operation",
			ResourceKey: "mail",
			Quality:     "basic",
			Status:      "pending",
		},
	}

	for _, p := range proposals {
		err := service.proposalModel.UpsertProposal(ctx, p)
		require.NoError(t, err)
	}

	// List all proposals
	result, err := service.ListProposals(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// List by status
	pending, err := service.ListProposalsByStatus(ctx, "demo-game", "development", "pending")
	require.NoError(t, err)
	assert.Len(t, pending, 2)

	accepted, err := service.ListProposalsByStatus(ctx, "demo-game", "development", "accepted")
	require.NoError(t, err)
	assert.Len(t, accepted, 0)
}

func TestProposalService_AcceptProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create proposal
	proposal := &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "player.manage",
		PageKey:     "player.manage",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
	}
	err := service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Accept proposal
	err = service.AcceptProposal(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)

	// Verify status changed
	result, err := service.GetProposal(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, "accepted", result.Status)
}

func TestProposalService_AcceptBlockedProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create blocked proposal
	proposal := &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "player.manage",
		PageKey:     "player.manage",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "blocked",
		Status:      "pending",
	}
	err := service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Try to accept blocked proposal
	err = service.AcceptProposal(ctx, "demo-game", "development", "player.manage")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func TestProposalService_RejectProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create proposal
	proposal := &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "player.manage",
		PageKey:     "player.manage",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
	}
	err := service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Reject proposal
	err = service.RejectProposal(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)

	// Verify status changed
	result, err := service.GetProposal(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, "rejected", result.Status)
}

func TestProposalService_ScopeIsolation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create proposal in scope 1
	proposal := &model.PageProposal{
		GameID:      "game-1",
		Env:         "prod",
		ProposalKey: "player.manage",
		PageKey:     "player.manage",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
	}
	err := service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Verify scope 1 has the proposal
	result1, err := service.ListProposals(ctx, "game-1", "prod")
	require.NoError(t, err)
	assert.Len(t, result1, 1)

	// Verify scope 2 has no proposals
	result2, err := service.ListProposals(ctx, "game-2", "prod")
	require.NoError(t, err)
	assert.Len(t, result2, 0)
}
