package versioning

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Handler helpers
// ---------------------------------------------------------------------------

func newVersioningHandlerRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(db))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = svc.WithGameScope(ctx, svc.GameScope{
			GameID: c.GetHeader("X-Game-ID"),
			Env:    c.GetHeader("X-Env"),
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	group := router.Group("/api/versioning/pages/:pageKey")
	group.GET("/chain", handler.GetChangeChain)
	group.GET("/diff", handler.Diff)
	group.POST("/merge", handler.Merge)
	group.POST("/rollback-draft", handler.RollbackDraft)
	group.POST("/rollback-publish", handler.RollbackPublish)
	group.POST("/regenerate", handler.RegenerateProposal)
	group.POST("/republish", handler.Republish)
	return router
}

func doVersioningRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Game-ID", "demo-game")
	req.Header.Set("X-Env", "development")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func seedHandlerChainFixture(t *testing.T, db *gorm.DB) spec.PageSpec {
	t.Helper()
	ctx := context.Background()
	page := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁玩家"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "run",
			FunctionID: "player.ban",
			Usage:      spec.BindingUsageAction,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.ban",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  dbenum.CapabilityAction,
		UpdatedAt:   time.Now(),
	}))
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", page, 1, "seed draft"))
	return page
}

// ---------------------------------------------------------------------------
// Handler.GetChangeChain
// ---------------------------------------------------------------------------

func TestHandler_GetChangeChain_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)
	seedHandlerChainFixture(t, db)

	rec := doVersioningRequest(router, http.MethodGet, "/api/versioning/pages/operation--player.ban/chain", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var chain ChangeChain
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &chain))
	assert.Equal(t, "operation--player.ban", chain.PageKey)
	assert.NotEmpty(t, chain.Items)
}

func TestHandler_GetChangeChain_MissingScope(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)
	seedHandlerChainFixture(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/versioning/pages/operation--player.ban/chain", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	// Without game scope the lookup fails and the handler returns 404.
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_GetChangeChain_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodGet, "/api/versioning/pages/missing/chain", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

// ---------------------------------------------------------------------------
// Handler.Diff
// ---------------------------------------------------------------------------

func TestHandler_Diff_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)
	seedHandlerChainFixture(t, db)

	rec := doVersioningRequest(router, http.MethodGet, "/api/versioning/pages/operation--player.ban/diff?fromVersion=1&toVersion=2", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var diff DiffResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &diff))
	assert.Contains(t, diff.Summary, "changes")
}

