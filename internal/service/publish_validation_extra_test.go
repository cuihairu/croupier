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

func mustMarshalPage(t *testing.T, page spec.PageSpec) []byte {
	t.Helper()
	raw, err := json.Marshal(page)
	require.NoError(t, err)
	return raw
}

// seedPublishableContract stores an enabled contract with the given schema.
func seedPublishableContract(t *testing.T, svc *ProposalService, functionID string, inputSchema string) {
	t.Helper()
	ctx := proposalTestContext()
	require.NoError(t, svc.contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  functionID,
		Enabled:     true,
		Version:     "1.0.0",
		InputSchema: []byte(inputSchema),
	}))
}

func TestAcceptAndPublish_ResourceQueryRequiresSelectors(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)
	seedPublishableContract(t, svc, "player.query", `{"type":"object"}`)

	// Resource page with a query binding that has no selectors at all.
	page := spec.PageSpec{
		PageKey:     "res--nosel",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家列表"},
		Category:    spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Resource:    &spec.ResourcePageSpec{},
		Bindings: []spec.PageFunctionBinding{
			{ID: "query", FunctionID: "player.query", Usage: spec.BindingUsageQuery,
				Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
		},
	}
	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "operation:res--nosel",
		PageKey: "res--nosel", PageType: "resource", ResourceKey: "player",
		Quality: "ready", Status: dbenum.ProposalStatusPending, PageSpec: mustMarshalPage(t, page),
	}
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))

	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selectors.output is required")
}

func TestAcceptAndPublish_InvalidSelectorDetails(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)
	seedPublishableContract(t, svc, "player.query",
		`{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`)

	page := testProposalPageSpec("op--badsel")
	page.Bindings[0].FunctionID = "player.query"
	page.Bindings[0].Selectors = &spec.BindingSelectors{
		Input: spec.SelectorAST{Assignments: []spec.InputAssignment{
			{Target: "not-a-pointer", Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/playerId"}},
		}},
	}
	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "operation:op--badsel",
		PageKey: "op--badsel", PageType: "operation", ResourceKey: "player",
		Quality: "ready", Status: dbenum.ProposalStatusPending, PageSpec: mustMarshalPage(t, page),
	}
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))

	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target must be a JSON Pointer")
}

func TestAcceptAndPublish_InvalidOutputAssignments(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)
	seedPublishableContract(t, svc, "player.query", `{"type":"object"}`)

	page := spec.PageSpec{
		PageKey:     "res--badout",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家列表"},
		Category:    spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Resource:    &spec.ResourcePageSpec{},
		Bindings: []spec.PageFunctionBinding{
			{
				ID: "query", FunctionID: "player.query", Usage: spec.BindingUsageQuery,
				Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
				Selectors: &spec.BindingSelectors{
					Output: []spec.OutputAssignment{{StateKey: "", Source: "bad-source"}},
				},
			},
			{
				ID: "act", FunctionID: "player.query", Usage: spec.BindingUsageAction,
				Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			},
		},
	}
	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "operation:res--badout",
		PageKey: "res--badout", PageType: "resource", ResourceKey: "player",
		Quality: "ready", Status: dbenum.ProposalStatusPending, PageSpec: mustMarshalPage(t, page),
	}
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))

	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output source must be a JSON Pointer")
	assert.Contains(t, err.Error(), "resource list query must map collection output")
}

func TestAcceptAndPublish_ConflictsWithExistingDraftOrPage(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)
	seedPublishableContract(t, svc, "player.query", `{"type":"object"}`)

	newPendingProposal := func(t *testing.T, key string) *model.PageProposal {
		t.Helper()
		proposal, err := buildOperationProposal(key, "player.query", func(p *spec.PageSpec) {
			p.Bindings = p.Bindings[:1]
			p.Bindings[0].Usage = spec.BindingUsageAction
			p.Bindings[0].ID = "act"
		})
		require.NoError(t, err)
		require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))
		return proposal
	}

	// Existing draft blocks acceptance.
	draftProposal := newPendingProposal(t, "op--draftclash")
	pageSpec, _, err := pageSpecFromProposal(draftProposal)
	require.NoError(t, err)
	draftJSON, err := json.Marshal(pageSpec)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: pageSpec.PageKey,
		Status: "draft", DraftRevision: 1, SpecJSON: string(draftJSON),
	}).Error)

	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", draftProposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// A plain accept reports the conflict as well.
	err = svc.AcceptProposal(ctx, "demo-game", "development", draftProposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft already exists")

	// Non-pending proposals are rejected up front.
	rejected, err := buildOperationProposal("op--rejected", "player.query", func(p *spec.PageSpec) {
		p.Bindings = p.Bindings[:1]
		p.Bindings[0].Usage = spec.BindingUsageAction
		p.Bindings[0].ID = "act"
	})
	require.NoError(t, err)
	rejected.Status = dbenum.ProposalStatusAccepted
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, rejected))
	err = svc.AcceptProposal(ctx, "demo-game", "development", rejected.ProposalKey)
	assert.Error(t, err)
	err = svc.RejectProposal(ctx, "demo-game", "development", rejected.ProposalKey)
	assert.Error(t, err)
}
