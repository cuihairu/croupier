package service

import (
	"context"
	"encoding/json"
	"errors"
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

type noopPublishValidator struct{}

func (noopPublishValidator) ValidatePageSpecInScope(_ context.Context, _, _ string, page spec.PageSpec, publish bool) []spec.Diagnostic {
	if !publish {
		return nil
	}
	var diagnostics []spec.Diagnostic
	functions := map[string]spec.FunctionSpec{
		"ghost.fn":     {Enabled: true},
		"fn.off":       {Enabled: false, InputSchema: spec.JSONSchema(`{"type":"object"}`)},
		"player.query": {Enabled: true},
		"mail.send":    {Enabled: true},
	}
	if page.PageKey == "op--missing" {
		return []spec.Diagnostic{{
			Code: "binding_function_not_found", Severity: spec.SeverityError,
			Message: "bound function does not exist: ghost.fn", Field: "query.functionId",
		}}
	}
	for _, binding := range page.Bindings {
		fn := functions[binding.FunctionID]
		if page.PageKey == "op--nosel" {
			fn.InputSchema = spec.JSONSchema(`{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`)
		}
		if page.PageKey == "res--badout" {
			fn.InputSchema = spec.JSONSchema(`{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`)
			fn.OutputSchema = spec.JSONSchema(`{"type":"object","properties":{"items":{"type":"array"}}}`)
		}
		if _, exists := functions[binding.FunctionID]; !exists {
			diagnostics = append(diagnostics, spec.Diagnostic{
				Code: "binding_function_not_found", Severity: spec.SeverityError,
				Message: "bound function does not exist: " + binding.FunctionID, Field: binding.ID + ".functionId",
			})
			continue
		}
		requiresInputSelectors := len(fn.InputSchema) > 0
		requiresOutputSelectors := page.Type == spec.PageTypeResource && binding.Usage == spec.BindingUsageQuery
		if requiresOutputSelectors && page.PageKey == "res--nosel" {
			diagnostics = append(diagnostics, spec.Diagnostic{
				Code: "binding_output_selector_missing", Severity: spec.SeverityError,
				Message: "selectors.output is required", Field: binding.ID + ".selectors.output",
			})
		}
		if requiresInputSelectors {
			if page.PageKey == "op--nosel" || page.PageKey == "res--badout" {
				if binding.Selectors == nil {
					diagnostics = append(diagnostics, spec.Diagnostic{
						Code: "binding_selector_missing", Severity: spec.SeverityError,
						Message: "selectors.input is required", Field: binding.ID + ".selectors.input",
					})
				}
			}
		}
		if fn.Enabled == false {
			diagnostics = append(diagnostics, spec.Diagnostic{
				Code: "binding_function_disabled", Severity: spec.SeverityError,
				Message: "bound function is disabled: " + binding.FunctionID,
				Field:   binding.ID + ".functionId",
			})
		}
		if binding.Selectors == nil {
			continue
		}
		input := spec.ValidateSelector(binding.Selectors.Input, fn.InputSchema, spec.SelectorContextForBinding(page, binding))
		for _, item := range input.Errors {
			diagnostics = append(diagnostics, spec.Diagnostic{
				Code: "binding_selector_" + item.Code, Severity: spec.SeverityError,
				Message: item.Message, Field: binding.ID + ".selectors.input." + item.Field,
			})
		}
		if requiresOutputSelectors {
			output := spec.ValidateOutputAssignments(binding.Selectors.Output, fn.OutputSchema)
			for _, item := range output.Errors {
				diagnostics = append(diagnostics, spec.Diagnostic{
					Code: "binding_output_selector_" + item.Code, Severity: spec.SeverityError,
					Message: item.Message, Field: binding.ID + ".selectors.output." + item.Field,
				})
			}
			if len(binding.Selectors.Output) == 0 {
				diagnostics = append(diagnostics, spec.Diagnostic{
					Code: "binding_output_selector_missing", Severity: spec.SeverityError,
					Message: "resource list query must map collection output", Field: binding.ID + ".selectors.output",
				})
			}
		}
	}
	return diagnostics
}

func TestAcceptAndPublish_RejectsBrokenBindings(t *testing.T) {
	db := setupTestDB(t)
	ctx := newPublishAuthorizedContext(t, db, "broken_binding_publisher")
	svc := NewProposalService(db)
	svc.SetPublishValidator(noopPublishValidator{})
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, mustProposal(buildOperationProposal("op--missing", "ghost.fn", nil))))
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "operation:op--missing")
	if err == nil {
		t.Fatal("expected error for missing binding")
	}
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
	ctx := newPublishAuthorizedContext(t, db, "selector_publisher")
	svc := NewProposalService(db)
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})
	svc.SetPublishValidator(noopPublishValidator{})

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

// T-M8 后分类名称由菜单系统（menu_items.labels）统一提供，提案发布不再
// 校验 category.labels 冲突，同 category key 的页面可并存发布。
func TestAcceptAndPublish_SharedCategoryKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := newPublishAuthorizedContext(t, db, "category_publisher")
	svc := NewProposalService(db)
	svc.SetPublishValidator(noopPublishValidator{})
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)

	// Publish the first page for category "player".
	first, err := buildOperationProposal("cat--first", "player.query", nil)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, first))
	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", first.ProposalKey)
	require.NoError(t, err)

	// A second page in the same category publishes successfully.
	second, err := buildOperationProposal("cat--second", "player.query", nil)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, second))
	result, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", second.ProposalKey)
	require.NoError(t, err)
	assert.Equal(t, "cat--second", result.PageKey)
}

func TestAcceptAndPublish_HappyPathFreezesSnapshot(t *testing.T) {
	db := setupTestDB(t)
	ctx := newPublishAuthorizedContext(t, db, "happy_path_publisher")
	svc := NewProposalService(db)
	svc.SetPublishValidator(noopPublishValidator{})
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})

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

	draft, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(
		ctx, "demo-game", "development", "op--happy")
	require.NoError(t, err)
	require.NotNil(t, draft)
	assert.Equal(t, "draft", draft.Status)
	assert.Equal(t, 1, draft.DraftRevision)
}

type erringDraftPublisher struct {
	message string
}

type strictPublishValidator struct{}

func (strictPublishValidator) ValidatePageSpecInScope(context.Context, string, string, spec.PageSpec, bool) []spec.Diagnostic {
	return nil
}

func (p erringDraftPublisher) PublishProposalDraft(context.Context, string, string, string, string, int, string) error {
	return errors.New(p.message)
}

func mustProposal(proposal *model.PageProposal, err error) *model.PageProposal {
	if err != nil {
		panic(err)
	}
	return proposal
}
