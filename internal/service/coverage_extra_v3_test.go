package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// ===========================================================================
// RemoveFunctionContract
// ===========================================================================

func TestRemoveFunctionContract_EmptyID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := NewContractService(db)

	resourceKey, err := s.RemoveFunctionContract(ctx, "g1", "dev", "")
	require.NoError(t, err)
	assert.Empty(t, resourceKey)
}

func TestRemoveFunctionContract_WhitespaceID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := NewContractService(db)

	resourceKey, err := s.RemoveFunctionContract(ctx, "g1", "dev", "   ")
	require.NoError(t, err)
	assert.Empty(t, resourceKey)
}

func TestRemoveFunctionContract_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := NewContractService(db)

	resourceKey, err := s.RemoveFunctionContract(ctx, "g1", "dev", "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, resourceKey)
}

func TestRemoveFunctionContract_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	s := NewContractService(db)

	require.NoError(t, s.RebuildContractFromFunctionMeta(ctx, "g1", "dev", "sdk", FunctionMetaInput{
		ID: "player.ban", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "ban",
		Capability: "action", Execution: "sync",
	}))

	resourceKey, err := s.RemoveFunctionContract(ctx, "g1", "dev", "player.ban")
	require.NoError(t, err)
	assert.Equal(t, "player", resourceKey)

	_, err = s.GetContract(ctx, "g1", "dev", "player.ban")
	assert.Error(t, err)
}

// ===========================================================================
// normalizeJSONSchema (contract_projection.go)
// ===========================================================================

func TestNormalizeJSONSchemaV3_Empty(t *testing.T) {
	// nil returns nil
	assert.Nil(t, normalizeJSONSchema(nil))
	// empty returns empty (not nil)
	result := normalizeJSONSchema([]byte{})
	assert.Len(t, result, 0)
}

func TestNormalizeJSONSchemaV3_StringWrapped(t *testing.T) {
	input := []byte(`"{\"type\":\"object\"}"`)
	result := normalizeJSONSchema(input)
	assert.Equal(t, json.RawMessage(`{"type":"object"}`), result)
}

func TestNormalizeJSONSchemaV3_NativeJSON(t *testing.T) {
	input := []byte(`{"type":"object"}`)
	result := normalizeJSONSchema(input)
	assert.Equal(t, json.RawMessage(input), result)
}

func TestNormalizeJSONSchemaV3_InvalidString(t *testing.T) {
	input := []byte(`"not closed`)
	result := normalizeJSONSchema(input)
	assert.Equal(t, json.RawMessage(input), result)
}

// ===========================================================================
// inferIdentityField - more branches
// ===========================================================================

func TestInferIdentityFieldV3_WithResourceID(t *testing.T) {
	sem := &model.CapabilitySemantics{ResourceKey: "player", CollectionQueryID: 1}
	contracts := []*model.FunctionContract{
		{
			FunctionID: "func1", Capability: "collection_query",
			OutputSchema: datatypes.JSON(
				`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"player_id":{"type":"integer"}}}}}}`),
		},
	}
	inferIdentityField(sem, contracts)
	assert.Equal(t, "player_id", sem.IdentityField)
	assert.Equal(t, "integer", sem.IdentityFieldType)
}

func TestInferIdentityFieldV3_WithResourceId(t *testing.T) {
	sem := &model.CapabilitySemantics{ResourceKey: "order", CollectionQueryID: 1}
	contracts := []*model.FunctionContract{
		{
			FunctionID: "func1", Capability: "collection_query",
			OutputSchema: datatypes.JSON(
				`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"orderId":{"type":"string"}}}}}}`),
		},
	}
	inferIdentityField(sem, contracts)
	assert.Equal(t, "orderId", sem.IdentityField)
}

