package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestProposalService_ListProposals(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create proposals
	proposals := []*model.PageProposal{
		{
			GameID:      "demo-game",
			Env:         "development",
			ProposalKey: "resource:player",
			PageKey:     "resource--player",
			PageType:    "resource",
			ResourceKey: "player",
			Quality:     "ready",
			Status:      "pending",
		},
		{
			GameID:      "demo-game",
			Env:         "development",
			ProposalKey: "operation:mail.send",
			PageKey:     "operation--mail.send",
			PageType:    "operation",
			ResourceKey: "mail",
			Quality:     "basic",
			Status:      "pending",
		},
	}

	for _, p := range proposals {
		err := service.proposalModel.UpsertProposal(ctx, p)
		require.NoError(t, err)
	}

	// List all proposals
	result, err := service.ListProposals(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// List by status
	pending, err := service.ListProposalsByStatus(ctx, "demo-game", "development", "pending")
	require.NoError(t, err)
	assert.Len(t, pending, 2)

	accepted, err := service.ListProposalsByStatus(ctx, "demo-game", "development", "accepted")
	require.NoError(t, err)
	assert.Len(t, accepted, 0)
}

func TestProposalService_AcceptProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	service := NewProposalService(db)

	// Create proposal
	page := testProposalPageSpec("resource--player")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)
	proposal := &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
		PageSpec:    pageJSON,
	}
	err = service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Accept proposal
	err = service.AcceptProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	// Verify status changed
	result, err := service.GetProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "accepted", result.Status)
	assert.Equal(t, "proposal_tester", result.UpdatedBy)

	draft, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	assert.Equal(t, "draft", draft.Status)
	assert.Equal(t, 1, draft.DraftRevision)
	assert.Equal(t, "proposal_tester", draft.UpdatedBy)
	assert.Equal(t, "resource--player", draft.PageKey)
	assert.Equal(t, "player", draft.CategoryKey)

	var stored spec.PageSpec
	require.NoError(t, json.Unmarshal([]byte(draft.SpecJSON), &stored))
	assert.Equal(t, spec.PageTypeOperation, stored.Type)
	assert.Equal(t, "player.query", stored.Bindings[0].FunctionID)

	versions, err := model.NewPageVersionModel(db).ListByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Equal(t, "draft", versions[0].Status)
	assert.Contains(t, versions[0].Message, "accept generated proposal")
}

func TestProposalService_AcceptProposalRequiresCanonicalPageSpec(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	service := NewProposalService(db)

	proposal := &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
	}
	err := service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	err = service.AcceptProposal(ctx, "demo-game", "development", "resource:player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical PageSpec")
}

func TestProposalService_AcceptProposalDoesNotOverwriteExistingDraft(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	service := NewProposalService(db)

	page := testProposalPageSpec("resource--player")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)
	err = service.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    "operation",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
		PageSpec:    pageJSON,
	})
	require.NoError(t, err)

	existing := &model.PageSpec{
		GameID:        "demo-game",
		Env:           "development",
		PageKey:       "resource--player",
		Type:          string(spec.PageTypeOperation),
		CategoryKey:   "player",
		SpecJSON:      `{"pageKey":"resource--player","type":"operation","title":{"zh-CN":"用户已编辑"},"category":{"key":"player","labels":{"zh-CN":"玩家"}},"operation":{"form":{"jsonSchema":{"type":"object"}}},"bindings":[{"id":"query","functionId":"player.query","usage":"query","execution":{"mode":"sync"}}]}`,
		Status:        "draft",
		DraftRevision: 3,
		UpdatedBy:     "manual_editor",
	}
	require.NoError(t, existing.SetTitle(map[string]string{"zh-CN": "用户已编辑"}))
	require.NoError(t, existing.SetCategoryLabels(map[string]string{"zh-CN": "玩家"}))
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, existing))

	err = service.AcceptProposal(ctx, "demo-game", "development", "resource:player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft already exists")

	draft, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	assert.Equal(t, 3, draft.DraftRevision)
	assert.Equal(t, "manual_editor", draft.UpdatedBy)
}

func TestProposalService_AcceptProposalRejectsErrorDiagnostics(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	page := testProposalPageSpec("resource--player")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)

	proposal := &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "needs_review",
		Status:      "pending",
		PageSpec:    pageJSON,
		Diagnostics: datatypes.JSON(`[{"code":"function_disabled","severity":"error","message":"function is disabled"}]`),
	}
	err = service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	err = service.AcceptProposal(ctx, "demo-game", "development", "resource:player")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocking diagnostics")
}

func TestProposalService_RejectProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create proposal
	proposal := &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
	}
	err := service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Reject proposal
	err = service.RejectProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	// Verify status changed
	result, err := service.GetProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "rejected", result.Status)
}

func TestProposalService_ScopeIsolation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewProposalService(db)

	// Create proposal in scope 1
	proposal := &model.PageProposal{
		GameID:      "game-1",
		Env:         "prod",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      "pending",
	}
	err := service.proposalModel.UpsertProposal(ctx, proposal)
	require.NoError(t, err)

	// Verify scope 1 has the proposal
	result1, err := service.ListProposals(ctx, "game-1", "prod")
	require.NoError(t, err)
	assert.Len(t, result1, 1)

	// Verify scope 2 has no proposals
	result2, err := service.ListProposals(ctx, "game-2", "prod")
	require.NoError(t, err)
	assert.Len(t, result2, 0)
}

func proposalTestContext() context.Context {
	ctx := context.WithValue(context.Background(), "username", "proposal_tester")
	return svc.WithGameScope(ctx, svc.GameScope{
		GameID: "demo-game",
		Env:    "development",
	})
}

func testProposalPageSpec(pageKey string) spec.PageSpec {
	return spec.PageSpec{
		PageKey:     pageKey,
		Type:        spec.PageTypeOperation,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"playerId":{"type":"string"}}}`),
				Layout:     spec.FormLayoutVertical,
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{
				ID:         "query",
				FunctionID: "player.query",
				Usage:      spec.BindingUsageQuery,
				Execution: spec.PageBindingExecution{
					Mode: spec.PageExecutionModeSync,
				},
			},
		},
	}
}
