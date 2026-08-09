package page

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	consoleapi "github.com/cuihairu/croupier/internal/api/console"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestServiceSaveDraftRequiresPageEditPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t)
	revision := 0

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权编辑页面")
}

func TestServiceSaveDraftUsesContextActorAndWritesAudit(t *testing.T) {
	service, ctx, auditStore := newPageTestService(t, "pages:edit")
	revision := 0

	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, resp.DraftRevision)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, "page_tester", draft.UpdatedBy)

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageDraftSave},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "page_tester", records[0].Actor.ID)
	assert.Equal(t, "page", records[0].Resource.Type)
	assert.Equal(t, "player.manage", records[0].Resource.ID)
	assert.Equal(t, "demo-game", records[0].Resource.GameID)
	assert.Equal(t, "development", records[0].Resource.Environment)
	assert.Equal(t, "player.manage", records[0].Details["page_key"])
	assert.Equal(t, "demo-game", records[0].Details["game_id"])
	assert.Equal(t, "development", records[0].Details["env"])
}

func TestServiceSaveDraftRejectsMissingCategoryKey(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "category.key is required")
}

func TestServiceGetDraftRejectsMissingCanonicalSpecJSON(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	require.NoError(t, service.svcCtx.PageSpecModel.Upsert(ctx, &model.PageSpec{
		GameID:             "demo-game",
		Env:                "development",
		PageKey:            "player.legacy",
		Type:               "operation",
		ResourceKey:        "player",
		CategoryKey:        "player",
		CategoryLabelsJSON: `{"zh-CN":"玩家"}`,
		TitleJSON:          `{"zh-CN":"旧页面"}`,
		Status:             "draft",
		DraftRevision:      1,
		UpdatedBy:          "legacy",
	}))

	_, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.legacy"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical PageSpec")
}

func TestServicePublishRequiresPagePublishPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := saveTestPageDraft(t, service, ctx)

	_, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权发布页面")
}

func TestServicePublishWritesActorAndAudit(t *testing.T) {
	service, ctx, auditStore := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	resp, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})

	require.NoError(t, err)
	assert.True(t, resp.Published)
	assert.Equal(t, revision, resp.PublishedVersion)

	published, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, "page_tester", published.PublishedBy)
	var contracts []spec.BindingContractSnapshot
	require.NoError(t, json.Unmarshal([]byte(published.BindingContractsJSON), &contracts))
	require.Len(t, contracts, 2)
	assert.Equal(t, spec.RiskSafe, contracts[0].Risk)
	assert.Equal(t, "player:action", contracts[0].Permission)
	assert.Equal(t, spec.RiskSafe, contracts[1].Risk)
	assert.Equal(t, "player:query", contracts[1].Permission)

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPagePublish},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "page_tester", records[0].Actor.ID)
	assert.Equal(t, "player.manage", records[0].Resource.ID)
	assert.Equal(t, revision, records[0].Details["published_version"])
}

func TestServicePublishUpdatesDraftVersionRecord(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	_, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.NoError(t, err)

	versions, err := service.Versions(ctx, &PageVersionsRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	require.Len(t, versions.Items, 1)
	assert.Equal(t, revision, versions.Items[0].Version)
	assert.Equal(t, "published", versions.Items[0].Status)
	assert.True(t, versions.Items[0].IsCurrentDraft)
	assert.True(t, versions.Items[0].IsCurrentPublished)
}

func TestServicePublishRejectsStaleDraftRevision(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)
	staleRevision := revision

	updated, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理（已更新）"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &staleRevision,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft revision conflict")

	_, err = service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "player.manage")
	assert.Error(t, err)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, updated.DraftRevision, draft.DraftRevision)
	assert.Equal(t, "玩家管理（已更新）", draft.Title["zh-CN"])
}

