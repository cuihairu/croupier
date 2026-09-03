package page

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// V9 helpers
// ---------------------------------------------------------------------------

// v9NoScopeCtx returns a context with an authenticated username but no game
// scope, so permission guards pass while requireScope fails.
func v9NoScopeCtx() context.Context {
	return context.WithValue(context.Background(), "username", "page_tester")
}

// v9UpsertProposal inserts a proposal row directly without side effects.
func v9UpsertProposal(t *testing.T, db *gorm.DB, ctx context.Context, proposalKey, pageKey string, status dbenum.ProposalStatus, pageSpecJSON []byte) *model.PageProposal {
	t.Helper()
	p := &model.PageProposal{
		GameID:           "demo-game",
		Env:              "development",
		ProposalKey:      proposalKey,
		PageKey:          pageKey,
		PageType:         "operation",
		Quality:          "basic",
		GeneratorVersion: "test-generator",
		PageSpec:         model.JSON(pageSpecJSON),
		Status:           status,
		UpdatedBy:        "v9",
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, p))
	return p
}

func v9CreateProposalVersion(t *testing.T, db *gorm.DB, ctx context.Context, proposalID uint) {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"snapshot": "v9"})
	require.NoError(t, err)
	v := &model.PageProposalVersion{
		ProposalID: proposalID,
		Version:    1,
		Proposal:   model.JSON(raw),
		CreatedBy:  "v9",
	}
	require.NoError(t, model.NewPageProposalVersionModel(db).CreateVersion(ctx, v))
}

func v9MarshalSpec(t *testing.T, page spec.PageSpec) []byte {
	t.Helper()
	raw, err := json.Marshal(page)
	require.NoError(t, err)
	return raw
}

// v9SaveDraftDirect saves an operation page draft through the service API.
func v9SaveDraftDirect(t *testing.T, service *Service, ctx context.Context, pageKey string, revision int) int {
	t.Helper()
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       pageKey,
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "页面"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)
	return resp.DraftRevision
}

// ---------------------------------------------------------------------------
// Scope / permission guard branches
// ---------------------------------------------------------------------------

func TestServiceRequireScopeGuardsV9(t *testing.T) {
	service, _, _ := newPageTestService(t, "admin:all")
	ctx := v9NoScopeCtx()
	rev := 0

	_, err := service.ListDrafts(ctx, &PageDraftListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID")

	_, err = service.GetDraft(ctx, &PageDraftRequest{PageKey: "p"})
	require.Error(t, err)

	_, err = service.Validate(ctx, &PageValidateRequest{PageKey: "p"})
	require.Error(t, err)

	_, err = service.Preview(ctx, &PagePreviewRequest{PageKey: "p"})
	require.Error(t, err)

	_, err = service.SaveDraft(ctx, &PageSaveRequest{PageKey: "p", DraftRevision: &rev})
	require.Error(t, err)

	_, err = service.RegenerateDraft(ctx, &PageRegenerateRequest{PageKey: "p", DraftRevision: &rev})
	require.Error(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{PageKey: "p", DraftRevision: &rev})
	require.Error(t, err)

	_, err = service.Unpublish(ctx, &PageUnpublishRequest{PageKey: "p"})
	require.Error(t, err)

	_, err = service.Versions(ctx, &PageVersionsRequest{PageKey: "p"})
	require.Error(t, err)

	_, err = service.VersionDetail(ctx, &PageVersionDetailRequest{PageKey: "p", VersionID: "1"})
	require.Error(t, err)

	_, err = service.Rollback(ctx, &PageRollbackRequest{PageKey: "p", VersionID: "1", ExpectedDraftRevision: &rev})
	require.Error(t, err)
}

func TestServiceGetDraftPermissionDeniedV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t) // no permissions granted

	_, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权")
}

// ---------------------------------------------------------------------------
// SaveDraft / RegenerateDraft / Publish error branches
// ---------------------------------------------------------------------------

func TestServiceSaveDraftConflictOnExistingPageV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")

	rev1 := v9SaveDraftDirect(t, service, ctx, "conflict.page", 0)
	assert.Equal(t, 1, rev1)

	stale := 0
	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "conflict.page",
		DraftRevision: &stale,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "页面"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "草稿版本冲突")
}

