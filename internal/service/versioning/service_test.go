package versioning

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
	)
	require.NoError(t, err)

	return db
}

func TestVersioningService_GetChangeChain(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create some contracts
	contractModel := model.NewFunctionContractModel(db)
	err := contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.list",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  "collection_query",
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Get change chain
	chain, err := service.GetChangeChain(ctx, &GetChangeChainRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.NotNil(t, chain)
	assert.Equal(t, "player", chain.ResourceKey)
	assert.Len(t, chain.Items, 1)
}

func TestVersioningService_Diff(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create semantics
	semanticsModel := model.NewCapabilitySemanticsModel(db)
	err := semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:        "demo-game",
		Env:           "development",
		ResourceKey:   "player",
		IdentityField: "player_id",
	})
	require.NoError(t, err)

	// Get diff
	diff, err := service.Diff(ctx, &DiffRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		FromVersion: 1,
		ToVersion:   1,
	})
	require.NoError(t, err)
	assert.NotNil(t, diff)
	assert.Contains(t, diff.Summary, "changes")
}

func TestVersioningService_Merge(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create semantics first
	semanticsModel := model.NewCapabilitySemanticsModel(db)
	err := semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
	})
	require.NoError(t, err)

	// Test auto merge
	result, err := service.Merge(ctx, &MergeRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Strategy:    MergeStrategyAuto,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Message, "auto-merged")

	// Test reject merge
	result, err = service.Merge(ctx, &MergeRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Strategy:    MergeStrategyReject,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "all changes rejected", result.Message)
}

func TestVersioningService_RollbackDraft(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Rollback draft
	result, err := service.RollbackDraft(ctx, &RollbackRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Version:     1,
		Reason:      "test rollback",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Message, "rolled back")
}

func TestVersioningService_RollbackPublish(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Rollback publish
	result, err := service.RollbackPublish(ctx, &RollbackRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Version:     1,
		Reason:      "test rollback",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Message, "rolled back")
}

func TestVersioningService_RegenerateProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create semantics and contracts first
	semanticsModel := model.NewCapabilitySemanticsModel(db)
	err := semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
	})
	require.NoError(t, err)

	// Create contract
	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.list",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  "collection_query",
	})
	require.NoError(t, err)

	// Regenerate proposal
	result, err := service.RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Message, "proposal regenerated")
}

func TestVersioningService_Republish(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create proposal first
	proposalModel := model.NewPageProposalModel(db)
	err := proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "accepted",
	})
	require.NoError(t, err)

	// Republish
	result, err := service.Republish(ctx, &RepublishRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Reason:      "test republish",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.Version)
}