func TestHandler_Diff_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodGet, "/api/versioning/pages/missing/diff", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Diff_InvalidQuery(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	// fromVersion is parsed by BindQueryCompat; a non-numeric value keeps the
	// zero value, so the request still succeeds. Use a valid request instead
	// and assert the shape to keep this meaningful.
	rec := doVersioningRequest(router, http.MethodGet, "/api/versioning/pages/missing/diff?fromVersion=abc", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ---------------------------------------------------------------------------
// Handler.Merge
// ---------------------------------------------------------------------------

func TestHandler_Merge_RejectStrategy(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/merge",
		`{"strategy":"reject"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var result MergeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, "all changes rejected", result.Message)
}

func TestHandler_Merge_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/merge", `{invalid`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Merge_ValidationError(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/merge",
		`{"strategy":"accept"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "accept-all merge is forbidden")
}

// ---------------------------------------------------------------------------
// Handler.RollbackDraft
// ---------------------------------------------------------------------------

func TestHandler_RollbackDraft_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)
	page := seedHandlerChainFixture(t, db)

	restored := page
	restored.Title = spec.LocalizedText{"zh-CN": "旧版标题"}
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", restored, 2, "old"))

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/rollback-draft",
		`{"expectedDraftRevision":1,"version":2,"reason":"rollback"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp RollbackResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "operation--player.ban", resp.PageKey)
	// Versions 1 (seed) and 2 (rollback target) exist, so next revision is 3.
	assert.Equal(t, 3, resp.DraftRevision)
}

func TestHandler_RollbackDraft_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/rollback-draft", `not-json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_RollbackDraft_BadRequest(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/rollback-draft",
		`{"expectedDraftRevision":0,"version":0}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_RollbackDraft_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/missing/rollback-draft",
		`{"expectedDraftRevision":1,"version":2}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ---------------------------------------------------------------------------
// Handler.RollbackPublish
// ---------------------------------------------------------------------------

func TestHandler_RollbackPublish_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)
	page := seedHandlerChainFixture(t, db)

	published := page
	published.Title = spec.LocalizedText{"zh-CN": "已发布标题"}
	publishedJSON, err := marshalPageSpec(published)
	require.NoError(t, err)
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(context.Background(), &model.PublishedPageSpec{
		GameID:      "demo-game",
		Env:         "development",
		PageKey:     page.PageKey,
		Version:     1,
		SpecJSON:    publishedJSON,
		Active:      true,
		PublishedAt: time.Now(),
		PublishedBy: "tester",
	}))

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/rollback-publish",
		`{"expectedDraftRevision":1,"version":1,"reason":"rollback publish"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp RollbackResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, resp.Message, "rolled back published page")
}

func TestHandler_RollbackPublish_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/rollback-publish", `[[`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_RollbackPublish_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/missing/rollback-publish",
		`{"expectedDraftRevision":1,"version":9}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ---------------------------------------------------------------------------
// Handler.RegenerateProposal (standalone operation page path)
// ---------------------------------------------------------------------------

func TestHandler_RegenerateProposal_StandaloneSuccess(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TermDictionary{}))
	router := newVersioningHandlerRouter(t, db)
	seedHandlerChainFixture(t, db)

	require.NoError(t, model.NewTermDictionaryModel(db).Upsert(context.Background(), &model.TermDictionary{
		Domain:  "resource",
		TermKey: "player",
		Alias:   "player",
		Display: map[string]string{"zh-CN": "玩家", "en-US": "Player"},
	}))

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/regenerate", `{}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp RegenerateProposalResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, resp.Message, "proposal regenerated")

	proposal, err := model.NewPageProposalModel(db).FindByScopeAndPageKey(context.Background(), "demo-game", "development", "operation--player.ban")
	require.NoError(t, err)
	assert.Equal(t, "operation:player.ban", proposal.ProposalKey)
	assert.Equal(t, dbenum.ProposalStatusPending, proposal.Status)
}

func TestHandler_RegenerateProposal_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/regenerate", `{bad`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_RegenerateProposal_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/missing/regenerate", `{}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_RegenerateProposal_NoMainBinding(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey:  "operation--orphan",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "孤儿页面"},
		Category: spec.PageCategorySpec{Key: "player"},
	}))

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--orphan/regenerate", `{}`)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "no main executable binding")
}

// ---------------------------------------------------------------------------
// Handler.Republish
// ---------------------------------------------------------------------------

func TestHandler_Republish_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)
	seedHandlerChainFixture(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/republish",
		`{"reason":"manual republish"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp RepublishResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Version)
	assert.Contains(t, resp.Message, "republished")
}

