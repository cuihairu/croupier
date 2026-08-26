package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestInbox_ContractChangesForPublishedAndDrafts(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	// A published page whose binding points at a vanished function -> stale.
	publishedSpec := map[string]interface{}{
		"pageKey":     "player--stale",
		"type":        "operation",
		"resourceKey": "player",
		"title":       map[string]string{"zh-CN": "旧页"},
		"category":    map[string]interface{}{"key": "player", "labels": map[string]string{"zh-CN": "玩家"}},
		"bindings": []map[string]interface{}{{
			"id": "query", "functionId": "ghost.fn", "usage": "query",
			"execution": map[string]interface{}{"mode": "sync"},
		}},
	}
	specJSON, err := json.Marshal(publishedSpec)
	require.NoError(t, err)

	published := &model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: "player--stale", Version: 1,
		SpecJSON: string(specJSON), BindingContractsJSON: "[]", RendererSchemaVersion: "page-spec:1",
		Active: true, PublishedBy: "tester",
	}
	require.NoError(t, db.Create(published).Error)

	// A draft whose binding references a missing function -> stale draft.
	draft := &model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "player--draft-stale",
		Type: "operation", ResourceKey: "player", CategoryKey: "player",
		Status: "draft", DraftRevision: 1, SpecJSON: string(specJSON),
	}
	require.NoError(t, draft.SetTitle(map[string]string{"zh-CN": "草稿"}))
	require.NoError(t, draft.SetCategoryLabels(map[string]string{"zh-CN": "玩家"}))
	require.NoError(t, db.Create(draft).Error)

	// An empty published row exercises the skip branch.
	require.NoError(t, db.Create(&model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: "", Version: 1, Active: true,
	}).Error)

	resp, err := svc.Inbox(ctx, "demo-game", "development", ProposalListFilter{})
	require.NoError(t, err)

	kinds := map[string]ContractChangeDTO{}
	for _, change := range resp.ContractChanges {
		if change.Kind == "published" || change.Kind == "draft" {
			kinds[change.Kind+"|"+change.PageKey] = change
		}
	}
	assert.Contains(t, kinds, "published|player--stale")
	assert.Contains(t, kinds, "draft|player--draft-stale")
	assert.Equal(t, len(resp.ContractChanges), resp.Summary.ContractChanges)

	// resourceKey filter keeps only matching rows.
	filtered, err := svc.Inbox(ctx, "demo-game", "development", ProposalListFilter{ResourceKey: "player"})
	require.NoError(t, err)
	assert.NotEmpty(t, filtered.ContractChanges)

	other, err := svc.Inbox(ctx, "demo-game", "development", ProposalListFilter{ResourceKey: "guild"})
	require.NoError(t, err)
	assert.Empty(t, other.ContractChanges)
}

func TestInbox_DraftFallsBackToPublishedStaleness(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := testProposalPageSpec("player--fresh")

	staleSpecJSON, err := json.Marshal(page)
	require.NoError(t, err)

	// Published snapshot with a stale binding.
	require.NoError(t, db.Create(&model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: "player--fresh", Version: 1,
		SpecJSON: string(staleSpecJSON), BindingContractsJSON: `[{"bindingId":"query","functionId":"ghost.fn","executionMode":"sync"}]`,
		RendererSchemaVersion: "page-spec:1", Active: true, PublishedBy: "tester",
	}).Error)

	// Fresh draft of the same page: no local diags, but published version>0
	// and a stale entry exists for the same page key.
	draftSpec := testProposalPageSpec("player--fresh")
	draftSpec.Bindings[0].FunctionID = "player.query"
	freshJSON, err := json.Marshal(draftSpec)
	require.NoError(t, err)

	contract := &model.FunctionContract{GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0"}
	require.NoError(t, db.Create(contract).Error)

	draft := &model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "player--fresh",
		Type: "operation", ResourceKey: "player", CategoryKey: "player",
		Status: "draft", DraftRevision: 2, PublishedVersion: 1, SpecJSON: string(freshJSON),
	}
	require.NoError(t, draft.SetTitle(map[string]string{"zh-CN": "新草稿"}))
	require.NoError(t, draft.SetCategoryLabels(map[string]string{"zh-CN": "玩家"}))
	require.NoError(t, db.Create(draft).Error)

	resp, err := svc.Inbox(ctx, "demo-game", "development", ProposalListFilter{})
	require.NoError(t, err)

	foundDraftFallback := false
	for _, change := range resp.ContractChanges {
		if change.Kind == "draft" && change.PageKey == "player--fresh" && len(change.BindingFreshness) > 0 {
			foundDraftFallback = true
		}
	}
	assert.True(t, foundDraftFallback, "draft should inherit published staleness diagnostics")
}

func TestProposalService_RejectAndAcceptValidationErrors(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	// createProposalVersionSnapshot guards.
	_, err := createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), nil, "reason", "actor")
	assert.Error(t, err)
	_, err = createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), &model.PageProposal{}, "reason", "actor")
	assert.Error(t, err)

	// Rejecting an unknown proposal surfaces the not-found error.
	err = svc.RejectProposal(ctx, "demo-game", "development", "nope")
	assert.Error(t, err)
}