func TestServiceRollbackRejectsStaleDraftRevision(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:rollback", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)
	staleRevision := revision

	updated, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理（已更新）"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)

	_, err = service.Rollback(ctx, &PageRollbackRequest{
		PageKey:               "player.manage",
		VersionID:             "1",
		ExpectedDraftRevision: &staleRevision,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft revision conflict")

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, updated.DraftRevision, draft.DraftRevision)
	assert.Equal(t, "玩家管理（已更新）", draft.Title["zh-CN"])
}

func TestServicePublishRejectsMissingBindingSelector(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	revision := 0
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindingsWithoutSelector(),
	})
	require.NoError(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &resp.DraftRevision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding.selectors.input is required before publish")
}

func TestServicePublishRejectsIncompleteBindingSelector(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	rebuildFunctionContract(t, service.svcCtx.DB, ctx, "player.query", reg.FunctionMeta{
		Enabled:     true,
		Version:     "1.0.0",
		Resource:    "player",
		Operation:   "query",
		InputSchema: `{"type":"object","properties":{"keyword":{"type":"string"}},"required":["keyword"]}`,
	})
	revision := 0
	bindings := testPageBindings()
	bindings[0].Selectors = &spec.BindingSelectors{Input: spec.SelectorAST{}}
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  bindings,
	})
	require.NoError(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &resp.DraftRevision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "required field not assigned")
}

func TestServicePublishRejectsInvalidOutputSelector(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	revision := 0
	bindings := testPageBindings()
	bindings[0].Selectors.Output = []spec.OutputAssignment{{
		StateKey: "players",
		Source:   "items",
		Shape:    spec.OutputShapeCollection,
	}}
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  bindings,
	})
	require.NoError(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &resp.DraftRevision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "output source must be a JSON Pointer")
}

func TestServicePublishRejectsMissingBindings(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	revision := 0
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  nil,
	})
	require.NoError(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &resp.DraftRevision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation page requires an action binding before publish")
}

func TestServicePublishDrivesConsoleMenuAndUnpublishRemovesIt(t *testing.T) {
	pageService, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	consoleService := consoleapi.NewService(pageService.svcCtx)
	revision := saveTestPageDraft(t, pageService, ctx)

	publishResp, err := pageService.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.NoError(t, err)
	assert.True(t, publishResp.Published)

	menu, err := consoleService.Menu(ctx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	assert.Equal(t, "player", menu.Items[0].Key)
	assert.Equal(t, "玩家", menu.Items[0].Title["zh-CN"])
	require.Len(t, menu.Items[0].Children, 1)
	assert.Equal(t, "player.manage", menu.Items[0].Children[0].Key)
	assert.Equal(t, "玩家管理", menu.Items[0].Children[0].Title["zh-CN"])
	assert.Equal(t, "/console/player/player.manage", menu.Items[0].Children[0].Path)
	assert.False(t, menu.Items[0].Locale)
	assert.False(t, menu.Items[0].Children[0].Locale)

	publishedPage, err := consoleService.Page(ctx, &consoleapi.ConsolePageRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, "demo-game", publishedPage.Page.GameID)
	assert.Equal(t, "development", publishedPage.Page.Env)
	assert.Equal(t, revision, publishedPage.Page.Version)

	prodCtx := svc.WithGameScope(ctx, svc.GameScope{GameID: "demo-game", Env: "production"})
	prodMenu, err := consoleService.Menu(prodCtx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	assert.Empty(t, prodMenu.Items)

	unpublishResp, err := pageService.Unpublish(ctx, &PageUnpublishRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.False(t, unpublishResp.Published)

	emptyMenu, err := consoleService.Menu(ctx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	assert.Empty(t, emptyMenu.Items)
	_, err = consoleService.Page(ctx, &consoleapi.ConsolePageRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found")
}

func TestServicePublishRejectsCategoryLabelConflict(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	firstRevision := saveTestPageDraft(t, service, ctx)
	_, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &firstRevision,
	})
	require.NoError(t, err)

	secondRevision := 0
	secondResp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.audit",
		DraftRevision: &secondRevision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家审计"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家管理"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)

	_, err = service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.audit",
		DraftRevision: &secondResp.DraftRevision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "category.labels must match existing published pages")
}

func TestServiceGetDraftReturnsPublishedBindingFreshness(t *testing.T) {
	pageService, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := saveTestPageDraft(t, pageService, ctx)
	_, err := pageService.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.NoError(t, err)

	rebuildFunctionContract(t, pageService.svcCtx.DB, ctx, "player.query", reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "danger",
		Permission:   "player:admin",
		Resource:     "player",
		Operation:    "query",
		InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
	})
	rebuildFunctionContract(t, pageService.svcCtx.DB, ctx, "player.action", reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "safe",
		Permission:   "player:action",
		Resource:     "player",
		Operation:    "action",
		InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	})

	draft, err := pageService.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	require.Len(t, draft.BindingFreshness, 1)
	assert.Equal(t, spec.BindingFreshnessGovernanceStale, draft.BindingFreshness[0].Status)
	assert.Equal(t, "binding_governance_stale", draft.BindingFreshness[0].Diagnostic.Code)
	assert.Equal(t, "player.query", draft.BindingFreshness[0].BindingID)
}