func TestServiceCorruptSpecDraftPathsV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	require.NoError(t, service.svcCtx.DB.Create(&model.PageSpec{
		GameID:        "demo-game",
		Env:           "development",
		PageKey:       "corrupt.spec",
		Type:          "operation",
		Status:        "draft",
		DraftRevision: 1,
		SpecJSON:      "{bad json",
	}).Error)

	_, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "corrupt.spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical PageSpec")

	_, err = service.Validate(ctx, &PageValidateRequest{PageKey: "corrupt.spec"})
	require.Error(t, err)

	_, err = service.Preview(ctx, &PagePreviewRequest{PageKey: "corrupt.spec"})
	require.Error(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{PageKey: "corrupt.spec", DraftRevision: intPtr(1)})
	require.Error(t, err)
}

func TestServiceRegenerateDraftApplyErrorV9(t *testing.T) {
	env := setupPageFlowEnv(t)
	rev := env.saveDraft(t, "regen.nocat", 0)

	// Proposal spec without category.key passes parsing but fails
	// applyPageSpecToModel.
	upsertProposalForRegenerate(t, env.service.svcCtx.DB, env.ctx, "prop:nocat", "regen.nocat", spec.PageSpec{
		PageKey: "regen.nocat",
		Type:    spec.PageTypeOperation,
	})

	_, err := env.service.RegenerateDraft(env.ctx, &PageRegenerateRequest{PageKey: "regen.nocat", DraftRevision: &rev})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "category.key")
}

// ---------------------------------------------------------------------------
// proposalReplacementForDraft branches
// ---------------------------------------------------------------------------

func TestProposalReplacementForDraftNilDraftV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")

	_, err := service.proposalReplacementForDraft(ctx, "demo-game", "development", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft is required")
}

