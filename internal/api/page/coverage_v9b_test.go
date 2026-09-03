package page

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// BulkPublish / BulkUnpublish 服务层边界用例
// ---------------------------------------------------------------------------

// 无 pages:publish 权限 → 拒绝。
func TestV9B_BulkPublish_NoPermission(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "page_tester")

	_, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.Error(t, err)
}

// 有权限但缺少 game scope → 拒绝。
func TestV9B_BulkPublish_NoScope(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:publish")

	_, err := service.BulkPublish(v9NoScopeCtx(), &PageBulkRequest{})
	require.Error(t, err)
}

// pending 提案质量非 ready/basic → 进入 Skipped。
func TestV9B_BulkPublish_SkipsLowQualityProposal(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")

	v9UpsertProposal(t, service.svcCtx.DB, ctx, "operation:lowq", "operation--lowq", dbenum.ProposalStatusPending, []byte(`{"pageKey":"operation--lowq","type":"operation"}`))
	proposalModel := model.NewPageProposalModel(service.svcCtx.DB)
	p, err := proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "operation:lowq")
	require.NoError(t, err)
	p.Quality = "draft"
	require.NoError(t, proposalModel.UpsertProposal(ctx, p))

	resp, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	assert.Contains(t, resp.Skipped, "operation--lowq")
	assert.NotContains(t, resp.Published, "operation--lowq")
}

// ready 提案但 spec 损坏 → 进入 Failed（含原因），不影响整体返回。
func TestV9B_BulkPublish_RecordsFailedProposal(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")

	v9UpsertProposal(t, service.svcCtx.DB, ctx, "operation:broken", "operation--broken", dbenum.ProposalStatusPending, []byte(`{not-json`))
	proposalModel := model.NewPageProposalModel(service.svcCtx.DB)
	p, err := proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "operation:broken")
	require.NoError(t, err)
	p.Quality = "ready"
	require.NoError(t, proposalModel.UpsertProposal(ctx, p))

	resp, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Failed)
	found := false
	for _, failure := range resp.Failed {
		if failure["pageKey"] == "operation--broken" {
			found = true
			assert.NotEmpty(t, failure["error"])
		}
	}
	assert.True(t, found, "broken proposal must appear in Failed list")
}

// 无权限 → BulkUnpublish 拒绝。
func TestV9B_BulkUnpublish_NoPermission(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "page_tester")

	_, err := service.BulkUnpublish(ctx, &PageBulkRequest{})
	require.Error(t, err)
}

// 缺少 scope → BulkUnpublish 拒绝。
func TestV9B_BulkUnpublish_NoScope(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:publish")

	_, err := service.BulkUnpublish(v9NoScopeCtx(), &PageBulkRequest{})
	require.Error(t, err)
}

// scope 内无已发布页面 → 空结果。
func TestV9B_BulkUnpublish_EmptyScope(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")

	resp, err := service.BulkUnpublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Unpublished)
}

// ---------------------------------------------------------------------------
// SeedDemoData / demoDataSeeder
// ---------------------------------------------------------------------------

func v9bEnableTermDict(service *Service) {
	service.svcCtx.TermDictModel = model.NewTermDictionaryModel(service.svcCtx.DB)
}

// 全链路：Terms 落库 + 一键发布 + 演示警告写入。
func TestV9B_SeedDemoData_Success(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	v9bEnableTermDict(service)

	resp, err := service.SeedDemoData(ctx, &PageSeedDemoRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, len(demoTerms), resp.Summary["terms"])
	assert.Equal(t, len(demoWarnings), resp.Summary["registrationWarnings"])

	// Terms 真实落库
	terms, err := service.svcCtx.TermDictModel.List(ctx, "resource")
	require.NoError(t, err)
	assert.NotEmpty(t, terms)

	// 演示警告真实写入 registry store
	warnings := service.svcCtx.RegistryStore.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "demo-game",
		Env:    "development",
	})
	assert.Len(t, warnings, len(demoWarnings))

	// 幂等：再次执行不报错
	_, err = service.SeedDemoData(ctx, &PageSeedDemoRequest{})
	require.NoError(t, err)
}

// 缺少 scope → 拒绝。
func TestV9B_SeedDemoData_NoScope(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:publish")
	v9bEnableTermDict(service)

	_, err := service.SeedDemoData(v9NoScopeCtx(), &PageSeedDemoRequest{})
	require.Error(t, err)
}