func TestServiceRegenerateDraftUsesLatestFunctionContractWithoutPublishing(t *testing.T) {
	pageService, ctx, auditStore := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := 0
	saveResp, err := pageService.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "operation--player.query",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "旧查询页"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)

	publishResp, err := pageService.Publish(ctx, &PagePublishRequest{
		PageKey:       "operation--player.query",
		DraftRevision: &saveResp.DraftRevision,
	})
	require.NoError(t, err)
	require.Equal(t, 1, publishResp.PublishedVersion)

	rebuildFunctionContract(t, pageService.svcCtx.DB, ctx, "player.query", reg.FunctionMeta{
		Enabled:      true,
		Version:      "2.0.0",
		Risk:         "safe",
		Permission:   "player:query",
		Resource:     "player",
		Operation:    "query",
		InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"},"server_id":{"type":"string","title":"区服"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
	})
	upsertProposalForRegenerate(t, pageService.svcCtx.DB, ctx, "operation:player.query", "operation--player.query", spec.PageSpec{
		PageKey:     "operation--player.query",
		Type:        spec.PageTypeOperation,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "Query"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: &spec.OperationPageSpec{
			Form: spec.DefaultFormPresentation(spec.JSONSchema(`{"type":"object","properties":{"keyword":{"type":"string"},"server_id":{"type":"string","title":"区服"}}}`)),
			ResultView: &spec.ResultViewSpec{
				SuccessMessage: spec.LocalizedText{"zh-CN": "操作成功"},
				ErrorMessage:   spec.LocalizedText{"zh-CN": "操作失败"},
			},
		},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "player.query",
			FunctionID: "player.query",
			Usage:      spec.BindingUsageAction,
			Selectors: &spec.BindingSelectors{
				Input: spec.SelectorAST{Assignments: []spec.InputAssignment{
					{Target: "/keyword", Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/keyword"}},
					{Target: "/server_id", Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/server_id"}},
				}},
			},
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	})

	regenerateResp, err := pageService.RegenerateDraft(ctx, &PageRegenerateRequest{
		PageKey:       "operation--player.query",
		DraftRevision: &saveResp.DraftRevision,
	})
	require.NoError(t, err)
	assert.Equal(t, "operation--player.query", regenerateResp.PageKey)
	assert.Equal(t, saveResp.DraftRevision+1, regenerateResp.DraftRevision)
	assert.Equal(t, spec.GeneratedPageQualityBasic, regenerateResp.Quality)
	require.NotNil(t, regenerateResp.Page.Operation)
	require.NotNil(t, regenerateResp.Page.Operation.Form)
	assert.Contains(t, string(regenerateResp.Page.Operation.Form.JSONSchema), `"server_id"`)
	assert.Contains(t, string(regenerateResp.Page.Operation.Form.JSONSchema), `"keyword"`)
	assert.Equal(t, spec.PageTypeOperation, regenerateResp.Page.Type)

	published, err := pageService.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "operation--player.query")
	require.NoError(t, err)
	assert.Equal(t, 1, published.Version)
	assert.True(t, published.Active)

	draft, err := pageService.GetDraft(ctx, &PageDraftRequest{PageKey: "operation--player.query"})
	require.NoError(t, err)
	assert.Equal(t, regenerateResp.DraftRevision, draft.DraftRevision)
	assert.Equal(t, "Query", draft.Title["zh-CN"])
	assertBindingFreshnessStatus(t, draft.BindingFreshness, spec.BindingFreshnessFunctionVersionStale)
	assertBindingFreshnessStatus(t, draft.BindingFreshness, spec.BindingFreshnessInputSchemaStale)

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageDraftSave},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	regenerateAudit := findAuditRecordByDetail(records, "action", "regenerate_default")
	require.NotNil(t, regenerateAudit)
	assert.Equal(t, "page_proposal", regenerateAudit.Details["source"])
}