func TestProposalReplacementForDraftByKeyV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	specJSON := v9MarshalSpec(t, spec.PageSpec{
		PageKey:  "bykey.page",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "页面"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	p := v9UpsertProposal(t, db, ctx, "prop:bykey", "bykey.page", dbenum.ProposalStatusPending, specJSON)
	v9CreateProposalVersion(t, db, ctx, p.ID)

	draft := &model.PageSpec{
		GameID:          "demo-game",
		Env:             "development",
		PageKey:         "bykey.page",
		BaseProposalKey: "prop:bykey",
	}
	repl, err := service.proposalReplacementForDraft(ctx, "demo-game", "development", draft)
	require.NoError(t, err)
	assert.Equal(t, "prop:bykey", repl.ProposalKey)
	assert.Equal(t, 1, repl.ProposalVersion)
	assert.Equal(t, "bykey.page", repl.PageSpec.PageKey)
}

func TestProposalReplacementForDraftRejectedStatusV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	specJSON := v9MarshalSpec(t, spec.PageSpec{
		PageKey:  "rejected.page",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	v9UpsertProposal(t, db, ctx, "prop:rejected", "rejected.page", dbenum.ProposalStatusRejected, specJSON)

	draft := &model.PageSpec{
		GameID:          "demo-game",
		Env:             "development",
		PageKey:         "rejected.page",
		BaseProposalKey: "prop:rejected",
	}
	_, err := service.proposalReplacementForDraft(ctx, "demo-game", "development", draft)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not usable")
}

func TestProposalReplacementForDraftInvalidSpecV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	v9UpsertProposal(t, db, ctx, "prop:badjson", "badjson.page", dbenum.ProposalStatusPending, []byte("{not json"))

	draft := &model.PageSpec{
		GameID:          "demo-game",
		Env:             "development",
		PageKey:         "badjson.page",
		BaseProposalKey: "prop:badjson",
	}
	_, err := service.proposalReplacementForDraft(ctx, "demo-game", "development", draft)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestProposalReplacementForDraftPageKeyMismatchV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	specJSON := v9MarshalSpec(t, spec.PageSpec{
		PageKey:  "other.page",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	p := v9UpsertProposal(t, db, ctx, "prop:mismatch", "mismatch.page", dbenum.ProposalStatusPending, specJSON)
	v9CreateProposalVersion(t, db, ctx, p.ID)

	draft := &model.PageSpec{GameID: "demo-game", Env: "development", PageKey: "mismatch.page"}
	_, err := service.proposalReplacementForDraft(ctx, "demo-game", "development", draft)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestProposalReplacementForDraftMissingVersionV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	specJSON := v9MarshalSpec(t, spec.PageSpec{
		PageKey:  "noversion.page",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	v9UpsertProposal(t, db, ctx, "prop:none", "noversion.page", dbenum.ProposalStatusPending, specJSON)

	draft := &model.PageSpec{GameID: "demo-game", Env: "development", PageKey: "noversion.page"}
	_, err := service.proposalReplacementForDraft(ctx, "demo-game", "development", draft)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

func TestValidatePageSpecDuplicateBindingIDV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")

	page := spec.PageSpec{
		PageKey:  "dup.page",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "重复"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Bindings: []spec.PageFunctionBinding{
			{ID: "dup", FunctionID: "player.query", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
			{ID: "dup", FunctionID: "player.action", Usage: spec.BindingUsageAction, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
		},
	}
	diags := service.validatePageSpec(ctx, page, false)
	assertDiagnostic(t, diags, "binding_id_duplicate", "bindings[1].id")
}

func TestValidatePublishedCategoryLabelsGuardsV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")

	// nil service context
	assert.Nil(t, NewService(nil).validatePublishedCategoryLabels(ctx, spec.PageSpec{
		Category: spec.PageCategorySpec{Key: "player"},
	}))

	// empty category key short-circuits
	assert.Nil(t, service.validatePublishedCategoryLabels(ctx, spec.PageSpec{}))

	// missing scope short-circuits
	assert.Nil(t, service.validatePublishedCategoryLabels(context.Background(), spec.PageSpec{
		Category: spec.PageCategorySpec{Key: "player"},
	}))
}

func TestValidatePublishedCategoryLabelsPublishedRowsV9(t *testing.T) {
	env := setupPageFlowEnv(t)
	db := env.service.svcCtx.DB

	rev := env.saveDraft(t, "cat.page", 0)
	_, err := env.service.Publish(env.ctx, &PagePublishRequest{PageKey: "cat.page", DraftRevision: &rev})
	require.NoError(t, err)

	labels := spec.LocalizedText{"zh-CN": "玩家"}

	// Same pageKey rows are skipped.
	assert.Nil(t, env.service.validatePublishedCategoryLabels(env.ctx, spec.PageSpec{
		PageKey:  "cat.page",
		Category: spec.PageCategorySpec{Key: "player", Labels: labels},
	}))

	// Published rows with corrupt SpecJSON produce a diagnostic.
	require.NoError(t, db.Create(&model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: "badpub.page",
		Version: 1, SpecJSON: "{bad", RendererSchemaVersion: rendererSchemaVersion, Active: true,
	}).Error)
	diags := env.service.validatePublishedCategoryLabels(env.ctx, spec.PageSpec{
		PageKey:  "cat.page",
		Category: spec.PageCategorySpec{Key: "player", Labels: labels},
	})
	require.NotEmpty(t, diags)
	assert.Equal(t, "published_page_spec_invalid", diags[0].Code)

	// Published rows in another category are skipped.
	require.NoError(t, db.Model(&model.PublishedPageSpec{}).Where("page_key = ?", "badpub.page").Update("active", false).Error)
	require.NoError(t, db.Create(&model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: "diffcat.page",
		Version: 1,
		SpecJSON: string(v9MarshalSpec(t, spec.PageSpec{
			PageKey:  "diffcat.page",
			Category: spec.PageCategorySpec{Key: "other", Labels: spec.LocalizedText{"zh-CN": "其他"}},
		})),
		RendererSchemaVersion: rendererSchemaVersion,
		Active:                true,
	}).Error)
	assert.Nil(t, env.service.validatePublishedCategoryLabels(env.ctx, spec.PageSpec{
		PageKey:  "cat.page",
		Category: spec.PageCategorySpec{Key: "player", Labels: labels},
	}))
}

func TestValidatePageShapeCompositeV9(t *testing.T) {
	diags := validatePageShape(spec.PageSpec{Type: spec.PageTypeComposite})
	assertDiagnostic(t, diags, "page_shape_missing", "composite")

	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeComposite, Composite: &spec.CompositePageSpec{}})
	assertDiagnostic(t, diags, "page_shape_missing", "composite")
}

func TestValidatePublishBindingSelectorsOutputBranchesV9(t *testing.T) {
	page := spec.PageSpec{Type: spec.PageTypeResource}
	fn := spec.FunctionSpec{Enabled: true, InputSchema: spec.JSONSchema(`{}`)}

	// Non-nil selectors without output on a resource query binding.
	binding := spec.PageFunctionBinding{
		ID:         "b1",
		FunctionID: "f1",
		Usage:      spec.BindingUsageQuery,
		Selectors:  &spec.BindingSelectors{},
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}
	diags := validatePublishBindingSelectors("b", binding, fn, page)
	assertDiagnostic(t, diags, "binding_output_selector_missing", "b.selectors.output")

	// Invalid output assignments surface selector errors.
	binding.Selectors = &spec.BindingSelectors{
		Output: []spec.OutputAssignment{{
			StateKey: "wrong",
			Source:   "not-a-pointer",
			Shape:    spec.OutputShapeScalar,
		}},
	}
	diags = validatePublishBindingSelectors("b", binding, fn, page)
	found := false
	for _, d := range diags {
		if len(d.Code) > len("binding_selector_") && d.Field == "b.selectors.output.not-a-pointer" {
			found = true
		}
	}
	assert.True(t, found, "expected selector error diagnostics, got %#v", diags)
	// Required-output mapping diagnostics reuse the selectors.output field.
	for _, d := range diags {
		if d.Code == "binding_output_selector_invalid" {
			assert.Equal(t, "b.selectors.output", d.Field)
		}
	}
}

// ---------------------------------------------------------------------------
// Infrastructure helper branches
// ---------------------------------------------------------------------------

func TestWithPageTransactionNilDBV9(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	err := service.withPageTransaction(context.Background(), func(context.Context, *model.PageSpecModel, *model.PublishedPageSpecModel, *model.PageVersionModel) error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database is not initialized")
}

func TestAuditPageEventActorFallbackV9(t *testing.T) {
	service, _, store := newPageTestService(t, "admin:all")

	// Background context: actor falls back to "unknown"; nil details map is
	// initialized before logging.
	service.auditPageEvent(context.Background(), audit.EventPageDraftSave, "demo-game", "development", "audit.page", nil)

	records, total, err := store.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageDraftSave},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "unknown", records[0].Actor.ID)
}

// v9FailingAuditStore embeds the in-memory store but fails every write.
type v9FailingAuditStore struct {
	audit.InMemoryAuditStore
}

func (s *v9FailingAuditStore) Create(record *audit.AuditRecord) error {
	return errors.New("audit store unavailable")
}

func TestAuditPageEventLogErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	service.svcCtx.AuditService = audit.NewAuditService(&v9FailingAuditStore{}, nil)

	// The audit write fails; auditPageEvent must not panic and only logs.
	service.auditPageEvent(ctx, audit.EventPagePublish, "demo-game", "development", "audit.page", map[string]interface{}{"k": "v"})
}

