package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// --- ExportAllPages / ExportToJSON ---

func TestDataExportService_ExportAllPages(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewDataExportService(db)

	// Create some page specs
	ps1 := &model.PageSpec{
		GameID:    "game1",
		Env:       "dev",
		PageKey:   "page-a",
		Type:      "operation",
		TitleJSON: `{"zh-CN":"页面A"}`,
		SpecJSON:  `{"pageKey":"page-a"}`,
		Status:    "draft",
	}
	ps2 := &model.PageSpec{
		GameID:    "game1",
		Env:       "dev",
		PageKey:   "page-b",
		Type:      "resource",
		TitleJSON: `{"zh-CN":"页面B"}`,
		SpecJSON:  `{"pageKey":"page-b"}`,
		Status:    "published",
	}
	ps3 := &model.PageSpec{
		GameID:    "game1",
		Env:       "dev",
		PageKey:   "page-c",
		Type:      "operation",
		TitleJSON: `{"zh-CN":"页面C"}`,
		SpecJSON:  `{"pageKey":"page-c"}`,
		Status:    "archived",
	}
	require.NoError(t, db.Create(ps1).Error)
	require.NoError(t, db.Create(ps2).Error)
	require.NoError(t, db.Create(ps3).Error)

	// Create published pages
	pp1 := &model.PublishedPageSpec{
		GameID:      "game1",
		Env:         "dev",
		PageKey:     "page-b",
		Version:     1,
		SpecJSON:    `{"pageKey":"page-b"}`,
		Active:      true,
		PublishedBy: "admin",
	}
	require.NoError(t, db.Create(pp1).Error)

	report, err := svc.ExportAllPages(ctx, "game1", "dev")
	require.NoError(t, err)
	assert.Equal(t, "game1", report.GameID)
	assert.Equal(t, "dev", report.Env)
	assert.Len(t, report.PageSpecs, 3)
	assert.Len(t, report.PublishedPages, 1)
	assert.Equal(t, 3, report.Summary.TotalPageSpecs)
	assert.Equal(t, 1, report.Summary.DraftPages)
	assert.Equal(t, 1, report.Summary.PublishedPages)
	assert.Equal(t, 1, report.Summary.ArchivedPages)
	assert.Equal(t, 1, report.Summary.TotalPublishedPages)
}

func TestDataExportService_ExportAllPages_EmptyScope(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewDataExportService(db)

	// Empty game/env - should return empty report
	report, err := svc.ExportAllPages(ctx, "", "")
	require.NoError(t, err)
	assert.Empty(t, report.PageSpecs)
	assert.Empty(t, report.PublishedPages)
}

func TestDataExportService_ExportAllPages_FilterByScope(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewDataExportService(db)

	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "g1", Env: "dev", PageKey: "p1", Type: "operation",
		SpecJSON: "{}", Status: "draft",
	}).Error)
	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "g2", Env: "dev", PageKey: "p2", Type: "operation",
		SpecJSON: "{}", Status: "draft",
	}).Error)

	report, err := svc.ExportAllPages(ctx, "g1", "dev")
	require.NoError(t, err)
	assert.Len(t, report.PageSpecs, 1)
	assert.Equal(t, "p1", report.PageSpecs[0].PageKey)
}

func TestDataExportService_ExportToJSON(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewDataExportService(db)

	data, err := svc.ExportToJSON(ctx, "game1", "dev")
	require.NoError(t, err)
	assert.NotNil(t, data)
	assert.Contains(t, string(data), "exportedAt")
}

// --- ProposalService helper functions ---

