package resourcecatalog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
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
		&model.BlockedProposalIssue{},
	)
	require.NoError(t, err)

	return db
}

func TestService_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// Create resource capability
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Labels:      map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Create contracts (need both collection_query and item_query for "identified")
	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.list",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  dbenum.CapabilityCollectionQuery,
		Execution:   "sync",
		Risk:        dbenum.RiskSafe,
		Source:      "sdk",
	})
	require.NoError(t, err)

	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.get",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  dbenum.CapabilityItemQuery,
		Execution:   "sync",
		Risk:        dbenum.RiskSafe,
		Source:      "sdk",
	})
	require.NoError(t, err)

	// Create semantics
	semanticsModel := model.NewCapabilitySemanticsModel(db)
	err = semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:            "demo-game",
		Env:               "development",
		ResourceKey:       "player",
		CollectionQueryID: 1,
		ItemQueryID:       2,
		IdentityField:     "player_id",
	})
	require.NoError(t, err)

	// List resources
	result, err := service.List(ctx, &ListRequest{
		GameID: "demo-game",
		Env:    "development",
	})
	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "player", result.Items[0].ResourceKey)
	assert.Equal(t, "identified", result.Items[0].Status)
}

func TestService_Detail(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// Create resource capability
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Labels:      map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Get detail
	result, err := service.Detail(ctx, &DetailRequest{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "player", result.ResourceKey)
}

func TestService_UpdateSemantics(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// Create resource capability
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Labels:      map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Update semantics
	result, err := service.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID:        "demo-game",
		Env:           "development",
		ResourceKey:   "player",
		IdentityField: "player_id",
		ChangeReason:  "test update",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "platform_review", result.Source)
}

func TestService_ScopeIsolation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// Create resource in scope 1
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID:      "game-1",
		Env:         "prod",
		ResourceKey: "player",
		Labels:      map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// List scope 1
	result1, err := service.List(ctx, &ListRequest{
		GameID: "game-1",
		Env:    "prod",
	})
	require.NoError(t, err)
	assert.Len(t, result1.Items, 1)

	// List scope 2
	result2, err := service.List(ctx, &ListRequest{
		GameID: "game-2",
		Env:    "prod",
	})
	require.NoError(t, err)
	assert.Len(t, result2.Items, 0)
}

func TestService_SearchFilter(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// Create resources
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Labels:      map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)
	err = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "mail",
		Labels:      map[string]interface{}{"zh-CN": "邮件"},
	})
	require.NoError(t, err)

	// Search for "player"
	result, err := service.List(ctx, &ListRequest{
		GameID: "demo-game",
		Env:    "development",
		Query:  "player",
	})
	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "player", result.Items[0].ResourceKey)
}

func TestService_ResolveConflict(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// Create resource capability
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Labels:      map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Create semantics with conflict
	semanticsModel := model.NewCapabilitySemanticsModel(db)
	conflicts := []spec.SemanticConflict{
		{
			Field: "identityField",
			Values: map[spec.SemanticSource]json.RawMessage{
				spec.SemanticSourceSDKExplicit: json.RawMessage(`"player_id"`),
				spec.SemanticSourceOpenAPIRest: json.RawMessage(`"id"`),
			},
		},
	}
	conflictsJSON, _ := json.Marshal(conflicts)
	err = semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
		Source:      "sdk_explicit",
		Conflicts:   conflictsJSON,
	})
	require.NoError(t, err)

	// Resolve conflict
	result, err := service.ResolveConflict(ctx, &ResolveConflictRequest{
		GameID:       "demo-game",
		Env:          "development",
		ResourceKey:  "player",
		Field:        "identityField",
		ChosenSource: "openapi_rest",
		Reason:       "choose openapi source",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify conflict resolved
	semantics, err := semanticsModel.FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	assert.Equal(t, "id", semantics.IdentityField)
}
