package page

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pageFlowEnv struct {
	service *Service
	handler *Handler
	router  *gin.Engine
	ctx     context.Context
}

// setupPageFlowEnv reuses the DB-backed service from service_test.go and adds
// an HTTP router with the authenticated, scoped context injected.
func setupPageFlowEnv(t *testing.T) *pageFlowEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service, ctx, _ := newPageTestService(t, "admin:all")

	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	RegisterDraftRoutes(router.Group("/api/v1/pages"), &svc.ServiceContext{})

	// Rebind the group to our handler by re-registering routes manually.
	router2 := gin.New()
	router2.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	g := router2.Group("/api/v1/pages")
	g.GET("", handler.ListDrafts)
	g.GET("/:pageKey", handler.GetDraft)
	g.PUT("/:pageKey", handler.SaveDraft)
	g.POST("/:pageKey/regenerate", handler.RegenerateDraft)
	g.POST("/:pageKey/validate", handler.Validate)
	g.POST("/:pageKey/preview", handler.Preview)
	g.POST("/:pageKey/publish", handler.Publish)
	g.POST("/:pageKey/unpublish", handler.Unpublish)
	g.GET("/:pageKey/versions", handler.Versions)
	g.GET("/:pageKey/versions/:versionId", handler.VersionDetail)
	g.POST("/:pageKey/rollback", handler.Rollback)
	_ = router

	return &pageFlowEnv{service: service, handler: handler, router: router2, ctx: ctx}
}

