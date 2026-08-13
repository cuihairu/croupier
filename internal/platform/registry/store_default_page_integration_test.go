package registry_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRegistrationMaterializesDefaultOperationProposal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentSessionDB{}, &model.FunctionContract{}, &model.ResourceCapability{}, &model.CapabilitySemantics{}, &model.CapabilitySemanticVersion{}, &model.PageProposal{}, &model.PageProposalVersion{}, &model.BlockedProposalIssue{}, &model.PageSpec{}, &model.PublishedPageSpec{}, &model.PageVersion{}))
	store := registry.NewStoreWithDB(db)
	store.SetContractService(service.NewContractService(db))
	err = store.UpsertAgent(&registry.AgentSession{AgentID: "agent-1", GameID: "demo-game", Env: "development", Functions: map[string]registry.FunctionMeta{
		"inventory.consume": {Enabled: true, Version: "1.0.0", Resource: "inventory", Operation: "consume", Capability: "action", Execution: "sync", Risk: "warning", InputSchema: `{"type":"object","properties":{"player_id":{"type":"string"},"item_id":{"type":"string"}},"required":["player_id","item_id"]}`, OutputSchema: `{"type":"object","properties":{"status":{"type":"string"}}}`},
	}})
	require.NoError(t, err)
	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "inventory.consume")
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"object","properties":{"player_id":{"type":"string"},"item_id":{"type":"string"}},"required":["player_id","item_id"]}`, string(contract.InputSchema))
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:inventory.consume")
	require.NoError(t, err)
	assert.Equal(t, "operation", proposal.PageType)
	assert.Equal(t, "basic", proposal.Quality)
	assert.NotEmpty(t, proposal.PageSpec)
	var generated spec.PageSpec
	require.NoError(t, json.Unmarshal(proposal.PageSpec, &generated))
	require.NotNil(t, generated.Operation)
	require.NotNil(t, generated.Operation.Form)
	assert.JSONEq(t, string(contract.InputSchema), string(generated.Operation.Form.JSONSchema))

	published, err := service.NewProposalService(db).AcceptAndPublishProposal(context.Background(), "demo-game", "development", proposal.ProposalKey)
	require.NoError(t, err)
	assert.Equal(t, proposal.PageKey, published.PageKey)
	page, err := model.NewPublishedPageSpecModel(db).FindLatestByScopeAndPageKey(context.Background(), "demo-game", "development", proposal.PageKey)
	require.NoError(t, err)
	assert.NotEmpty(t, page.SpecJSON)
}

func TestUpsertAgentDoesNotInferSDKResourceOrCapability(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentSessionDB{}, &model.FunctionContract{}, &model.ResourceCapability{}, &model.CapabilitySemantics{}, &model.CapabilitySemanticVersion{}, &model.PageProposal{}, &model.PageProposalVersion{}, &model.BlockedProposalIssue{}, &model.PageSpec{}, &model.PublishedPageSpec{}, &model.PageVersion{}))

	store := registry.NewStoreWithDB(db)
	store.SetContractService(service.NewContractService(db))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`,
			},
		},
	}))

	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "player.list")
	require.NoError(t, err)
	assert.Empty(t, contract.ResourceKey)
	assert.Empty(t, contract.OperationKey)
	assert.Empty(t, contract.Capability)

	proposalModel := model.NewPageProposalModel(db)
	proposal, err := proposalModel.FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:player.list")
	require.NoError(t, err)
	assert.Equal(t, "operation", proposal.PageType)
	assert.Equal(t, "operation--player.list", proposal.PageKey)

	_, err = proposalModel.FindByScopeAndKey(context.Background(), "demo-game", "development", "resource:player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	capabilities, err := model.NewResourceCapabilityModel(db).ListByScope(context.Background(), "demo-game", "development")
	require.NoError(t, err)
	assert.Empty(t, capabilities)
}

func TestUpsertAgentRemovesContractsAndProposalsAbsentFromSnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentSessionDB{}, &model.FunctionContract{}, &model.ResourceCapability{}, &model.CapabilitySemantics{}, &model.CapabilitySemanticVersion{}, &model.PageProposal{}, &model.PageProposalVersion{}, &model.BlockedProposalIssue{}, &model.PageSpec{}, &model.PublishedPageSpec{}, &model.PageVersion{}))

	store := registry.NewStoreWithDB(db)
	store.SetContractService(service.NewContractService(db))
	first := &registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				Resource:     "player",
				Operation:    "list",
				Capability:   "collection_query",
				Execution:    "sync",
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},"total":{"type":"integer"}}}`,
			},
		},
	}
	require.NoError(t, store.UpsertAgent(first))

	ctx := context.Background()
	contractModel := model.NewFunctionContractModel(db)
	proposalModel := model.NewPageProposalModel(db)
	_, err = contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	require.NoError(t, err)
	_, err = proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "demo-game",
		Env:       "development",
		Functions: map[string]registry.FunctionMeta{},
	}))

	_, err = contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = model.NewResourceCapabilityModel(db).FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUpsertAgentKeepsContractDeclaredByAnotherAgent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentSessionDB{}, &model.FunctionContract{}, &model.ResourceCapability{}, &model.CapabilitySemantics{}, &model.CapabilitySemanticVersion{}, &model.PageProposal{}, &model.PageProposalVersion{}, &model.BlockedProposalIssue{}, &model.PageSpec{}, &model.PublishedPageSpec{}, &model.PageVersion{}))

	store := registry.NewStoreWithDB(db)
	store.SetContractService(service.NewContractService(db))
	function := registry.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	}
	for _, agentID := range []string{"agent-1", "agent-2"} {
		require.NoError(t, store.UpsertAgent(&registry.AgentSession{
			AgentID: agentID,
			GameID:  "demo-game",
			Env:     "development",
			Functions: map[string]registry.FunctionMeta{
				"mail.send": function,
			},
		}))
	}

	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "demo-game",
		Env:       "development",
		Functions: map[string]registry.FunctionMeta{},
	}))

	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(context.Background(), "demo-game", "development", "mail.send")
	require.NoError(t, err)
	assert.Equal(t, "mail.send", contract.FunctionID)
	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:mail.send")
	require.NoError(t, err)
}