func TestProposalService_ListProposalDTOs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	// Create proposals with different statuses
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusPending,
		ResourceKey: "player", PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "o:m1", PageKey: "pk2",
		PageType: "operation", Quality: "basic", Status: dbenum.ProposalStatusAccepted,
		ResourceKey: "mail", PageSpec: []byte(`{"pageKey":"pk2","type":"operation"}`),
	}))

	// List all
	dtos, err := svc.ListProposalDTOs(ctx, "g1", "dev", ProposalListFilter{})
	require.NoError(t, err)
	assert.Len(t, dtos, 2)

	// Filter by status
	dtos, err = svc.ListProposalDTOs(ctx, "g1", "dev", ProposalListFilter{Status: "pending"})
	require.NoError(t, err)
	assert.Len(t, dtos, 1)
	assert.Equal(t, "pending", dtos[0].Status)

	// Filter by resourceKey
	dtos, err = svc.ListProposalDTOs(ctx, "g1", "dev", ProposalListFilter{ResourceKey: "mail"})
	require.NoError(t, err)
	assert.Len(t, dtos, 1)
	assert.Equal(t, "o:m1", dtos[0].ProposalKey)

	// Filter by both
	dtos, err = svc.ListProposalDTOs(ctx, "g1", "dev", ProposalListFilter{Status: "pending", ResourceKey: "player"})
	require.NoError(t, err)
	assert.Len(t, dtos, 1)
}

func TestProposalService_GetProposalDTO(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusPending,
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))

	dto, err := svc.GetProposalDTO(ctx, "g1", "dev", "r:p1")
	require.NoError(t, err)
	assert.Equal(t, "r:p1", dto.ProposalKey)
	assert.Equal(t, "pk1", dto.PageKey)
}

func TestProposalService_GetProposalDTO_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	_, err := svc.GetProposalDTO(ctx, "g1", "dev", "nonexistent")
	require.Error(t, err)
}

func TestProposalService_Inbox(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	// Create publishable proposal (pending + ready + no error diagnostics)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusPending,
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))

	// Create needs_review proposal (pending + basic but has error diagnostics)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "o:p2", PageKey: "pk2",
		PageType: "operation", Quality: "basic", Status: dbenum.ProposalStatusPending,
		PageSpec:    []byte(`{"pageKey":"pk2","type":"operation"}`),
		Diagnostics: datatypes.JSON(`[{"code":"x","severity":"error","message":"err"}]`),
	}))

	// Create rejected proposal (should not appear in any queue)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "o:p3", PageKey: "pk3",
		PageType: "operation", Quality: "ready", Status: dbenum.ProposalStatusRejected,
		PageSpec: []byte(`{"pageKey":"pk3","type":"operation"}`),
	}))

	inbox, err := svc.Inbox(ctx, "g1", "dev", ProposalListFilter{})
	require.NoError(t, err)
	assert.Len(t, inbox.Publishable, 1, "should have 1 publishable proposal")
	assert.Len(t, inbox.NeedsReview, 1, "should have 1 needs_review proposal")
	assert.Equal(t, 1, inbox.Summary.Publishable)
	assert.Equal(t, 1, inbox.Summary.NeedsReview)
}

// --- Helper function tests ---

func TestStringSliceFromJSONV2(t *testing.T) {
	t.Parallel()

	// nil
	assert.Nil(t, stringSliceFromJSON(nil))
	// empty
	assert.Nil(t, stringSliceFromJSON([]byte{}))
	// valid
	result := stringSliceFromJSON([]byte(`["a","b","c"]`))
	assert.Equal(t, []string{"a", "b", "c"}, result)
	// with empty strings (should be filtered)
	result = stringSliceFromJSON([]byte(`["a","","  ","b"]`))
	assert.Equal(t, []string{"a", "b"}, result)
	// invalid JSON
	assert.Nil(t, stringSliceFromJSON([]byte(`not json`)))
}

func TestNormalizeJSONMapV2(t *testing.T) {
	t.Parallel()

	// nil
	assert.Nil(t, normalizeJSONMap(nil))
	// empty
	assert.Nil(t, normalizeJSONMap(map[string]interface{}{}))
	// all empty strings
	assert.Nil(t, normalizeJSONMap(map[string]interface{}{"a": "  ", "b": ""}))
	// valid
	result := normalizeJSONMap(map[string]interface{}{"zh-CN": "标题", "en-US": "Title"})
	assert.Equal(t, "标题", result["zh-CN"])
	assert.Equal(t, "Title", result["en-US"])
	// non-string values are skipped
	result = normalizeJSONMap(map[string]interface{}{"key": 42})
	assert.Nil(t, result)
}