func TestServiceRegenerateDraftRejectsRevisionConflict(t *testing.T) {
	pageService, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, pageService, ctx)
	staleRevision := revision - 1

	_, err := pageService.RegenerateDraft(ctx, &PageRegenerateRequest{
		PageKey:       "player.manage",
		DraftRevision: &staleRevision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft revision conflict")
}

func TestServiceRegenerateDraftRejectsMissingProposal(t *testing.T) {
	pageService, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, pageService, ctx)

	_, err := pageService.RegenerateDraft(ctx, &PageRegenerateRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "latest PageProposal is required for default regeneration")
}

func TestServicePublishesBasicGeneratedOperationPage(t *testing.T) {
	pageService, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	consoleService := consoleapi.NewService(pageService.svcCtx)
	generated := generator.GenerateForOperation(spec.OperationSpec{
		FunctionID:  "player.query",
		ResourceKey: "player",
		Operation:   "query",
		Enabled:     true,
	}, generator.GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"player.query": {
				ID: "player.query",
				InputSchema: spec.JSONSchema(`{
					"type":"object",
					"properties":{"keyword":{"type":"string"}}
				}`),
			},
		},
	})
	require.Equal(t, spec.GeneratedPageQualityBasic, generated.Quality)
	require.Len(t, generated.Bindings, 1)
	require.NotNil(t, generated.Bindings[0].Selectors)
	assertSelectorAssignment(t, generated.Bindings[0].Selectors.Input, "/keyword", spec.SourceForm, "/keyword")

	revision := 0
	saveResp, err := pageService.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       generated.PageKey,
		DraftRevision: &revision,
		Type:          generated.Type,
		ResourceKey:   generated.ResourceKey,
		Title:         map[string]string(generated.Title),
		Category:      generated.Category,
		Operation:     generated.Operation,
		Bindings:      generated.Bindings,
	})
	require.NoError(t, err)

	publishResp, err := pageService.Publish(ctx, &PagePublishRequest{
		PageKey:       generated.PageKey,
		DraftRevision: &saveResp.DraftRevision,
	})
	require.NoError(t, err)
	assert.True(t, publishResp.Published)

	menu, err := consoleService.Menu(ctx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	require.Len(t, menu.Items[0].Children, 1)
	assert.Equal(t, generated.PageKey, menu.Items[0].Children[0].Key)
}

func TestServiceKeepsSamePageKeyIsolatedByScope(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	devRevision := saveTestPageDraft(t, service, ctx)
	prodCtx := svc.WithGameScope(ctx, svc.GameScope{GameID: "demo-game", Env: "production"})
	prodRevision := saveTestPageDraft(t, service, prodCtx)

	devDraft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	prodDraft, err := service.GetDraft(prodCtx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)

	assert.Equal(t, devRevision, devDraft.DraftRevision)
	assert.Equal(t, prodRevision, prodDraft.DraftRevision)
	assert.Equal(t, "development", devDraft.Env)
	assert.Equal(t, "production", prodDraft.Env)

	nextProdRevision := prodRevision
	resp, err := service.SaveDraft(prodCtx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &nextProdRevision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "生产玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)
	assert.Equal(t, prodRevision+1, resp.DraftRevision)

	devDraft, err = service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, devRevision, devDraft.DraftRevision)
	assert.Equal(t, "玩家管理", devDraft.Title["zh-CN"])
}

