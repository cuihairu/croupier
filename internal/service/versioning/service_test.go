package versioning

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
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

func TestResourceProposalKey(t *testing.T) {
	assert.Equal(t, "resource:player", resourceProposalKey("player"))
	assert.Equal(t, "resource:inventory", resourceProposalKey("inventory"))
	assert.Equal(t, "resource:", resourceProposalKey(""))
	assert.Equal(t, "resource:  ", resourceProposalKey("  "))
}

func TestResourcePageKey(t *testing.T) {
	assert.Equal(t, "resource--player", resourcePageKey("player"))
	assert.Equal(t, "resource--inventory", resourcePageKey("inventory"))
	assert.Equal(t, "resource--", resourcePageKey(""))
	assert.Equal(t, "resource--", resourcePageKey("  "))
}

func TestMergeMessage(t *testing.T) {
	msg := mergeMessage(0, 0, false)
	assert.Equal(t, "no contract changes require merge", msg)

	msg2 := mergeMessage(1, 0, false)
	assert.Contains(t, msg2, "found 1 safe changes")

	msg3 := mergeMessage(0, 2, false)
	assert.Contains(t, msg3, "2 conflicts")

	msg4 := mergeMessage(1, 0, true)
	assert.Contains(t, msg4, "auto-merged 1")

	msg5 := mergeMessage(1, 1, true)
	assert.Contains(t, msg5, "auto-merged 1")
	assert.Contains(t, msg5, "1 conflicts")
}

func TestJsonString(t *testing.T) {
	assert.Equal(t, json.RawMessage(`"hello"`), jsonString("hello"))
	assert.Equal(t, json.RawMessage(`""`), jsonString(""))
	assert.Equal(t, json.RawMessage(`"test"`), jsonString("test"))
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "a", firstNonEmpty("a", "b", "c"))
	assert.Equal(t, "b", firstNonEmpty("", "b", "c"))
	assert.Equal(t, "c", firstNonEmpty("", "", "c"))
	assert.Empty(t, firstNonEmpty("", "", ""))
}

func TestSamePageSpec(t *testing.T) {
	// Test empty specs
	spec1 := spec.PageSpec{}
	spec2 := spec.PageSpec{}
	assert.True(t, samePageSpec(spec1, spec2))

	// Test with same page key
	spec3 := spec.PageSpec{PageKey: "test"}
	spec4 := spec.PageSpec{PageKey: "test"}
	assert.True(t, samePageSpec(spec3, spec4))

	// Test with different page key
	spec5 := spec.PageSpec{PageKey: "test1"}
	spec6 := spec.PageSpec{PageKey: "test2"}
	assert.False(t, samePageSpec(spec5, spec6))
}

func TestDigestRaw(t *testing.T) {
	digest1 := digestRaw([]byte("test"))
	digest2 := digestRaw([]byte("test"))
	assert.Equal(t, digest1, digest2)

	digest3 := digestRaw([]byte("different"))
	assert.NotEqual(t, digest1, digest3)
}