// 无发布权限 → 拒绝。
func TestV9B_SeedDemoData_NoPermission(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	v9bEnableTermDict(service)
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "page_tester")

	_, err := service.SeedDemoData(ctx, &PageSeedDemoRequest{})
	require.Error(t, err)
}

// RegistryStore 为 nil 时 seedRegistrationWarnings 静默跳过（不 panic）。
func TestV9B_SeedRegistrationWarnings_NilStore(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:publish")
	service.svcCtx.RegistryStore = nil

	seeder := newDemoDataSeeder(service, "demo-game", "development")
	assert.NotPanics(t, func() { seeder.seedRegistrationWarnings() })
}

// newDemoDataSeeder 保存 scope 字段。
func TestV9B_NewDemoDataSeeder_Fields(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:publish")

	seeder := newDemoDataSeeder(service, "g1", "prod")
	require.NotNil(t, seeder)
	assert.Equal(t, "g1", seeder.gameID)
	assert.Equal(t, "prod", seeder.env)
	assert.Same(t, service, seeder.svc)
}

// upsertTerm 幂等：同一 key 二次 upsert 更新显示名而非报错。
func TestV9B_UpsertTerm_Idempotent(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	v9bEnableTermDict(service)
	seeder := newDemoDataSeeder(service, "demo-game", "development")

	require.NoError(t, seeder.upsertTerm(ctx, "resource", "player", "玩家"))
	require.NoError(t, seeder.upsertTerm(ctx, "resource", "player", "玩家V2"))

	terms, err := service.svcCtx.TermDictModel.List(ctx, "resource")
	require.NoError(t, err)
	require.Len(t, terms, 1)
	assert.Equal(t, "玩家V2", terms[0].Display["zh-CN"])
}

// seed 内部 BulkPublish 失败（无发布权限）→ 错误透传。
func TestV9B_Seed_BulkPublishErrorPropagates(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	v9bEnableTermDict(service)
	seeder := newDemoDataSeeder(service, "demo-game", "development")
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "page_tester")

	_, err := seeder.seed(ctx)
	require.Error(t, err)
}

// upsertTerm 底层存储失败 → 错误带 term 标识透传。
func TestV9B_Seed_UpsertTermError(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:publish")
	v9bEnableTermDict(service)

	sqlDB, err := service.svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	seeder := newDemoDataSeeder(service, "demo-game", "development")
	_, err = seeder.seed(svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert term")
}

// ---------------------------------------------------------------------------
// Handler 层：bulk / seed-demo / versions / rollback
// ---------------------------------------------------------------------------

// v9bHandlerContext 构造带 game scope + 认证用户的 gin 上下文。
func v9bHandlerContext(serviceCtx context.Context, method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, nil)
	c.Request = req.WithContext(serviceCtx)
	return c, rec
}

func TestV9B_Handler_BulkPublish_Success(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	seedResourceProposal(t, service)
	h := NewHandler(service)

	c, rec := v9bHandlerContext(ctx, http.MethodPost, "/api/v1/pages/bulk-publish")
	h.BulkPublish(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "published")
}

