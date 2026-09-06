// 补齐 GetAnalyticsFilters 的安装配置优先链路与文件回退错误分支。
package agent

import (
	"context"
	"testing"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http/httptest"
)

func newInstallationSvc(t *testing.T) *extensioninstallation.Service {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}, &model.ExtensionEvent{}))
	repos := extensiongorm.NewBundle(db)
	return extensioninstallation.NewService(repos.Installation, nil, nil)
}

func installAnalytics(t *testing.T, svc *extensioninstallation.Service, config map[string]any) {
	t.Helper()
	_, err := svc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAnalyticsExtensionID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         config,
		Operator:       "tester",
	})
	require.NoError(t, err)
}

// 安装配置携带 filters → 走安装优先分支并返回（不走文件回退）。
func TestGetAnalyticsFiltersFromInstallationConfig(t *testing.T) {
	inst := newInstallationSvc(t)
	installAnalytics(t, inst, map[string]any{
		"filters": []map[string]any{
			{"gameId": "g1", "filters": []string{"level", "vip"}},
		},
	})
	s := NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}})

	resp, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "g1", resp.Items[0].GameId)
	assert.Equal(t, 1, resp.Count)
}

// 安装配置无 filters 键 → 空集（ok=true 短路，不回退文件）。
func TestGetAnalyticsFiltersInstallationConfigWithoutFilters(t *testing.T) {
	inst := newInstallationSvc(t)
	installAnalytics(t, inst, map[string]any{"other": 1})
	s := NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}})

	resp, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Count)
}

// 安装配置的 filters 结构非法 → 透传错误（handler 500 分支）。
func TestGetAnalyticsFiltersInstallationConfigBroken(t *testing.T) {
	inst := newInstallationSvc(t)
	installAnalytics(t, inst, map[string]any{"filters": "not-a-list"})
	s := NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}})

	_, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.Error(t, err)
}

// 文件回退分支：指向不存在文件 → 空 items 兜底。
func TestGetAnalyticsFiltersFileFallbackEmpty(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	resp, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

// 安装列表查询失败（缺表）→ 降级文件回退而非报错。
func TestLoadFiltersInstallationListError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}))
	repos := extensiongorm.NewBundle(db)
	inst := extensioninstallation.NewService(repos.Installation, nil, nil)
	require.NoError(t, db.Migrator().DropTable(&model.ExtensionInstallation{}))

	s := NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}})
	items, ok, err := s.loadFiltersFromAnalyticsInstallation(context.Background())
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, items)
}

func TestHandlerGetAnalyticsFiltersInternalServerError(t *testing.T) {
	inst := newInstallationSvc(t)
	installAnalytics(t, inst, map[string]any{"filters": map[string]any{"bad": true}})
	h := NewHandler(NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}}))

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("GET", "/agents/analytics-filters", nil)
	h.GetAnalyticsFilters(ctx)
	assert.Equal(t, 500, rec.Code)
}