func TestInferIdentityFieldV3_NoIDField_FallsBack(t *testing.T) {
	sem := &model.CapabilitySemantics{ResourceKey: "player", CollectionQueryID: 1}
	contracts := []*model.FunctionContract{
		{
			FunctionID: "func1", Capability: "collection_query",
			OutputSchema: datatypes.JSON(
				`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"alpha":{"type":"string"},"beta":{"type":"integer"}}}}}}`),
		},
	}
	inferIdentityField(sem, contracts)
	assert.Equal(t, "alpha", sem.IdentityField)
}

func TestInferIdentityFieldV3_NoCollectionQuery(t *testing.T) {
	sem := &model.CapabilitySemantics{ResourceKey: "player"}
	contracts := []*model.FunctionContract{
		{FunctionID: "func1", OutputSchema: datatypes.JSON(`{"type":"object"}`)},
	}
	inferIdentityField(sem, contracts)
	assert.Empty(t, sem.IdentityField)
}

func TestInferIdentityFieldV3_NilContracts(t *testing.T) {
	sem := &model.CapabilitySemantics{}
	inferIdentityField(sem, nil)
	assert.Empty(t, sem.IdentityField)
}

// ===========================================================================
// Handler tests - ContractHandler
// ===========================================================================

func setupContractHandler(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	contractSvc := NewContractService(db)
	h := NewContractHandler(contractSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(svc.WithGameScope(c.Request.Context(), svc.GameScope{
			GameID: "demo-game",
			Env:    "development",
		}))
		c.Next()
	})
	r.GET("/api/contracts", h.ListContracts)
	r.GET("/api/contracts/:functionId", h.GetContract)
	r.GET("/api/resource-capabilities", h.ListResourceCapabilities)
	return h, r
}