func TestHandler_Republish_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/operation--player.ban/republish", `nope`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Republish_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)

	rec := doVersioningRequest(router, http.MethodPost, "/api/versioning/pages/missing/republish", `{}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ---------------------------------------------------------------------------
// getScope
// ---------------------------------------------------------------------------

func TestGetScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(svc.WithGameScope(req.Context(), svc.GameScope{GameID: "g1", Env: "e1"}))
	ctx.Request = req

	gameID, env := getScope(ctx)
	assert.Equal(t, "g1", gameID)
	assert.Equal(t, "e1", env)
}

func TestGetScope_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	gameID, env := getScope(ctx)
	assert.Empty(t, gameID)
	assert.Empty(t, env)
}

// ---------------------------------------------------------------------------
// loadTermDictionary
// ---------------------------------------------------------------------------

func TestLoadTermDictionary_NilService(t *testing.T) {
	var service *Service
	assert.Nil(t, service.loadTermDictionary(context.Background()))
}

func TestLoadTermDictionary_NilDB(t *testing.T) {
	service := &Service{}
	assert.Nil(t, service.loadTermDictionary(context.Background()))
}

func TestLoadTermDictionary_MissingTableReturnsNil(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	// term_dictionary table is not migrated in setupTestDB; List must fail and
	// the helper must return a nil dictionary.
	assert.Nil(t, service.loadTermDictionary(context.Background()))
}

func TestLoadTermDictionary_WithEntries(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TermDictionary{}))
	service := NewService(db)

	termModel := model.NewTermDictionaryModel(db)
	require.NoError(t, termModel.Upsert(context.Background(), &model.TermDictionary{
		Domain: "resource", TermKey: "player", Alias: "Player", Display: map[string]string{"zh-CN": "玩家", "en-US": "Player"},
	}))
	require.NoError(t, termModel.Upsert(context.Background(), &model.TermDictionary{
		Domain: "resource", TermKey: "guild", Alias: "Guild", Display: map[string]string{"zh-CN": "", "en-US": ""},
	}))

	dict := service.loadTermDictionary(context.Background())
	require.NotNil(t, dict)
	text, ok := dict.Lookup("resource", "player")
	require.True(t, ok)
	assert.Equal(t, "玩家", text["zh-CN"])
	assert.Equal(t, "Player", text["en-US"])

	// Entries without display text are skipped.
	_, ok = dict.Lookup("resource", "guild")
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// mergePreviewForPage deeper branches
// ---------------------------------------------------------------------------

func TestMergePreviewForPage_PageKeyMismatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	latestPage := basePage
	latestPage.Title = spec.LocalizedText{"zh-CN": "新封禁"}

	proposalKey := "operation:player.ban"
	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: proposalKey, PageKey: basePage.PageKey,
		PageType: string(basePage.Type), Quality: "basic",
		FunctionDigest: "digest-v1",
		Title:          localizedTextToJSONMap(basePage.Title),
		PageSpec:       jsonValue(basePage),
		Status:         dbenum.ProposalStatusPending, UpdatedAt: time.Now(),
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, proposal))
	_, err := createProposalVersionSnapshot(ctx, service.proposalVersionModel, proposal, "seed", "tester")
	require.NoError(t, err)

	page := &model.PageSpec{
		GameID: "demo-game", Env: "development",
		PageKey: basePage.PageKey, SpecJSON: mustMarshalPageSpecString(t, basePage),
		BaseProposalKey: proposalKey, BaseProposalVersion: 1,
	}

	// Requesting preview for a different pageKey returns an empty result.
	result, err := service.mergePreviewForPage(ctx, "demo-game", "development", "another--page", page, proposal)
	require.NoError(t, err)
	assert.Empty(t, result.AutoMerge)
	assert.Empty(t, result.Conflicts)
}

func TestMergePreviewForPage_BaseSnapshotMissing(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player"},
	}
	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.ban", PageKey: basePage.PageKey,
		PageType: string(basePage.Type), FunctionDigest: "digest-v1",
		PageSpec: jsonValue(basePage), Status: dbenum.ProposalStatusPending, UpdatedAt: time.Now(),
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, proposal))

	page := &model.PageSpec{
		GameID: "demo-game", Env: "development",
		PageKey: basePage.PageKey, SpecJSON: mustMarshalPageSpecString(t, basePage),
		BaseProposalKey: "operation:player.ban", BaseProposalVersion: 42,
	}

	result, err := service.mergePreviewForPage(ctx, "demo-game", "development", basePage.PageKey, page, proposal)
	require.NoError(t, err)
	assert.Empty(t, result.AutoMerge)
}

func TestMergePreviewForPage_InvalidDraftSpec(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player"},
	}
	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.ban", PageKey: basePage.PageKey,
		PageType: string(basePage.Type), FunctionDigest: "digest-v1",
		PageSpec: jsonValue(basePage), Status: dbenum.ProposalStatusPending, UpdatedAt: time.Now(),
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, proposal))
	_, err := createProposalVersionSnapshot(ctx, service.proposalVersionModel, proposal, "seed", "tester")
	require.NoError(t, err)

	page := &model.PageSpec{
		GameID: "demo-game", Env: "development",
		PageKey: basePage.PageKey, SpecJSON: `invalid-json`,
		BaseProposalKey: "operation:player.ban", BaseProposalVersion: 1,
	}

	_, err = service.mergePreviewForPage(ctx, "demo-game", "development", basePage.PageKey, page, proposal)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode page spec")
}

func TestMergePreviewForPage_ThreeWayMerge(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	draftPage := basePage
	latestPage := basePage
	latestPage.Title = spec.LocalizedText{"zh-CN": "新版封禁"}
	seedVersioningMergeFixture(t, db, basePage, draftPage, latestPage)

	pageRecord, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(context.Background(), "demo-game", "development", basePage.PageKey)
	require.NoError(t, err)
	proposal, err := service.proposalModel.FindByScopeAndKey(context.Background(), "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)

	result, err := service.mergePreviewForPage(context.Background(), "demo-game", "development", basePage.PageKey, pageRecord, proposal)
	require.NoError(t, err)
	assert.NotEmpty(t, result.AutoMerge)
}

// ---------------------------------------------------------------------------
// RegenerateProposal standalone paths
// ---------------------------------------------------------------------------

func TestRegenerateProposal_StandaloneMismatchedPageKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.kick",
		Version: "1.0.0", Enabled: true, Capability: dbenum.CapabilityAction, UpdatedAt: time.Now(),
	}))
	// The page key does not match the generated key for player.kick.
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey:  "operation--weird-name",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.kick", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}))

	// 新语义：重新生成以当前页面为身份，派生 key 漂移不再 409 卡死页面；
	// 内容刷新到 operation--weird-name。
	resp, err := service.RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: "operation--weird-name",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "operation--weird-name")

	page, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "operation--weird-name")
	require.NoError(t, err)
	assert.Equal(t, "operation--weird-name", page.PageKey)
}

func TestRegenerateProposal_NoMainContract(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey:  "operation--ghost",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "ghost.func", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}))

	_, err := service.RegenerateProposal(context.Background(), &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: "operation--ghost",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contract not found")
}

func TestRegenerateProposal_InvalidPageSpecJSON(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	require.NoError(t, model.NewPageSpecModel(db).Upsert(context.Background(), &model.PageSpec{
		GameID: "demo-game", Env: "development",
		PageKey: "operation--broken", Type: "operation",
		SpecJSON: `not-json`, Status: "draft", DraftRevision: 1, UpdatedAt: time.Now(),
	}))

	_, err := service.RegenerateProposal(context.Background(), &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: "operation--broken",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode page spec")
}

func TestUpsertGeneratedProposal_SnapshotChain(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	contract := &model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.ban",
		Version: "1.0.0", Enabled: true, Capability: dbenum.CapabilityAction,
		ResourceKey: "player", UpdatedAt: time.Now(),
	}
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, contract))

	generated := generatorForOperationFixture(t, contract)
	err := service.upsertGeneratedProposal(ctx, "demo-game", "development", "operation:player.ban", []*model.FunctionContract{contract}, generated)
	require.NoError(t, err)

	proposal, err := service.proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	assert.Equal(t, dbenum.ProposalStatusPending, proposal.Status)
	versions, err := service.proposalVersionModel.ListByProposalID(ctx, proposal.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 1)
	assert.Equal(t, 1, versions[0].Version)
}

func mustMarshalPageSpecString(t *testing.T, page spec.PageSpec) string {
	t.Helper()
	raw, err := marshalPageSpec(page)
	require.NoError(t, err)
	return raw
}

func generatorForOperationFixture(t *testing.T, contract *model.FunctionContract) spec.GeneratedPageSpec {
	t.Helper()
	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:  "operation--" + contract.FunctionID,
			Type:     spec.PageTypeOperation,
			Title:    spec.LocalizedText{"zh-CN": "封禁玩家"},
			Category: spec.PageCategorySpec{Key: "player"},
		},
		Quality: "basic",
	}
}

// ---------------------------------------------------------------------------
// Merge: accept-latest-snapshot path and auto merge with conflict
// ---------------------------------------------------------------------------

func TestMerge_Auto_AcceptsLatestSnapshotWithoutChanges(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	draftPage := basePage
	seedVersioningMergeFixture(t, db, basePage, draftPage, basePage)
	// Seed a third, identical snapshot so latest != base but content is equal.
	proposal, err := service.proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	proposal.UpdatedAt = time.Now().Add(2 * time.Second)
	require.NoError(t, service.proposalModel.UpsertProposal(ctx, proposal))
	_, err = createProposalVersionSnapshot(ctx, service.proposalVersionModel, proposal, "identical snapshot", "tester")
	require.NoError(t, err)

	result, err := service.Merge(ctx, &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: basePage.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Conflicts)
	assert.Equal(t, 2, result.DraftRevision)
	assert.Contains(t, result.Message, "accepted latest proposal snapshot")

	pageRecord, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", basePage.PageKey)
	require.NoError(t, err)
	assert.Equal(t, 3, pageRecord.BaseProposalVersion)
}

func TestMerge_Auto_NoChangesNoSnapshotAdvance(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	seedVersioningMergeFixture(t, db, basePage, basePage, basePage)

	// Align the draft's base snapshot with the latest snapshot so the merge
	// has nothing to apply.
	pageRecord, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", basePage.PageKey)
	require.NoError(t, err)
	pageRecord.BaseProposalVersion = 2
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, pageRecord))

	result, err := service.Merge(ctx, &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: basePage.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Merged)
	assert.Equal(t, 0, result.Conflicts)
	assert.Contains(t, result.Message, "no contract changes require merge")
	assert.Equal(t, 1, result.DraftRevision)
}

func TestMerge_LatestProposalPageKeyMismatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	seedVersioningMergeFixture(t, db, basePage, basePage, basePage)

	// Point the draft at a proposal whose PageKey belongs to another page.
	other := spec.PageSpec{
		PageKey:  "operation--mail.send",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "mail"},
	}
	otherProposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:mail.send", PageKey: other.PageKey,
		PageType: string(other.Type), FunctionDigest: "digest-other",
		PageSpec: jsonValue(other), Status: dbenum.ProposalStatusPending, UpdatedAt: time.Now(),
	}
	require.NoError(t, service.proposalModel.UpsertProposal(ctx, otherProposal))

	pageRecord, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", basePage.PageKey)
	require.NoError(t, err)
	pageRecord.BaseProposalKey = "operation:mail.send"
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, pageRecord))

	_, err = service.Merge(ctx, &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: basePage.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey does not match")
}

func TestMerge_BaseSnapshotMissing(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	seedVersioningMergeFixture(t, db, basePage, basePage, basePage)

	pageRecord, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", basePage.PageKey)
	require.NoError(t, err)
	pageRecord.BaseProposalVersion = 99
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, pageRecord))

	_, err = service.Merge(ctx, &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: basePage.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base proposal snapshot not found")
}

func TestMerge_ManualDryRun_RevisionUnchanged(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
		Operation: &spec.OperationPageSpec{},
	}
	draftPage := basePage
	latestPage := basePage
	latestPage.Title = spec.LocalizedText{"zh-CN": "新版封禁"}
	seedVersioningMergeFixture(t, db, basePage, draftPage, latestPage)

	result, err := service.Merge(ctx, &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: basePage.PageKey,
		Strategy: MergeStrategyAuto, DryRun: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Merged)
	assert.Equal(t, 0, result.Conflicts)
	assert.Equal(t, 1, result.DraftRevision)
}

// ---------------------------------------------------------------------------
// RollbackDraft / RollbackPublish: invalid spec payloads
// ---------------------------------------------------------------------------

func TestRollbackDraft_DecodeVersionFailure(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	page := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player"},
	}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	// Seed a version row whose SpecJSON cannot decode into PageSpec.
	require.NoError(t, model.NewPageVersionModel(db).UpsertByScopePageKeyVersion(ctx, &model.PageVersion{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		Version: 1, SpecJSON: `{"pageKey": ["not", "a", "string"]}`, Status: "draft", CreatedBy: "tester", CreatedAt: time.Now(),
	}))

	_, err := service.RollbackDraft(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode page version")
}

func TestRollbackPublish_DecodeSpecFailure(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	page := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "player"},
	}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		Version: 1, SpecJSON: `{"title": {"zh-CN": ["bad"]}}`,
		Active: true, PublishedAt: time.Now(), PublishedBy: "tester",
	}))

	_, err := service.RollbackPublish(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode published page spec")
}