func TestV9B_Handler_BulkPublish_Error(t *testing.T) {
	h := setupTestHandler(t)

	c, rec := newTestContext(http.MethodPost, "/api/v1/pages/bulk-publish", "")
	h.BulkPublish(c)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestV9B_Handler_BulkUnpublish_Success(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	seedResourceProposal(t, service)
	_, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	h := NewHandler(service)

	c, rec := v9bHandlerContext(ctx, http.MethodPost, "/api/v1/pages/bulk-unpublish")
	h.BulkUnpublish(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "unpublished")
}

func TestV9B_Handler_BulkUnpublish_Error(t *testing.T) {
	h := setupTestHandler(t)

	c, rec := newTestContext(http.MethodPost, "/api/v1/pages/bulk-unpublish", "")
	h.BulkUnpublish(c)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestV9B_Handler_SeedDemoData_Success(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	v9bEnableTermDict(service)
	h := NewHandler(service)

	c, rec := v9bHandlerContext(ctx, http.MethodPost, "/api/v1/pages/seed-demo")
	h.SeedDemoData(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "summary")
}

func TestV9B_Handler_SeedDemoData_Error(t *testing.T) {
	h := setupTestHandler(t)

	c, rec := newTestContext(http.MethodPost, "/api/v1/pages/seed-demo", "")
	h.SeedDemoData(c)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// Versions：页面不存在 → 200 空列表（total=0）。
func TestV9B_Handler_Versions_MissingPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	h := NewHandler(service)

	c, rec := v9bHandlerContext(ctx, http.MethodGet, "/api/v1/pages/missing-page/versions")
	c.Params = gin.Params{{Key: "pageKey", Value: "missing-page"}}
	h.Versions(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "\"total\":0")
}

// VersionDetail：版本不存在 → 404。
func TestV9B_Handler_VersionDetail_NotFound(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	h := NewHandler(service)

	c, rec := v9bHandlerContext(ctx, http.MethodGet, "/api/v1/pages/missing-page/versions/1")
	c.Params = gin.Params{
		{Key: "pageKey", Value: "missing-page"},
		{Key: "versionId", Value: "1"},
	}
	h.VersionDetail(c)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Rollback：版本存在但草稿缺失 → 事务内 ErrPageNotFound → handler 404 分支。
func TestV9B_Handler_Rollback_DraftMissing(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:rollback")
	h := NewHandler(service)

	require.NoError(t, service.svcCtx.DB.Create(&model.PageVersion{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "ghost-page",
		Version:  1,
		SpecJSON: `{"pageKey":"ghost-page","type":"operation"}`,
		Status:   "published",
	}).Error)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pages/ghost-page/rollback", strings.NewReader(`{"versionId":"1","expectedDraftRevision":0}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req.WithContext(ctx)
	c.Params = gin.Params{{Key: "pageKey", Value: "ghost-page"}}
	h.Rollback(c)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Rollback：页面不存在 → 404。
func TestV9B_Handler_Rollback_NotFound(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:rollback")
	h := NewHandler(service)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pages/missing-page/rollback", strings.NewReader(`{"versionId":"1","expectedDraftRevision":0}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req.WithContext(ctx)
	c.Params = gin.Params{{Key: "pageKey", Value: "missing-page"}}
	h.Rollback(c)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ListDrafts：带 scope 与读权限 → 200。
func TestV9B_Handler_ListDrafts_Success(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	h := NewHandler(service)

	c, rec := v9bHandlerContext(ctx, http.MethodGet, "/api/v1/pages")
	h.ListDrafts(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "items") || strings.Contains(rec.Body.String(), "["))
}

// ListDrafts：缺少 scope → 服务错误分支（response.Error）。
func TestV9B_Handler_ListDrafts_ServiceError(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	h := NewHandler(service)

	c, rec := v9bHandlerContext(v9NoScopeCtx(), http.MethodGet, "/api/v1/pages")
	h.ListDrafts(c)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// Versions：缺少 scope → 服务错误分支（非 NotFound）。
func TestV9B_Handler_Versions_ServiceError(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	h := NewHandler(service)

	c, rec := v9bHandlerContext(v9NoScopeCtx(), http.MethodGet, "/api/v1/pages/p1/versions")
	c.Params = gin.Params{{Key: "pageKey", Value: "p1"}}
	h.Versions(c)

	assert.NotEqual(t, http.StatusOK, rec.Code)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

// VersionDetail：缺少 scope → 服务错误分支（非 NotFound）。
func TestV9B_Handler_VersionDetail_ServiceError(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	h := NewHandler(service)

	c, rec := v9bHandlerContext(v9NoScopeCtx(), http.MethodGet, "/api/v1/pages/p1/versions/1")
	c.Params = gin.Params{
		{Key: "pageKey", Value: "p1"},
		{Key: "versionId", Value: "1"},
	}
	h.VersionDetail(c)

	assert.NotEqual(t, http.StatusOK, rec.Code)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

// Rollback：缺少 scope → 服务错误分支（非 NotFound）。
func TestV9B_Handler_Rollback_ServiceError(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:rollback")
	h := NewHandler(service)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pages/p1/rollback", strings.NewReader(`{"versionId":"1","expectedDraftRevision":0}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req.WithContext(v9NoScopeCtx())
	c.Params = gin.Params{{Key: "pageKey", Value: "p1"}}
	h.Rollback(c)

	assert.NotEqual(t, http.StatusOK, rec.Code)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}