func TestDiagnosticsFromJSONV2(t *testing.T) {
	t.Parallel()

	// nil
	assert.Nil(t, diagnosticsFromJSON(nil))
	// invalid JSON
	diags := diagnosticsFromJSON([]byte(`{invalid`))
	require.Len(t, diags, 1)
	assert.Equal(t, "proposal_diagnostics_invalid", diags[0].Code)
	// valid
	diags = diagnosticsFromJSON([]byte(`[{"code":"x","severity":"warning","message":"m"}]`))
	require.Len(t, diags, 1)
	assert.Equal(t, "x", diags[0].Code)
}

func TestHasBlockingDiagnosticsV2(t *testing.T) {
	t.Parallel()

	assert.False(t, hasBlockingDiagnostics(nil))
	assert.False(t, hasBlockingDiagnostics([]byte{}))
	assert.False(t, hasBlockingDiagnostics([]byte(`[]`)))
	assert.False(t, hasBlockingDiagnostics([]byte(`not json`)))
	assert.False(t, hasBlockingDiagnostics([]byte(`[{"severity":"warning"}]`)))
	assert.True(t, hasBlockingDiagnostics([]byte(`[{"severity":"error"}]`)))
}

func TestHasDiagnosticSeverityV2(t *testing.T) {
	t.Parallel()

	assert.False(t, hasDiagnosticSeverity(nil, "error"))
	assert.False(t, hasDiagnosticSeverity([]spec.Diagnostic{}, "error"))
	assert.True(t, hasDiagnosticSeverity([]spec.Diagnostic{{Severity: "error"}}, "error"))
	assert.False(t, hasDiagnosticSeverity([]spec.Diagnostic{{Severity: "warning"}}, "error"))
}

func TestProposalDTOFromModelV2(t *testing.T) {
	t.Parallel()

	// nil
	_, err := proposalDTOFromModel(nil)
	assert.Error(t, err)

	// empty PageSpec
	_, err = proposalDTOFromModel(&model.PageProposal{
		ProposalKey: "r:p1",
		PageKey:     "pk1",
		PageType:    "resource",
		PageSpec:    []byte{},
	})
	assert.Error(t, err)

	// invalid PageSpec JSON
	_, err = proposalDTOFromModel(&model.PageProposal{
		ProposalKey: "r:p1",
		PageKey:     "pk1",
		PageType:    "resource",
		PageSpec:    []byte(`{invalid`),
	})
	assert.Error(t, err)
}

func TestCreateProposalVersionSnapshot_NilProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	versionModel := model.NewPageProposalVersionModel(db)

	_, err := createProposalVersionSnapshot(ctx, versionModel, nil, "reason", "actor")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be persisted")
}

func TestCreateProposalVersionSnapshot_ZeroID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	versionModel := model.NewPageProposalVersionModel(db)

	_, err := createProposalVersionSnapshot(ctx, versionModel, &model.PageProposal{}, "reason", "actor")
	assert.Error(t, err)
}

func TestProposalService_AcceptProposal_NotPending(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusAccepted,
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))

	err := svc.AcceptProposal(ctx, "g1", "dev", "r:p1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestProposalService_RejectProposal_NotPending(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusAccepted,
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))

	err := svc.RejectProposal(ctx, "g1", "dev", "r:p1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestProposalService_RejectProposal_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	err := svc.RejectProposal(ctx, "g1", "dev", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProposalService_AcceptAndPublishProposal_NotPending(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusAccepted,
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))

	_, err := svc.AcceptAndPublishProposal(ctx, "g1", "dev", "r:p1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestProposalService_AcceptAndPublishProposal_NotReadyOrBasic(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "needs_review", Status: dbenum.ProposalStatusPending,
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))

	_, err := svc.AcceptAndPublishProposal(ctx, "g1", "dev", "r:p1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only ready/basic")
}

func TestProposalService_AcceptAndPublishProposal_HasBlockingDiagnostics(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusPending,
		PageSpec:    []byte(`{"pageKey":"pk1","type":"resource"}`),
		Diagnostics: datatypes.JSON(`[{"severity":"error","message":"err"}]`),
	}))

	_, err := svc.AcceptAndPublishProposal(ctx, "g1", "dev", "r:p1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocking diagnostics")
}