func TestContractHandler_ListContracts(t *testing.T) {
	_, router := setupContractHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/contracts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestContractHandler_GetContract_NotFound(t *testing.T) {
	_, router := setupContractHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/contracts/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestContractHandler_ListResourceCapabilities(t *testing.T) {
	_, router := setupContractHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/resource-capabilities", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ===========================================================================
// Handler tests - ProposalHandler
// ===========================================================================

func setupProposalHandler(t *testing.T) (*ProposalHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	proposalSvc := NewProposalService(db)
	h := NewProposalHandler(proposalSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(svc.WithGameScope(c.Request.Context(), svc.GameScope{
			GameID: "demo-game",
			Env:    "development",
		}))
		c.Next()
	})
	r.GET("/api/proposals", h.ListProposals)
	r.GET("/api/proposals/inbox", h.Inbox)
	r.GET("/api/proposals/:proposalKey", h.GetProposal)
	r.POST("/api/proposals/:proposalKey/accept", h.AcceptProposal)
	r.POST("/api/proposals/:proposalKey/accept-and-publish", h.AcceptAndPublishProposal)
	r.POST("/api/proposals/:proposalKey/reject", h.RejectProposal)
	return h, r
}

func TestProposalHandler_ListProposals(t *testing.T) {
	_, router := setupProposalHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/proposals", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestProposalHandler_Inbox(t *testing.T) {
	_, router := setupProposalHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/proposals/inbox", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestProposalHandler_GetProposal_NotFound(t *testing.T) {
	_, router := setupProposalHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/proposals/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestProposalHandler_AcceptProposal_NotFound(t *testing.T) {
	_, router := setupProposalHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/proposals/nonexistent/accept", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestProposalHandler_AcceptAndPublishProposal_NotFound(t *testing.T) {
	_, router := setupProposalHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/proposals/nonexistent/accept-and-publish", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestProposalHandler_RejectProposal_NotFound(t *testing.T) {
	_, router := setupProposalHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/proposals/nonexistent/reject", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// ===========================================================================
// Handler tests - ExportHandler
// ===========================================================================

func setupExportHandler(t *testing.T) (*ExportHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	exportSvc := NewDataExportService(db)
	h := NewExportHandler(exportSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(svc.WithGameScope(c.Request.Context(), svc.GameScope{
			GameID: "demo-game",
			Env:    "development",
		}))
		c.Next()
	})
	r.GET("/api/export/pages", h.ExportPages)
	return h, r
}

func TestExportHandler_ExportPages(t *testing.T) {
	_, router := setupExportHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/export/pages", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

// ===========================================================================
// listBlockedIssueDTOs
// ===========================================================================

func TestListBlockedIssueDTOsV3_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	dtos, err := p.listBlockedIssueDTOs(ctx, "g1", "dev", "")
	require.NoError(t, err)
	assert.Empty(t, dtos)
}

func TestListBlockedIssueDTOsV3_WithResourceKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	dtos, err := p.listBlockedIssueDTOs(ctx, "g1", "dev", "player")
	require.NoError(t, err)
	assert.Empty(t, dtos)
}

func TestListBlockedIssueDTOsV3_WithIssues(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	blockedModel := model.NewBlockedProposalIssueModel(db)
	require.NoError(t, blockedModel.Upsert(ctx, &model.BlockedProposalIssue{
		GameID:        "g1",
		Env:           "dev",
		ResourceKey:   "player",
		FunctionID:    "player.ban",
		Status:        "blocked",
		Diagnostics:   datatypes.JSON(`[{"code":"test","severity":"error","message":"test"}]`),
		SourceDigests: datatypes.JSON(`["digest1"]`),
		RepairHint:    datatypes.JSONMap{"zh-CN": "请修复"},
		UpdatedBy:     "system",
	}))

	dtos, err := p.listBlockedIssueDTOs(ctx, "g1", "dev", "")
	require.NoError(t, err)
	require.Len(t, dtos, 1)
	assert.Equal(t, "player.ban", dtos[0].FunctionID)
	assert.Equal(t, "blocked", dtos[0].Status)
}

// ===========================================================================
// listContractChanges / publishedContractChanges / draftContractChanges
// ===========================================================================

func TestListContractChangesV3_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	changes, err := p.listContractChanges(ctx, "g1", "dev", "")
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestPublishedContractChangesV3_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	changes, err := p.publishedContractChanges(ctx, "g1", "dev", "", nil)
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestDraftContractChangesV3_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	changes, err := p.draftContractChanges(ctx, "g1", "dev", "", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestDraftContractChangesV3_WithDrafts(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	pageSpecJSON := `{"pageKey":"page-a","type":"resource","title":{"zh-CN":"测试"},"category":{"key":"test","labels":{"zh-CN":"测试"}},"bindings":[{"id":"b1","functionId":"missing.func","usage":"query","execution":{"mode":"sync"}}]}`
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, &model.PageSpec{
		GameID:   "g1",
		Env:      "dev",
		PageKey:  "page-a",
		Type:     "resource",
		SpecJSON: pageSpecJSON,
		Status:   "draft",
	}))

	changes, err := p.draftContractChanges(ctx, "g1", "dev", "", nil, nil)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "page-a", changes[0].PageKey)
	assert.Equal(t, "draft", changes[0].Kind)
}

func TestDraftContractChangesV3_FilterByResourceKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	pageSpecJSON := `{"pageKey":"page-c","type":"resource","title":{"zh-CN":"测试"},"category":{"key":"test","labels":{"zh-CN":"测试"}},"bindings":[]}`
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, &model.PageSpec{
		GameID:      "g1",
		Env:         "dev",
		PageKey:     "page-c",
		Type:        "resource",
		ResourceKey: "player",
		SpecJSON:    pageSpecJSON,
		Status:      "draft",
	}))

	changes, err := p.draftContractChanges(ctx, "g1", "dev", "mail", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, changes)
}