func TestBuildBindingContracts_Branches(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "fn.ok", Enabled: true, Version: "1.0.0",
		InputSchema: datatypes.JSON(`{"type":"object"}`),
	}).Error)
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "fn.off", Enabled: false, Version: "1.0.0",
	}).Error)

	// Empty function id.
	_, err := svc.buildBindingContracts(ctx, "demo-game", "development", []spec.PageFunctionBinding{{ID: "b"}})
	assert.Error(t, err)

	// Missing contract.
	_, err = svc.buildBindingContracts(ctx, "demo-game", "development", []spec.PageFunctionBinding{{ID: "b", FunctionID: "fn.missing"}})
	assert.Error(t, err)

	// Disabled contract.
	_, err = svc.buildBindingContracts(ctx, "demo-game", "development", []spec.PageFunctionBinding{{ID: "b", FunctionID: "fn.off"}})
	assert.Error(t, err)

	// Happy path sorts snapshots by binding id.
	snapshots, err := svc.buildBindingContracts(ctx, "demo-game", "development", []spec.PageFunctionBinding{
		{ID: "zz", FunctionID: "fn.ok"},
		{ID: "aa", FunctionID: "fn.ok"},
	})
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	assert.Equal(t, "aa", snapshots[0].BindingID)
	assert.Equal(t, "zz", snapshots[1].BindingID)
}

func TestRemoveFunctionContract_Flows(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewContractService(db)

	// Blank and unknown ids are no-ops.
	key, err := svc.RemoveFunctionContract(ctx, "demo-game", "development", "   ")
	require.NoError(t, err)
	assert.Empty(t, key)

	key, err = svc.RemoveFunctionContract(ctx, "demo-game", "development", "missing.fn")
	require.NoError(t, err)
	assert.Empty(t, key)

	contract := &model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "doomed.fn", ResourceKey: "player", Enabled: true,
	}
	require.NoError(t, svc.contractModel.UpsertContract(ctx, contract))

	key, err = svc.RemoveFunctionContract(ctx, "demo-game", "development", "doomed.fn")
	require.NoError(t, err)
	assert.Equal(t, "player", key)

	_, err = svc.contractModel.FindByScopeAndFunctionID(ctx, "demo-game", "development", "doomed.fn")
	assert.Error(t, err)
}

func TestPreserveReviewedCapability_AndIdentityDiagnostics(t *testing.T) {
	// Nil arguments are tolerated.
	preserveReviewedCapability(nil, nil)
	preserveReviewedCapability(&model.ResourceCapability{}, nil)

	next := &model.ResourceCapability{}
	existing := &model.ResourceCapability{
		Labels:      datatypes.JSONMap{"zh-CN": "玩家"},
		Description: datatypes.JSONMap{"zh-CN": "管理玩家"},
		CategoryKey: "players",
		Tags:        datatypes.JSON(`["ops"]`),
		UpdatedBy:   "reviewer",
	}
	preserveReviewedCapability(next, existing)
	assert.Equal(t, "reviewer", next.UpdatedBy)
	assert.Equal(t, "players", next.CategoryKey)
	assert.NotNil(t, next.Labels)

	// Identity diagnostic helpers.
	setIdentityDiagnostic(nil, "code", "message")
	sem := &model.CapabilitySemantics{}
	setIdentityDiagnostic(sem, "resource_identity_not_verifiable", "no identity")
	assert.NotEmpty(t, sem.Diagnostics)
}

func TestLoadTermDictionary_Variants(t *testing.T) {
	ctx := context.Background()

	// Nil database short-circuits to an empty dictionary.
	nilSvc := NewContractService(nil)
	assert.Nil(t, nilSvc.loadTermDictionary(ctx))

	// Database errors degrade gracefully.
	closed := NewContractService(newClosedServiceDB(t))
	assert.Nil(t, closed.loadTermDictionary(ctx))

	db := setupTestDB(t)
	svc := NewContractService(db)
	require.NoError(t, db.Create(&model.TermDictionary{Domain: "resource", TermKey: "player", Alias: "user", Display: map[string]string{"zh-CN": "玩家", "en-US": "Player"}, SortOrder: 1}).Error)
	require.NoError(t, db.Create(&model.TermDictionary{Domain: "operation", TermKey: "query", Alias: "list"}).Error)

	terms := svc.loadTermDictionary(ctx)
	require.NotEmpty(t, terms)
	text, ok := terms["resource/user"]
	require.True(t, ok)
	assert.Equal(t, "玩家", text["zh-CN"])
	assert.Equal(t, "Player", text["en-US"])

	_, ok = terms["operation/list"]
	assert.False(t, ok, "entries without display text are skipped")
}

func TestFunctionSpecsByScope_ErrorBranch(t *testing.T) {
	svc := NewContractService(newClosedServiceDB(t))
	_, err := FunctionSpecsByScope(context.Background(), svc.contractModel, "demo-game", "development")
	assert.Error(t, err)
}