func TestProposalService_AcceptAndPublishProposal_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	_, err := svc.AcceptAndPublishProposal(ctx, "g1", "dev", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProposalService_AcceptProposal_HasBlockingDiagnostics(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	page := testProposalPageSpec("resource--player")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "resource--player",
		PageType: "resource", Quality: "ready", Status: dbenum.ProposalStatusPending,
		PageSpec:    pageJSON,
		Diagnostics: datatypes.JSON(`[{"severity":"error","message":"blocking"}]`),
	}))

	err = svc.AcceptProposal(ctx, "g1", "dev", "r:p1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocking diagnostics")
}

// --- PageSpec helper tests ---

func TestPageSpecFromProposalV2(t *testing.T) {
	t.Parallel()

	// nil
	_, _, err := pageSpecFromProposal(nil)
	assert.Error(t, err)

	// empty PageSpec
	_, _, err = pageSpecFromProposal(&model.PageProposal{PageSpec: []byte{}})
	assert.Error(t, err)

	// invalid JSON
	_, _, err = pageSpecFromProposal(&model.PageProposal{PageSpec: []byte(`{invalid`)})
	assert.Error(t, err)

	// valid
	_, _, err = pageSpecFromProposal(&model.PageProposal{
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource","title":{"zh-CN":"标题"},"category":{"key":"cat","labels":{"zh-CN":"类别"}}}`),
	})
	assert.NoError(t, err)
}

func TestValidateAcceptedPageSpecV2(t *testing.T) {
	t.Parallel()

	proposal := &model.PageProposal{PageKey: "pk1"}

	// Missing gameID
	page := spec.PageSpec{
		PageKey:  "pk1",
		Type:     spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "标题"},
		Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "类别"}},
		Resource: &spec.ResourcePageSpec{},
		Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "f1", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
	}
	err := validateAcceptedPageSpec("", "dev", proposal, page)
	assert.Error(t, err)

	// Missing env
	err = validateAcceptedPageSpec("g1", "", proposal, page)
	assert.Error(t, err)

	// Missing pageKey
	page.PageKey = ""
	err = validateAcceptedPageSpec("g1", "dev", proposal, page)
	assert.Error(t, err)
	page.PageKey = "pk1"

	// Invalid type
	page.Type = "invalid"
	err = validateAcceptedPageSpec("g1", "dev", proposal, page)
	assert.Error(t, err)
	page.Type = spec.PageTypeResource

	// Missing title
	page.Title = nil
	err = validateAcceptedPageSpec("g1", "dev", proposal, page)
	assert.Error(t, err)
	page.Title = spec.LocalizedText{"zh-CN": "标题"}

	// Missing category key
	page.Category.Key = ""
	err = validateAcceptedPageSpec("g1", "dev", proposal, page)
	assert.Error(t, err)
	page.Category.Key = "cat"

	// Missing category labels
	page.Category.Labels = nil
	err = validateAcceptedPageSpec("g1", "dev", proposal, page)
	assert.Error(t, err)
	page.Category.Labels = spec.LocalizedText{"zh-CN": "类别"}

	// Missing bindings
	page.Bindings = nil
	err = validateAcceptedPageSpec("g1", "dev", proposal, page)
	assert.Error(t, err)

	// Shape mismatch (resource type without resource body)
	page.Bindings = []spec.PageFunctionBinding{{ID: "b1", FunctionID: "f1", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}}
	page.Resource = nil
	err = validateAcceptedPageSpec("g1", "dev", proposal, page)
	assert.Error(t, err)
}

func TestIsValidProposalPageTypeV2(t *testing.T) {
	t.Parallel()

	assert.True(t, isValidProposalPageType(spec.PageTypeResource))
	assert.True(t, isValidProposalPageType(spec.PageTypeOperation))
	assert.True(t, isValidProposalPageType(spec.PageTypeTask))
	assert.True(t, isValidProposalPageType(spec.PageTypeReport))
	assert.False(t, isValidProposalPageType("invalid"))
}

func TestPageShapeMatchesTypeV2(t *testing.T) {
	t.Parallel()

	assert.True(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeResource, Resource: &spec.ResourcePageSpec{}}))
	assert.False(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeResource}))
	assert.True(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeOperation, Operation: &spec.OperationPageSpec{}}))
	assert.False(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeOperation}))
	assert.True(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeTask, Task: &spec.TaskPageSpec{}}))
	assert.False(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeTask}))
	assert.True(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeReport, Report: &spec.ReportPageSpec{}}))
	assert.False(t, pageShapeMatchesType(spec.PageSpec{Type: spec.PageTypeReport}))
	assert.False(t, pageShapeMatchesType(spec.PageSpec{Type: "invalid"}))
}

