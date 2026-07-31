package service

import (
	"context"
	"testing"

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

func TestContractService_RebuildContractFromFunctionMeta(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	// Test rebuilding contract
	meta := FunctionMetaInput{
		ID:           "player.ban",
		Version:      "1.0.0",
		Enabled:      true,
		Summary:      "Ban a player",
		Description:  "Ban a player from the game",
		InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
		Resource:     "player",
		Operation:    "ban",
		Capability:   "action",
		Execution:    "sync",
		Risk:         "high",
		Permission:   "player.ban.invoke",
		Tags:         []string{"player", "admin"},
	}

	err := service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta)
	require.NoError(t, err)

	// Verify contract was created
	contract, err := service.GetContract(ctx, "demo-game", "development", "player.ban")
	require.NoError(t, err)
	assert.Equal(t, "player.ban", contract.FunctionID)
	assert.Equal(t, "1.0.0", contract.Version)
	assert.Equal(t, "player", contract.ResourceKey)
	assert.Equal(t, "ban", contract.OperationKey)
	assert.Equal(t, "action", contract.Capability)
	assert.Equal(t, "sync", contract.Execution)
	assert.Equal(t, "high", contract.Risk)
	assert.Equal(t, "sdk", contract.Source)
}

func TestContractService_RebuildResourceCapability(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	// Create some contracts first
	contracts := []FunctionMetaInput{
		{
			ID:         "player.list",
			Version:    "1.0.0",
			Enabled:    true,
			Resource:   "player",
			Operation:  "list",
			Capability: "collection_query",
			Execution:  "sync",
		},
		{
			ID:         "player.get",
			Version:    "1.0.0",
			Enabled:    true,
			Resource:   "player",
			Operation:  "get",
			Capability: "item_query",
			Execution:  "sync",
		},
		{
			ID:         "player.create",
			Version:    "1.0.0",
			Enabled:    true,
			Resource:   "player",
			Operation:  "create",
			Capability: "create",
			Execution:  "sync",
		},
	}

	for _, meta := range contracts {
		err := service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta)
		require.NoError(t, err)
	}

	// Rebuild resource capability
	err := service.RebuildResourceCapability(ctx, "demo-game", "development", "player")
	require.NoError(t, err)

	// Verify resource capability was created
	cap, err := service.ListResourceCapabilities(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, cap, 1)
	assert.Equal(t, "player", cap[0].ResourceKey)
}

func TestContractService_ListContracts(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	// Create contracts
	meta1 := FunctionMetaInput{
		ID:         "player.list",
		Version:    "1.0.0",
		Enabled:    true,
		Resource:   "player",
		Operation:  "list",
		Capability: "collection_query",
	}
	meta2 := FunctionMetaInput{
		ID:         "mail.send",
		Version:    "1.0.0",
		Enabled:    true,
		Resource:   "mail",
		Operation:  "send",
		Capability: "action",
	}

	err := service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta1)
	require.NoError(t, err)
	err = service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta2)
	require.NoError(t, err)

	// List all contracts
	contracts, err := service.ListContracts(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, contracts, 2)

	// List by resource
	playerContracts, err := service.contractModel.ListByResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	assert.Len(t, playerContracts, 1)
	assert.Equal(t, "player.list", playerContracts[0].FunctionID)
}

func TestContractService_RebuildProposalsForResource(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	// Create contract
	meta := FunctionMetaInput{
		ID:         "player.list",
		Version:    "1.0.0",
		Enabled:    true,
		Resource:   "player",
		Operation:  "list",
		Capability: "collection_query",
	}
	err := service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta)
	require.NoError(t, err)

	// Build resource capability
	err = service.RebuildResourceCapability(ctx, "demo-game", "development", "player")
	require.NoError(t, err)

	// Rebuild proposals
	err = service.RebuildProposalsForResource(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
}

func TestContractService_ScopeIsolation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	// Create contract in scope 1
	meta := FunctionMetaInput{
		ID:         "player.list",
		Version:    "1.0.0",
		Enabled:    true,
		Resource:   "player",
		Operation:  "list",
		Capability: "collection_query",
	}
	err := service.RebuildContractFromFunctionMeta(ctx, "game-1", "prod", "sdk", meta)
	require.NoError(t, err)

	// Verify scope 1 has the contract
	contracts1, err := service.ListContracts(ctx, "game-1", "prod")
	require.NoError(t, err)
	assert.Len(t, contracts1, 1)

	// Verify scope 2 has no contracts
	contracts2, err := service.ListContracts(ctx, "game-2", "prod")
	require.NoError(t, err)
	assert.Len(t, contracts2, 0)
}