func TestPagePublishSourceWithProposalDigestsV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	specJSON := v9MarshalSpec(t, spec.PageSpec{
		PageKey:  "dig.page",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	p := &model.PageProposal{
		GameID:           "demo-game",
		Env:              "development",
		ProposalKey:      "prop:dig",
		PageKey:          "dig.page",
		PageType:         "operation",
		Quality:          "basic",
		GeneratorVersion: "gen-v9",
		FunctionDigest:   "fn-digest",
		SemanticsDigest:  "sem-digest",
		PageSpec:         model.JSON(specJSON),
		Status:           dbenum.ProposalStatusPending,
		UpdatedBy:        "v9",
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, p))

	src := service.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{BaseProposalKey: "prop:dig"})
	assert.Equal(t, "fn-digest", src.FunctionDigest)
	assert.Equal(t, "sem-digest", src.SemanticsDigest)
	assert.Equal(t, "gen-v9", src.GeneratorVersion)
}

func TestRebuildAllProposalsGuardsV9(t *testing.T) {
	// Missing edit permission.
	noPerm, ctx1, _ := newPageTestService(t)
	_, err := noPerm.RebuildAllProposals(ctx1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权")

	// Missing scope.
	withPerm, _, _ := newPageTestService(t, "pages:edit")
	_, err = withPerm.RebuildAllProposals(v9NoScopeCtx())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID")

	// Contract service failure (capabilities table gone).
	broken, ctx3, _ := newPageTestService(t, "pages:edit")
	require.NoError(t, broken.svcCtx.DB.Migrator().DropTable("resource_capabilities"))
	_, err = broken.RebuildAllProposals(ctx3)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Broken data-source error paths
// ---------------------------------------------------------------------------

func TestServiceBrokenModelErrorPathsV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")

	broken, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	service.svcCtx.PageSpecModel = model.NewPageSpecModel(broken)
	service.svcCtx.PageVersionModel = model.NewPageVersionModel(broken)
	service.svcCtx.PublishedPageSpecModel = model.NewPublishedPageSpecModel(broken)

	_, err = service.ListDrafts(ctx, &PageDraftListRequest{})
	require.Error(t, err)

	_, err = service.Versions(ctx, &PageVersionsRequest{PageKey: "p"})
	require.Error(t, err)

	_, err = service.VersionDetail(ctx, &PageVersionDetailRequest{PageKey: "p", VersionID: "1"})
	require.Error(t, err)

	_, err = service.Rollback(ctx, &PageRollbackRequest{PageKey: "p", VersionID: "1", ExpectedDraftRevision: intPtr(1)})
	require.Error(t, err)

	diags := service.validatePublishedCategoryLabels(ctx, spec.PageSpec{
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	require.Len(t, diags, 1)
	assert.Equal(t, "category_label_check_failed", diags[0].Code)

	assert.Nil(t, service.bindingFreshnessForPublishedDraft(ctx, &model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "p", PublishedVersion: 2,
	}))
}

func TestNormalizedFunctionsContractErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("function_contracts"))

	assert.Empty(t, service.normalizedFunctions(ctx))
}

