package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
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
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
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
		ID:                "player.ban",
		Version:           "1.0.0",
		Enabled:           true,
		Summary:           "Ban a player",
		Description:       "Ban a player from the game",
		InputSchema:       `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
		OutputSchema:      `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
		Resource:          "player",
		Operation:         "ban",
		Capability:        "action",
		Execution:         "sync",
		ApprovalRequired:  true,
		ApprovalPolicyKey: "two_person",
		Risk:              "high",
		Permission:        "player.ban.invoke",
		Tags:              []string{"player", "admin"},
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
	assert.Equal(t, true, contract.Approval["required"])
	assert.Equal(t, "two_person", contract.Approval["policyKey"])
	assert.Equal(t, "high", contract.Risk)
	assert.Equal(t, "sdk", contract.Source)
}

func TestContractService_RebuildContractRejectsUnstableKeys(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	err := service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID:          "Player.Ban",
		Version:     "1.0.0",
		Enabled:     true,
		Resource:    "player",
		Operation:   "banPlayer",
		Capability:  "action",
		InputSchema: `{"type":"object"}`,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "function contract validation failed")
	contracts, listErr := service.ListContracts(ctx, "demo-game", "development")
	require.NoError(t, listErr)
	assert.Empty(t, contracts)
}

func TestContractService_RebuildContractRejectsPresentationSchema(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	err := service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID:          "player.list",
		Version:     "1.0.0",
		Enabled:     true,
		Resource:    "player",
		Operation:   "list",
		Capability:  "collection_query",
		Execution:   "sync",
		InputSchema: `{"type":"object","x-menu":"Players"}`,
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, `forbidden presentation field "x-menu" at input_schema.x-menu`)
	contracts, listErr := service.ListContracts(ctx, "demo-game", "development")
	require.NoError(t, listErr)
	assert.Empty(t, contracts)
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
		ID:           "player.list",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "list",
		Capability:   "collection_query",
		Execution:    "sync",
		InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}},"total":{"type":"integer"}}}`,
	}
	err := service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta)
	require.NoError(t, err)

	// Build resource capability
	err = service.RebuildResourceCapability(ctx, "demo-game", "development", "player")
	require.NoError(t, err)

	// Rebuild proposals
	err = service.RebuildProposalsForResource(ctx, "demo-game", "development", "player")
	require.NoError(t, err)

	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "resource--player", proposal.PageKey)
	assert.Equal(t, "resource", proposal.PageType)
	// With collection_query and identity field, quality should be ready
	assert.Equal(t, "ready", proposal.Quality)
	assert.NotEmpty(t, proposal.PageSpec)
	assert.Len(t, proposal.FunctionDigest, 64)
	assert.Len(t, proposal.SemanticsDigest, 64)
}

func TestContractService_RebuildProposalForFunctionWithoutResource(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	meta := FunctionMetaInput{
		ID:           "player.ban",
		Version:      "1.0.0",
		Enabled:      true,
		Operation:    "ban",
		Capability:   "action",
		Execution:    "sync",
		Summary:      "Ban player",
		InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	}
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta))
	require.NoError(t, service.RebuildProposalForFunction(ctx, "demo-game", "development", "player.ban"))

	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	assert.Equal(t, "operation--player.ban", proposal.PageKey)
	assert.Equal(t, "operation", proposal.PageType)
	assert.Equal(t, "", proposal.ResourceKey)
	assert.NotEmpty(t, proposal.PageSpec)
	assert.Len(t, proposal.FunctionDigest, 64)
	assert.Len(t, proposal.SemanticsDigest, 64)
}

func TestContractService_RebuildProposalForCRUDFunctionWithoutResource(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	meta := FunctionMetaInput{
		ID:           "player.create",
		Version:      "1.0.0",
		Enabled:      true,
		Operation:    "create",
		Capability:   "create",
		Execution:    "sync",
		InputSchema:  `{"type":"object","properties":{"name":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"id":{"type":"string"}}}`,
	}
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta))
	require.NoError(t, service.RebuildProposalForFunction(ctx, "demo-game", "development", "player.create"))

	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.create")
	require.NoError(t, err)
	assert.Equal(t, "operation--player.create", proposal.PageKey)
	assert.Equal(t, "operation", proposal.PageType)
	assert.Equal(t, "", proposal.ResourceKey)
}

func TestContractService_RebuildProposalsForResourceFallsBackToStandaloneCRUDProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID:           "player.create",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "create",
		Capability:   "create",
		Execution:    "sync",
		InputSchema:  `{"type":"object","properties":{"name":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"id":{"type":"string"}}}`,
	}))
	require.NoError(t, service.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, service.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))

	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.create")
	require.NoError(t, err)
	assert.Equal(t, "operation--player.create", proposal.PageKey)
	assert.Equal(t, "operation", proposal.PageType)
	assert.Equal(t, "player", proposal.ResourceKey)
	assert.Equal(t, "basic", proposal.Quality)
	assert.NotEmpty(t, proposal.PageSpec)

	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestContractService_RebuildProposalsForResourceConsumesCRUDInResourceProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID:           "player.list",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "list",
		Capability:   "collection_query",
		Execution:    "sync",
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}},"total":{"type":"integer"}}}`,
	}))
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID:           "player.create",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "create",
		Capability:   "create",
		Execution:    "sync",
		InputSchema:  `{"type":"object","properties":{"name":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"id":{"type":"string"}}}`,
	}))
	require.NoError(t, service.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, service.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))

	resourceProposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "resource", resourceProposal.PageType)

	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.create")
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestContractService_RebuildProposalsForResourceKeepsUnsafeActionStandalone(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.list", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "list", Capability: "collection_query", Execution: "sync",
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`,
	}))
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.ban", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "ban", Capability: "action", Execution: "sync",
		InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"reason":{"type":"string"}},"required":["id","reason"]}`,
	}))
	require.NoError(t, service.RebuildResourceCapability(ctx, "demo-game", "development", "player"))

	semantics, err := model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	semantics.Actions = datatypes.JSON(`[{"functionId":"player.ban","subject":"resource_item","identityInput":"/id"}]`)
	require.NoError(t, model.NewCapabilitySemanticsModel(db).UpsertSemantics(ctx, semantics))
	require.NoError(t, service.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))

	proposalModel := model.NewPageProposalModel(db)
	resourceProposal, err := proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	var resourcePage spec.PageSpec
	require.NoError(t, json.Unmarshal(resourceProposal.PageSpec, &resourcePage))
	require.NotNil(t, resourcePage.Resource)
	assert.Empty(t, resourcePage.Resource.ListView.RowActions)
	assert.False(t, pageHasBinding(resourcePage.Bindings, "action.ban"))

	operationProposal, err := proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	assert.Equal(t, "operation", operationProposal.PageType)
}