func TestIsValidBindingUsageV2(t *testing.T) {
	t.Parallel()

	assert.True(t, isValidBindingUsage(spec.BindingUsageQuery))
	assert.True(t, isValidBindingUsage(spec.BindingUsageDetail))
	assert.True(t, isValidBindingUsage(spec.BindingUsageAction))
	assert.True(t, isValidBindingUsage(spec.BindingUsageTask))
	assert.True(t, isValidBindingUsage(spec.BindingUsageReport))
	assert.False(t, isValidBindingUsage("invalid"))
}

func TestIsValidExecutionModeV2(t *testing.T) {
	t.Parallel()

	assert.True(t, isValidExecutionMode(spec.PageExecutionModeSync))
	assert.True(t, isValidExecutionMode(spec.PageExecutionModeTask))
	assert.False(t, isValidExecutionMode("invalid"))
}

func TestExecutionModeForFunctionSpecV2(t *testing.T) {
	t.Parallel()

	assert.Equal(t, spec.PageExecutionModeTask, executionModeForFunctionSpec(spec.FunctionSpec{Execution: spec.FunctionExecutionTask}))
	assert.Equal(t, spec.PageExecutionModeSync, executionModeForFunctionSpec(spec.FunctionSpec{Execution: spec.FunctionExecutionSync}))
}

func TestIsMissingTableErrV2(t *testing.T) {
	t.Parallel()

	assert.False(t, isMissingTableErr(nil))
	assert.False(t, isMissingTableErr(fmt.Errorf("some error")))
	assert.True(t, isMissingTableErr(fmt.Errorf("no such table: foo")))
	assert.True(t, isMissingTableErr(fmt.Errorf("table does not exist")))
	assert.True(t, isMissingTableErr(fmt.Errorf("undefined table: bar")))
}

func TestNormalizeLocalizedTextV2(t *testing.T) {
	t.Parallel()

	assert.Nil(t, normalizeLocalizedText(nil))
	assert.Nil(t, normalizeLocalizedText(map[string]string{}))
	assert.Nil(t, normalizeLocalizedText(map[string]string{"a": "  "}))

	result := normalizeLocalizedText(map[string]string{"zh": "标题", "en": "Title"})
	assert.Equal(t, "标题", result["zh-CN"])
	assert.Equal(t, "Title", result["en-US"])

	result = normalizeLocalizedText(map[string]string{"zh-cn": "标题2"})
	assert.Equal(t, "标题2", result["zh-CN"])

	result = normalizeLocalizedText(map[string]string{"custom": "value"})
	assert.Equal(t, "value", result["custom"])
}

func TestHasDefaultLocaleV2(t *testing.T) {
	t.Parallel()

	assert.False(t, hasDefaultLocale(nil))
	assert.False(t, hasDefaultLocale(map[string]string{}))
	assert.False(t, hasDefaultLocale(map[string]string{"zh-CN": "  "}))
	assert.True(t, hasDefaultLocale(map[string]string{"zh-CN": "标题"}))
}

func TestLocalizedTextEqualV2(t *testing.T) {
	t.Parallel()

	assert.True(t, localizedTextEqual(nil, nil))
	assert.True(t, localizedTextEqual(map[string]string{}, map[string]string{}))
	assert.True(t, localizedTextEqual(
		map[string]string{"zh-CN": "标题"},
		map[string]string{"zh-CN": "标题"},
	))
	assert.False(t, localizedTextEqual(
		map[string]string{"zh-CN": "标题A"},
		map[string]string{"zh-CN": "标题B"},
	))
	assert.False(t, localizedTextEqual(
		map[string]string{"zh-CN": "A"},
		map[string]string{"en-US": "B"},
	))
}

func TestSchemaHasFieldsV2(t *testing.T) {
	t.Parallel()

	assert.False(t, schemaHasFields(nil))
	assert.False(t, schemaHasFields(spec.JSONSchema{}))
	assert.False(t, schemaHasFields(spec.JSONSchema(`{"type":"object"}`)))
	assert.True(t, schemaHasFields(spec.JSONSchema(`{"properties":{"name":{"type":"string"}}}`)))
	assert.True(t, schemaHasFields(spec.JSONSchema(`{"required":["name"]}`)))
}