// ---------------------------------------------------------------------------
// Transaction error paths (triggered via dropped tables / unique conflicts)
// ---------------------------------------------------------------------------

func TestServiceSaveDraftTxFindErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_specs"))

	rev := 0
	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "tx.find",
		DraftRevision: &rev,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "页面"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.Error(t, err)
}

func TestServiceRegenerateDraftTxVersionErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	rev := v9SaveDraftDirect(t, service, ctx, "regen.tx", 0)
	upsertProposalForRegenerate(t, db, ctx, "prop:regen-tx", "regen.tx", spec.PageSpec{
		PageKey:  "regen.tx",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "页面"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	require.NoError(t, db.Migrator().DropTable("page_versions"))

	_, err := service.RegenerateDraft(ctx, &PageRegenerateRequest{PageKey: "regen.tx", DraftRevision: &rev})
	require.Error(t, err)
}

func TestServicePublishDuplicateVersionErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")

	rev := v9SaveDraftDirect(t, service, ctx, "pub.dup", 0)
	_, err := service.Publish(ctx, &PagePublishRequest{PageKey: "pub.dup", DraftRevision: &rev})
	require.NoError(t, err)

	_, err = service.Unpublish(ctx, &PageUnpublishRequest{PageKey: "pub.dup"})
	require.NoError(t, err)

	// Re-publishing the same version collides with the inactive row on the
	// scope+version unique index.
	_, err = service.Publish(ctx, &PagePublishRequest{PageKey: "pub.dup", DraftRevision: &rev})
	require.Error(t, err)
}

func TestServiceUnpublishDeactivateErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")

	v9SaveDraftDirect(t, service, ctx, "unpub.err", 0)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("published_page_specs"))

	_, err := service.Unpublish(ctx, &PageUnpublishRequest{PageKey: "unpub.err"})
	require.Error(t, err)
}

func TestServiceRollbackTxFindErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "rb2.page",
		Type: "operation", Status: "draft", DraftRevision: 1,
		SpecJSON: `{"pageKey":"rb2.page","category":{"key":"c"}}`,
	}).Error)
	require.NoError(t, db.Create(&model.PageVersion{
		GameID: "demo-game", Env: "development", PageKey: "rb2.page",
		Version: 1, SpecJSON: `{"pageKey":"rb2.page","category":{"key":"c"}}`,
		Status: "draft", CreatedBy: "v9",
	}).Error)

	require.NoError(t, db.Migrator().DropTable("page_specs"))
	_, err := service.Rollback(ctx, &PageRollbackRequest{PageKey: "rb2.page", VersionID: "1", ExpectedDraftRevision: intPtr(1)})
	require.Error(t, err)
}