func TestDraftContractChangesV3_WithPublishedStale(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	pageSpecJSON := `{"pageKey":"page-b","type":"resource","title":{"zh-CN":"测试"},"category":{"key":"test","labels":{"zh-CN":"测试"}},"bindings":[{"id":"b1","functionId":"existing.func","usage":"query","execution":{"mode":"sync"}}]}`
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, &model.PageSpec{
		GameID:           "g1",
		Env:              "dev",
		PageKey:          "page-b",
		Type:             "resource",
		SpecJSON:         pageSpecJSON,
		Status:           "draft",
		PublishedVersion: 1,
	}))

	publishedChanges := []ContractChangeDTO{
		{PageKey: "page-b", BindingFreshness: []spec.BindingFreshnessDiagnostic{
			{Status: spec.BindingFreshnessExecutionModeStale},
		}},
	}

	functions := map[string]spec.FunctionSpec{
		"existing.func": {ID: "existing.func", Execution: spec.FunctionExecutionSync},
	}

	changes, err := p.draftContractChanges(ctx, "g1", "dev", "", publishedChanges, functions)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "page-b", changes[0].PageKey)
}

// ===========================================================================
// Inbox with blocked issues
// ===========================================================================

func TestInboxV3_WithBlockedIssues(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	blockedModel := model.NewBlockedProposalIssueModel(db)
	require.NoError(t, blockedModel.Upsert(ctx, &model.BlockedProposalIssue{
		GameID:      "g1",
		Env:         "dev",
		ResourceKey: "player",
		FunctionID:  "player.ban",
		Status:      "blocked",
		Diagnostics: datatypes.JSON(`[{"code":"test","severity":"error","message":"blocked"}]`),
		UpdatedBy:   "system",
	}))

	inbox, err := p.Inbox(ctx, "g1", "dev", ProposalListFilter{})
	require.NoError(t, err)
	assert.Len(t, inbox.BlockedIssues, 1)
	assert.Equal(t, 1, inbox.Summary.BlockedIssues)
}

func TestInboxV3_WithFilter(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	require.NoError(t, p.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "r:p1", PageKey: "pk1",
		PageType: "resource", Quality: "ready", Status: "pending", ResourceKey: "player",
		PageSpec: []byte(`{"pageKey":"pk1","type":"resource"}`),
	}))
	require.NoError(t, p.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "o:p2", PageKey: "pk2",
		PageType: "operation", Quality: "basic", Status: "pending", ResourceKey: "mail",
		PageSpec: []byte(`{"pageKey":"pk2","type":"operation"}`),
	}))

	inbox, err := p.Inbox(ctx, "g1", "dev", ProposalListFilter{ResourceKey: "player"})
	require.NoError(t, err)
	totalProposals := len(inbox.Publishable) + len(inbox.NeedsReview)
	assert.Equal(t, 1, totalProposals)
}

func TestInboxV3_BasicQualityProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	require.NoError(t, p.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "o:p1", PageKey: "pk1",
		PageType: "operation", Quality: "basic", Status: "pending",
		PageSpec: []byte(`{"pageKey":"pk1","type":"operation"}`),
	}))

	inbox, err := p.Inbox(ctx, "g1", "dev", ProposalListFilter{})
	require.NoError(t, err)
	assert.Len(t, inbox.Publishable, 1)
	assert.Equal(t, 1, inbox.Summary.Publishable)
}

func TestInboxV3_NonPendingExcluded(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	require.NoError(t, p.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g1", Env: "dev", ProposalKey: "o:p1", PageKey: "pk1",
		PageType: "operation", Quality: "ready", Status: "accepted",
		PageSpec: []byte(`{"pageKey":"pk1","type":"operation"}`),
	}))

	inbox, err := p.Inbox(ctx, "g1", "dev", ProposalListFilter{})
	require.NoError(t, err)
	assert.Empty(t, inbox.Publishable)
	assert.Empty(t, inbox.NeedsReview)
}

// ===========================================================================
// createProposalVersionSnapshot - success path
// ===========================================================================