func newPageTestService(t *testing.T, permissions ...string) (*Service, context.Context, *audit.InMemoryAuditStore) {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	store := reg.NewStore()

	admin := model.Admin{Username: "page_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)

	role := model.Role{Name: "page_tester_role", Description: "page tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		grantPermission(t, db, role.ID, permissionID)
	}

	auditStore := audit.NewInMemoryAuditStore()
	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:                     db,
		AdminModel:             model.NewAdminModel(db),
		RoleModel:              model.NewRoleModel(db),
		PermissionModel:        model.NewPermissionModel(db),
		PageSpecModel:          model.NewPageSpecModel(db),
		PublishedPageSpecModel: model.NewPublishedPageSpecModel(db),
		PageVersionModel:       model.NewPageVersionModel(db),
		RegistryStore:          store,
		AuditService:           audit.NewAuditService(auditStore, nil),
		Cache:                  nullCache,
		CacheHelper:            cache.NewCacheHelper(nullCache),
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", admin.Username)
	rebuildFunctionContract(t, db, ctx, "player.query", reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "safe",
		Permission:   "player:query",
		Resource:     "player",
		Operation:    "query",
		InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
	})
	rebuildFunctionContract(t, db, ctx, "player.action", reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "safe",
		Permission:   "player:action",
		Resource:     "player",
		Operation:    "action",
		InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	})
	return NewService(svcCtx), ctx, auditStore
}