func TestServiceRollbackTxApplyErrorV9(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "admin:all")
	db := service.svcCtx.DB

	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "rb3.page",
		Type: "operation", Status: "draft", DraftRevision: 1,
		SpecJSON: `{"pageKey":"rb3.page","category":{"key":"c"}}`,
	}).Error)
	// Version spec without category.key parses but fails applyPageSpecToModel.
	require.NoError(t, db.Create(&model.PageVersion{
		GameID: "demo-game", Env: "development", PageKey: "rb3.page",
		Version: 1, SpecJSON: `{"pageKey":"rb3.page"}`,
		Status: "draft", CreatedBy: "v9",
	}).Error)

	_, err := service.Rollback(ctx, &PageRollbackRequest{PageKey: "rb3.page", VersionID: "1", ExpectedDraftRevision: intPtr(1)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "category.key")
}

// ---------------------------------------------------------------------------
// Handler-level uncovered branches
// ---------------------------------------------------------------------------

func TestHandlerCoverageFlowsV9(t *testing.T) {
	env := setupPageFlowEnv(t)
	db := env.service.svcCtx.DB

	// GetDraft success.
	env.saveDraft(t, "http.page", 0)
	rec := env.do(t, http.MethodGet, "/api/v1/pages/http.page", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	// GetDraft generic error (corrupt canonical spec).
	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "corrupt.http",
		Type: "operation", Status: "draft", DraftRevision: 1, SpecJSON: "{bad",
	}).Error)
	rec = env.do(t, http.MethodGet, "/api/v1/pages/corrupt.http", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// SaveDraft success over HTTP.
	rev := 0
	saveReq := PageSaveRequest{
		PageKey:       "http.put",
		DraftRevision: &rev,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "页面"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	}
	body, err := json.Marshal(saveReq)
	require.NoError(t, err)
	rec = env.do(t, http.MethodPut, "/api/v1/pages/http.put", string(body))
	assert.Equal(t, http.StatusOK, rec.Code)

	// SaveDraft URI binding failure (no pageKey param).
	ginCtx, ginRec := newTestContext(http.MethodPut, "/api/v1/pages/", "{}")
	env.handler.SaveDraft(ginCtx)
	assert.Equal(t, http.StatusBadRequest, ginRec.Code)

	// RegenerateDraft JSON binding failure.
	rec = env.do(t, http.MethodPost, "/api/v1/pages/http.page/regenerate", "{bad json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// RegenerateDraft success with a seeded proposal.
	regenRev := env.saveDraft(t, "regen.http", 0)
	upsertProposalForRegenerate(t, db, env.ctx, "prop:regen-http", "regen.http", spec.PageSpec{
		PageKey:  "regen.http",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "页面"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	})
	rec = env.do(t, http.MethodPost, "/api/v1/pages/regen.http/regenerate", `{"draftRevision":`+itoa(regenRev)+`}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Validate success and generic error.
	rec = env.do(t, http.MethodPost, "/api/v1/pages/http.page/validate", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodPost, "/api/v1/pages/corrupt.http/validate", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Preview success.
	rec = env.do(t, http.MethodPost, "/api/v1/pages/http.page/preview", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	// Publish JSON binding failure.
	rec = env.do(t, http.MethodPost, "/api/v1/pages/http.page/publish", "{bad json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Rollback success over HTTP.
	rbRev1 := env.saveDraft(t, "rb.http", 0)
	rbRev2 := env.saveDraft(t, "rb.http", rbRev1)
	require.Equal(t, rbRev1+1, rbRev2)
	rec = env.do(t, http.MethodPost, "/api/v1/pages/rb.http/rollback", `{"versionId":"1","expectedDraftRevision":`+itoa(rbRev2)+`}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// RebuildProposals error (request context carries a username but no game
	// scope, so the permission guard passes and requireScope fails).
	rbCtx, rbRec := newTestContext(http.MethodPost, "/api/v1/pages/proposals/rebuild", "")
	rbCtx.Request = rbCtx.Request.WithContext(v9NoScopeCtx())
	env.handler.RebuildProposals(rbCtx)
	assert.Equal(t, http.StatusBadRequest, rbRec.Code)
}