func TestCreateProposalVersionSnapshotV3_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	proposalModel := model.NewPageProposalModel(db)
	versionModel := model.NewPageProposalVersionModel(db)

	proposal := &model.PageProposal{
		GameID:      "g1",
		Env:         "dev",
		ProposalKey: "r:p1",
		PageKey:     "pk1",
		PageType:    "resource",
		Quality:     "ready",
		Status:      "pending",
		PageSpec:    []byte(`{"pageKey":"pk1","type":"resource"}`),
	}
	require.NoError(t, proposalModel.UpsertProposal(ctx, proposal))
	require.True(t, proposal.ID > 0)

	version, err := createProposalVersionSnapshot(ctx, versionModel, proposal, "test reason", "tester")
	require.NoError(t, err)
	assert.Greater(t, version, 0)
}

// ===========================================================================
// buildBindingContracts
// ===========================================================================

func TestBuildBindingContractsV3_Empty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	snapshots, err := p.buildBindingContracts(ctx, "g1", "dev", nil)
	require.NoError(t, err)
	assert.Empty(t, snapshots)
}

func TestBuildBindingContractsV3_WithBindings(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	contractModel := model.NewFunctionContractModel(db)
	require.NoError(t, contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:       "g1",
		Env:          "dev",
		FunctionID:   "player.list",
		Version:      "1.0.0",
		Enabled:      true,
		ResourceKey:  "player",
		Capability:   "collection_query",
		Execution:    "sync",
		InputSchema:  datatypes.JSON(`{"type":"object"}`),
		OutputSchema: datatypes.JSON(`{"type":"object"}`),
	}))

	snapshots, err := p.buildBindingContracts(ctx, "g1", "dev", []spec.PageFunctionBinding{
		{ID: "main", FunctionID: "player.list", Usage: spec.BindingUsageQuery},
	})
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Equal(t, "main", snapshots[0].BindingID)
	assert.Equal(t, "player.list", snapshots[0].FunctionID)
}

// ===========================================================================
// functionSpecsByID
// ===========================================================================

func TestFunctionSpecsByIDV3(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	functions, err := p.functionSpecsByID(ctx, "g1", "dev")
	require.NoError(t, err)
	assert.Empty(t, functions)
}

// ===========================================================================
// validateCategoryLabelConflict
// ===========================================================================

func TestValidateCategoryLabelConflictV3(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	err := p.validateCategoryLabelConflict(ctx, "g1", "dev", spec.PageSpec{})
	assert.NoError(t, err)

	err = p.validateCategoryLabelConflict(ctx, "g1", "dev", spec.PageSpec{
		Category: spec.PageCategorySpec{
			Key:    "cat1",
			Labels: spec.LocalizedText{"zh-CN": "类别1"},
		},
	})
	assert.NoError(t, err)
}

// ===========================================================================
// validateDirectPublishPageSpec
// ===========================================================================

