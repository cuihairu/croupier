package service

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildOperationProposal returns a pending ready proposal whose single binding
// points at functionID.
func buildOperationProposal(pageKey, functionID string, mutate func(*spec.PageSpec)) (*model.PageProposal, error) {
	page := testProposalPageSpec(pageKey)
	page.Bindings[0].FunctionID = functionID
	// Publishable operation pages require an action binding as well.
	page.Bindings = append(page.Bindings, spec.PageFunctionBinding{
		ID:         "act",
		FunctionID: functionID,
		Usage:      spec.BindingUsageAction,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	})
	if mutate != nil {
		mutate(&page)
	}
	pageJSON, err := json.Marshal(page)
	if err != nil {
		return nil, err
	}
	return &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "operation:" + pageKey,
		PageKey:     pageKey,
		PageType:    "operation",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      dbenum.ProposalStatusPending,
		PageSpec:    pageJSON,
	}, nil
}

func TestAcceptAndPublish_RejectsBrokenBindings(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, mustProposal(buildOperationProposal("op--missing", "ghost.fn", nil))))
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "operation:op--missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, mustProposal(buildOperationProposal("op--disabled", "fn.off", nil))))
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "fn.off", Enabled: false, Version: "1.0.0",
	}).Error)
	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "operation:op--disabled")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestAcceptAndPublish_RequiresInputSelectors(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	// The bound contract has input fields, so publishing without selectors
	// must fail with a selectors.input detail.
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.query",
		Enabled:     true,
		Version:     "1.0.0",
		InputSchema: []byte(`{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`),
	}).Error)

	proposal, err := buildOperationProposal("op--nosel", "player.query", nil)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))

	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selectors.input is required")
}

func TestAcceptAndPublish_CategoryLabelConflict(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)

	// Publish the first page for category "player".
	first, err := buildOperationProposal("cat--first", "player.query", nil)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, first))
	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", first.ProposalKey)
	require.NoError(t, err)

	// A second page in the same category with different labels conflicts.
	conflicting, err := buildOperationProposal("cat--second", "player.query", func(p *spec.PageSpec) {
		p.Category.Labels = spec.LocalizedText{"zh-CN": "玩家管理"}
	})
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, conflicting))
	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", conflicting.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "category labels conflict")

	// Identical labels are accepted.
	consistent, err := buildOperationProposal("cat--third", "player.query", func(p *spec.PageSpec) {
		p.Category.Labels = spec.LocalizedText{"zh-CN": "玩家"}
	})
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, consistent))
	result, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", consistent.ProposalKey)
	require.NoError(t, err)
	assert.Equal(t, "cat--third", result.PageKey)
}

func TestAcceptAndPublish_HappyPathFreezesSnapshot(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "2.0.0",
		Risk:         dbenum.RiskSafe,
		InputSchema:  []byte(`{"type":"object"}`),
		OutputSchema: []byte(`{"type":"object"}`),
	}).Error)

	proposal, err := buildOperationProposal("op--happy", "player.query", nil)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))

	result, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.NoError(t, err)
	assert.Equal(t, "op--happy", result.PageKey)
	assert.Equal(t, 1, result.PublishedVersion)

	published, err := model.NewPublishedPageSpecModel(db).FindLatestByScopeAndPageKey(
		ctx, "demo-game", "development", "op--happy")
	require.NoError(t, err)
	require.NotNil(t, published)
	assert.True(t, published.Active)
	assert.NotEmpty(t, published.BindingContractsJSON)
}

func mustProposal(proposal *model.PageProposal, err error) *model.PageProposal {
	if err != nil {
		panic(err)
	}
	return proposal
}