func TestBindingRequiresOutputSelectorsV2(t *testing.T) {
	t.Parallel()

	assert.True(t, bindingRequiresOutputSelectors(
		spec.PageFunctionBinding{Usage: spec.BindingUsageQuery},
		spec.PageSpec{Type: spec.PageTypeResource},
	))
	assert.False(t, bindingRequiresOutputSelectors(
		spec.PageFunctionBinding{Usage: spec.BindingUsageQuery},
		spec.PageSpec{Type: spec.PageTypeOperation},
	))
	assert.True(t, bindingRequiresOutputSelectors(
		spec.PageFunctionBinding{Usage: spec.BindingUsageReport},
		spec.PageSpec{Type: spec.PageTypeReport},
	))
	assert.False(t, bindingRequiresOutputSelectors(
		spec.PageFunctionBinding{Usage: spec.BindingUsageAction},
		spec.PageSpec{Type: spec.PageTypeResource},
	))
}

func TestActorFromContextV2(t *testing.T) {
	t.Parallel()

	// Without username
	actor := actorFromContext(context.Background())
	assert.Equal(t, "system", actor)

	// With username
	ctx := context.WithValue(context.Background(), "username", "admin")
	actor = actorFromContext(ctx)
	assert.Equal(t, "admin", actor)
}

func TestDigestJSONV2(t *testing.T) {
	t.Parallel()

	assert.Empty(t, digestJSON(nil))
	assert.Empty(t, digestJSON(datatypes.JSON{}))
	assert.NotEmpty(t, digestJSON(datatypes.JSON(`{"key":"value"}`)))
}

func TestParsePublishedSnapshotV2(t *testing.T) {
	t.Parallel()

	// Empty spec
	item := model.PublishedPageSpec{PageKey: "pk1"}
	page, contracts := parsePublishedSnapshot(item)
	assert.Equal(t, "pk1", page.PageKey)
	assert.Nil(t, contracts)

	// With spec
	item = model.PublishedPageSpec{
		PageKey:              "pk2",
		SpecJSON:             `{"pageKey":"pk2","type":"resource","title":{"zh-CN":"标题"},"category":{"key":"cat","labels":{"zh-CN":"类别"}},"bindings":[{"id":"b1","functionId":"f1","usage":"query","execution":{"mode":"sync"}}]}`,
		BindingContractsJSON: `[{"bindingID":"b1","functionID":"f1"}]`,
	}
	page, contracts = parsePublishedSnapshot(item)
	assert.Equal(t, "pk2", page.PageKey)
	assert.Len(t, contracts, 1)
}

func TestParseDraftPageSpecV2(t *testing.T) {
	t.Parallel()

	// Empty spec - falls back to model fields
	item := model.PageSpec{
		PageKey:       "pk1",
		Type:          "operation",
		ResourceKey:   "player",
		CategoryKey:   "player",
		CategoryOrder: 1,
	}
	page := parseDraftPageSpec(item)
	assert.Equal(t, "pk1", page.PageKey)
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "player", page.ResourceKey)

	// With spec
	item.SpecJSON = `{"pageKey":"pk2","type":"resource","title":{"zh-CN":"标题"},"category":{"key":"cat","labels":{"zh-CN":"类别"}}}`
	page = parseDraftPageSpec(item)
	assert.Equal(t, "pk2", page.PageKey)
}

func TestBlockedIssueDTOFromModelV2(t *testing.T) {
	t.Parallel()

	issue := &model.BlockedProposalIssue{
		GameID:      "g1",
		Env:         "dev",
		ResourceKey: " player ",
		FunctionID:  " f1 ",
		Diagnostics: []byte(`[{"code":"x","severity":"error","message":"err"}]`),
		Status:      "open",
		UpdatedBy:   "system",
	}
	dto := blockedIssueDTOFromModel(issue)
	assert.Equal(t, "player", dto.ResourceKey)
	assert.Equal(t, "f1", dto.FunctionID)
	assert.Equal(t, "open", dto.Status)
	assert.Len(t, dto.Diagnostics, 1)
}