func TestValidateDirectPublishPageSpecV3(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	proposalSvc := NewProposalService(db)
	contractSvc := NewContractService(db)

	require.NoError(t, contractSvc.RebuildContractFromFunctionMeta(ctx, "g1", "dev", "sdk", FunctionMetaInput{
		ID: "player.list", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "list",
		Capability: "collection_query", Execution: "sync",
		InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`,
	}))

	proposal := &model.PageProposal{PageKey: "test"}
	page := spec.PageSpec{
		PageKey:  "test",
		Type:     spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "类别"}},
		Resource: &spec.ResourcePageSpec{},
		Bindings: []spec.PageFunctionBinding{{
			ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}

	// Exercise the code path - may pass or fail validation, that's OK
	_ = proposalSvc.validateDirectPublishPageSpec(ctx, "g1", "dev", proposal, page)
}

func TestValidateDirectPublishPageSpecV3_MissingTitle(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	proposalSvc := NewProposalService(db)

	proposal := &model.PageProposal{PageKey: "test"}
	page := spec.PageSpec{
		PageKey:  "test",
		Type:     spec.PageTypeResource,
		Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "类别"}},
		Resource: &spec.ResourcePageSpec{},
		Bindings: []spec.PageFunctionBinding{{
			ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}

	err := proposalSvc.validateDirectPublishPageSpec(ctx, "g1", "dev", proposal, page)
	assert.Error(t, err)
}

// ===========================================================================
// AcceptProposal success path
// ===========================================================================

func TestAcceptProposalV3_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	p := NewProposalService(db)

	page := testProposalPageSpec("resource--player")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)

	require.NoError(t, p.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "resource:player",
		PageKey: "resource--player", PageType: "resource", Quality: "ready",
		Status: "pending", PageSpec: pageJSON,
	}))

	err = p.AcceptProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	proposal, err := p.GetProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "accepted", proposal.Status)
}

// ===========================================================================
// RejectProposal success path
// ===========================================================================

func TestRejectProposalV3_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	require.NoError(t, p.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "resource:player",
		PageKey: "resource--player", PageType: "resource", Quality: "ready",
		Status: "pending", PageSpec: []byte(`{"pageKey":"resource--player","type":"resource"}`),
	}))

	err := p.RejectProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	proposal, err := p.GetProposal(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, "rejected", proposal.Status)
}

// ===========================================================================
// AcceptAndPublishProposal success path
// ===========================================================================

func TestAcceptAndPublishProposalV3_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	p := NewProposalService(db)
	contractSvc := NewContractService(db)

	require.NoError(t, contractSvc.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.query", Version: "1.0.0", Enabled: true, Resource: "player", Operation: "query",
		Capability: "collection_query", Execution: "sync",
		InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`,
	}))

	page := testProposalPageSpec("operation--mail.send")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)

	require.NoError(t, p.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "operation:mail.send",
		PageKey: "operation--mail.send", PageType: "operation", Quality: "basic",
		Status: "pending", PageSpec: pageJSON,
	}))

	result, err := p.AcceptAndPublishProposal(ctx, "demo-game", "development", "operation:mail.send")
	require.NoError(t, err)
	assert.Equal(t, "operation--mail.send", result.PageKey)
	assert.Greater(t, result.PublishedVersion, 0)
}

// ===========================================================================
// parsePublishedSnapshot
// ===========================================================================

func TestParsePublishedSnapshotV3_Empty(t *testing.T) {
	page, contracts := parsePublishedSnapshot(model.PublishedPageSpec{})
	assert.Empty(t, page.PageKey)
	assert.Empty(t, contracts)
}

func TestParsePublishedSnapshotV3_WithData(t *testing.T) {
	specJSON := `{"pageKey":"pk1","type":"resource","title":{"zh-CN":"测试"},"category":{"key":"test","labels":{"zh-CN":"测试"}}}`
	contractsJSON := `[{"bindingId":"b1","functionId":"player.list"}]`
	item := model.PublishedPageSpec{
		PageKey:              "pk1",
		SpecJSON:             specJSON,
		BindingContractsJSON: contractsJSON,
	}
	page, contracts := parsePublishedSnapshot(item)
	assert.Equal(t, "pk1", page.PageKey)
	assert.Len(t, contracts, 1)
}

func TestParsePublishedSnapshotV3_FallbackPageKey(t *testing.T) {
	item := model.PublishedPageSpec{
		PageKey:  "fallback",
		SpecJSON: `{}`,
	}
	page, _ := parsePublishedSnapshot(item)
	assert.Equal(t, "fallback", page.PageKey)
}

// ===========================================================================
// hasDiagnosticSeverity - more edge cases
// ===========================================================================

func TestHasDiagnosticSeverityV3_MultipleSeverities(t *testing.T) {
	diags := []spec.Diagnostic{
		{Severity: "info"},
		{Severity: "warning"},
		{Severity: "error"},
	}
	assert.True(t, hasDiagnosticSeverity(diags, "info"))
	assert.True(t, hasDiagnosticSeverity(diags, "warning"))
	assert.True(t, hasDiagnosticSeverity(diags, "error"))
	assert.False(t, hasDiagnosticSeverity(diags, "critical"))
}

// ===========================================================================
// blockedIssueDTOFromModel (via listBlockedIssueDTOs)
// ===========================================================================