func pageHasBinding(bindings []spec.PageFunctionBinding, bindingID string) bool {
	for _, binding := range bindings {
		if binding.ID == bindingID {
			return true
		}
	}
	return false
}

func TestContractService_RebuildProposalsPreservesAcceptedStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	meta := FunctionMetaInput{
		ID:           "player.list",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "list",
		Capability:   "collection_query",
		Execution:    "sync",
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},"total":{"type":"integer"}}}`,
	}
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", meta))
	require.NoError(t, service.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, service.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))

	proposalModel := model.NewPageProposalModel(db)
	proposal, err := proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	proposal.Status = "accepted"
	proposal.UpdatedBy = "operator"
	require.NoError(t, proposalModel.UpsertProposal(ctx, proposal))

	require.NoError(t, service.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))

	proposal, err = proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "accepted", proposal.Status)
	assert.Equal(t, "operator", proposal.UpdatedBy)
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

func TestOperationSpecFromContract(t *testing.T) {
	// Test nil contract
	spec := OperationSpecFromContract(nil)
	assert.Empty(t, spec.FunctionID)

	// Test with contract
	contract := &model.FunctionContract{
		FunctionID:   "player.list",
		ResourceKey:  "player",
		OperationKey: "list",
		Capability:   "collection_query",
		Execution:    "sync",
		Risk:         "safe",
		Permission:   "player:list",
		Enabled:      true,
	}
	spec = OperationSpecFromContract(contract)
	assert.Equal(t, "player.list", spec.FunctionID)
	assert.Equal(t, "player", spec.ResourceKey)
	assert.Equal(t, "list", spec.Operation)
	assert.Equal(t, "collection_query", string(spec.Capability))
	assert.Equal(t, "sync", string(spec.Execution))
	assert.Equal(t, "safe", string(spec.Risk))
	assert.Equal(t, "player:list", spec.Permission)
	assert.True(t, spec.Enabled)
}

func TestFunctionSpecFromContract(t *testing.T) {
	// Test nil contract
	spec := FunctionSpecFromContract(nil)
	assert.Empty(t, spec.ID)

	// Test with contract
	contract := &model.FunctionContract{
		FunctionID:   "player.list",
		Version:      "1.0.0",
		Enabled:      true,
		Deprecated:   false,
		InputSchema:  []byte(`{"type":"object"}`),
		OutputSchema: []byte(`{"type":"array"}`),
		ResourceKey:  "player",
		OperationKey: "list",
		Capability:   "collection_query",
		Execution:    "sync",
		Risk:         "safe",
		Permission:   "player:list",
	}
	spec = FunctionSpecFromContract(contract)
	assert.Equal(t, "player.list", spec.ID)
	assert.Equal(t, "1.0.0", spec.Version)
	assert.True(t, spec.Enabled)
	assert.False(t, spec.Deprecated)
	assert.Equal(t, "player", spec.Resource)
	assert.Equal(t, "list", spec.Operation)
}

func TestJsonMapToApprovalPolicy(t *testing.T) {
	// Test empty map
	policy := ApprovalPolicyFromJSONMap(nil)
	assert.False(t, policy.Required)
	assert.Empty(t, policy.PolicyKey)

	// Test with values
	values := map[string]interface{}{
		"required":  true,
		"policyKey": "high-risk",
	}
	policy = ApprovalPolicyFromJSONMap(values)
	assert.True(t, policy.Required)
	assert.Equal(t, "high-risk", policy.PolicyKey)

	// Test with policy_key (snake_case)
	values2 := map[string]interface{}{
		"required":   true,
		"policy_key": "admin-approval",
	}
	policy2 := ApprovalPolicyFromJSONMap(values2)
	assert.True(t, policy2.Required)
	assert.Equal(t, "admin-approval", policy2.PolicyKey)
}

func TestJsonMapToLocalizedText(t *testing.T) {
	// Test empty map
	text := LocalizedTextFromJSONMap(nil)
	assert.Nil(t, text)

	// Test with values
	values := map[string]interface{}{
		"zh-CN": "玩家列表",
		"en":    "Player List",
	}
	text = LocalizedTextFromJSONMap(values)
	assert.Equal(t, "玩家列表", text["zh-CN"])
	assert.Equal(t, "Player List", text["en"])

	// Test with empty values
	values2 := map[string]interface{}{
		"zh-CN": "",
		"en":    "  ",
	}
	text2 := LocalizedTextFromJSONMap(values2)
	assert.Nil(t, text2)
}

func TestResourceProposalKey(t *testing.T) {
	assert.Equal(t, "resource:player", resourceProposalKey("player"))
	assert.Equal(t, "resource:inventory", resourceProposalKey("inventory"))
	assert.Empty(t, resourceProposalKey(""))
	assert.Empty(t, resourceProposalKey("  "))
}

func TestStandaloneProposalKey(t *testing.T) {
	assert.Equal(t, "operation:mail.send", standaloneProposalKey("operation", "mail.send"))
	assert.Equal(t, "task:reward.batch", standaloneProposalKey("task", "reward.batch"))
	assert.Equal(t, "report:analytics.retention", standaloneProposalKey("report", "analytics.retention"))
	assert.Empty(t, standaloneProposalKey("operation", ""))
}

func TestIsCRUDCapability(t *testing.T) {
	assert.True(t, isCRUDCapability("collection_query"))
	assert.True(t, isCRUDCapability("item_query"))
	assert.True(t, isCRUDCapability("create"))
	assert.True(t, isCRUDCapability("update"))
	assert.True(t, isCRUDCapability("delete"))
	assert.False(t, isCRUDCapability("action"))
	assert.False(t, isCRUDCapability("task"))
	assert.False(t, isCRUDCapability("report"))
	assert.False(t, isCRUDCapability(""))
}

func TestPreserveReviewedSemantics(t *testing.T) {
	// Test nil inputs
	preserveReviewedSemantics(nil, nil)
	preserveReviewedSemantics(nil, &model.CapabilitySemantics{})
	preserveReviewedSemantics(&model.CapabilitySemantics{}, nil)

	// Test preserving platform_review source
	next := &model.CapabilitySemantics{
		Source: "sdk_explicit",
	}
	existing := &model.CapabilitySemantics{
		Source:            "platform_review",
		UpdatedBy:         "admin",
		IdentityField:     "player_id",
		IdentityFieldType: "string",
		CollectionQueryID: 1,
		CollectionPath:    "/items",
		PageFieldName:     "page",
		PageSizeFieldName: "page_size",
	}
	preserveReviewedSemantics(next, existing)
	assert.Equal(t, "platform_review", next.Source)
	assert.Equal(t, "admin", next.UpdatedBy)
	assert.Equal(t, "player_id", next.IdentityField)
	assert.Equal(t, "string", next.IdentityFieldType)
	assert.Equal(t, uint(1), next.CollectionQueryID)
	assert.Equal(t, "/items", next.CollectionPath)
	assert.Equal(t, "page", next.PageFieldName)
	assert.Equal(t, "page_size", next.PageSizeFieldName)

	// Test not overwriting with empty values
}

func TestSchemaScalarTypeV2(t *testing.T) {
	tests := []struct {
		input    map[string]json.RawMessage
		expected string
	}{
		{map[string]json.RawMessage{"type": []byte(`"string"`)}, "string"},
		{map[string]json.RawMessage{"type": []byte(`"number"`)}, "number"},
		{map[string]json.RawMessage{"type": []byte(`"integer"`)}, "integer"},
		{map[string]json.RawMessage{"type": []byte(`"boolean"`)}, "boolean"},
		{map[string]json.RawMessage{}, ""},
	}

	for _, tt := range tests {
		result := schemaScalarType(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestParseJSONSchemaV2(t *testing.T) {
	// Test valid JSON
	schema := parseJSONSchema([]byte(`{"type":"object","properties":{"name":{"type":"string"}}}`))
	assert.NotNil(t, schema)

	// Test empty
	schema = parseJSONSchema([]byte{})
	assert.Nil(t, schema)

	// Test nil
	schema = parseJSONSchema(nil)
	assert.Nil(t, schema)
}

func TestSchemaStringV2(t *testing.T) {
	tests := []struct {
		input    json.RawMessage
		expected string
	}{
		{json.RawMessage(`"hello"`), "hello"},
		{json.RawMessage(`"world"`), "world"},
		{json.RawMessage(`""`), ""},
		{json.RawMessage(`null`), ""},
		{json.RawMessage(`123`), ""},
	}

	for _, tt := range tests {
		result := schemaString(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestTaskSemanticsByStartFunction(t *testing.T) {
	// Test nil semantics
	result := taskSemanticsByStartFunction(nil)
	assert.Empty(t, result)

	// Test empty tasks
	sem := &model.CapabilitySemantics{
		Tasks: []byte(`[]`),
	}
	result = taskSemanticsByStartFunction(sem)
	assert.Empty(t, result)

	// Test with tasks
	sem2 := &model.CapabilitySemantics{
		Tasks: []byte(`[{"start":{"functionId":"task.start"},"taskId":{"resultPath":"/id"}}]`),
	}
	result = taskSemanticsByStartFunction(sem2)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "task.start")
}

func TestReportSemanticsByQueryFunction(t *testing.T) {
	// Test nil semantics
	result := reportSemanticsByQueryFunction(nil)
	assert.Empty(t, result)

	// Test empty reports
	sem := &model.CapabilitySemantics{
		Reports: []byte(`[]`),
	}
	result = reportSemanticsByQueryFunction(sem)
	assert.Empty(t, result)

	// Test with reports
	sem2 := &model.CapabilitySemantics{
		Reports: []byte(`[{"query":{"functionId":"report.query"},"datasetPath":"/data"}]`),
	}
	result = reportSemanticsByQueryFunction(sem2)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "report.query")
}

func TestGeneratedProposalChanged(t *testing.T) {
	// Test with different digests
	existing := &model.PageProposal{
		FunctionDigest: "digest1",
	}
	next := &model.PageProposal{
		FunctionDigest: "digest2",
	}
	changed := generatedProposalChanged(existing, next)
	assert.True(t, changed)

	// Test with same digests
	existing2 := &model.PageProposal{
		FunctionDigest: "digest1",
	}
	next2 := &model.PageProposal{
		FunctionDigest: "digest1",
	}
	changed = generatedProposalChanged(existing2, next2)
	assert.False(t, changed)
}

func TestContractService_BlockedIssueIsScopedToFunction(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.ban", Version: "1.0.0", Enabled: false, Resource: "player", Operation: "ban",
		Capability: "action", Execution: "sync",
	}))
	require.NoError(t, service.RebuildProposalForFunction(ctx, "demo-game", "development", "player.ban"))

	issues, err := model.NewBlockedProposalIssueModel(db).ListByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "player.ban", issues[0].FunctionID)

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.mail", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "mail",
		Capability: "action", Execution: "sync",
	}))
	require.NoError(t, service.RebuildProposalForFunction(ctx, "demo-game", "development", "player.mail"))

	issues, err = model.NewBlockedProposalIssueModel(db).ListByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "player.ban", issues[0].FunctionID)

	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestContractService_RebuildResourceCapabilityRecordsSourceConflict(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewContractService(db)

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "openapi", FunctionMetaInput{
		ID: "player.list.rest", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "list-rest",
		Capability: "collection_query", Execution: "sync",
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`,
	}))
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.list.sdk", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "list-sdk",
		Capability: "collection_query", Execution: "sync",
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"player_id":{"type":"string"}}}}}}`,
	}))

	require.NoError(t, service.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	semantics, err := model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)

	contracts, err := model.NewFunctionContractModel(db).ListByResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	var sdkID uint
	for _, contract := range contracts {
		if contract.FunctionID == "player.list.sdk" {
			sdkID = contract.ID
		}
	}
	assert.Equal(t, sdkID, semantics.CollectionQueryID)

	var conflicts []spec.SemanticConflict
	require.NoError(t, json.Unmarshal(semantics.Conflicts, &conflicts))
	require.Len(t, conflicts, 1)
	assert.Equal(t, "collectionQueryID", conflicts[0].Field)
	assert.Empty(t, conflicts[0].Resolution)
}