func (e *pageFlowEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *pageFlowEnv) saveDraft(t *testing.T, pageKey string, revision int) int {
	t.Helper()
	resp, err := e.service.SaveDraft(e.ctx, &PageSaveRequest{
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
// Handler error branches over HTTP
// ---------------------------------------------------------------------------

func TestPageFlow_HandlerNotFoundAndValidation(t *testing.T) {
	env := setupPageFlowEnv(t)

	// GetDraft for a missing page.
	rec := env.do(t, http.MethodGet, "/api/v1/pages/no-such-page", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Regenerate a missing page.
	rec = env.do(t, http.MethodPost, "/api/v1/pages/no-such-page/regenerate", `{"draftRevision":1}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Validate / Preview / Publish / Unpublish missing pages.
	rec = env.do(t, http.MethodPost, "/api/v1/pages/no-such-page/validate", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = env.do(t, http.MethodPost, "/api/v1/pages/no-such-page/preview", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = env.do(t, http.MethodPost, "/api/v1/pages/no-such-page/publish", `{"draftRevision":1}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = env.do(t, http.MethodPost, "/api/v1/pages/no-such-page/unpublish", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// SaveDraft bind failures.
	rec = env.do(t, http.MethodPut, "/api/v1/pages/bad-page", "{not-json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// SaveDraft payload violations.
	rev := 0
	base := func() PageSaveRequest {
		return PageSaveRequest{
			PageKey:       "violations",
			DraftRevision: &rev,
			Type:          spec.PageTypeOperation,
			ResourceKey:   "player",
			Title:         map[string]string{"zh-CN": "页面"},
			Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Operation:     testOperationPageSpec(),
			Bindings:      testPageBindings(),
		}
	}
	req := base()
	req.DraftRevision = nil
	_, err := env.service.SaveDraft(env.ctx, &req)
	require.Error(t, err)

	req = base()
	req.Type = spec.PageType("bogus")
	_, err = env.service.SaveDraft(env.ctx, &req)
	require.Error(t, err)

	req = base()
	req.Category.Key = " "
	_, err = env.service.SaveDraft(env.ctx, &req)
	require.Error(t, err)

	req = base()
	req.Title = map[string]string{"en-US": "only english"}
	_, err = env.service.SaveDraft(env.ctx, &req)
	require.Error(t, err)

	req = base()
	req.Category.Labels = spec.LocalizedText{"en-US": "english"}
	_, err = env.service.SaveDraft(env.ctx, &req)
	require.Error(t, err)

	// Fresh page with non-zero revision -> conflict.
	stale := 3
	req = base()
	req.DraftRevision = &stale
	_, err = env.service.SaveDraft(env.ctx, &req)
	require.Error(t, err)
}

func TestPageFlow_RegenerateConflictAndMissingProposal(t *testing.T) {
	env := setupPageFlowEnv(t)
	rev := env.saveDraft(t, "regen.page", 0)

	// Missing draftRevision.
	_, err := env.service.RegenerateDraft(env.ctx, &PageRegenerateRequest{PageKey: "regen.page"})
	require.Error(t, err)

	// Revision conflict.
	stale := rev + 10
	_, err = env.service.RegenerateDraft(env.ctx, &PageRegenerateRequest{PageKey: "regen.page", DraftRevision: &stale})
	require.Error(t, err)

	// No proposal available for regeneration.
	_, err = env.service.RegenerateDraft(env.ctx, &PageRegenerateRequest{PageKey: "regen.page", DraftRevision: &rev})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PageProposal")
}

func TestPageFlow_PublishFlows(t *testing.T) {
	env := setupPageFlowEnv(t)
	rev := env.saveDraft(t, "publish.page", 0)

	// Missing draftRevision.
	_, err := env.service.Publish(env.ctx, &PagePublishRequest{PageKey: "publish.page"})
	require.Error(t, err)

	// Revision conflict.
	stale := rev + 1
	_, err = env.service.Publish(env.ctx, &PagePublishRequest{PageKey: "publish.page", DraftRevision: &stale})
	require.Error(t, err)

	// Successful publish over HTTP.
	rec := env.do(t, http.MethodPost, "/api/v1/pages/publish.page/publish", `{"draftRevision":`+itoa(rev)+`}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Unpublish round trip.
	rec = env.do(t, http.MethodPost, "/api/v1/pages/publish.page/unpublish", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPageFlow_PreviewValidationFailure(t *testing.T) {
	env := setupPageFlowEnv(t)

	// A draft whose binding references an unknown function fails preview.
	rev := 0
	req := PageSaveRequest{
		PageKey:       "preview.bad",
		DraftRevision: &rev,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "预览"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Operation:     testOperationPageSpec(),
		Bindings: []spec.PageFunctionBinding{
			{ID: "ghost", FunctionID: "ghost.function", Usage: spec.BindingUsageQuery},
		},
	}
	_, err := env.service.SaveDraft(env.ctx, &req)
	require.NoError(t, err)

	_, err = env.service.Preview(env.ctx, &PagePreviewRequest{PageKey: "preview.bad"})
	require.Error(t, err)

	rec := env.do(t, http.MethodPost, "/api/v1/pages/preview.bad/preview", "")
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	// Validate is advisory and still succeeds.
	resp, err := env.service.Validate(env.ctx, &PageValidateRequest{PageKey: "preview.bad"})
	require.NoError(t, err)
	assert.False(t, resp.Valid)
}

func TestPageFlow_VersionsAndRollback(t *testing.T) {
	env := setupPageFlowEnv(t)

	rev1 := env.saveDraft(t, "version.page", 0)
	rev2 := env.saveDraft(t, "version.page", rev1)

	// Versions listing with clamp branches.
	resp, err := env.service.Versions(env.ctx, &PageVersionsRequest{PageKey: "version.page", Limit: 500, Offset: -5})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)
	assert.Equal(t, rev2, resp.CurrentDraftRevision)

	// Version detail happy path and missing version.
	detail, err := env.service.VersionDetail(env.ctx, &PageVersionDetailRequest{PageKey: "version.page", VersionID: itoa(rev1)})
	require.NoError(t, err)
	assert.Equal(t, rev1, detail.Version)

	_, err = env.service.VersionDetail(env.ctx, &PageVersionDetailRequest{PageKey: "version.page", VersionID: "999"})
	require.Error(t, err)

	// Rollback validation branches.
	_, err = env.service.Rollback(env.ctx, &PageRollbackRequest{PageKey: "version.page", VersionID: " ", ExpectedDraftRevision: &rev2})
	require.Error(t, err)

	_, err = env.service.Rollback(env.ctx, &PageRollbackRequest{PageKey: "version.page", VersionID: itoa(rev1)})
	require.Error(t, err)

	_, err = env.service.Rollback(env.ctx, &PageRollbackRequest{PageKey: "version.page", VersionID: "404", ExpectedDraftRevision: &rev2})
	require.Error(t, err)

	expected := rev2 + 5
	_, err = env.service.Rollback(env.ctx, &PageRollbackRequest{PageKey: "version.page", VersionID: itoa(rev1), ExpectedDraftRevision: &expected})
	require.Error(t, err)

	// Successful rollback.
	rbResp, err := env.service.Rollback(env.ctx, &PageRollbackRequest{PageKey: "version.page", VersionID: itoa(rev1), ExpectedDraftRevision: &rev2})
	require.NoError(t, err)
	assert.Equal(t, rev2+1, rbResp.DraftRevision)

	// HTTP: versions endpoint with bad query and rollback round trip.
	rec := env.do(t, http.MethodGet, "/api/v1/pages/version.page/versions?limit=abc", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rec = env.do(t, http.MethodGet, "/api/v1/pages/version.page/versions/1", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = env.do(t, http.MethodGet, "/api/v1/pages/version.page/versions/777", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = env.do(t, http.MethodPost, "/api/v1/pages/version.page/rollback", "{bad json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPageFlow_ListDraftsFilters(t *testing.T) {
	env := setupPageFlowEnv(t)
	env.saveDraft(t, "list.a", 0)

	rev := 0
	_, err := env.service.SaveDraft(env.ctx, &PageSaveRequest{
		PageKey:       "other.b",
		DraftRevision: &rev,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "order",
		Title:         map[string]string{"zh-CN": "订单"},
		Category:      spec.PageCategorySpec{Key: "order", Labels: spec.LocalizedText{"zh-CN": "订单"}},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.NoError(t, err)

	resp, err := env.service.ListDrafts(env.ctx, &PageDraftListRequest{ResourceKey: "player"})
	require.NoError(t, err)
	keys := map[string]bool{}
	for _, item := range resp.Items {
		keys[item.PageKey] = true
	}
	assert.True(t, keys["list.a"])
	assert.False(t, keys["other.b"])

	resp, err = env.service.ListDrafts(env.ctx, &PageDraftListRequest{Status: "draft"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Items)

	rec := env.do(t, http.MethodGet, "/api/v1/pages?status=draft", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ---------------------------------------------------------------------------
// Internal helper edge cases
// ---------------------------------------------------------------------------

func TestPageFlow_InternalHelpers(t *testing.T) {
	env := setupPageFlowEnv(t)

	// buildPageSpecJSON with a corrupted SpecJSON.
	bad := &model.PageSpec{SpecJSON: "{not-json"}
	_, err := buildPageSpecJSON(bad)
	require.Error(t, err)

	// pagePublishSource guards.
	empty := NewService(nil)
	assert.Equal(t, pagePublishSource{}, empty.pagePublishSource(env.ctx, "g", "e", nil))

	db := env.service.svcCtx.DB
	src := env.service.pagePublishSource(env.ctx, "demo-game", "development", &model.PageSpec{BaseProposalKey: "no-such-proposal"})
	assert.Empty(t, src.FunctionDigest)

	// normalizedFunctions without scope/DB returns empty.
	anonService := NewService(&svc.ServiceContext{})
	assert.Empty(t, anonService.normalizedFunctions(context.Background()))

	// auditPageEvent without audit service is a no-op.
	anonService.auditPageEvent(context.Background(), "page.draft.save", "g", "e", "k", nil)

	// proposalReplacementForDraft guards.
	_, err = empty.proposalReplacementForDraft(env.ctx, "g", "e", nil)
	require.Error(t, err)

	// findDraft on unknown page.
	_, err = env.service.findDraft(env.ctx, "ghost.page")
	require.Error(t, err)

	// Corrupt version SpecJSON fails VersionDetail.
	require.NoError(t, db.Create(&model.PageVersion{
		GameID: "demo-game", Env: "development", PageKey: "corrupt.page",
		Version: 1, SpecJSON: "{broken", Status: "draft", CreatedBy: "t",
	}).Error)
	_, err = env.service.VersionDetail(env.ctx, &PageVersionDetailRequest{PageKey: "corrupt.page", VersionID: "1"})
	require.Error(t, err)

	_, err = env.service.Rollback(env.ctx, &PageRollbackRequest{PageKey: "corrupt.page", VersionID: "1", ExpectedDraftRevision: intPtr(1)})
	require.Error(t, err)
}

func intPtr(v int) *int { return &v }

func itoa(v int) string {
	b, _ := json.Marshal(v)
	return strings.Trim(string(b), `"`)
}