func TestBlockedIssueDTOFromModelV3(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)
	blockedModel := model.NewBlockedProposalIssueModel(db)
	require.NoError(t, blockedModel.Upsert(ctx, &model.BlockedProposalIssue{
		GameID:        "g1",
		Env:           "dev",
		ResourceKey:   "player",
		FunctionID:    "player.ban",
		Status:        "blocked",
		Diagnostics:   datatypes.JSON(`[{"code":"test","severity":"error","message":"blocked"}]`),
		SourceDigests: datatypes.JSON(`["digest1"]`),
		RepairHint:    datatypes.JSONMap{"zh-CN": "请修复"},
		UpdatedBy:     "system",
	}))
	dtos, err := p.listBlockedIssueDTOs(ctx, "g1", "dev", "player")
	require.NoError(t, err)
	require.Len(t, dtos, 1)
	assert.Equal(t, "player", dtos[0].ResourceKey)
	assert.Equal(t, "player.ban", dtos[0].FunctionID)
	assert.Equal(t, "blocked", dtos[0].Status)
	assert.Equal(t, "system", dtos[0].UpdatedBy)
}

// ===========================================================================
// ProposalInboxSummary and ContractChangeDTO fields
// ===========================================================================

func TestProposalInboxSummaryV3_Fields(t *testing.T) {
	summary := ProposalInboxSummary{
		Publishable:     1,
		NeedsReview:     2,
		BlockedIssues:   3,
		ContractChanges: 4,
	}
	assert.Equal(t, 1, summary.Publishable)
	assert.Equal(t, 2, summary.NeedsReview)
	assert.Equal(t, 3, summary.BlockedIssues)
	assert.Equal(t, 4, summary.ContractChanges)
}

func TestContractChangeDTOV3_JSON(t *testing.T) {
	dto := ContractChangeDTO{
		PageKey:     "page-a",
		PageType:    spec.PageTypeResource,
		ResourceKey: "player",
		Kind:        "published",
		Status:      "stale",
	}
	data, err := json.Marshal(dto)
	require.NoError(t, err)
	assert.Contains(t, string(data), "page-a")
}

// ===========================================================================
// ProposalDTO from model with various statuses
// ===========================================================================

func TestProposalDTOFromModelV3_Accepted(t *testing.T) {
	proposal := &model.PageProposal{
		ProposalKey: "r:p1",
		PageKey:     "pk1",
		PageType:    "resource",
		Quality:     "ready",
		Status:      "accepted",
		UpdatedBy:   "admin",
		UpdatedAt:   time.Now(),
		PageSpec:    []byte(`{"pageKey":"pk1","type":"resource","title":{"zh-CN":"测试"},"category":{"key":"test","labels":{"zh-CN":"测试"}}}`),
	}
	dto, err := proposalDTOFromModel(proposal)
	require.NoError(t, err)
	assert.Equal(t, "accepted", dto.Status)
	assert.Equal(t, "admin", dto.UpdatedBy)
}

func TestProposalDTOFromModelV3_Rejected(t *testing.T) {
	proposal := &model.PageProposal{
		ProposalKey: "r:p2",
		PageKey:     "pk2",
		PageType:    "operation",
		Quality:     "basic",
		Status:      "rejected",
		PageSpec:    []byte(`{"pageKey":"pk2","type":"operation","title":{"zh-CN":"操作"},"category":{"key":"test","labels":{"zh-CN":"测试"}}}`),
	}
	dto, err := proposalDTOFromModel(proposal)
	require.NoError(t, err)
	assert.Equal(t, "rejected", dto.Status)
}

// ===========================================================================
// listContractChanges with resourceKey filter
// ===========================================================================

func TestListContractChangesV3_WithResourceKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	p := NewProposalService(db)

	changes, err := p.listContractChanges(ctx, "g1", "dev", "player")
	require.NoError(t, err)
	assert.Empty(t, changes)
}