func saveTestPageDraft(t *testing.T, service *Service, ctx context.Context) int {
	t.Helper()

	revision := 0
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
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

func testOperationPageSpec() *spec.OperationPageSpec {
	return &spec.OperationPageSpec{
		Form: spec.DefaultFormPresentation(spec.JSONSchema(`{
			"type":"object",
			"properties":{"keyword":{"type":"string"}},
			"required":["keyword"]
		}`)),
		ResultView: &spec.ResultViewSpec{
			SuccessMessage: spec.LocalizedText{"zh-CN": "操作成功"},
			ErrorMessage:   spec.LocalizedText{"zh-CN": "操作失败"},
		},
	}
}

func testPageBindings() []spec.PageFunctionBinding {
	return []spec.PageFunctionBinding{
		{
			ID:         "player.query",
			FunctionID: "player.query",
			Usage:      spec.BindingUsageQuery,
			Selectors: &spec.BindingSelectors{
				Input: spec.SelectorAST{Assignments: []spec.InputAssignment{{
					Target: "/keyword",
					Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/keyword"},
				}}},
			},
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		{
			ID:         "player.action",
			FunctionID: "player.action",
			Usage:      spec.BindingUsageAction,
			Selectors: &spec.BindingSelectors{
				Input: spec.SelectorAST{Assignments: []spec.InputAssignment{{
					Target: "/keyword",
					Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/keyword"},
				}}},
			},
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
	}
}

func testPageBindingsWithoutSelector() []spec.PageFunctionBinding {
	return []spec.PageFunctionBinding{
		{
			ID:         "player.query",
			FunctionID: "player.query",
			Usage:      spec.BindingUsageQuery,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
	}
}

func assertDiagnostic(t *testing.T, diags []spec.Diagnostic, code string, field string) {
	t.Helper()
	for _, diag := range diags {
		if diag.Code == code && diag.Field == field {
			return
		}
	}
	t.Fatalf("expected diagnostic %s at %s, got %#v", code, field, diags)
}

func assertBindingFreshnessStatus(t *testing.T, diagnostics []spec.BindingFreshnessDiagnostic, status spec.BindingFreshnessStatus) {
	t.Helper()
	for _, diag := range diagnostics {
		if diag.Status == status {
			return
		}
	}
	t.Fatalf("expected binding freshness status %s, got %#v", status, diagnostics)
}

func assertSelectorAssignment(t *testing.T, selector spec.SelectorAST, target string, sourceKind spec.ValueSourceKind, sourcePath string) {
	t.Helper()
	for _, assignment := range selector.Assignments {
		if assignment.Target == target && assignment.Source.Kind == sourceKind && assignment.Source.Path == sourcePath {
			return
		}
	}
	t.Fatalf("expected selector assignment %s <- %s:%s, got %#v", target, sourceKind, sourcePath, selector.Assignments)
}

func findAuditRecordByDetail(records []*audit.AuditRecord, key string, value interface{}) *audit.AuditRecord {
	for _, record := range records {
		if record != nil && record.Details[key] == value {
			return record
		}
	}
	return nil
}

func rebuildFunctionContract(t *testing.T, db *gorm.DB, ctx context.Context, functionID string, meta reg.FunctionMeta) {
	t.Helper()
	input := struct {
		ID           string
		Version      string
		Enabled      bool
		Summary      string
		Description  string
		InputSchema  string
		OutputSchema string
		Resource     string
		Operation    string
		Capability   string
		Execution    string
		Risk         string
		Permission   string
		Tags         []string
	}{
		ID:           functionID,
		Version:      meta.Version,
		Enabled:      meta.Enabled,
		Summary:      meta.Summary,
		Description:  meta.Description,
		InputSchema:  meta.InputSchema,
		OutputSchema: meta.OutputSchema,
		Resource:     meta.Resource,
		Operation:    meta.Operation,
		Capability:   meta.Capability,
		Execution:    meta.Execution,
		Risk:         meta.Risk,
		Permission:   meta.Permission,
		Tags:         meta.Tags,
	}
	require.NoError(t, dashboardservice.NewContractService(db).RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", input))
}

func upsertProposalForRegenerate(t *testing.T, db *gorm.DB, ctx context.Context, proposalKey string, pageKey string, pageSpec spec.PageSpec) {
	t.Helper()
	raw, err := json.Marshal(pageSpec)
	require.NoError(t, err)
	proposal := &model.PageProposal{
		GameID:           "demo-game",
		Env:              "development",
		ProposalKey:      proposalKey,
		PageKey:          pageKey,
		PageType:         string(pageSpec.Type),
		ResourceKey:      pageSpec.ResourceKey,
		Quality:          string(spec.GeneratedPageQualityBasic),
		GeneratorVersion: "test-generator",
		Title:            map[string]interface{}{"zh-CN": pageSpec.Title["zh-CN"]},
		CategoryKey:      pageSpec.Category.Key,
		PageSpec:         raw,
		Status:           "pending",
		UpdatedBy:        "test",
	}
	proposalModel := model.NewPageProposalModel(db)
	require.NoError(t, proposalModel.UpsertProposal(ctx, proposal))
	proposalRaw, err := json.Marshal(proposal)
	require.NoError(t, err)
	version := &model.PageProposalVersion{
		ProposalID:   proposal.ID,
		Version:      1,
		Proposal:     proposalRaw,
		ChangeReason: "test proposal regenerate",
		CreatedBy:    "test",
	}
	require.NoError(t, model.NewPageProposalVersionModel(db).CreateVersion(ctx, version))
}

func errorDiagnostics(diags []spec.Diagnostic) []spec.Diagnostic {
	var errors []spec.Diagnostic
	for _, diag := range diags {
		if diag.Severity == spec.SeverityError {
			errors = append(errors, diag)
		}
	}
	return errors
}

func grantPermission(t *testing.T, db *gorm.DB, roleID uint, permissionID string) {
	t.Helper()

	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return
	}
	parts := strings.SplitN(permissionID, ":", 2)
	action := "*"
	if len(parts) == 2 {
		action = parts[1]
	}
	permission := model.Permission{
		ID:       permissionID,
		Name:     permissionID,
		Resource: parts[0],
		Action:   action,
		Category: "dashboard",
	}
	require.NoError(t, db.Where("id = ?", permission.ID).FirstOrCreate(&permission).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: roleID, PermissionID: permissionID}).Error)
}
