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
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
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

func TestVersioningService_MergeReject(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Test reject merge (should work without any data)
	result, err := service.Merge(ctx, &MergeRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Strategy:    MergeStrategyReject,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "all changes rejected", result.Message)
}

func TestVersioningService_MergeAutoNoDraft(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Test auto merge without draft should fail
	_, err := service.Merge(ctx, &MergeRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Strategy:    MergeStrategyAuto,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page draft not found")
}

func TestVersioningService_RollbackDraftNoData(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Rollback draft without data should fail
	_, err := service.RollbackDraft(ctx, &RollbackRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Version:     1,
		Reason:      "test rollback",
	})
	assert.Error(t, err)
}

func TestVersioningService_RollbackPublishNoData(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Rollback publish without data should fail
	_, err := service.RollbackPublish(ctx, &RollbackRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Version:     1,
		Reason:      "test rollback",
	})
	assert.Error(t, err)
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

	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
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

	// Regenerate proposal
	result, err := service.RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Message, "regenerated")
}

func TestVersioningService_RepublishNoDraft(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Republish without draft should fail
	_, err := service.Republish(ctx, &RepublishRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Reason:      "test republish",
	})
	assert.Error(t, err)
}
