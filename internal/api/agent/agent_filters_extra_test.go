// Covers the install-config-first chain of GetAnalyticsFilters and its file-fallback error branches.
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

// Install config carries filters -> take the install-first branch and return (no file fallback).
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

// Install config has no filters key -> empty set (ok=true short-circuit, no file fallback).
func TestGetAnalyticsFiltersInstallationConfigWithoutFilters(t *testing.T) {
	inst := newInstallationSvc(t)
	installAnalytics(t, inst, map[string]any{"other": 1})
	s := NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}})

	resp, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Count)
}

// Install config's filters shape is invalid -> error passed through (handler 500 branch).
func TestGetAnalyticsFiltersInstallationConfigBroken(t *testing.T) {
	inst := newInstallationSvc(t)
	installAnalytics(t, inst, map[string]any{"filters": "not-a-list"})
	s := NewService(&svc.ServiceContext{Extensions: &svc.ExtensionServices{Installation: inst}})

	_, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.Error(t, err)
}

// File fallback branch: path points at a nonexistent file -> empty items fallback.
func TestGetAnalyticsFiltersFileFallbackEmpty(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	resp, err := s.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

// Install list query fails (missing table) -> degrade to the file fallback instead of erroring.
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
