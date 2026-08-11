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
