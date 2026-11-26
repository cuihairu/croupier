package model

import (
	"context"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestBlockedProposalIssueModel(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&BlockedProposalIssue{})
	require.NoError(t, err)

	model := NewBlockedProposalIssueModel(db)
	ctx := context.Background()

	// Create
	issue := &BlockedProposalIssue{
		GameID:      "bp-game-prod",
		Env:         "prod",
		ResourceKey: "player",
		FunctionID:  "player.ban",
		Status:      "open",
	}
	err = model.Create(ctx, issue)
	require.NoError(t, err)

	// List by scope
	issues, err := model.ListByScope(ctx, "bp-game-prod", "prod")
	require.NoError(t, err)
	assert.Len(t, issues, 1)

	// Find by scope and resource key
	found, err := model.FindByScopeAndResourceKey(ctx, "bp-game-prod", "prod", "player")
	require.NoError(t, err)
	assert.Equal(t, "player.ban", found.FunctionID)

	// List by scope and resource key
	issues, err = model.ListByScopeAndResourceKey(ctx, "bp-game-prod", "prod", "player")
	require.NoError(t, err)
	assert.Len(t, issues, 1)

	// Update status
	err = model.UpdateStatus(ctx, issue.ID, "resolved", "admin")
	require.NoError(t, err)

	// Resolve (already resolved, should be idempotent)
	err = model.Resolve(ctx, "bp-game-prod", "prod", "player", "player.ban", "admin")
	require.NoError(t, err)
}

// ===== OpenAPISource CRUD Tests =====

func TestPageProposalModel_UpsertAndFind(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&PageProposal{})
	require.NoError(t, err)

	model := NewPageProposalModel(db)
	ctx := context.Background()

	// Create proposal
	proposal := &PageProposal{
		GameID:      "pp-game1",
		Env:         "prod",
		ProposalKey: "player",
		PageKey:     "player-dashboard",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      dbenum.ProposalStatusPending,
	}
	err = model.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Find by scope and key
	found, err := model.FindByScopeAndKey(ctx, "pp-game1", "prod", "player")
	require.NoError(t, err)
	assert.Equal(t, "player-dashboard", found.PageKey)

	// Find by scope and page key
	found2, err := model.FindByScopeAndPageKey(ctx, "pp-game1", "prod", "player-dashboard")
	require.NoError(t, err)
	assert.Equal(t, "player", found2.ProposalKey)

	// List by scope
	proposals, err := model.ListByScope(ctx, "pp-game1", "prod")
	require.NoError(t, err)
	assert.Len(t, proposals, 1)

	// List by status
	proposals, err = model.ListByStatus(ctx, "pp-game1", "prod", dbenum.ProposalStatusPending)
	require.NoError(t, err)
	assert.Len(t, proposals, 1)

	// List by scope and resource key
	proposals, err = model.ListByScopeAndResourceKey(ctx, "pp-game1", "prod", "player")
	require.NoError(t, err)
	assert.Len(t, proposals, 1)

	// Upsert update
	proposal.Status = dbenum.ProposalStatusAccepted
	proposal.ID = found.ID
	err = model.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Verify update
	found3, err := model.FindByScopeAndKey(ctx, "pp-game1", "prod", "player")
	require.NoError(t, err)
	assert.Equal(t, dbenum.ProposalStatusAccepted, found3.Status)

	// ListByScopeStatusAndResourceKey
	proposals, err = model.ListByScopeStatusAndResourceKey(ctx, "pp-game1", "prod", dbenum.ProposalStatusAccepted, "player")
	require.NoError(t, err)
	assert.Len(t, proposals, 1)

	// DeleteByScopeAndKey
	err = model.DeleteByScopeAndKey(ctx, "pp-game1", "prod", "player")
	require.NoError(t, err)

	proposals, err = model.ListByScope(ctx, "pp-game1", "prod")
	require.NoError(t, err)
	assert.Len(t, proposals, 0)
}

// ===== PageProposalVersionModel Tests =====

func TestPageProposalVersionModel(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&PageProposal{}, &PageProposalVersion{})
	require.NoError(t, err)

	proposalModel := NewPageProposalModel(db)
	versionModel := NewPageProposalVersionModel(db)
	ctx := context.Background()

	// Create proposal
	proposal := &PageProposal{
		GameID:      "ppv-game1",
		Env:         "prod",
		ProposalKey: "item",
		PageKey:     "item-dashboard",
		Status:      dbenum.ProposalStatusPending,
	}
	err = proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Get next version
	nextVer, err := versionModel.GetNextVersion(ctx, proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, nextVer)

	// Create version
	ver := &PageProposalVersion{
		ProposalID: proposal.ID,
		Version:    1,
		Proposal:   JSON(`{"type":"resource"}`),
	}
	err = versionModel.CreateVersion(ctx, ver)
	require.NoError(t, err)

	// List versions
	versions, err := versionModel.ListByProposalID(ctx, proposal.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 1)

	// Find by proposal ID and version
	found, err := versionModel.FindByProposalIDAndVersion(ctx, proposal.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, found.Version)

	// Latest by proposal ID
	latest, err := versionModel.LatestByProposalID(ctx, proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, latest.Version)

	// Get next version again
	nextVer, err = versionModel.GetNextVersion(ctx, proposal.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, nextVer)
}

// ===== TaskRunModel Tests =====
